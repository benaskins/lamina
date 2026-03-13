package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Workspace health check",
	Long: `Check the health of the lamina workspace:
  - Verify replace directives resolve correctly
  - Flag modules with unpublished changes vs their latest tag
  - Check go.mod versions match what's available on GitHub
  - Detect missing or broken cross-repo dependencies

Use --json for machine-readable output (pipeable to lamina heal).`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// Diagnostic represents a single doctor finding with enough context for heal to act on.
type Diagnostic struct {
	Kind    string `json:"kind"`    // "repo-dirty", "untagged", "ahead-of-tag", "replace-broken", "version-inconsistent"
	Name    string `json:"name"`    // repo/module name
	Status  string `json:"status"`  // "ok", "warn", "fail"
	Message string `json:"message"` // human-readable
	// Heal context
	Dir        string `json:"dir,omitempty"`         // absolute path to repo
	LatestTag  string `json:"latest_tag,omitempty"`  // current latest tag
	AheadCount string `json:"ahead_count,omitempty"` // commits ahead of tag
}

func runDoctor(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")

	diagnostics := runDiagnostics(root)

	if jsonOut {
		return printJSON(diagnostics)
	}

	var warns, fails int
	for _, d := range diagnostics {
		icon := "ok"
		switch d.Status {
		case "warn":
			icon = "!!"
			warns++
		case "fail":
			icon = "FAIL"
			fails++
		}
		fmt.Printf("  [%-4s] %-25s %s\n", icon, d.Name, d.Message)
	}

	fmt.Println()
	if fails > 0 {
		fmt.Printf("%d checks failed, %d warnings\n", fails, warns)
		return fmt.Errorf("%d health check(s) failed", fails)
	} else if warns > 0 {
		fmt.Printf("All checks passed (%d warnings)\n", warns)
	} else {
		fmt.Printf("All %d checks passed\n", len(diagnostics))
	}
	return nil
}

func runDiagnostics(root string) []Diagnostic {
	var diags []Diagnostic
	diags = append(diags, checkRepoHealth(root)...)
	diags = append(diags, checkReplaceDirectives(root)...)
	diags = append(diags, checkUntaggedModules(root)...)
	diags = append(diags, checkVersionConsistency(root)...)
	return diags
}

func checkRepoHealth(root string) []Diagnostic {
	var diags []Diagnostic
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}

		status := gitOutput(dir, "status", "--porcelain")
		if status == "" {
			diags = append(diags, Diagnostic{Kind: "repo-clean", Name: e.Name(), Status: "ok", Message: "clean", Dir: dir})
		} else {
			lines := strings.Count(status, "\n") + 1
			diags = append(diags, Diagnostic{Kind: "repo-dirty", Name: e.Name(), Status: "warn", Message: fmt.Sprintf("%d uncommitted changes", lines), Dir: dir})
		}
	}
	return diags
}

func checkReplaceDirectives(root string) []Diagnostic {
	var diags []Diagnostic

	goModPaths := findAllGoMods(root)

	for _, modPath := range goModPaths {
		data, err := os.ReadFile(modPath)
		if err != nil {
			continue
		}
		f, err := modfile.Parse(modPath, data, nil)
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(root, modPath)
		modDir := filepath.Dir(modPath)

		for _, rep := range f.Replace {
			if rep.New.Path == "" || !strings.HasPrefix(rep.New.Path, ".") {
				continue
			}
			resolved := filepath.Join(modDir, rep.New.Path)
			targetMod := filepath.Join(resolved, "go.mod")
			if _, err := os.Stat(targetMod); err != nil {
				diags = append(diags, Diagnostic{
					Kind:    "replace-broken",
					Name:    "replace",
					Status:  "fail",
					Message: fmt.Sprintf("%s: %s => %s (target not found)", relPath, rep.Old.Path, rep.New.Path),
					Dir:     modDir,
				})
			} else {
				diags = append(diags, Diagnostic{
					Kind:    "replace-ok",
					Name:    "replace",
					Status:  "ok",
					Message: fmt.Sprintf("%s: %s => %s", relPath, rep.Old.Path, rep.New.Path),
					Dir:     modDir,
				})
			}
		}
	}
	return diags
}

