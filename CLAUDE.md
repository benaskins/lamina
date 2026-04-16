# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Lamina is a **git repository** that serves as the workspace root for a personal compute cluster running on a Mac Studio (Apple Silicon). It contains:

1. The **`lamina` CLI** (`cmd/lamina/`) — a workspace management tool for coordinating across sub-repos
2. **Claude Code skills** (`skills/`) — embedded skill definitions for workspace operations
3. **Applications** (`apps/`) — workspace apps built on axon modules (imago, revue, valuer, vita)

The workspace is populated by `lamina init`, which clones all sub-repos into this directory. Each sub-repo has its own `.git` and is pushed to GitHub separately (gitignored here).

### Sub-repos (independent git repos, cloned by `lamina init`)

| Repo | Purpose |
|------|---------|
| **aurelia** | Process supervisor for native processes and Docker containers, with macOS enhancements |
| **axon** | Shared Go toolkit for AI-powered web services: HTTP lifecycle, auth, SSE, streaming |
| **axon-auth** | WebAuthn authentication: passkey registration, login, session management |
| **axon-base** | Postgres primitives: pgx pool, repository interfaces, goose migrations, row scanning |
| **axon-book** | Event-sourced double-entry bookkeeping: ledger, chart of accounts, journal entries |
| **axon-chat** | Chat service: LLM integration, tool calling, SSE streaming, agent management |
| **axon-code** | Claude Code integration: structured tool calling from axon-loop agents |
| **axon-cost** | LLM inference cost tracking: middleware wrapping axon-talk, rate tables, budget alerts |
| **axon-eval** | Evaluation framework: scenario plans run against a live service cluster |
| **axon-face** | Frontend component library for axon service UIs |
| **axon-fact** | Event sourcing primitives: Event type, EventStore/Projector/Publisher interfaces |
| **axon-gate** | Deploy approval gate with Signal notifications and review UI |
| **axon-hand** | Shared chassis for factory agents: LLM client config, worker identity, CLI, lifecycle |
| **axon-lens** | Image generation pipeline: prompt merging, FLUX.1 via MLX, gallery storage |
| **axon-look** | Analytics event ingestion and querying backed by ClickHouse |
| **axon-loop** | Provider-agnostic conversation loop for LLM-powered agents |
| **axon-memo** | Long-term memory extraction and consolidation for LLM agents |
| **axon-mind** | Embedded Prolog engine for structured inference over facts and rules |
| **axon-nats** | NATS adapters: EventBus[T] for cross-instance fan-out |
| **axon-push** | Push notification primitives |
| **axon-rule** | Composable business rules and guard-driven state machine (Specification pattern) |
| **axon-scan** | Code quality pipeline: static analysis, security scanning, test execution, attestation |
| **axon-sign** | Cryptographic signing: Ed25519 keypairs, SSHSIG signatures, key rotation, provenance |
| **axon-snip** | Code assembly engine: PRD analysis, module selection, scaffold generation |
| **axon-synd** | Syndication engine: publish to static site, syndicate to Bluesky, Mastodon, Threads |
| **axon-talk** | LLM provider adapters for axon-loop (OpenAI-compatible, Anthropic) |
| **axon-tape** | Buffered token stream filter: pluggable matchers, content safety, PII redaction |
| **axon-task** | Generic async task runner with pluggable workers |
| **axon-tool** | Tool definition and execution primitives for LLM agents |
| **axon-wire** | HTTP transport routing outbound requests through a Cloudflare Worker proxy |

Each sub-repo has its own `CLAUDE.md` or `AGENTS.md`. When working in a sub-repo, read its project-level docs first.

## Three-layer Architecture

```
lamina (at rest)                    aurelia (in flight)
 │                                   │
 ├── repo status, deps, testing      ├── process supervision
 ├── releases, health checks         ├── health checks, restarts
 └── workspace coordination          └── service dependencies
                    │
                    ▼
              axon (building material)
```

- **lamina** manages the workspace as source: repo status, dependency graphs, testing, releases
- **aurelia** supervises the system in flight: process lifecycle, health checks, service dependencies
- **axon** is the building material: a suite of Go libraries you assemble services from

## Dependency Graph

Primitives (stdlib-only):
```
axon-mind    : embedded Prolog engine
axon-push    : push notification primitives
axon-rule    : business rules + state machine (Specification pattern)
axon-tape    : buffered token stream filter
axon-tool    : tool definitions for LLM agents
axon-wire    : HTTP proxy transport
```

