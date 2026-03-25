# RFC: axon-mask — PII Vault for the Axon Ecosystem

> **Status**: Draft
> **Date**: 2026-03-25
> **Author**: Security audit session

---

## 1. Problem

Axon services store personally identifiable information (PII) in two places
that resist deletion:

1. **axon-fact event streams** — append-only by design. `MessageAppended`,
   `MemoryExtracted`, `AgentCreated` and other events embed user content,
   conversation text, and memory extractions directly in the JSONB `data`
   field. There is no mechanism to delete or anonymise individual events.

2. **axon-memo memory store** — extracted memories, relationship metrics, and
   personality context contain raw conversation text and emotional analysis.
   Embeddings are derived vectors that can approximate original text via
   inversion attacks.

GDPR Article 17 (right to erasure), CCPA, and most regulatory frameworks
require the ability to delete all personal data for a given user on request.
The current architecture cannot satisfy this.

## 2. Goals

1. **Erasable PII** — all personal content can be permanently destroyed per
   user via a single operation (crypto-shredding).
2. **Immutable events preserved** — axon-fact remains append-only. Events
   reference content by opaque ID; the content lives in a mutable, encrypted,
   deletable store.
3. **Subject Access Requests** — all PII for a user can be exported from a
   single source.
4. **Retention policies** — content can be assigned a TTL and automatically
   reaped.
5. **Minimal disruption** — axon-fact requires zero changes. axon-chat and
   axon-memo adopt the vault at the handler/emitter layer. Read paths resolve
   content refs transparently.
6. **Broad scope** — axon-chat, axon-memo, axon-auth, and axon-look are all
   covered by the erasure orchestration.

## 3. Non-Goals

- Encrypting non-PII operational data (metrics, config, model parameters).
- Homomorphic or searchable encryption on vault content.
- Multi-tenant key isolation (single-operator system; per-user keys suffice).

## 4. Design

### 4.1 New Module: `axon-mask`

A Go library with no service dependencies. Provides:

- A `Vault` interface for storing, retrieving, and erasing PII.
- A PostgreSQL implementation with AES-256-GCM encryption.
- Per-user data encryption keys (DEKs) wrapped by a master key encryption
  key (KEK) sourced from the environment (OpenBao / aurelia secrets).
- Retention policy enforcement via expiry timestamps and a reaper.

### 4.2 Vault Interface

```go
package mask

import "context"

// ContentRef is an opaque reference to vaulted content.
// Format: "mask:<content-id>" (e.g. "mask:cnt_a1b2c3d4e5f6").
type ContentRef string

// Vault stores, retrieves, and erases PII.
type Vault interface {
    // Store encrypts content under the user's DEK and returns an opaque ref.
    Store(ctx context.Context, userID string, content []byte) (ContentRef, error)

    // Retrieve decrypts a single content item.
    // Returns ErrErased if the user's key has been destroyed.
    // Returns ErrNotFound if the content ID does not exist.
    Retrieve(ctx context.Context, ref ContentRef) ([]byte, error)

    // RetrieveBatch decrypts multiple items in one round-trip.
    // Missing or erased refs are absent from the returned map (no error).
    RetrieveBatch(ctx context.Context, refs []ContentRef) (map[ContentRef][]byte, error)

    // EraseUser destroys all DEKs for the user, rendering their content
    // permanently unreadable. Ciphertext rows are left for async cleanup.
    EraseUser(ctx context.Context, userID string) error

    // DeleteContent removes specific content items (for retention reaping
    // or selective cleanup).
    DeleteContent(ctx context.Context, refs ...ContentRef) error

    // ExportUser decrypts and returns all content for a user, keyed by ref.
    // Used for Subject Access Requests (GDPR Article 15).
    ExportUser(ctx context.Context, userID string) ([]ExportItem, error)

    // ReapExpired deletes content past its expiry timestamp.
    // Returns the number of items reaped.
    ReapExpired(ctx context.Context) (int64, error)
}

// ExportItem is a single content item in a SAR export.
type ExportItem struct {
    Ref       ContentRef
    Content   []byte
    CreatedAt time.Time
    ExpiresAt *time.Time
}

// StoreOption configures a Store call.
type StoreOption func(*storeConfig)

// WithExpiry sets a TTL on the stored content.
func WithExpiry(d time.Duration) StoreOption

// Sentinel errors.
var (
    ErrErased   = errors.New("mask: user content has been erased")
    ErrNotFound = errors.New("mask: content not found")
)
```

