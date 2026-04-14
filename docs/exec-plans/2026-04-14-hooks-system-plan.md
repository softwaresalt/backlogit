---
title: "Hooks System: Internal Lifecycle Hooks and External Webhook Dispatch"
date: 2026-04-14
origin: ".backlogit/archive/007-DL.md, .backlogit/queue/032-DL.md"
status: reviewed
---

<!-- plan-review-attempt: 2 -->

# Hooks System: Internal Lifecycle Hooks and External Webhook Dispatch

## Problem Frame

backlogit lifecycle operations (create, update, archive, ship, adopt) execute as
isolated functions with no extension points. This prevents validation rules from
being enforced uniformly (orphaned tasks slip through), external systems from
receiving change notifications, and the existing 008-DL agent-automation event
layer from connecting to actual mutation events.

This plan introduces a two-layer hooks system:

1. **Internal lifecycle hooks** (007-DL): a pre/post callback registry wired
   into every lifecycle mutation, providing synchronous validation (pre) and
   async side-effects (post).
2. **External webhook dispatch** (032-DL Phase 1): an HTTP POST notification
   system that registers as a post-hook, pushing change events to configured
   endpoints (Slack, Teams, custom URLs).

The hook engine also provides the missing interface contract that the 008-DL
agent-automation plan depends on: once hooks fire, the existing
`HookEventWriter` can emit events to `hooks_queue.jsonl` as a built-in
post-hook rather than requiring manual emit calls scattered through the
codebase.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Pre-hooks can reject mutations by returning an error | 007-DL Chosen Direction |
| R2 | Post-hooks are fire-and-forget; failures log warnings but never rollback | 007-DL Chosen Direction |
| R3 | Built-in hooks ship as defaults: parent enforcement, status validation, event emission | 007-DL Chosen Direction |
| R4 | hooks.yaml controls which built-in hooks are active | 007-DL Chosen Direction |
| R5 | Hook context carries item ID, artifact type, old/new values, actor | 007-DL Chosen Direction |
| R6 | Hook points: CreateArtifact, UpdateArtifact, ArchiveItem, ShipShipment, AdoptItem, MoveShipmentStatus | 007-DL Chosen Direction |
| R7 | Post-hooks emit events to hooks_queue.jsonl via existing HookEventWriter | 008-DL interface contract (see origin: docs/exec-plans/2026-04-11-agent-automation-hooks-plan.md) |
| R8 | Generic webhook dispatcher sends HTTP POST to configured URLs | 032-DL Chosen Direction |
| R9 | Webhook payloads use a stable JSON schema: event_type, item_id, title, status, timestamp, details | 032-DL Chosen Direction |
| R10 | Webhook dispatch is async; failures log to telemetry but never fail operations | 032-DL Chosen Direction |
| R11 | hooks.yaml gains a notifications section with endpoint URLs, event filters, auth headers | 032-DL Chosen Direction |
| R12 | Rate limiter prevents webhook storms during bulk operations | 032-DL Chosen Direction |
| R13 | Environment variable expansion for sensitive webhook URLs | 032-DL Chosen Direction |

## Scope Boundaries

### In Scope

* `internal/hooks/` package: HookPoint, HookPhase, HookContext, HookFunc, HookRunner
* Built-in pre-hooks: status transition validation
* Built-in post-hooks: hook event emission (to existing 008-DL JSONL writer), index invalidation signal
* Wiring HookRunner into the Workspace struct and NewWorkspace constructor
* Instrumenting 6 lifecycle methods with pre/post hook calls
* WebhookNotifier: HTTP POST dispatcher with async dispatch, rate limiting, env var expansion
* hooks.yaml expansion: lifecycle hook configuration and notification endpoints
* Contract tests for hook firing, pre-hook rejection, post-hook failure isolation

### Non-Goals

* User-defined hooks via Go plugins (deferred to v2)
* Customizable webhook payloads via Go templates (deferred to v2)
* GitHub-specific webhook integration (032-DL Phase 2)
* Bidirectional sync or inbound webhooks (032-DL Phase 3)
* `backlogit_test_webhook` CLI command (deferred to v2)
* Retry/exponential backoff for failed webhooks (v1 is fire-and-forget)
* Hook points for stash mutations or `AddItemToShipment` (deferred per 008-DL plan)

### Deferred to Implementation

* Exact token bucket parameters for the webhook rate limiter
* Whether `ClaimShipment` warrants a hook point (low mutation impact, can add in v2)
* Optimal hook priority values for built-in ordering

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target
a single skill domain and produce a verifiable exit state.

### Unit 1: Hook Types and Runner

**Files:** `internal/hooks/hooks.go`
**Test files:** `internal/hooks/hooks_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/hook_events.go` for type conventions; `internal/config/schema.go` for validation tags
**Dependencies:** none (foundation unit)

**Approach:**

Define the hook system's core types:

```go
type HookPoint string
const (
    HookCreateArtifact     HookPoint = "create_artifact"
    HookUpdateArtifact     HookPoint = "update_artifact"
    HookArchiveItem        HookPoint = "archive_item"
    HookShipShipment       HookPoint = "ship_shipment"
    HookAdoptItem          HookPoint = "adopt_item"
    HookMoveShipmentStatus HookPoint = "move_shipment_status"
)

type HookPhase string
const (
    PhasePre  HookPhase = "pre"
    PhasePost HookPhase = "post"
)

type HookContext struct {
    ItemID       string
    ArtifactType string
    OldValues    map[string]any
    NewValues    map[string]any
    Actor        string
    Workspace    string // root path for safe resolution
    TopLevel     bool   // false when called from within another hooked operation
}

type HookFunc func(ctx context.Context, hc HookContext) error

type HookRegistration struct {
    Name     string
    Priority int // lower fires first; default 100
    Fn       HookFunc
}
```

`HookRunner` stores registrations keyed by `HookPoint + HookPhase`. It exposes:

* `Register(point HookPoint, phase HookPhase, reg HookRegistration)`
* `FirePre(ctx, point, hookCtx) error` — snapshots pre-hook registrations under read lock, releases lock, then fires in priority order; first error stops execution and returns
* `FirePost(ctx, point, hookCtx)` — snapshots post-hook registrations under read lock, releases lock, then fires in priority order; errors are logged via `slog.Warn`, never returned

The runner uses a `sync.RWMutex` to protect registration (write) vs snapshot
(read). The lock is NOT held during callback execution — the runner copies the
registration slice under the read lock, releases it, then iterates the copy.
This prevents deadlocks if a callback attempts registration and avoids holding
the lock across arbitrary user code.

> [!IMPORTANT]
> **Nested operation boundary**: Complex operations like `ShipShipment` call
> `setArtifactStatus`, `ArchiveItem`, and `MoveShipmentStatus` internally. If
> each inner call fires hooks, the same mutation produces duplicate events and
> potentially conflicting pre-hook validation. The `TopLevel` field solves this:
>
> * Top-level callers set `TopLevel: true` in their HookContext
> * Inner operations called within a hooked operation set `TopLevel: false`
> * Post-hooks that emit external events (EmitHookEvent, WebhookNotifier)
>   check `TopLevel` and skip when false, preventing duplicate notifications
> * Pre-hooks (validation) always fire regardless of `TopLevel` to maintain
>   invariant enforcement
>
> Implementation: internal lifecycle helpers that are called from within a
> top-level operation accept an explicit `topLevel bool` parameter. Top-level
> MCP handlers and CLI commands always pass `true`. ShipShipment passes `false`
> when calling setArtifactStatus, ArchiveItem, and MoveShipmentStatus. This
> avoids `context.Value` (which creates hidden coupling and SA1029 lint risk)
> in favor of idiomatic explicit Go parameters.

**Verification:**

* `go test ./internal/hooks/...` passes
* Tests cover: registration, priority ordering, pre-hook error rejection, post-hook error swallowing, concurrent registration+firing, empty hook list (no-op), TopLevel=false skips external post-hooks, snapshot-before-execute prevents deadlock when callback attempts registration

### Unit 2: Built-in Pre-Hooks

