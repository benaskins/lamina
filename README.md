# Lamina

> **This software is designed for managing hobby compute environments on macOS. Nothing else.**
>
> Lamina exists to support experiments and research into autonomous LLM-backed agents on a private, sovereign server. It includes everything you'd need to treat that server like a production stack — monitoring, deployment, process supervision — but this is for education and exploration, not actual production use.
>
> You could use this to build self-modifying, self-deploying agents that reason about their own uptime, check whether the services they depend on are alive, and operate those services if needed. That's the point. The ACL and security model is weak right now and will improve over time.
>
> It is not built for scale. It's intended for a single user on macOS, working at the command line with tools like [Claude Code](https://docs.anthropic.com/en/docs/claude-code). No IDE integration, no GUI — just a shell.

Lamina is the workspace and CLI for a personal compute cluster built on Go. It represents the system at rest — the modules, the configuration, the structure — and provides the tooling to manage it all.

The workspace is decomposed into small, focused repos that an AI coding agent can reason about independently. Three layers make up the system:

- **lamina** (this repo) manages the workspace — repo status, dependency graphs, testing, releases
- **[aurelia](https://github.com/benaskins/aurelia)** supervises the system in flight — process lifecycle, health checks, service dependencies
- **[axon](https://github.com/benaskins/axon)** and the **axon-\*** modules are the building material — a suite of Go libraries you assemble services from

## Workspace

**Toolkit**

| Repo | Description |
|------|-------------|
| [axon](https://github.com/benaskins/axon) | Shared toolkit for AI-powered web services — server lifecycle, auth, database, metrics, SSE, stream filtering |

**Primitives**

| Repo | Description |
|------|-------------|
| [axon-tool](https://github.com/benaskins/axon-tool) | Tool definition and execution primitives for LLM agents |
| [axon-loop](https://github.com/benaskins/axon-loop) | Provider-agnostic conversation loop for LLM-powered agents |
| [axon-talk](https://github.com/benaskins/axon-talk) | LLM provider adapters for axon-loop (Ollama, more to come) |
| [axon-fact](https://github.com/benaskins/axon-fact) | Event sourcing primitives — Event type, EventStore/Projector/Publisher interfaces |
| [axon-mind](https://github.com/benaskins/axon-mind) | Embedded Prolog engine for structured inference over facts and rules |
| [axon-nats](https://github.com/benaskins/axon-nats) | NATS adapters for axon services — EventBus[T] for horizontal scaling |
| [axon-lens](https://github.com/benaskins/axon-lens) | Image generation pipeline — prompt merging, FLUX.1 via MLX, gallery storage |

**Domain packages**

| Repo | Description |
|------|-------------|
| [axon-auth](https://github.com/benaskins/axon-auth) | WebAuthn-based authentication with passkey registration, login, and session management |
| [axon-chat](https://github.com/benaskins/axon-chat) | Chat with LLM integration, tool calling, SSE streaming, and agent management |
| [axon-gate](https://github.com/benaskins/axon-gate) | Deploy approval gate with Signal notifications and a review UI |
| [axon-look](https://github.com/benaskins/axon-look) | Analytics event ingestion and querying backed by ClickHouse |
| [axon-memo](https://github.com/benaskins/axon-memo) | Long-term memory extraction and consolidation for LLM agents |
| [axon-task](https://github.com/benaskins/axon-task) | Generic asynchronous task runner with pluggable workers |

**Standalone tools**

| Repo | Description |
|------|-------------|
| [aurelia](https://github.com/benaskins/aurelia) | macOS-native process supervisor for native processes and Docker containers |
| [axon-eval](https://github.com/benaskins/axon-eval) | Evaluation framework for running scenario plans against a live service cluster |

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
  axon         ─── server lifecycle, auth, SSE, metrics

Primitives:
  axon-tool    ─── tool definitions for LLM agents
  axon-loop    ─── conversation loop (depends on axon-tool)
  axon-talk    ─── LLM provider adapters (depends on axon-loop)
  axon-fact    ─── event sourcing primitives (no dependencies)
  axon-mind    ─── embedded Prolog engine (no dependencies)
  axon-nats    ─── NATS adapters (depends on axon)
  axon-lens    ─── image pipeline (depends on axon-tool)

Domain packages (handlers, stores, types — no main of their own):
  axon-auth    ─── authentication (axon)
  axon-chat    ─── chat + agents (axon, axon-loop, axon-tool, axon-fact)
  axon-gate    ─── deploy approval gate (axon)
  axon-look    ─── analytics (axon)
  axon-memo    ─── long-term memory (axon)
  axon-task    ─── task runner (axon)

Standalone tools:
  aurelia      ─── process supervisor
  axon-eval    ─── evaluation framework
```

None of the domain packages are services on their own — they're Lego bricks. A service is assembled in a composition root (a `main.go` that picks which bricks to snap together, wires them up, and starts listening). The `examples/` directory shows what this looks like in practice.

Lamina manages all of this as source. Aurelia runs it.

## Getting started

```bash
git clone https://github.com/benaskins/lamina-mono.git lamina
cd lamina
go install ./cmd/lamina/
lamina init
```

`lamina init` clones all workspace repos into the current directory. Repos that already exist are skipped, so it's safe to run again.

Once initialised, `lamina repo` shows the state of everything:

```bash
lamina repo            # summary table
lamina repo status     # full git status across all repos
```

## Lamina CLI

The `lamina` command manages the workspace — checking repo status, running tests, tracking dependencies, and coordinating releases across all modules.

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

lamina eval plans/smoke.yaml       # Run an evaluation plan against the cluster
lamina skills                      # List embedded Claude Code skills
```

## Install

Each module is a standalone Go package:

```
go get github.com/benaskins/axon@latest
go get github.com/benaskins/axon-loop@latest
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.26+.

## Slop guard

Every repo is checked by [slop-guard](https://github.com/benaskins/dotfiles/blob/master/scripts/slop-guard) — a bash script that catches AI-generated filler words and comment patterns in source files. It runs as a pre-commit hook and as a Claude Code post-tool-use hook.

Last full scan: **2026-03-06** — 0 issues across all repos.

| Repo | Status |
|------|--------|
| lamina | clean |
| aurelia | clean |
| axon | clean |
| axon-auth | clean |
| axon-chat | clean |
| axon-eval | clean |
| axon-gate | clean |
| axon-lens | clean |
| axon-look | clean |
| axon-loop | clean |
| axon-memo | clean |
| axon-talk | clean |
| axon-fact | clean |
| axon-mind | clean |
| axon-nats | clean |
| axon-task | clean |
| axon-tool | clean |

## A note on how this was built

Lamina was designed by Ben Askins and built in collaboration with Claude,
Anthropic's AI assistant. The architecture, taste, and decisions are human.
The typing is fast.

## License

All libraries are released under the MIT license.
