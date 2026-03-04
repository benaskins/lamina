package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type testResult struct {
	Repo     string  `json:"repo"`
	Passed   bool    `json:"passed"`
	Duration float64 `json:"duration_secs"`
	Error    string  `json:"error,omitempty"`
}

var testCmd = &cobra.Command{
	Use:   "test [repo...]",
	Short: "Run go test ./... across repos",
	Long:  `Run go test ./... in one or more repos. With no arguments, tests all axon-* libraries.`,
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	repos, err := resolveTestRepos(root, args)
	if err != nil {
		return err
	}

	var results []testResult
	for _, repo := range repos {
		dir := filepath.Join(root, repo)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			results = append(results, testResult{
				Repo:   repo,
				Passed: false,
				Error:  "no go.mod found",
			})
			continue
		}

		if !jsonOut {
			fmt.Printf("━━━ %s ━━━\n", repo)
		}

		start := time.Now()
		testExec := exec.Command("go", "test", "./...")
		testExec.Dir = dir
		if !jsonOut {
			testExec.Stdout = os.Stdout
			testExec.Stderr = os.Stderr
		}
		runErr := testExec.Run()
		duration := time.Since(start).Seconds()

		result := testResult{
			Repo:     repo,
			Passed:   runErr == nil,
			Duration: duration,
		}
		if runErr != nil {
			result.Error = runErr.Error()
		}
		results = append(results, result)

		if !jsonOut {
			fmt.Println()
		}
	}

	if jsonOut {
		return printJSON(results)
	}

	// Summary
	fmt.Println("━━━ Summary ━━━")
	passed, failed := 0, 0
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
			failed++
		} else {
			passed++
		}
		fmt.Printf("  %s %s (%.1fs)\n", status, r.Repo, r.Duration)
	}
	fmt.Printf("\n%d passed, %d failed\n", passed, failed)

	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

func resolveTestRepos(root string, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	// Default: all axon-* directories with go.mod
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var repos []string
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "axon-") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "go.mod")); err != nil {
			continue
		}
		repos = append(repos, e.Name())
	}
	sort.Strings(repos)
	return repos, nil
}
