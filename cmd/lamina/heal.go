package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var healCmd = &cobra.Command{
	Use:   "heal",
	Short: "Fix issues found by doctor",
	Long: `Automatically fix healable issues in the workspace.

Can run standalone or consume doctor's JSON output via pipe:
  lamina doctor --json | lamina heal
  lamina heal                         # runs doctor internally
  lamina heal --dry-run               # show what would be done`,
	RunE: runHeal,
}

func init() {
	healCmd.Flags().Bool("dry-run", false, "Show what would be done without doing it")
	rootCmd.AddCommand(healCmd)
}

func runHeal(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	diags, err := loadDiagnostics()
	if err != nil {
		return err
	}

	// Filter to healable issues
	var actions []Diagnostic
	for _, d := range diags {
		switch d.Kind {
		case "untagged", "ahead-of-tag", "agent-docs-missing":
			actions = append(actions, d)
		}
	}

	if len(actions) == 0 {
		fmt.Println("Nothing to heal")
		return nil
	}

	for _, d := range actions {
		switch d.Kind {
		case "untagged":
			if err := healUntagged(cmd.Context(), root, d, dryRun); err != nil {
				fmt.Printf("  FAIL %s: %v\n", d.Name, err)
			}
		case "ahead-of-tag":
			if err := healAheadOfTag(cmd.Context(), root, d, dryRun); err != nil {
				fmt.Printf("  FAIL %s: %v\n", d.Name, err)
			}
		case "agent-docs-missing":
			if err := healAgentDocs(d, dryRun); err != nil {
				fmt.Printf("  FAIL %s: %v\n", d.Name, err)
			}
		}
	}

	return nil
}

func loadDiagnostics() ([]Diagnostic, error) {
	// Check if stdin has data (piped)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		var diags []Diagnostic
		if err := json.Unmarshal(data, &diags); err != nil {
			return nil, fmt.Errorf("parsing diagnostics JSON: %w", err)
		}
		return diags, nil
	}

	// No pipe — run doctor internally
	root, err := workspaceRoot()
	if err != nil {
		return nil, err
	}
	return runDiagnostics(root), nil
}

// repoNameFromDiagnostic extracts the repo name from a diagnostic Name field
// (e.g., "imago tags" → "imago").
func repoNameFromDiagnostic(d Diagnostic) string {
	return strings.TrimSuffix(d.Name, " tags")
}

func healUntagged(ctx context.Context, root string, d Diagnostic, dryRun bool) error {
	if d.Dir == "" {
		return fmt.Errorf("no directory in diagnostic")
	}

	tag := "v0.1.0"
	name := repoNameFromDiagnostic(d)
	return releaseOne(ctx, root, name, tag, dryRun)
}

func healAheadOfTag(ctx context.Context, root string, d Diagnostic, dryRun bool) error {
	if d.Dir == "" {
		return fmt.Errorf("no directory in diagnostic")
	}
	if d.LatestTag == "" {
		return fmt.Errorf("no latest tag in diagnostic")
	}

	nextTag, err := bumpPatch(d.LatestTag)
	if err != nil {
		return fmt.Errorf("computing next version: %w", err)
	}

	name := repoNameFromDiagnostic(d)
	return releaseOne(ctx, root, name, nextTag, dryRun)
}

const claudeMDPointer = "# CLAUDE.md\n\nRead [AGENTS.md](./AGENTS.md) for project context.\n"

func healAgentDocs(d Diagnostic, dryRun bool) error {
	if d.Dir == "" {
		return fmt.Errorf("no directory in diagnostic")
	}

	name := strings.TrimSuffix(d.Name, " docs")
	claudePath := d.Dir + "/CLAUDE.md"
	agentsPath := d.Dir + "/AGENTS.md"

	// Create AGENTS.md if missing
	if _, err := os.Stat(agentsPath); err != nil {
		if dryRun {
			fmt.Printf("  [dry-run] would create %s/AGENTS.md\n", name)
		} else {
			content := fmt.Sprintf("# %s\n\n## Build & Test\n\n```bash\ngo test ./...\ngo vet ./...\n```\n", name)
			if err := os.WriteFile(agentsPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing AGENTS.md: %w", err)
			}
			fmt.Printf("  Created %s/AGENTS.md\n", name)
		}
	}

	// Create or fix CLAUDE.md
	needsClaude := false
	if data, err := os.ReadFile(claudePath); err != nil {
		needsClaude = true
	} else if !strings.Contains(string(data), "AGENTS.md") {
		needsClaude = true
	}

	if needsClaude {
		if dryRun {
			fmt.Printf("  [dry-run] would create %s/CLAUDE.md pointing to AGENTS.md\n", name)
		} else {
			if err := os.WriteFile(claudePath, []byte(claudeMDPointer), 0644); err != nil {
				return fmt.Errorf("writing CLAUDE.md: %w", err)
			}
			fmt.Printf("  Created %s/CLAUDE.md\n", name)
		}
	}

	return nil
}

// bumpPatch increments the patch version: v0.3.0 -> v0.3.1
func bumpPatch(tag string) (string, error) {
	v := strings.TrimPrefix(tag, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("unexpected version format: %s", tag)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("parsing patch version: %w", err)
	}
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch+1), nil
}
