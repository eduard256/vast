package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/eduard256/vast/internal/api"
	"github.com/eduard256/vast/pkg/claude"
)

var (
	client   *claude.Client
	messages []Message
	mu       sync.Mutex
)

type Message struct {
	From string `json:"from"` // "person" or "ai"
	Text string `json:"text"`
	Time int64  `json:"time"` // unix ms
}

func Init() {
	client = &claude.Client{
		URL:      os.Getenv("CLAUDE_URL"),
		User:     os.Getenv("CLAUDE_USER"),
		Password: os.Getenv("CLAUDE_PASSWORD"),
		CWD:      os.Getenv("CLAUDE_CWD"),
	}

	// when AI sends a text message, save it to chat
	client.OnMessage(func(from, text string) {
		addMessage(from, text)
	})

	api.HandleFunc("api/chat/send", apiSend)
	api.HandleFunc("api/chat/messages", apiMessages)
	api.HandleFunc("api/chat/status", apiStatus)
	api.HandleFunc("api/chat/stop", apiStop)
	api.HandleFunc("api/chat/clear", apiClear)
}

func apiSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		api.Error(w, errors.New("text is empty"), http.StatusBadRequest)
		return
	}

	addMessage("person", req.Text)

	// start AI in background
	go func() {
		result, err := client.Send(req.Text)
		if err != nil {
			addMessage("ai", "Error: "+err.Error())
			return
		}
		// if result differs from last AI message, add it
		mu.Lock()
		lastText := ""
		if len(messages) > 0 {
			last := messages[len(messages)-1]
			if last.From == "ai" {
				lastText = last.Text
			}
		}
		mu.Unlock()

		if result != "" && result != lastText {
			addMessage("ai", result)
		}
	}()

	api.Response(w, map[string]string{"status": "ok"})
}

func apiMessages(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	if messages == nil {
		api.Response(w, []Message{})
		return
	}
	api.Response(w, messages)
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	working, toolsUsed := client.Status()
	api.Response(w, map[string]any{
		"working":    working,
		"tools_used": toolsUsed,
	})
}

func apiStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	if err := client.Stop(); err != nil {
		api.Error(w, err, http.StatusInternalServerError)
		return
	}
	api.Response(w, map[string]string{"status": "ok"})
}

func apiClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	messages = nil
	mu.Unlock()

	client.ClearSession()
	api.Response(w, map[string]string{"status": "ok"})
}

func addMessage(from, text string) {
	mu.Lock()
	messages = append(messages, Message{
		From: from,
		Text: text,
		Time: time.Now().UnixMilli(),
	})
	mu.Unlock()
}
