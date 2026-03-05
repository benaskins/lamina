# Lamina

A modular Go toolkit for building LLM-powered services.

Lamina is a collection of open-source libraries that provide the building blocks for AI agent infrastructure: conversation loops, tool execution, memory, authentication, analytics, evaluation, and deployment.

## Libraries

| Module | Description |
|--------|-------------|
| [axon](https://github.com/benaskins/axon) | Shared toolkit for AI-powered web services — server lifecycle, auth, database, metrics, SSE, stream filtering |
| [axon-loop](https://github.com/benaskins/axon-loop) | Provider-agnostic conversation loop for LLM-powered agents |
| [axon-tool](https://github.com/benaskins/axon-tool) | Tool definition and execution primitives for LLM agents |
| [axon-memo](https://github.com/benaskins/axon-memo) | Long-term memory extraction and consolidation for LLM agents |
| [axon-chat](https://github.com/benaskins/axon-chat) | Chat service with LLM integration, tool calling, SSE streaming, and agent management |
| [axon-auth](https://github.com/benaskins/axon-auth) | WebAuthn-based authentication with passkey registration, login, and session management |
| [axon-eval](https://github.com/benaskins/axon-eval) | Evaluation framework for running scenario plans against a live service cluster |
| [axon-look](https://github.com/benaskins/axon-look) | Analytics event ingestion and querying backed by ClickHouse |
| [axon-gate](https://github.com/benaskins/axon-gate) | Deploy approval gate with Signal notifications and a review UI |
| [axon-task](https://github.com/benaskins/axon-task) | Asynchronous task runner for Claude Code sessions and image generation |

## How they fit together

```
axon-tool ─── tool definitions
    │
axon-loop ─── conversation loop (uses axon-tool)
    │
axon-chat ─── chat service (uses axon-loop, axon-tool, axon)
    │
axon-memo ─── long-term memory (uses axon)
axon-auth ─── authentication (uses axon)
axon-look ─── analytics (uses axon)
axon-gate ─── deploy gate (uses axon)
axon-task ─── task runner (uses axon)
axon-eval ─── evaluation (standalone)
```

`axon` provides the foundation: HTTP server lifecycle, auth middleware, database helpers, Prometheus metrics, and SSE streaming. The other libraries build on it for specific capabilities.

## Lamina CLI

The `lamina` command manages the workspace — running tests across modules, checking repo status, executing evaluation plans, and coordinating dependencies.

```bash
lamina repo                     # Summary table of all sub-repos
lamina repo status              # Git status across every sub-repo
lamina test                     # Run go test ./... across all modules
lamina test axon-chat           # Test a specific module
lamina eval plans/smoke.yaml    # Run an evaluation plan against the cluster
lamina deps                     # Show dependency graph between modules
```

## Install

Each library is a standalone Go module:

```
go get github.com/benaskins/axon@latest
go get github.com/benaskins/axon-loop@latest
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.24+.

## License

All libraries are released under the MIT license.
