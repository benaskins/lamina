package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type repoInfo struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
	SHA    string `json:"sha"`
	Commit string `json:"commit"`
}

var knownVerbs = map[string]bool{
	"list":   true,
	"status": true,
	"fetch":  true,
	"push":   true,
	"rebase": true,
}

var repoCmd = &cobra.Command{
	Use:   "repo [name] [verb]",
	Short: "Git operations across workspace repos",
	Long: `Manage git repos in the lamina workspace.

  lamina repo                    Summary table of all repos (default: list)
  lamina repo list               Summary table (branch, SHA, clean/dirty, commit)
  lamina repo status             Full git status for every repo
  lamina repo fetch              Git fetch all repos
  lamina repo push --all         Git push all repos (--all required)
  lamina repo rebase --all       Git pull --rebase all repos (--all required)

  lamina repo axon               Full git status for axon (default: status)
  lamina repo axon status        Full git status for axon
  lamina repo axon fetch         Git fetch axon
  lamina repo axon push          Git push axon
  lamina repo axon rebase        Git pull --rebase axon`,
	RunE:              runRepo,
	DisableFlagParsing: false,
	Args:              cobra.ArbitraryArgs,
}

func init() {
	repoCmd.Flags().Bool("all", false, "Required for workspace-wide push and rebase")
	rootCmd.AddCommand(repoCmd)
}

func runRepo(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	// No args → list
	if len(args) == 0 {
		return repoList(cmd, root)
	}

	// arg[0] is a known verb → workspace-wide operation
	if knownVerbs[args[0]] {
		return dispatchVerb(cmd, root, args[0], "")
	}

	// arg[0] is a repo name
	repoName := args[0]
	if _, err := resolveRepoDir(root, repoName); err != nil {
		return err
	}

	verb := "status" // default verb for a named repo
	if len(args) >= 2 {
		verb = args[1]
		if !knownVerbs[verb] {
			return fmt.Errorf("unknown verb %q (valid: list, status, fetch, push, rebase)", verb)
		}
	}

	return dispatchVerb(cmd, root, verb, repoName)
}

func dispatchVerb(cmd *cobra.Command, root, verb, target string) error {
	switch verb {
	case "list":
		return repoList(cmd, root)
	case "status":
		return repoStatus(root, target)
	case "fetch":
		return repoFetch(root, target)
	case "push":
		return repoPush(cmd, root, target)
	case "rebase":
		return repoRebase(cmd, root, target)
	default:
		return fmt.Errorf("unknown verb %q", verb)
	}
}

// repoList prints the summary table (existing behaviour).
func repoList(cmd *cobra.Command, root string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	repos, err := findRepos(root)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(repos)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPO\tBRANCH\tSTATUS\tSHA\tCOMMIT")
	for _, r := range repos {
		status := "clean"
		if r.Dirty {
			status = "dirty"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Branch, status, r.SHA, r.Commit)
	}
	return w.Flush()
}

// repoStatus runs `git status` for one or all repos.
func repoStatus(root, target string) error {
	return forEachRepo(root, target, func(name, dir string) error {
		printHeader(name)
		return streamGit(dir, "status")
	})
}

// repoFetch runs `git fetch` for one or all repos.
func repoFetch(root, target string) error {
	return forEachRepo(root, target, func(name, dir string) error {
		printHeader(name)
		return streamGit(dir, "fetch", "--prune")
	})
}

// repoPush runs `git push` for one repo or all repos (requires --all).
func repoPush(cmd *cobra.Command, root, target string) error {
	if target == "" {
		allFlag, _ := cmd.Flags().GetBool("all")
		if !allFlag {
			return fmt.Errorf("push across all repos requires --all flag (or specify a repo name)")
		}
	}
	return forEachRepo(root, target, func(name, dir string) error {
		printHeader(name)
		return streamGit(dir, "push")
	})
}

// repoRebase runs `git pull --rebase` for one repo or all repos (requires --all).
func repoRebase(cmd *cobra.Command, root, target string) error {
	if target == "" {
		allFlag, _ := cmd.Flags().GetBool("all")
		if !allFlag {
			return fmt.Errorf("rebase across all repos requires --all flag (or specify a repo name)")
		}
	}
	return forEachRepo(root, target, func(name, dir string) error {
		printHeader(name)
		return streamGit(dir, "pull", "--rebase")
	})
}

// forEachRepo runs fn for a single repo (if target is set) or all repos.
func forEachRepo(root, target string, fn func(name, dir string) error) error {
	if target != "" {
		dir, err := resolveRepoDir(root, target)
		if err != nil {
			return err
		}
		return fn(target, dir)
	}

	repos, err := findRepos(root)
	if err != nil {
		return err
	}
	for _, r := range repos {
		dir, err := resolveRepoDir(root, r.Name)
		if err != nil {
			continue
		}
		if err := fn(r.Name, dir); err != nil {
			return err
		}
	}
	return nil
}

func printHeader(name string) {
	fmt.Printf("━━━ %s ━━━\n", name)
}

// streamGit runs a git command with stdout/stderr connected to the terminal.
func streamGit(dir string, args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// resolveRepoDir validates that name is a git repo under root.
// Checks both root-level and apps/ subdirectory.
func resolveRepoDir(root, name string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace root: %w", err)
	}

	// Try root-level first, then apps/
	candidates := []string{
		filepath.Join(root, name),
		filepath.Join(root, "apps", name),
	}

	for _, dir := range candidates {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absDir, absRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("repo name %q escapes workspace root", name)
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		return dir, nil
	}

	return "", fmt.Errorf("repo %q not found in workspace", name)
}

func findRepos(root string) ([]repoInfo, error) {
	var repos []repoInfo

	// Scan root-level repos
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		repos = append(repos, repoInfoFrom(e.Name(), dir))
	}

	// Scan apps/ subdirectory
	appsDir := filepath.Join(root, "apps")
	if appEntries, err := os.ReadDir(appsDir); err == nil {
		for _, e := range appEntries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(appsDir, e.Name())
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue
			}
			repos = append(repos, repoInfoFrom(e.Name(), dir))
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})
	return repos, nil
}

func repoInfoFrom(name, dir string) repoInfo {
	return repoInfo{
		Name:   name,
		Branch: gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"),
		SHA:    gitOutput(dir, "rev-parse", "--short", "HEAD"),
		Commit: gitOutput(dir, "log", "-1", "--format=%s"),
		Dirty:  gitOutput(dir, "status", "--porcelain") != "",
	}
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
