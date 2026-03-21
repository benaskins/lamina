// Example: minimal GPT-style chat service using axon + axon-loop + axon-talk.
//
// This shows how to stitch together the axon modules into a working
// chat service with SSE streaming. It uses Ollama as the LLM backend
// via axon-talk, but you could swap in any provider that implements
// loop.LLMClient.
//
// Run:
//
//	go run . -model llama3.2
//
// Then open http://localhost:8080 in a browser, or curl:
//
//	curl -N -X POST http://localhost:8080/chat \
//	  -H "Content-Type: application/json" \
//	  -d '{"message": "Hello!"}'
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/benaskins/axon"
	"github.com/benaskins/axon/sse"
	loop "github.com/benaskins/axon-loop"
	"github.com/benaskins/axon-talk/ollama"
)

func main() {
	model := flag.String("model", "llama3.2", "Ollama model to use")
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	client, err := ollama.NewClientFromEnvironment()
	if err != nil {
		slog.Error("failed to create Ollama client", "error", err)
		return
	}

	mux := http.NewServeMux()
	mux.Handle("POST /chat", &chatHandler{
		client: client,
		model:  *model,
	})
	mux.HandleFunc("GET /", serveIndex)

	slog.Info("starting chat server", "port", *port, "model", *model)
	axon.ListenAndServe(*port, mux)
}

// chatHandler streams LLM responses as Server-Sent Events.
type chatHandler struct {
	client loop.LLMClient
	model  string

	// conversations holds per-session message history.
	// A real service would use a database — this is just an example.
	mu            sync.Mutex
	conversations map[string][]loop.Message
}

type chatRequest struct {
	Message string `json:"message"`
	Session string `json:"session"`
}

func (h *chatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Session == "" {
		req.Session = "default"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		axon.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Load conversation history
	h.mu.Lock()
	if h.conversations == nil {
		h.conversations = make(map[string][]loop.Message)
	}
	messages := h.conversations[req.Session]
	messages = append(messages, loop.Message{Role: "user", Content: req.Message})
	h.mu.Unlock()

	// Set up SSE streaming
	sse.SetSSEHeaders(w)

	// Run the conversation loop, streaming tokens as SSE events
	result, err := loop.Run(r.Context(), loop.RunConfig{
		Client: h.client,
		Request: &loop.Request{
			Model:    h.model,
			Messages: messages,
			Stream:   true,
		},
		Callbacks: loop.Callbacks{
			OnToken: func(token string) {
				sse.SendEvent(w, flusher, map[string]string{
					"type":  "token",
					"token": token,
				})
			},
		},
	})

	if err != nil {
		slog.Error("chat failed", "error", err)
		sse.SendEvent(w, flusher, map[string]string{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	// Save conversation history
	messages = append(messages, loop.Message{Role: "assistant", Content: result.Content})
	h.mu.Lock()
	h.conversations[req.Session] = messages
	h.mu.Unlock()

	sse.SendEvent(w, flusher, map[string]string{"type": "done"})
}

// serveIndex returns a minimal HTML page with a chat UI.
func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, indexHTML)
}

const indexHTML = `<!DOCTYPE html>
<html>
<head>
<title>chat</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui; max-width: 640px; margin: 0 auto; padding: 1rem; height: 100vh; display: flex; flex-direction: column; }
  #messages { flex: 1; overflow-y: auto; padding: 1rem 0; }
  .msg { margin-bottom: 1rem; line-height: 1.5; }
  .msg.user { color: #666; }
  .msg.assistant { color: #000; }
  form { display: flex; gap: 0.5rem; padding-top: 1rem; border-top: 1px solid #eee; }
  input { flex: 1; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px; font-size: 1rem; }
  button { padding: 0.5rem 1rem; border: none; background: #000; color: #fff; border-radius: 4px; cursor: pointer; }
</style>
</head>
<body>
<div id="messages"></div>
<form onsubmit="send(event)">
  <input id="input" placeholder="Say something..." autofocus>
  <button type="submit">Send</button>
</form>
<script>
const messages = document.getElementById('messages');
const input = document.getElementById('input');

function send(e) {
  e.preventDefault();
  const text = input.value.trim();
  if (!text) return;
  input.value = '';
  append('user', text);

  const div = append('assistant', '');
  fetch('/chat', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({message: text}),
  }).then(resp => {
    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    function read() {
      reader.read().then(({done, value}) => {
        if (done) return;
        buf += decoder.decode(value, {stream: true});
        const lines = buf.split('\n');
        buf = lines.pop();
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const data = JSON.parse(line.slice(6));
          if (data.type === 'token') div.textContent += data.token;
        }
        read();
      });
    }
    read();
  });
}

function append(role, text) {
  const div = document.createElement('div');
  div.className = 'msg ' + role;
  div.textContent = text;
  messages.appendChild(div);
  messages.scrollTop = messages.scrollHeight;
  return div;
}
</script>
</body>
</html>`
