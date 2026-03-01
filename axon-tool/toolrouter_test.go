package tool

import (
	"context"
	"errors"
	"testing"
)

func sampleTools() []ToolDef {
	return []ToolDef{
		{Name: "web_search", Description: "Search the web for information"},
		{Name: "take_photo", Description: "Take a photo"},
		{Name: "current_time", Description: "Get the current time"},
	}
}

func fakeGenerator(response string) TextGenerator {
	return func(ctx context.Context, prompt string) (string, error) {
		return response, nil
	}
}

func TestRoute_SelectsTool(t *testing.T) {
	gen := fakeGenerator(`{"tools": ["web_search"]}`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "look up the weather", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(selected))
	}
	if selected[0].Name != "web_search" {
		t.Errorf("expected web_search, got %s", selected[0].Name)
	}
}

func TestRoute_EmptyToolsResponse(t *testing.T) {
	gen := fakeGenerator(`{"tools": []}`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "hello there", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 0 {
		t.Errorf("expected 0 tools, got %d", len(selected))
	}
}

func TestRoute_EmptyToolsInput(t *testing.T) {
	called := false
	gen := func(ctx context.Context, prompt string) (string, error) {
		called = true
		return "", nil
	}
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected != nil {
		t.Errorf("expected nil, got %v", selected)
	}
	if called {
		t.Error("generator should not be called when tools list is empty")
	}
}

func TestRoute_FallbackScanOnInvalidJSON(t *testing.T) {
	gen := fakeGenerator(`I think you should use web_search for that`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "look up something", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 tool from fallback scan, got %d", len(selected))
	}
	if selected[0].Name != "web_search" {
		t.Errorf("expected web_search, got %s", selected[0].Name)
	}
}

func TestRoute_MultipleTools(t *testing.T) {
	gen := fakeGenerator(`{"tools": ["web_search", "take_photo"]}`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "search and take a photo", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(selected))
	}
}

func TestRoute_DeduplicatesTools(t *testing.T) {
	gen := fakeGenerator(`{"tools": ["web_search", "web_search"]}`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "search twice", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 {
		t.Errorf("expected 1 deduplicated tool, got %d", len(selected))
	}
}

func TestRoute_GeneratorError(t *testing.T) {
	gen := func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("llm unavailable")
	}
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "hello", sampleTools())
	if err == nil {
		t.Fatal("expected error")
	}
	if selected != nil {
		t.Errorf("expected nil on error, got %v", selected)
	}
}

func TestRoute_IgnoresUnknownTools(t *testing.T) {
	gen := fakeGenerator(`{"tools": ["unknown_tool"]}`)
	router := NewToolRouter(gen)

	selected, err := router.Route(context.Background(), "do something", sampleTools())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 0 {
		t.Errorf("expected 0 tools for unknown name, got %d", len(selected))
	}
}
