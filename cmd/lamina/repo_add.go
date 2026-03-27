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

var repoAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a local repo to the workspace and create its GitHub remote",
	Long: `Register an existing local repo with the lamina workspace.

Creates a public GitHub repo under benaskins/, adds the remote, pushes,
appends to repos.yaml, and installs pre-commit hooks.

The repo must already exist as a directory with a .git folder under the
workspace root (or apps/ for --app).

Examples:
  lamina repo add axon-face
  lamina repo add axon-face --private
  lamina repo add imago --app --description "Interview-driven blog writer"`,
	Args: cobra.ExactArgs(1),
	RunE: runRepoAdd,
}

func init() {
	repoAddCmd.Flags().Bool("private", false, "Create a private GitHub repo")
	repoAddCmd.Flags().Bool("app", false, "Register as an app (lives under apps/)")
	repoAddCmd.Flags().String("description", "", "GitHub repo description")
	repoCmd.AddCommand(repoAddCmd)
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	private, _ := cmd.Flags().GetBool("private")
	isApp, _ := cmd.Flags().GetBool("app")
	description, _ := cmd.Flags().GetString("description")

	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	r := repo{Name: name}
	if isApp {
		r.Kind = "app"
	}
	dir := repoDir(root, r)

	// Validate the local repo exists
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("%s is not a git repo under the workspace root", name)
	}

	ghOwner := "benaskins"
	ghURL := fmt.Sprintf("https://github.com/%s/%s.git", ghOwner, name)

	// Check if already in repos.yaml
	repos, err := loadRepos(root)
	if err != nil {
		return err
	}
	for _, r := range repos {
		if r.Name == name {
			return fmt.Errorf("%s is already in repos.yaml", name)
		}
	}

	// Check if remote already exists
	existingRemote := gitOutput(dir, "remote", "get-url", "origin")
	if existingRemote != "" {
		return fmt.Errorf("%s already has origin remote: %s", name, existingRemote)
	}

	// Create GitHub repo
	fmt.Printf("Creating GitHub repo %s/%s...\n", ghOwner, name)
	ghArgs := []string{"repo", "create", ghOwner + "/" + name, "--source", dir}
	if private {
		ghArgs = append(ghArgs, "--private")
	} else {
		ghArgs = append(ghArgs, "--public")
	}
	if description != "" {
		ghArgs = append(ghArgs, "--description", description)
	}
	ghArgs = append(ghArgs, "--push")

	c := exec.Command("gh", ghArgs...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("gh repo create failed: %w", err)
	}

	// Append to repos.yaml
	fmt.Printf("Adding %s to repos.yaml...\n", name)
	r.URL = ghURL
	if err := appendToReposYAML(root, r); err != nil {
		return fmt.Errorf("failed to update repos.yaml: %w", err)
	}

	// Install pre-commit hooks
	fmt.Printf("Installing pre-commit hooks...\n")
	if err := installHooks(dir); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: hook install failed: %v\n", err)
	}

	fmt.Printf("\nDone: %s added to workspace\n", name)
	fmt.Printf("  GitHub: https://github.com/%s/%s\n", ghOwner, name)
	fmt.Printf("  Local:  %s\n", dir)
	return nil
}

// appendToReposYAML reads repos.yaml, appends the new entry, sorts, and writes back.
func appendToReposYAML(root string, r repo) error {
	path := filepath.Join(root, "repos.yaml")

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f reposFile
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("parse repos.yaml: %w", err)
		}
	}

	f.Repos = append(f.Repos, r)

	// Sort by name for consistent ordering
	sortReposByName(f.Repos)

	enc, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}

	// yaml.Marshal uses 4-space indent by default; normalize to 2-space
	// to match the hand-written repos.yaml style.
	out := strings.ReplaceAll(string(enc), "    ", "  ")

	return os.WriteFile(path, []byte(out), 0644)
}

func sortReposByName(repos []repo) {
	for i := 1; i < len(repos); i++ {
		for j := i; j > 0 && strings.Compare(repos[j].Name, repos[j-1].Name) < 0; j-- {
			repos[j], repos[j-1] = repos[j-1], repos[j]
		}
	}
}