### 4.3 Encryption Architecture

```
                ┌─────────────────────────────┐
                │     Master KEK              │
                │  (OpenBao / env var)        │
                │  AES-256, never persisted   │
                │  in database                │
                └──────────┬──────────────────┘
                           │ wraps/unwraps
                ┌──────────▼──────────────────┐
                │  Per-User DEK               │
                │  vault_keys table           │
                │  wrapped_key = AES-KW(DEK)  │
                │  one active key per user    │
                └──────────┬──────────────────┘
                           │ encrypts/decrypts
                ┌──────────▼──────────────────┐
                │  Content                    │
                │  vault_content table        │
                │  AES-256-GCM(plaintext)     │
                │  unique nonce per row       │
                └─────────────────────────────┘
```

**Key lifecycle:**

1. First `Store()` for a user → generate random 256-bit DEK, wrap with
   master KEK via AES-KW, persist to `vault_keys`.
2. Subsequent `Store()` calls → load and unwrap the user's active DEK.
3. `Retrieve()` → load wrapped DEK from `vault_keys`, unwrap, decrypt
   content.
4. `EraseUser()` → set `destroyed = true` on all key rows for the user.
   `Retrieve()` returns `ErrErased`. Background reaper deletes ciphertext
   rows with destroyed keys.

**Why not HKDF (derive keys from master + userID)?**
HKDF is simpler but means the master key can always re-derive user keys.
Erasure would require maintaining a denylist of erased users, and a master
key rotation would require re-deriving all keys. Random DEKs with wrapping
give true crypto-shredding — once the wrapped key row is gone, the content
is irrecoverable regardless of master key.

### 4.4 Schema

```sql
-- axon-mask/migrations/001_create_vault.sql

CREATE TABLE vault_keys (
    user_id     TEXT NOT NULL,
    key_id      TEXT NOT NULL,
    wrapped_key BYTEA NOT NULL,       -- DEK encrypted by master KEK (AES-KW)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    destroyed   BOOLEAN NOT NULL DEFAULT false,

    PRIMARY KEY (user_id, key_id)
);

CREATE TABLE vault_content (
    id          TEXT PRIMARY KEY,      -- content ID (e.g. "cnt_a1b2c3d4e5f6")
    user_id     TEXT NOT NULL,
    key_id      TEXT NOT NULL,         -- which DEK version encrypted this
    nonce       BYTEA NOT NULL,        -- AES-GCM nonce (12 bytes)
    ciphertext  BYTEA NOT NULL,        -- AES-256-GCM encrypted content
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,           -- NULL = no expiry

    FOREIGN KEY (user_id, key_id) REFERENCES vault_keys(user_id, key_id)
);

CREATE INDEX idx_vault_content_user ON vault_content(user_id);
CREATE INDEX idx_vault_content_expires ON vault_content(expires_at)
    WHERE expires_at IS NOT NULL;
CREATE INDEX idx_vault_keys_destroyed ON vault_keys(user_id)
    WHERE destroyed = true;
```

### 4.5 How Events Change

Events stop embedding PII. They reference vaulted content instead.

#### axon-chat: MessageAppended

**Before:**
```go
type MessageAppended struct {
    ID         string  `json:"id"`
    Role       string  `json:"role"`
    Content    string  `json:"content"`            // PII inline
    Thinking   string  `json:"thinking,omitempty"` // PII inline
    ToolCalls  string  `json:"tool_calls,omitempty"`
    Images     string  `json:"images,omitempty"`
    DurationMs *int64  `json:"duration_ms,omitempty"`
}
```

