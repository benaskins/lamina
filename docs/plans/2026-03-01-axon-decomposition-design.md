# Axon Decomposition: axon-tool, axon-agent, axon-chat

## Context

axon-chat is a single Go module containing tool definitions, a conversation loop, HTTP handlers, SSE streaming, persistence interfaces, and an embedded SvelteKit frontend. We're preparing it for open source by decomposing it into three modules with one-way dependencies.

## Why Go

- **Supply chain safety** — a Go binary compiles to a single static executable. No node_modules, no pip packages, no shared libraries to audit or update. The dependency tree is small, auditable, and pinned by `go.sum`.
- **Strong type system** — catches errors at compile time, not at runtime in production.
- **Small, capable stdlib** — HTTP servers, JSON, crypto, concurrency — all built in, no third-party framework required.
- **Easy for humans to reason about** — explicit control flow, no hidden magic, one obvious way to do things.
- **Easy for agents to reason about** — the same properties that help humans help AI coding agents. Go's explicitness and consistency make it a strong target for agent-assisted development.
- **Efficient for compute** — compiled, low memory overhead, excellent concurrency primitives. When compute becomes a tradeable commodity, efficiency matters.
- **Build speed and iteration speed** — fast compilation means tight feedback loops. Correctness is enforced both syntactically and semantically by the compiler.
- **Interpreters are a liability** — expressive languages that run on interpreters carry runtime overhead, supply chain risk, and deployment complexity. As compute becomes a commodity, the economics favour compiled languages with minimal runtime dependencies.

## Goals

1. **axon-tool** — standalone tool framework usable without an agent or chat UI
2. **axon-agent** — conversation loop that can run agents with or without tools, no HTTP or UI
3. **axon-chat** — full chat application (HTTP, SSE, persistence, frontend) for open source

## Design

### Dependency Direction

```
axon-chat → axon-agent → axon-tool
```

One-way only. No package ever imports something that imports it.

### axon-tool

The tool primitives library. No LLM opinion, no HTTP, no UI.

**Types:**

```go
type ToolDef struct {
    Name        string
    Description string
    Parameters  ParameterSchema
    Execute     func(ctx *ToolContext, args map[string]any) ToolResult
}

type ToolResult struct {
    Content string
}

type ToolContext struct {
    Ctx            context.Context
    UserID         string
    Username       string
    AgentSlug      string
    ConversationID string
    SystemPrompt   string
}
```

**Interfaces:**

```go
type Searcher interface {
    Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type WeatherProvider interface {
    GetWeather(ctx context.Context, location string) (*WeatherResult, error)
}
```

**Components that need an LLM** take a simple function, not a ChatClient:

```go
type TextGenerator func(ctx context.Context, prompt string) (string, error)
```

- `PageFetcher` — fetches web pages, uses `TextGenerator` for content extraction
- `ToolRouter` — LLM-based tool selection, uses `TextGenerator`
- `SearchQualifier` — query refinement, uses `TextGenerator`

**Generic tool constructors** (closure-based, dependencies injected at construction):

- `WebSearchTool(s Searcher) ToolDef`
- `FetchPageTool(f *PageFetcher) ToolDef`
- `CurrentTimeTool() ToolDef`
- `CheckWeatherTool(w WeatherProvider) ToolDef`

### axon-agent

The conversation loop. Works with or without tools. No HTTP, no persistence, no UI.

**Types:**

```go
type Message struct {
    Role     string
    Content  string
    Thinking string
    ToolCalls []ToolCall
}

type ToolCall struct {
    Name      string
    Arguments map[string]any
}

type ChatResponse struct {
    Content  string
    Thinking string
    Done     bool
    ToolCalls []ToolCall
}
```

**ChatClient interface** (provider-agnostic):

```go
type ChatClient interface {
    Chat(ctx context.Context, req *ChatRequest, fn func(ChatResponse) error) error
}

type ChatRequest struct {
    Model    string
    Messages []Message
    Tools    []tool.ToolDef  // from axon-tool
    Stream   bool
    Options  map[string]any
}
```

**Conversation loop** with callbacks:

```go
type AgentCallbacks struct {
    OnToken   func(token string)
    OnThinking func(token string)
    OnToolUse func(name string)
    OnDone    func(durationMs int64)
}

func Run(ctx context.Context, client ChatClient, req *ChatRequest, tools map[string]tool.ToolDef, toolCtx *tool.ToolContext, cb AgentCallbacks) (*Result, error)
```

