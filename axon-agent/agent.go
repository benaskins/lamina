// Package agent provides a provider-agnostic conversation loop for
// LLM-powered agents. It handles message exchange, tool call dispatch,
// and streaming — with no HTTP, persistence, or UI concerns.
package agent

import (
	"context"

	tool "github.com/benaskins/axon-tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role      string
	Content   string
	Thinking  string
	ToolCalls []ToolCall
}

// ToolCall represents an LLM's decision to invoke a tool.
type ToolCall struct {
	Name      string
	Arguments map[string]any
}

// ChatRequest is a provider-agnostic request to an LLM.
type ChatRequest struct {
	Model         string
	Messages      []Message
	Tools         []tool.ToolDef
	Stream        bool
	Think         *bool
	Options       map[string]any
	MaxIterations int // Maximum tool-call loop iterations. Defaults to 20 if 0.
}

// ChatResponse is a provider-agnostic streamed response chunk from an LLM.
type ChatResponse struct {
	Content   string
	Thinking  string
	Done      bool
	ToolCalls []ToolCall
}

// ChatClient abstracts communication with an LLM backend.
// Implementations translate to/from provider-specific APIs
// (e.g. Ollama, OpenAI, Anthropic).
type ChatClient interface {
	Chat(ctx context.Context, req *ChatRequest, fn func(ChatResponse) error) error
}
