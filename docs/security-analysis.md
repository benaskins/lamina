# Lamina Workspace — Security Threat Analysis

> **Date**: 2026-03-25
> **Scope**: All subrepos (aurelia, axon, axon-auth, axon-chat, axon-eval, axon-fact,
> axon-gate, axon-lens, axon-look, axon-loop, axon-memo, axon-mind, axon-nats,
> axon-synd, axon-talk, axon-task, axon-tool, axon-wire)
>
> **Purpose**: Identify vulnerabilities and gaps relevant to commercialisation
> or deployment in a regulated environment (SOC 2, ISO 27001, PCI-DSS, GDPR, HIPAA).

---

## Executive Summary

The codebase demonstrates **strong security fundamentals** — cryptographically
secure token generation, parameterised queries, proper cookie flags, WebAuthn
authentication, and structured secrets management. However, several gaps exist
that would block commercial or regulated deployment. These fall into three
categories:

1. **Exploitable vulnerabilities** (fix immediately)
2. **Architectural gaps** (address before going to production)
3. **Compliance/governance gaps** (required for regulated environments)

---

## 1. Exploitable Vulnerabilities

### 1.1 CRITICAL — Timing Attack on API Key Validation

| | |
|---|---|
| **File** | `axon-auth/handler_service_user.go:33` |
| **Code** | `if h.internalAPIKey == "" \|\| apiKey != h.internalAPIKey` |
| **Impact** | Attackers can brute-force the internal API key by measuring response-time deltas |
| **Fix** | Use `crypto/subtle.ConstantTimeCompare()` |

The same issue exists in the Cloudflare Worker proxy:

| | |
|---|---|
| **File** | `infra/cloudflare/wire-proxy/src/index.ts:23` |
| **Code** | `token !== env.WIRE_TOKEN` |
| **Fix** | Use `crypto.subtle.timingSafeEqual()` |

### 1.2 CRITICAL — Unauthenticated Internal Endpoints

| | |
|---|---|
| **File** | `axon-chat/handler_internal.go:10-30`, `axon-chat/server.go:200-209` |
| **Issue** | `InternalMessagesHandler()` and `InternalAgentHandler()` have **no authentication** — documented as "for internal service-to-service calls" but nothing verifies the caller |
| **Impact** | Any client that can reach the service (SSRF, compromised network, misconfigured firewall) gets full access to all conversations and agent configs |
| **Fix** | Add mTLS, bearer-token validation, or bind to a separate listener on a private interface |

### 1.3 HIGH — Timing Attack on Wire Proxy Token

Already noted in §1.1 — the Cloudflare Worker uses `!==` for token
comparison.

---

## 2. Architectural / Design Gaps

### 2.1 No Rate Limiting on Auth Endpoints

- `axon-auth/handler_login.go`, `handler_register.go`, `handler_service_user.go`
  have no per-IP or per-account rate limiting.
- Aurelia's daemon has a token-bucket limiter (20 req/s sustained, 100 burst)
  but that protects the supervisor API, not the axon HTTP services.
- **Risk**: Credential-stuffing and brute-force attacks against WebAuthn begin/finish.

### 2.2 Session Management Weaknesses

| Gap | Detail |
|-----|--------|
| **No token rotation** | Session tokens are long-lived with no sliding-window refresh or rotation on activity (`axon-auth/`) |
| **365-day service-user sessions** | `axon-auth/server.go:58` — machine tokens live a full year |
| **Cache masks revocation** | `axon/auth.go:167-189` — validated sessions cached for 30 s; a revoked token stays valid until cache expires |
| **Session fixation risk** | WebAuthn ceremony state stored in client-side cookies (`handler_login.go:59-77`); an attacker who pre-sets these cookies can influence the ceremony |

### 2.3 SameSite=Lax (Not Strict)

All session cookies use `SameSite: http.SameSiteLaxMode`. This permits the
cookie to ride along on top-level cross-site navigations, leaving a narrow
CSRF window for state-changing GET endpoints (if any exist).

### 2.4 User Enumeration

`axon-auth/handler_login.go:38-47` returns different error messages for
"user not found" vs "no passkeys registered", allowing attackers to enumerate
valid accounts.

### 2.5 TLS Is Opt-In

`axon/server.go:92-95` falls back to plain HTTP when no TLS config is
provided. All examples (`examples/chat/main.go`, `axon-chat/example/main.go`,
`axon-auth/example/main.go`) use HTTP. Production deployments must enforce TLS
at the reverse-proxy layer (Traefik), but this is not guaranteed by the code.

### 2.6 No CORS Configuration

No CORS headers are set in any axon middleware. This is fine while the SPA is
served from the same origin, but becomes a problem when APIs are consumed by
third-party clients or separate front-ends.

