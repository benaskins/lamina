package main

import (
	"os"
	"path/filepath"
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
