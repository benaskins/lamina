package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type moduleStats struct {
	Name     string  `json:"name"`
	Impl     int     `json:"impl_lines"`
	Test     int     `json:"test_lines"`
	Total    int     `json:"total_lines"`
	Coverage float64 `json:"coverage,omitempty"`
}

var statsCmd = &cobra.Command{
	Use:   "stats [repo...]",
	Short: "Lines of code and coverage across workspace modules",
	Long: `Show lines of Go code split between implementation and tests.

With no arguments, scans all axon-* modules plus axon and aurelia.
Use --cover to also run tests and report coverage (slower).`,
	RunE: runStats,
}

func init() {
	statsCmd.Flags().Bool("cover", false, "Run tests and report coverage (slower)")
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	cover, _ := cmd.Flags().GetBool("cover")

	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	repos, err := resolveStatsRepos(root, args)
	if err != nil {
		return err
	}

	var results []moduleStats
	for _, repo := range repos {
		dir, _ := resolveRepoDir(root, repo)
		if dir == "" {
			dir = filepath.Join(root, repo)
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			continue
		}

		impl, test, err := countLines(dir)
		if err != nil {
			continue
		}

		ms := moduleStats{
			Name:  repo,
			Impl:  impl,
			Test:  test,
			Total: impl + test,
		}

		if cover {
			if cov, err := moduleCoverage(dir); err == nil {
				ms.Coverage = cov
			}
		}

		results = append(results, ms)
	}

	if jsonOut {
		return printJSON(results)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	if cover {
		fmt.Fprintf(w, "Module\t  Impl\t  Test\t  Total\t  Cover\t\n")
	} else {
		fmt.Fprintf(w, "Module\t  Impl\t  Test\t  Total\t\n")
	}

	var totalImpl, totalTest int
	var covSum float64
	var covCount int
	for _, ms := range results {
		totalImpl += ms.Impl
		totalTest += ms.Test
		if cover {
			covStr := "—"
			if ms.Coverage > 0 {
				covStr = fmt.Sprintf("%.1f%%", ms.Coverage)
				covSum += ms.Coverage
				covCount++
			}
			fmt.Fprintf(w, "%s\t  %d\t  %d\t  %d\t  %s\t\n", ms.Name, ms.Impl, ms.Test, ms.Total, covStr)
		} else {
			fmt.Fprintf(w, "%s\t  %d\t  %d\t  %d\t\n", ms.Name, ms.Impl, ms.Test, ms.Total)
		}
	}

	fmt.Fprintln(w, "\t\t\t\t")
	if cover {
		avgCov := "—"
		if covCount > 0 {
			avgCov = fmt.Sprintf("%.1f%%", covSum/float64(covCount))
		}
		fmt.Fprintf(w, "Total\t  %d\t  %d\t  %d\t  %s\t\n", totalImpl, totalTest, totalImpl+totalTest, avgCov)
	} else {
		fmt.Fprintf(w, "Total\t  %d\t  %d\t  %d\t\n", totalImpl, totalTest, totalImpl+totalTest)
	}

	w.Flush()
	return nil
}

func countLines(dir string) (impl, test int, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" || base == "static" || base == "web" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		n, err := countFileLines(path)
		if err != nil {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") {
			test += n
		} else {
			impl += n
		}
		return nil
	})
	return
}

func countFileLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "//") {
			n++
		}
	}
	return n, scanner.Err()
}

func moduleCoverage(dir string) (float64, error) {
	coverFile := filepath.Join(os.TempDir(), fmt.Sprintf("lamina-stats-%d.out", os.Getpid()))
	defer os.Remove(coverFile)

	testExec := exec.Command("go", "test", "-coverprofile="+coverFile, "-coverpkg=./...", "./...")
	testExec.Dir = dir
	if err := testExec.Run(); err != nil {
		return 0, err
	}

	coverFunc := exec.Command("go", "tool", "cover", "-func="+coverFile)
	coverFunc.Dir = dir
	out, err := coverFunc.Output()
	if err != nil {
		return 0, err
	}

	return parseCoverageFunc(string(out))
}

func resolveStatsRepos(root string, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "axon" || name == "aurelia" || strings.HasPrefix(name, "axon-") {
			if _, err := os.Stat(filepath.Join(root, name, "go.mod")); err == nil {
				repos = append(repos, name)
			}
		}
	}

	// Also check apps/
	appsDir := filepath.Join(root, "apps")
	if appEntries, err := os.ReadDir(appsDir); err == nil {
		for _, e := range appEntries {
			if !e.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(appsDir, e.Name(), "go.mod")); err == nil {
				repos = append(repos, e.Name())
			}
		}
	}

	sort.Strings(repos)
	return repos, nil
}