**Files:** `internal/hooks/builtin_pre.go`
**Test files:** `internal/hooks/builtin_pre_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** orphaned tasks compound learning (see origin: docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md)
**Dependencies:** Unit 1

**Approach:**

> [!IMPORTANT]
> `CreateArtifact` already validates parent hierarchy via `validateArtifactParent`
> (artifacts.go:266-309). Do NOT duplicate that logic in a pre-hook. The existing
> inline validation is the correct enforcement point for v1. Pre-hooks in this
> unit serve as extension points for FUTURE user-defined or plugin validation,
> not to re-implement what already works.

One built-in pre-hook for v1:

1. `ValidateStatusTransition` (priority 20, pre-update): When `NewValues`
   contains a status change, validates the transition is allowed. The valid
   transition map is derived from the workspace `config.yaml` status
   definitions (queued, active, blocked, review, done, accepted, rejected,
   archived) rather than hard-coded. The default transition map covers all
   transitions documented in the backlogit workflow:
   `queued→active`, `queued→blocked`, `active→done`, `active→blocked`,
   `active→review`, `blocked→active`, `review→done`, `review→accepted`,
   `review→rejected`, `done→archived`.
   Invalid transitions return `ErrInvalidStatusTransition`. The transition
   map is loaded from the `lifecycle.transitions` section of hooks.yaml,
   falling back to the default map when absent. This allows operators to
   customize transitions without code changes.

The pre-hook accepts a `*config.WorkspaceConfig` at registration time via
closure to avoid repeated config loads. Error sentinels (`ErrHook`,
`ErrInvalidStatusTransition`) are defined in `internal/errors/errors.go` to
participate in the shared error hierarchy, following the project convention.
Hook code wraps these sentinels with `fmt.Errorf("context: %w", err)`.

A future v2 iteration may refactor the existing `validateArtifactParent` logic
into a pre-hook to consolidate all validation in the hook pipeline, but that
refactoring is out of scope for this shipment.

**Verification:**

* Tests cover: valid status transitions pass, invalid transitions rejected, missing status field is no-op, pre-hook does not duplicate parent validation, transition map derived from config status definitions
* Contract test: `tests/contract/hook_events_test.go` — assert only the 6 approved HookPoint constants emit events; no internal helpers leak hook events

### Unit 3: Built-in Post-Hooks

**Files:** `internal/hooks/builtin_post.go`
**Test files:** `internal/hooks/builtin_post_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/hook_events.go` HookEventWriter API
**Dependencies:** Unit 1; existing `internal/events/` package

**Approach:**

Two built-in post-hooks:

1. `EmitHookEvent` (priority 50, post-all): Constructs a compact event payload
   from the `HookContext` and appends it via the `HookEventAppender` interface
   (defined in `internal/hooks/`). The interface has a single method:
   `AppendEvent(ctx context.Context, event HookEventPayload) error`. The
   existing `HookEventWriter` in `internal/events/` satisfies this interface
   via an adapter registered from `core` during `NewWorkspace`. This
   decouples the hooks package from the events package.

   The event payload uses a compact, versioned schema optimized for agent
   context windows:

   ```go
   type HookEventPayload struct {
       SchemaVersion int               `json:"schema_version"` // v1
       EventType     string            `json:"event_type"`
       ItemID        string            `json:"item_id"`
       ArtifactType  string            `json:"artifact_type"`
       Actor         string            `json:"actor"`
       Timestamp     time.Time         `json:"timestamp"`
       ChangedFields []string          `json:"changed_fields,omitempty"` // field names only
       StatusDelta   *StatusDelta      `json:"status_delta,omitempty"`
       TitleDelta    *StringDelta      `json:"title_delta,omitempty"`
   }
   type StatusDelta struct {
       From string `json:"from"`
       To   string `json:"to"`
   }
   type StringDelta struct {
       From string `json:"from"`
       To   string `json:"to"`
   }
   ```

   Full `OldValues`/`NewValues` maps are NOT included in the event payload.
   Only field names that changed (`ChangedFields`) and key deltas (status,
   title) are emitted. This keeps agent-facing events compact per Constitution
   Principle IX.

   Event type maps directly from `HookPoint` (e.g. `create_artifact`,
   `update_artifact`).
   **Skips when `TopLevel` is false** to prevent duplicate events from nested
   operations (e.g. ShipShipment → ArchiveItem). Only the top-level operation
   emits the event.

2. `LogIndexStale` (priority 90, post-all): Emits a `slog.Info` message
   signaling that the SQLite index may be stale after a Markdown mutation.
   This is informational for now; the actual index sync happens within the
   lifecycle methods themselves. Future iterations may use this hook to
   trigger async rehydration.

The `EmitHookEvent` hook requires a `HookEventAppender` implementation at
registration time, passed via closure. The adapter from `internal/events/`
is injected from `core/workspace.go` during `NewWorkspace`.

**Verification:**

* Tests cover: event emission appends compact payload with correct fields, schema_version is set, event type matches hook point, ChangedFields lists only field names (no values), post-hook errors are swallowed (simulated writer failure), LogIndexStale emits log entry
* Contract test: `tests/contract/hook_events_test.go` — validate event JSON shape matches `HookEventPayload` schema, verify `backlogit_poll_hook_events` returns events with the versioned schema

### Unit 4: Lifecycle Hook Configuration and Loader

**Files:** `internal/config/schema.go`, `internal/config/defaults.go`, `internal/config/loader.go`
**Test files:** `internal/config/hooks_config_test.go` (existing), `internal/config/loader_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first (read existing config loading, then extend)
**Patterns to follow:** existing `HooksConfig` struct at `internal/config/schema.go:94-105`; `LoadRegistry()` pattern in `loader.go`
**Dependencies:** none (can proceed in parallel with Unit 1)

**Approach:**

> [!IMPORTANT]
> **Loader gap**: `config.Load()` reads `config.yaml` and `LoadRegistry()` reads
> `registry.yaml`, but NO function currently reads `hooks.yaml` back into
> `HooksConfig`. `DefaultHooksConfig()` and `WriteDefaults()` create the file,
> but there is no `LoadHooks()`. This unit MUST create the loader before any
> downstream unit can reference hooks configuration at runtime.

**Step 1: Add `LoadHooks()` function** to `loader.go`, following the same
pattern as `LoadRegistry()`:

```go
func LoadHooks(workspacePath string) (*HooksConfig, error) {
    hooksPath := filepath.Join(workspacePath, "hooks.yaml")
    data, err := os.ReadFile(hooksPath)
    if err != nil {
        if os.IsNotExist(err) {
            return DefaultHooksConfig(), nil
        }
        return nil, fmt.Errorf("read hooks: %w", err)
    }
    var cfg HooksConfig
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse hooks: %w", err)
    }
    if err := validate.Struct(&cfg); err != nil {
        return nil, fmt.Errorf("validate hooks: %w", err)
    }
    return &cfg, nil
}
```

Falls back to `DefaultHooksConfig()` when the file is absent, matching the
graceful degradation pattern of `LoadRegistry()`.

**Step 2: Expand `HooksConfig` struct** to include lifecycle and notification
sections:

```go
type HooksConfig struct {
    Enabled            bool                    `yaml:"enabled"`
    EventThresholds    HookEventThresholds     `yaml:"event_thresholds,omitempty"`
    AgentSubscriptions map[string][]string     `yaml:"agent_subscriptions,omitempty"`
    Lifecycle          LifecycleHooksConfig    `yaml:"lifecycle,omitempty"`
    Notifications      NotificationsConfig     `yaml:"notifications,omitempty"`
}

type LifecycleHooksConfig struct {
    ValidateTransition bool                `yaml:"validate_transition"`
    EmitEvents         bool                `yaml:"emit_events"`
    Transitions        map[string][]string `yaml:"transitions,omitempty"` // from-status → allowed to-statuses
}

type NotificationsConfig struct {
    Endpoints []WebhookEndpoint `yaml:"endpoints,omitempty"`
    RateLimit int               `yaml:"rate_limit_per_second,omitempty" validate:"omitempty,gte=1,lte=100"`
}

type WebhookEndpoint struct {
    URL          string            `yaml:"url" validate:"required,url"`
    EventFilter  []string          `yaml:"event_filter,omitempty"`
    Headers      map[string]string `yaml:"headers,omitempty"`
    TimeoutSecs  int               `yaml:"timeout_secs,omitempty" validate:"omitempty,gte=1,lte=60"`
}
```

**Step 3: Update `DefaultHooksConfig()`** in defaults.go to include:

```yaml
lifecycle:
  validate_transition: true
  emit_events: true
  transitions:
    queued: [active, blocked]
    active: [done, blocked, review]
    blocked: [active]
    review: [done, accepted, rejected]
    done: [archived]
notifications:
  rate_limit_per_second: 10
  endpoints: []
```

**Verification:**

