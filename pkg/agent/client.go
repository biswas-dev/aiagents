package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider represents an LLM provider
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// Client handles communication with LLM APIs
type Client struct {
	provider   Provider
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures the client
type ClientOption func(*Client)

func WithModel(model string) ClientOption {
	return func(c *Client) { c.model = model }
}

func WithAPIKey(key string) ClientOption {
	return func(c *Client) { c.apiKey = key }
}

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// NewAnthropicClient creates a client for the Claude API
func NewAnthropicClient(opts ...ClientOption) *Client {
	c := &Client{
		provider:   ProviderAnthropic,
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		model:      "claude-sonnet-4-6",
		baseURL:    "https://api.anthropic.com",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewOpenAIClient creates a client for the OpenAI API
func NewOpenAIClient(opts ...ClientOption) *Client {
	c := &Client{
		provider:   ProviderOpenAI,
		apiKey:     os.Getenv("OPENAI_API_KEY"),
		model:      "gpt-4o",
		baseURL:    "https://api.openai.com",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Model returns the client's model name
func (c *Client) Model() string { return c.model }

// Provider returns the client's provider
func (c *Client) ProviderName() Provider { return c.provider }

// Send sends a request to the Claude Messages API
func (c *Client) Send(req *MessagesRequest) (*Response, error) {
	if c.provider == ProviderOpenAI {
		return c.sendOpenAI(req)
	}
	return c.sendAnthropic(req)
}

func (c *Client) sendAnthropic(req *MessagesRequest) (*Response, error) {
	req.Model = c.model
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	var result Response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) sendOpenAI(req *MessagesRequest) (*Response, error) {
	// Convert Anthropic-style request to OpenAI format
	oaiMessages := make([]map[string]any, 0, len(req.Messages)+1)
	if req.System != "" {
		oaiMessages = append(oaiMessages, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		var content any
		if err := json.Unmarshal(m.Content, &content); err == nil {
			// If it's a string, use it directly
			if s, ok := content.(string); ok {
				oaiMessages = append(oaiMessages, map[string]any{"role": m.Role, "content": s})
				continue
			}
		}
		oaiMessages = append(oaiMessages, map[string]any{"role": m.Role, "content": string(m.Content)})
	}

	// Convert tools
	var oaiTools []map[string]any
	for _, t := range req.Tools {
		oaiTools = append(oaiTools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  json.RawMessage(t.InputSchema),
			},
		})
	}

	oaiReq := map[string]any{
		"model":    c.model,
		"messages": oaiMessages,
	}
	if len(oaiTools) > 0 {
		oaiReq["tools"] = oaiTools
	}
	if req.Temperature != nil {
		oaiReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		oaiReq["max_tokens"] = req.MaxTokens
	}

	body, _ := json.Marshal(oaiReq)
	httpReq, _ := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	// Parse OpenAI response into our unified Response format
	var oaiResp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(data, &oaiResp)

	result := &Response{
		ID:    oaiResp.ID,
		Usage: Usage{InputTokens: oaiResp.Usage.PromptTokens, OutputTokens: oaiResp.Usage.CompletionTokens},
	}

	if len(oaiResp.Choices) > 0 {
		choice := oaiResp.Choices[0]
		if choice.Message.Content != "" {
			result.Content = append(result.Content, ContentBlock{Type: "text", Text: choice.Message.Content})
		}
		for _, tc := range choice.Message.ToolCalls {
			result.Content = append(result.Content, ContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
		if choice.FinishReason == "tool_calls" {
			result.StopReason = "tool_use"
		} else {
			result.StopReason = "end_turn"
		}
	}
	return result, nil
}

// SendWithRetry sends a request with exponential backoff retry
func (c *Client) SendWithRetry(req *MessagesRequest, maxRetries int) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.Send(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		errStr := err.Error()
		retryable := strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "500") ||
			strings.Contains(errStr, "502") ||
			strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "529")
		if !retryable {
			return nil, err
		}

		backoff := time.Duration(1<<attempt) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		time.Sleep(backoff)
	}
	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}
