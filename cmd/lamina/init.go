package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// repo describes a workspace repo to clone.
type repo struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// reposFile is the YAML structure for repos.yaml.
type reposFile struct {
	Repos []repo `yaml:"repos"`
}

// defaultRepos is the built-in fallback when repos.yaml doesn't exist.
var defaultRepos = []repo{
	{"aurelia", "https://github.com/benaskins/aurelia.git"},
	{"axon", "https://github.com/benaskins/axon.git"},
	{"axon-auth", "https://github.com/benaskins/axon-auth.git"},
	{"axon-chat", "https://github.com/benaskins/axon-chat.git"},
	{"axon-eval", "https://github.com/benaskins/axon-eval.git"},
	{"axon-fact", "https://github.com/benaskins/axon-fact.git"},
	{"axon-gate", "https://github.com/benaskins/axon-gate.git"},
	{"axon-lens", "https://github.com/benaskins/axon-lens.git"},
	{"axon-look", "https://github.com/benaskins/axon-look.git"},
	{"axon-loop", "https://github.com/benaskins/axon-loop.git"},
	{"axon-memo", "https://github.com/benaskins/axon-memo.git"},
	{"axon-talk", "https://github.com/benaskins/axon-talk.git"},
	{"axon-task", "https://github.com/benaskins/axon-task.git"},
	{"axon-tool", "https://github.com/benaskins/axon-tool.git"},
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
		dir := filepath.Join(root, r.Name)

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
	}

	fmt.Printf("\nDone: %d cloned, %d skipped\n", cloned, skipped)

	// Install pre-commit hooks in all repos
	fmt.Println("\nInstalling pre-commit hooks...")
	for _, r := range repos {
		dir := filepath.Join(root, r.Name)
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
