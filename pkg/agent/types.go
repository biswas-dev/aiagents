package agent

import "encoding/json"

// Message represents a message in the conversation
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ContentBlock is a single block in a response (text or tool_use)
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// Response from the LLM API
type Response struct {
	ID         string         `json:"id"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Model      string         `json:"model"`
	Usage      Usage          `json:"usage"`
}

// Usage tracks token consumption
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Tool defines a tool the agent can use
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MessagesRequest is the request body for the Claude Messages API
type MessagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	System      string    `json:"system,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	TopK        *int      `json:"top_k,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Ptr helpers for optional fields
func Ptr[T any](v T) *T { return &v }

// MustJSON marshals a string to JSON
func MustJSON(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// MustMarshal marshals any value to JSON
func MustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
