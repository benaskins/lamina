package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ToolRouter uses an LLM (via TextGenerator) to decide which tools
// are appropriate for a given user message, preventing the conversational
// model from being biased toward calling tools on every turn.
type ToolRouter struct {
	generate TextGenerator
}

// toolRouterResponse is the structured output from the routing LLM.
type toolRouterResponse struct {
	Tools []string `json:"tools"`
}

// NewToolRouter creates a new tool router that uses the given TextGenerator
// to evaluate which tools should be offered for a user message.
func NewToolRouter(generate TextGenerator) *ToolRouter {
	return &ToolRouter{generate: generate}
}

// Route evaluates the user's message against available tools and returns
// only the tools that should be offered to the conversational model.
// On error or timeout, returns nil (safe default).
func (tr *ToolRouter) Route(ctx context.Context, userMessage string, tools []ToolDef) ([]ToolDef, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	// Build tool descriptions and index.
	var toolList strings.Builder
	toolIndex := make(map[string]ToolDef)
	var names []string
	for _, t := range tools {
		toolIndex[t.Name] = t
		names = append(names, t.Name)
		fmt.Fprintf(&toolList, "- %s: %s\n", t.Name, t.Description)
	}

	prompt := buildRouterPrompt(toolList.String(), userMessage)

	routeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()

	response, err := tr.generate(routeCtx, prompt)

	latency := time.Since(start).Milliseconds()

	if err != nil {
		slog.Warn("tool router failed, defaulting to no tools", "error", err, "latency_ms", latency)
		return nil, err
	}

	selected := parseToolRouterResponse(response, toolIndex)

	slog.Info("tool router decision",
		"user_message", userMessage,
		"raw_response", response,
		"selected_tools", selectedToolNames(selected),
		"latency_ms", latency,
	)

	return selected, nil
}

// buildRouterPrompt constructs the prompt sent to the LLM for routing.
func buildRouterPrompt(toolDescriptions, userMessage string) string {
	return fmt.Sprintf(`You decide which tools (if any) should be used for this conversation turn.
Return a JSON object with a "tools" array containing the tool names to use. Use an empty array if no tools are needed.

Most messages need no tools. Only select a tool when the user's message clearly calls for it.

Available tools:
%s
User message: %s`, toolDescriptions, userMessage)
}

// parseToolRouterResponse parses the JSON response and returns matching tools.
// Falls back to scanning for tool names if JSON parsing fails.
func parseToolRouterResponse(response string, available map[string]ToolDef) []ToolDef {
	var result toolRouterResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		slog.Warn("tool router response not valid JSON, falling back to scan",
			"response", response, "error", err)
		return scanForToolNames(response, available)
	}

	var selected []ToolDef
	seen := make(map[string]bool)
	for _, name := range result.Tools {
		if t, ok := available[name]; ok && !seen[name] {
			selected = append(selected, t)
			seen[name] = true
		}
	}
	return selected
}

// scanForToolNames is a fallback that scans the response text for known tool names.
func scanForToolNames(response string, available map[string]ToolDef) []ToolDef {
	cleaned := strings.ToLower(response)
	var selected []ToolDef
	for name, t := range available {
		if strings.Contains(cleaned, name) {
			selected = append(selected, t)
		}
	}
	return selected
}

// selectedToolNames returns tool names for logging.
func selectedToolNames(tools []ToolDef) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
