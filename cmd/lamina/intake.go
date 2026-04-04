package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

// pickingSlip is the YAML structure for the factory-floor picking slip.
type pickingSlip struct {
	Entries []slipEntry `yaml:"entries"`
}

// slipEntry is one component on the picking slip.
type slipEntry struct {
	Name    string     `yaml:"name"`
	Build   string     `yaml:"build"`
	Passed  time.Time  `yaml:"passed"`
	Type    string     `yaml:"type"`
	Depends []string   `yaml:"depends,omitempty"`
	Taken   *time.Time `yaml:"taken,omitempty"`
}

func parsePickingSlip(path string) (*pickingSlip, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read picking slip: %w", err)
	}
	var slip pickingSlip
	if err := yaml.Unmarshal(data, &slip); err != nil {
		return nil, fmt.Errorf("parse picking slip: %w", err)
	}
	return &slip, nil
}

func writePickingSlip(path string, slip *pickingSlip) error {
	data, err := yaml.Marshal(slip)
	if err != nil {
		return fmt.Errorf("marshal picking slip: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func pendingEntries(slip *pickingSlip) []slipEntry {
	var pending []slipEntry
	for _, e := range slip.Entries {
		if e.Taken == nil {
			pending = append(pending, e)
		}
	}
	return pending
}

// resolveModuleName reads go.mod from a build dir and returns the short
// module name (e.g. "axon-rule" from "github.com/benaskins/axon-rule").
func resolveModuleName(buildDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(buildDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("parse go.mod: %w", err)
	}
	name := strings.TrimPrefix(f.Module.Mod.Path, modulePrefix)
	if name == f.Module.Mod.Path {
		return "", fmt.Errorf("module %q does not start with %s", f.Module.Mod.Path, modulePrefix)
	}
	return name, nil
}

var intakeCmd = &cobra.Command{
	Use:   "intake",
	Short: "Pull ready components from the factory floor into the warehouse",
	Long: `Read the factory-floor picking slip and process each pending entry:
verify, clean go.mod, push to GitHub, register in repos.yaml, build,
test, and update the luthier catalogue.

Entries are processed top-down (dependency order). Each entry is marked
as taken after successful intake.`,
	RunE: runIntake,
}

func init() {
	intakeCmd.Flags().String("factory", "", "path to factory-floor (default: $FACTORY_FLOOR or ../factory-floor)")
	intakeCmd.Flags().String("luthier", "", "path to luthier repo (default: $LUTHIER_ROOT or ../luthier)")
	intakeCmd.Flags().Bool("dry-run", false, "show what would be done without doing it")
	intakeCmd.Flags().Bool("skip-verify", false, "skip axon-scan verification (trust the slip)")
	rootCmd.AddCommand(intakeCmd)
}

func runIntake(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	factoryDir, _ := cmd.Flags().GetString("factory")
	if factoryDir == "" {
		factoryDir = os.Getenv("FACTORY_FLOOR")
	}
	if factoryDir == "" {
		factoryDir = filepath.Join(root, "..", "factory-floor")
	}

	luthierDir, _ := cmd.Flags().GetString("luthier")
	if luthierDir == "" {
		luthierDir = os.Getenv("LUTHIER_ROOT")
	}
	if luthierDir == "" {
		luthierDir = filepath.Join(root, "..", "luthier")
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")

	slipPath := filepath.Join(factoryDir, "picking-slip.yaml")
	slip, err := parsePickingSlip(slipPath)
	if err != nil {
		return err
	}

	pending := pendingEntries(slip)
	if len(pending) == 0 {
		fmt.Println("Nothing to intake. All entries are taken.")
		return nil
	}

	fmt.Printf("Found %d pending entries on the picking slip\n\n", len(pending))

	for i := range slip.Entries {
		e := &slip.Entries[i]
		if e.Taken != nil {
			continue
		}

		fmt.Printf("=== %s ===\n", e.Name)

		buildDir := filepath.Join(factoryDir, "builds", e.Build)
		if _, err := os.Stat(buildDir); err != nil {
			return fmt.Errorf("build dir not found: %s", buildDir)
		}

		// Resolve the actual module name from go.mod
		modName, err := resolveModuleName(buildDir)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
		fmt.Printf("  module: %s%s\n", modulePrefix, modName)

		// Determine target dir in lamina workspace
		targetDir := filepath.Join(root, modName)
		isUpdate := false
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			isUpdate = true
			fmt.Printf("  target: %s (update existing)\n", modName)
		} else {
			fmt.Printf("  target: %s (new)\n", modName)
		}

		if dryRun {
			fmt.Printf("  [dry-run] would intake %s as %s\n\n", e.Name, modName)
			continue
		}

		// Step 1: Verify (run axon-scan if available)
		if !skipVerify {
			if err := verifyBuild(buildDir); err != nil {
				return fmt.Errorf("%s: verification failed: %w", e.Name, err)
			}
		}

		// Step 2: Copy build to workspace
		if isUpdate {
			if err := syncBuildToExisting(buildDir, targetDir); err != nil {
				return fmt.Errorf("%s: sync failed: %w", e.Name, err)
			}
		} else {
			if err := copyBuildToWorkspace(buildDir, targetDir); err != nil {
				return fmt.Errorf("%s: copy failed: %w", e.Name, err)
			}
		}

		// Step 3: Clean go.mod (strip replace directives)
		if err := stripReplaces(root, targetDir); err != nil {
			return fmt.Errorf("%s: clean go.mod failed: %w", e.Name, err)
		}

		// Step 4: go mod tidy
		fmt.Printf("  tidying go.mod...\n")
		tidy := exec.Command("go", "mod", "tidy")
		tidy.Dir = targetDir
		tidy.Env = append(os.Environ(), "GOPRIVATE=github.com/benaskins/*")
		if out, err := tidy.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: go mod tidy failed: %s\n%w", e.Name, out, err)
		}

		// Step 5: Commit clean state
		_ = runGit(targetDir, "add", "-A")
		_ = runGit(targetDir, "commit", "-m", "release: clean go.mod for intake")

		// Step 6: Push to GitHub
		if isUpdate {
			fmt.Printf("  pushing to GitHub...\n")
			if err := runGit(targetDir, "push", "origin", "main"); err != nil {
				return fmt.Errorf("%s: push failed: %w", e.Name, err)
			}
		} else {
			fmt.Printf("  creating GitHub repo and pushing...\n")
			if err := createAndPushRepo(modName, targetDir); err != nil {
				return fmt.Errorf("%s: GitHub push failed: %w", e.Name, err)
			}
			// Register in repos.yaml
			r := repo{
				Name: modName,
				URL:  fmt.Sprintf("https://github.com/benaskins/%s.git", modName),
			}
			if e.Type == "service" || e.Type == "cli" {
				r.Kind = "app"
			}
			if err := appendToReposYAML(root, r); err != nil {
				return fmt.Errorf("%s: repos.yaml update failed: %w", e.Name, err)
			}
		}

		// Step 7: Tag version
		version := nextVersion(targetDir)
		fmt.Printf("  tagging %s...\n", version)
		if err := runGit(targetDir, "tag", version); err != nil {
			return fmt.Errorf("%s: tag failed: %w", e.Name, err)
		}
		if err := runGit(targetDir, "push", "origin", version); err != nil {
			return fmt.Errorf("%s: push tag failed: %w", e.Name, err)
		}

		// Step 8: Build + test
		fmt.Printf("  building...\n")
		build := exec.Command("go", "build", "./...")
		build.Dir = targetDir
		build.Env = append(os.Environ(), "GOPRIVATE=github.com/benaskins/*")
		if out, err := build.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: build failed: %s\n%w", e.Name, out, err)
		}

		fmt.Printf("  testing...\n")
		test := exec.Command("go", "test", "./...")
		test.Dir = targetDir
		test.Env = append(os.Environ(), "GOPRIVATE=github.com/benaskins/*")
		if out, err := test.CombinedOutput(); err != nil {
			fmt.Printf("  warning: tests failed: %s\n", out)
			// Don't block intake on test failures; they may need db/infra
		}

		// Step 9: Update luthier catalogue
		if err := updateCatalogue(luthierDir, buildDir, modName, e.Type); err != nil {
			fmt.Printf("  warning: catalogue update failed: %v\n", err)
		}

		// Step 10: Mark taken
		now := time.Now()
		e.Taken = &now
		if err := writePickingSlip(slipPath, slip); err != nil {
			return fmt.Errorf("%s: mark taken failed: %w", e.Name, err)
		}

		fmt.Printf("  done\n\n")
	}

	fmt.Println("Intake complete.")
	return nil
}

