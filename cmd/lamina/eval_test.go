package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalCmd_MissingPlanFile(t *testing.T) {
	// Use RunE directly to test the command logic
	err := runEval(evalCmd, []string{"/nonexistent/plan.yaml"})
	if err == nil {
		t.Error("expected error for missing plan file")
	}
}

func TestEvalCmd_LoadsValidPlan(t *testing.T) {
	yaml := `
name: test plan
scenarios:
  - name: greeting
    message: "Hello"
    rubric:
      - type: min_length
        value: 1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	// Set URLs to unreachable addresses so we fail at client creation, not plan loading
	t.Setenv("AUTH_URL", "http://127.0.0.1:1")
	t.Setenv("CHAT_URL", "http://127.0.0.1:2")
	t.Setenv("ANALYTICS_URL", "http://127.0.0.1:3")

	err := runEval(evalCmd, []string{path})
	if err == nil {
		t.Error("expected error (no services running)")
		return
	}

	// Error should be about creating client, not loading plan
	errStr := err.Error()
	if strings.Contains(errStr, "load plan") {
		t.Errorf("error should not be about plan loading: %v", err)
	}
	if !strings.Contains(errStr, "create client") {
		t.Errorf("expected 'create client' error, got: %v", err)
	}
}
