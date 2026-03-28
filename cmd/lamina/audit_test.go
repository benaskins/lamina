package main

import (
	"testing"
)

func TestClassifyVulnFindings(t *testing.T) {
	findings := []vulnFinding{
		{ID: "GO-2025-4251", Module: "github.com/ollama/ollama", FixedIn: ""},
		{ID: "GO-2025-3553", Module: "github.com/golang-jwt/jwt/v5", FixedIn: "v5.2.2"},
		{ID: "GO-2026-4601", Module: "net/url", FixedIn: "go1.26.1"},
	}

	fixable, unfixable := classifyVulns(findings)

	if len(fixable) != 2 {
		t.Errorf("expected 2 fixable, got %d", len(fixable))
	}
	if len(unfixable) != 1 {
		t.Errorf("expected 1 unfixable, got %d", len(unfixable))
	}
	if unfixable[0].ID != "GO-2025-4251" {
		t.Errorf("expected unfixable to be GO-2025-4251, got %s", unfixable[0].ID)
	}
}

func TestParseGovulncheckJSON(t *testing.T) {
	// govulncheck -json outputs one JSON object per line, with different "message" types
	jsonOutput := `{"config":{"protocol_version":"v1.0.0"}}
{"progress":{"message":"Scanning your code and 47 packages across 12 dependent modules for known vulnerabilities..."}}
{"finding":{"osv":"GO-2025-3553","fixed_version":"v5.2.2","trace":[{"module":"github.com/golang-jwt/jwt/v5","version":"v5.2.1","package":"github.com/golang-jwt/jwt/v5","function":"Parser.ParseUnverified"}]}}
{"finding":{"osv":"GO-2025-4251","trace":[{"module":"github.com/ollama/ollama","version":"v0.17.6","package":"github.com/ollama/ollama/api","function":"Client.Chat"}]}}
`
	findings := parseGovulncheckJSON([]byte(jsonOutput))

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	if findings[0].ID != "GO-2025-3553" {
		t.Errorf("expected GO-2025-3553, got %s", findings[0].ID)
	}
	if findings[0].FixedIn != "v5.2.2" {
		t.Errorf("expected fixed_version v5.2.2, got %q", findings[0].FixedIn)
	}
	if findings[0].Module != "github.com/golang-jwt/jwt/v5" {
		t.Errorf("expected module github.com/golang-jwt/jwt/v5, got %q", findings[0].Module)
	}

	if findings[1].ID != "GO-2025-4251" {
		t.Errorf("expected GO-2025-4251, got %s", findings[1].ID)
	}
	if findings[1].FixedIn != "" {
		t.Errorf("expected empty fixed_version, got %q", findings[1].FixedIn)
	}
}

func TestParseGosecJSON(t *testing.T) {
	jsonOutput := `{"Issues":[{"severity":"HIGH","confidence":"HIGH","cwe":{"id":"327"},"rule_id":"G401","details":"Use of weak cryptographic primitive","file":"/tmp/foo.go","line":"42","column":"5","code":"md5.New()"}]}`

	findings := parseGosecJSON([]byte(jsonOutput))

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "G401" {
		t.Errorf("expected G401, got %s", findings[0].RuleID)
	}
	if findings[0].File != "/tmp/foo.go" {
		t.Errorf("expected /tmp/foo.go, got %s", findings[0].File)
	}
}

func TestParseStaticcheckJSON(t *testing.T) {
	jsonOutput := `{"code":"SA1012","message":"do not pass a nil Context","location":{"file":"/tmp/foo_test.go","line":269,"column":14},"end":{"file":"","line":0,"column":0},"severity":"error"}
{"code":"S1001","message":"should use copy(to, from)","location":{"file":"/tmp/bar.go","line":226,"column":2},"end":{"file":"","line":0,"column":0},"severity":"error"}
`
	findings := parseStaticcheckJSON([]byte(jsonOutput))

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Code != "SA1012" {
		t.Errorf("expected SA1012, got %s", findings[0].Code)
	}
	if findings[1].Code != "S1001" {
		t.Errorf("expected S1001, got %s", findings[1].Code)
	}
}

func TestAuditResult_Pass(t *testing.T) {
	r := auditResult{}
	if err := r.check(); err != nil {
		t.Errorf("empty audit should pass, got: %v", err)
	}
}

func TestAuditResult_FailOnFixableVulns(t *testing.T) {
	r := auditResult{
		fixableVulns: []vulnFinding{{ID: "GO-2025-3553", FixedIn: "v5.2.2"}},
	}
	if err := r.check(); err == nil {
		t.Error("expected error for fixable vulns")
	}
}

func TestAuditResult_WarnOnUnfixableVulns(t *testing.T) {
	r := auditResult{
		unfixableVulns: []vulnFinding{{ID: "GO-2025-4251"}},
	}
	if err := r.check(); err != nil {
		t.Errorf("unfixable vulns should warn not block, got: %v", err)
	}
}

func TestAuditResult_FailOnGosec(t *testing.T) {
	r := auditResult{
		gosecFindings: []gosecFinding{{RuleID: "G401"}},
	}
	if err := r.check(); err == nil {
		t.Error("expected error for gosec findings")
	}
}

func TestAuditResult_FailOnStaticcheck(t *testing.T) {
	r := auditResult{
		staticcheckFindings: []staticcheckFinding{{Code: "SA1012"}},
	}
	if err := r.check(); err == nil {
		t.Error("expected error for staticcheck findings")
	}
}
