package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "lamina",
	Short:   "Workspace management tool for the lamina compute cluster",
	Long:    `lamina provides workspace-wide operations across all repos in the lamina compute cluster — listing repos, showing dependency graphs, and running tests.`,
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// workspaceRoot returns the workspace root directory.
// Uses LAMINA_ROOT env var if set, otherwise falls back to cwd.
func workspaceRoot() (string, error) {
	if root := os.Getenv("LAMINA_ROOT"); root != "" {
		return root, nil
	}
	return os.Getwd()
}
