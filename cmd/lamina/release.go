package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

var releaseCmd = &cobra.Command{
	Use:   "release <module> <version>",
	Short: "Tag and push a version for a workspace module",
	Long: `Tag a workspace module with a semver version and push the tag to origin.

If the module depends on other workspace modules that have unpublished changes,
those will be listed as warnings.

Examples:
  lamina release axon v0.4.0
  lamina release axon-chat v0.2.0
  lamina release --all v0.1.0       Tag all untagged modules`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runRelease,
}

func init() {
	releaseCmd.Flags().Bool("all", false, "Release all modules that lack the specified version tag")
	releaseCmd.Flags().Bool("dry-run", false, "Show what would be done without doing it")
	rootCmd.AddCommand(releaseCmd)
}

func runRelease(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	allFlag, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if allFlag {
		if len(args) != 1 {
			return fmt.Errorf("--all requires exactly one argument: the version (e.g., lamina release --all v0.1.0)")
		}
		return releaseAll(root, args[0], dryRun)
	}

	if len(args) != 2 {
		return fmt.Errorf("requires module name and version (e.g., lamina release axon v0.4.0)")
	}
	return releaseOne(root, args[0], args[1], dryRun)
}

func releaseOne(root, name, version string, dryRun bool) error {
	dir := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("%q is not a git repo in the workspace", name)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return fmt.Errorf("%q has no go.mod", name)
	}

	// Check for dirty state
	if status := gitOutput(dir, "status", "--porcelain"); status != "" {
		return fmt.Errorf("%s has uncommitted changes — commit or stash first", name)
	}

	// Check if tag already exists
	existing := gitOutput(dir, "tag", "-l", version)
	if existing != "" {
		return fmt.Errorf("%s already has tag %s", name, version)
	}

	// Check workspace deps for unpublished changes
	warnings := checkDepsPublished(root, dir)
	for _, w := range warnings {
		fmt.Printf("  warning: %s\n", w)
	}

	if dryRun {
		fmt.Printf("[dry-run] would tag %s at %s and push\n", name, version)
		return nil
	}

	fmt.Printf("Tagging %s %s...\n", name, version)
	if err := runGit(dir, "tag", version); err != nil {
		return fmt.Errorf("git tag failed: %w", err)
	}

	fmt.Printf("Pushing tag %s...\n", version)
	if err := runGit(dir, "push", "origin", version); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	fmt.Printf("Released %s %s\n", name, version)
	return nil
}

func releaseAll(root, version string, dryRun bool) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	var modules []releaseModule

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err != nil {
			continue
		}

		// Skip if tag already exists
		existing := gitOutput(dir, "tag", "-l", version)
		if existing != "" {
			continue
		}

		// Skip if dirty
		if status := gitOutput(dir, "status", "--porcelain"); status != "" {
			fmt.Printf("  skipping %s (uncommitted changes)\n", e.Name())
			continue
		}

		deps := workspaceDeps(modPath)
		modules = append(modules, releaseModule{name: e.Name(), deps: deps})
	}

	if len(modules) == 0 {
		fmt.Println("No modules need tagging")
		return nil
	}

	// Topological sort: release dependencies before dependents
	ordered := topoSort(modules)

	for _, name := range ordered {
		if err := releaseOne(root, name, version, dryRun); err != nil {
			return err
		}
	}
	return nil
}

func workspaceDeps(goModPath string) []string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil
	}
	var deps []string
	for _, req := range f.Require {
		if strings.HasPrefix(req.Mod.Path, modulePrefix) {
			deps = append(deps, strings.TrimPrefix(req.Mod.Path, modulePrefix))
		}
	}
	return deps
}

func checkDepsPublished(root, dir string) []string {
	modPath := filepath.Join(dir, "go.mod")
	deps := workspaceDeps(modPath)
	var warnings []string
	for _, dep := range deps {
		depDir := filepath.Join(root, dep)
		if _, err := os.Stat(filepath.Join(depDir, ".git")); err != nil {
			continue
		}
		// Check if HEAD is tagged
		tag := gitOutput(depDir, "describe", "--exact-match", "--tags", "HEAD")
		if tag == "" {
			warnings = append(warnings, fmt.Sprintf("dependency %s has untagged commits at HEAD", dep))
		}
	}
	return warnings
}

func topoSort(modules []releaseModule) []string {
	byName := make(map[string]releaseModule)
	for _, m := range modules {
		byName[m.name] = m
	}

	visited := make(map[string]bool)
	var order []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		if m, ok := byName[name]; ok {
			for _, dep := range m.deps {
				visit(dep)
			}
		}
		order = append(order, name)
	}

	for _, m := range modules {
		visit(m.name)
	}
	return order
}

type releaseModule struct {
	name string
	deps []string
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
