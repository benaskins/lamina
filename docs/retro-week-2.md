# Retro: Feb 19 – Mar 6, 2026

Fifteen calendar days. 14 repos. ~400 commits across all repos. One human, one AI partner, evenings and weekends.

---

## Timeline

### Feb 19–22: aurelia (103 commits)

The foundation. Four days produced a complete process supervisor: YAML spec parsing, native process driver with supervision, health checks (HTTP/TCP/exec), REST API with daemon orchestrator, CLI, Keychain secrets, Docker container driver, GPU observability via cgo Metal/IOKit, dependency-ordered startup/shutdown, crash recovery with state persistence, LaunchAgent integration, Traefik routing generation, blue-green deploys, external service type, zero-downtime daemon restart via SIGTERM orphaning, and a full security hardening pass (file permissions, PID reuse protection, atomic writes, input validation, fuzz tests).

This was the most intense period. The git log shows multiple fix/refactor commits per day — a rapid build-test-harden cycle.

### Feb 24–26: aurelia polish + axon born

Aurelia stabilised: slop-guard added as pre-commit hook, JSON output, bootstrap script, driver leak fixes. On Feb 26, axon was created — absorbing servicekit into a shared toolkit with auth middleware, SSE, metrics, database migrations, and server lifecycle. Immediately hardened: SQL injection fix, health 503, shutdown hooks. pgx v5 migration same day.

### Feb 28: Service extraction

Four services extracted from aurelia-core-infrastructure into independent repos in a single day:
- **axon-chat** — chat library from the monolith
- **axon-auth** — WebAuthn/passkey authentication
- **axon-task** — task runner
- **axon-gate** — deploy approval gate

Each got its own go.mod, tests, and README. The AURELIA_ROOT forwarding to launchd was also added this day.

### Mar 1: The decomposition

The biggest architectural day. axon-chat was decomposed into three layers per the design doc:
- **axon-tool** — tool primitives (PageFetcher, ToolRouter, SearchQualifier, SearXNG, OpenMeteo)
- **axon-agent** (later axon-loop) — provider-agnostic conversation loop
- **axon-photo** (later axon-lens) — image generation tools

axon-chat was then refactored to use these: Ollama adapter created, streamChat replaced with agent.Run, photo/image code stripped out, de-companioned for open source.

### Mar 2: Memory service + internal APIs

axon-memo built from scratch: types, interfaces, LLM parsing, extractor, consolidator, retriever, scheduler, HTTP handlers. Internal communication patterns established: axon.InternalClient, axon.DecodeJSON. Chat service got internal endpoints for memory service decoupling and ExtraTools extension point.

### Mar 3–4: Analytics + evaluation

**axon-look**: ClickHouse HTTP client, event ingestion, query API, SvelteKit analytics dashboard with run filtering and eval dashboard pages. SQL injection fix same day.

**axon-eval**: YAML test plan loader, programmatic grading, LLM-as-judge via OllamaJudge, conversation management, analytics event emission. Full evaluation framework from zero in ~1 day.

**axon-chat**: Sync chat endpoint added (for eval), memory integration (recall_memory tool, idle extraction, MemoryClient), analytics event emission, X-Axon-Run-Id propagation.

### Mar 5: Open source day

The big rename: axon-agent → axon-loop, axon-photo → axon-lens, axon-test → axon-eval, axon-anal → axon-look. All four-letter names. ChatClient → LLMClient, ChatRequest/Response → Request/Response. Skills → Tools in axon-chat frontend and API.

All repos: Apache 2.0 → MIT, replace directives removed, published to Go module proxy. Security fixes across auth (SameSite, user enumeration, input validation), chat (PageFetcher race), eval (agent slug, error handling). README/CLAUDE.md rewrites across the board.

### Mar 6: FLUX.1 migration + housekeeping

axon-lens: ImageGenerator interface, FluxGenerator (flux.swift CLI), ImageWorker, thumbnail generation. axon-task: genericised executor with Worker interface. Composition root in aurelia-core-infrastructure rewired from ComfyUI to FLUX.1.

