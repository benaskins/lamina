package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

type moduleDep struct {
	Module string   `json:"module"`
	Path   string   `json:"path"`
	Deps   []string `json:"deps"`
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Show dependency graph between workspace modules",
	RunE:  runDeps,
}

func init() {
	rootCmd.AddCommand(depsCmd)
}

func runDeps(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")

	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	modules, err := findModules(root)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(modules)
	}

	printDepsTree(modules)
	return nil
}

const modulePrefix = "github.com/benaskins/"

func findModules(root string) ([]moduleDep, error) {
	// Collect all workspace module names for filtering
	workspaceModules := make(map[string]bool)

	var goModPaths []string

	// Find go.mod files: top-level repos and aurelia-core-infrastructure/cmd/*
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		modPath := filepath.Join(root, e.Name(), "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			goModPaths = append(goModPaths, modPath)
		}
	}

	// Also scan aurelia-core-infrastructure/cmd/* for service modules
	aciCmd := filepath.Join(root, "aurelia-core-infrastructure", "cmd")
	if svcEntries, err := os.ReadDir(aciCmd); err == nil {
		for _, e := range svcEntries {
			if !e.IsDir() {
				continue
			}
			modPath := filepath.Join(aciCmd, e.Name(), "go.mod")
			if _, err := os.Stat(modPath); err == nil {
				goModPaths = append(goModPaths, modPath)
			}
		}
	}

	// First pass: collect all workspace module names (github.com/benaskins/* modules)
	for _, modPath := range goModPaths {
		data, err := os.ReadFile(modPath)
		if err != nil {
			continue
		}
		f, err := modfile.Parse(modPath, data, nil)
		if err != nil {
			continue
		}
		if strings.HasPrefix(f.Module.Mod.Path, modulePrefix) {
			workspaceModules[f.Module.Mod.Path] = true
		}
	}

	// Second pass: build dependency list for all modules that either
	// are workspace modules or depend on workspace modules (services)
	var modules []moduleDep
	for _, modPath := range goModPaths {
		data, err := os.ReadFile(modPath)
		if err != nil {
			continue
		}
		f, err := modfile.Parse(modPath, data, nil)
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(root, filepath.Dir(modPath))
		mod := moduleDep{
			Module: f.Module.Mod.Path,
			Path:   relPath,
		}

		for _, req := range f.Require {
			if workspaceModules[req.Mod.Path] {
				mod.Deps = append(mod.Deps, req.Mod.Path)
			}
		}

		// Include if it's a workspace module OR if it has workspace deps
		if !workspaceModules[f.Module.Mod.Path] && len(mod.Deps) == 0 {
			continue
		}

		sort.Strings(mod.Deps)
		modules = append(modules, mod)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Module < modules[j].Module
	})
	return modules, nil
}

func printDepsTree(modules []moduleDep) {
	// Group: libraries (no deps on services) vs services (in cmd/)
	var libs, services []moduleDep
	for _, m := range modules {
		if strings.Contains(m.Path, "/cmd/") {
			services = append(services, m)
		} else {
			libs = append(libs, m)
		}
	}

	if len(libs) > 0 {
		fmt.Println("Libraries:")
		for _, m := range libs {
			shortName := strings.TrimPrefix(m.Module, modulePrefix)
			if len(m.Deps) == 0 {
				fmt.Printf("  %s\n", shortName)
			} else {
				depNames := shortNames(m.Deps)
				fmt.Printf("  %s → %s\n", shortName, strings.Join(depNames, ", "))
			}
		}
	}

	if len(services) > 0 {
		fmt.Println("\nServices:")
		for _, m := range services {
			label := filepath.Base(m.Path)
			if len(m.Deps) == 0 {
				fmt.Printf("  %s\n", label)
			} else {
				depNames := shortNames(m.Deps)
				fmt.Printf("  %s → %s\n", label, strings.Join(depNames, ", "))
			}
		}
	}
}

func shortNames(modules []string) []string {
	out := make([]string, len(modules))
	for i, m := range modules {
		out[i] = strings.TrimPrefix(m, modulePrefix)
	}
	return out
}
