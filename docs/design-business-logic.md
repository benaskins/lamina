# Designing Business Logic for Axon Services

> How would you write business logic if you were building a service from axon
> components? A general ledger as the example.

## The Pattern Today

Looking across the existing axon services, the current layering is:

```
handler (HTTP) → store (persistence interface) → postgres (implementation)
```

Handlers do double duty — they are both the HTTP translation layer _and_ the
place where business logic lives. This works well for CRUD-shaped services
like axon-gate (create approval, resolve approval) or axon-auth (register,
login, validate). The "business logic" in those cases is thin enough that it
doesn't need its own layer.

But a general ledger is a different animal.

## Where the Pattern Breaks Down

A ledger has rules:

- Every transaction must balance (debits == credits)
- Accounts have types (asset, liability, equity, revenue, expense) with
  normal balance directions
- Period closes lock prior entries
- Adjusting entries, reversals, and accruals have specific sequencing rules
- Trial balances and financial statements are derived computations

If you put all of this in handlers, you get:

```go
func (h *ledgerHandler) PostEntry(w http.ResponseWriter, r *http.Request) {
    // decode request
    // validate account exists
    // check period is open
    // validate debits == credits
    // check account types and normal balances
    // persist the entry
    // update running balances
    // respond
}
```

That handler is doing HTTP concerns _and_ accounting rules. The accounting
rules can't be tested without spinning up an HTTP request. And if you later
need to post an entry from a batch import, a webhook, or a CLI command, you
have to extract the logic anyway.

## The Missing Layer: Domain Services

The axon pattern needs one more layer for logic-heavy domains:

```
handler (HTTP) → service (business logic) → store (persistence interface)
```

The handler stays thin — decode, delegate, encode. The service owns the
domain rules. The store remains a persistence interface.

## General Ledger: Concrete Design

### Types (`types.go`)

Pure data. No methods with side effects. JSON tags for API, db tags if using
sqlx-style scanning.

```go
package ledger

import "time"

type AccountType string

const (
    Asset     AccountType = "asset"
    Liability AccountType = "liability"
    Equity    AccountType = "equity"
    Revenue   AccountType = "revenue"
    Expense   AccountType = "expense"
)

// NormalBalance returns the sign that increases this account type.
func (t AccountType) NormalBalance() EntryType {
    switch t {
    case Asset, Expense:
        return Debit
    default:
        return Credit
    }
}

type EntryType string

const (
    Debit  EntryType = "debit"
    Credit EntryType = "credit"
)

type Account struct {
    ID        string      `json:"id"`
    Code      string      `json:"code"`
    Name      string      `json:"name"`
    Type      AccountType `json:"type"`
    ParentID  *string     `json:"parent_id,omitempty"`
    CreatedAt time.Time   `json:"created_at"`
}

type LineItem struct {
    AccountID string    `json:"account_id"`
    Type      EntryType `json:"type"`
    Amount    int64     `json:"amount"` // cents, no floats
    Memo      string    `json:"memo,omitempty"`
}

type JournalEntry struct {
    ID        string     `json:"id"`
    Date      time.Time  `json:"date"`
    Reference string     `json:"reference"`
    Lines     []LineItem `json:"lines"`
    PeriodID  string     `json:"period_id"`
    CreatedAt time.Time  `json:"created_at"`
    CreatedBy string     `json:"created_by"`
    Reversed  bool       `json:"reversed"`
}

type Period struct {
    ID     string    `json:"id"`
    Name   string    `json:"name"`
    Start  time.Time `json:"start"`
    End    time.Time `json:"end"`
    Closed bool      `json:"closed"`
}

type AccountBalance struct {
    AccountID string `json:"account_id"`
    Debits    int64  `json:"debits"`
    Credits   int64  `json:"credits"`
    Balance   int64  `json:"balance"` // signed, positive = normal direction
}
```

### Store Interface (`store.go`)

Following the axon convention — a Go interface defining what persistence can
do, with no implementation details. The store is deliberately "dumb" — it
reads and writes, it does not enforce business rules.

```go
package ledger

import "context"

type Store interface {
    // Accounts
    CreateAccount(ctx context.Context, account *Account) error
    GetAccount(ctx context.Context, id string) (*Account, error)
    ListAccounts(ctx context.Context) ([]Account, error)

    // Journal entries
    CreateJournalEntry(ctx context.Context, entry *JournalEntry) error
    GetJournalEntry(ctx context.Context, id string) (*JournalEntry, error)
    ListJournalEntries(ctx context.Context, periodID string) ([]JournalEntry, error)
    VoidJournalEntry(ctx context.Context, id string) error

    // Periods
    GetPeriod(ctx context.Context, id string) (*Period, error)
    GetPeriodForDate(ctx context.Context, date time.Time) (*Period, error)
    ClosePeriod(ctx context.Context, id string) error

    // Balances — computed or cached, implementation decides
    GetAccountBalance(ctx context.Context, accountID string, asOf time.Time) (*AccountBalance, error)
    GetTrialBalance(ctx context.Context, asOf time.Time) ([]AccountBalance, error)
}
```

