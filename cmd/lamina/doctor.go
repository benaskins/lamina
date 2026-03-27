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
  - Check repos have AGENTS.md and CLAUDE.md

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
		switch d.Status {
		case "warn":
			fmt.Printf("  [!!  ] %-25s %s\n", d.Name, d.Message)
			warns++
		case "fail":
			fmt.Printf("  [FAIL] %-25s %s\n", d.Name, d.Message)
			fails++
		}
	}

	if fails > 0 {
		fmt.Printf("\n%d checks failed, %d warnings\n", fails, warns)
		return fmt.Errorf("%d health check(s) failed", fails)
	} else if warns > 0 {
		fmt.Printf("\nAll checks passed (%d warnings)\n", warns)
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
	diags = append(diags, checkAgentDocs(root)...)
	return diags
}

// repoEntry is a name + directory pair found by scanning the workspace.
type repoEntry struct {
	name string
	dir  string
}

// scanRepoEntries returns all git repos found at root level and under apps/.
func scanRepoEntries(root string) []repoEntry {
	var entries []repoEntry

	scanDirs := []string{root}
	appsDir := filepath.Join(root, "apps")
	if _, err := os.Stat(appsDir); err == nil {
		scanDirs = append(scanDirs, appsDir)
	}

	for _, scanDir := range scanDirs {
		dirEntries, _ := os.ReadDir(scanDir)
		for _, e := range dirEntries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(scanDir, e.Name())
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue
			}
			entries = append(entries, repoEntry{name: e.Name(), dir: dir})
		}
	}
	return entries
}

func checkRepoHealth(root string) []Diagnostic {
	var diags []Diagnostic
	for _, re := range scanRepoEntries(root) {
		status := gitOutput(re.dir, "status", "--porcelain")
		if status == "" {
			diags = append(diags, Diagnostic{Kind: "repo-clean", Name: re.name, Status: "ok", Message: "clean", Dir: re.dir})
		} else {
			lines := strings.Count(status, "\n") + 1
			diags = append(diags, Diagnostic{Kind: "repo-dirty", Name: re.name, Status: "warn", Message: fmt.Sprintf("%d uncommitted changes", lines), Dir: re.dir})
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
	for _, re := range scanRepoEntries(root) {
		if _, err := os.Stat(filepath.Join(re.dir, "go.mod")); err != nil {
			continue
		}

		latestTag := gitOutput(re.dir, "describe", "--tags", "--abbrev=0")
		if latestTag == "" {
			diags = append(diags, Diagnostic{
				Kind: "untagged", Name: re.name + " tags", Status: "warn",
				Message: "no version tags", Dir: re.dir,
			})
			continue
		}

		headAtTag := gitOutput(re.dir, "describe", "--exact-match", "--tags", "HEAD")
		if headAtTag != "" {
			diags = append(diags, Diagnostic{
				Kind: "tag-current", Name: re.name + " tags", Status: "ok",
				Message: fmt.Sprintf("HEAD at %s", latestTag), Dir: re.dir, LatestTag: latestTag,
			})
		} else {
			ahead := gitOutput(re.dir, "rev-list", latestTag+"..HEAD", "--count")
			diags = append(diags, Diagnostic{
				Kind: "ahead-of-tag", Name: re.name + " tags", Status: "warn",
				Message: fmt.Sprintf("%s commits ahead of %s", ahead, latestTag),
				Dir: re.dir, LatestTag: latestTag, AheadCount: ahead,
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
	for _, re := range scanRepoEntries(root) {
		cmdDir := filepath.Join(re.dir, "cmd")
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

func checkAgentDocs(root string) []Diagnostic {
	var diags []Diagnostic
	for _, re := range scanRepoEntries(root) {
		hasAgents := false
		hasClaude := false
		claudePoints := false

		if _, err := os.Stat(filepath.Join(re.dir, "AGENTS.md")); err == nil {
			hasAgents = true
		}
		if data, err := os.ReadFile(filepath.Join(re.dir, "CLAUDE.md")); err == nil {
			hasClaude = true
			claudePoints = strings.Contains(string(data), "AGENTS.md")
		}

		name := re.name + " docs"
		switch {
		case hasAgents && hasClaude && claudePoints:
			diags = append(diags, Diagnostic{Kind: "agent-docs-ok", Name: name, Status: "ok", Message: "AGENTS.md + CLAUDE.md", Dir: re.dir})
		case !hasAgents && !hasClaude:
			diags = append(diags, Diagnostic{Kind: "agent-docs-missing", Name: name, Status: "warn", Message: "missing AGENTS.md and CLAUDE.md", Dir: re.dir})
		case !hasAgents:
			diags = append(diags, Diagnostic{Kind: "agent-docs-missing", Name: name, Status: "warn", Message: "missing AGENTS.md", Dir: re.dir})
		case !hasClaude:
			diags = append(diags, Diagnostic{Kind: "agent-docs-missing", Name: name, Status: "warn", Message: "missing CLAUDE.md", Dir: re.dir})
		case !claudePoints:
			diags = append(diags, Diagnostic{Kind: "agent-docs-missing", Name: name, Status: "warn", Message: "CLAUDE.md does not reference AGENTS.md", Dir: re.dir})
		}
	}
	return diags
}

func findAllGoMods(root string) []string {
	var goModPaths []string

	scanDirs := []string{root}
	appsDir := filepath.Join(root, "apps")
	if _, err := os.Stat(appsDir); err == nil {
		scanDirs = append(scanDirs, appsDir)
	}

	for _, scanDir := range scanDirs {
		entries, _ := os.ReadDir(scanDir)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(scanDir, e.Name())
			modPath := filepath.Join(dir, "go.mod")
			if _, err := os.Stat(modPath); err == nil {
				goModPaths = append(goModPaths, modPath)
			}
			// Scan cmd/* for nested service modules
			cmdDir := filepath.Join(dir, "cmd")
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
	}
	return goModPaths
}