Primitives (with deps):
```
axon         : server lifecycle, auth, SSE, metrics
axon-base    : Postgres pool, repository, migrations (pgx, goose)
axon-cost    : LLM cost tracking middleware (axon-talk, axon-fact)
axon-fact    : event sourcing primitives (pgx, goose)
axon-loop    : conversation loop (axon-tool)
axon-nats    : NATS adapters (axon, nats.go)
axon-scan    : code quality pipeline (axon-loop, axon-tool)
axon-sign    : cryptographic signing (golang.org/x/crypto)
axon-talk    : LLM provider adapters: OpenAI-compatible, Anthropic (axon-tape, axon-tool)
```

Domain packages:
```
axon-auth    : authentication (axon)
axon-book    : double-entry bookkeeping (axon, axon-fact)
axon-chat    : chat + agents (axon, axon-loop, axon-tool, axon-fact)
axon-gate    : deploy approval gate (axon)
axon-lens    : image pipeline (axon-tool)
axon-look    : analytics (axon)
axon-memo    : long-term memory (axon)
axon-snip    : code assembly (axon-loop, axon-talk, axon-tool)
axon-synd    : syndication engine (axon, axon-fact, axon-gate)
axon-task    : task runner (axon)
```

Standalone:
```
axon-eval    : evaluation framework
```

## lamina CLI

The `lamina` command (`cmd/lamina/`) manages the workspace:

```bash
lamina init                     # Clone all workspace repos
lamina repo                     # Summary table of all sub-repos
lamina repo status              # Full git status for every sub-repo
lamina repo fetch               # Git fetch all sub-repos
lamina repo <name> push         # Git push a specific sub-repo
lamina repo push --all          # Git push all repos (--all required)
lamina repo rebase --all        # Git pull --rebase all repos

lamina deps                     # Show dependency graph between workspace modules
lamina test                     # Run go test ./... across all axon-* modules
lamina test axon-chat           # Run tests for a specific module

lamina doctor                   # Check workspace health (stale deps, unpublished changes)
lamina heal                     # Fix issues found by doctor

lamina release axon-tool v0.2.0 # Tag a module and push the tag
lamina release --dry-run axon v1.0  # Preview what a release would do

lamina catalogue                # Show workspace module catalogue
lamina apps build <name>        # Build a workspace application
lamina apps install <name>      # Build + install to ~/.local/bin

lamina eval plans/smoke.yaml    # Run a YAML evaluation plan against the cluster
lamina skills                   # List embedded Claude Code skills
```

## Quick Reference

### lamina (this repo)
```bash
just build          # Build lamina CLI to bin/
just install        # Build + install to ~/.local/bin
just test           # Vet + run tests across axon-* modules
just install-flux   # Clone, build, install flux.swift CLI
```

### aurelia (process supervisor)
```bash
cd aurelia
just build          # Build binary
just install        # Build + install to ~/.local/bin + restart daemon
just test           # Unit tests (short)
just test-all       # All tests including slow
just test-integration  # Integration tests (requires Docker/OrbStack)
just lint           # go vet
```

### axon-* modules
```bash
cd axon-chat        # or any axon-* module
go test ./...       # All tests
go test -run TestName ./  # Single test
go vet ./...        # Lint
```

## Folder Structure

```
lamina/
├── cmd/lamina/         # CLI source
├── apps/               # Workspace applications (imago, revue, valuer, vita)
├── skills/             # Embedded Claude Code skills
│   ├── debug-lamina/   # Extends /debug
│   ├── deploy-lamina/  # Extends /deploy
│   ├── ground-lamina/  # Extends /ground
│   └── verify-lamina/  # Extends /verify
├── plans/              # Evaluation plans (YAML)
├── docs/
├── repos.yaml          # Workspace catalogue (all sub-repos)
├── go.work             # Go workspace (all modules + apps)
├── go.mod
├── justfile
└── [sub-repos]/        # Cloned by `lamina init`, gitignored
```

## Planning

- Plan steps should be commit-sized — each step produces one testable, committable change

## Conventions

- **Task runner**: `just` (justfile) in lamina and aurelia; standard `go` tooling in axon-* modules
- **Internal domain**: `*.hestia.internal` / `*.limen.internal` (not `.local`)
- **Native services**: Go binaries compiled for darwin/arm64, embed SvelteKit UIs via `//go:embed`
- **Containerized services**: Run on OrbStack (postgres, grafana, loki, traefik, vault, etc.)
- **AI agent docs**: Each project has `CLAUDE.md` or `AGENTS.md` with full architecture docs for any AI coding agent
- **Module publishing**: All axon-* modules are public on GitHub under MIT license, resolved via Go module proxy
- **GOPRIVATE**: `github.com/benaskins/*` is set in go env to bypass sum DB cache delays
- **No Python**: deliberate choice — stack is Go + Swift (MLX) + TypeScript (SvelteKit)
- **Slop guard**: `slop-guard` runs as pre-commit hook and Claude Code post-tool-use hook
