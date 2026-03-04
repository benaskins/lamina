# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Lamina is a **git repository** that serves as the monorepo root for a personal compute cluster running on a Mac Studio (Apple Silicon). It contains:

1. The **`lamina` CLI** (`cmd/lamina/`) — a workspace management tool for coordinating across sub-repos
2. **Go library modules** (`axon-agent/`, `axon-photo/`, `axon-tool/`) — shared modules tracked in this repo
3. **Independent sub-repos** — each has its own `.git` and is pushed to GitHub separately (gitignored here)

### Sub-repos (independent git repos, gitignored)

| Repo | Purpose | Language |
|------|---------|----------|
| **aurelia** | macOS-native process supervisor for native processes and Docker containers | Go 1.26 |
| **aurelia-core-infrastructure** | Personal compute cluster — services, infrastructure, deployment configs | Go 1.24 + SvelteKit |
| **axon** | Shared Go toolkit for AI-powered web services (HTTP lifecycle, auth, SSE, streaming) | Go 1.24 |
| **axon-anal** | Analytics event ingestion and querying service | Go |
| **axon-auth** | Authentication service | Go |
| **axon-chat** | Chat service with LLM integration, tool calling, SSE streaming | Go |
| **axon-gate** | Deploy gate service | Go |
| **axon-memo** | Long-term memory extraction and consolidation service | Go |
| **axon-task** | Task runner service | Go |
| **axon-test** | Test library for running eval scenarios against the cluster | Go |

### Modules in this repo (tracked by lamina git)

| Module | Purpose |
|--------|---------|
| **axon-agent** | LLM conversation loop with tool calling |
| **axon-photo** | Photo management with LLM-powered prompts |
| **axon-tool** | Tool definition types for axon-agent |

Each sub-repo has its own `CLAUDE.md` or `AGENTS.md`. When working in a sub-repo, read its project-level docs first.

## How the Projects Relate

```
axon (shared library)
 └── imported by aurelia-core-infrastructure/services/* (chat, auth, memory, deploy-gate, task-runner)

aurelia (process supervisor daemon)
 └── manages services defined in aurelia-core-infrastructure/aurelia/services/*.yaml

aurelia-core-infrastructure (the cluster)
 └── services are Go binaries that import axon and are supervised by aurelia
```

- **axon** is the foundation: provides server lifecycle, auth middleware, database helpers, SSE, and stream filtering
- **aurelia** is the orchestrator: reads YAML service specs, manages process/container lifecycle, health checks, dependencies
- **aurelia-core-infrastructure** is the application layer: chat, auth, memory, and other services built on axon, deployed via aurelia

## lamina CLI

The `lamina` command (`cmd/lamina/`) manages the workspace:

```bash
lamina repo                     # Summary table of all sub-repos
lamina repo status              # Full git status for every sub-repo
lamina repo fetch               # Git fetch all sub-repos
lamina repo <name> push         # Git push a specific sub-repo
lamina deps                     # Show dependency graph between workspace modules
lamina test                     # Run go test ./... across all axon-* modules
lamina test axon-chat           # Run tests for a specific module
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

### aurelia-core-infrastructure (cluster)
```bash
cd aurelia-core-infrastructure
just up             # Start all services (idempotent)
just status         # Check cluster health
just build-native SERVICE  # Build a native service (chat, auth, memory, etc.)
just deploy-prod SERVICE   # Build + deploy via aurelia
just ship-prod SERVICE     # Test → build → deploy
just test SERVICE          # Run tests for a service
```

### axon (shared toolkit)
```bash
cd axon
go test ./...       # All tests
go test -run TestName ./  # Single test
go vet ./...        # Lint
```

## Planning

- Plan steps should be commit-sized — each step produces one testable, committable change

## Conventions

- **Task runner**: `just` (justfile) in lamina, aurelia, and aurelia-core-infrastructure; standard `go` tooling in axon-* modules
- **Go workspace**: aurelia-core-infrastructure uses `go.work` for multi-module development
- **Service specs**: YAML files in `aurelia-core-infrastructure/aurelia/services/`
- **Internal domain**: `*.studio.internal` (not `.local`)
- **Native services**: Go binaries compiled for darwin/arm64, embed SvelteKit UIs via `//go:embed`
- **Containerized services**: Run on OrbStack (postgres, grafana, loki, traefik, vault, etc.)
- **AI agent docs**: Each project has `AGENTS.md` with full architecture docs for any AI coding agent
