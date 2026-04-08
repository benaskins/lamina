package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

var catalogueCmd = &cobra.Command{
	Use:   "catalogue",
	Short: "Generate the axon component catalogue from the workspace",
	Long: `Walks all axon-* modules in the workspace, reads their go.mod and
package documentation, and generates a catalogue YAML.

If an existing catalogue is provided via --base, hand-written fields
(purpose, use_when, class) are preserved for known components. New
modules get auto-generated entries.

Experimental modules (class: experiment) are excluded by default.
Use --include-experiments to include them.`,
	RunE: runCatalogue,
}

func init() {
	catalogueCmd.Flags().String("base", "", "Base catalogue to merge with (preserves hand-written fields)")
	catalogueCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	catalogueCmd.Flags().Bool("include-experiments", false, "Include experimental modules")
	rootCmd.AddCommand(catalogueCmd)
}

type catalogueFile struct {
	Name            string               `yaml:"name"`
	Language        string               `yaml:"language"`
	Version         string               `yaml:"version"`
	Description     string               `yaml:"description"`
	ScaffoldCommand string               `yaml:"scaffold_command"`
	VerifyCommand   string               `yaml:"verify_command"`
	Components      []catalogueComponent `yaml:"components"`
	Patterns        []cataloguePattern   `yaml:"patterns"`
	FileConvs       []catalogueFileConv  `yaml:"file_conventions"`
	BoundaryNotes   string               `yaml:"boundary_notes"`
}

type catalogueComponent struct {
	Name     string   `yaml:"name"`
	Class    string   `yaml:"class"`
	Purpose  string   `yaml:"purpose"`
	UseWhen  string   `yaml:"use_when"`
	Package  string   `yaml:"package"`
	Requires []string `yaml:"requires,omitempty"`
}

type cataloguePattern struct {
	Requirement string `yaml:"requirement"`
	Pattern     string `yaml:"pattern"`
}

type catalogueFileConv struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
}

func runCatalogue(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	basePath, _ := cmd.Flags().GetString("base")
	outputPath, _ := cmd.Flags().GetString("output")
	includeExperiments, _ := cmd.Flags().GetBool("include-experiments")

	// Load base catalogue if provided (preserves hand-written metadata).
	var base *catalogueFile
	if basePath != "" {
		base, err = loadCatalogueFile(basePath)
		if err != nil {
			return fmt.Errorf("load base catalogue: %w", err)
		}
	}

	// Discover all axon-* modules.
	components, err := discoverComponents(root)
	if err != nil {
		return err
	}

	// Merge with base: preserve hand-written fields for known components.
	if base != nil {
		existing := make(map[string]catalogueComponent)
		for _, c := range base.Components {
			existing[c.Name] = c
		}
		for i, c := range components {
			if prev, ok := existing[c.Name]; ok {
				// Preserve hand-written fields.
				if prev.Purpose != "" {
					components[i].Purpose = prev.Purpose
				}
				if prev.UseWhen != "" {
					components[i].UseWhen = prev.UseWhen
				}
				if prev.Class != "" {
					components[i].Class = prev.Class
				}
			}
		}
	}

	// Filter experiments unless requested.
	if !includeExperiments {
		var filtered []catalogueComponent
		for _, c := range components {
			if c.Class != "experiment" {
				filtered = append(filtered, c)
			}
		}
		components = filtered
	}

	// Build output catalogue.
	cat := buildOutputCatalogue(base, components)

	data, err := yaml.Marshal(cat)
	if err != nil {
		return fmt.Errorf("marshal catalogue: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "catalogue: wrote %d components to %s\n", len(components), outputPath)
	} else {
		fmt.Print(string(data))
	}

	return nil
}

func loadCatalogueFile(path string) (*catalogueFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cat catalogueFile
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

// discoverComponents walks the workspace for axon-* modules and builds
// component entries from go.mod and package documentation.
func discoverComponents(root string) ([]catalogueComponent, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}

	var components []catalogueComponent
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "axon-") && name != "axon" {
			continue
		}

		dir := filepath.Join(root, name)
		modPath := filepath.Join(dir, "go.mod")
		modData, err := os.ReadFile(modPath)
		if err != nil {
			continue // not a Go module
		}

		f, err := modfile.Parse(modPath, modData, nil)
		if err != nil {
			continue
		}

		comp := catalogueComponent{
			Name:    name,
			Package: f.Module.Mod.Path,
			Class:   classifyModule(name, dir),
			Purpose: readPurpose(dir, name),
			UseWhen: fmt.Sprintf("See AGENTS.md in %s", name),
		}

		// Collect axon-* dependencies.
		for _, req := range f.Require {
			if strings.HasPrefix(req.Mod.Path, modulePrefix+"axon") && !req.Indirect {
				depName := strings.TrimPrefix(req.Mod.Path, modulePrefix)
				comp.Requires = append(comp.Requires, depName)
			}
		}
		sort.Strings(comp.Requires)

		components = append(components, comp)
	}

	sort.Slice(components, func(i, j int) bool {
		// Sort by class priority then name.
		ci, cj := classPriority(components[i].Class), classPriority(components[j].Class)
		if ci != cj {
			return ci < cj
		}
		return components[i].Name < components[j].Name
	})

	return components, nil
}