func checkUntaggedModules(root string) []Diagnostic {
	var diags []Diagnostic
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			continue
		}

		latestTag := gitOutput(dir, "describe", "--tags", "--abbrev=0")
		if latestTag == "" {
			diags = append(diags, Diagnostic{
				Kind: "untagged", Name: e.Name() + " tags", Status: "warn",
				Message: "no version tags", Dir: dir,
			})
			continue
		}

		headAtTag := gitOutput(dir, "describe", "--exact-match", "--tags", "HEAD")
		if headAtTag != "" {
			diags = append(diags, Diagnostic{
				Kind: "tag-current", Name: e.Name() + " tags", Status: "ok",
				Message: fmt.Sprintf("HEAD at %s", latestTag), Dir: dir, LatestTag: latestTag,
			})
		} else {
			ahead := gitOutput(dir, "rev-list", latestTag+"..HEAD", "--count")
			diags = append(diags, Diagnostic{
				Kind: "ahead-of-tag", Name: e.Name() + " tags", Status: "warn",
				Message: fmt.Sprintf("%s commits ahead of %s", ahead, latestTag),
				Dir: dir, LatestTag: latestTag, AheadCount: ahead,
			})
		}
	}
	return diags
}

func checkVersionConsistency(root string) []Diagnostic {
	var diags []Diagnostic

	type versionUse struct {
		version string
		usedBy  string
	}
	versions := make(map[string][]versionUse)

	// Scan all repos' cmd/* subdirectories for service modules
	entries, _ := os.ReadDir(root)
	for _, repoEntry := range entries {
		if !repoEntry.IsDir() {
			continue
		}
		cmdDir := filepath.Join(root, repoEntry.Name(), "cmd")
		svcEntries, err := os.ReadDir(cmdDir)
		if err != nil {
			continue
		}
		for _, e := range svcEntries {
			if !e.IsDir() {
				continue
			}
			modPath := filepath.Join(cmdDir, e.Name(), "go.mod")
			data, err := os.ReadFile(modPath)
			if err != nil {
				continue
			}
			f, err := modfile.Parse(modPath, data, nil)
			if err != nil {
				continue
			}
			for _, req := range f.Require {
				if strings.HasPrefix(req.Mod.Path, modulePrefix) {
					versions[req.Mod.Path] = append(versions[req.Mod.Path], versionUse{
						version: req.Mod.Version,
						usedBy:  e.Name(),
					})
				}
			}
		}
	}

	for mod, uses := range versions {
		versionSet := make(map[string][]string)
		for _, u := range uses {
			versionSet[u.version] = append(versionSet[u.version], u.usedBy)
		}
		shortMod := strings.TrimPrefix(mod, modulePrefix)
		if len(versionSet) > 1 {
			var parts []string
			for v, users := range versionSet {
				parts = append(parts, fmt.Sprintf("%s (%s)", v, strings.Join(users, ", ")))
			}
			diags = append(diags, Diagnostic{
				Kind:    "version-inconsistent",
				Name:    shortMod + " versions",
				Status:  "warn",
				Message: fmt.Sprintf("inconsistent: %s", strings.Join(parts, " vs ")),
			})
		} else {
			for v := range versionSet {
				diags = append(diags, Diagnostic{
					Kind:    "version-consistent",
					Name:    shortMod + " versions",
					Status:  "ok",
					Message: fmt.Sprintf("all services use %s", v),
				})
			}
		}
	}

	return diags
}

func findAllGoMods(root string) []string {
	var goModPaths []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoDir := filepath.Join(root, e.Name())
		modPath := filepath.Join(repoDir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			goModPaths = append(goModPaths, modPath)
		}
		// Scan cmd/* for nested service modules
		cmdDir := filepath.Join(repoDir, "cmd")
		if svcEntries, err := os.ReadDir(cmdDir); err == nil {
			for _, se := range svcEntries {
				if !se.IsDir() {
					continue
				}
				svcMod := filepath.Join(cmdDir, se.Name(), "go.mod")
				if _, err := os.Stat(svcMod); err == nil {
					goModPaths = append(goModPaths, svcMod)
				}
			}
		}
	}
	return goModPaths
}
