package guardrails

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/biswas-dev/aiagents/pkg/agent"
)

// Validator validates agent outputs before they reach the user
type Validator struct {
	client *agent.Client
	rules  []Rule
}

// Rule is a validation rule
type Rule struct {
	Name  string
	Check func(output string) (bool, string) // returns (passed, reason)
}

// NewValidator creates a validator with default rules
func NewValidator(client *agent.Client) *Validator {
	v := &Validator{client: client}
	v.rules = []Rule{
		{Name: "pii_check", Check: CheckPII},
		{Name: "length_check", Check: CheckLength(50000)},
	}
	return v
}

// AddRule adds a custom validation rule
func (v *Validator) AddRule(r Rule) {
	v.rules = append(v.rules, r)
}

// Validate checks the output against all rules, then optionally runs LLM critic
func (v *Validator) Validate(output, taskDescription string) (*ValidationResult, error) {
	result := &ValidationResult{Pass: true, Output: output}

	// Run rule-based checks
	for _, rule := range v.rules {
		passed, reason := rule.Check(output)
		if !passed {
			result.Pass = false
			result.Issues = append(result.Issues, fmt.Sprintf("[%s] %s", rule.Name, reason))
		}
	}

	return result, nil
}

// ValidateWithCritic runs LLM-based validation (more expensive, more thorough)
func (v *Validator) ValidateWithCritic(output, taskDescription string) (*ValidationResult, error) {
	// First run rule-based checks
	result, err := v.Validate(output, taskDescription)
	if err != nil {
		return nil, err
	}

	// Then run LLM critic
	prompt := fmt.Sprintf(`Review this agent output for:
1. Factual accuracy — flag claims that seem invented
2. Safety — flag harmful instructions or dangerous advice
3. Completeness — does it answer the question?
4. Format — is the output well-structured?

Output ONLY valid JSON: {"pass": true/false, "issues": ["issue1"], "corrected_output": "..."}
If pass=true, corrected_output should equal the original.

Task: %s

Agent output:
%s`, taskDescription, output)

	resp, err := v.client.Send(&agent.MessagesRequest{
		MaxTokens:   1024,
		Temperature: agent.Ptr(0.0),
		Messages:    []agent.Message{{Role: "user", Content: agent.MustJSON(prompt)}},
	})
	if err != nil {
		return result, nil // Fail open — return rule-based result
	}

	for _, b := range resp.Content {
		if b.Type == "text" {
			var critic struct {
				Pass      bool     `json:"pass"`
				Issues    []string `json:"issues"`
				Corrected string   `json:"corrected_output"`
			}
			if err := json.Unmarshal([]byte(b.Text), &critic); err != nil {
				result.Issues = append(result.Issues, "[critic] produced invalid JSON")
				result.Pass = false
				return result, nil
			}
			if !critic.Pass {
				result.Pass = false
				for _, issue := range critic.Issues {
					result.Issues = append(result.Issues, "[critic] "+issue)
				}
				if critic.Corrected != "" {
					result.CorrectedOutput = critic.Corrected
				}
			}
		}
	}

	return result, nil
}

// ValidationResult holds the validation outcome
type ValidationResult struct {
	Pass            bool     `json:"pass"`
	Issues          []string `json:"issues,omitempty"`
	Output          string   `json:"output"`
	CorrectedOutput string   `json:"corrected_output,omitempty"`
}

// CheckPII detects common PII patterns
func CheckPII(output string) (bool, string) {
	patterns := map[string]*regexp.Regexp{
		"SSN":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		"Credit Card": regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
		"API Key":     regexp.MustCompile(`\b(sk-|ak-|key-|AKIA)[A-Za-z0-9]{16,}\b`),
	}
	var found []string
	for name, pattern := range patterns {
		if pattern.MatchString(output) {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		return false, fmt.Sprintf("potential PII detected: %s", strings.Join(found, ", "))
	}
	return true, ""
}

// CheckLength validates output length
func CheckLength(maxLen int) func(string) (bool, string) {
	return func(output string) (bool, string) {
		if len(output) > maxLen {
			return false, fmt.Sprintf("output too long: %d chars (max %d)", len(output), maxLen)
		}
		return true, ""
	}
}

// CheckBlockedTerms rejects outputs containing blocked terms
func CheckBlockedTerms(terms []string) func(string) (bool, string) {
	return func(output string) (bool, string) {
		lower := strings.ToLower(output)
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				return false, fmt.Sprintf("blocked term found: %q", term)
			}
		}
		return true, ""
	}
}