* Existing hooks_config_test.go tests still pass
* New test: `LoadHooks` reads a hooks.yaml with lifecycle and notifications sections
* New test: `LoadHooks` returns defaults when file is absent
* New test: `LoadHooks` rejects invalid YAML
* New test: default config has lifecycle hooks enabled, empty endpoints

### Unit 5: Wire HookRunner into Workspace

**Files:** `internal/core/workspace.go`
**Test files:** `internal/core/workspace_test.go` (new or extend existing)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** existing NewWorkspace init sequence (config → DB → schema → migrations → recovery)
**Dependencies:** Units 1, 2, 3, 4

**Approach:**

Add `HookRunner *hooks.HookRunner` to the Workspace struct. In `NewWorkspace`,
after config loading and before the migration guard:

1. Call `config.LoadHooks(backlogitDir)` to load hooks.yaml (Unit 4 adds this)
2. Store `HooksConfig` on Workspace (new field: `Hooks *config.HooksConfig`)
3. Create a `hooks.NewHookRunner()`
4. If `Lifecycle.ValidateTransition` is true, register `ValidateStatusTransition`
   as a pre-update hook
5. If `Lifecycle.EmitEvents` is true, create a `HookEventWriter` and register
   `EmitHookEvent` as a post-hook on all points
6. Always register `LogIndexStale` as a post-hook

The runner is available to all lifecycle functions via `ws.HookRunner`.

**Verification:**

* `NewWorkspace` creates a workspace with a non-nil HookRunner
* Built-in hooks are registered based on config flags
* Disabling a config flag skips registration of that hook
* Workspace.Close still works (no new resources to close on the runner)

### Unit 6: Instrument Core Lifecycle Methods (CreateArtifact, UpdateArtifact)

**Files:** `internal/core/artifacts.go`
**Test files:** `internal/core/artifacts_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first (understand existing flow, then add hook calls)
**Patterns to follow:** existing error wrapping in artifacts.go
**Dependencies:** Units 1, 5

**Approach:**

Add pre/post hook calls to the two highest-frequency lifecycle methods:

**CreateArtifact:**
```go
// Before any file/DB writes:
hookCtx := hooks.HookContext{
    ItemID:       "", // not yet assigned
    ArtifactType: artifactType,
    NewValues:    map[string]any{"title": title, "parent_id": parentID, ...},
    Actor:        "backlogit",
    Workspace:    ws.RootPath,
}
if err := ws.HookRunner.FirePre(ctx, hooks.HookCreateArtifact, hookCtx); err != nil {
    return nil, fmt.Errorf("pre-create hook: %w", err)
}

// ... existing creation logic ...

// After successful creation:
hookCtx.ItemID = artifact.ID
hookCtx.NewValues["id"] = artifact.ID
hookCtx.NewValues["status"] = artifact.Status
ws.HookRunner.FirePost(ctx, hooks.HookCreateArtifact, hookCtx)
```

**UpdateArtifact:**
```go
// Load current state for OldValues
hookCtx := hooks.HookContext{
    ItemID:       id,
    ArtifactType: existing.ArtifactType,
    OldValues:    map[string]any{"status": existing.Status, ...},
    NewValues:    updates,
    Actor:        "backlogit",
    Workspace:    ws.RootPath,
}
if err := ws.HookRunner.FirePre(ctx, hooks.HookUpdateArtifact, hookCtx); err != nil {
    return nil, fmt.Errorf("pre-update hook: %w", err)
}

// ... existing update logic ...

ws.HookRunner.FirePost(ctx, hooks.HookUpdateArtifact, hookCtx)
```

Guard hook calls with a nil check on `ws.HookRunner` to maintain backward
compatibility with tests that create Workspace structs without full
initialization.

Top-level MCP handlers and CLI commands set `TopLevel: true`. When
`CreateArtifact` or `UpdateArtifact` is called internally from another hooked
operation (e.g., during `ShipShipment`), the caller passes `topLevel: false`
explicitly via a parameter on the internal helper. This avoids `context.Value`
(hidden coupling, SA1029 lint risk) in favor of idiomatic Go.

**Verification:**

* Existing artifact creation and update tests still pass
* New test: creating a level-2 task without parent_id is still rejected by existing `validateArtifactParent` (hooks do not interfere)
* New test: updating status with invalid transition is rejected by pre-hook
* New test: post-hook fires after successful creation (verify via mock hook)
* New test: pre-hook error prevents artifact file creation (no side effects)
* New test: TopLevel=false suppresses post-hook event emission
* Contract test: `tests/contract/hook_events_test.go` — assert CreateArtifact and UpdateArtifact hook points emit compact HookEventPayload

### Unit 7a: Instrument Lifecycle Transitions (ArchiveItem, MoveShipmentStatus)

**Files:** `internal/core/archive.go`, `internal/core/shipment.go`
**Test files:** `internal/core/archive_test.go`, `internal/core/shipment_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** Unit 6 hook call pattern
**Dependencies:** Units 1, 5, 6

**Approach:**

Apply the same pre/post pattern to two lifecycle methods that do not involve
nested operation boundaries:

* **ArchiveItem** (`archive.go`): Pre-hook with `OldValues` = current state,
  `NewValues` = `{"archived": true}`. Post-hook fires after successful archive.

* **MoveShipmentStatus** (`shipment.go`): Pre-hook with `OldValues` =
  `{"status": old}`, `NewValues` = `{"status": new}`. Post-hook fires after
  status transition completes.

Each method gets the same nil-guard pattern as Unit 6. HookContext construction
follows the same field conventions.

**Verification:**

* Existing tests for both methods still pass
* New test per method: pre-hook rejection prevents the operation
* New test per method: post-hook fires after success
* Integration test: archive with emit_events enabled produces a hook event in hooks_queue.jsonl

### Unit 7b: Instrument Nested Lifecycle Transitions (ShipShipment, AdoptItem)

**Files:** `internal/core/shipment_lifecycle.go`
**Test files:** `internal/core/shipment_lifecycle_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** Unit 6 hook call pattern, explicit `topLevel bool` parameter
**Dependencies:** Units 1, 5, 6, 7a

**Approach:**

> [!IMPORTANT]
> `ShipShipment` internally calls `setArtifactStatus`, `ArchiveItem`, and
> `MoveShipmentStatus`. These inner calls MUST pass `topLevel: false` to their
> HookContext so post-hooks that produce external events (EmitHookEvent,
> WebhookNotifier) skip duplicate emission. Pre-hooks (validation) still fire
> on inner calls to maintain invariant enforcement.
>
> The `topLevel` flag is passed as an explicit `bool` parameter to each
> internal lifecycle helper, NOT via `context.Value`. This avoids hidden
> coupling, SA1029 lint risk, and follows idiomatic Go parameter passing.

* **ShipShipment** (`shipment_lifecycle.go`): Pre-hook with shipment metadata.
  Post-hook fires after all items are released and the shipment status changes.
  This is the **top-level** operation; inner calls to setArtifactStatus and
  ArchiveItem pass `topLevel: false`.

* **AdoptItem** (`shipment_lifecycle.go`): Pre-hook with `OldValues` =
  `{"parent_id": old}`, `NewValues` = `{"parent_id": new}`. Post-hook fires
  after successful re-parenting.

Each method gets the same nil-guard pattern as Unit 6. HookContext construction
follows the same field conventions.

**Verification:**

* Existing tests for both methods still pass
* New test per method: pre-hook rejection prevents the operation
* New test per method: post-hook fires after success
* Integration test: ShipShipment produces ONE top-level event, not duplicate events for each inner operation
* Contract test: `tests/contract/hook_events_test.go` — assert that only top-level operations emit events via `HookEventAppender`

### Unit 8: WebhookNotifier

**Files:** `internal/hooks/webhook.go`
**Test files:** `internal/hooks/webhook_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/hook_events.go` for async patterns; `net/http` standard patterns
**Dependencies:** Unit 1 (uses HookContext type)

**Approach:**

`WebhookNotifier` dispatches HTTP POST notifications asynchronously:

```go
type WebhookNotifier struct {
    endpoints  []WebhookEndpointConfig
    client     *http.Client
    rateLimiter *rate.Limiter // golang.org/x/time/rate
    logger     *slog.Logger
}

