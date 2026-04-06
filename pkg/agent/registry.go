package agent

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ToolRegistry manages tool definitions and handlers
type ToolRegistry struct {
	definitions []Tool
	handlers    map[string]ToolFunc
	mu          sync.RWMutex
}

// NewToolRegistry creates a new ToolRegistry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{handlers: make(map[string]ToolFunc)}
}

// Register adds a tool to the registry
func (tr *ToolRegistry) Register(name, description string, schema json.RawMessage, fn ToolFunc) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.definitions = append(tr.definitions, Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
	})
	tr.handlers[name] = fn
}

// Execute runs a tool by name
func (tr *ToolRegistry) Execute(name string, input json.RawMessage) (string, error) {
	tr.mu.RLock()
	fn, ok := tr.handlers[name]
	tr.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return fn(input)
}

// Definitions returns all registered tool definitions
func (tr *ToolRegistry) Definitions() []Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	defs := make([]Tool, len(tr.definitions))
	copy(defs, tr.definitions)
	return defs
}

// Has returns true if a tool is registered
func (tr *ToolRegistry) Has(name string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	_, ok := tr.handlers[name]
	return ok
}

// Names returns all registered tool names
func (tr *ToolRegistry) Names() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	names := make([]string, 0, len(tr.handlers))
	for name := range tr.handlers {
		names = append(names, name)
	}
	return names
}

// Wrap wraps a tool function with middleware
func Wrap(fn ToolFunc, middleware ...func(ToolFunc) ToolFunc) ToolFunc {
	for i := len(middleware) - 1; i >= 0; i-- {
		fn = middleware[i](fn)
	}
	return fn
}

// ReadOnly wraps a tool to reject mutating operations
func ReadOnly(fn ToolFunc) ToolFunc {
	return func(input json.RawMessage) (string, error) {
		var raw map[string]any
		json.Unmarshal(input, &raw)
		if op, ok := raw["operation"]; ok {
			switch op {
			case "delete", "update", "insert", "drop", "truncate", "alter":
				return "", fmt.Errorf("write operation %q not allowed in read-only mode", op)
			}
		}
		return fn(input)
	}
}

// WithMaxOutputLen wraps a tool to truncate output exceeding maxLen
func WithMaxOutputLen(maxLen int) func(ToolFunc) ToolFunc {
	return func(fn ToolFunc) ToolFunc {
		return func(input json.RawMessage) (string, error) {
			output, err := fn(input)
			if len(output) > maxLen {
				output = output[:maxLen] + fmt.Sprintf("\n... [truncated, %d chars total]", len(output))
			}
			return output, err
		}
	}
}