The loop: send messages to LLM, stream tokens via callbacks, detect tool calls, execute via axon-tool's `ToolDef.Execute`, append tool results to messages, repeat until no tool calls remain.

Stream filtering (tool call detection in streamed text) lives here — it's about parsing LLM output, not tool concerns.

### axon-chat

The full chat application. Imports both axon-agent and axon-tool.

**Ollama adapter** — implements `agent.ChatClient` by translating to/from `ollamaapi` types. This is where the Ollama dependency concentrates.

**HTTP layer:**
- `Server` type, route registration, auth middleware injection
- SSE streaming: wires `AgentCallbacks` → SSE events over HTTP
- Prometheus metrics

**Persistence:**
- `Store` interface (users, agents, conversations, messages, gallery images)
- Domain types: `Conversation`, `ConversationSummary`, `Message`, `Agent`, `AgentSummary`, `GalleryImage`
- `chattest.MemoryStore` for testing

**Chat-specific tools** (implement `tool.ToolDef` but depend on chat-layer concerns):
- `take_photo`, `take_private_photo` — image generation via TaskRunner
- `use_claude` — code change submission via TaskRunner
- `PromptMerger` — image prompt assembly (depends on Store for recent messages)

**Frontend:**
- SvelteKit app in `web/`
- Embedded via `//go:embed`
- Auth URL made configurable (currently hardcoded to `studio.internal`)

**Other chat-layer concerns:**
- `TaskRunner` interface
- `ImageStore`
- `ModelLister` interface
- Title generation
- Background task polling

## De-companioning (applies across all three modules)

- User agent `Aurelia/1.0` in PageFetcher → generic default (e.g. `axon-tool/1.0`)
- Hardcoded `extractionModel` and `qualifierModel` → configurable via constructor
- Test fixtures referencing "Aurelia" → generic names
- Frontend `auth.studio.internal` → configurable auth URL
- Vite dev server hostnames → `localhost` defaults
- Compiled frontend assets → rebuild after changes

## Implementation Plan

Each step is one commit-sized change.

### Phase 1: Create axon-tool

1. Create `axon-tool/` module with `go.mod` (`github.com/benaskins/axon-tool`)
2. Define core types: `ToolDef`, `ToolResult`, `ToolContext`, `ParameterSchema`, `TextGenerator`
3. Define interfaces: `Searcher`, `SearchResult`, `WeatherProvider`, `WeatherResult`
4. Move `PageFetcher` from axon-chat — replace `ChatClient` with `TextGenerator`, replace `Aurelia/1.0` user agent, make extraction model configurable
5. Move `ToolRouter` from axon-chat — replace `ChatClient` with `TextGenerator`, make model configurable
6. Move `SearchQualifier` from axon-chat — replace `ChatClient` with `TextGenerator`, make model configurable
7. Move `SearXNGClient` (implements `Searcher`) from axon-chat
8. Move `OpenMeteoClient` (implements `WeatherProvider`) from axon-chat
9. Create generic tool constructors: `WebSearchTool`, `FetchPageTool`, `CurrentTimeTool`, `CheckWeatherTool`
10. Add tests for moved components

### Phase 2: Create axon-agent

11. Create `axon-agent/` module with `go.mod` (`github.com/benaskins/axon-agent`)
12. Define types: `Message`, `ToolCall`, `ChatClient`, `ChatRequest`, `ChatResponse`
13. Define `AgentCallbacks` and the `Run` function signature
14. Extract the conversation loop from `handler.go:streamChat` — strip HTTP/SSE/persistence, use callbacks
15. Move stream filtering (tool call detection in streamed text) — currently in `axon/stream`, decide if it stays in axon or moves to axon-agent
16. Add tests for the conversation loop (with and without tools)

### Phase 3: Refactor axon-chat

17. Add `axon-tool` and `axon-agent` as dependencies in axon-chat's `go.mod`
18. Create Ollama adapter: implement `agent.ChatClient` wrapping `ollamaapi.Client`
19. Refactor `chatHandler` to use `agent.Run` for the conversation loop
20. Refactor tools to use `tool.ToolDef` — chat-specific tools (`take_photo`, `use_claude`) built in axon-chat, generic tools imported from axon-tool
21. Wire `AgentCallbacks` → SSE events in the HTTP handler
22. Remove code that moved to axon-tool and axon-agent
23. Make frontend auth URL configurable (replace hardcoded `studio.internal`)
24. Update Vite dev server config to use `localhost` defaults
25. Rebuild frontend assets
26. Clean up test fixtures (remove "Aurelia" references)
27. Verify all tests pass across all three modules
