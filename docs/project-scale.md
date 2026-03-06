# Project Scale

A snapshot of what exists after ~15 calendar days of development (Feb 19 – Mar 5, 2026), built by one human with an AI coding partner (Claude Code).

## The codebase

| Metric | Count |
|--------|-------|
| Lines of Go | 47,300 |
| Lines of Svelte/TypeScript | 5,900 |
| Lines of documentation | 1,200 |
| Repositories | 14 |
| Commits | 300+ |

## What it does

A complete personal compute platform for macOS:

- **aurelia** — process supervisor with macOS launchd integration, container lifecycle, health checks, dependency ordering, port allocation, GPU monitoring via Objective-C bridge
- **axon** — shared Go toolkit for AI-powered web services (HTTP lifecycle, auth middleware, SSE, metrics, database migrations)
- **axon-chat** — chat service with LLM integration, tool calling, SSE streaming, agent management, and a SvelteKit frontend
- **axon-auth** — WebAuthn/passkey authentication with session management and a SvelteKit UI
- **axon-look** — analytics event ingestion and querying backed by ClickHouse, with a SvelteKit dashboard
- **axon-memo** — long-term memory extraction and consolidation for LLM agents
- **axon-task** — asynchronous task runner for Claude Code sessions and image generation
- **axon-gate** — deploy approval gate with Signal notifications and a review UI
- **axon-loop** — provider-agnostic LLM conversation loop with tool calling
- **axon-talk** — LLM provider adapters (Ollama)
- **axon-tool** — tool definition and execution primitives
- **axon-lens** — Image generation pipeline (FLUX.1 via MLX, prompt merging, gallery)
- **axon-eval** — evaluation framework with LLM-as-judge grading
- **lamina** — workspace CLI with dependency graphing, health checks, release management, and embedded Claude Code skills

## Disciplines involved

Building this without an AI partner would require expertise across at least seven distinct areas:

1. **Systems programming** — aurelia's process supervisor with macOS launchd integration, container lifecycle management, health checks with dependency ordering, port allocation, and GPU monitoring via an Objective-C bridge
2. **Backend/API engineering** — 8 Go services with HTTP lifecycle, SSE streaming, database migrations, middleware chains, and Prometheus metrics
3. **Frontend engineering** — SvelteKit UIs for chat, auth, and analytics with SSE consumption, WebAuthn browser flows, and form state management
4. **DevOps/platform engineering** — workspace tooling, dependency graphing, release management with topological sorting, health checks, and service manifests
5. **AI/ML engineering** — LLM conversation loop with tool calling, provider abstraction, prompt engineering, and an evaluation framework with LLM-as-judge grading
6. **Security engineering** — WebAuthn/passkey authentication, session management, macOS Keychain integration with audit logging
7. **Domain modelling/architecture** — the three-layer decomposition (at rest / in flight / building material), module boundaries designed for AI agent reasoning, and DDD-driven API naming

## Traditional team estimate

What this would look like staffed conventionally:

| Component | Person-hours | Typical role |
|-----------|-------------|--------------|
| aurelia (process supervisor) | 400–600 | Senior systems engineer |
| axon (shared toolkit) | 80–120 | Backend lead |
| axon-chat (+ frontend) | 200–300 | Full-stack engineer |
| axon-auth (+ WebAuthn) | 120–160 | Security-aware full-stack |
| axon-look (analytics) | 80–120 | Backend + data |
| axon-memo (memory) | 60–80 | Backend |
| axon-task (task runner) | 60–80 | Backend |
| axon-gate (deploy gate) | 40–60 | Backend |
| axon-loop + axon-talk + axon-tool | 80–120 | AI/ML engineer |
| axon-lens (image pipeline) | 40–60 | Backend + ML |
| axon-eval (evaluation) | 60–80 | AI/ML + QA |
| lamina CLI + workspace tooling | 80–120 | Platform engineer |
| Architecture, docs, module design | 80–120 | Architect |
| **Total** | **~1,400–2,000** | |

That's roughly **8–12 person-months** of professional engineering work. A small startup might staff this with 3 senior engineers for a quarter.

## What the AI changes

The speed matters, but the bigger shift is scope. An AI partner changes what one person is willing to attempt.

A cross-repo type rename — touching types, tests, mocks, and a SvelteKit frontend across 3 repos — takes a full day of careful human work. With an AI partner, it happens in one conversation alongside several other changes.

One person spans all seven disciplines because the AI covers the breadth while the human provides taste, direction, and architectural judgement. The human decides "Skills should be called Tools" based on domain modelling principles; twenty files get updated in minutes, correctly, with tests passing.

The result: a single developer ships what would traditionally require a small team. Not by working harder, but by spending almost all their time on decisions rather than implementation.