axon-memo: Mayer ABI trust model replaced ad-hoc relationship dimensions. Durable memories, CLI, design principles.

Dotfiles overhauled for Go+Swift+SvelteKit stack. Slop-guard audit across all repos (clean). 17 issues triaged across workspace — 10 closed, 3 migrated, rest commented. Blog post drafted.

---

## Original Roadmap Status

From `aurelia-core-infrastructure/docs/plans/roadmap.md`:

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Memory System — Chat Integration | **Done** | Recall, extraction, consolidation all wired. Idle timeout extraction, recall_memory tool. Mar 2–4. |
| 2 | Nomad Update Strategies | **Superseded** | Nomad was replaced by aurelia itself. aurelia handles restarts, health checks, dependency ordering, crash recovery natively. |
| 3 | TUI Tool Use Status | **Partially done** | SSE events emit tool invocations. Frontend shows tool use. Status labels may not be fully wired — needs verification. |
| 4 | Vault Database Credentials | **Not started** | DATABASE_URL is still hardcoded in service specs. Vault is running but credential injection isn't wired. |
| 5 | NL MIDI Interface | **Not started** | No work done. Original design assumed Nomad. Would need redesign for aurelia. |

---

## What Went Well

**Velocity through decomposition.** The decision to split into small, focused repos paid off immediately. Each module is small enough for an agent to hold in context. The Mar 5 rename day — touching every repo — would have been terrifying in a monorepo. With independent repos and clear interfaces, it was routine.

**Architecture-first approach.** The three-layer mental model (at rest / in flight / building material) guided every decision. When in doubt about where code belongs, the model answers it. This is why the decomposition on Mar 1 went smoothly — the boundaries were already conceptually clear.

**Security hardening as you go.** Every extraction included a security pass: path traversal fixes, input validation, type assertion safety, race condition fixes. The code review issues from Mar 4 caught real bugs, and most were fixed within 48 hours.

**The rename discipline.** Renaming early (day 15) rather than living with wrong names was the right call. axon-agent → axon-loop is a better name. Skills → Tools is correct domain language. The cost of renaming goes up exponentially with time.

## What Could Be Better

**Test coverage is uneven.** axon-chat has significant gaps (no tests for tool_router, searxng, weather, fetch_page, conversation handlers). axon-task has zero tests for handler.go and postgres_store.go. The code review issues flagged this consistently.

**Documentation lags implementation.** The project-scale doc, README, and CLAUDE.md all needed updates after the fact. Design docs exist for early work but not for later features (analytics, eval framework).

**No end-to-end testing.** axon-eval exists but hasn't been run against the live cluster since the FLUX.1 migration. The flux.swift binary needs HF token setup before image generation works.

**Signing key path.** The git signing key in aurelia-core-infrastructure still references the old lamina child path. Needs a permanent fix in gitconfig, not the `-c` workaround.

**Unpublished changes accumulate.** lamina doctor found axon-gate and axon-memo with unpublished commits after today's fixes. The tagging discipline is good but needs to be habitual after every fix session.

---

## Open Issues Summary (22 total)

| Category | Count | Repos |
|----------|-------|-------|
| Aurelia bugs/improvements | 9 | aurelia |
| Security/auth design decisions | 3 | axon-auth (2), axon-gate (1) |
| Code quality/test coverage | 7 | axon-chat, axon-eval, axon-look, axon-loop, axon-memo, axon-task, axon-tool |
| Middleware/infra design | 1 | axon |
| Image pipeline hardening | 1 | axon-lens |
| Blocked on external setup | 1 | lamina-mono #7 (HF token) |

---

## Lamina 1.0 — Light on the Hill

Lamina 1.0 is the point where a single developer can clone the workspace, run `lamina init && lamina up`, and have a fully operational personal compute platform within an hour on first use. After a power cycle, everything recovers within minutes. The system should be self-healing, observable, and ready for agents to operate autonomously.

### What 1.0 means