type WebhookPayload struct {
    SchemaVersion int        `json:"schema_version"` // v1
    EventType     string     `json:"event_type"`
    ItemID        string     `json:"item_id"`
    ArtifactType  string     `json:"artifact_type,omitempty"`
    Title         string     `json:"title,omitempty"`
    Status        string     `json:"status,omitempty"`
    Timestamp     time.Time  `json:"timestamp"`
    ChangedFields []string   `json:"changed_fields,omitempty"`
}
```

The webhook payload mirrors the compact `HookEventPayload` schema (Unit 3)
and MUST NOT include full old/new value maps or artifact body content (Security
Guardrails §2). The `WebhookNotifier.Dispatch` constructs this from the
`HookContext` fields.

* `NewWebhookNotifier(endpoints, rateLimit, logger)` — creates client with
  configurable timeout (per-endpoint or default 10s), initializes rate limiter
* `Dispatch(ctx, hookCtx) error` — the `HookFunc` signature. **Skips when
  `TopLevel` is false** (same guard as EmitHookEvent). For each endpoint
  matching the event filter: marshal payload, acquire rate limiter token, launch
  goroutine to POST. Errors are logged, never returned (post-hook contract).
* `Shutdown(ctx context.Context) error` — drains in-flight webhook goroutines
  before the process exits. Uses a `sync.WaitGroup` internally to track active
  dispatches. Called from `Workspace.Close()`. This prevents goroutine leaks
  when backlogit runs as a CLI command (short-lived process).
* URL values undergo `os.ExpandEnv` at notifier creation for env var resolution
* Endpoints with empty `event_filter` receive all events
* Endpoints with a filter list receive only matching `HookPoint` event types

Use `golang.org/x/time/rate` for the token bucket limiter. The rate limiter is
shared across all endpoints to prevent aggregate webhook storms.

**Verification:**

* Test with `httptest.Server`: payload matches WebhookPayload schema
* Test: rate limiter blocks excess dispatches
* Test: env var expansion in URL
* Test: event filter matches correctly (allow-list semantics)
* Test: endpoint timeout enforced
* Test: dispatch error is logged but returns nil
* Test: TopLevel=false skips webhook dispatch
* Test: Shutdown drains in-flight dispatches before returning

### Unit 9: Wire WebhookNotifier as Post-Hook

**Files:** `internal/core/workspace.go`, `internal/hooks/webhook.go`
**Test files:** `internal/core/workspace_test.go`, `tests/integration/webhook_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Unit 5 registration pattern
**Dependencies:** Units 5, 8, 4

**Approach:**

In `NewWorkspace`, after loading `HooksConfig`:

1. If `Notifications.Endpoints` is non-empty and `Enabled` is true:
   a. Expand env vars in all endpoint URLs
   b. Create `WebhookNotifier` with configured endpoints and rate limit
   c. Register `notifier.Dispatch` as a post-hook on all hook points
      (priority 80, after EmitHookEvent at 50 but before LogIndexStale at 90)
   d. Store `notifier` on Workspace (new field: `webhookNotifier`) for
      shutdown draining

2. In `Workspace.Close()`, call `webhookNotifier.Shutdown(ctx)` before
   closing the database connection to ensure in-flight webhooks complete.

The `Dispatch` function already satisfies the `HookFunc` signature, so
registration is straightforward.

**Verification:**

* Integration test: configure a webhook endpoint pointing at httptest.Server,
  create an artifact, verify the server receives the POST with correct payload
* Test: no endpoints configured → no webhook hook registered
* Test: endpoints configured but hooks disabled → no webhook hook registered

## Dependency Graph

```text
Unit 1 (types + runner) ─────────────────────┐
                                              │
Unit 4 (config expansion) ──────────┐         │
                                    │         │
Unit 2 (pre-hooks) ──── requires ───┤── Unit 5 (wire into Workspace) ──┐
                                    │                                   │
Unit 3 (post-hooks) ─── requires ───┘         │                        │
                                              │         ┌── Unit 6 (instrument create/update)
Unit 8 (WebhookNotifier) ── requires ─────────┘         │
                                                        ├── Unit 7a (instrument archive/move-status)
                                                        │
                                                        └── Unit 7b (instrument ship/adopt; requires 7a)
                                              Unit 9 (wire webhook) ── requires Unit 8
```

Recommended execution order: 1 → 4 (parallel) → 2 → 3 → 5 → 6 → 7a (parallel) → 7b → 8 → 9

Units 1 and 4 can proceed in parallel since they have no shared dependencies.
Units 6 and 7a can also proceed in parallel after Unit 5 completes.
Unit 7b depends on 7a (ShipShipment calls ArchiveItem and MoveShipmentStatus).

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | HookRunner lives on Workspace struct | All lifecycle functions already receive `*Workspace`; adding a field avoids changing every function signature. Runner is initialized once, reused across all operations. | Pass runner as parameter to every function (too invasive, changes every caller); global singleton (violates Constitution principle on global mutable state) |
| D2 | Pre-hook errors block the operation | Validation hooks must be enforceable. If a pre-hook returns an error, the mutation must not proceed. This matches the 007-DL deliberation's chosen direction. | Advisory-only pre-hooks (defeats the purpose of validation); pre-hooks return warnings (ambiguous semantics) |
| D3 | Post-hook errors are swallowed with slog.Warn | Post-hooks are side-effects. A failed webhook notification or event emission must never cause a user's artifact creation to fail. Constitution principle IX demands agent context efficiency — error noise from side-effects would pollute tool responses. | Return post-hook errors as warnings in tool response (adds complexity, confuses agents); aggregate errors (complicates error handling) |
| D4 | Hooks are ordered by integer priority | Predictable execution order prevents subtle bugs. Parent validation (priority 10) must fire before status validation (priority 20). Event emission (50) fires before webhook dispatch (80). | Unordered execution (non-deterministic); named dependency ordering (over-engineered for v1) |
| D5 | YAML-only hook configuration for v1 | Go plugin hooks add significant complexity (build constraints, plugin compatibility, security surface). YAML configuration is sufficient for controlling built-in hooks. User-extensible hooks can be added in v2 if needed. | Go plugins (complex, security risk); Wasm hooks (premature); DSL (over-engineering) |
| D6 | Expand existing HooksConfig struct | The hooks.yaml file already exists with the 008-DL event system config. Adding `lifecycle` and `notifications` sections under the same struct keeps configuration cohesive. | Separate hooks config file (fragmentation); separate struct (awkward loading) |
| D7 | Fire-and-forget webhook dispatch for v1 | Retry logic adds complexity (queue management, exponential backoff, dead letter handling). For v1, a single attempt with configurable timeout is sufficient. If the endpoint is down, the event is lost but logged. | Exponential backoff retry (adds queue dependency); persistent retry queue (scope creep) |
| D8 | Nil-guard HookRunner calls in lifecycle methods | Many existing tests create `Workspace{}` directly without full initialization. A nil check on `ws.HookRunner` before `FirePre`/`FirePost` maintains backward compatibility without requiring every test to set up a runner. | Require all tests to initialize HookRunner (too invasive); skip hook calls in test mode (hides bugs) |
| D9 | Rate limiter shared across all webhook endpoints | A per-endpoint limiter could allow N × endpoints requests/second during bulk operations. A single shared limiter caps total outbound webhook traffic regardless of endpoint count, preventing aggregate storms. | Per-endpoint limiter (allows aggregate storms); no limiter (risk of webhook flooding) |
| D10 | Status transition map is config-driven with sensible defaults | The transition map is loaded from `hooks.yaml lifecycle.transitions` with a default fallback covering all 8 statuses from `config.yaml`. Making it configurable allows operators to customize transitions without code changes. The default map covers all documented backlogit workflow transitions. | Hard-coded-only transitions (inflexible, missed config.yaml statuses); no transition validation (misses the validation opportunity) |
| D11 | Do not duplicate existing parent validation in a pre-hook | `CreateArtifact` already validates parent hierarchy via `validateArtifactParent` (artifacts.go:266-309). Adding a `ValidateParentRequired` pre-hook would duplicate working logic and create two enforcement points to maintain. Future v2 may refactor inline validation into hooks for consolidation. | Duplicate validation as pre-hook (maintenance burden, confusing double-rejection); remove inline validation immediately (risky refactor, out of scope) |
| D12 | TopLevel flag via explicit parameter prevents duplicate events from nested operations | ShipShipment calls setArtifactStatus, ArchiveItem, and MoveShipmentStatus internally. Without a boundary, each nested call emits duplicate hook events and webhook notifications. TopLevel flag lets post-hooks that produce external side-effects (event emission, webhooks) skip when nested. Pre-hooks (validation) always fire regardless. Implementation: internal lifecycle helpers accept an explicit `topLevel bool` parameter, NOT `context.Value` (which creates hidden coupling and SA1029 lint risk). | Per-operation suppression flags (brittle); context.Value depth key (hidden coupling, SA1029); transaction-scoped hook deferral (over-engineered); separate "aggregate" event types (adds complexity without solving the root cause) |
| D13 | WebhookNotifier includes Shutdown drain | CLI processes exit immediately after the command completes. Fire-and-forget goroutines for webhook dispatch would be killed before completing. A sync.WaitGroup drain called from Workspace.Close() ensures in-flight webhooks complete before exit. | No drain (goroutine leak in CLI); synchronous dispatch (blocks user operations on network I/O) |
| D14 | Dedicated LoadHooks function following LoadRegistry pattern | config.Load() reads config.yaml and LoadRegistry() reads registry.yaml, but no function reads hooks.yaml. hooks.yaml is written by WriteDefaults but never loaded at runtime. LoadHooks follows the same pattern: read file, unmarshal, validate, fall back to defaults when absent. | Embed hooks config in config.yaml (conflates workspace config with hook config); lazy-load on first hook call (adds complexity, harder to test) |

