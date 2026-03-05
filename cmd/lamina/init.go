package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// workspaceRepos is the canonical list of repos that make up the lamina workspace.
var workspaceRepos = []struct {
	Name string
	URL  string
}{
	{"aurelia", "https://github.com/benaskins/aurelia.git"},
	{"axon", "https://github.com/benaskins/axon.git"},
	{"axon-auth", "https://github.com/benaskins/axon-auth.git"},
	{"axon-chat", "https://github.com/benaskins/axon-chat.git"},
	{"axon-eval", "https://github.com/benaskins/axon-eval.git"},
	{"axon-gate", "https://github.com/benaskins/axon-gate.git"},
	{"axon-lens", "https://github.com/benaskins/axon-lens.git"},
	{"axon-look", "https://github.com/benaskins/axon-look.git"},
	{"axon-loop", "https://github.com/benaskins/axon-loop.git"},
	{"axon-memo", "https://github.com/benaskins/axon-memo.git"},
	{"axon-talk", "https://github.com/benaskins/axon-talk.git"},
	{"axon-task", "https://github.com/benaskins/axon-task.git"},
	{"axon-tool", "https://github.com/benaskins/axon-tool.git"},
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

	var cloned, skipped int

	for _, repo := range workspaceRepos {
		dir := filepath.Join(root, repo.Name)

		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			fmt.Printf("  skip  %s (already exists)\n", repo.Name)
			skipped++
			continue
		}

		fmt.Printf("  clone %s\n", repo.Name)
		c := exec.Command("git", "clone", repo.URL, dir)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  error cloning %s: %v\n", repo.Name, err)
			continue
		}
		cloned++
	}

	fmt.Printf("\nDone: %d cloned, %d skipped\n", cloned, skipped)
	return nil
}
