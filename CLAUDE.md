# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Lamina is a **git repository** that serves as the workspace root for a personal compute cluster running on a Mac Studio (Apple Silicon). It contains:

1. The **`lamina` CLI** (`cmd/lamina/`) — a workspace management tool for coordinating across sub-repos
2. **Claude Code skills** (`skills/`) — embedded skill definitions for workspace operations
3. **Example applications** (`examples/`) — reference implementations showing how to assemble axon modules

The workspace is populated by `lamina init`, which clones all sub-repos into this directory. Each sub-repo has its own `.git` and is pushed to GitHub separately (gitignored here).

### Sub-repos (independent git repos, cloned by `lamina init`)

| Repo | Purpose |
|------|---------|
| **aurelia** | macOS-native process supervisor for native processes and Docker containers |
| **axon** | Shared Go toolkit for AI-powered web services (HTTP lifecycle, auth, SSE, streaming) |
| **axon-auth** | WebAuthn-based authentication with passkey registration, login, and session management |
| **axon-chat** | Chat service with LLM integration, tool calling, SSE streaming, and agent management |
| **axon-eval** | Evaluation framework for running scenario plans against a live service cluster |
| **axon-gate** | Deploy approval gate with Signal notifications and a review UI |
| **axon-lens** | Image generation pipeline — prompt merging, FLUX.1 via MLX, gallery storage |
| **axon-look** | Analytics event ingestion and querying backed by ClickHouse |
| **axon-loop** | Provider-agnostic conversation loop for LLM-powered agents |
| **axon-memo** | Long-term memory extraction and consolidation for LLM agents |
| **axon-talk** | LLM provider adapters for axon-loop (Ollama, more to come) |
| **axon-task** | Generic asynchronous task runner with pluggable workers |
| **axon-tool** | Tool definition and execution primitives for LLM agents |

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

- **lamina** manages the workspace as source — repo status, dependency graphs, testing, releases
- **aurelia** supervises the system in flight — process lifecycle, health checks, service dependencies
- **axon** is the building material — a suite of Go libraries you assemble services from

## Dependency Graph

Libraries (no service dependencies):
```
axon         ─── server lifecycle, auth, SSE, metrics
axon-tool    ─── tool definitions for LLM agents
axon-loop    ─── conversation loop (depends on axon-tool)
axon-talk    ─── LLM provider adapters (depends on axon-loop)
axon-lens    ─── image pipeline (depends on axon-tool)
```

Services (built from libraries):
```
axon-auth    ─── authentication (axon)
axon-chat    ─── chat + agents (axon, axon-loop, axon-tool)
axon-gate    ─── deploy approval gate (axon)
axon-look    ─── analytics (axon)
axon-memo    ─── long-term memory (axon)
axon-task    ─── task runner (axon)
```

Standalone:
```
axon-eval    ─── evaluation framework
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

lamina eval plans/smoke.yaml    # Run a YAML evaluation plan against the cluster
lamina skills                   # List embedded Claude Code skills
```

## Quick Reference

### lamina (this repo)
```bash
just build          # Build lamina CLI to bin/
just install        # Build + install to ~/.local/bin
just test           # Vet + run tests across axon-* modules
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
├── skills/             # Embedded Claude Code skills
│   ├── embed.go        # go:embed for skill files
│   ├── lamina-workspace/
│   └── repo-archive/
├── examples/
│   └── chat/           # Example chat service (axon + axon-loop + axon-talk)
├── plans/              # Evaluation plans (YAML)
├── docs/
├── justfile
├── go.mod
└── [sub-repos]/        # Cloned by `lamina init`, gitignored
```

## Planning

- Plan steps should be commit-sized — each step produces one testable, committable change

## Conventions

- **Task runner**: `just` (justfile) in lamina and aurelia; standard `go` tooling in axon-* modules
- **Internal domain**: `*.studio.internal` (not `.local`)
- **Native services**: Go binaries compiled for darwin/arm64, embed SvelteKit UIs via `//go:embed`
- **Containerized services**: Run on OrbStack (postgres, grafana, loki, traefik, vault, etc.)
- **AI agent docs**: Each project has `CLAUDE.md` or `AGENTS.md` with full architecture docs for any AI coding agent
- **Module publishing**: All axon-* modules are public on GitHub under MIT license, resolved via Go module proxy
- **GOPRIVATE**: `github.com/benaskins/*` is set in go env to bypass sum DB cache delays
