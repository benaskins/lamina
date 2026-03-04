package agent_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	agent "github.com/benaskins/axon-agent"
	tool "github.com/benaskins/axon-tool"
)

func TestRunSimpleChat(t *testing.T) {
	client := &stubClient{
		responses: []agent.ChatResponse{
			{Content: "Hello there!", Done: true},
		},
	}

	var tokens []string
	var doneCalled bool

	result, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test-model",
		Messages: []agent.Message{{Role: "user", Content: "Hi"}},
	}, nil, nil, agent.Callbacks{
		OnToken: func(token string) {
			tokens = append(tokens, token)
		},
		OnDone: func(durationMs int64) {
			doneCalled = true
		},
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Hello there!" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello there!")
	}
	if !doneCalled {
		t.Error("OnDone was not called")
	}
	if len(tokens) == 0 {
		t.Error("expected at least one OnToken call")
	}
}

func TestRunWithToolCall(t *testing.T) {
	callCount := 0
	client := &multiTurnClient{
		turns: [][]agent.ChatResponse{
			// First turn: model calls a tool
			{
				{
					Content: "",
					ToolCalls: []agent.ToolCall{
						{Name: "current_time", Arguments: map[string]any{}},
					},
					Done: true,
				},
			},
			// Second turn: model responds with final answer
			{
				{Content: "It is 3pm.", Done: true},
			},
		},
	}

	tools := map[string]tool.ToolDef{
		"current_time": {
			Name: "current_time",
			Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
				callCount++
				return tool.ToolResult{Content: "Current time: Monday, 3:00 PM"}
			},
		},
	}

	var toolUses []string
	result, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test-model",
		Messages: []agent.Message{{Role: "user", Content: "What time is it?"}},
	}, tools, &tool.ToolContext{Ctx: context.Background()}, agent.Callbacks{
		OnToolUse: func(name string, args map[string]any) {
			toolUses = append(toolUses, name)
		},
	})

	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Errorf("tool executed %d times, want 1", callCount)
	}
	if len(toolUses) != 1 || toolUses[0] != "current_time" {
		t.Errorf("OnToolUse calls = %v, want [current_time]", toolUses)
	}
	if result.Content != "It is 3pm." {
		t.Errorf("Content = %q, want %q", result.Content, "It is 3pm.")
	}
}

func TestRunNoTools(t *testing.T) {
	client := &stubClient{
		responses: []agent.ChatResponse{
			{Content: "Just chatting.", Done: true},
		},
	}

	result, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test-model",
		Messages: []agent.Message{{Role: "user", Content: "Hello"}},
	}, nil, nil, agent.Callbacks{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Just chatting." {
		t.Errorf("Content = %q, want %q", result.Content, "Just chatting.")
	}
}

func TestRunWithThinking(t *testing.T) {
	client := &stubClient{
		responses: []agent.ChatResponse{
			{Thinking: "Let me consider...", Done: false},
			{Content: "Here's my answer.", Done: true},
		},
	}

	var thinkingTokens []string
	result, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test-model",
		Messages: []agent.Message{{Role: "user", Content: "Think about this"}},
	}, nil, nil, agent.Callbacks{
		OnThinking: func(token string) {
			thinkingTokens = append(thinkingTokens, token)
		},
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Thinking != "Let me consider..." {
		t.Errorf("Thinking = %q, want %q", result.Thinking, "Let me consider...")
	}
	if len(thinkingTokens) == 0 {
		t.Error("expected OnThinking callback")
	}
}

func TestRunPassesToolsToClient(t *testing.T) {
	var receivedTools []tool.ToolDef
	client := &spyClient{
		onChat: func(req *agent.ChatRequest) {
			receivedTools = req.Tools
		},
		responses: []agent.ChatResponse{
			{Content: "ok", Done: true},
		},
	}

	tools := map[string]tool.ToolDef{
		"current_time": tool.CurrentTimeTool(),
	}

	_, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test",
		Messages: []agent.Message{{Role: "user", Content: "time?"}},
	}, tools, &tool.ToolContext{Ctx: context.Background()}, agent.Callbacks{})

	if err != nil {
		t.Fatal(err)
	}
	if len(receivedTools) != 1 {
		t.Fatalf("expected 1 tool in request, got %d", len(receivedTools))
	}
	if receivedTools[0].Name != "current_time" {
		t.Errorf("tool name = %q, want %q", receivedTools[0].Name, "current_time")
	}
}