**After:**
```go
type MessageAppended struct {
    ID            string  `json:"id"`
    Role          string  `json:"role"`
    ContentRef    string  `json:"content_ref,omitempty"`    // "mask:cnt_..."
    ThinkingRef   string  `json:"thinking_ref,omitempty"`   // "mask:cnt_..."
    ToolCallsRef  string  `json:"tool_calls_ref,omitempty"` // "mask:cnt_..."
    ImagesRef     string  `json:"images_ref,omitempty"`     // "mask:cnt_..."
    DurationMs    *int64  `json:"duration_ms,omitempty"`

    // Deprecated: present in pre-vault events only. Projectors check
    // ContentRef first; fall back to Content for historical events.
    Content    string `json:"content,omitempty"`
    Thinking   string `json:"thinking,omitempty"`
    ToolCalls  string `json:"tool_calls,omitempty"`
    Images     string `json:"images,omitempty"`
}
```

#### axon-chat: Emission path

```go
// handler.go — emitting a user message
contentRef, err := vault.Store(ctx, userID, []byte(msg.Content))
if err != nil { ... }

emitEvent(ctx, es, stream, MessageAppended{
    ID:         msgID,
    Role:       "user",
    ContentRef: string(contentRef),
    DurationMs: duration,
})
```

#### axon-chat: Read path (ConversationProjector)

The projector stores the ref, not the content:

```go
func (p *ConversationProjector) Handle(ctx context.Context, e fact.Event) error {
    // ...
    case "message.appended":
        var data MessageAppended
        json.Unmarshal(e.Data, &data)

        msg := Message{
            ID:         data.ID,
            Role:       data.Role,
            ContentRef: data.ContentRef,
            DurationMs: data.DurationMs,
        }

        // Backward compat: pre-vault events have inline content
        if data.ContentRef == "" {
            msg.Content = data.Content
            msg.Thinking = data.Thinking
        }

        return p.store.AppendMessage(ctx, convID, msg)
}
```

The `GetMessages` handler resolves refs at read time:

```go
func (h *messagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    msgs, _ := h.store.GetMessages(ctx, convID)
    resolved := h.resolver.ResolveMessages(ctx, msgs)
    axon.WriteJSON(w, http.StatusOK, resolved)
}
```

Where `ResolveMessages` is a thin helper:

```go
// resolver.go
type Resolver struct {
    vault mask.Vault
}

func (r *Resolver) ResolveMessages(ctx context.Context, msgs []Message) []Message {
    // Collect all refs
    var refs []mask.ContentRef
    for _, m := range msgs {
        if m.ContentRef != "" { refs = append(refs, mask.ContentRef(m.ContentRef)) }
        // ... ThinkingRef, ToolCallsRef, ImagesRef
    }

    // Single batch fetch
    resolved, _ := r.vault.RetrieveBatch(ctx, refs)

    // Hydrate
    out := make([]Message, len(msgs))
    for i, m := range msgs {
        out[i] = m
        if m.ContentRef != "" {
            if b, ok := resolved[mask.ContentRef(m.ContentRef)]; ok {
                out[i].Content = string(b)
            } else {
                out[i].Content = "[content erased]"
            }
        }
        // ... same for Thinking, ToolCalls, Images
    }
    return out
}
```

#### axon-memo: MemoryExtracted

**Before:**
```go
type MemoryExtracted struct {
    MemoryID       string  `json:"memory_id"`
    ConversationID string  `json:"conversation_id"`
    AgentSlug      string  `json:"agent_slug"`
    UserID         string  `json:"user_id"`
    MemoryType     string  `json:"memory_type"`
    Content        string  `json:"content"`       // PII inline
    Importance     float64 `json:"importance"`
}
```

**After:**
```go
type MemoryExtracted struct {
    MemoryID       string  `json:"memory_id"`
    ConversationID string  `json:"conversation_id"`
    AgentSlug      string  `json:"agent_slug"`
    UserID         string  `json:"user_id"`
    MemoryType     string  `json:"memory_type"`
    ContentRef     string  `json:"content_ref"`   // "mask:cnt_..."
    Importance     float64 `json:"importance"`

    // Deprecated: pre-vault events only
    Content string `json:"content,omitempty"`
}
```

