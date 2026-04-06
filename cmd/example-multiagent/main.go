package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/biswas-dev/aiagents/pkg/agent"
	"github.com/biswas-dev/aiagents/pkg/tools"
)

func main() {
	client := agent.NewAnthropicClient(agent.WithModel("claude-sonnet-4-6"))

	goal := "Analyze the current directory's Go code: research best practices, analyze the code quality, and write a summary report."
	if len(os.Args) > 1 {
		goal = strings.Join(os.Args[1:], " ")
	}

	fmt.Fprintf(os.Stderr, "Goal: %s\n", goal)
	fmt.Fprintf(os.Stderr, "Planning...\n\n")

	// Step 1: Plan
	planResp, err := client.Send(&agent.MessagesRequest{
		MaxTokens:   1024,
		Temperature: agent.Ptr(0.0),
		System: `You are a task planner. Break the goal into 2-4 ordered subtasks.
Each subtask should be assigned to one of: researcher, analyst, writer.
Output ONLY a JSON array: [{"id":"t1","agent":"researcher","instruction":"..."}]`,
		Messages: []agent.Message{{Role: "user", Content: agent.MustJSON("Goal: " + goal)}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Planning error: %v\n", err)
		os.Exit(1)
	}

	type Task struct {
		ID          string `json:"id"`
		Agent       string `json:"agent"`
		Instruction string `json:"instruction"`
	}
	var tasks []Task
	for _, b := range planResp.Content {
		if b.Type == "text" {
			json.Unmarshal([]byte(b.Text), &tasks)
		}
	}

	fmt.Fprintf(os.Stderr, "Plan: %d subtasks\n", len(tasks))
	for _, t := range tasks {
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", t.ID, t.Agent, t.Instruction[:min(60, len(t.Instruction))])
	}
	fmt.Fprintln(os.Stderr)

	// Step 2: Execute each subtask
	cwd, _ := os.Getwd()
	results := map[string]string{}

	for _, task := range tasks {
		registry := agent.NewToolRegistry()
		var system string

		switch task.Agent {
		case "researcher":
			tools.RegisterSearchTool(registry)
			tools.RegisterFileTools(registry, cwd)
			system = "You are a research agent. Use tools to gather information. Be thorough and cite sources."
		case "analyst":
			tools.RegisterFileTools(registry, cwd)
			tools.RegisterShellTool(registry, cwd, []string{"go", "wc", "grep", "find", "git"})
			system = "You are a code analysis agent. Read code, run analysis tools, and report findings with specifics."
		case "writer":
			system = "You are a technical writer. Synthesize information into clear, well-structured reports."
		default:
			tools.RegisterFileTools(registry, cwd)
			system = "You are a helpful assistant."
		}

		a := agent.New(client, registry, system,
			agent.WithMaxIterations(8),
			agent.WithBudget(agent.NewBudget(8, 30000, 0.30)),
		)

		// Include context from previous tasks
		instruction := task.Instruction
		for prevID, prevResult := range results {
			instruction += fmt.Sprintf("\n\nContext from %s:\n%s", prevID, prevResult)
		}

		fmt.Fprintf(os.Stderr, "Running %s (%s)...\n", task.ID, task.Agent)
		result, err := a.Run(instruction)
		if err != nil {
			results[task.ID] = "Error: " + err.Error()
		} else {
			results[task.ID] = result
		}
	}

	// Step 3: Synthesize
	fmt.Fprintf(os.Stderr, "\nSynthesizing results...\n")
	var ctx strings.Builder
	for id, result := range results {
		fmt.Fprintf(&ctx, "=== %s ===\n%s\n\n", id, result)
	}

	synthResp, err := client.Send(&agent.MessagesRequest{
		MaxTokens: 4096,
		System:    "Synthesize these agent results into a cohesive final report. Keep it structured and actionable.",
		Messages:  []agent.Message{{Role: "user", Content: agent.MustJSON(fmt.Sprintf("Goal: %s\n\nResults:\n%s", goal, ctx.String()))}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Synthesis error: %v\n", err)
		os.Exit(1)
	}
	for _, b := range synthResp.Content {
		if b.Type == "text" {
			fmt.Println(b.Text)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