### Business Logic (`service.go`)

This is the new layer. The service takes a store (and any other
dependencies) via constructor injection, and all domain rules live here.

```go
package ledger

import (
    "context"
    "errors"
    "fmt"
    "time"
)

var (
    ErrUnbalanced     = errors.New("entry does not balance: total debits must equal total credits")
    ErrPeriodClosed   = errors.New("period is closed")
    ErrNoLines        = errors.New("journal entry must have at least two lines")
    ErrAccountMissing = errors.New("account not found")
    ErrAlreadyVoided  = errors.New("entry already voided")
)

// Service enforces accounting rules. It is the only path for
// mutations — handlers call this, not the store directly.
type Service struct {
    store Store
}

func NewService(store Store) *Service {
    return &Service{store: store}
}

// PostEntry validates and records a journal entry.
func (s *Service) PostEntry(ctx context.Context, entry *JournalEntry) error {
    // Rule: must have at least two lines
    if len(entry.Lines) < 2 {
        return ErrNoLines
    }

    // Rule: debits must equal credits
    var totalDebits, totalCredits int64
    for _, line := range entry.Lines {
        switch line.Type {
        case Debit:
            totalDebits += line.Amount
        case Credit:
            totalCredits += line.Amount
        default:
            return fmt.Errorf("invalid entry type: %s", line.Type)
        }
    }
    if totalDebits != totalCredits {
        return ErrUnbalanced
    }

    // Rule: all referenced accounts must exist
    for _, line := range entry.Lines {
        if _, err := s.store.GetAccount(ctx, line.AccountID); err != nil {
            return fmt.Errorf("%w: %s", ErrAccountMissing, line.AccountID)
        }
    }

    // Rule: period must be open
    period, err := s.store.GetPeriodForDate(ctx, entry.Date)
    if err != nil {
        return fmt.Errorf("no period for date %s: %w", entry.Date.Format("2006-01-02"), err)
    }
    if period.Closed {
        return ErrPeriodClosed
    }
    entry.PeriodID = period.ID

    return s.store.CreateJournalEntry(ctx, entry)
}

// ReverseEntry creates a new entry that exactly offsets the original.
func (s *Service) ReverseEntry(ctx context.Context, id string, date time.Time, by string) (*JournalEntry, error) {
    original, err := s.store.GetJournalEntry(ctx, id)
    if err != nil {
        return nil, err
    }
    if original.Reversed {
        return nil, ErrAlreadyVoided
    }

    // Flip debits and credits
    reversal := &JournalEntry{
        Date:      date,
        Reference: fmt.Sprintf("REV:%s", original.Reference),
        CreatedBy: by,
    }
    for _, line := range original.Lines {
        flipped := Debit
        if line.Type == Debit {
            flipped = Credit
        }
        reversal.Lines = append(reversal.Lines, LineItem{
            AccountID: line.AccountID,
            Type:      flipped,
            Amount:    line.Amount,
            Memo:      fmt.Sprintf("Reversal of %s", original.ID),
        })
    }

    if err := s.PostEntry(ctx, reversal); err != nil {
        return nil, fmt.Errorf("reversal failed validation: %w", err)
    }

    if err := s.store.VoidJournalEntry(ctx, id); err != nil {
        return nil, err
    }

    return reversal, nil
}

// ClosePeriod marks a period as closed after verifying it balances.
func (s *Service) ClosePeriod(ctx context.Context, periodID string) error {
    period, err := s.store.GetPeriod(ctx, periodID)
    if err != nil {
        return err
    }
    if period.Closed {
        return ErrPeriodClosed
    }

    // Rule: trial balance must balance before closing
    tb, err := s.store.GetTrialBalance(ctx, period.End)
    if err != nil {
        return fmt.Errorf("cannot compute trial balance: %w", err)
    }
    var totalDebits, totalCredits int64
    for _, ab := range tb {
        totalDebits += ab.Debits
        totalCredits += ab.Credits
    }
    if totalDebits != totalCredits {
        return fmt.Errorf("trial balance does not balance: debits=%d credits=%d", totalDebits, totalCredits)
    }

    return s.store.ClosePeriod(ctx, periodID)
}

// TrialBalance returns the trial balance as of a given date.
// This is a read-only operation, no rules to enforce — but
// it lives on the service so handlers don't reach past it.
func (s *Service) TrialBalance(ctx context.Context, asOf time.Time) ([]AccountBalance, error) {
    return s.store.GetTrialBalance(ctx, asOf)
}
```

### Handlers (`handler.go`)

Thin. Decode, call service, encode. Following the axon convention of a
handler struct with injected dependencies.

