package main

import (
	"fmt"
	"os"

	"github.com/biswas-dev/aiagents/pkg/agent"
	"github.com/biswas-dev/aiagents/pkg/tools"
)

func main() {
	client := agent.NewAnthropicClient(agent.WithModel("claude-sonnet-4-6"))

	registry := agent.NewToolRegistry()
	cwd, _ := os.Getwd()
	tools.RegisterFileTools(registry, cwd)
	tools.RegisterShellTool(registry, cwd, []string{"go", "git", "grep", "wc", "find"})

	a := agent.New(client, registry,
		`You are a senior code reviewer. When reviewing code:
1. Read the files completely
2. Check for: bugs, security issues, performance problems, error handling gaps
3. Run 'go vet' and check for issues
4. Provide specific, actionable feedback with file paths and line numbers
Never approve code that has security vulnerabilities.
Format your review as a structured list of findings.`,
		agent.WithMaxIterations(15),
		agent.WithBudget(agent.NewBudget(15, 80000, 0.75)),
	)

	target := "."
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	fmt.Fprintf(os.Stderr, "Reviewing code in: %s\n\n", target)
	result, err := a.Run(fmt.Sprintf("Review the Go code in %s. Focus on bugs, security issues, and code quality. List your findings.", target))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}