### 2.7 LLM / AI-Specific Risks

| Risk | Status |
|------|--------|
| **Prompt injection** | Tool calls are routed through a validated registry (`axon-loop/run.go:217-231`); unknown tools rejected; panic recovery in place. **Mitigated.** |
| **Tool output re-injection** | Tool results become `RoleTool` messages, not re-injected as system prompts. **Mitigated.** |
| **Agent autonomy limits** | No explicit guardrails on how many tool calls an agent can chain, cost budgets, or human-in-the-loop checkpoints. **Gap.** |
| **Memory poisoning** | `axon-memo` extracts long-term memories from conversations. A crafted prompt could plant false memories that influence future sessions. **Gap.** |

---

## 3. Data Protection & Privacy Gaps

### 3.1 Event Sourcing & Right to Erasure

`axon-fact` stores immutable event streams. Under GDPR Article 17 (right to
erasure) and similar regulations, you must be able to delete or anonymise
personal data on request. An append-only event store with no crypto-shredding
or tombstone mechanism makes this extremely difficult.

### 3.2 PII in Long-Term Memory

`axon-memo` extracts and persists memory fragments from conversations. These
will inevitably contain PII (names, preferences, potentially health or
financial details). There is no:

- Classification of stored memories by sensitivity
- Retention policy or automatic expiry
- Mechanism to enumerate and delete a user's memories on request

### 3.3 Financial Data (axon-book)

Double-entry bookkeeping backed by event sourcing. For PCI-DSS or financial
regulation, journal entries containing account numbers or transaction details
need encryption at rest and field-level access controls — neither is present.

### 3.4 Logging & Sensitive Data Leakage

Structured logging via `slog` is used throughout. The analysis did not find
secrets being logged, but there is no systematic **log scrubbing** or
**redaction framework** to prevent accidental PII/token leakage as the
codebase grows.

### 3.5 No Backup / Disaster Recovery Evidence

No backup configuration, snapshot strategy, or recovery runbook was found in
the codebase or infrastructure configuration.

---

## 4. Supply Chain & Dependency Security

| Check | Status |
|-------|--------|
| Dependencies pinned to exact versions | Yes — all `go.mod` files |
| `go.sum` integrity hashes present | Yes |
| No `replace` directives | Confirmed |
| No deprecated crypto libraries | Confirmed (`golang.org/x/crypto v0.43.0`) |
| GOPRIVATE set for `github.com/benaskins/*` | Yes — bypasses sum DB cache |
| SCA / vulnerability scanning | **Not present** — no `govulncheck`, Dependabot, or Snyk |
| SBOM generation | **Not present** |

---

## 5. What You Need for Commercialisation / Regulated Environments

### 5.1 Immediate Fixes (Pre-Production)

| # | Action | Effort |
|---|--------|--------|
| 1 | Constant-time comparison for all token/key checks | Small |
| 2 | Authenticate internal service-to-service endpoints (mTLS or bearer tokens) | Medium |
| 3 | Add rate limiting middleware to auth and API endpoints | Medium |
| 4 | Implement session token rotation / sliding expiry | Medium |
| 5 | Unify auth error messages to prevent user enumeration | Small |
| 6 | Enforce TLS-only at the application level (reject plain HTTP in production mode) | Small |

### 5.2 Compliance Framework Requirements

#### SOC 2 Type II

- [ ] **Access control policy** — document who can access what, RBAC enforcement
- [ ] **Audit logging** — comprehensive, tamper-evident audit trail (partially done via slog)
- [ ] **Change management** — documented review/approval process for code and infra changes
- [ ] **Incident response plan** — documented procedure for security incidents
- [ ] **Vendor risk management** — inventory and assessment of third-party dependencies
- [ ] **Monitoring & alerting** — real-time detection of anomalous behaviour
- [ ] **Data retention & disposal** — defined lifecycle for all data classes

#### GDPR / Privacy Regulation

- [ ] **Data inventory & mapping** — catalogue all PII flows across services
- [ ] **Right to erasure** — implement crypto-shredding or tombstones in axon-fact event streams
- [ ] **Right to access** — API to export all data held about a user (SAR endpoint)
- [ ] **Memory management** — retention policies, classification, and deletion for axon-memo
- [ ] **Data Processing Agreement (DPA)** — required for LLM provider integrations
- [ ] **Privacy Impact Assessment** — required for AI/profiling features
- [ ] **Consent management** — record and honour user consent for data processing
- [ ] **Cross-border data transfer** — document where data flows (Cloudflare Workers, LLM APIs)

#### PCI-DSS (if handling payments via axon-book)