## Risks and Caveats

1. **Performance overhead**: Every lifecycle operation now runs through the hook
   runner. Risk is low: the runner iterates a small slice of functions. Pre-hooks
   are synchronous but fast (config lookup, map check). Post-hooks (event emission,
   webhook dispatch) are either lightweight writes or async goroutines.

2. **Backward compatibility**: Existing tests that construct `Workspace{}` directly
   will have a nil `HookRunner`. All hook call sites must nil-guard. Missing a
   nil-guard causes a nil pointer dereference — test coverage must catch this.

3. **Webhook reliability**: Fire-and-forget means notifications can be lost if
   endpoints are temporarily unavailable. This is acceptable for v1 but must be
   documented. Operators relying on webhooks for critical workflows should use
   the JSONL event stream (hooks_queue.jsonl) as the reliable channel.

4. **Config migration**: Existing hooks.yaml files lack the `lifecycle` and
   `notifications` sections. The YAML deserializer handles missing sections
   gracefully (zero values), and `DefaultHooksConfig()` provides sensible
   defaults. No migration script needed.

5. **Concurrent hook registration**: The runner uses `sync.RWMutex`. Registration
   happens once during `NewWorkspace` (single-goroutine init). Firing happens
   concurrently from multiple lifecycle operations. The read lock during firing
   is sufficient.

6. **Existing parent validation is preserved**: The `validateArtifactParent`
   function in `CreateArtifact` remains the enforcement point for parent
   hierarchy. The hook system does not replace it. A future refactoring could
   consolidate inline validation into pre-hooks, but that is out of scope.

7. **Nested operation boundary**: ShipShipment calls ArchiveItem,
   setArtifactStatus, and MoveShipmentStatus internally. The `TopLevel` flag on
   HookContext prevents duplicate post-hook events. Risk: if a new lifecycle
   method is added that calls another hooked method, the developer must
   remember to set `TopLevel: false`. This is documented in the hook package
   GoDoc and tested in integration tests.

8. **008-DL integration**: The `EmitHookEvent` post-hook replaces the need for
   manual event emission scattered through the codebase. Once hooks are wired,
   the 008-DL plan's "emitter" units become simpler: they just need to register
   additional event types, not instrument every call site.

9. **New dependency**: `golang.org/x/time/rate` for the webhook rate limiter.
   This is a well-maintained Go team package with no transitive dependencies.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **PRESENT** — HookRunner added to
  Workspace struct (exported); HookContext is a new public type consumed by
  external hook implementations; hooks.yaml schema gains lifecycle and
  notifications sections
* security, auth, permission, or compliance-sensitive behavior: **PRESENT** —
  webhook endpoints accept auth headers; env var expansion for URLs could
  expose secrets if misconfigured; outbound HTTP from the MCP server is a new
  network surface
* migration, backfill, destructive data/config action, or irreversible step:
  **ABSENT** — existing hooks.yaml files gracefully handle missing sections
* external integration, operator checkpoint, or external dependency:
  **PRESENT** — webhook dispatch to external HTTP endpoints; new
  `golang.org/x/time/rate` dependency
* high runtime, rollout, or rollback risk: **ABSENT** — hooks are controlled by
  config flags; disabling lifecycle hooks in hooks.yaml reverts to current
  behavior

Requires plan hardening: **yes**

Rationale: Public API changes (HookRunner on Workspace, HookContext types),
outbound HTTP surface (webhook dispatch), and auth header handling warrant
hardening review for security boundaries, rollback procedures, and contract
stability.

## Runtime Verification and Closure

### Hook Engine (Units 1–7)

* **Runtime surface changed**: Core lifecycle behavior. Every create, update,
  archive, ship, and adopt operation now fires hooks.
* **Runtime verification**: After deployment, create a test artifact with
  hooks enabled. Verify hooks_queue.jsonl receives the event. Verify creating
  a task without parent_id is rejected. Verify status transitions are
  validated.
* **Closure artifact**: Monitoring checklist for hook execution times (should
  be <10ms per hook). Rollback trigger: disable `lifecycle` section in
  hooks.yaml to revert to pre-hook behavior. Validation window: 1 day.
  Owner: ship agent.

### Webhook Dispatch (Units 8–9)

* **Runtime surface changed**: Outbound HTTP POST to external endpoints.
* **Runtime verification**: Configure a test webhook endpoint (e.g., webhook.site).
  Create an artifact. Verify the endpoint receives the POST within 10 seconds.
  Verify rate limiting works during bulk operations.
* **Closure artifact**: Monitoring checklist for webhook dispatch latency and
  failure rate. Rollback trigger: empty the `notifications.endpoints` list in
  hooks.yaml to disable all webhooks. Validation window: 1 day.
  Owner: ship agent.

## Learnings Applied

* **Orphaned tasks** (`docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`):
  The existing `validateArtifactParent` function already implements the systemic
  prevention recommended by this learning. The hook system preserves this inline
  validation (Decision D11) rather than duplicating it. The hook engine provides
  the extension point for future validation consolidation.

* **Stale binary SQLite out-of-memory** (`docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`):
  Hook call sites must not hold database connections longer than necessary.
  Post-hooks that write to JSONL use their own file handles, not the
  Workspace's DB connection.

## Standards Check

| Standard | Compliance | Notes |
|---|---|---|
| Go 1.22+ with GoDoc comments | Compliant | All exported types and functions in `internal/hooks/` will have GoDoc comments |
| `golangci-lint` zero warnings | Compliant | New code follows existing lint configuration |
| Test-first development | Compliant | Every unit specifies test-first or characterization-first execution |
| Sentinel error hierarchy | Compliant | New errors (`ErrHook`, `ErrInvalidStatusTransition`, `ErrWebhookDispatch`) defined in `internal/errors/errors.go` per shared hierarchy convention |
| Structured logging via slog | Compliant | Post-hook failures use `slog.Warn`; webhook dispatch uses `slog.Info` for send and `slog.Error` for failures |
| Path containment via SafeResolve | Compliant | hooks.yaml loaded from `.backlogit/` via existing config loader; no new path resolution needed |
| CQRS data architecture | Compliant | Hook events write to JSONL (append-only history); no Markdown file pollution |
| Constitution IX (agent context efficiency) | Compliant | Hook errors in post-hooks are swallowed, not added to tool responses |
| No panic in library code | Compliant | All error paths use returns, not panics |
| Parameterized DB queries | N/A | No new database queries in hook system |

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Type-Safe Go | Compliant | HookPoint, HookPhase, HookContext are typed structs with validation |
| II. MCP Protocol Fidelity | Compliant | No MCP tool changes; hooks fire behind existing tools |
| III. Test-First Development | Compliant | All units specify test-first execution |
| IV. Workspace Containment | Compliant | hooks.yaml under .backlogit/; webhook URLs in config, not artifacts |
| V. Structured Observability | Compliant | slog logging for hook execution and webhook dispatch |
| VI. Single-Binary Simplicity | Compliant | One new dependency (x/time/rate), pure Go |
| VII. CQRS Data Architecture | Compliant | Events to JSONL, config in YAML, no Markdown pollution |
| VIII. Git-Friendly Persistence | Compliant | hooks.yaml is human-readable YAML |
| IX. Agent Context Efficiency | Compliant | Hook errors swallowed in post-hooks; no context pollution |

## Plan Review

### Attempt 1 — Gate Decision: FAIL (revised)

