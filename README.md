# Lamina

> **This software is designed for managing hobby compute environments on macOS. Nothing else.**
>
> Lamina exists to support experiments and research into autonomous LLM-backed agents on a private, sovereign server. It includes everything you'd need to treat that server like a production stack (monitoring, deployment, process supervision) but this is for education and exploration, not actual production use.
>
> You could use this to build self-modifying, self-deploying agents that reason about their own uptime, check whether the services they depend on are alive, and operate those services if needed. That's the point. The ACL and security model is weak right now and will improve over time.
>
> It is not built for scale. It's intended for a single user on macOS, working at the command line with tools like [Claude Code](https://docs.anthropic.com/en/docs/claude-code). No IDE integration, no GUI, just a shell.

Lamina is the workspace and CLI for a personal compute cluster built on Go. It represents the system at rest (modules, configuration, structure) and provides the tooling to manage it all.

The workspace is decomposed into small, focused repos that an AI coding agent can reason about independently. Three layers make up the system:

- **lamina** (this repo) manages the workspace: repo status, dependency graphs, testing, releases
- **[aurelia](https://github.com/benaskins/aurelia)** supervises the system in flight: process lifecycle, health checks, service dependencies
- **[axon](https://github.com/benaskins/axon)** and the **axon-\*** modules are the building material: a suite of Go libraries you assemble services from

## Workspace

**Toolkit**

| Repo | Description |
|------|-------------|
| [axon](https://github.com/benaskins/axon) | Shared toolkit for AI-powered web services: server lifecycle, auth, database, metrics, SSE, stream filtering |

**Primitives**

| Repo | Description |
|------|-------------|
| [axon-tool](https://github.com/benaskins/axon-tool) | Tool definition and execution primitives for LLM agents |
| [axon-loop](https://github.com/benaskins/axon-loop) | Provider-agnostic conversation loop for LLM-powered agents |
| [axon-talk](https://github.com/benaskins/axon-talk) | LLM provider adapters (OpenAI-compatible, Anthropic) |
| [axon-tape](https://github.com/benaskins/axon-tape) | Buffered token stream filter with pluggable matchers, content safety, PII redaction |
| [axon-base](https://github.com/benaskins/axon-base) | PostgreSQL foundation: connection pooling, repository interfaces, migrations, row scanning |
| [axon-fact](https://github.com/benaskins/axon-fact) | Event sourcing primitives: Event type, EventStore/Projector/Publisher interfaces |
| [axon-rule](https://github.com/benaskins/axon-rule) | Composable business-rule predicates and typed state machine |
| [axon-sign](https://github.com/benaskins/axon-sign) | Cryptographic signing: Ed25519 keypairs, SSHSIG signatures, key rotation, provenance |
| [axon-mind](https://github.com/benaskins/axon-mind) | Embedded Prolog engine for structured inference over facts and rules |
| [axon-wire](https://github.com/benaskins/axon-wire) | HTTP transport that routes outbound requests through a Cloudflare Worker proxy |
| [axon-nats](https://github.com/benaskins/axon-nats) | NATS EventBus[T] for cross-instance fan-out |
| [axon-push](https://github.com/benaskins/axon-push) | Push notification primitives |

**Agent infrastructure**

| Repo | Description |
|------|-------------|
| [axon-hand](https://github.com/benaskins/axon-hand) | Shared chassis for autonomous agents: LLM client config, worker identity, CLI, lifecycle |
| [axon-code](https://github.com/benaskins/axon-code) | Coding agent library with LLM conversation loop, file-system and shell tools |
| [axon-snip](https://github.com/benaskins/axon-snip) | Code assembly engine: PRD analysis, module selection, scaffold generation |
| [axon-scan](https://github.com/benaskins/axon-scan) | Code quality pipeline: static analysis, security scanning, test execution, signed attestation |
| [axon-cost](https://github.com/benaskins/axon-cost) | LLM inference cost tracking middleware with rate tables and budget alerts |
| [axon-lore](https://github.com/benaskins/axon-lore) | Architecture Decision Records: structured ADR generation and management |

**Domain packages**

| Repo | Description |
|------|-------------|
| [axon-auth](https://github.com/benaskins/axon-auth) | WebAuthn-based authentication with passkey registration, login, and session management |
| [axon-book](https://github.com/benaskins/axon-book) | Event-sourced double-entry bookkeeping: ledger, chart of accounts, journal entries |
| [axon-chat](https://github.com/benaskins/axon-chat) | Chat with LLM integration, tool calling, SSE streaming, and agent management |
| [axon-face](https://github.com/benaskins/axon-face) | Bubble Tea TUI components for LLM-powered conversational interfaces |
| [axon-gate](https://github.com/benaskins/axon-gate) | Deploy approval gate with Signal notifications and a review UI |
| [axon-lens](https://github.com/benaskins/axon-lens) | Image generation pipeline: prompt merging, FLUX.1 via MLX, gallery storage |
| [axon-look](https://github.com/benaskins/axon-look) | Analytics event ingestion and querying backed by ClickHouse |
| [axon-memo](https://github.com/benaskins/axon-memo) | Long-term memory extraction and consolidation for LLM agents |
| [axon-synd](https://github.com/benaskins/axon-synd) | Syndication engine: publish to a static site, syndicate to Bluesky, Mastodon, Threads |
| [axon-task](https://github.com/benaskins/axon-task) | Generic asynchronous task runner with pluggable workers |

**Standalone tools**

| Repo | Description |
|------|-------------|
| [aurelia](https://github.com/benaskins/aurelia) | Process supervisor for native processes and Docker containers, with macOS-specific enhancements |
| [axon-eval](https://github.com/benaskins/axon-eval) | Evaluation framework for running scenario plans against a live service cluster |

**Applications**

| App | Description |
|-----|-------------|
| [imago](https://github.com/benaskins/imago) | Interactive interview CLI for structured blog post generation |
| [revue](https://github.com/benaskins/revue) | Code review agent |
| [vita](https://github.com/benaskins/vita) | Personal knowledge base |

## How they fit together

```
lamina (at rest)                    aurelia (in flight)
 │                                   │
 ├── repo status, deps, testing      ├── process supervision
 ├── releases, workspace health      ├── health checks, restarts
 └── workspace coordination          └── service dependencies
                    │
                    ▼
              axon (building material)

Toolkit:
  axon         --- server lifecycle, auth, SSE, metrics

Primitives:
  axon-tool    --- tool definitions for LLM agents
  axon-loop    --- conversation loop (axon-talk, axon-tool)
  axon-talk    --- LLM provider adapters (axon-tape, axon-tool)
  axon-tape    --- buffered token stream filter
  axon-base    --- PostgreSQL foundation (pgx, goose)
  axon-fact    --- event sourcing primitives
  axon-rule    --- business rules + state machine
  axon-sign    --- cryptographic signing (golang.org/x/crypto)
  axon-mind    --- embedded Prolog engine
  axon-wire    --- HTTP proxy transport
  axon-nats    --- NATS adapters (axon-push, nats.go)
  axon-push    --- push notification primitives

Agent infrastructure:
  axon-hand    --- agent chassis (axon-talk)
  axon-code    --- coding agent (axon-loop, axon-talk, axon-tool)
  axon-snip    --- code assembly (axon-loop, axon-talk, axon-tool)
  axon-scan    --- quality pipeline (axon-loop, axon-tool)
  axon-cost    --- cost tracking (axon-talk, axon-fact)
  axon-lore    --- ADR generation (axon-snip)

Domain packages (handlers, stores, types; no main of their own):
  axon-auth    --- authentication (axon)
  axon-book    --- double-entry bookkeeping (axon, axon-fact)
  axon-chat    --- chat + agents (axon, axon-loop, axon-tool, axon-fact)
  axon-face    --- TUI components (axon-loop, axon-talk, axon-tool)
  axon-gate    --- deploy approval gate (axon)
  axon-lens    --- image pipeline (axon-tool)
  axon-look    --- analytics (axon)
  axon-memo    --- long-term memory (axon)
  axon-synd    --- syndication engine (axon, axon-fact, axon-gate)
  axon-task    --- task runner (axon)

Standalone tools:
  aurelia      --- process supervisor
  axon-eval    --- evaluation framework
```

None of the domain packages are services on their own. They're Lego bricks. A service is assembled in a composition root (a `main.go` that picks which bricks to snap together, wires them up, and starts listening). The `apps/` directory shows what this looks like in practice.

Lamina manages all of this as source. Aurelia runs it.

## Why lamina

Lamina is for building autonomous, LLM-backed agents on your own hardware. If you want to run a personal compute cluster on a Mac, where agents can reason about their own uptime, manage the services they depend on, and operate without phoning home, this is the stack for it.

**What you get:**

- A workspace of small, composable Go libraries (axon) that an AI coding agent can reason about independently. Each module does one thing: conversation loops, tool execution, event sourcing, cost tracking, memory, syndication. You assemble services by snapping them together in a `main.go`.
- A process supervisor (aurelia) purpose-built for mixed native + container stacks on macOS. It handles dependency ordering, health checks, automatic restarts, and secrets injection — things you'd otherwise cobble together from launchd, Docker Compose, and shell scripts.
- A workspace manager (lamina) that coordinates across 30+ repos: dependency graphs, cross-module testing, release tagging, and health checks that catch version drift before it breaks things.

**What it is not:**

- A cloud deployment platform. Lamina targets a single machine (macOS, Apple Silicon). There is no clustering, auto-scaling, or multi-tenant isolation.
- A production-grade system. The ACL and security model is minimal. This is for personal experimentation and research.
- A framework you import. Lamina is a workspace tool. Axon modules are the libraries you import. Aurelia is the supervisor you run. They're independent layers with hard dependency boundaries.
- A GUI application. Everything is CLI-first, designed to be operated from the terminal with tools like [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

## Getting started

### Prerequisites

- **Go 1.26+** — all modules target Go 1.26.1
- **just** — task runner, used by lamina and aurelia ([install](https://github.com/casey/just#installation))
- **git** — each sub-repo is an independent git repository
- **OrbStack** or **Docker** (optional) — required for containerised services (Postgres, Grafana, Traefik, etc.) and aurelia integration tests

### Setup

```bash
git clone https://github.com/benaskins/lamina-mono.git lamina
cd lamina
just install        # Build lamina CLI → ~/.local/bin
lamina init         # Clone all sub-repos, wire go.work, install pre-commit hooks
```

`lamina init` clones 35+ repos from `repos.yaml` into the workspace directory. Repos that already exist are skipped, so it's safe to run again. For apps, it also wires local `replace` directives so you develop against workspace copies of axon modules.

Once initialised, check the state of everything:

```bash
lamina repo            # Summary table
lamina repo status     # Full git status across all repos
lamina doctor          # Health check: stale deps, untagged modules, missing docs
```

### Working on an axon module

Each axon module is a standalone Go package with its own git repo, cloned into the workspace root. The `go.work` file ties them together for local development — edits to one module are immediately visible to all others without publishing.

```bash
cd axon-chat           # Enter a module
go test ./...          # Test in isolation
go vet ./...           # Lint
```

To test across the full workspace:

```bash
lamina test            # Run go test ./... across all modules
lamina test axon-chat  # Test a specific module
```

Each module has its own `CLAUDE.md` or `AGENTS.md` with architecture docs. Read those before making changes.

### Working on an app

Applications live in `apps/` and compose axon modules into runnable binaries. An app is a `main.go` that picks which modules to snap together, wires them up, and starts listening (or launches a TUI).

```bash
lamina apps build vita      # Build an app
lamina apps install vita    # Build + install to ~/.local/bin
```

Apps use `replace` directives to point at workspace copies of axon modules, so local changes are picked up immediately.

### Working with aurelia

Aurelia supervises the running system — native Go binaries and Docker containers, managed through YAML service definitions in `~/.aurelia/services/`.

```bash
cd aurelia
just build             # Build binary
just install           # Build + install + restart daemon
just test              # Unit tests
just test-integration  # Integration tests (requires OrbStack)
```

Aurelia is not imported by axon services. It supervises them from the outside: starting processes in dependency order, running health checks, restarting on failure, and injecting secrets from macOS Keychain or OpenBao. The boundary between "building material" (axon) and "operations" (aurelia) is intentional and strict.

### Keeping things healthy

```bash
lamina doctor          # Six health checks: repo cleanliness, broken replace directives,
                       # untagged modules, unpublished commits, version drift, missing docs
lamina heal            # Auto-fix: tag untagged modules, bump versions, scaffold missing docs
lamina doctor --json | lamina heal  # Pipe doctor output into heal
```

Run `lamina doctor` regularly, especially after cross-module refactors. It catches version inconsistencies — where two services depend on different versions of the same axon module — before they surface as runtime bugs.

## Lamina CLI

The `lamina` command manages the workspace: repo status, tests, dependency tracking, and releases across all modules.

```bash
lamina init                        # Clone all workspace repos
lamina repo                        # Summary table of all sub-repos
lamina repo status                 # Full git status for every sub-repo
lamina repo axon-chat              # Git status for a single repo
lamina repo fetch                  # Git fetch all repos
lamina repo axon-chat push         # Git push a single repo
lamina repo push --all             # Git push all repos (--all required)
lamina repo rebase --all           # Git pull --rebase all repos

lamina deps                        # Show dependency graph between modules
lamina test                        # Run go test ./... across all modules
lamina test axon-chat              # Test a specific module

lamina doctor                      # Check workspace health (stale deps, unpublished changes)
lamina heal                        # Fix issues found by doctor

lamina release axon-tool v0.2.0    # Tag a module and push the tag
lamina release --dry-run axon v1.0 # Preview what a release would do

lamina catalogue                   # Show workspace module catalogue
lamina apps build <name>           # Build a workspace application
lamina apps install <name>         # Build + install to ~/.local/bin

lamina eval plans/smoke.yaml       # Run an evaluation plan against the cluster
lamina skills                      # List embedded Claude Code skills
```

## Using axon modules outside the workspace

Each module is a standalone Go package, published on GitHub under MIT:

```
go get github.com/benaskins/axon@latest
go get github.com/benaskins/axon-loop@latest
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.26+. You don't need lamina or aurelia to use individual axon modules in your own projects.

## Slop guard

Every repo is checked by [slop-guard](https://github.com/benaskins/dotfiles/blob/master/scripts/slop-guard), a bash script that catches AI-generated filler words and comment patterns in source files. It runs as a pre-commit hook and as a Claude Code post-tool-use hook.

## A note on how this was built

Lamina was designed by Ben Askins and built in collaboration with Claude,
Anthropic's AI assistant. The architecture, taste, and decisions are human.
The typing is fast.

## License

All libraries are released under the MIT license.