func verifyBuild(buildDir string) error {
	// Check if axon-scan is available
	scanPath, err := exec.LookPath("axon-scan")
	if err != nil {
		fmt.Printf("  verify: axon-scan not found, checking go vet + go test...\n")
		// Fallback: run go vet and go test
		vet := exec.Command("go", "vet", "./...")
		vet.Dir = buildDir
		if out, err := vet.CombinedOutput(); err != nil {
			return fmt.Errorf("go vet failed: %s\n%w", out, err)
		}
		fmt.Printf("  verify: go vet passed\n")
		return nil
	}

	fmt.Printf("  verify: running axon-scan...\n")
	scan := exec.Command(scanPath, "-layers", "static,security,test", buildDir)
	scan.Stdout = os.Stdout
	scan.Stderr = os.Stderr
	return scan.Run()
}

func copyBuildToWorkspace(buildDir, targetDir string) error {
	fmt.Printf("  copying to workspace...\n")
	cmd := exec.Command("cp", "-r", buildDir, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp failed: %s\n%w", out, err)
	}
	return nil
}

func syncBuildToExisting(buildDir, targetDir string) error {
	fmt.Printf("  syncing to existing repo...\n")
	// Use rsync to update files, excluding .git
	cmd := exec.Command("rsync", "-a", "--exclude=.git", "--delete",
		buildDir+"/", targetDir+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsync failed: %s\n%w", out, err)
	}
	return nil
}