Memory recall resolves refs via `RetrieveBatch` the same way. Memories
with erased content are excluded from recall results.

Retention policies are applied at vault time:

```go
// Emotional memories: 90-day TTL
vault.Store(ctx, userID, []byte(mem.Content), mask.WithExpiry(90*24*time.Hour))

// Episodic memories: 1-year TTL
vault.Store(ctx, userID, []byte(mem.Content), mask.WithExpiry(365*24*time.Hour))

// Semantic memories: no expiry (but erased with user)
vault.Store(ctx, userID, []byte(mem.Content))
```

### 4.6 Embedding Handling (Option D)

Embeddings remain in `axon-memo`'s memory store as unencrypted vectors.
They are required for live vector similarity search and cannot be
meaningfully encrypted without destroying search capability.

**On user erasure:**

The erasure orchestrator (see §4.8) calls `MemoryStore.DeleteByUser(ctx, userID)`
which deletes all memory rows (including embeddings) for that user.

**Documented risk:** Embedding inversion attacks can approximate original
text given white-box access to the embedding model. This is:

- Low likelihood (requires DB access + model weights)
- Low impact (approximate reconstruction, not verbatim)
- Fully mitigated by erasure (embeddings deleted, not orphaned)

This risk acceptance should be documented in the Privacy Impact Assessment.

### 4.7 Subject Access Requests

`axon-mask` provides `ExportUser()` which returns all vaulted content for
a user. The erasure orchestrator enriches this with non-vaulted data:

```go
type SARExport struct {
    // From axon-mask vault
    VaultContent []mask.ExportItem `json:"vault_content"`

    // From axon-auth (mutable stores, already deletable)
    User         *auth.User        `json:"user"`
    Sessions     []*auth.Session   `json:"sessions"`

    // From axon-memo (non-vaulted metadata)
    Memories     []memo.Memory     `json:"memories"`
    Relationship *memo.Metrics     `json:"relationship_metrics"`

    // From axon-look (analytics — UserID only, no content)
    AnalyticsNote string           `json:"analytics_note"`

    ExportedAt   time.Time         `json:"exported_at"`
}
```

### 4.8 Erasure Orchestration

User erasure is a cross-cutting operation. It lives in the **composition
root** (the binary that assembles all services), not in any single library.

```go
// Orchestrated erasure — called from admin API or CLI
func EraseUser(ctx context.Context, userID string, deps ErasureDeps) error {
    var errs []error

    // 1. Destroy vault keys (makes all vaulted content unreadable)
    if err := deps.Vault.EraseUser(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("vault: %w", err))
    }

    // 2. Delete embeddings and memory rows
    if err := deps.MemoryStore.DeleteByUser(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("memo: %w", err))
    }

    // 3. Delete auth data (user, sessions, passkeys, invites)
    if err := deps.UserStore.DeleteUser(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("auth: %w", err))
    }
    if err := deps.SessionStore.DeleteUserSessions(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("sessions: %w", err))
    }

    // 4. Emit erasure event (for audit trail — no PII in event)
    deps.EventStore.Append(ctx, "erasure-log", []fact.Event{{
        ID:   uuid.NewString(),
        Type: "user.erased",
        Data: json.RawMessage(fmt.Sprintf(`{"user_id":%q}`, userID)),
    }})

    // 5. Audit log
    slog.Info("user erased", "user_id", userID, "errors", len(errs))

    return errors.Join(errs...)
}
```

### 4.9 Module Dependency Graph (Updated)

```
axon-mask       ─── PII vault (no deps except database/sql, crypto)

axon-fact       ─── event sourcing (unchanged, no dependency on mask)
axon-tool       ─── tool definitions (unchanged)
axon-loop       ─── conversation loop (unchanged)
axon-talk       ─── LLM adapters (unchanged)

axon-chat       ─── chat service (adds dependency on axon-mask)
axon-memo       ─── memory service (adds dependency on axon-mask)

axon-auth       ─── authentication (unchanged — already mutable/deletable)
axon-look       ─── analytics (unchanged — no PII content, only user IDs)
```

