package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

var appsCmd = &cobra.Command{
	Use:   "apps [name]",
	Short: "Manage workspace applications",
	Long: `List and manage applications in the lamina workspace.

  lamina apps                    List all apps with status and dependencies
  lamina apps build <name>       Build an app binary
  lamina apps install [name]     Build and install app(s) to ~/.local/bin
  lamina apps wire [name]        Wire replace directives for local development`,
	RunE: runApps,
	Args: cobra.ArbitraryArgs,
}

var appsBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build an app binary",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppsBuild,
}

var appsInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Build and install app(s) to ~/.local/bin",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAppsInstall,
}

var appsWireCmd = &cobra.Command{
	Use:   "wire [name]",
	Short: "Wire replace directives for local development",
	Long:  `Add replace directives to app go.mod files pointing at workspace libraries.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAppsWire,
}

func init() {
	appsCmd.AddCommand(appsBuildCmd)
	appsCmd.AddCommand(appsInstallCmd)
	appsCmd.AddCommand(appsWireCmd)
	rootCmd.AddCommand(appsCmd)
}

type appInfo struct {
	Name   string   `json:"name"`
	Branch string   `json:"branch"`
	Dirty  bool     `json:"dirty"`
	SHA    string   `json:"sha"`
	Commit string   `json:"commit"`
	Deps   []string `json:"deps"`
}

func runApps(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")

	apps, err := findApps(root)
	if err != nil {
		return err
	}

	if len(apps) == 0 {
		fmt.Println("No apps found (add apps with kind: app in repos.yaml)")
		return nil
	}

	if jsonOut {
		return printJSON(apps)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tBRANCH\tSTATUS\tSHA\tDEPS")
	for _, a := range apps {
		status := "clean"
		if a.Dirty {
			status = "dirty"
		}
		deps := "-"
		if len(a.Deps) > 0 {
			deps = strings.Join(a.Deps, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.Name, a.Branch, status, a.SHA, deps)
	}
	return w.Flush()
}

func findApps(root string) ([]appInfo, error) {
	appsDir := filepath.Join(root, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var apps []appInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(appsDir, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}

		info := appInfo{
			Name:   e.Name(),
			Branch: gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"),
			SHA:    gitOutput(dir, "rev-parse", "--short", "HEAD"),
			Commit: gitOutput(dir, "log", "-1", "--format=%s"),
			Dirty:  gitOutput(dir, "status", "--porcelain") != "",
		}

		// Parse workspace dependencies
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			if f, err := modfile.Parse(modPath, data, nil); err == nil {
				for _, req := range f.Require {
					if strings.HasPrefix(req.Mod.Path, modulePrefix) {
						info.Deps = append(info.Deps, strings.TrimPrefix(req.Mod.Path, modulePrefix))
					}
				}
			}
		}

		apps = append(apps, info)
	}
	return apps, nil
}

func runAppsBuild(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	name := args[0]
	dir := filepath.Join(root, "apps", name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("app %q not found in apps/", name)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Building %s...\n", name)
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, name), fmt.Sprintf("./cmd/%s", name))
	build.Dir = dir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	fmt.Printf("Built %s → %s\n", name, filepath.Join(binDir, name))
	return nil
}

func runAppsInstall(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installDir := filepath.Join(home, ".local", "bin")

	var names []string
	if len(args) > 0 {
		names = []string{args[0]}
	} else {
		// Install all apps
		apps, err := findApps(root)
		if err != nil {
			return err
		}
		for _, a := range apps {
			names = append(names, a.Name)
		}
	}

	for _, name := range names {
		dir := filepath.Join(root, "apps", name)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			fmt.Fprintf(os.Stderr, "  skipping %s (not found)\n", name)
			continue
		}

		fmt.Printf("Building %s...\n", name)
		build := exec.Command("go", "build", "-o", filepath.Join(installDir, name), fmt.Sprintf("./cmd/%s", name))
		build.Dir = dir
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  error building %s: %v\n", name, err)
			continue
		}
		fmt.Printf("Installed %s → %s\n", name, filepath.Join(installDir, name))
	}
	return nil
}

func runAppsWire(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	var names []string
	if len(args) > 0 {
		names = []string{args[0]}
	} else {
		apps, err := findApps(root)
		if err != nil {
			return err
		}
		for _, a := range apps {
			names = append(names, a.Name)
		}
	}

	for _, name := range names {
		dir := filepath.Join(root, "apps", name)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
			continue
		}
		fmt.Printf("Wiring %s...\n", name)
		if err := wireAppReplaces(root, dir); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		}
	}
	return nil
}
