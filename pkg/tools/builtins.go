package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biswas-dev/aiagents/pkg/agent"
)

// RegisterFileTools adds file reading/writing tools
func RegisterFileTools(r *agent.ToolRegistry, basePath string) {
	r.Register("read_file", "Read the contents of a file. Returns the file content as a string.",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to read (relative to workspace)"}},"required":["path"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal(input, &p)
			fullPath := filepath.Join(basePath, filepath.Clean(p.Path))
			// Security: prevent path traversal
			if !strings.HasPrefix(fullPath, basePath) {
				return "", fmt.Errorf("path traversal detected: %s", p.Path)
			}
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", p.Path, err)
			}
			if len(data) > 50000 {
				return string(data[:50000]) + "\n... [truncated]", nil
			}
			return string(data), nil
		},
	)

	r.Register("write_file", "Write content to a file. Creates parent directories if needed.",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to write"},"content":{"type":"string","description":"Content to write"}},"required":["path","content"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			json.Unmarshal(input, &p)
			fullPath := filepath.Join(basePath, filepath.Clean(p.Path))
			if !strings.HasPrefix(fullPath, basePath) {
				return "", fmt.Errorf("path traversal detected: %s", p.Path)
			}
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err := os.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path), nil
		},
	)

	r.Register("list_files", "List files in a directory. Returns file names.",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Directory path (default: .)"},"pattern":{"type":"string","description":"Glob pattern to filter files (e.g. *.go)"}}}`),
		func(input json.RawMessage) (string, error) {
			var p struct {
				Path    string `json:"path"`
				Pattern string `json:"pattern"`
			}
			json.Unmarshal(input, &p)
			if p.Path == "" {
				p.Path = "."
			}
			fullPath := filepath.Join(basePath, filepath.Clean(p.Path))
			if !strings.HasPrefix(fullPath, basePath) {
				return "", fmt.Errorf("path traversal detected")
			}
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return "", err
			}
			var lines []string
			for _, e := range entries {
				name := e.Name()
				if p.Pattern != "" {
					matched, _ := filepath.Match(p.Pattern, name)
					if !matched {
						continue
					}
				}
				prefix := "  "
				if e.IsDir() {
					prefix = "d "
				}
				lines = append(lines, prefix+name)
			}
			return strings.Join(lines, "\n"), nil
		},
	)
}

// RegisterShellTool adds a shell command execution tool
func RegisterShellTool(r *agent.ToolRegistry, workDir string, allowedCommands []string) {
	r.Register("run_command", "Execute a shell command and return its output. Use for running tests, builds, or other CLI operations.",
		json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to execute"}},"required":["command"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct{ Command string `json:"command"` }
			json.Unmarshal(input, &p)

			// Basic command validation
			if len(allowedCommands) > 0 {
				allowed := false
				cmdBase := strings.Fields(p.Command)[0]
				for _, ac := range allowedCommands {
					if cmdBase == ac {
						allowed = true
						break
					}
				}
				if !allowed {
					return "", fmt.Errorf("command %q not in allowlist: %v", cmdBase, allowedCommands)
				}
			}

			cmd := exec.Command("sh", "-c", p.Command)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()
			result := string(output)
			if len(result) > 10000 {
				result = result[:10000] + "\n... [truncated]"
			}
			if err != nil {
				return result + "\nExit error: " + err.Error(), nil
			}
			return result, nil
		},
	)
}

// RegisterSearchTool adds a placeholder web search tool
func RegisterSearchTool(r *agent.ToolRegistry) {
	r.Register("search_web", "Search the web for current information on a topic. Returns relevant results.",
		json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query"}},"required":["query"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct{ Query string `json:"query"` }
			json.Unmarshal(input, &p)
			// TODO: Replace with real search API (SerpAPI, Tavily, Brave Search)
			return fmt.Sprintf("[Search placeholder] Results for: %s\nReplace RegisterSearchTool with a real search API implementation.", p.Query), nil
		},
	)
}

// RegisterCalculator adds a basic calculator tool
func RegisterCalculator(r *agent.ToolRegistry) {
	r.Register("calculator", "Evaluate a mathematical expression. Supports basic arithmetic.",
		json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string","description":"Math expression to evaluate (e.g. '2 + 3 * 4')"}},"required":["expression"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct{ Expression string `json:"expression"` }
			json.Unmarshal(input, &p)
			// Use bc for evaluation (safe, no code injection)
			cmd := exec.Command("bc", "-l")
			cmd.Stdin = strings.NewReader(p.Expression + "\n")
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("calculation error: %s", string(output))
			}
			return strings.TrimSpace(string(output)), nil
		},
	)
}
