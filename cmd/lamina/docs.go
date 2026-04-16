package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/benaskins/lamina/internal/config"
	"github.com/benaskins/lamina/internal/docreview"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Documentation tools",
}

var docsReviewCmd = &cobra.Command{
	Use:   "review <module> [module...]",
	Short: "LLM-powered documentation review",
	Long: `Review and update documentation for one or more workspace modules.

Uses the configured LLM to inspect exports, file structure, and go.mod,
then updates README.md and AGENTS.md to match the current code.

Requires LLM configuration in ~/.config/lamina/config.yaml:
  llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
    api_key_env: ANTHROPIC_API_KEY

Examples:
  lamina docs review axon-tool
  lamina docs review axon-chat axon-loop axon-talk
  lamina docs review --all`,
	Args: cobra.MinimumNArgs(0),
	RunE: runDocsReview,
}

func init() {
	docsReviewCmd.Flags().Bool("dry-run", false, "Show proposed changes without writing")
	docsReviewCmd.Flags().Bool("all", false, "Review all workspace modules")
	docsCmd.AddCommand(docsReviewCmd)
	rootCmd.AddCommand(docsCmd)
}

func runDocsReview(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	allFlag, _ := cmd.Flags().GetBool("all")

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if !cfg.LLMConfigured() {
		return fmt.Errorf("LLM not configured — add an llm section to ~/.config/lamina/config.yaml")
	}

	apiKey, err := cfg.LLM.APIKey()
	if err != nil {
		return err
	}

	llm, err := newLLMClient(cfg.LLM.Provider, cfg.LLM.BaseURL, apiKey)
	if err != nil {
		return err
	}

	var modules []string
	if allFlag {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
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
			modules = append(modules, e.Name())
		}
	} else {
		if len(args) == 0 {
			return fmt.Errorf("specify module names or use --all")
		}
		modules = args
	}

	for _, name := range modules {
		dir := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			fmt.Printf("  skipping %s (not a git repo)\n", name)
			continue
		}

		latestTag := gitOutput(dir, "describe", "--tags", "--abbrev=0")
		prevTag := ""
		if latestTag != "" {
			prevTag = gitOutput(dir, "describe", "--tags", "--abbrev=0", latestTag+"^")
		}

		var writer docreview.Writer
		if dryRun {
			writer = &docreview.DryRunWriter{Dir: dir}
		} else {
			writer = &docreview.FileWriter{Dir: dir}
		}

		engine := docreview.NewEngine(llm, cfg.LLM.Model, dir, writer)

		fmt.Printf("\n── Reviewing %s ──\n", name)
		_, err := engine.Review(cmd.Context(), name, prevTag, latestTag, func(token string) {
			fmt.Print(token)
		})
		if err != nil {
			fmt.Printf("\n  error: %v\n", err)
			continue
		}
		fmt.Println()

		if dryRun {
			if dw, ok := writer.(*docreview.DryRunWriter); ok && len(dw.Changes) > 0 {
				for _, c := range dw.Changes {
					fmt.Printf("  [dry-run] would write %s (%d bytes)\n", c.Path, len(c.Content))
				}
			}
		} else {
			// Commit if changes were made
			if st := gitOutput(dir, "status", "--porcelain"); st != "" {
				fmt.Printf("  Committing doc updates for %s...\n", name)
				_ = runGit(dir, "add", "README.md", "AGENTS.md")
				_ = runGit(dir, "commit", "-m", "docs: update documentation for accuracy")
			}
		}
	}

	return nil
}

