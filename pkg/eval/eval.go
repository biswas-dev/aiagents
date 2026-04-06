package eval

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/biswas-dev/aiagents/pkg/agent"
)

// TestCase defines an evaluation test
type TestCase struct {
	Name           string
	Goal           string
	ExpectedAnswer string
	ExpectedTools  []string
	MaxIterations  int
	Validators     []func(string) bool
}

// Result holds the evaluation outcome for a single test
type Result struct {
	TestName    string        `json:"test_name"`
	Passed      bool          `json:"passed"`
	Answer      string        `json:"answer"`
	ToolsCalled []string      `json:"tools_called,omitempty"`
	Iterations  int           `json:"iterations"`
	Duration    time.Duration `json:"duration"`
	FailReason  string        `json:"fail_reason,omitempty"`
}

// Suite runs a set of test cases against an agent
type Suite struct {
	Cases []TestCase
}

// Run executes all test cases and returns results
func (s *Suite) Run(a *agent.Agent) []Result {
	var results []Result
	for _, tc := range s.Cases {
		start := time.Now()
		answer, err := a.Run(tc.Goal)
		duration := time.Since(start)

		r := Result{
			TestName: tc.Name,
			Answer:   answer,
			Duration: duration,
		}

		if err != nil {
			r.FailReason = "agent error: " + err.Error()
		} else if tc.ExpectedAnswer != "" && !strings.Contains(strings.ToLower(answer), strings.ToLower(tc.ExpectedAnswer)) {
			r.FailReason = fmt.Sprintf("expected %q in answer", tc.ExpectedAnswer)
		} else {
			r.Passed = true
			for _, v := range tc.Validators {
				if !v(answer) {
					r.Passed = false
					r.FailReason = "custom validator failed"
					break
				}
			}
		}

		results = append(results, r)
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %s (%.1fs)\n", status, tc.Name, duration.Seconds())
	}
	return results
}

// LLMJudge uses an LLM to score an answer (1-5)
func LLMJudge(client *agent.Client, question, answer string) (int, string) {
	resp, err := client.Send(&agent.MessagesRequest{
		MaxTokens:   512,
		Temperature: agent.Ptr(0.0),
		Messages: []agent.Message{{
			Role: "user",
			Content: agent.MustJSON(fmt.Sprintf(`Score this answer 1-5 (5=perfect). Output ONLY JSON: {"score": N, "reasoning": "..."}

Question: %s

Answer: %s`, question, answer)),
		}},
	})
	if err != nil {
		return 0, "judge error: " + err.Error()
	}
	for _, b := range resp.Content {
		if b.Type == "text" {
			var result struct {
				Score     int    `json:"score"`
				Reasoning string `json:"reasoning"`
			}
			json.Unmarshal([]byte(b.Text), &result)
			return result.Score, result.Reasoning
		}
	}
	return 0, "no response"
}

// SelfConsistent runs inference N times and returns majority answer + confidence
func SelfConsistent(client *agent.Client, prompt string, extract func(string) string, samples int) (string, float64) {
	answers := make([]string, samples)
	done := make(chan struct{}, samples)

	for i := 0; i < samples; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			resp, err := client.Send(&agent.MessagesRequest{
				MaxTokens:   512,
				Temperature: agent.Ptr(0.4),
				Messages:    []agent.Message{{Role: "user", Content: agent.MustJSON(prompt)}},
			})
			if err != nil {
				return
			}
			for _, b := range resp.Content {
				if b.Type == "text" {
					answers[idx] = extract(b.Text)
				}
			}
		}(i)
	}
	for i := 0; i < samples; i++ {
		<-done
	}

	counts := map[string]int{}
	for _, a := range answers {
		if a != "" {
			counts[a]++
		}
	}
	best, bestCount := "", 0
	for a, c := range counts {
		if c > bestCount {
			best, bestCount = a, c
		}
	}
	return best, float64(bestCount) / float64(samples)
}
