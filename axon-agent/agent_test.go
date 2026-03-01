package agent_test

import (
	"context"
	"testing"

	agent "github.com/benaskins/axon-agent"
)

func TestMessageFields(t *testing.T) {
	msg := agent.Message{
		Role:    "assistant",
		Content: "Hello!",
		Thinking: "Let me think...",
		ToolCalls: []agent.ToolCall{
			{Name: "web_search", Arguments: map[string]any{"query": "go"}},
		},
	}

	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want %q", msg.Role, "assistant")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Name != "web_search" {
		t.Errorf("ToolCall name = %q, want %q", msg.ToolCalls[0].Name, "web_search")
	}
	if msg.ToolCalls[0].Arguments["query"] != "go" {
		t.Errorf("ToolCall query = %v, want %q", msg.ToolCalls[0].Arguments["query"], "go")
	}
}

func TestChatRequestFields(t *testing.T) {
	req := agent.ChatRequest{
		Model: "llama3",
		Messages: []agent.Message{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
		Options: map[string]any{"temperature": 0.7},
	}

	if req.Model != "llama3" {
		t.Errorf("Model = %q, want %q", req.Model, "llama3")
	}
	if !req.Stream {
		t.Error("Stream should be true")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(req.Messages))
	}
}

func TestChatResponseFields(t *testing.T) {
	resp := agent.ChatResponse{
		Content:  "Here you go",
		Thinking: "Processing...",
		Done:     true,
		ToolCalls: []agent.ToolCall{
			{Name: "current_time", Arguments: map[string]any{}},
		},
	}

	if !resp.Done {
		t.Error("Done should be true")
	}
	if resp.Content != "Here you go" {
		t.Errorf("Content = %q, want %q", resp.Content, "Here you go")
	}
}

// stubClient implements ChatClient for testing.
type stubClient struct {
	responses []agent.ChatResponse
}

func (s *stubClient) Chat(ctx context.Context, req *agent.ChatRequest, fn func(agent.ChatResponse) error) error {
	for _, resp := range s.responses {
		if err := fn(resp); err != nil {
			return err
		}
	}
	return nil
}

func TestChatClientInterface(t *testing.T) {
	client := &stubClient{
		responses: []agent.ChatResponse{
			{Content: "Hello ", Done: false},
			{Content: "World!", Done: true},
		},
	}

	var c agent.ChatClient = client
	var collected string

	err := c.Chat(context.Background(), &agent.ChatRequest{
		Model:    "test",
		Messages: []agent.Message{{Role: "user", Content: "Hi"}},
	}, func(resp agent.ChatResponse) error {
		collected += resp.Content
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if collected != "Hello World!" {
		t.Errorf("got %q, want %q", collected, "Hello World!")
	}
}