10 P1 findings required plan revision before harvest. No P0 findings after
assessment (CR-001 downgraded from P0 to P1 since the fix is a validation
strengthening, not an architectural change). The plan is fundamentally sound
but needed targeted corrections to pass.

**Revisions applied (attempt 1 → attempt 2):**

* F-01: Auth header validation changed from SHOULD-warn to MUST-reject
* F-02: Status transitions derived from config.yaml + hooks.yaml, not hard-coded (added `Transitions` to `LifecycleHooksConfig`)
* F-03: Replaced `context.Value` with explicit `topLevel bool` parameter throughout Units 6, 7a, 7b; updated D12 decision
* F-04: Error sentinels moved from `internal/hooks/errors.go` to `internal/errors/errors.go`
* F-05: Added contract test requirements to Units 2, 3, 6, 7b verification sections
* F-06: Added snapshot-before-execute pattern to HookRunner — copy registration under lock, release, then iterate
* F-07: Defined compact `HookEventPayload` struct with `schema_version`, `ChangedFields`, `StatusDelta`; replaced `map[string]any`; aligned `WebhookPayload` schema
* F-08: Defined `HookEventAppender` interface in `internal/hooks/`; events-backed adapter injected from core
* F-09: Fixed In Scope text — removed "parent hierarchy enforcement" (contradicted D11)
* F-10: Split Unit 7 into Unit 7a (ArchiveItem, MoveShipmentStatus) and Unit 7b (ShipShipment, AdoptItem)

Plan hardening was required and the requirement was satisfied by the
`## Plan Hardening` section below. Strict-safety ProposedAction/ActionRisk
classifications are present (PA-1 through PA-4).

### Summary

30 raw findings from 6 reviewer personas, deduplicated to 19 unique findings:
10 P1, 9 P2, 0 P3.

### Findings

#### P0 — Critical (must fix before proceeding)

None.

#### P1 — High (should fix before proceeding)

**F-01: Auth header validation must reject literal secrets, not just warn**
(CR-001 · Unit 4)
The plan says LoadHooks SHOULD warn when a header value lacks `$`. The
constitution (Principle IV) says no secrets in `.backlogit/` files. Strengthen
SHOULD to MUST: `LoadHooks` rejects header values that do not contain `$` or
start with `env:` prefix. Alternatively, change the config schema to
`headers_from_env: map[string]string` where values are env var names, not
raw values.
*Reviewers: Constitution Reviewer*

**F-02: Status transition whitelist is too narrow and hard-coded**
(GQ-001 + AP-001 · Unit 2)
The plan allows only 5 transitions (queued→active, active→done, active→blocked,
blocked→active, queued→blocked). The actual config.yaml defines 8 statuses:
queued, active, blocked, review, done, accepted, rejected, archived. The
hard-coded map will reject valid transitions like active→review or review→done.
Fix: derive the transition map from `config.yaml` status definitions, or at
minimum load it from hooks.yaml. Hard-coding a subset is a breaking change.
*Reviewers: Go Quality Reviewer, Agent-Native Parity Reviewer*

**F-03: Replace context.Value nesting with explicit parameter**
(SB-003 + AS-003 + GQ-004 + LR-002 · Units 6, 7)
Four reviewers flagged `context.Value` for hook depth propagation as hidden
coupling, SA1029 lint risk, and framework-level complexity for one known
aggregation path (ShipShipment). Fix: pass `topLevel bool` as an explicit
parameter to internal lifecycle helpers that ShipShipment calls, or add a
`WithTopLevel(bool)` option to the lifecycle function signatures. This is
more idiomatic Go than ambient context flags.
*Reviewers: Scope Boundary Auditor, Architecture Strategist, Go Quality Reviewer,
Learnings Researcher*

**F-04: Hook error sentinels must use shared hierarchy**
(CR-002 · Unit 2)
The plan creates `internal/hooks/errors.go` with new sentinels. The project
convention is to define sentinels in `internal/errors/errors.go` so all error
types participate in the shared hierarchy. Fix: define `ErrHook`,
`ErrInvalidStatusTransition`, and `ErrWebhookDispatch` in `internal/errors/`
and wrap from hook code.
*Reviewers: Constitution Reviewer*

**F-05: Add contract tests for event schema and approved hook points**
(CR-003 + SB-005 · Units 3, 6, 7)
The plan specifies unit and integration tests but no contract tests for the
event schema consumed by agents via `backlogit_poll_hook_events`. Also missing:
a contract test asserting that only the 6 approved lifecycle operations emit
hook events, preventing regressions from internal helpers leaking events. Fix:
add `tests/contract/hook_events_test.go` validating event JSON shape, approved
hook points, and MCP tool compatibility.
*Reviewers: Constitution Reviewer, Scope Boundary Auditor*

**F-06: Snapshot registrations under RWMutex before executing callbacks**
(GQ-002 · Unit 1)
HookRunner holds the read lock across arbitrary callback execution. If a
callback attempts to register a new hook (unlikely but possible), it
deadlocks. Fix: copy the registration slice under read lock, release the lock,
then execute the snapshot. This is the standard pattern for observer/listener
dispatch in Go.
*Reviewers: Go Quality Reviewer*

**F-07: Event payload needs compact agent-facing schema and version marker**
(CR-005 + AP-004 · Unit 3)
Hook event payloads carry `OldValues`/`NewValues` as `map[string]any`, which
can dump high-volume data through the agent-facing polling tool. New event
types (`create_artifact`, `update_artifact`) change the contract for existing
consumers. Fix: define a compact whitelisted payload (IDs, type, status/title
deltas, field names changed, actor, timestamp) and add a `schema_version`
field. Defer full OldValues/NewValues to a detail endpoint or log query.
*Reviewers: Constitution Reviewer, Agent-Native Parity Reviewer*

**F-08: Decouple hooks from events via writer interface**
(AS-002 · Unit 3)
`internal/hooks/builtin_post.go` imports `internal/events` for
`HookEventWriter`, creating a dependency from the hook engine to a specific
downstream consumer. Fix: define a narrow `EventWriter` interface in
`internal/hooks/` (single `Append` method) and have the events package satisfy
it. Register the adapter from `core` during NewWorkspace, keeping hooks
ignorant of the events package.
*Reviewers: Architecture Strategist*

**F-09: Fix "In Scope" text to remove parent enforcement contradiction**
(SB-001 · General)
The "In Scope" section says "Built-in pre-hooks: parent hierarchy enforcement"
but Decision D11 explicitly excludes parent validation as a hook. Fix: update
the In Scope text to say "Built-in pre-hooks: status transition validation"
to match the actual plan content.
*Reviewers: Scope Boundary Auditor*

**F-10: Split Unit 7 into two sub-units**
(SB-006 · Unit 7)
Unit 7 covers 4 operations across 3 files with nested suppression logic for
ShipShipment. This exceeds the stated "medium" effort. Fix: split into
Unit 7a (ArchiveItem, MoveShipmentStatus — simpler methods without nesting)
and Unit 7b (ShipShipment, AdoptItem — complex nested operations requiring
TopLevel propagation).
*Reviewers: Scope Boundary Auditor*

#### P2 — Moderate (user discretion)

**F-11: LogIndexStale adds surface without new capability** (SB-004 · Unit 3)
The hook emits an slog.Info that signals index staleness, but the actual sync
happens elsewhere. Consider cutting this hook and keeping it as a regular log
statement within lifecycle methods. *Reviewers: Scope Boundary Auditor*

**F-12: Consider always-initializing a no-op HookRunner** (AS-004 + LR-004 ·
Unit 5)
Nil-guard at every call site spreads hook awareness. An alternative: always
create a no-op runner (empty registrations) so lifecycle code unconditionally
calls `ws.HookRunner.FirePre(...)`. Reduces defensive checks.
*Reviewers: Architecture Strategist, Learnings Researcher*

**F-13: No telemetry.jsonl capture for webhook operations** (CR-004 · Unit 8)
New runtime operations (webhook dispatch, rate limiting, shutdown drain) lack
telemetry capture per Constitution Principle V.
*Reviewers: Constitution Reviewer*

**F-14: map[string]any for OldValues/NewValues crosses package boundaries**
(GQ-005 · Unit 1)
Prefer typed structs for data crossing package boundaries. Consider a
`HookDelta` struct with typed fields.
*Reviewers: Go Quality Reviewer*

**F-15: LoadHooks count regression risk** (LR-003 · Unit 4)
Adding a new config loader may affect item counting or rehydration paths.
Add regression test for item counts after LoadHooks integration.
*Reviewers: Learnings Researcher*