1. **Stable module APIs** — all axon-* modules at v1.0.0 with documented interfaces and no breaking changes expected
2. **Self-hosting** — `lamina init && lamina up` brings the full stack online (aurelia manages the rest)
3. **Observable** — every service emits metrics, logs, and traces; analytics dashboard shows system health
4. **Agent-ready** — agents can recall memory, submit tasks, generate images, search the web, and coordinate work through axon-plan
5. **Documented** — each module has a README that a new developer (or agent) can reason from independently

### Capabilities needed for 1.0

| Capability | Module | Status | Priority |
|-----------|--------|--------|----------|
| Process supervision | aurelia | Done | — |
| Shared toolkit | axon | Done | — |
| Authentication | axon-auth | Done (needs /internal auth) | Medium |
| Chat + agents | axon-chat | Done (needs test coverage) | Medium |
| Tool primitives | axon-tool | Done | — |
| Conversation loop | axon-loop | Done (needs ToolCallID) | Low |
| LLM providers | axon-talk | Done (Ollama only) | — |
| Image generation | axon-lens | Done (needs HF token setup) | High |
| Analytics | axon-look | Done | — |
| Long-term memory | axon-memo | Done (needs consolidation fixes) | Medium |
| Task runner | axon-task | Done | — |
| Deploy gate | axon-gate | Done | — |
| Evaluation | axon-eval | Done | — |
| Workspace CLI | lamina | Done | — |
| Agent coordination | axon-plan | Not started | High |
| Vault credential injection | aurelia + services | Not started | Medium |
| `lamina up` (full stack bootstrap) | lamina + aurelia | Not started | High |
| Second LLM provider | axon-talk | Not started (Anthropic?) | Medium |
| MIDI interface | new module | Not started | Low |

### Proposed next capabilities (beyond current issues)

**axon-plan** — The design doc exists. This is the missing piece for multi-agent coordination. Without it, agents can only work on what you tell them. With it, they can discover goals, claim tasks, and coordinate. Start with a CLI (`plan goal`, `plan task`, `plan claim`) and HTTP API, add event-driven discovery later.

**lamina up** — A single command that brings the entire stack online: starts aurelia daemon, ensures containers are running (postgres, clickhouse, etc.), checks health of all services, reports what's ready and what's not. This is the difference between "a collection of repos" and "a platform."

**Second LLM provider** — axon-talk currently only has Ollama. Adding an Anthropic adapter (Claude API) would validate the provider abstraction and enable cloud/local hybrid operation. The interface is clean — this should be straightforward.

**Vault integration** — DATABASE_URL and other secrets are still hardcoded in service specs. Vault is running. Wire it up so credentials are injected at service start time via aurelia's environment variable mechanism.

---

## Monday Recommendations

### Must do

1. **Tag axon-gate and axon-memo** — both have unpublished commits from today's fixes. Quick: `lamina release axon-gate v0.1.2` and `lamina release axon-memo v0.2.1`.

2. **Fix git signing key path** — the `-c user.signingkey=` workaround is fragile. Update the gitconfig in aurelia-core-infrastructure permanently.

3. **Set up HF token for flux.swift** — this is blocking the image generation pipeline. Get a token from huggingface.co, accept the FLUX.1-schnell license, set HF_TOKEN in the task-runner environment. Then close issue #7.

### Should do

4. **Start axon-plan** — the design doc is ready. Begin with the domain types (Goal, Task) and a CLI. This is the highest-value new capability and feeds directly into the Lamina 1.0 vision.

5. **axon-auth #2** — decide on the auth model for `/internal/service-user`. Four options were proposed in the issue comment. Pick one and implement it. This is a real security gap.

6. **Test coverage sprint** — the code review issues consistently flagged test gaps. Pick the highest-risk gaps: axon-chat handler tests, axon-task handler/store tests, axon-memo consolidator tests.

### Could do

7. **`lamina up` prototype** — even a simple version that checks aurelia status and starts missing services would be valuable.

8. **Vault credential injection** — roadmap item #4, still relevant.

9. **Publish blog post** — the Will Larson draft is ready for your review and polish.
