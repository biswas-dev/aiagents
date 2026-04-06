package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/biswas-dev/aiagents/pkg/agent"
	"github.com/biswas-dev/aiagents/pkg/tools"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <goal>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 'List all Go files in the current directory'\n", os.Args[0])
		os.Exit(1)
	}

	goal := strings.Join(os.Args[1:], " ")
	provider := os.Getenv("AI_PROVIDER") // "openai" or "anthropic" (default)

	var client *agent.Client
	if provider == "openai" {
		client = agent.NewOpenAIClient()
		slog.Info("using OpenAI", "model", client.Model())
	} else {
		client = agent.NewAnthropicClient()
		slog.Info("using Anthropic", "model", client.Model())
	}

	// Set up tools
	registry := agent.NewToolRegistry()
	cwd, _ := os.Getwd()
	tools.RegisterFileTools(registry, cwd)
	tools.RegisterShellTool(registry, cwd, nil) // nil = all commands allowed
	tools.RegisterSearchTool(registry)
	tools.RegisterCalculator(registry)

	// Create agent with budget
	budget := agent.NewBudget(15, 100000, 1.00)
	a := agent.New(client, registry,
		`You are a helpful assistant with access to file system, shell, search, and calculator tools.
Use tools to accomplish the user's goal. Be thorough but efficient.
Always verify your work before giving a final answer.`,
		agent.WithMaxIterations(15),
		agent.WithBudget(budget),
		agent.WithHook(func(e agent.HookEvent) {
			if e.Type == "tool_execution" {
				status := "ok"
				if e.ToolError != nil {
					status = "error"
				}
				fmt.Fprintf(os.Stderr, "  → %s [%s] (%v)\n", e.ToolName, status, e.Duration)
			}
		}),
	)

	fmt.Fprintf(os.Stderr, "Goal: %s\n\n", goal)
	result, err := a.Run(goal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Budget: %s\n", budget.Summary())
		os.Exit(1)
	}

	fmt.Println(result)
	fmt.Fprintf(os.Stderr, "\nBudget: %s\n", budget.Summary())
}
