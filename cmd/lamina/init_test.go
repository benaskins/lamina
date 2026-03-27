package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepos_FromYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `repos:
  - name: axon
    url: https://github.com/benaskins/axon.git
  - name: axon-chat
    url: https://github.com/benaskins/axon-chat.git
  - name: imago
    url: https://github.com/benaskins/imago.git
    kind: app
`
	if err := os.WriteFile(filepath.Join(dir, "repos.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := loadRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Fatalf("got %d repos, want 3", len(repos))
	}
	if repos[0].Name != "axon" {
		t.Errorf("repos[0].Name = %q, want %q", repos[0].Name, "axon")
	}
	if repos[1].URL != "https://github.com/benaskins/axon-chat.git" {
		t.Errorf("repos[1].URL = %q, want axon-chat URL", repos[1].URL)
	}
	if repos[2].Kind != "app" {
		t.Errorf("repos[2].Kind = %q, want %q", repos[2].Kind, "app")
	}
}

func TestRepoDir_Apps(t *testing.T) {
	r := repo{Name: "imago", Kind: "app"}
	got := repoDir("/workspace", r)
	want := "/workspace/apps/imago"
	if got != want {
		t.Errorf("repoDir(app) = %q, want %q", got, want)
	}
}

func TestRepoDir_Library(t *testing.T) {
	r := repo{Name: "axon"}
	got := repoDir("/workspace", r)
	want := "/workspace/axon"
	if got != want {
		t.Errorf("repoDir(lib) = %q, want %q", got, want)
	}
}

func TestIsApp(t *testing.T) {
	if (repo{Name: "axon"}).IsApp() {
		t.Error("library should not be an app")
	}
	if !(repo{Name: "imago", Kind: "app"}).IsApp() {
		t.Error("app should be an app")
	}
}

func TestLoadRepos_FallsBackToDefault(t *testing.T) {
	dir := t.TempDir() // no repos.yaml present

	repos, err := loadRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) == 0 {
		t.Fatal("expected default repos, got empty")
	}
	// Verify a known repo is present
	found := false
	for _, r := range repos {
		if r.Name == "axon" {
			found = true
			break
		}
	}
	if !found {
		t.Error("default repos should include 'axon'")
	}
}

func TestLoadRepos_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repos.yaml"), []byte("not: [valid: yaml: {"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadRepos(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
