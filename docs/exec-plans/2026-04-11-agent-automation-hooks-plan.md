---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-11T00:00:00Z
    origin: .backlogit/queue/008-DL.md
    status: revised
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-11-agent-automation-hooks-plan.md
title: Agent-Automation Hooks for MCP Event Signals
---

## Problem Frame

Agents operating in backlogit (Stage, Ship) rely on polling MCP queries to
discover work. They have no push-oriented signal when backlog state changes
materially. This means agents must repeatedly scan the full workspace to detect
conditions like stash overflow, stale blocked items, or shipment readiness.

008-DL introduces an append-only hook event queue that post-hooks (from 007-DL)
populate with structured signals. A new MCP tool surface lets agents poll for
unprocessed events efficiently, replacing broad workspace scans with targeted
event consumption.

**Hard dependency**: 007-DL (Internal lifecycle hooks engine) must be
implemented first. This plan defines the minimal hook interface contract that
007-DL must satisfy.

## Requirements Trace

| #  | Requirement                                             | Origin                                   |
|----|---------------------------------------------------------|------------------------------------------|
| R1 | Append-only JSONL event queue for hook signals          | 008-DL §Chosen Direction, Option A       |
| R2 | Three built-in event types in v1 (of five planned)      | 008-DL §Chosen Direction, event list     |
| R3 | MCP tool for polling unprocessed events                 | 008-DL §Chosen Direction, consumption    |
| R4 | Per-consumer acknowledgement (multi-agent safe)         | Design review finding (multi-agent)      |
| R5 | Configurable thresholds via hooks.yaml                  | 008-DL §Open Questions, thresholds       |
| R6 | Agent subscription filtering                            | 008-DL §Chosen Direction, subscriptions  |
| R7 | Cross-process concurrency safety                        | Codebase pattern (stash_lock.go)         |
| R8 | Deduplication for transition-based signals              | Design review finding (edge transitions) |
| R9 | Time-based signals computed at poll time (not queued)    | Design review finding (blocked_stale)    |

## Scope Boundaries

### In Scope

- HookEvent struct and append-only HookEventWriter (internal/events/)
- Per-consumer checkpoint files in .gitignore'd runtime path (internal/events/)
- HookEventReader with consumer-scoped polling (internal/events/)
- Derived signal provider interface for time-based signals (internal/events/)
- Hook emitter orchestration package (internal/hooks/) — separate from events transport
- `backlogit_poll_hook_events` MCP tool (read-only poll, typed request/response)
- `backlogit_ack_hook_events` MCP tool (separate acknowledgement, typed structs)
- HooksConfig expansion for v1-implemented signals only (internal/config/)
- hooks.yaml schema definition and defaults (v1 signals only)
- Three built-in event signals in v1: feature_review_ready, post_merge_closure,
  blocked_stale (derived). Two deferred to v2: stash_overflow, shipment_ready
- Transition-based deduplication for edge-triggered signals
- Agent documentation updates for Stage and Ship

### Non-Goals

- MCP server-sent notifications or push (stretch goal deferred)
- Event queue rotation, compaction, or garbage collection (v2)
- Pre-hook event signals (only post-hooks emit events)
- Custom user-defined event types
- CLI commands for event management (MCP-only in v1)
- Dashboard or UI for event visualization

### Deferred to Implementation

