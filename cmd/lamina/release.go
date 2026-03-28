package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	loop "github.com/benaskins/axon-loop"
	talk "github.com/benaskins/axon-talk"
	"github.com/benaskins/axon-talk/anthropic"
	"github.com/benaskins/axon-talk/openai"
	"github.com/benaskins/lamina/internal/config"
	"github.com/benaskins/lamina/internal/docreview"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

var semverRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`)

func validateVersion(version string) error {
	if !semverRe.MatchString(version) {
		return fmt.Errorf("invalid version %q: must be semver like v1.0.0 or v1.2.3-beta.1", version)
	}
	return nil
}

var releaseCmd = &cobra.Command{
	Use:   "release <module> <version>",
	Short: "Tag and push a version for a workspace module",
	Long: `Tag a workspace module with a semver version and push the tag to origin.

If the module depends on other workspace modules that have unpublished changes,
those will be listed as warnings.

Examples:
  lamina release axon v0.4.0
  lamina release axon-chat v0.2.0
  lamina release --all v0.1.0       Tag all untagged modules`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runRelease,
}

func init() {
	releaseCmd.Flags().Bool("all", false, "Release all modules that lack the specified version tag")
	releaseCmd.Flags().Bool("dry-run", false, "Show what would be done without doing it")
	releaseCmd.Flags().Bool("backfill-notes", false, "Create GitHub releases for existing tags that lack them")
	rootCmd.AddCommand(releaseCmd)
}

func runRelease(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	allFlag, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	backfill, _ := cmd.Flags().GetBool("backfill-notes")

	if backfill {
		if len(args) > 0 {
			return fmt.Errorf("--backfill-notes takes no arguments")
		}
		return backfillReleaseNotes(root, dryRun)
	}

	if allFlag {
		if len(args) != 1 {
			return fmt.Errorf("--all requires exactly one argument: the version (e.g., lamina release --all v0.1.0)")
		}
		if err := validateVersion(args[0]); err != nil {
			return err
		}
		return releaseAll(cmd.Context(), root, args[0], dryRun)
	}

	if len(args) != 2 {
		return fmt.Errorf("requires module name and version (e.g., lamina release axon v0.4.0)")
	}
	if err := validateVersion(args[1]); err != nil {
		return err
	}
	return releaseOne(cmd.Context(), root, args[0], args[1], dryRun)
}

func releaseOne(ctx context.Context, root, name, version string, dryRun bool) error {
	dir, err := resolveRepoDir(root, name)
	if err != nil {
		return fmt.Errorf("%q is not a git repo in the workspace", name)
	}
	isApp := strings.HasPrefix(dir, filepath.Join(root, "apps")+string(os.PathSeparator))
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return fmt.Errorf("%q has no go.mod", name)
	}

	// Check for dirty state
	if status := gitOutput(dir, "status", "--porcelain"); status != "" {
		return fmt.Errorf("%s has uncommitted changes — commit or stash first", name)
	}

	// Check if tag already exists
	existing := gitOutput(dir, "tag", "-l", version)
	if existing != "" {
		return fmt.Errorf("%s already has tag %s", name, version)
	}

	// Check workspace deps for unpublished changes
	warnings := checkDepsPublished(root, dir)
	for _, w := range warnings {
		fmt.Printf("  warning: %s\n", w)
	}

	// Security audit
	fmt.Printf("Running security audit on %s...\n", name)
	audit := runSecurityAudit(dir)
	audit.print(dir)
	if !dryRun {
		if err := audit.check(); err != nil {
			return fmt.Errorf("release blocked: %v", err)
		}
	}

	if dryRun {
		fmt.Printf("[dry-run] would tag %s at %s and push\n", name, version)

		prevTag := gitOutput(dir, "describe", "--tags", "--abbrev=0")
		var logRange string
		if prevTag != "" {
			logRange = prevTag + "..HEAD"
		} else {
			logRange = "HEAD"
		}
		commitLog := gitOutput(dir, "log", "--oneline", logRange)
		diff := gitOutput(dir, "diff", "--stat", logRange)
		notes := releaseNotes(ctx, dir, name, version, commitLog, diff)
		fmt.Printf("[dry-run] release notes:\n%s\n", notes)
		runDocReview(ctx, dir, name, prevTag, version, true)
		return nil
	}

	// For apps: strip replace directives, tidy, commit, tag, then restore
	if isApp {
		if err := stripAppReplaces(root, dir); err != nil {
			return fmt.Errorf("stripping replace directives: %w", err)
		}
		tidyCmd := exec.Command("go", "mod", "tidy")
		tidyCmd.Dir = dir
		if err := tidyCmd.Run(); err != nil {
			restoreAppReplaces(dir)
			return fmt.Errorf("go mod tidy after stripping replaces: %w", err)
		}
		_ = runGit(dir, "add", "go.mod", "go.sum")
		_ = runGit(dir, "commit", "-m", fmt.Sprintf("release: strip replace directives for %s", version))
	}

	fmt.Printf("Tagging %s %s...\n", name, version)
	if err := runGit(dir, "tag", version); err != nil {
		if isApp {
			restoreAppReplaces(dir)
		}
		return fmt.Errorf("git tag failed: %w", err)
	}

	fmt.Printf("Pushing tag %s...\n", version)
	if err := runGit(dir, "push", "origin", version); err != nil {
		if isApp {
			restoreAppReplaces(dir)
		}
		return fmt.Errorf("git push failed: %w", err)
	}

	// For apps: push the commit with stripped replaces, then restore for local dev
	if isApp {
		_ = runGit(dir, "push", "origin", "main")
		if err := restoreAppReplaces(dir); err != nil {
			fmt.Printf("  warning: could not restore replace directives: %v\n", err)
		}
	}

	fmt.Printf("Released %s %s\n", name, version)

	// Create GitHub release with notes
	prevTag := gitOutput(dir, "describe", "--tags", "--abbrev=0", version+"^")
	var logRange string
	if prevTag != "" {
		logRange = prevTag + ".." + version
	} else {
		logRange = version // first release — all commits
	}
	commitLog := gitOutput(dir, "log", "--oneline", logRange)
	diff := gitOutput(dir, "diff", "--stat", logRange)
	notes := releaseNotes(ctx, dir, name, version, commitLog, diff)

	fmt.Printf("Creating GitHub release %s...\n", version)
	ghArgs := []string{"release", "create", version, "--title", version, "--notes", notes}
	ghCmd := exec.Command("gh", ghArgs...)
	ghCmd.Dir = dir
	ghCmd.Stdout = os.Stdout
	ghCmd.Stderr = os.Stderr
	if err := ghCmd.Run(); err != nil {
		fmt.Printf("  warning: gh release create failed: %v\n", err)
	}

	// Run LLM doc review if configured
	runDocReview(ctx, dir, name, prevTag, version, false)

	// Refresh getlamina.ai with updated versions and deps
	refreshScript := filepath.Join(root, "scripts", "refresh-getlamina")
	if _, err := os.Stat(refreshScript); err == nil {
		fmt.Println("Refreshing getlamina.ai...")
		refresh := exec.Command(refreshScript, "--deploy")
		refresh.Dir = root
		refresh.Stdout = os.Stdout
		refresh.Stderr = os.Stderr
		if err := refresh.Run(); err != nil {
			fmt.Printf("  warning: refresh-getlamina failed: %v\n", err)
		}
	}

	return nil
}

func releaseAll(ctx context.Context, root, version string, dryRun bool) error {
	var modules []releaseModule

	// Scan root-level and apps/ directories
	scanDirs := []string{root}
	appsDir := filepath.Join(root, "apps")
	if _, err := os.Stat(appsDir); err == nil {
		scanDirs = append(scanDirs, appsDir)
	}

	for _, scanDir := range scanDirs {
		entries, err := os.ReadDir(scanDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(scanDir, e.Name())
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue
			}
			modPath := filepath.Join(dir, "go.mod")
			if _, err := os.Stat(modPath); err != nil {
				continue
			}

			// Skip if tag already exists
			existing := gitOutput(dir, "tag", "-l", version)
			if existing != "" {
				continue
			}

			// Skip if dirty
			if status := gitOutput(dir, "status", "--porcelain"); status != "" {
				fmt.Printf("  skipping %s (uncommitted changes)\n", e.Name())
				continue
			}

			deps := workspaceDeps(modPath)
			modules = append(modules, releaseModule{name: e.Name(), deps: deps})
		}
	}

	if len(modules) == 0 {
		fmt.Println("No modules need tagging")
		return nil
	}

	// Topological sort: release dependencies before dependents
	ordered := topoSort(modules)

	for _, name := range ordered {
		if err := releaseOne(ctx, root, name, version, dryRun); err != nil {
			return err
		}
	}
	return nil
}

func workspaceDeps(goModPath string) []string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil
	}
	var deps []string
	for _, req := range f.Require {
		if strings.HasPrefix(req.Mod.Path, modulePrefix) {
			deps = append(deps, strings.TrimPrefix(req.Mod.Path, modulePrefix))
		}
	}
	return deps
}

func checkDepsPublished(root, dir string) []string {
	modPath := filepath.Join(dir, "go.mod")
	deps := workspaceDeps(modPath)
	var warnings []string
	for _, dep := range deps {
		depDir, err := resolveRepoDir(root, dep)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(depDir, ".git")); err != nil {
			continue
		}
		// Check if HEAD is tagged
		tag := gitOutput(depDir, "describe", "--exact-match", "--tags", "HEAD")
		if tag == "" {
			warnings = append(warnings, fmt.Sprintf("dependency %s has untagged commits at HEAD", dep))
		}
	}
	return warnings
}

func topoSort(modules []releaseModule) []string {
	byName := make(map[string]releaseModule)
	for _, m := range modules {
		byName[m.name] = m
	}

	visited := make(map[string]bool)
	var order []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		if m, ok := byName[name]; ok {
			for _, dep := range m.deps {
				visit(dep)
			}
		}
		order = append(order, name)
	}

	for _, m := range modules {
		visit(m.name)
	}
	return order
}

type releaseModule struct {
	name string
	deps []string
}

const releaseNotesPrompt = `You are writing release notes for a Go module in the lamina workspace. These notes appear on GitHub releases and should match the voice of generativeplane.com — direct, technically precise, no filler, no corporate polish. The author builds tools for sovereign compute on Apple Silicon.

