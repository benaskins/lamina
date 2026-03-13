package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Install pre-commit hooks in workspace repos",
	Long: `Install pre-commit hooks that enforce go vet, test coverage, and slop-guard.

  lamina hooks              Install hooks in current directory
  lamina hooks --all        Install hooks in all workspace repos

Hooks are self-contained bash scripts with no lamina dependency at runtime.
Each repo can set a coverage threshold via a .coverage-threshold file.`,
	RunE: runHooks,
}

func init() {
	hooksCmd.Flags().Bool("all", false, "Install hooks in all workspace repos")
	rootCmd.AddCommand(hooksCmd)
}

func runHooks(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all {
		root, err := workspaceRoot()
		if err != nil {
			return err
		}
		repos, err := loadRepos(root)
		if err != nil {
			return err
		}
		var installed, failed int
		for _, r := range repos {
			dir := filepath.Join(root, r.Name)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue
			}
			if err := installHooks(dir); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", r.Name, err)
				failed++
			} else {
				fmt.Printf("  ✓ %s\n", r.Name)
				installed++
			}
		}
		fmt.Printf("\n%d installed, %d failed\n", installed, failed)
		if failed > 0 {
			return fmt.Errorf("%d hook installation(s) failed", failed)
		}
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	return installHooks(dir)
}

func installHooks(repoDir string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); err != nil {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	return os.WriteFile(hookPath, []byte(preCommitHook), 0755)
}

const preCommitHook = `#!/usr/bin/env bash
set -euo pipefail

# Pre-commit hook: go vet, coverage check, slop-guard
# Installed by lamina hooks — self-contained, no lamina dependency needed.

# 1. go vet
if [ -f go.mod ]; then
    echo "Running go vet..."
    go vet ./...
fi

# 2. Coverage check on staged .go files
staged_go=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' | grep -v '_test\.go$' || true)
if [ -n "$staged_go" ] && [ -f go.mod ]; then
    threshold=0
    if [ -f .coverage-threshold ]; then
        threshold=$(head -1 .coverage-threshold | tr -d '[:space:]')
    fi

    if [ "$threshold" -gt 0 ] 2>/dev/null; then
        echo "Running tests with coverage (threshold: ${threshold}%)..."
        cover_out=$(mktemp)
        trap 'rm -f "$cover_out"' EXIT

        if go test -coverprofile="$cover_out" -coverpkg=./... ./... 2>&1 | tail -1 | grep -q "FAIL"; then
            echo "Tests failed."
            exit 1
        fi

        # Parse coverage percentage
        coverage=$(go tool cover -func="$cover_out" 2>/dev/null | grep '^total:' | awk '{print $NF}' | tr -d '%')
        if [ -z "$coverage" ]; then
            echo "Could not determine coverage."
            exit 1
        fi

        # Compare as integers (truncate decimal)
        cover_int=${coverage%%.*}
        if [ "$cover_int" -lt "$threshold" ]; then
            echo "Coverage ${coverage}% is below threshold ${threshold}%"
            exit 1
        fi
        echo "Coverage: ${coverage}% (threshold: ${threshold}%)"
    fi
fi

# 3. slop-guard (optional — skip if not installed)
if command -v slop-guard &>/dev/null; then
    staged=$(git diff --cached --name-only --diff-filter=ACM)
    if [ -n "$staged" ]; then
        echo "$staged" | xargs slop-guard
    fi
fi
`
