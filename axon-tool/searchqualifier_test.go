package tool

import (
	"context"
	"errors"
	"testing"
)

func TestSearchQualifier_RefinesQuery(t *testing.T) {
	generator := func(ctx context.Context, prompt string) (string, error) {
		return "golang concurrency patterns goroutines", nil
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "concurrency patterns", "You are a Go programming expert.")

	if result != "golang concurrency patterns goroutines" {
		t.Errorf("expected refined query, got %q", result)
	}
}

func TestSearchQualifier_ReturnsOriginalOnEmptyResponse(t *testing.T) {
	generator := func(ctx context.Context, prompt string) (string, error) {
		return "", nil
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "concurrency patterns", "You are a Go programming expert.")

	if result != "concurrency patterns" {
		t.Errorf("expected original query, got %q", result)
	}
}

func TestSearchQualifier_ReturnsOriginalOnError(t *testing.T) {
	generator := func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("model unavailable")
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "concurrency patterns", "You are a Go programming expert.")

	if result != "concurrency patterns" {
		t.Errorf("expected original query, got %q", result)
	}
}

func TestSearchQualifier_ReturnsOriginalOnEmptySystemPrompt(t *testing.T) {
	called := false
	generator := func(ctx context.Context, prompt string) (string, error) {
		called = true
		return "should not be used", nil
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "concurrency patterns", "")

	if result != "concurrency patterns" {
		t.Errorf("expected original query, got %q", result)
	}
	if called {
		t.Error("TextGenerator should not be called when system prompt is empty")
	}
}

func TestSearchQualifier_ReturnsOriginalOnWhitespaceSystemPrompt(t *testing.T) {
	generator := func(ctx context.Context, prompt string) (string, error) {
		return "should not be used", nil
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "concurrency patterns", "   ")

	if result != "concurrency patterns" {
		t.Errorf("expected original query, got %q", result)
	}
}

func TestSearchQualifier_ReturnsOriginalWhenGeneratorNil(t *testing.T) {
	sq := NewSearchQualifier(nil)
	result := sq.Qualify(context.Background(), "concurrency patterns", "You are a Go expert.")

	if result != "concurrency patterns" {
		t.Errorf("expected original query, got %q", result)
	}
}

func TestSearchQualifier_TrimsWhitespaceFromResponse(t *testing.T) {
	generator := func(ctx context.Context, prompt string) (string, error) {
		return "  refined query  \n", nil
	}

	sq := NewSearchQualifier(generator)
	result := sq.Qualify(context.Background(), "original", "You are an expert.")

	if result != "refined query" {
		t.Errorf("expected trimmed result, got %q", result)
	}
}
