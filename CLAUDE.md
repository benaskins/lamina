# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Workspace Overview

Lamina is a workspace directory (not a git repo) containing three independent Go projects that form a personal compute cluster running on a Mac Studio (Apple Silicon):

| Project | Purpose | Language |
|---------|---------|----------|
| **aurelia** | macOS-native process supervisor for managing native processes and Docker containers | Go 1.26 |
| **aurelia-core-infrastructure** | Personal compute cluster — services, infrastructure, deployment configs | Go 1.24 + SvelteKit |
| **axon** | Shared Go toolkit for building AI-powered web services (HTTP lifecycle, auth, SSE, streaming) | Go 1.24 |

Each project is its own git repo with its own `CLAUDE.md` (or `AGENTS.md`). When working in a subproject, read its project-level docs first.

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

## Quick Reference

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

## Conventions

- **Task runner**: `just` (justfile) in aurelia and aurelia-core-infrastructure; standard `go` tooling in axon
- **Go workspace**: aurelia-core-infrastructure uses `go.work` for multi-module development
- **Service specs**: YAML files in `aurelia-core-infrastructure/aurelia/services/`
- **Internal domain**: `*.studio.internal` (not `.local`)
- **Native services**: Go binaries compiled for darwin/arm64, embed SvelteKit UIs via `//go:embed`
- **Containerized services**: Run on OrbStack (postgres, grafana, loki, traefik, vault, etc.)
- **AI agent docs**: Each project has `AGENTS.md` with full architecture docs for any AI coding agent