**F-16: ClaimShipment hook point left undecided** (SB-002 · General)
Resolve at plan time: exclude ClaimShipment from v1 hook points (low mutation
impact). Document in Non-Goals.
*Reviewers: Scope Boundary Auditor*

**F-17: CLI/MCP parity for status validation** (AP-002 · Units 6, 7)
Verify that both CLI and MCP paths route through the same core lifecycle
functions so hook validation is identical.
*Reviewers: Agent-Native Parity Reviewer*

**F-18: WebhookNotifier Dispatch async/sync clarity** (GQ-003 · Unit 8)
Dispatch returns error (sync signature) but launches goroutines (async).
Clarify that Dispatch always returns nil and the error return exists only to
satisfy HookFunc signature.
*Reviewers: Go Quality Reviewer*

**F-19: internal/hooks package cohesion** (AS-001 · General)
Consider splitting WebhookNotifier into `internal/hooks/webhook/` subpackage
to separate engine from transport adapter.
*Reviewers: Architecture Strategist*

#### P3 — Low (advisory)

None.

### Set-Aside Findings

**LR-001 (batch-failure-silent-nil-return)**: Set aside. The batch-failure
learning applies to batch operations where partial success matters. Post-hook
fire-and-forget (D3) is an intentional design decision matching the 007-DL
deliberation's chosen direction, not a batch operation pattern.

**AP-003 (event queue scaling)**: Set aside. The existing poll/ack
infrastructure and hooks_queue.jsonl were designed and shipped in 008-DL.
Scaling improvements (pagination, retention, compaction) are valid but belong
in a separate improvement scope, not this plan.

### Reviewer Attribution

| Finding | Reviewer(s) | Model |
|---|---|---|
| F-01 | Constitution Reviewer | Claude Opus 4.6 |
| F-02 | Go Quality Reviewer, Agent-Native Parity Reviewer | Claude Opus 4.6, GPT-5.4 |
| F-03 | Scope Boundary Auditor, Architecture Strategist, Go Quality Reviewer, Learnings Researcher | Claude Opus 4.6, GPT-5.4 |
| F-04 | Constitution Reviewer | Claude Opus 4.6 |
| F-05 | Constitution Reviewer, Scope Boundary Auditor | Claude Opus 4.6 |
| F-06 | Go Quality Reviewer | Claude Opus 4.6 |
| F-07 | Constitution Reviewer, Agent-Native Parity Reviewer | Claude Opus 4.6, GPT-5.4 |
| F-08 | Architecture Strategist | GPT-5.4 |
| F-09 | Scope Boundary Auditor | Claude Opus 4.6 |
| F-10 | Scope Boundary Auditor | Claude Opus 4.6 |
| F-11–F-19 | Various | Mixed |

### Next Steps

The gate returned **FAIL** with 10 P1 findings. The plan must be revised to
address F-01 through F-10 before proceeding to harvest. Most fixes are
targeted corrections (text edits, validation strengthening, test additions)
rather than architectural redesigns.

Recommended revision order:
1. F-09: Fix scope text contradiction (trivial text edit)
2. F-02: Derive status transitions from config (design change in Unit 2)
3. F-01: Strengthen auth header validation (validation change in Unit 4)
4. F-03: Replace context.Value with explicit parameter (design change in Units 6, 7)
5. F-04: Move error sentinels to internal/errors/ (file move)
6. F-06: Snapshot-before-execute pattern for RWMutex (design change in Unit 1)
7. F-07: Define compact event payload schema (design change in Unit 3)
8. F-08: Define EventWriter interface to decouple hooks from events (design change in Unit 3)
9. F-10: Split Unit 7 into 7a and 7b (structural change)
10. F-05: Add contract test requirements (verification addition)

### Attempt 2 — Gate Decision: PASS

All 10 P1 findings from attempt 1 were addressed with targeted plan revisions.
Codebase compatibility verified against existing source files:

* `internal/errors/errors.go`: No duplicate sentinels. `ErrHookEvent` exists
  (027-F) but `ErrHook`, `ErrInvalidStatusTransition`, and `ErrWebhookDispatch`
  do not. Clean additions.
* `internal/events/hook_events.go`: Existing `HookEvent` struct uses
  `Payload map[string]any`. The `HookEventAppender` adapter can convert
  `HookEventPayload` → `HookEvent` by marshaling compact fields into the
  payload map. Compatible.
* `internal/core/workspace.go`: No `HookRunner` or `Hooks` fields exist.
  `NewWorkspace` init sequence (config → DB → schema → migrations → recovery)
  has a clear insertion point after config loading.
* `internal/config/schema.go`: Existing `HooksConfig` struct has `Enabled`,
  `EventThresholds`, `AgentSubscriptions`. Adding `Lifecycle` and
  `Notifications` fields is a clean expansion.
* `docs/compound/`: All relevant learnings are referenced. No missed learnings
  that would affect implementation.

#### Findings

##### P0/P1 — None

All 10 P1 findings from attempt 1 have been resolved.

##### P2 — None

P2 findings from attempt 1 (F-11 through F-19) remain as advisory
observations. They do not block harvest.

##### P3 — Advisory

**F-20: LoadHooks code snippet omits header validation logic** (GQ-004 · Unit 4)
The `LoadHooks` Go snippet (lines ~320-337) uses generic `validate.Struct()`
but does not show the custom header value rejection logic inline. The prose
correctly specifies MUST-reject semantics. Implementation will add custom
validation beyond struct tags.
*Self-review observation*

#### Reviewer Attribution

| Reviewer | Model | Method |
|---|---|---|
| Self-review (Stage agent) | Claude Opus 4.6 | Direct codebase verification |
| Quick-plan-check (explore) | Claude Haiku 4.5 | Automated revision verification |

#### Next Steps

The gate returned **PASS**. Proceed to `harvest` to decompose this plan into
backlogit work items.

## Plan Hardening

### Hardening Required: Yes

Three hardening signals are present: public API/contract change, security
surface (outbound HTTP + auth headers), and external integration (webhook
dispatch). The following sections deepen verification, rollback, and guardrail
detail for the risky surfaces.

### Risk Triggers and Protected Invariants

| Trigger | Invariant to Preserve |
|---|---|
| HookRunner added to Workspace struct (exported) | Existing code that constructs `Workspace{}` directly must not break; nil-guard on HookRunner is mandatory at every call site |
| HookContext is a new public type | Type contract must be stable before external consumers depend on it; field additions are non-breaking, field removals or renames are breaking |
| hooks.yaml schema gains lifecycle and notifications sections | Existing hooks.yaml files (v1 agent-event config) must deserialize without error; missing new sections default to zero values |
| Outbound HTTP POST from webhook dispatch | backlogit has never made outbound network calls; this is a new network surface that changes the security posture |
| Auth headers stored in hooks.yaml | hooks.yaml is Git-tracked; auth headers containing secrets must use env var references, never literal values |
| Env var expansion on webhook URLs | Misconfigured env vars could expose internal URLs or silently send to wrong endpoints |
| `golang.org/x/time/rate` new dependency | Must be audited for supply chain risk; must not introduce CGo or platform-specific constraints |

### Risky Actions

#### PA-1: Add HookRunner to Workspace Struct

* **Summary**: Add `HookRunner *hooks.HookRunner` and `Hooks *config.HooksConfig` fields to the exported `Workspace` struct
* **Targets**: `internal/core/workspace.go`
* **Change kind**: public API extension
* **ActionRisk**: moderate — field addition is non-breaking but establishes a contract that external code may depend on
* **Rollback**: Remove the fields and all hook call sites; all lifecycle methods revert to pre-hook behavior via nil-guards
* **Approval required**: no (non-breaking addition)
* **ActionResult**: planned

#### PA-2: Instrument Lifecycle Methods with Hook Calls

* **Summary**: Add `FirePre`/`FirePost` calls to CreateArtifact, UpdateArtifact, ArchiveItem, ShipShipment, AdoptItem, MoveShipmentStatus
* **Targets**: `internal/core/artifacts.go`, `internal/core/archive.go`, `internal/core/shipment_lifecycle.go`, `internal/core/shipment.go`
* **Change kind**: runtime behavior change (every mutation now fires hooks)
* **ActionRisk**: moderate — hooks are side-effect-free by default (empty runner), but misconfigured pre-hooks could reject valid operations
* **Rollback**: Set `lifecycle.validate_transition: false` and `lifecycle.emit_events: false` in hooks.yaml to disable all built-in hooks. Alternatively, set `enabled: false` to disable the entire hook system.
* **Approval required**: no (config-controlled, reversible)
* **ActionResult**: planned

