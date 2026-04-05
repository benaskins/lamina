package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePickingSlip(t *testing.T) {
	content := `entries:
  - name: axon-hand
    build: axon-hand
    passed: 2026-04-04T13:00:00+11:00
    type: library

  - name: code-hand
    build: code-hand
    passed: 2026-04-04T13:02:00+11:00
    type: service
    depends: [axon-hand]

  - name: taken-already
    build: taken-already
    passed: 2026-04-04T13:01:00+11:00
    type: library
    taken: 2026-04-04T14:00:00+11:00
`

	dir := t.TempDir()
	path := filepath.Join(dir, "picking-slip.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	slip, err := parsePickingSlip(path)
	if err != nil {
		t.Fatalf("parsePickingSlip: %v", err)
	}

	if len(slip.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(slip.Entries))
	}

	// First entry
	e := slip.Entries[0]
	if e.Name != "axon-hand" {
		t.Errorf("entry[0].Name = %q, want %q", e.Name, "axon-hand")
	}
	if e.Build != "axon-hand" {
		t.Errorf("entry[0].Build = %q, want %q", e.Build, "axon-hand")
	}
	if e.Type != "library" {
		t.Errorf("entry[0].Type = %q, want %q", e.Type, "library")
	}
	if e.Taken != nil {
		t.Errorf("entry[0].Taken should be nil, got %v", e.Taken)
	}

	// Second entry with depends
	e = slip.Entries[1]
	if len(e.Depends) != 1 || e.Depends[0] != "axon-hand" {
		t.Errorf("entry[1].Depends = %v, want [axon-hand]", e.Depends)
	}

	// Third entry already taken
	e = slip.Entries[2]
	if e.Taken == nil {
		t.Fatal("entry[2].Taken should not be nil")
	}

	// pendingEntries should skip taken
	pending := pendingEntries(slip)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(pending))
	}
	if pending[0].Name != "axon-hand" {
		t.Errorf("pending[0].Name = %q, want %q", pending[0].Name, "axon-hand")
	}
	if pending[1].Name != "code-hand" {
		t.Errorf("pending[1].Name = %q, want %q", pending[1].Name, "code-hand")
	}
}

func TestMarkTaken(t *testing.T) {
	content := `entries:
  - name: axon-hand
    build: axon-hand
    passed: 2026-04-04T13:00:00+11:00
    type: library
`

	dir := t.TempDir()
	path := filepath.Join(dir, "picking-slip.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	slip, err := parsePickingSlip(path)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 4, 15, 0, 0, 0, time.FixedZone("AEST", 11*3600))
	slip.Entries[0].Taken = &now

	if err := writePickingSlip(path, slip); err != nil {
		t.Fatalf("writePickingSlip: %v", err)
	}

	// Re-read and verify
	slip2, err := parsePickingSlip(path)
	if err != nil {
		t.Fatal(err)
	}

	if slip2.Entries[0].Taken == nil {
		t.Fatal("expected Taken to be set after write")
	}

	// Should now have 0 pending
	if pending := pendingEntries(slip2); len(pending) != 0 {
		t.Errorf("expected 0 pending after marking taken, got %d", len(pending))
	}
}

func TestResolveModuleName(t *testing.T) {
	// Create a fake build dir with go.mod
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "axon-rule-sm")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatal(err)
	}

	gomod := "module github.com/benaskins/axon-rule\n\ngo 1.26.1\n"
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}

	name, err := resolveModuleName(buildDir)
	if err != nil {
		t.Fatalf("resolveModuleName: %v", err)
	}
	if name != "axon-rule" {
		t.Errorf("resolveModuleName = %q, want %q", name, "axon-rule")
	}
}

func TestCleanClaudeMD_ScaffoldDetected(t *testing.T) {
	dir := t.TempDir()

	// Write a scaffold CLAUDE.md with factory markers
	scaffold := `# CLAUDE.md

## What This Is

Initialise the Go module. Do stuff.

## Framework: Axon/Lamina (go 1.26)

### Patterns

- generic boilerplate here
`
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(scaffold), 0644); err != nil {
		t.Fatal(err)
	}

	// Write an AGENTS.md with constraints
	agents := `# axon-test

Test library.

## Constraints

- No external dependencies
- Tests must use t.TempDir()
`
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a .original that should be deleted
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md.original"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanClaudeMD(dir, "axon-test"); err != nil {
		t.Fatalf("cleanClaudeMD: %v", err)
	}

	// Verify scaffold was replaced
	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "Framework: Axon/Lamina") {
		t.Error("scaffold marker still present after cleanup")
	}
	if !strings.Contains(content, "# axon-test") {
		t.Error("expected module name as heading")
	}
	if !strings.Contains(content, "No external dependencies") {
		t.Error("expected constraints from AGENTS.md")
	}

	// Verify .original was removed
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md.original")); err == nil {
		t.Error("CLAUDE.md.original should have been removed")
	}
}

func TestCleanClaudeMD_CleanFileUntouched(t *testing.T) {
	dir := t.TempDir()

	clean := "# axon-foo\n\nA proper post-build CLAUDE.md.\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(clean), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanClaudeMD(dir, "axon-foo"); err != nil {
		t.Fatalf("cleanClaudeMD: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != clean {
		t.Error("clean CLAUDE.md should not have been modified")
	}
}

func TestCleanClaudeMD_NoFile(t *testing.T) {
	dir := t.TempDir()

	// Should not error when CLAUDE.md doesn't exist
	if err := cleanClaudeMD(dir, "axon-foo"); err != nil {
		t.Fatalf("cleanClaudeMD: %v", err)
	}
}
