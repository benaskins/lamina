# Lamina

Lamina is the workspace and CLI for a personal compute cluster built on Go. It represents the system at rest — the modules, the configuration, the structure — and provides the tooling to manage it all.

The workspace is deliberately decomposed into small, focused repos that an AI coding agent can reason about independently. Three layers make up the system:

- **lamina** (this repo) manages the workspace — repo status, dependency graphs, testing, releases
- **[aurelia](https://github.com/benaskins/aurelia)** supervises the system in flight — process lifecycle, health checks, service dependencies
- **[axon](https://github.com/benaskins/axon)** is a suite of opinionated Go libraries you assemble services from

## Workspace

| Repo | Description |
|------|-------------|
| [aurelia](https://github.com/benaskins/aurelia) | macOS-native process supervisor for native processes and Docker containers |
| [axon](https://github.com/benaskins/axon) | Shared toolkit for AI-powered web services — server lifecycle, auth, database, metrics, SSE, stream filtering |
| [axon-loop](https://github.com/benaskins/axon-loop) | Provider-agnostic conversation loop for LLM-powered agents |
| [axon-tool](https://github.com/benaskins/axon-tool) | Tool definition and execution primitives for LLM agents |
| [axon-chat](https://github.com/benaskins/axon-chat) | Chat service with LLM integration, tool calling, SSE streaming, and agent management |
| [axon-auth](https://github.com/benaskins/axon-auth) | WebAuthn-based authentication with passkey registration, login, and session management |
| [axon-eval](https://github.com/benaskins/axon-eval) | Evaluation framework for running scenario plans against a live service cluster |
| [axon-gate](https://github.com/benaskins/axon-gate) | Deploy approval gate with Signal notifications and a review UI |
| axon-lens | Photo and image management with LLM-powered prompts |
| [axon-look](https://github.com/benaskins/axon-look) | Analytics event ingestion and querying backed by ClickHouse |
| [axon-memo](https://github.com/benaskins/axon-memo) | Long-term memory extraction and consolidation for LLM agents |
| [axon-task](https://github.com/benaskins/axon-task) | Asynchronous task runner for Claude Code sessions and image generation |

## How they fit together

```
lamina (at rest)                    aurelia (in flight)
 │                                   │
 ├── repo status, deps, testing      ├── process supervision
 ├── releases, health checks         ├── health checks, restarts
 └── workspace coordination          └── service dependencies
                    │
                    ▼
              axon (building material)
               ├── axon-auth ─── authentication
               ├── axon-chat ─── chat (+ axon-loop, axon-tool)
               ├── axon-gate ─── deploy gate
               ├── axon-look ─── analytics
               ├── axon-memo ─── long-term memory
               └── axon-task ─── task runner

              axon-tool ─── tool definitions
               ├── axon-loop ─── conversation loop
               └── axon-lens ─── image management

              axon-eval ─── evaluation (standalone)
```

You build services from axon modules, lamina manages them as source, and aurelia runs them.

## Lamina CLI

The `lamina` command manages the workspace — checking repo status, running tests, tracking dependencies, and coordinating releases across all modules.

```bash
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
```

## Install

Each module is a standalone Go package:

```
go get github.com/benaskins/axon@latest
go get github.com/benaskins/axon-loop@latest
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.24+.

## A note on how this was built

Lamina was designed by Ben Askins and built in collaboration with Claude,
Anthropic's AI assistant. The architecture, taste, and decisions are human.
The typing is fast.

## License

All libraries are released under the MIT license.