`axon-mask` is a leaf dependency with no axon imports. It depends only on
`database/sql`, `crypto/*`, and the standard library.

### 4.10 Migration Path

**Phase 1: Dual-read (backward compatible)**

Deploy axon-mask. Update axon-chat and axon-memo to vault new content and
emit refs. Projectors and read paths handle both formats:

```go
// Projector: accept both old and new format
if data.ContentRef != "" {
    msg.ContentRef = data.ContentRef
} else {
    msg.Content = data.Content  // legacy inline
}
```

Old events with inline PII continue to work. New events use refs.

**Phase 2: Backfill (optional, for full compliance)**

A migration tool reads historical event streams, vaults the inline PII,
and rewrites the event `data` field in-place:

```sql
-- Pseudocode: replace inline content with ref
UPDATE events
SET data = jsonb_set(data, '{content_ref}', to_jsonb(vault_ref))
       - 'content'
WHERE type = 'message.appended'
  AND data ? 'content'
  AND NOT data ? 'content_ref';
```

This is a one-time migration. It breaks strict immutability but the
events table is already the system's private store — no external consumers
rely on content stability.

**Phase 3: Cleanup**

Remove deprecated `Content`/`Thinking`/`ToolCalls`/`Images` fields from
event structs. Remove dual-read logic from projectors.

## 5. Retention Policy Defaults

| Content Type | Source | Default TTL | Rationale |
|---|---|---|---|
| Conversation messages | axon-chat | No expiry | User controls via conversation delete |
| Agent system prompts | axon-chat | No expiry | Configuration, not transient |
| Emotional memories | axon-memo | 90 days | High sensitivity, decays in relevance |
| Episodic memories | axon-memo | 1 year | Contextual, moderate shelf life |
| Semantic memories | axon-memo | No expiry | Durable facts (erased with user) |
| Relationship metrics | axon-memo | No expiry | Not vaulted (deleted on erasure) |
| Personality context | axon-memo | No expiry | Not vaulted (deleted on erasure) |

All content is erased immediately on `EraseUser` regardless of TTL.

## 6. Performance Considerations

**Write path:** One additional DB write per content item (vault INSERT).
The vault write can share the same Postgres transaction as the event
Append if co-located, or be a separate write if the vault is on a
different database.

**Read path:** `RetrieveBatch` adds one SELECT + in-process decryption per
read request. For a conversation with 100 messages, this is one query
returning 100 rows, each requiring AES-GCM decrypt (~microseconds per
item). The DEK unwrap is cached per-user for the request lifetime.

**Mitigation:** The read model can cache resolved content in-process
(short TTL, never persisted). For hot conversations, content will be in
the Go process memory from the previous read.

## 7. Security Properties

| Property | Mechanism |
|---|---|
| Confidentiality at rest | AES-256-GCM per content item |
| Per-user key isolation | Random DEK per user, wrapped by master KEK |
| Crypto-shredding | Destroy DEK → content irrecoverable |
| Key management | Master KEK in OpenBao, never in database |
| Nonce uniqueness | Random 12-byte nonce per content item |
| Audit trail | Erasure events in axon-fact (no PII in event) |
| Forward secrecy | Key rotation creates new DEK; old DEK destroyed on rotation |

## 8. Open Items

- [ ] Decide whether `axon-mask` Postgres store shares the same database as
      axon-fact or runs on a separate instance (separate = stronger isolation,
      same = simpler ops)
- [ ] Define admin API / CLI surface for triggering erasure and SAR export
- [ ] Determine if `axon-look` analytics events need anonymisation on erasure
      (currently they contain `user_id` but no content)
- [ ] Write integration tests covering the full erasure → retrieve → ErrErased
      flow
- [ ] Benchmark `RetrieveBatch` at conversation scale (100-500 messages)