// spyClient records the ChatRequest for inspection.
type spyClient struct {
	onChat    func(req *agent.ChatRequest)
	responses []agent.ChatResponse
}

func (s *spyClient) Chat(ctx context.Context, req *agent.ChatRequest, fn func(agent.ChatResponse) error) error {
	if s.onChat != nil {
		s.onChat(req)
	}
	for _, resp := range s.responses {
		if err := fn(resp); err != nil {
			return err
		}
	}
	return nil
}

func TestRunMaxIterationsExceeded(t *testing.T) {
	// Client that always returns a tool call, forcing infinite loop
	client := &alwaysToolCallClient{}

	tools := map[string]tool.ToolDef{
		"noop": {
			Name: "noop",
			Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
				return tool.ToolResult{Content: "ok"}
			},
		},
	}

	_, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:         "test",
		Messages:      []agent.Message{{Role: "user", Content: "loop"}},
		MaxIterations: 3,
	}, tools, &tool.ToolContext{Ctx: context.Background()}, agent.Callbacks{})

	if err == nil {
		t.Fatal("expected error for max iterations exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "max iterations") {
		t.Errorf("error = %q, want it to contain 'max iterations'", err.Error())
	}
}

func TestRunUnknownToolCall(t *testing.T) {
	client := &multiTurnClient{
		turns: [][]agent.ChatResponse{
			// First turn: model calls a tool that doesn't exist
			{
				{
					ToolCalls: []agent.ToolCall{
						{Name: "nonexistent_tool", Arguments: map[string]any{}},
					},
					Done: true,
				},
			},
			// Second turn: model responds with final answer
			{
				{Content: "Sorry about that.", Done: true},
			},
		},
	}

	tools := map[string]tool.ToolDef{
		"real_tool": {
			Name: "real_tool",
			Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
				return tool.ToolResult{Content: "ok"}
			},
		},
	}

	result, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test",
		Messages: []agent.Message{{Role: "user", Content: "call something"}},
	}, tools, &tool.ToolContext{Ctx: context.Background()}, agent.Callbacks{})

	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Sorry about that." {
		t.Errorf("Content = %q, want %q", result.Content, "Sorry about that.")
	}
}

func TestRunChatClientError(t *testing.T) {
	client := &errorClient{err: fmt.Errorf("connection refused")}

	_, err := agent.Run(context.Background(), client, &agent.ChatRequest{
		Model:    "test",
		Messages: []agent.Message{{Role: "user", Content: "Hi"}},
	}, nil, nil, agent.Callbacks{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want it to contain 'connection refused'", err.Error())
	}
}

// alwaysToolCallClient always returns a tool call on every Chat invocation.
type alwaysToolCallClient struct{}

func (a *alwaysToolCallClient) Chat(ctx context.Context, req *agent.ChatRequest, fn func(agent.ChatResponse) error) error {
	return fn(agent.ChatResponse{
		ToolCalls: []agent.ToolCall{
			{Name: "noop", Arguments: map[string]any{}},
		},
		Done: true,
	})
}

// errorClient always returns an error.
type errorClient struct {
	err error
}

func (e *errorClient) Chat(ctx context.Context, req *agent.ChatRequest, fn func(agent.ChatResponse) error) error {
	return e.err
}

// multiTurnClient simulates a client that returns different responses on each call.
type multiTurnClient struct {
	turns [][]agent.ChatResponse
	call  int
}

func (m *multiTurnClient) Chat(ctx context.Context, req *agent.ChatRequest, fn func(agent.ChatResponse) error) error {
	if m.call >= len(m.turns) {
		return nil
	}
	responses := m.turns[m.call]
	m.call++
	for _, resp := range responses {
		if err := fn(resp); err != nil {
			return err
		}
	}
	return nil
}
