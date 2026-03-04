package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	tool "github.com/benaskins/axon-tool"
)

// Callbacks receives streaming events from the conversation loop.
// All fields are optional — nil callbacks are skipped.
type Callbacks struct {
	OnToken    func(token string)
	OnThinking func(token string)
	OnToolUse  func(name string, args map[string]any)
	OnDone     func(durationMs int64)
}

// Result is the final output of a conversation loop run.
type Result struct {
	Content  string
	Thinking string
}

// Run executes a conversation loop: sends messages to the LLM, streams
// the response, executes tool calls, and repeats until no more tool
// calls are made.
//
// tools and toolCtx may be nil for simple chat without tool support.
func Run(ctx context.Context, client ChatClient, req *ChatRequest, tools map[string]tool.ToolDef, toolCtx *tool.ToolContext, cb Callbacks) (*Result, error) {
	start := time.Now()
	messages := make([]Message, len(req.Messages))
	copy(messages, req.Messages)

	// Guard against nil toolCtx when tools are provided
	if tools != nil && toolCtx == nil {
		toolCtx = &tool.ToolContext{Ctx: ctx}
	}

	// Resolve max iterations
	maxIter := req.MaxIterations
	if maxIter <= 0 {
		maxIter = 20
	}

	var finalContent strings.Builder
	var finalThinking strings.Builder

	// Build tool list from map for the ChatClient
	var toolDefs []tool.ToolDef
	for _, td := range tools {
		toolDefs = append(toolDefs, td)
	}
	sort.Slice(toolDefs, func(i, j int) bool {
		return toolDefs[i].Name < toolDefs[j].Name
	})

	iterations := 0
	for {
		iterations++
		if iterations > maxIter {
			return nil, fmt.Errorf("max iterations (%d) exceeded", maxIter)
		}
		chatReq := &ChatRequest{
			Model:    req.Model,
			Messages: messages,
			Tools:    toolDefs,
			Stream:   req.Stream,
			Think:    req.Think,
			Options:  req.Options,
		}

		var turnContent strings.Builder
		var turnThinking strings.Builder
		var toolCalls []ToolCall

		err := client.Chat(ctx, chatReq, func(resp ChatResponse) error {
			if resp.Thinking != "" {
				turnThinking.WriteString(resp.Thinking)
				if cb.OnThinking != nil {
					cb.OnThinking(resp.Thinking)
				}
			}

			if resp.Content != "" {
				turnContent.WriteString(resp.Content)
				if cb.OnToken != nil {
					cb.OnToken(resp.Content)
				}
			}

			if len(resp.ToolCalls) > 0 {
				toolCalls = append(toolCalls, resp.ToolCalls...)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("chat failed: %w", err)
		}

		content := turnContent.String()
		thinking := turnThinking.String()

		// If tool calls, don't count tool-call content as final output
		if len(toolCalls) > 0 {
			content = ""
		}

		finalContent.WriteString(content)
		finalThinking.WriteString(thinking)

		// No tool calls — conversation turn is complete
		if len(toolCalls) == 0 {
			break
		}

		// Append assistant message with tool calls to history
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   turnContent.String(),
			Thinking:  thinking,
			ToolCalls: toolCalls,
		})

		// Execute each tool call
		for _, tc := range toolCalls {
			if cb.OnToolUse != nil {
				cb.OnToolUse(tc.Name, tc.Arguments)
			}

			if def, ok := tools[tc.Name]; ok {
				result := def.Execute(toolCtx, tc.Arguments)
				messages = append(messages, Message{
					Role:    "tool",
					Content: result.Content,
				})
			} else {
				slog.Warn("unknown tool called", "tool", tc.Name)
				messages = append(messages, Message{
					Role:    "tool",
					Content: fmt.Sprintf("Error: unknown tool %q", tc.Name),
				})
			}
		}
	}

	durationMs := time.Since(start).Milliseconds()
	if cb.OnDone != nil {
		cb.OnDone(durationMs)
	}

	return &Result{
		Content:  finalContent.String(),
		Thinking: finalThinking.String(),
	}, nil
}