- [ ] **Encryption at rest** — AES-256 for financial data in PostgreSQL
- [ ] **Field-level access controls** — restrict access to account/transaction details
- [ ] **Network segmentation** — isolate payment-related services
- [ ] **Key management** — HSM or dedicated key management service
- [ ] **Quarterly vulnerability scans** — by Approved Scanning Vendor (ASV)

#### HIPAA (if handling health data via LLM conversations)

- [ ] **Business Associate Agreement** with LLM providers
- [ ] **PHI encryption** — at rest and in transit
- [ ] **Access audit trail** — who accessed what PHI, when
- [ ] **Minimum necessary** — limit PHI exposure to what each service needs

### 5.3 Engineering Infrastructure

| Capability | Current State | Required |
|------------|--------------|----------|
| **Vulnerability scanning** | None | `govulncheck` in CI, Dependabot/Snyk for deps |
| **SBOM** | None | Generate CycloneDX/SPDX on every release |
| **Static analysis (SAST)** | `go vet` only | Add `gosec`, `staticcheck`, semgrep |
| **Secret scanning** | `slop-guard` pre-commit | Add trufflehog/gitleaks in CI |
| **Container scanning** | None | Trivy or Grype for Docker images |
| **Penetration testing** | None | Annual third-party pentest |
| **DAST** | None | Consider OWASP ZAP against staging |
| **Backup & DR** | None evident | Automated backups, tested restore, documented RTO/RPO |
| **Log aggregation** | Loki (infra) | Ensure all services ship logs; add alerting rules |
| **Uptime monitoring** | Health checks in aurelia | External uptime monitoring (e.g., Grafana synthetic) |

### 5.4 AI-Specific Governance

For commercial LLM-powered products, regulators (EU AI Act, NIST AI RMF) increasingly require:

- [ ] **Model card / AI transparency** — document which models are used, for what purpose
- [ ] **Human-in-the-loop** — configurable checkpoints where a human must approve agent actions
- [ ] **Cost / token budgets** — prevent runaway LLM spend from unbounded tool-call loops
- [ ] **Output filtering** — content safety filters on LLM responses before delivery to users
- [ ] **Bias & fairness testing** — if the system makes decisions affecting users
- [ ] **AI incident reporting** — process for logging and reporting AI system failures
- [ ] **Memory audit** — ability to review and correct stored memories (axon-memo)

---

## 6. Threat Model Summary

| Threat Category | Severity | Status |
|----------------|----------|--------|
| **SQL / NoSQL Injection** | Critical | **Mitigated** — parameterised queries throughout |
| **Command Injection** | Critical | **Mitigated** — argument arrays, trusted specs only |
| **XSS** | High | **Mitigated** — html/template auto-escaping, strconv.Quote for JS |
| **Timing Attacks** | High | **Vulnerable** — 2 locations need constant-time comparison |
| **Unauthenticated Internal APIs** | Critical | **Vulnerable** — axon-chat internal endpoints |
| **Brute Force / Credential Stuffing** | High | **Vulnerable** — no rate limiting on auth |
| **Session Hijacking** | Medium | **Partially mitigated** — good cookie flags, but no rotation |
| **CSRF** | Medium | **Partially mitigated** — SameSite=Lax, no CSRF tokens |
| **User Enumeration** | Low | **Vulnerable** — different error messages |
| **Prompt Injection** | High | **Mitigated** — tool registry, panic recovery, role separation |
| **Memory Poisoning** | Medium | **Gap** — no memory integrity validation |
| **Data Erasure (GDPR)** | High | **Gap** — immutable event store, no crypto-shredding |
| **PII Leakage in Logs** | Medium | **Gap** — no redaction framework |
| **Supply Chain Attack** | Medium | **Partially mitigated** — pinned deps, but no SCA scanning |
| **Denial of Service** | Medium | **Partially mitigated** — request size limits, but no rate limiting |

---

## 7. Positive Security Properties

The following are already well-implemented:

- **Token generation**: `crypto/rand` with 256-bit entropy, SHA-256 hashed storage
- **WebAuthn**: Mature `go-webauthn` library, proper credential validation
- **Cookie security**: `HttpOnly`, `Secure` (configurable), `SameSite=Lax`
- **Request body limits**: 1 MB caps on auth and chat endpoints
- **Path traversal protection**: Workspace-root boundary validation in lamina CLI
- **Secrets management**: OpenBao (Vault) with macOS Keychain fallback, audit logging
- **Structured logging**: `slog` throughout with consistent field patterns
- **Wire proxy**: HTTPS enforced for non-localhost, token authentication
- **Dependency hygiene**: Exact version pins, go.sum integrity, no replace directives
- **Process supervision**: Aurelia validates service specs, rate-limits its API, uses 0700 permissions
