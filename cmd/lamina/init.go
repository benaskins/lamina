package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// repo describes a workspace repo to clone.
type repo struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Kind string `yaml:"kind,omitempty"` // "app" for apps, empty/"repo" for libraries/services
}

// IsApp returns true if the repo is an application.
func (r repo) IsApp() bool {
	return r.Kind == "app"
}

// reposFile is the YAML structure for repos.yaml.
type reposFile struct {
	Repos []repo `yaml:"repos"`
}

// repoDir returns the filesystem path for a repo relative to the workspace root.
// Apps live under apps/<name>, everything else at the root.
func repoDir(root string, r repo) string {
	if r.IsApp() {
		return filepath.Join(root, "apps", r.Name)
	}
	return filepath.Join(root, r.Name)
}

// defaultRepos is the built-in fallback when repos.yaml doesn't exist.
var defaultRepos = []repo{
	{Name: "aurelia", URL: "https://github.com/benaskins/aurelia.git"},
	{Name: "axon", URL: "https://github.com/benaskins/axon.git"},
	{Name: "axon-auth", URL: "https://github.com/benaskins/axon-auth.git"},
	{Name: "axon-book", URL: "https://github.com/benaskins/axon-book.git"},
	{Name: "axon-chat", URL: "https://github.com/benaskins/axon-chat.git"},
	{Name: "axon-eval", URL: "https://github.com/benaskins/axon-eval.git"},
	{Name: "axon-fact", URL: "https://github.com/benaskins/axon-fact.git"},
	{Name: "axon-gate", URL: "https://github.com/benaskins/axon-gate.git"},
	{Name: "axon-lens", URL: "https://github.com/benaskins/axon-lens.git"},
	{Name: "axon-look", URL: "https://github.com/benaskins/axon-look.git"},
	{Name: "axon-loop", URL: "https://github.com/benaskins/axon-loop.git"},
	{Name: "axon-memo", URL: "https://github.com/benaskins/axon-memo.git"},
	{Name: "axon-mind", URL: "https://github.com/benaskins/axon-mind.git"},
	{Name: "axon-nats", URL: "https://github.com/benaskins/axon-nats.git"},
	{Name: "axon-synd", URL: "https://github.com/benaskins/axon-synd.git"},
	{Name: "axon-talk", URL: "https://github.com/benaskins/axon-talk.git"},
	{Name: "axon-task", URL: "https://github.com/benaskins/axon-task.git"},
	{Name: "axon-tool", URL: "https://github.com/benaskins/axon-tool.git"},
}

// loadRepos reads repos.yaml from the given directory. If the file doesn't
// exist, returns the built-in default list.
func loadRepos(dir string) ([]repo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "repos.yaml"))
	if os.IsNotExist(err) {
		return defaultRepos, nil
	}
	if err != nil {
		return nil, err
	}

	var f reposFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("repos.yaml: %w", err)
	}
	return f.Repos, nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Clone all workspace repos into the current directory",
	Long: `Populate the lamina workspace by cloning all known repos.

Repos that already exist locally are skipped. Run from the workspace
root (the directory containing this repo).`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	repos, err := loadRepos(root)
	if err != nil {
		return err
	}

	var cloned, skipped int

	for _, r := range repos {
		dir := repoDir(root, r)

		// Ensure parent directory exists for apps
		if r.IsApp() {
			if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
				fmt.Fprintf(os.Stderr, "  error creating apps dir: %v\n", err)
				continue
			}
		}

		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			fmt.Printf("  skip  %s (already exists)\n", r.Name)
			skipped++
			continue
		}

		fmt.Printf("  clone %s\n", r.Name)
		c := exec.Command("git", "clone", r.URL, dir)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  error cloning %s: %v\n", r.Name, err)
			continue
		}
		cloned++

		// Wire up replace directives for apps so they use workspace libraries
		if r.IsApp() {
			if err := wireAppReplaces(root, dir); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: could not wire replaces for %s: %v\n", r.Name, err)
			}
		}
	}

	fmt.Printf("\nDone: %d cloned, %d skipped\n", cloned, skipped)

	// Install pre-commit hooks in all repos
	fmt.Println("\nInstalling pre-commit hooks...")
	for _, r := range repos {
		dir := repoDir(root, r)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		if err := installHooks(dir); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", r.Name, err)
		} else {
			fmt.Printf("  ✓ %s\n", r.Name)
		}
	}

	return nil
}

// wireAppReplaces adds replace directives to an app's go.mod pointing workspace
// dependencies at their local paths. This lets apps use local library changes
// during development.
func wireAppReplaces(root, appDir string) error {
	modPath := filepath.Join(appDir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil // no go.mod, nothing to do
	}

	modContent := string(data)
	changed := false

	// Find all workspace dependencies and add replace directives
	for _, line := range strings.Split(modContent, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, modulePrefix) {
			continue
		}
		// Extract module path (before version)
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		modName := parts[0]
		shortName := strings.TrimPrefix(modName, modulePrefix)

		// Check if workspace has this module
		localDir := filepath.Join(root, shortName)
		if _, err := os.Stat(filepath.Join(localDir, "go.mod")); err != nil {
			continue
		}

		// Compute relative path from app to library
		relPath, err := filepath.Rel(appDir, localDir)
		if err != nil {
			continue
		}

		// Skip if replace already exists for this module
		if strings.Contains(modContent, "replace "+modName) {
			continue
		}

		// Add replace directive using go mod edit
		c := exec.Command("go", "mod", "edit", "-replace", modName+"="+relPath)
		c.Dir = appDir
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: could not add replace for %s: %v\n", shortName, err)
			continue
		}
		changed = true
	}

	if changed {
		fmt.Printf("  wired %s replace directives for local development\n", filepath.Base(appDir))
	}
	return nil
}
