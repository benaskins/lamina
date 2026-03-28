package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit [module]",
	Short: "Run security analysis on a workspace module",
	Long: `Run govulncheck, gosec, and staticcheck against a module.

If no module is specified, audits the lamina root module.

Fixable vulnerabilities (with a known fix version) cause a failure.
Unfixable vulnerabilities (no upstream fix) are printed as warnings.
Any gosec or staticcheck finding causes a failure.

Examples:
  lamina audit              Audit the lamina root module
  lamina audit axon-chat    Audit a specific module`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAudit,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	root, err := workspaceRoot()
	if err != nil {
		return err
	}

	var dir string
	if len(args) == 0 {
		dir = root
	} else {
		dir, err = resolveRepoDir(root, args[0])
		if err != nil {
			return fmt.Errorf("%q is not a git repo in the workspace", args[0])
		}
	}

	result := runSecurityAudit(dir)
	result.print(dir)
	return result.check()
}

// runSecurityAudit runs govulncheck, gosec, and staticcheck on a module directory.
func runSecurityAudit(dir string) auditResult {
	var result auditResult

	// govulncheck
	out, err := runTool(dir, "govulncheck", "-json", "./...")
	if err != nil && len(out) == 0 {
		fmt.Fprintf(os.Stderr, "  warning: govulncheck failed to run: %v\n", err)
	} else {
		findings := parseGovulncheckJSON(out)
		result.fixableVulns, result.unfixableVulns = classifyVulns(findings)
	}

	// gosec
	out, err = runTool(dir, "gosec", "-exclude=G104,G706", "-fmt", "json", "-quiet", "./...")
	if err != nil && len(out) == 0 {
		fmt.Fprintf(os.Stderr, "  warning: gosec failed to run: %v\n", err)
	} else {
		result.gosecFindings = parseGosecJSON(out)
	}

	// staticcheck
	out, err = runTool(dir, "staticcheck", "-f", "json", "./...")
	if err != nil && len(out) == 0 {
		fmt.Fprintf(os.Stderr, "  warning: staticcheck failed to run: %v\n", err)
	} else {
		result.staticcheckFindings = parseStaticcheckJSON(out)
	}

	return result
}

func runTool(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return out, err
}

// --- types ---

type vulnFinding struct {
	ID     string
	Module string
	FixedIn string
}

type gosecFinding struct {
	RuleID   string
	Severity string
	Details  string
	File     string
	Line     string
}

type staticcheckFinding struct {
	Code    string
	Message string
	File    string
	Line    int
}

type auditResult struct {
	fixableVulns        []vulnFinding
	unfixableVulns      []vulnFinding
	gosecFindings       []gosecFinding
	staticcheckFindings []staticcheckFinding
}

func (r *auditResult) check() error {
	var problems []string
	if len(r.fixableVulns) > 0 {
		problems = append(problems, fmt.Sprintf("%d fixable vulnerabilities", len(r.fixableVulns)))
	}
	if len(r.gosecFindings) > 0 {
		problems = append(problems, fmt.Sprintf("%d gosec findings", len(r.gosecFindings)))
	}
	if len(r.staticcheckFindings) > 0 {
		problems = append(problems, fmt.Sprintf("%d staticcheck findings", len(r.staticcheckFindings)))
	}
	if len(problems) > 0 {
		return fmt.Errorf("security audit failed: %s", strings.Join(problems, ", "))
	}
	return nil
}

func (r *auditResult) print(dir string) {
	name := dir
	fmt.Printf("Security audit: %s\n", name)

	if len(r.fixableVulns) > 0 {
		fmt.Printf("  FAIL  %d fixable vulnerabilities:\n", len(r.fixableVulns))
		for _, v := range r.fixableVulns {
			fmt.Printf("         %s in %s (fix: %s)\n", v.ID, v.Module, v.FixedIn)
		}
	}
	if len(r.unfixableVulns) > 0 {
		fmt.Printf("  WARN  %d unfixable vulnerabilities (no upstream fix):\n", len(r.unfixableVulns))
		for _, v := range r.unfixableVulns {
			fmt.Printf("         %s in %s\n", v.ID, v.Module)
		}
	}
	if len(r.gosecFindings) > 0 {
		fmt.Printf("  FAIL  %d gosec findings:\n", len(r.gosecFindings))
		for _, f := range r.gosecFindings {
			fmt.Printf("         %s %s (%s:%s)\n", f.RuleID, f.Details, f.File, f.Line)
		}
	}
	if len(r.staticcheckFindings) > 0 {
		fmt.Printf("  FAIL  %d staticcheck findings:\n", len(r.staticcheckFindings))
		for _, f := range r.staticcheckFindings {
			fmt.Printf("         %s %s (%s:%d)\n", f.Code, f.Message, f.File, f.Line)
		}
	}

	if r.check() == nil {
		if len(r.unfixableVulns) > 0 {
			fmt.Println("  PASS  (with warnings)")
		} else {
			fmt.Println("  PASS")
		}
	}
}

// --- parsers ---

func parseGovulncheckJSON(data []byte) []vulnFinding {
	var findings []vulnFinding
	seen := make(map[string]bool)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg struct {
			Finding *struct {
				OSV          string `json:"osv"`
				FixedVersion string `json:"fixed_version"`
				Trace        []struct {
					Module  string `json:"module"`
					Version string `json:"version"`
				} `json:"trace"`
			} `json:"finding"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Finding == nil {
			continue
		}
		if seen[msg.Finding.OSV] {
			continue
		}
		seen[msg.Finding.OSV] = true

		f := vulnFinding{
			ID:      msg.Finding.OSV,
			FixedIn: msg.Finding.FixedVersion,
		}
		if len(msg.Finding.Trace) > 0 {
			f.Module = msg.Finding.Trace[0].Module
		}
		findings = append(findings, f)
	}
	return findings
}

func parseGosecJSON(data []byte) []gosecFinding {
	var report struct {
		Issues []struct {
			RuleID   string `json:"rule_id"`
			Severity string `json:"severity"`
			Details  string `json:"details"`
			File     string `json:"file"`
			Line     string `json:"line"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil
	}

	findings := make([]gosecFinding, len(report.Issues))
	for i, issue := range report.Issues {
		findings[i] = gosecFinding{
			RuleID:   issue.RuleID,
			Severity: issue.Severity,
			Details:  issue.Details,
			File:     issue.File,
			Line:     issue.Line,
		}
	}
	return findings
}

func parseStaticcheckJSON(data []byte) []staticcheckFinding {
	var findings []staticcheckFinding
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			Code     string `json:"code"`
			Message  string `json:"message"`
			Location struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"location"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Code == "" {
			continue
		}
		findings = append(findings, staticcheckFinding{
			Code:    entry.Code,
			Message: entry.Message,
			File:    entry.Location.File,
			Line:    entry.Location.Line,
		})
	}
	return findings
}

func classifyVulns(findings []vulnFinding) (fixable, unfixable []vulnFinding) {
	for _, f := range findings {
		if f.FixedIn != "" {
			fixable = append(fixable, f)
		} else {
			unfixable = append(unfixable, f)
		}
	}
	return
}
