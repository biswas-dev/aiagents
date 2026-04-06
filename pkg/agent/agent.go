package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ToolFunc is the function signature for tool implementations
type ToolFunc func(input json.RawMessage) (string, error)

// Hook is called at various points in the agent lifecycle
type Hook func(event HookEvent)

// HookEvent carries information about an agent event
type HookEvent struct {
	Type       string
	Iteration  int
	ToolName   string
	ToolInput  json.RawMessage
	ToolOutput string
	ToolError  error
	Duration   time.Duration
	Response   *Response
}

// Agent is the core agent that runs the ReAct loop
type Agent struct {
	client  *Client
	tools   *ToolRegistry
	system  string
	maxIter int
	budget  *Budget
	hooks   []Hook
	logger  *slog.Logger
}

// AgentOption configures the agent
type AgentOption func(*Agent)

func WithMaxIterations(n int) AgentOption {
	return func(a *Agent) { a.maxIter = n }
}

func WithBudget(b *Budget) AgentOption {
	return func(a *Agent) { a.budget = b }
}

func WithHook(h Hook) AgentOption {
	return func(a *Agent) { a.hooks = append(a.hooks, h) }
}

func WithLogger(l *slog.Logger) AgentOption {
	return func(a *Agent) { a.logger = l }
}

// New creates a new Agent
func New(client *Client, tools *ToolRegistry, system string, opts ...AgentOption) *Agent {
	a := &Agent{
		client:  client,
		tools:   tools,
		system:  system,
		maxIter: 10,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run executes the agent loop with the given goal
func (a *Agent) Run(goal string) (string, error) {
	return a.RunWithMessages([]Message{{Role: "user", Content: MustJSON(goal)}})
}

// RunWithMessages executes the agent loop with pre-built messages
func (a *Agent) RunWithMessages(messages []Message) (string, error) {
	for i := 0; i < a.maxIter; i++ {
		// Check budget
		if a.budget != nil {
			if err := a.budget.Check(); err != nil {
				return "", fmt.Errorf("budget exceeded at iteration %d: %w", i, err)
			}
		}

		a.logger.Info("agent iteration", "iteration", i, "messages", len(messages))

		// Call the LLM
		iterStart := time.Now()
		resp, err := a.client.SendWithRetry(&MessagesRequest{
			MaxTokens: 4096,
			System:    a.system,
			Tools:     a.tools.Definitions(),
			Messages:  messages,
		}, 3)
		if err != nil {
			return "", fmt.Errorf("iteration %d: %w", i, err)
		}

		// Record budget usage
		if a.budget != nil {
			a.budget.Record(resp.Usage)
		}

		// Fire hook
		a.fireHook(HookEvent{
			Type:      "llm_response",
			Iteration: i,
			Response:  resp,
			Duration:  time.Since(iterStart),
		})

		// Add assistant response to history
		messages = append(messages, Message{Role: "assistant", Content: MustMarshal(resp.Content)})

		// Check if done
		if resp.StopReason == "end_turn" {
			for _, block := range resp.Content {
				if block.Type == "text" {
					a.logger.Info("agent completed", "iterations", i+1)
					return block.Text, nil
				}
			}
		}

		// Process tool calls
		var results []map[string]any
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}

			toolStart := time.Now()
			a.logger.Info("tool call", "tool", block.Name, "input", string(block.Input))

			output, toolErr := a.tools.Execute(block.Name, block.Input)
			toolDuration := time.Since(toolStart)

			if toolErr != nil {
				output = "Error: " + toolErr.Error()
				a.logger.Error("tool error", "tool", block.Name, "error", toolErr, "duration", toolDuration)
			} else {
				a.logger.Info("tool result", "tool", block.Name, "output_len", len(output), "duration", toolDuration)
			}

			a.fireHook(HookEvent{
				Type:       "tool_execution",
				Iteration:  i,
				ToolName:   block.Name,
				ToolInput:  block.Input,
				ToolOutput: output,
				ToolError:  toolErr,
				Duration:   toolDuration,
			})

			results = append(results, map[string]any{
				"type":        "tool_result",
				"tool_use_id": block.ID,
				"content":     output,
			})
		}

		if len(results) > 0 {
			messages = append(messages, Message{Role: "user", Content: MustMarshal(results)})
		}
	}

	return "", fmt.Errorf("max iterations (%d) reached", a.maxIter)
}

func (a *Agent) fireHook(e HookEvent) {
	for _, h := range a.hooks {
		h(e)
	}
}