```go
package ledger

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/benaskins/axon"
)

type handler struct {
    svc *Service
}

func (h *handler) postEntry(w http.ResponseWriter, r *http.Request) {
    var entry JournalEntry
    if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
        axon.WriteError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    entry.CreatedBy = axon.UserFromContext(r.Context())

    if err := h.svc.PostEntry(r.Context(), &entry); err != nil {
        switch {
        case errors.Is(err, ErrUnbalanced), errors.Is(err, ErrNoLines),
            errors.Is(err, ErrAccountMissing):
            axon.WriteError(w, http.StatusUnprocessableEntity, err.Error())
        case errors.Is(err, ErrPeriodClosed):
            axon.WriteError(w, http.StatusConflict, err.Error())
        default:
            axon.WriteError(w, http.StatusInternalServerError, "failed to post entry")
        }
        return
    }

    axon.WriteJSON(w, http.StatusCreated, entry)
}

func (h *handler) getTrialBalance(w http.ResponseWriter, r *http.Request) {
    asOf := time.Now()
    if v := r.URL.Query().Get("as_of"); v != "" {
        var err error
        asOf, err = time.Parse("2006-01-02", v)
        if err != nil {
            axon.WriteError(w, http.StatusBadRequest, "invalid as_of date")
            return
        }
    }

    tb, err := h.svc.TrialBalance(r.Context(), asOf)
    if err != nil {
        axon.WriteError(w, http.StatusInternalServerError, "failed to compute trial balance")
        return
    }

    axon.WriteJSON(w, http.StatusOK, tb)
}
```

### Server Wiring (`server.go`)

Following the axon convention — `NewServer()` takes dependencies, registers
routes, returns a configured mux.

```go
package ledger

import "net/http"

func NewServer(store Store) http.Handler {
    svc := NewService(store)
    h := &handler{svc: svc}

    mux := http.NewServeMux()

    mux.HandleFunc("POST /api/entries", h.postEntry)
    mux.HandleFunc("GET /api/entries/{id}", h.getEntry)
    mux.HandleFunc("POST /api/entries/{id}/reverse", h.reverseEntry)

    mux.HandleFunc("GET /api/accounts", h.listAccounts)
    mux.HandleFunc("POST /api/accounts", h.createAccount)

    mux.HandleFunc("GET /api/trial-balance", h.getTrialBalance)

    mux.HandleFunc("POST /api/periods/{id}/close", h.closePeriod)

    return mux
}
```

### Test Support (`ledgertest/store.go`)

Following the `*test/` convention from axon-chat, axon-auth, axon-gate — an
in-memory store implementation for testing.

```go
package ledgertest

import (
    "context"
    "sync"

    ledger ".."
)

type MemoryStore struct {
    mu       sync.RWMutex
    accounts map[string]*ledger.Account
    entries  map[string]*ledger.JournalEntry
    periods  map[string]*ledger.Period
}

func NewMemoryStore() *MemoryStore { /* ... */ }
```

The key thing: **service tests don't need HTTP**. You instantiate the
service with a `MemoryStore` and test the business rules directly:

```go
func TestPostEntry_MustBalance(t *testing.T) {
    store := ledgertest.NewMemoryStore()
    svc := ledger.NewService(store)

    // seed accounts
    store.CreateAccount(ctx, &ledger.Account{ID: "cash", Type: ledger.Asset})
    store.CreateAccount(ctx, &ledger.Account{ID: "revenue", Type: ledger.Revenue})
    // seed an open period
    store.SeedPeriod(&ledger.Period{ID: "2024-01", Start: ..., End: ...})

    entry := &ledger.JournalEntry{
        Lines: []ledger.LineItem{
            {AccountID: "cash", Type: ledger.Debit, Amount: 1000},
            {AccountID: "revenue", Type: ledger.Credit, Amount: 999}, // off by 1
        },
    }

    err := svc.PostEntry(context.Background(), entry)
    if !errors.Is(err, ledger.ErrUnbalanced) {
        t.Fatalf("expected ErrUnbalanced, got %v", err)
    }
}
```

## The Principle

The axon ecosystem follows a consistent contract:

| Layer | Owns | Testable via |
|-------|------|-------------|
| **Types** | Data shape, JSON serialization | Compile-time |
| **Store** (interface) | Persistence contract | — |
| **Store** (postgres) | SQL, migrations | Integration tests with real DB |
| **Store** (memory) | Test double | Unit tests |
| **Service** | Business rules, domain invariants | Unit tests with memory store |
| **Handler** | HTTP decode/encode, status codes | `httptest` with memory store |
| **Server** | Route wiring, dependency assembly | `httptest` integration |

For CRUD-thin services (axon-gate, axon-auth), the service layer is
unnecessary overhead — handlers can call the store directly. That's fine.

For rule-heavy domains (ledger, payroll, inventory, billing), the service
layer is where the real work happens. It keeps business logic:

- **Testable** without HTTP or a database
- **Reusable** across entry points (API, CLI, batch import, webhook)
- **Readable** — the handler is "how to talk HTTP", the service is "how
  accounting works"

The axon toolkit doesn't enforce this — it gives you `ListenAndServe`, `WriteJSON`,
`WriteError`, SSE, auth middleware, and database setup. You compose what you need.
The service layer is a natural extension of the existing conventions when the
domain demands it.
