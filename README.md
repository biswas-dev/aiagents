# aiagents

A production-grade AI agent framework in Go. Build autonomous agents that reason, use tools, and iterate toward goals — with built-in budget controls, knowledge graphs, guardrails, and multi-agent orchestration.

This is the companion code for [The Complete Engineer's Guide to AI Agents](https://anshumanbiswas.com/guides/ai-agents-complete-guide).

## Features

- **Provider-agnostic** — works with Claude (Anthropic) and GPT (OpenAI) via unified API
- **ReAct loop** — the core Thought → Action → Observation agent loop
- **Tool registry** — type-safe tool registration with built-in file, shell, search, and calculator tools
- **Budget controls** — hard limits on iterations, tokens, and cost
- **Knowledge graphs** — in-memory graph with entity/relationship queries, import/export
- **Guardrails** — PII detection, output validation, LLM-as-critic
- **Evaluation framework** — test suites, LLM-as-judge, self-consistency sampling
- **Multi-agent orchestration** — planner → worker → synthesizer pattern
- **Hooks** — lifecycle callbacks for logging, monitoring, and custom behavior
- **Retry with backoff** — automatic retries for transient API errors

## Quick Start

```bash
# Clone
git clone https://github.com/biswas-dev/aiagents.git
cd aiagents

# Set your API key
export ANTHROPIC_API_KEY="sk-ant-..."
# OR for OpenAI:
# export OPENAI_API_KEY="sk-..."
# export AI_PROVIDER="openai"

# Run the CLI agent
go run ./cmd/agent "List all Go files in this project and summarize what each package does"
```

## Installation

```bash
go get github.com/biswas-dev/aiagents
```

## Usage

### Basic Agent

```go
package main

import (
    "fmt"
    "github.com/biswas-dev/aiagents/pkg/agent"
    "github.com/biswas-dev/aiagents/pkg/tools"
)

func main() {
    client := agent.NewAnthropicClient()
    registry := agent.NewToolRegistry()
    tools.RegisterSearchTool(registry)

    a := agent.New(client, registry,
        "You are a research assistant. Use tools to find information.",
        agent.WithMaxIterations(10),
        agent.WithBudget(agent.NewBudget(10, 50000, 0.50)),
    )

    result, err := a.Run("What are the top AI agent frameworks in 2025?")
    if err != nil {
        panic(err)
    }
    fmt.Println(result)
}
```

### Custom Tools

```go
registry := agent.NewToolRegistry()

registry.Register("get_weather",
    "Get current weather for a city",
    json.RawMessage(`{
        "type": "object",
        "properties": {
            "city": {"type": "string", "description": "City name"}
        },
        "required": ["city"]
    }`),
    func(input json.RawMessage) (string, error) {
        var p struct{ City string `json:"city"` }
        json.Unmarshal(input, &p)
        // Call your weather API here
        return fmt.Sprintf("72°F and sunny in %s", p.City), nil
    },
)
```

### Knowledge Graph

```go
import "github.com/biswas-dev/aiagents/pkg/knowledge"

kg := knowledge.NewGraph()

// Add entities
kg.AddEntity(&knowledge.Entity{
    ID: "auth-service", Type: "service",
    Properties: map[string]string{"language": "Go", "slo": "99.9%"},
})
kg.AddEntity(&knowledge.Entity{
    ID: "user-db", Type: "database",
    Properties: map[string]string{"engine": "PostgreSQL"},
})

// Add relationships
kg.AddRelationship(knowledge.Relationship{
    SourceID: "auth-service", TargetID: "user-db", Relation: "depends_on",
})

// Register as agent tools
kg.RegisterTools(registry)

// Or query directly
deps := kg.Neighbors("auth-service", "depends_on")

// Import/export JSON
data, _ := kg.Export()
kg.Import(data)
```

### Guardrails

```go
import "github.com/biswas-dev/aiagents/pkg/guardrails"

validator := guardrails.NewValidator(client)
validator.AddRule(guardrails.Rule{
    Name: "no_sql_injection",
    Check: guardrails.CheckBlockedTerms([]string{"DROP TABLE", "DELETE FROM", "'; --"}),
})

result, _ := validator.Validate(agentOutput, "summarize quarterly report")
if !result.Pass {
    fmt.Println("Issues:", result.Issues)
}

// Or with LLM critic (more thorough, costs tokens)
result, _ = validator.ValidateWithCritic(agentOutput, "summarize quarterly report")
```

### Multi-Agent Orchestration

```go
// See cmd/example-multiagent/main.go for a complete example
go run ./cmd/example-multiagent "Analyze this codebase and write a technical report"
```

### Evaluation

```go
import "github.com/biswas-dev/aiagents/pkg/eval"

suite := &eval.Suite{
    Cases: []eval.TestCase{
        {
            Name:           "basic_math",
            Goal:           "What is 15 * 23?",
            ExpectedAnswer: "345",
        },
        {
            Name: "file_reading",
            Goal: "Read the go.mod file and tell me the module name",
            ExpectedAnswer: "github.com/biswas-dev/aiagents",
        },
    },
}

results := suite.Run(myAgent)
```

## Project Structure

```
aiagents/
├── cmd/
│   ├── agent/              # CLI agent
│   ├── example-research/   # Research agent example
│   ├── example-codereview/ # Code review agent example
│   └── example-multiagent/ # Multi-agent orchestration example
├── pkg/
│   ├── agent/              # Core: client, agent loop, tools, budget, types
│   ├── tools/              # Built-in tools: file, shell, search, calculator
│   ├── knowledge/          # In-memory knowledge graph
│   ├── guardrails/         # Output validation, PII detection, LLM critic
│   └── eval/               # Evaluation framework, LLM-as-judge
├── examples/
│   └── knowledge_graph.json # Example KG data
├── go.mod
├── LICENSE                  # Apache 2.0
└── README.md
```

## Examples

```bash
# General-purpose agent
go run ./cmd/agent "Find all TODO comments in this codebase"

# Research agent
go run ./cmd/example-research "AI agent security best practices"

# Code review agent
go run ./cmd/example-codereview ./pkg/agent

# Multi-agent orchestration
go run ./cmd/example-multiagent "Research Go testing best practices, analyze our test coverage, and write recommendations"
```

## OpenAI Support

Set the provider via environment variable:

```bash
export AI_PROVIDER=openai
export OPENAI_API_KEY=sk-...
go run ./cmd/agent "Hello, what model are you?"
```

The framework automatically converts between Anthropic and OpenAI API formats.

## Runtime Security

For production deployments, pair this framework with [AI Agent Lens](https://aiagentlens.com) (AgentShield) to add runtime security — a 7-layer analysis pipeline that intercepts every shell command and MCP tool call before execution.

## Learn More

- [The Complete Engineer's Guide to AI Agents](https://anshumanbiswas.com/guides/ai-agents-complete-guide) — the full guide with architecture diagrams
- [The Complete Engineer's Guide to AI Agents (Blog)](https://anshumanbiswas.com/blog/the-complete-engineers-guide-to-ai-agents) — condensed blog version
- [AI Agent Lens](https://aiagentlens.com) — runtime security for AI agents

## License

Apache 2.0 — see [LICENSE](LICENSE).
