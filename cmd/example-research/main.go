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
	tools.RegisterSearchTool(registry)
	tools.RegisterCalculator(registry)

	a := agent.New(client, registry,
		`You are a research assistant. Use the search tool to gather information
before answering. Always cite your sources. Be thorough and accurate.`,
		agent.WithMaxIterations(10),
		agent.WithBudget(agent.NewBudget(10, 50000, 0.50)),
	)

	topic := "latest developments in AI agent frameworks in 2025"
	if len(os.Args) > 1 {
		topic = os.Args[1]
	}

	fmt.Fprintf(os.Stderr, "Researching: %s\n\n", topic)
	result, err := a.Run(fmt.Sprintf("Research the following topic and give me a concise summary with the top 3 key findings: %s", topic))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}