- Exact UUID generation library choice (crypto/rand or google/uuid)
- Whether hooks.yaml is loaded as a separate file or merged into the workspace
  config loader (depends on 007-DL's loader design)
- Precise JSON field names for payload contents per event type

## Required 007-DL Hook Interface Contract

This plan assumes 007-DL provides the following minimal interface. If 007-DL's
final design differs, the emitter registrations in Unit 4 must adapt.

```go
// HookPoint identifies the lifecycle operation a hook attaches to.
type HookPoint string

const (
    HookCreateArtifact      HookPoint = "create_artifact"
    HookMoveArtifactStatus  HookPoint = "move_artifact_status"
    HookArchiveItem         HookPoint = "archive_item"
    HookShipShipment        HookPoint = "ship_shipment"
    HookAdoptItem           HookPoint = "adopt_item"
    HookCascadeStatusUpdate HookPoint = "cascade_status_update"
)

// HookContext carries contextual data about the triggering operation.
type HookContext struct {
    ItemID       string
    ArtifactType string
    OldValues    map[string]any
    NewValues    map[string]any
    Actor        string
}

// HookFunc is the callback signature for pre- and post-hooks.
type HookFunc func(ctx context.Context, hc HookContext) error

// Workspace exposes hook registration (assumed from 007-DL design).
// RegisterPostHook(point HookPoint, name string, fn HookFunc)
```

**Additional hooks needed beyond 007-DL's stated scope**:

| Hook Point             | Needed By          | Gap Status                           |
|------------------------|--------------------|--------------------------------------|
| `stash_mutate`         | `stash_overflow`   | Not in 007-DL; needs stash hook add  |
| `add_to_shipment`      | `shipment_ready`   | Not in 007-DL; needs shipment hook   |

The plan scopes v1 emitters to only those backed by 007-DL's guaranteed hooks.
`stash_overflow` and `shipment_ready` are deferred unless 007-DL adds hooks for
stash mutations and shipment item additions. If those hooks become available,
the emitters can be added as a follow-up unit.

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort, targets a
single skill domain, and specifies a verifiable exit state.

### Unit 1: HookEvent model and append-only HookEventWriter

**Files:** `internal/events/hook_events.go`
**Test files:** `internal/events/hook_events_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/telemetry.go` (TelemetryWriter),
`internal/core/stash_lock.go` (cross-process locking)
**Dependencies:** none

**Approach:**

Define the `HookEvent` struct and `HookEventWriter`:

```go
type HookEvent struct {
    Sequence  int64          `json:"seq"`        // Monotonic append counter
    ID        string         `json:"id"`         // UUID for identity/dedup
    Timestamp time.Time      `json:"timestamp"`
    EventType string         `json:"event_type"` // stash_overflow, blocked_stale, etc.
    ItemID    string         `json:"item_id,omitempty"`
    Payload   map[string]any `json:"payload,omitempty"`
}
```

Key design decisions:

- `Sequence` is a monotonic counter allocated under the cross-process lock on
  every append. The writer counts existing lines while holding the `.lock`
  sidecar, ensuring no two processes can derive the same next sequence. This
  avoids the race condition of caching the counter at startup.
- No `Processed` field: consumption state lives in per-consumer checkpoint
  files (Unit 2), keeping the queue truly append-only
- `HookEventWriter` uses the dual-layer lock pattern from stash: `sync.Mutex`
  for in-process goroutines + `.lock` sidecar for cross-process safety
- Queue file path: `.backlogit/hooks_queue.jsonl`
- Errors wrap `ErrConfig` or a new `ErrHookEvent` sentinel from
  `internal/errors/errors.go`
- All write operations emit `slog.Info` on success and `slog.Warn` on failure

**Verification:**

- Unit tests: append events, verify JSONL format, verify monotonic sequence
- Concurrent goroutine test: 10 goroutines appending simultaneously, all events
  present with unique sequences
- Cross-process sequence safety: simulate two writers, verify no duplicate sequences
- File locked during write (mock `.lock` sidecar existence)
- 90%+ line coverage on internal/events/hook_events.go

### Unit 2: Per-consumer checkpoint store

**Files:** `internal/events/hook_checkpoint.go`
**Test files:** `internal/events/hook_checkpoint_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/reader.go` (JSONL reading),
`internal/core/stash_lock.go` (file locking)
**Dependencies:** Unit 1 (uses HookEvent.Sequence)

**Approach:**

Implement per-consumer acknowledgement tracking using individual checkpoint files:

```go
type ConsumerCheckpoint struct {
    ConsumerID    string    `json:"consumer_id"`
    LastAckedSeq  int64     `json:"last_acked_seq"`
    AckedAt       time.Time `json:"acked_at"`
}
```

- Checkpoint directory: `.backlogit/runtime/hooks/` (gitignored, ephemeral)
- One file per consumer: `{consumer_id}.checkpoint.json` (atomic write via
  temp-file-then-rename)
- `SaveCheckpoint(consumerID string, seq int64) error` — atomic rewrite of
  consumer's checkpoint file. Rejects seq < current (monotonic ack only).
- `LoadCheckpoint(consumerID string) (int64, error)` — reads consumer's file,
  returns last acked sequence (or 0 if file missing)
- Cross-process lock per consumer file for concurrent access safety
- Errors wrap `ErrValidation` for ack regression, `ErrConfig` for I/O failures

This design means:

- Stage and Ship each get their own consumer ID and checkpoint file
- An event is "unprocessed" for consumer X if `event.Sequence > X.LastAckedSeq`
- No event data is ever mutated or deleted from the queue
- Checkpoint files are runtime state, not source of truth — gitignored and
  disposable (consumers restart from seq 0 if checkpoint is lost)

**Verification:**

- Test save + load roundtrip for multiple consumers
- Test that consumer A acking does not affect consumer B's position
- Test ack regression rejection: ack seq < current returns ErrValidation
- Test atomic rewrite: verify no partial writes on concurrent access
- Test missing checkpoint file returns seq 0 (graceful restart)
- 90%+ line coverage on internal/events/hook_checkpoint.go

### Unit 3: HookEventReader with consumer-scoped polling

**Files:** `internal/events/hook_reader.go`
**Test files:** `internal/events/hook_reader_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/reader.go` (ReadAllEvents, TailEvents)
**Dependencies:** Unit 1, Unit 2

**Approach:**

Implement the read path that combines the event queue with per-consumer
checkpoints. Derived signals (like `blocked_stale`) are returned in a separate
response field to avoid polluting the monotonic ack stream.

```go
// DerivedSignalProvider abstracts the computation of time-based signals.
// This keeps the reader independent of SQLite or any specific data source.
type DerivedSignalProvider interface {
    ComputeSignals(ctx context.Context) ([]HookEvent, error)
}

// PollResult separates queued events (ackable) from derived signals (ephemeral).
type PollResult struct {
    Events         []HookEvent `json:"events"`          // From JSONL queue, monotonic seq
    DerivedSignals []HookEvent `json:"derived_signals"`  // Ephemeral, no seq, recomputed each poll
}

// PollHookEvents returns unprocessed events for a consumer, respecting
// optional event type filters and result limits. Derived signals are
// returned separately and are excluded from the ack stream.
func PollHookEvents(
    queuePath string,
    consumerID string,
    checkpointDir string,
    eventTypes []string,  // nil = all types
    limit int,            // 0 = default (50)
    derivedProvider DerivedSignalProvider, // nil = skip derived signals
) (*PollResult, error)
```

The `blocked_stale` signal is computed at poll time via the injected
`DerivedSignalProvider`. A concrete implementation wraps a `*sql.DB` query
for items with `status = 'blocked'` whose `updated_at` exceeds the configured
threshold. Derived events have no sequence number (Sequence = 0) and are
returned in the `DerivedSignals` field. They are never written to disk and
reappear on next poll if the condition persists. Consumers cannot ack them
because they live outside the monotonic sequence space.

**Verification:**

- Test poll returns only events after consumer's checkpoint
- Test event type filtering
- Test limit cap
- Test derived `blocked_stale` signal via mock DerivedSignalProvider
- Test that derived events appear in DerivedSignals field, not Events
- Test that derived events are excluded from ack checkpoint progression
- Test nil provider (no derived signals returned)
- 90%+ line coverage on internal/events/hook_reader.go

### Unit 4: Built-in post-hook event emitters

**Files:** `internal/hooks/emitters.go`
**Test files:** `internal/hooks/emitters_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/core/shipment_lifecycle.go` (status cascade
pattern), `internal/core/blocking_cascade.go` (child-status aggregation)
**Dependencies:** Unit 1, 007-DL (hook engine)

**Approach:**

Create a new `internal/hooks/` package that orchestrates hook emitters. This
separates emitter business logic from the low-level event transport layer in
`internal/events/`. The hooks package depends on `internal/events/` for
`HookEventWriter`, not the reverse.

**v1 implements only emitters backed by 007-DL's guaranteed hook points:**

| Emitter                | Hook Point              | Logic                                                    |
|------------------------|-------------------------|----------------------------------------------------------|
| `feature_review_ready` | `move_artifact_status`  | Post-move: if artifact is feature, check all children terminal |
| `post_merge_closure`   | `ship_shipment`         | Post-ship: emit with shipment ID and item list           |
| `blocked_stale`        | *(derived at poll time)* | Not a post-hook; computed in Unit 3 via DerivedSignalProvider |

**Deferred emitters** (require hooks not in 007-DL's current scope):

| Emitter           | Required Hook       | Status     |
|-------------------|---------------------|------------|
| `stash_overflow`  | `stash_mutate`      | Deferred   |
| `shipment_ready`  | `add_to_shipment`   | Deferred   |

Each emitter uses **transition-based deduplication**: emit only when the
condition transitions from false to true. For `feature_review_ready`, this
means checking whether the feature was already in "all children terminal" state
before the triggering move. The hook context's `OldValues`/`NewValues` from
007-DL provide the before/after state needed for edge detection.

Registration pattern:

```go
func RegisterBuiltinEmitters(ws *Workspace, writer *HookEventWriter) {
    ws.RegisterPostHook(HookMoveArtifactStatus, "feature_review_ready", 
        makeFeatureReviewReadyEmitter(writer))
    ws.RegisterPostHook(HookShipShipment, "post_merge_closure",
        makePostMergeClosureEmitter(writer))
}
```

**Verification:**

- Test `feature_review_ready`: mock feature with 3 tasks, move last to done,
  verify event emitted
- Test transition dedup: move an already-terminal task, verify NO event emitted
- Test `post_merge_closure`: mock ship shipment, verify event with correct
  payload
- Verify emitters do not emit on pre-hook (post-only)
- 90%+ line coverage on internal/hooks/emitters.go

### Unit 5: HooksConfig schema expansion and hooks.yaml defaults

**Files:** `internal/config/schema.go`, `internal/config/defaults.go`
**Test files:** `internal/config/schema_test.go` (existing, extend)
**Effort size:** small
**Skill domain:** config
**Execution note:** characterization-first (read existing config tests, then extend)
**Patterns to follow:** `internal/config/schema.go` (WorkspaceConfig, existing
HooksConfig stub), `internal/config/defaults.go` (default config templates)
**Dependencies:** none (can be developed in parallel with Units 1-2)

**Approach:**

Expand `HooksConfig` from its current stub. Only include thresholds for signals
implemented in v1 (`feature_review_ready`, `post_merge_closure`, `blocked_stale`).
Deferred signals (`stash_overflow`, `shipment_ready`) have their thresholds and
subscriptions added later in v2:

```go
// Current (lines 94-97):
type HooksConfig struct {
    Enabled bool `yaml:"enabled"`
}

// v1 expansion (only implemented signals):
type HooksConfig struct {
    Enabled            bool                       `yaml:"enabled"`
    EventThresholds    HookEventThresholds        `yaml:"event_thresholds"`
    AgentSubscriptions map[string][]string         `yaml:"agent_subscriptions"`
}

type HookEventThresholds struct {
    BlockedStaleDays int `yaml:"blocked_stale_days" validate:"gte=0"`
}
```

Add defaults in `defaults.go` for hooks.yaml template with v1 signals only:

```yaml
enabled: true
event_thresholds:
  blocked_stale_days: 7
agent_subscriptions:
  stage:
    - blocked_stale
    - feature_review_ready
  ship:
    - post_merge_closure
    - feature_review_ready
```

**Verification:**

- Test YAML roundtrip: marshal/unmarshal with all fields
- Test validation: negative thresholds rejected
- Test defaults applied when fields omitted
- Existing config tests remain green
- 90%+ line coverage on new HooksConfig code paths

### Unit 6: `backlogit_poll_hook_events` and `backlogit_ack_hook_events` MCP tools

**Files:** `internal/mcp/tools.go`, `internal/mcp/server.go`
**Test files:** `tests/contract/hook_events_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first (contract tests for schema validation)
**Patterns to follow:** `internal/mcp/tools.go` (existing tool registration,
five-step handler pattern), `tests/contract/` (MCP contract test pattern)
**Dependencies:** Unit 1, Unit 2, Unit 3, Unit 5

**Approach:**

Register two new MCP tools with typed Go request/response structs:

```go
// PollHookEventsRequest defines the typed input for the poll tool.
type PollHookEventsRequest struct {
    ConsumerID     string   `json:"consumer_id"     validate:"required"`
    EventTypes     []string `json:"event_types"`     // nil = all types
    Limit          int      `json:"limit"`           // 0 = default (50)
    IncludeDerived bool     `json:"include_derived"` // default true
}

// PollHookEventsResponse defines the typed output for the poll tool.
type PollHookEventsResponse struct {
    Events         []HookEvent `json:"events"`
    DerivedSignals []HookEvent `json:"derived_signals,omitempty"`
    ConsumerID     string      `json:"consumer_id"`
    CheckpointSeq  int64       `json:"checkpoint_seq"`
}

// AckHookEventsRequest defines the typed input for the ack tool.
type AckHookEventsRequest struct {
    ConsumerID string `json:"consumer_id" validate:"required"`
    ThroughSeq int64  `json:"through_seq" validate:"required,gte=1"`
}

// AckHookEventsResponse defines the typed output for the ack tool.
type AckHookEventsResponse struct {
    ConsumerID    string `json:"consumer_id"`
    AckedThrough  int64  `json:"acked_through"`
    PreviousSeq   int64  `json:"previous_seq"`
}
```

**`backlogit_poll_hook_events`** (read-only):

| Param             | Type     | Required | Default | Description                          |
|-------------------|----------|----------|---------|--------------------------------------|
| `consumer_id`     | string   | yes      |         | Consumer identity (e.g. "stage")     |
| `event_types`     | []string | no       | all     | Event type filter (typed array)      |
| `limit`           | number   | no       | 50      | Max events to return                 |
| `include_derived` | bool     | no       | true    | Include derived signals (blocked_stale) |

Returns `PollHookEventsResponse` JSON. Events and derived signals in separate
fields. Side-effect free.

**`backlogit_ack_hook_events`** (write):

| Param          | Type   | Required | Description                          |
|----------------|--------|----------|--------------------------------------|
| `consumer_id`  | string | yes      | Consumer identity                    |
| `through_seq`  | number | yes      | Acknowledge all events ≤ this sequence |

Updates the consumer checkpoint. Returns `AckHookEventsResponse` with new and
previous checkpoint state.

Server struct changes in `server.go`: add `HookEventWriter` field (alongside
existing `EventWriter` and `TelemetryWriter`).

**Verification:**

- Contract tests: validate tool parameter schemas against typed structs
- Integration test: write events → poll → verify returned → ack → re-poll →
  verify empty
- Test consumer isolation: consumer A polls, consumer B still sees events
- Test event type filtering ([]string, not comma-separated)
- Test derived signal in separate DerivedSignals response field
- Test workspace-not-initialized error response
- 90%+ line coverage on poll and ack handler functions

### Unit 7: Agent protocol documentation updates

**Files:** `.github/agents/stage.agent.md`, `.github/agents/ship.agent.md`,
`.github/instructions/backlogit.instructions.md`
**Test files:** none (documentation)
**Effort size:** small
**Skill domain:** docs
**Execution note:** review existing agent files, add hook event protocol
**Patterns to follow:** existing agent file structure (phases, steps)
**Dependencies:** Unit 6 (tools must exist before documenting)

**Approach:**

Add a "Hook Event Consumption" section to both Stage and Ship agent files:

- At session start, call `backlogit_poll_hook_events` with agent's consumer ID
- Process returned events as priority signals before regular queue scanning
- After processing, call `backlogit_ack_hook_events` with highest processed seq
- Document each event type and expected agent response

Update `backlogit.instructions.md` to reference hook event tools in the
"Available operations" section.

**Verification:**

- Agent files pass markdownlint
- Hook event tools are referenced with correct parameter names
- Consumer IDs match config defaults (stage, ship)

## Dependency Graph

```text
Unit 5 (config)  ─────────────────────────────────────┐
                                                       │
Unit 1 (event model + writer)                          │
  │                                                    │
  ├── Unit 2 (checkpoint store)                        │
  │     │                                              │
  │     └── Unit 3 (reader + derived signals) ─────────┤
  │                                                    │
  └── Unit 4 (emitters) ── requires 007-DL ──┐         │
                                              │        │
                                              ▼        ▼
                                     Unit 6 (MCP tools)
                                              │
                                              ▼
                                     Unit 7 (agent docs)
```

Units 1 and 5 can be developed in parallel (no shared dependencies).
Units 2 and 4 depend on Unit 1 and can proceed in parallel.
Unit 3 depends on both Unit 1 and Unit 2.
Unit 4 additionally depends on 007-DL being implemented.
Unit 6 depends on Units 1–3 and 5.
Unit 7 depends on Unit 6.

## Decisions

| #  | Decision                                     | Rationale                                                                                          | Alternatives Rejected                                                            |
|----|----------------------------------------------|----------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------|
| D1 | Append-only JSONL queue (no rewrite)         | Matches CQRS architecture; proven by telemetry.jsonl and stash.jsonl patterns                      | SQLite table (adds non-ephemeral DB dependency), in-memory queue (not durable)   |
| D2 | Per-consumer checkpoint files, not global     | Stage and Ship are independent consumers; global "processed" flag causes missed events             | Global processed flag on event (breaks multi-consumer), shared cursor (races)    |
| D3 | Separate poll and ack tools                  | Poll is side-effect free; ack is explicit. Prevents event loss on agent crash between poll and use | Combined poll+mark_consumed (loses events on crash), auto-ack on read (same)     |
| D4 | Monotonic sequence counter on events         | Enables efficient checkpoint resume without scanning UUIDs; compaction-friendly                    | UUID-only (bad for ordering/resume), timestamp-only (not monotonic under concurrency) |
| D5 | blocked_stale as derived poll-time signal    | Time-based condition cannot be detected by post-hooks; avoids infinite duplicate emission          | Stored queue event (duplicates forever), cron mechanism (adds complexity)         |
| D6 | Transition-based dedup for edge signals      | Prevents spam from repeated mutations that don't change the condition                              | Emit on every check (noise), external dedup layer (over-engineering)             |
| D7 | Defer stash_overflow and shipment_ready      | 007-DL does not define hooks for stash mutations or AddToShipment; cannot implement without them   | Implement with polling fallback (conflates hook events with polling)              |
| D8 | Cross-process lock (mutex + .lock sidecar)   | Multiple agent processes may run concurrently; in-process mutex alone is insufficient              | sync.Mutex only (single-process), flock (not portable to Windows)                |

## Risks and Caveats

1. **007-DL interface stability**: This plan defines a minimal hook interface
   contract. If 007-DL's final design differs materially (different callback
   signature, different hook points), Unit 4 emitters need revision. Mitigated
   by keeping emitters as thin adapters.

2. **Queue file growth**: `hooks_queue.jsonl` grows unbounded in v1. For
   active workspaces this should be manageable (events are small, ~200 bytes
   each). Compaction based on minimum acknowledged sequence across consumers
   is the designed extension point but deferred to v2.

3. **Two deferred event types**: `stash_overflow` and `shipment_ready` require
   hook points that 007-DL does not currently define. These are high-value
   signals. The risk is that 007-DL's scope does not expand to cover them,
   leaving the event type list incomplete. Mitigation: if 007-DL adds
   `stash_mutate` and `add_to_shipment` hooks, the emitters are a small
   follow-up unit.

4. **Derived signal freshness**: `blocked_stale` is computed from the SQLite
   index. If the index is stale (not synced after out-of-band edits), the
   signal may be inaccurate. This is consistent with all other index-dependent
   operations in backlogit.

5. **Checkpoint loss on runtime dir cleanup**: Consumer checkpoint files live in
   `.backlogit/runtime/hooks/` which is gitignored and ephemeral. If the
   directory is deleted, consumers restart from sequence 0 and reprocess all
   events. This is by design: event processing should be idempotent, and the
   worst case is redundant work, not data loss.

## Learnings Applied

- **Stash JSONL dual-reader pattern** (`docs/compound/workflow-issues/f015-shipment-stash-patterns.md`):
  Informed the append-only queue design and rehydration count semantics. The
  hooks queue follows the same pattern: append-only writes, full-file reads
  with consumer-specific filtering.

- **Advisory file lock with stale TTL** (`docs/compound/go-patterns/advisory-file-lock-stale-ttl-go-2026-04-08.md`):
  Directly adopted for the HookEventWriter. The `lockStashFile` pattern in
  `internal/core/stash_lock.go` is the template for cross-process safety.

- **Unstaged MCP tool registrations** (`docs/compound/ci-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md`):
  Both new MCP tools must be registered, committed, and verified in CI before
  merging. Tool count assertion tests must be updated.

- **Stash staleness scripting gap** (`docs/compound/workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md`):
  The `stash_overflow` event type partially addresses this gap, though it is
  deferred in v1 pending stash mutation hooks from 007-DL.

## Standards Check

| Standard                          | Compliance | Notes                                           |
|-----------------------------------|------------|-------------------------------------------------|
| GoDoc on all exports              | ✓          | All new types, functions, constants documented   |
| golangci-lint zero findings       | ✓          | Enforced by CI gate                              |
| Sentinel errors from errors.go    | ✓          | New ErrHookEvent sentinel + fmt.Errorf wrapping  |
| log/slog for logging              | ✓          | HookEventWriter logs at Info/Warn levels         |
| path/filepath for FS ops          | ✓          | Queue and checkpoint paths via filepath.Join     |
| Test-first development            | ✓          | All units specify test-first execution           |
| Cross-process locking             | ✓          | Stash-style .lock sidecar pattern adopted        |
| MCP contract tests                | ✓          | Unit 6 includes contract test for tool schemas   |
| Constitution CQRS compliance      | ✓          | Queue is append-only JSONL; no SQLite writes     |
| 90%+ coverage gate (Principle III)| ✓          | Every unit requires 90%+ line coverage           |
| Typed MCP tool schemas (Principle II) | ✓      | Go request/response structs for poll/ack tools   |

## Constitution Check

| Principle                  | Status | Notes                                                   |
|----------------------------|--------|---------------------------------------------------------|
| I. Type-Safe Go            | ✓      | Typed structs for all boundaries, no `any` in APIs      |
| II. MCP Protocol Fidelity  | ✓      | Tools always visible, typed schemas, error responses     |
| III. Test-First Development | ✓      | All units test-first, 90%+ coverage gate per unit       |
| IV. Workspace Containment  | ✓      | Queue in .backlogit/; checkpoints in .backlogit/runtime/ |
| V. Structured Observability| ✓      | slog on all write paths                                 |
| VI. Single-Binary Simplicity| ✓     | No new dependencies; uses crypto/rand for UUIDs         |
| VII. CQRS Data Architecture | ✓     | JSONL append-only, derived signals not stored            |
| VIII. Git-Friendly Persistence| ✓   | Queue is JSONL, checkpoints gitignored runtime state     |
| IX. Agent Context Efficiency| ✓     | Poll returns filtered JSON, not raw file content         |

## Revision History

| Date       | Revision | Changes                                                    |
|------------|----------|------------------------------------------------------------|
| 2026-04-11 | v1.0     | Initial plan                                               |
| 2026-04-11 | v1.1     | Address plan-review gate findings (1 P0, 8 P1):           |
|            |          | - MF1: Sequence counter allocated under lock per append    |
|            |          | - MF2: Derived signals in separate PollResult field        |
|            |          | - MF3: Typed Go request/response structs for MCP tools     |
|            |          | - MF4: 90%+ coverage gate on every unit                    |
|            |          | - MF5: Consumer checkpoints in .gitignore'd runtime path   |
|            |          | - MF6: DerivedSignalProvider interface injected into reader |
|            |          | - MF7: Emitters in internal/hooks/ package                 |
|            |          | - MF8: v1 scoped to 3 signals throughout                   |
|            |          | - MF9: Config defaults trimmed to v1 signals only          |
