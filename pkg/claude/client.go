package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Client struct {
	URL      string // http://host:port
	User     string
	Password string
	CWD      string // working directory on remote server

	mu        sync.Mutex
	processID string
	sessionID string
	working   bool
	toolsUsed int
	onMessage func(from, text string)
}

type ChatRequest struct {
	Prompt             string `json:"prompt"`
	CWD                string `json:"cwd"`
	Model              string `json:"model,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`
}

// Send starts a new AI request in background. Blocks until AI finishes.
// Returns final result text and error.
func (c *Client) Send(prompt string) (string, error) {
	c.mu.Lock()
	if c.working {
		c.mu.Unlock()
		return "", fmt.Errorf("claude: already working")
	}
	c.working = true
	c.toolsUsed = 0
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.working = false
		c.processID = ""
		c.mu.Unlock()
	}()

	req := ChatRequest{
		Prompt:    prompt,
		CWD:       c.CWD,
		Model:     "sonnet",
		SessionID: c.sessionID,
	}

	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", c.URL+"/chat", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	httpReq.SetBasicAuth(c.User, c.Password)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude: status %d", resp.StatusCode)
	}

	c.mu.Lock()
	c.processID = resp.Header.Get("X-Process-ID")
	c.mu.Unlock()

	return c.parseSSE(resp)
}

// Stop cancels the current AI request
func (c *Client) Stop() error {
	c.mu.Lock()
	pid := c.processID
	c.mu.Unlock()

	if pid == "" {
		return nil
	}

	httpReq, err := http.NewRequest("DELETE", c.URL+"/chat/"+pid, nil)
	if err != nil {
		return err
	}
	httpReq.SetBasicAuth(c.User, c.Password)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ClearSession resets the conversation
func (c *Client) ClearSession() {
	c.mu.Lock()
	c.sessionID = ""
	c.mu.Unlock()
}

// Status returns current AI state
func (c *Client) Status() (working bool, toolsUsed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.working, c.toolsUsed
}

// OnMessage sets callback for AI text messages
func (c *Client) OnMessage(f func(from, text string)) {
	c.onMessage = f
}

func (c *Client) parseSSE(resp *http.Response) (string, error) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var result string

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(line[5:])
		var msg map[string]any
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue
		}

		switch msg["type"] {
		case "system":
			if sid, ok := msg["session_id"].(string); ok {
				c.mu.Lock()
				c.sessionID = sid
				c.mu.Unlock()
			}

		case "assistant":
			if message, ok := msg["message"].(map[string]any); ok {
				if content, ok := message["content"].([]any); ok {
					for _, item := range content {
						block, ok := item.(map[string]any)
						if !ok {
							continue
						}
						switch block["type"] {
						case "text":
							if text, ok := block["text"].(string); ok && text != "" && c.onMessage != nil {
								c.onMessage("ai", text)
							}
						case "tool_use":
							c.mu.Lock()
							c.toolsUsed++
							c.mu.Unlock()

							if c.onMessage != nil {
								name, _ := block["name"].(string)
								input, _ := block["input"].(map[string]any)
								c.onMessage("tool", toolSummary(name, input))
							}
						}
					}
				}
			}

		case "result":
			if r, ok := msg["result"].(string); ok {
				result = r
			}
		}
	}

	return result, scanner.Err()
}

func toolSummary(name string, input map[string]any) string {
	// pick the most meaningful field from input
	for _, key := range []string{"command", "query", "pattern", "file_path", "url", "prompt", "description", "text"} {
		if v, ok := input[key].(string); ok && v != "" {
			if len(v) > 100 {
				v = v[:100] + "..."
			}
			return name + ": " + v
		}
	}
	return name
}
