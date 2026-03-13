package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type testResult struct {
	Repo     string  `json:"repo"`
	Passed   bool    `json:"passed"`
	Duration float64 `json:"duration_secs"`
	Coverage float64 `json:"coverage,omitempty"`
	Error    string  `json:"error,omitempty"`
}

var testCmd = &cobra.Command{
	Use:   "test [repo...]",
	Short: "Run go test ./... across repos",
	Long:  `Run go test ./... in one or more repos. With no arguments, tests all axon-* libraries.`,
	RunE:  runTest,
}

func init() {
	testCmd.Flags().Bool("cover", false, "Run with coverage profiling")
	testCmd.Flags().Int("min-cover", 0, "Fail if any module is below N% coverage (implies --cover)")
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	cover, _ := cmd.Flags().GetBool("cover")
	minCover, _ := cmd.Flags().GetInt("min-cover")
	if minCover > 0 {
		cover = true
	}

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

		var testArgs []string
		var coverFile string
		if cover {
			coverFile = filepath.Join(os.TempDir(), fmt.Sprintf("lamina-cover-%s-%d.out", repo, time.Now().UnixNano()))
			testArgs = []string{"test", "-coverprofile=" + coverFile, "-coverpkg=./...", "./..."}
		} else {
			testArgs = []string{"test", "./..."}
		}

		testExec := exec.Command("go", testArgs...)
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

		// Parse coverage if enabled and tests passed
		if cover && runErr == nil && coverFile != "" {
			coverFunc := exec.Command("go", "tool", "cover", "-func="+coverFile)
			coverFunc.Dir = dir
			if out, err := coverFunc.Output(); err == nil {
				if cov, err := parseCoverageFunc(string(out)); err == nil {
					result.Coverage = cov

					// Check against threshold
					threshold := minCover
					if threshold == 0 {
						threshold = readCoverageThreshold(dir)
					}
					if threshold > 0 && cov < float64(threshold) {
						result.Passed = false
						result.Error = fmt.Sprintf("coverage %.1f%% below threshold %d%%", cov, threshold)
					}
				}
			}
			os.Remove(coverFile)
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
		coverStr := ""
		if r.Coverage > 0 {
			coverStr = fmt.Sprintf(" [%.1f%%]", r.Coverage)
		}
		fmt.Printf("  %s %s (%.1fs)%s\n", status, r.Repo, r.Duration, coverStr)
	}
	fmt.Printf("\n%d passed, %d failed\n", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func parseCoverageFunc(output string) (float64, error) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "total:") {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
			return strconv.ParseFloat(pctStr, 64)
		}
	}
	return 0, fmt.Errorf("no total coverage line found")
}

func readCoverageThreshold(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, ".coverage-threshold"))
	if err != nil {
		return 0
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return val
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