Voice rules:
- Short declarative sentences. State what changed and why it matters.
- Technical specifics over vague descriptions. Name the types, interfaces, and patterns.
- No marketing language, no "exciting", no "we're pleased to announce".
- No bullet-point walls. A few sentences or a short paragraph is better.
- If the change is trivial (docs-only, formatting), say so plainly in one sentence. Don't dress it up.
- If the change is substantial, explain what it enables — not just what was modified.

Format rules:
- Do not include the version number as a heading — the caller adds that.
- Output only the markdown body, no fences.
- For trivial changes, one or two sentences is enough. Don't pad.
- For substantial changes, a short paragraph followed by specifics if needed.`

// releaseNotes generates release notes using an LLM, falling back to the
// deterministic commit-grouping format if no LLM is configured.
func releaseNotes(ctx context.Context, dir, name, version, commitLog, diff string) string {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return generateReleaseNotes(version, commitLog)
	}

	apiKey, err := cfg.NotesAPIKey()
	if err != nil {
		return generateReleaseNotes(version, commitLog)
	}

	client, err := newLLMClient(cfg.NotesProvider(), cfg.NotesBaseURL(), apiKey)
	if err != nil {
		return generateReleaseNotes(version, commitLog)
	}

	// Read module description from AGENTS.md first paragraph
	moduleDesc := readModuleDescription(dir)

	var userMsg strings.Builder
	fmt.Fprintf(&userMsg, "Module: %s\nVersion: %s\n", name, version)
	if moduleDesc != "" {
		fmt.Fprintf(&userMsg, "\nWhat this module does:\n%s\n", moduleDesc)
	}
	fmt.Fprintf(&userMsg, "\nCommit log:\n%s\n", commitLog)
	fmt.Fprintf(&userMsg, "\nDiff:\n%s\n", diff)

	req := &talk.Request{
		Model: cfg.NotesModel(),
		Messages: []talk.Message{
			{Role: talk.RoleSystem, Content: releaseNotesPrompt},
			{Role: talk.RoleUser, Content: userMsg.String()},
		},
	}

	result, err := loop.Run(ctx, loop.RunConfig{
		Client:  client,
		Request: req,
	})
	if err != nil {
		fmt.Printf("  warning: LLM release notes failed, using commit log: %v\n", err)
		return generateReleaseNotes(version, commitLog)
	}

	notes := strings.TrimSpace(result.Content)
	if notes == "" {
		return generateReleaseNotes(version, commitLog)
	}
	return fmt.Sprintf("## What's new in %s\n\n%s\n", version, notes)
}

// readModuleDescription extracts the first paragraph from AGENTS.md (after the heading).
func readModuleDescription(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	var para []string
	started := false
	for _, line := range lines {
		// Skip heading lines
		if strings.HasPrefix(line, "#") {
			if started {
				break // hit the next section
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if started {
				break // end of first paragraph
			}
			continue
		}
		started = true
		para = append(para, trimmed)
	}
	return strings.Join(para, " ")
}

// generateReleaseNotes formats a git log into grouped release notes markdown.
// Used as a fallback when no LLM is configured.
func generateReleaseNotes(version, commitLog string) string {
	var features, fixes, other []string

	for _, line := range strings.Split(commitLog, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split "hash message" — take everything after the first space
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		msg := parts[1]

		switch {
		case strings.HasPrefix(msg, "feat:") || strings.HasPrefix(msg, "feat("):
			msg = strings.TrimPrefix(msg, "feat: ")
			msg = strings.TrimPrefix(msg, "feat(")
			if i := strings.Index(msg, "):"); i >= 0 {
				msg = strings.TrimSpace(msg[i+2:])
			}
			features = append(features, msg)
		case strings.HasPrefix(msg, "fix:") || strings.HasPrefix(msg, "fix("):
			msg = strings.TrimPrefix(msg, "fix: ")
			msg = strings.TrimPrefix(msg, "fix(")
			if i := strings.Index(msg, "):"); i >= 0 {
				msg = strings.TrimSpace(msg[i+2:])
			}
			fixes = append(fixes, msg)
		default:
			// Strip prefix for display: "docs: foo" → "foo"
			if i := strings.Index(msg, ": "); i >= 0 {
				msg = msg[i+2:]
			}
			other = append(other, msg)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## What's new in %s\n", version)

	if len(features) > 0 {
		b.WriteString("\n### Features\n")
		for _, f := range features {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	if len(fixes) > 0 {
		b.WriteString("\n### Fixes\n")
		for _, f := range fixes {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	if len(other) > 0 {
		b.WriteString("\n### Other\n")
		for _, o := range other {
			fmt.Fprintf(&b, "- %s\n", o)
		}
	}

	return b.String()
}

func runDocReview(ctx context.Context, dir, name, oldTag, newTag string, dryRun bool) {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil || !cfg.LLMConfigured() {
		return
	}

	apiKey, err := cfg.LLM.APIKey()
	if err != nil {
		fmt.Printf("  warning: LLM configured but API key unavailable: %v\n", err)
		return
	}

	llm, err := newLLMClient(cfg.LLM.Provider, cfg.LLM.BaseURL, apiKey)
	if err != nil {
		fmt.Printf("  warning: could not create LLM client: %v\n", err)
		return
	}

	var writer docreview.Writer
	if dryRun {
		writer = &docreview.DryRunWriter{Dir: dir}
	} else {
		writer = &docreview.FileWriter{Dir: dir}
	}

	engine := docreview.NewEngine(llm, cfg.LLM.Model, dir, writer)

	fmt.Printf("\nReviewing documentation for %s...\n", name)
	_, err = engine.Review(ctx, name, oldTag, newTag, func(token string) {
		fmt.Print(token)
	})
	if err != nil {
		fmt.Printf("\n  warning: doc review failed: %v\n", err)
		return
	}
	fmt.Println()

	if dryRun {
		if dw, ok := writer.(*docreview.DryRunWriter); ok && len(dw.Changes) > 0 {
			fmt.Println("\n[dry-run] proposed doc changes:")
			for _, c := range dw.Changes {
				fmt.Printf("  would write %s (%d bytes)\n", c.Path, len(c.Content))
			}
		}
	} else {
		// Check if docs were modified and commit
		if st := gitOutput(dir, "status", "--porcelain"); st != "" {
			fmt.Printf("Committing doc updates for %s...\n", name)
			_ = runGit(dir, "add", "README.md", "AGENTS.md")
			_ = runGit(dir, "commit", "-m", fmt.Sprintf("docs: update documentation after %s release", newTag))
			_ = runGit(dir, "push", "origin", "main")
		}
	}
}

func backfillReleaseNotes(root string, dryRun bool) error {
	// Collect all repo directories from root and apps/
	type entry struct {
		name string
		dir  string
	}
	var repos []entry

	scanDirs := []string{root}
	appsDir := filepath.Join(root, "apps")
	if _, err := os.Stat(appsDir); err == nil {
		scanDirs = append(scanDirs, appsDir)
	}
	for _, scanDir := range scanDirs {
		entries, err := os.ReadDir(scanDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(scanDir, e.Name())
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
				continue
			}
			repos = append(repos, entry{name: e.Name(), dir: dir})
		}
	}

	var created int
	for _, r := range repos {
		// Get all tags in chronological order
		tagsOut := gitOutput(r.dir, "tag", "--sort=version:refname")
		if tagsOut == "" {
			continue
		}
		tags := strings.Split(tagsOut, "\n")

		// Check which tags have GitHub releases
		existingReleases := gitOutput(r.dir, "ls-remote", "--tags", "origin")
		_ = existingReleases // tags exist remotely, but we need to check gh releases

		for i, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" || !semverRe.MatchString(tag) {
				continue
			}

			// Check if GitHub release exists
			ghCheck := exec.Command("gh", "release", "view", tag)
			ghCheck.Dir = r.dir
			if err := ghCheck.Run(); err == nil {
				continue // release already exists
			}

			// Generate notes from commit log
			var logRange string
			if i > 0 {
				logRange = tags[i-1] + ".." + tag
			} else {
				logRange = tag
			}
			commitLog := gitOutput(r.dir, "log", "--oneline", logRange)
			notes := generateReleaseNotes(tag, commitLog)

			if dryRun {
				fmt.Printf("[dry-run] would create release %s/%s\n", r.name, tag)
				continue
			}

			fmt.Printf("Creating release %s/%s...\n", r.name, tag)
			ghCmd := exec.Command("gh", "release", "create", tag, "--title", tag, "--notes", notes)
			ghCmd.Dir = r.dir
			ghCmd.Stdout = os.Stdout
			ghCmd.Stderr = os.Stderr
			if err := ghCmd.Run(); err != nil {
				fmt.Printf("  warning: failed to create release %s/%s: %v\n", r.name, tag, err)
				continue
			}
			created++
		}
	}

	if created > 0 {
		fmt.Printf("\nCreated %d GitHub releases\n", created)
	} else if !dryRun {
		fmt.Println("All tags already have GitHub releases")
	}
	return nil
}

func newLLMClient(provider, baseURL, apiKey string) (talk.LLMClient, error) {
	switch provider {
	case "anthropic":
		return anthropic.NewClient("https://api.anthropic.com", apiKey), nil
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return openai.NewClient(baseURL, apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q (supported: anthropic, openai)", provider)
	}
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stripAppReplaces removes all workspace replace directives from an app's go.mod
// and pins require versions to the latest published tag for each dependency.
func stripAppReplaces(root, appDir string) error {
	modPath := filepath.Join(appDir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}
	f, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return err
	}

	for _, rep := range f.Replace {
		// Only strip local (relative path) replaces for workspace modules
		if !strings.HasPrefix(rep.New.Path, ".") && !strings.HasPrefix(rep.New.Path, "/") {
			continue
		}

		// Before dropping the replace, pin the require to the latest published tag
		if strings.HasPrefix(rep.Old.Path, modulePrefix) {
			depName := strings.TrimPrefix(rep.Old.Path, modulePrefix)
			depDir, err := resolveRepoDir(root, depName)
			if err == nil {
				latestTag := gitOutput(depDir, "describe", "--tags", "--abbrev=0")
				if latestTag != "" {
					if err := f.AddRequire(rep.Old.Path, latestTag); err != nil {
						fmt.Fprintf(os.Stderr, "    warning: could not pin %s to %s: %v\n", depName, latestTag, err)
					}
				}
			}
		}

		if err := f.DropReplace(rep.Old.Path, rep.Old.Version); err != nil {
			return fmt.Errorf("dropping replace for %s: %w", rep.Old.Path, err)
		}
	}

	f.Cleanup()
	out, err := f.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(modPath, out, 0644)
}

// restoreAppReplaces restores replace directives by re-wiring them from the workspace.
// Uses git checkout to restore go.mod to the pre-strip state, then tidies.
func restoreAppReplaces(appDir string) error {
	if err := runGit(appDir, "checkout", "HEAD~1", "--", "go.mod", "go.sum"); err != nil {
		return err
	}
	_ = runGit(appDir, "add", "go.mod", "go.sum")
	return runGit(appDir, "commit", "-m", "release: restore replace directives for local development")
}