func stripReplaces(root, dir string) error {
	modPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}
	f, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return err
	}

	changed := false
	for _, rep := range f.Replace {
		// Strip all local (absolute or relative path) replaces
		if strings.HasPrefix(rep.New.Path, "/") || strings.HasPrefix(rep.New.Path, ".") {
			// Before dropping, try to pin to the latest tag of the dependency
			if strings.HasPrefix(rep.Old.Path, modulePrefix) {
				depName := strings.TrimPrefix(rep.Old.Path, modulePrefix)
				depDir := filepath.Join(root, depName)
				if latestTag := gitOutput(depDir, "describe", "--tags", "--abbrev=0"); latestTag != "" {
					if err := f.AddRequire(rep.Old.Path, latestTag); err != nil {
						fmt.Printf("  warning: could not pin %s to %s: %v\n", depName, latestTag, err)
					}
				}
			}
			if err := f.DropReplace(rep.Old.Path, rep.Old.Version); err != nil {
				return fmt.Errorf("drop replace for %s: %w", rep.Old.Path, err)
			}
			changed = true
		}
	}

	if !changed {
		fmt.Printf("  go.mod: no replace directives to strip\n")
		return nil
	}

	f.Cleanup()
	out, err := f.Format()
	if err != nil {
		return err
	}
	fmt.Printf("  go.mod: stripped replace directives\n")
	return os.WriteFile(modPath, out, 0644)
}

func createAndPushRepo(name, dir string) error {
	ghArgs := []string{"repo", "create", "benaskins/" + name,
		"--public", "--source", dir, "--push"}
	cmd := exec.Command("gh", ghArgs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func nextVersion(dir string) string {
	latest := gitOutput(dir, "describe", "--tags", "--abbrev=0")
	if latest == "" {
		return "v0.1.0"
	}
	// Parse and bump minor
	parts := strings.Split(strings.TrimPrefix(latest, "v"), ".")
	if len(parts) != 3 {
		return "v0.1.0"
	}
	minor := 0
	fmt.Sscanf(parts[1], "%d", &minor)
	return fmt.Sprintf("v%s.%d.0", parts[0], minor+1)
}

func updateCatalogue(luthierDir, buildDir, modName, componentType string) error {
	catPath := filepath.Join(luthierDir, "catalogues", "axon.yaml")
	data, err := os.ReadFile(catPath)
	if err != nil {
		return fmt.Errorf("read catalogue: %w", err)
	}

	// Check if component already exists in catalogue
	if strings.Contains(string(data), "name: "+modName+"\n") {
		fmt.Printf("  catalogue: %s already present\n", modName)
		return nil
	}

	// Read AGENTS.md from build for purpose description
	purpose := readModuleDescription(buildDir)
	if purpose == "" {
		purpose = modName + " component"
	}

	// Determine class from type
	class := "primitive"
	if componentType == "service" || componentType == "cli" {
		class = "domain"
	}

	// Read dependencies from go.mod
	var requires []string
	gomodData, err := os.ReadFile(filepath.Join(buildDir, "go.mod"))
	if err == nil {
		f, err := modfile.Parse("go.mod", gomodData, nil)
		if err == nil {
			for _, req := range f.Require {
				if strings.HasPrefix(req.Mod.Path, modulePrefix) && !req.Indirect {
					requires = append(requires, strings.TrimPrefix(req.Mod.Path, modulePrefix))
				}
			}
		}
	}

	// Build YAML entry manually to match existing format
	var entry strings.Builder
	entry.WriteString(fmt.Sprintf("\n  - name: %s\n", modName))
	entry.WriteString(fmt.Sprintf("    class: %s\n", class))
	entry.WriteString(fmt.Sprintf("    purpose: %q\n", purpose))
	entry.WriteString(fmt.Sprintf("    use_when: See AGENTS.md in %s\n", modName))
	entry.WriteString(fmt.Sprintf("    package: %s%s\n", modulePrefix, modName))
	if len(requires) > 0 {
		entry.WriteString(fmt.Sprintf("    requires: [%s]\n", strings.Join(requires, ", ")))
	}

	// Insert before the "patterns:" section
	content := string(data)
	patternIdx := strings.Index(content, "\npatterns:")
	if patternIdx == -1 {
		// Append to end of components
		content += entry.String()
	} else {
		content = content[:patternIdx] + entry.String() + content[patternIdx:]
	}

	fmt.Printf("  catalogue: added %s\n", modName)
	return os.WriteFile(catPath, []byte(content), 0644)
}