// readPurpose extracts a one-line purpose from the module's Go package doc
// comment, AGENTS.md, or README.md.
func readPurpose(dir, name string) string {
	// Try package doc comment from Go source files.
	if purpose := readPackageDoc(dir); purpose != "" {
		return purpose
	}

	// Try first content line of AGENTS.md.
	if purpose := readFirstContentLine(filepath.Join(dir, "AGENTS.md")); purpose != "" {
		return purpose
	}

	// Try first content line of README.md.
	if purpose := readFirstContentLine(filepath.Join(dir, "README.md")); purpose != "" {
		return purpose
	}

	return name + " component"
}

// readPackageDoc reads the package doc comment from .go files in the root dir.
func readPackageDoc(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "// Package ") {
				// Extract everything after "// Package <name> "
				rest := strings.TrimPrefix(line, "// ")
				parts := strings.SplitN(rest, " ", 3)
				if len(parts) >= 3 {
					return parts[2]
				}
			}
		}
	}
	return ""
}

// readFirstContentLine reads the first non-heading, non-empty line from a markdown file.
func readFirstContentLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
			continue
		}
		if len(line) > 120 {
			line = line[:120]
		}
		return line
	}
	return ""
}

// classifyModule guesses a component class from its name and structure.
func classifyModule(name, dir string) string {
	// Platform: core infrastructure (axon, axon-hand, axon-code, axon-snip)
	switch name {
	case "axon", "axon-hand", "axon-code", "axon-snip":
		return "platform"
	}

	// Check for explicit class in existing catalogue.yaml in the module.
	catPath := filepath.Join(dir, "catalogue.yaml")
	if data, err := os.ReadFile(catPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "class: experiment") {
				return "experiment"
			}
		}
	}

	// Primitives: low-level building blocks
	primitives := map[string]bool{
		"axon-loop": true, "axon-talk": true, "axon-tool": true,
		"axon-fact": true, "axon-nats": true, "axon-base": true,
		"axon-lore": true, "axon-sign": true, "axon-rule": true,
		"axon-tape": true,
	}
	if primitives[name] {
		return "primitive"
	}

	return "domain"
}

func classPriority(class string) int {
	switch class {
	case "platform":
		return 0
	case "primitive":
		return 1
	case "domain":
		return 2
	case "experiment":
		return 3
	default:
		return 4
	}
}

func buildOutputCatalogue(base *catalogueFile, components []catalogueComponent) *catalogueFile {
	if base != nil {
		base.Components = components
		return base
	}

	return &catalogueFile{
		Name:            "Axon/Lamina",
		Language:        "go",
		Version:         "1.26",
		Description:     "Go toolkit for AI-powered services on Apple Silicon. Process supervision via aurelia.",
		ScaffoldCommand: `mkdir -p {{.Name}}/cmd/{{.Name}} && cd {{.Name}} && go mod init github.com/benaskins/{{.Name}}`,
		VerifyCommand:   `cd {{.Name}} && go mod tidy && go build ./...`,
		Components:      components,
	}
}