#### PA-3: Outbound HTTP Webhook Dispatch

* **Summary**: WebhookNotifier sends HTTP POST requests to configured external URLs
* **Targets**: `internal/hooks/webhook.go`, network (outbound HTTP)
* **Change kind**: external integration (new network surface)
* **ActionRisk**: high — backlogit has never made outbound HTTP calls; introduces network dependency, potential data leakage, and new failure mode
* **Rollback**: Remove all entries from `notifications.endpoints` in hooks.yaml. No webhooks fire when the endpoints list is empty. No code change required.
* **Approval required**: yes (first outbound network surface)
* **ActionResult**: planned

#### PA-4: Auth Headers in hooks.yaml

* **Summary**: WebhookEndpoint config includes `headers` map for authentication tokens
* **Targets**: `internal/config/schema.go`, `.backlogit/hooks.yaml`
* **Change kind**: config change with security implications
* **ActionRisk**: high — hooks.yaml is Git-tracked; literal secrets in headers would be committed to version control
* **Rollback**: Remove headers from endpoint config; no auth on webhooks
* **Approval required**: no (mitigated by env var expansion requirement below)
* **ActionResult**: planned

### Security Guardrails

1. **Auth header values MUST support env var expansion.** The `Headers` map
   values undergo `os.ExpandEnv` at notifier creation time, same as URLs.
   Documentation and default config MUST show env var references
   (`$WEBHOOK_TOKEN`) not literal tokens. The `LoadHooks` validator MUST
   reject any header value that does not start with `$` or `env:`. Literal
   secret values fail validation with `ErrConfig`, preventing accidental
   secret exposure in Git-tracked hooks.yaml (Constitution Principle IV).

2. **Webhook payload MUST NOT include raw artifact content.** The
   `WebhookPayload` schema includes `event_type`, `item_id`, `title`,
   `status`, `timestamp`, and `details` (old/new values). It MUST NOT
   include the full Markdown body, description, or custom_fields, which
   could contain sensitive project data.

3. **Outbound HTTP requests MUST respect timeouts.** Each endpoint has a
   configurable `timeout_secs` (default 10, max 60). The `http.Client`
   uses this timeout. No infinite waits.

4. **Rate limiter prevents webhook amplification.** A single rate limiter
   (default 10/sec, max 100/sec) caps total outbound requests. During bulk
   operations (e.g., ShipShipment releasing 20 items), the TopLevel flag
   ensures only ONE webhook per top-level operation, and the rate limiter
   provides a second defense.

5. **No SSRF surface.** Webhook URLs are configured by the workspace owner
   in hooks.yaml (a committed file). There is no MCP tool or API that
   accepts arbitrary URLs at runtime. The attack surface is limited to
   whoever has write access to hooks.yaml.

### Reinforced Verification Detail

#### Hook Engine Verification (Units 1–7)

**Environment prechecks:**
* Verify `.backlogit/hooks.yaml` exists and is parseable before testing
* Verify `hooks_queue.jsonl` is writable (HookEventWriter requires append access)

**Target scenarios:**

| Scenario | Expected Outcome | Blocked Path |
|---|---|---|
| Create task with hooks enabled | hooks_queue.jsonl receives `create_artifact` event with correct payload | If event missing: check EmitHookEvent registration, check TopLevel flag |
| Create task without parent_id (level-2) | Rejected by existing `validateArtifactParent` (NOT by hook) | If accepted: inline validation regression, not a hook issue |
| Update artifact with invalid status transition | Rejected by `ValidateStatusTransition` pre-hook with `ErrInvalidStatusTransition` | If accepted: check pre-hook registration, check hooks.yaml `validate_transition: true` |
| ShipShipment with 3 items | ONE `ship_shipment` event in hooks_queue.jsonl, NOT 3+1 events | If duplicates: TopLevel boundary not propagated via context.Value |
| Disable `lifecycle` section in hooks.yaml | No hooks fire; all operations behave as pre-hook era | If hooks still fire: LoadHooks not reading config, or registration not gated on config flags |

**Rollback procedure:**
1. Set `enabled: false` in hooks.yaml → disables entire hook system
2. If granular: set `lifecycle.validate_transition: false` → disables status validation only
3. If granular: set `lifecycle.emit_events: false` → disables event emission only
4. No code change or binary rebuild required for any rollback step

**Rollback trigger:** Any of: pre-hook rejects a previously-valid operation;
post-hook causes measurable latency (>100ms per operation); hook event
emission corrupts hooks_queue.jsonl format.

#### Webhook Dispatch Verification (Units 8–9)

**Environment prechecks:**
* Confirm `golang.org/x/time/rate` is in `go.sum` and passes `go mod verify`
* Confirm no CGo dependency introduced
* Confirm httptest.Server works in CI environment

**Target scenarios:**

| Scenario | Expected Outcome | Blocked Path |
|---|---|---|
| Configure test endpoint, create artifact | Endpoint receives POST within 10s with correct WebhookPayload JSON | If no POST: check endpoint config, event filter, TopLevel flag, rate limiter |
| Configure endpoint with env var URL (`$TEST_WEBHOOK_URL`) | URL expanded from environment at startup | If literal `$TEST_WEBHOOK_URL` in request: os.ExpandEnv not called |
| Configure endpoint with auth header using env var | Header value expanded; request includes auth header | If missing: header expansion not wired |
| Trigger 50 rapid operations | Rate limiter caps dispatches to configured rate; excess events logged but not sent | If all 50 dispatched instantly: rate limiter not initialized |
| Endpoint returns 500 error | Error logged via slog.Error; operation succeeds; no retry | If operation fails: post-hook contract violated |
| CLI command exits after webhook dispatch | Shutdown drain waits for in-flight requests to complete | If goroutine leak detected: sync.WaitGroup not incremented or Shutdown not called from Close() |

**Rollback procedure:**
1. Remove all entries from `notifications.endpoints` in hooks.yaml
2. Restart the MCP server or CLI process
3. No outbound HTTP requests will be made

**Rollback trigger:** Any of: webhook dispatch causes operation latency >1s;
auth headers appear in logs or telemetry; webhook payload contains unexpected
fields (description, custom_fields); rate limiter fails to cap outbound traffic.

### Operational Closure Expectations

**Monitoring checklist (hook engine):**
* Hook execution time per operation (target: <10ms for all pre-hooks, <50ms for all post-hooks excluding webhook)
* Hook event count in hooks_queue.jsonl (should match lifecycle operation count)
* Pre-hook rejection rate (should be near zero in normal operation; high rate indicates misconfigured transition rules)

**Monitoring checklist (webhook dispatch):**
* Webhook dispatch latency (time from hook fire to HTTP response)
* Webhook failure rate (HTTP 4xx/5xx or timeout)
* Rate limiter drop count (events skipped due to rate limiting)
* Shutdown drain time (should complete within 30s)

**Validation window:** 1 day for both hook engine and webhook dispatch.

**Owner:** ship agent during shipment; operator post-merge.

### Learnings and Instructions Consulted

* `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`: Confirmed existing inline parent validation is sufficient; hook system does not need to duplicate it (Decision D11)
* `docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`: Post-hooks must not hold DB connections; JSONL writes use separate file handles
* `.github/instructions/strict-safety.instructions.md`: ProposedAction/ActionRisk vocabulary applied to PA-1 through PA-4
* `.github/instructions/release-observability.instructions.md`: Monitoring plan, rollback triggers, and validation window requirements satisfied
* `.github/instructions/constitution.instructions.md`: Principle IV (workspace containment) verified for hooks.yaml and webhook config; Principle IX (agent context efficiency) verified for post-hook error swallowing

### Unresolved Operator Decisions

1. **PA-3 approval**: Outbound HTTP webhook dispatch is the first network
   surface in backlogit. Operator should confirm this is acceptable before
   the ship agent implements Unit 8. If declined, Units 8 and 9 are dropped
   and the shipment delivers only the internal hook engine (Units 1–7).

2. **Auth header validation**: ~~Should `LoadHooks` emit a warning when a
   header value does not contain `$`?~~ Resolved: `LoadHooks` MUST reject
   header values that do not start with `$` or `env:` (see F-01, Security
   Guardrails §1). This is now a hard validation error, not a heuristic
   warning.
