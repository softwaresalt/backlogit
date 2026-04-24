---
title: "Agent Session Disaster Recovery"
description: "Implementation plan for checkpoint discovery, lifecycle tooling, and agent harness recovery protocol to support interrupted session resumption."
date: 2026-04-23
origin: ".backlogit/queue/040-DL.md"
status: reviewed
---

# Agent Session Disaster Recovery

## Problem Frame

When an agent session terminates unexpectedly — remote server disconnect, power failure, context window exhaustion, accidental terminal close, or MCP process kill — work in progress is lost. The agent must restart from scratch or hope a human can manually piece together state from memory files and checkpoint JSON blobs.

Infrastructure-level crash safety is solid (shipped in 042-S): atomic file writes, crash-safe delete, stale lock recovery, DB-FS reconciliation. But **application-level recovery orchestration is missing**: checkpoints are written but never read back, there's no discovery/validation tool, no cleanup mechanism, and no standardized schema.

This plan adds checkpoint discovery, validation, and cleanup to the backlogit MCP tool surface, a standardized checkpoint schema with lifecycle management, and agent harness protocol updates that use these tools for interactive session recovery.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Agents can discover unresolved checkpoints filtered by agent, shipment, or feature at session start | 040-DL Chosen Direction |
| R2 | Agents can read and validate a specific checkpoint, getting parsed structured JSON back | 040-DL Chosen Direction |
| R3 | Agents can mark checkpoints as resolved when a session completes normally | 040-DL Chosen Direction |
| R4 | Stale and resolved checkpoints can be cleaned up based on a configurable retention policy | 040-DL Chosen Direction |
| R5 | Checkpoints follow a standardized schema with required fields for discoverability | 040-DL Chosen Direction |
| R6 | Stage and Ship agents call checkpoint discovery at session start and present recovery options interactively | 040-DL Chosen Direction |
| R7 | Agents write checkpoints at phase boundaries (harness-complete, task-complete, review-gate, CI-pass, PR-ready) | 040-DL Chosen Direction |
| R8 | Checkpoint retention is configurable in config.yaml with a 7-day default | 040-DL Open Questions |

## Scope Boundaries

### In Scope

- Standardized checkpoint V1 schema struct with Go validator tags, including all supporting types (`CheckpointContext`, `CheckpointProgress`, `CheckpointFilter`, `CheckpointSummary`, `CleanupResult`)
- Sentinel errors (`ErrCheckpointNotFound`, `ErrCheckpointInvalid`, `ErrCheckpointCorrupt`) in `internal/errors/errors.go` with `domainError()` routing
- Checkpoint lifecycle functions: list, get, resolve, cleanup — all with `context.Context` first parameter and path-traversal containment
- Four new MCP tools: `backlogit_list_checkpoints`, `backlogit_get_checkpoint`, `backlogit_resolve_checkpoint`, `backlogit_cleanup_checkpoints`
- CLI command group: `backlogit checkpoint list|get|resolve|cleanup` for human/non-MCP parity
- `checkpoint_retention` config section in `config.yaml` with post-load defaulting
- Unit tests for schema validation and lifecycle functions, including concurrent-access tests
- Contract tests for all four MCP tools (written before handlers in same unit)
- Integration test in `tests/integration/` for end-to-end recovery flow
- Session-start recovery protocol updates for Stage and Ship agents with deterministic recovery state machine
- Mid-session checkpoint cadence documentation
- Checkpoint lifecycle documentation

### Non-Goals

- Automatic resume without operator confirmation (interactive only)
- Cross-machine session migration or real-time replication
- MCP transport layer changes (covered by contingency stash 21E17BFC)
- Sub-task-level checkpointing within a single task execution
- Changes to `memories.json` or `docs/memory/` — these are separate persistence surfaces
- Cross-process file locking for `memories.json` (separate concern)

### Deferred to Implementation

- Exact error message wording for validation failures
- Handling of pre-V1 checkpoint files (existing unschematized JSON) during migration

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target a single skill domain and specify a verifiable exit state.

### Unit 1: Checkpoint V1 Schema and Validation

**Files:** `internal/events/checkpoint_schema.go`
**Test files:** `internal/events/checkpoint_schema_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/models/artifact.go` (struct + validator tags), `internal/events/hook_checkpoint.go` (checkpoint persistence pattern)
**Dependencies:** none

**Approach:**

Define a `CheckpointV1` struct with required fields and validator tags:

```go
type CheckpointV1 struct {
    SchemaVersion int                 `json:"schema_version" validate:"eq=1"`
    Agent         string              `json:"agent" validate:"required,oneof=ship stage"`
    SessionID     string              `json:"session_id" validate:"required"`
    Phase         string              `json:"phase" validate:"required"`
    Status        string              `json:"status" validate:"required,oneof=active resolved"`
    CreatedAt     time.Time           `json:"created_at" validate:"required"`
    UpdatedAt     time.Time           `json:"updated_at" validate:"required"`
    Context       CheckpointContext   `json:"context"`
    Progress      *CheckpointProgress `json:"progress,omitempty"`
    ResumeHint    string              `json:"resume_hint,omitempty"`
}

// CheckpointContext holds the work scope for recovery discovery and filtering.
type CheckpointContext struct {
    ShipmentID string   `json:"shipment_id,omitempty"`
    FeatureID  string   `json:"feature_id,omitempty"`
    TaskIDs    []string `json:"task_ids,omitempty"`
    Branch     string   `json:"branch,omitempty"`
}

// CheckpointProgress tracks completed vs remaining work for resume decisions.
type CheckpointProgress struct {
    TasksCompleted []string `json:"tasks_completed,omitempty"`
    TasksRemaining []string `json:"tasks_remaining,omitempty"`
    FilesModified  []string `json:"files_modified,omitempty"`
    Decisions      []string `json:"decisions,omitempty"`
}

// CheckpointFilter is the parameter struct for ListCheckpoints filtering.
type CheckpointFilter struct {
    Agent      string        `json:"agent,omitempty"`
    Status     string        `json:"status,omitempty"`
    ShipmentID string        `json:"shipment_id,omitempty"`
    FeatureID  string        `json:"feature_id,omitempty"`
    MaxAge     time.Duration `json:"max_age,omitempty"`
}

// CheckpointSummary is the lightweight struct returned by ListCheckpoints.
type CheckpointSummary struct {
    Filename      string    `json:"filename"`
    Agent         string    `json:"agent"`
    SessionID     string    `json:"session_id"`
    Phase         string    `json:"phase"`
    Status        string    `json:"status"`
    CreatedAt     time.Time `json:"created_at"`
    ShipmentID    string    `json:"shipment_id,omitempty"`
    FeatureID     string    `json:"feature_id,omitempty"`
    ResumeHint    string    `json:"resume_hint,omitempty"`
    ValidationErr string    `json:"validation_error,omitempty"`
}

// CleanupResult reports the outcome of a checkpoint cleanup operation.
type CleanupResult struct {
    ArchivedCount int      `json:"archived_count"`
    ArchivedFiles []string `json:"archived_files"`
    SkippedCount  int      `json:"skipped_count"`
    Errors        []string `json:"errors,omitempty"`
}
```

Also define sentinel errors in `internal/errors/errors.go`:

```go
var (
    ErrCheckpointNotFound = errors.New("backlogit: checkpoint not found")
    ErrCheckpointInvalid  = errors.New("backlogit: checkpoint validation failed")
    ErrCheckpointCorrupt  = errors.New("backlogit: checkpoint file corrupt or unparseable")
)
```

Add corresponding cases in `domainError()` in `internal/mcp/errors.go` to map these to proper MCP error categories (not-found, validation-error, internal).

Include `ParseCheckpoint(data []byte) (*CheckpointV1, error)` and `ValidateCheckpoint(cp *CheckpointV1) error` functions. Cache `validator.New()` at package level per compound learning. Use `Progress *CheckpointProgress` (pointer) so `json:"omitempty"` works correctly with `encoding/json`.

**Verification:**
- `go test ./internal/events/...` passes with tests for valid schema, missing required fields, invalid agent/status values, and round-trip JSON marshal/unmarshal
- `go vet ./internal/events/...` clean

### Unit 2: Checkpoint Lifecycle Functions

**Files:** `internal/events/checkpoint_lifecycle.go`
**Test files:** `internal/events/checkpoint_lifecycle_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/events/hook_checkpoint.go` (file I/O + atomic writes), `internal/events/fsutil.go` (syncWriteFileAtomic)
**Dependencies:** Unit 1

**Approach:**

Implement four functions in a new file. All functions take `context.Context` as their first parameter for cancellation propagation (codebase convention). All filename parameters enforce path-traversal containment: reject any input containing path separators, resolve against the checkpoint directory with `filepath.Abs`, and verify the result is within `.backlogit/checkpoints/`.

1. **`ListCheckpoints(ctx context.Context, checkpointDir string, filter CheckpointFilter) ([]CheckpointSummary, error)`**
   - Read all `checkpoint-*.json` files from the directory
   - Parse each with `ParseCheckpoint`; quarantine unparseable files to `.backlogit/quarantine/checkpoints/` with a warning log (do not silently skip — this is a disaster recovery surface where hidden corruption causes data loss)
   - Include quarantined files in the summary with `ValidationErr` populated so operators can distinguish "no checkpoint" from "corrupted checkpoints"
   - Apply optional filters: consumer_id (agent), status, shipment_id, feature_id, max_age
   - Return summaries sorted by `created_at` descending (most recent first)

2. **`GetCheckpoint(ctx context.Context, checkpointDir, filename string) (*CheckpointV1, error)`**
   - Validate filename is basename-only (reject path separators)
   - Read and parse a specific checkpoint file
   - Validate schema; return `ErrCheckpointInvalid` on validation failure, `ErrCheckpointNotFound` wrapping `os.ErrNotExist` on missing file
   - Return parsed struct with validation result

3. **`ResolveCheckpoint(ctx context.Context, checkpointDir, filename string) error`**
   - Validate filename is basename-only (reject path separators)
   - Read checkpoint, set `status=resolved` and `updated_at=now`, write back atomically
   - Use `syncWriteFileAtomic` pattern from `fsutil.go`
   - If already resolved, return nil immediately without modifying the file (idempotent per D7)

4. **`CleanupCheckpoints(ctx context.Context, checkpointDir string, retentionDays int) (CleanupResult, error)`**
   - Guard: if `retentionDays <= 0`, return error immediately (prevents accidental mass-archive)
   - Scan checkpoint files; identify those that are resolved OR older than `retentionDays`
   - Move eligible files to `.backlogit/archive/checkpoints/` (not delete)
   - Apply Windows-safe rename: gate `os.Remove(dst)` on `runtime.GOOS == "windows"` before `os.Rename` for the archive move
   - On archive rename failure, rename temp back to original path (crash-safe rollback per compound learning)
   - Return `CleanupResult` with `archived_count` and `archived_files`

Use short-write guards and Windows-safe atomic rename per compound learnings.

**Verification:**
- `go test ./internal/events/...` passes with tests for: list with filters, get valid/invalid, resolve lifecycle, cleanup with retention, empty directory edge case, quarantine of unparseable files, path-traversal rejection, concurrent list+resolve under race detector
- `go vet ./internal/events/...` clean

### Unit 3: Checkpoint Retention Configuration

**Files:** `internal/config/schema.go`, `internal/config/defaults.go`, `.backlogit/config.yaml`
**Test files:** `internal/config/schema_test.go` (extend existing)
**Effort size:** small
**Skill domain:** config
**Execution note:** characterization-first (verify existing config loads, then extend)
**Patterns to follow:** existing `queue_layout` config pattern in `internal/config/schema.go`
**Dependencies:** none (parallel with Units 1-2)

**Approach:**

Add a `CheckpointRetention` struct to the config schema:

```go
type CheckpointRetention struct {
    RetentionDays int `yaml:"retention_days" validate:"omitempty,gte=1"`
}
```

Wire into the top-level config struct. Add post-load defaulting in the loader: `if cfg.CheckpointRetention.RetentionDays == 0 { cfg.CheckpointRetention.RetentionDays = 7 }`, matching the existing `BugLevel` defaulting pattern at `loader.go:32`. The `Enabled` flag is removed (YAGNI — retention is always enabled; use `retention_days` alone). Update `WriteDefaults()` to include the new section. Ensure existing configs without this section load correctly (zero-value triggers the default, not a validation error).

**Verification:**
- Existing config tests pass unchanged
- New test: config with `checkpoint_retention` section loads correctly
- New test: config without section uses default (retention_days=7)
- New test: config with retention_days=0 uses default (7), not validation error
- `go test ./internal/config/...` passes

### Unit 4: MCP Tool Registrations, Handlers, and CLI Commands

**Files:** `internal/mcp/tools.go`, `internal/cli/checkpoint.go`
**Test files:** `tests/contract/checkpoint_tools_test.go` (contract test stubs written first in this unit)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write failing contract test stubs FIRST, then implement handlers to make them pass
**Patterns to follow:** `handleSaveMemory` (lines 837-851), `handleCreateCheckpoint` (lines 853-867) in `internal/mcp/tools.go`; existing CLI command groups for CLI parity
**Dependencies:** Unit 2

**Approach:**

Register four new tools in `RegisterTools()` after the existing checkpoint tool (line 151):

1. **`backlogit_list_checkpoints`**
   - Params: `consumer_id` (optional string — matches hook checkpoint naming convention), `status` (optional string), `shipment_id` (optional string), `feature_id` (optional string), `max_age_hours` (optional number)
   - Handler calls `events.ListCheckpoints` with filter built from params
   - Response contract: `{"checkpoints": [CheckpointSummary...], "total": N, "quarantined": N}`

2. **`backlogit_get_checkpoint`**
   - Params: `filename` (required string)
   - Handler validates filename is basename-only before calling `events.GetCheckpoint`
   - Response contract: `{"checkpoint": CheckpointV1, "filename": "...", "valid": true|false, "validation_error": "..."}`
   - Maps `ErrCheckpointNotFound` via `domainError()` to not-found (not internal error)

3. **`backlogit_resolve_checkpoint`**
   - Params: `filename` (required string)
   - Handler calls `events.ResolveCheckpoint`
   - Response contract: `{"ok": true, "filename": "...", "status": "resolved", "resolved_at": "..."}`

4. **`backlogit_cleanup_checkpoints`**
   - Params: `retention_days` (optional number, defaults to config value)
   - Handler calls `events.CleanupCheckpoints`; guards `retention_days <= 0` before calling
   - Response contract: `{"archived_count": N, "skipped_count": N, "errors": [...]}`

Each handler follows the `requireWorkspace` guard pattern. Use `toolResultJSON` for structured responses.

**CLI command group** (`internal/cli/checkpoint.go`):

Add `backlogit checkpoint` with subcommands `list`, `get`, `resolve`, `cleanup` mirroring the MCP tools. Each subcommand calls the same lifecycle functions from `internal/events/`. This ensures human/automation parity with the MCP surface.

**Contract test stubs** (written first, expected to fail until handlers are implemented):

```go
// tests/contract/checkpoint_tools_test.go
func TestListCheckpoints_Empty(t *testing.T)           { /* ... */ }
func TestListCheckpoints_WithFilters(t *testing.T)     { /* ... */ }
func TestGetCheckpoint_ValidFile(t *testing.T)          { /* ... */ }
func TestGetCheckpoint_MissingFile(t *testing.T)        { /* ... */ }
func TestResolveCheckpoint_Lifecycle(t *testing.T)      { /* ... */ }
func TestResolveCheckpoint_Idempotent(t *testing.T)     { /* ... */ }
func TestCleanupCheckpoints_RetentionPolicy(t *testing.T) { /* ... */ }
func TestCleanupCheckpoints_DefaultsToConfig(t *testing.T) { /* ... */ }
func TestCreateCheckpoint_V1Schema(t *testing.T)        { /* ... */ }
```

**Verification:**
- `go build ./cmd/backlogit` succeeds
- `go vet ./...` clean
- Tools appear in MCP tool listing
- Contract tests pass
- CLI commands produce equivalent output to MCP tools

### Unit 5: Upgrade `backlogit_create_checkpoint` for V1 Schema

**Files:** `internal/mcp/tools.go`, `internal/events/memory.go`
**Test files:** `internal/events/memory_test.go` (extend)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first (existing tests pass, then extend)
**Patterns to follow:** existing `CreateCheckpoint` in `internal/events/memory.go` (lines 39-49)
**Dependencies:** Unit 1

**Approach:**

Upgrade the existing `backlogit_create_checkpoint` tool to support V1 schema validation. The schema version is **persisted in every checkpoint** and lifecycle functions enforce validation on read/write based on the stored version:

1. When the `state_dump` JSON contains `"schema_version": 1`, parse as `CheckpointV1` and validate before writing. Reject invalid V1 payloads with `ErrCheckpointInvalid`.
2. When `schema_version` is absent in the JSON, write raw JSON as before (backwards-compatible for pre-V1 callers).
3. When V1 validation is used, auto-set `created_at` and `updated_at` if missing, and default `status` to `active`.
4. Use `syncWriteFileAtomic` instead of `os.WriteFile` for crash safety — applied **unconditionally to both V1 and legacy paths** (no reason legacy writes should lack crash safety).

**Verification:**
- Existing `memory_test.go` tests pass unchanged (backwards compatibility)
- New test: V1 schema checkpoint validates and writes correctly
- New test: invalid V1 schema returns validation error
- `go test ./internal/events/...` passes

### Unit 6: Unit Tests for Schema and Lifecycle

**Files:** `internal/events/checkpoint_schema_test.go`, `internal/events/checkpoint_lifecycle_test.go`
**Test files:** (these ARE the test files)
**Effort size:** medium
**Skill domain:** tests
**Execution note:** test-first (written alongside Units 1-2, finalized here)
**Patterns to follow:** `internal/events/hook_checkpoint_test.go` (temp dir setup, round-trip assertions), `internal/events/memory_test.go`
**Dependencies:** Units 1, 2

**Approach:**

Schema tests:
- Valid V1 checkpoint parses and validates
- Missing required fields return validation error (agent, session_id, phase, status, timestamps)
- Invalid `agent` value ("unknown") rejected
- Invalid `status` value ("cancelled") rejected
- JSON round-trip preserves all fields including optional ones
- Empty `context` and `progress` structs are valid

Lifecycle tests:
- `ListCheckpoints` with empty directory returns empty slice
- `ListCheckpoints` with mixed valid/invalid files quarantines invalid, includes them in summary with `ValidationErr`
- `ListCheckpoints` filter by consumer_id (agent), status, shipment_id
- `ListCheckpoints` filter by max_age (time-based)
- `ListCheckpoints_ConcurrentResolve` — spin N goroutines calling `ListCheckpoints` while another concurrently calls `ResolveCheckpoint`; verify no race detector violations
- `GetCheckpoint` returns parsed struct
- `GetCheckpoint` for missing file returns `ErrCheckpointNotFound`
- `GetCheckpoint` rejects filename with path separators (path-traversal containment)
- `ResolveCheckpoint` sets status=resolved and updates timestamp
- `ResolveCheckpoint` on already-resolved returns nil (idempotent per D7) — assert double-resolve returns nil
- `CleanupCheckpoints` moves resolved files to archive
- `CleanupCheckpoints` moves stale files (older than retention) to archive
- `CleanupCheckpoints` does not move recent active files
- `CleanupCheckpoints` creates archive directory if missing
- `CleanupCheckpoints` with retentionDays=0 returns error (guard)
- `CleanupCheckpoints` archive rename handles Windows (pre-existing destination)

**Verification:**
- `go test ./internal/events/... -count=1` all pass
- `go test ./internal/events/... -race` no data races

### Unit 7: Integration Test for End-to-End Recovery Flow

**Files:** `tests/integration/checkpoint_recovery_test.go`
**Test files:** (this IS the test file)
**Effort size:** small
**Skill domain:** tests
**Execution note:** test-first
**Patterns to follow:** existing integration test infrastructure in `tests/integration/`
**Dependencies:** Unit 4

**Approach:**

Integration test exercises the full cross-module recovery flow in a real workspace:

- `TestCheckpointRecoveryFlow` — full lifecycle: init workspace → create V1 checkpoint → list checkpoints (verify discovery) → get checkpoint (verify parse + validation) → resolve checkpoint → cleanup (verify archive) → verify archive directory contents → verify resolved checkpoints are gone from active directory
- `TestCheckpointRecoveryFlow_WithConfig` — same flow but with custom `checkpoint_retention` config (retention_days=1), verifying config-driven cleanup behavior
- `TestCheckpointRecoveryFlow_QuarantineCorrupt` — place a corrupt JSON file alongside valid checkpoints, verify list returns it with `ValidationErr` populated and quarantines it

Use real workspace setup (not mocked) to exercise config loading, file routing, lifecycle functions, and MCP handlers in a single pass.

**Verification:**
- `go test ./tests/integration/... -run Checkpoint` all pass
- No flaky tests (deterministic time handling, isolated temp workspaces)

### Unit 8: Agent Recovery Protocol Updates

**Files:** `.github/agents/stage.agent.md`, `.github/agents/ship.agent.md`
**Test files:** N/A (instruction files)
**Effort size:** medium
**Skill domain:** docs
**Execution note:** N/A
**Patterns to follow:** existing Session Continuity sections in both agent files
**Dependencies:** Unit 4 (protocol references the new MCP tools)

**Approach:**

Update the Session Continuity section in both agent files to add a **deterministic recovery state machine** with observable outputs:

**Recovery state machine:**

```
SESSION_START
  → call list_checkpoints(consumer_id="{self}", status="active", max_age_hours=168)
  → if empty: FRESH_START
  → if non-empty: RECOVERY_DECISION

RECOVERY_DECISION
  → present checkpoint summaries (phase, shipment/feature context, tasks completed, resume hint, validation status)
  → surface any checkpoints with validation_error (quarantined) as warnings
  → ask operator: "Resume from checkpoint {filename}, or start fresh?"
  → log chosen action and reason to slog
  → if resume: RESUME_FROM_CHECKPOINT
  → if fresh: FRESH_START

RESUME_FROM_CHECKPOINT
  → call get_checkpoint(filename="{chosen}")
  → if valid=false: warn operator, fall back to FRESH_START
  → restore context from checkpoint
  → resolve any other active checkpoints from prior sessions
  → continue from checkpoint phase

FRESH_START
  → resolve all active checkpoints from prior sessions (cleanup)
  → proceed with existing memory scan
```

**Mid-session checkpoint cadence** (add to the checkpoint triggers subsection):
- Write a V1 checkpoint after each phase boundary
- Ship phases: harness-complete, per-task-complete, review-gate, CI-pass, PR-ready
- Stage phases: triage-complete, deliberation-complete, plan-review-complete, harvest-complete
- Include `resume_hint` with actionable next step

**Session-end resolution** (add to session end):
- On normal completion, call `backlogit_resolve_checkpoint` for all active checkpoints from this session
- On graceful context-window shutdown, write a final checkpoint with `resume_hint` before ending (best-effort — context-window detection is client-dependent and may not always be available)

**Verification:**
- Agent files are valid markdown
- Protocol references only tools that exist (Units 1-4 shipped)
- Recovery state machine covers all paths: fresh start, resume, decline-resume, invalid checkpoint, no checkpoint, quarantined checkpoint
- Each state transition produces observable slog output for debugging
- Non-interactive/test runs can be verified by checking slog output (no hanging on prompts)

## Dependency Graph

```
Unit 1 (Schema + Errors) ──┬──→ Unit 2 (Lifecycle) ──→ Unit 4 (MCP Tools + CLI + Contract Tests) ──→ Unit 7 (Integration Tests)
                           │                                    │
                           └──→ Unit 5 (Upgrade create)         └──→ Unit 8 (Agent Protocol)
                           │
                           └──→ Unit 6 (Unit Tests)

Unit 3 (Config) ──→ Unit 2 (Lifecycle cleanup uses retention config)
```

**Sequencing rationale:**
- Units 1 and 3 have no dependencies and can start in parallel
- Unit 2 depends on the schema (Unit 1) and config (Unit 3) for cleanup behavior
- Unit 4 depends on Unit 2 for the underlying functions; includes contract test stubs written first (test-first)
- Unit 5 depends on Unit 1 for schema validation
- Unit 6 finalizes alongside Units 1-2 (test-first means tests are written during those units)
- Unit 7 depends on Unit 4 (integration tests need the full tool surface)
- Unit 8 depends on Unit 4 (agent protocol references the new tools)

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Checkpoint files stay in flat `.backlogit/checkpoints/` directory | Simpler implementation; filter-based discovery handles all use cases; structured subdirectories add routing complexity for minimal benefit at current scale | Nested `checkpoints/{agent}/{shipment}/` paths |
| D2 | Cleanup archives to `.backlogit/archive/checkpoints/` rather than deleting | Consistent with artifact archival pattern; allows forensic recovery; aligns with compound learning on source artifact archival | Hard delete of old checkpoints |
| D3 | Schema version persisted in checkpoint JSON; lifecycle enforces validation on read/write based on stored version | Validation boundary is at the persistence layer, not caller choice; prevents different entrypoints from interpreting the same checkpoint differently | Caller-controlled opt-in via parameter (leaky boundary) |
| D4 | `ListCheckpoints` quarantines unparseable files (not silent skip) | For disaster recovery, hidden corruption causes data loss; quarantine makes corruption visible to operators while still returning valid results | Skip with warning log only (AS-6, F-10) |
| D5 | Retention config defaults to 7 days, configurable in `config.yaml` with post-load defaulting | Balances cleanup frequency against recovery window; post-load defaulting prevents validation errors on existing configs; `Enabled` flag removed (YAGNI) | Hardcoded retention, `Enabled` toggle, or no retention |
| D6 | Interactive resume (show + ask), never automatic | Prevents silent incorrect resumption from stale state; gives operator control; avoids compounding errors from outdated context | Automatic resume on checkpoint discovery |
| D7 | `ResolveCheckpoint` is idempotent (return nil on already-resolved, do not modify file) | Simplifies agent logic; session-end cleanup can resolve all checkpoints without checking state first | Error on double-resolve |
| D8 | All lifecycle functions take `context.Context` as first parameter | Codebase convention; enables cancellation propagation for long-running cleanup; prevents locked-in signature change later | Omit context (convention violation) |
| D9 | Filename parameters enforce basename-only + path containment | Workspace containment security boundary; reject path separators and verify resolved path is within `.backlogit/checkpoints/` | Accept arbitrary paths (traversal risk) |
| D10 | CLI command group `backlogit checkpoint` mirrors MCP tools | Human/automation parity; every other major tool surface has CLI equivalent; enables debugging without MCP client | MCP-only (parity gap) |

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| Pre-V1 checkpoint files fail to parse during `ListCheckpoints` | D4: quarantine with `ValidationErr` populated; document migration path in checkpoint lifecycle docs |
| Checkpoint accumulation if agents fail to resolve | Retention-based cleanup (Unit 2) handles this; cleanup runs even without resolution |
| Time-based filter sensitivity across timezones | Use UTC internally (RFC3339); compare against `time.Now().UTC()` |
| `syncWriteFileAtomic` Windows pre-remove race | Apply compound learning: gate `os.Remove(dst)` on `runtime.GOOS == "windows"` only |
| Checkpoint files growing large (token waste on read) | Log warning if checkpoint exceeds 4KB; keep schema focused on essential resume state |
| Config schema backwards compatibility | Post-load defaulting with `omitempty` validation; zero-value triggers default, not error |
| Path traversal in filename parameters | D9: basename-only validation + `filepath.Abs` containment check |
| Archive rename collision on Windows | Gate `os.Remove(dst)` before `os.Rename` on Windows; suffix with modification timestamp if collision risk is high |
| Same-second filename collision | Pre-existing (1-second granularity); noted as known limitation; sub-second precision can be added if observed |

## Plan Hardening Signals (REQUIRED)

* **Public API, schema, or contract change**: YES — four new MCP tools added to the tool surface; checkpoint schema is a new public contract
* **Security, auth, permission, or compliance-sensitive behavior**: NO — checkpoints contain workflow state, not secrets
* **Migration, backfill, destructive data/config action, or irreversible step**: NO — cleanup archives (not deletes); old checkpoints remain readable
* **External integration, operator checkpoint, or external dependency**: NO — all changes are internal to backlogit
* **High runtime, rollout, or rollback risk**: NO — new tools are additive; no existing behavior changes

**Requires plan hardening: no** — The new tools are additive, cleanup is non-destructive (archive, not delete), and the schema is opt-in. The primary risk (pre-V1 file handling) is mitigated by graceful degradation (D4).

## Runtime Verification and Closure

| Unit | Runtime Surface | Verification | Closure |
|---|---|---|---|
| Unit 4 | MCP tools (4 new) + CLI commands | Verify tools appear in `backlogit_get_metadata_catalog`; invoke each tool manually via MCP and CLI; confirm structured JSON responses match defined contracts | Tool surface documented in metadata catalog; CLI parity verified |
| Unit 5 | Existing `create_checkpoint` tool (modified) | Verify backwards compatibility: call without V1 schema still works; call with V1 validates; both paths use atomic writes | Rollback: revert handler to ignore V1 detection |
| Unit 7 | Integration test surface | Full create → list → get → resolve → cleanup flow with config-driven retention | Integration test covers cross-module recovery path |
| Unit 8 | Agent behavior (Stage + Ship) | Manual test: start a Ship session, verify recovery state machine executes; verify all state transitions produce slog output | Recovery state machine is documented and testable |

## Learnings Applied

| Learning | File | How Applied |
|---|---|---|
| Short-write guard before fsync | `docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md` | Applied in `ResolveCheckpoint` and all atomic writes in Unit 2 |
| Windows-safe atomic rename | `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md` | Gate `os.Remove(dst)` on `runtime.GOOS == "windows"` in `syncWriteFileAtomic` usage and archive move |
| Crash-safe delete-rename rollback | `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md` | On archive rename failure, rename temp back to original path for recoverability |
| Advisory file lock stale TTL | `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` | Informs stale checkpoint detection approach (time-based, not lock-based) |
| Source artifact archival pattern | `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md` | Cleanup archives to `archive/checkpoints/` rather than deleting |
| Unstaged MCP tool registrations | `docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md` | Ensure all 4 new tool registrations are committed before pushing |
| Cache validator.New() at package level | `docs/compound/go-implementation/feature-001-core-implementation.md` | Reuse cached validator instance in checkpoint schema validation |
| filepath.Abs before filepath.Clean | `docs/compound/go-implementation/feature-001-core-implementation.md` | Path containment uses `filepath.Abs` before comparison (D9) |
| Normalize filter slices before SQL | `docs/compound/config-issues/queue-view-empty-filter-values-2026-04-05.md` | Apply to filter params in `ListCheckpoints` (trim blanks, drop empty) |
| Session recovery protocol stability | `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md` | Recovery state machine stabilized before agent adoption (Unit 8 after Units 1-7) |

## Standards Check

| Standard | Compliance |
|---|---|
| Go 1.22+ with GoDoc on all exports | Yes — all new exported functions, types, and structs will have GoDoc comments |
| golangci-lint zero warnings | Yes — all new code will pass lint; pointer fields for optional structs with `omitempty` |
| Test-first development | Yes — contract test stubs written before handlers in Unit 4; unit tests colocated with Units 1-2; integration test in Unit 7 |
| Struct validation at boundaries | Yes — `CheckpointV1` and all supporting types use `go-playground/validator` tags |
| Atomic file writes | Yes — all checkpoint writes (both V1 and legacy) use `syncWriteFileAtomic` |
| Workspace containment | Yes — all filename params enforce basename-only + `filepath.Abs` containment (D9) |
| Sentinel errors | Yes — `ErrCheckpointNotFound`, `ErrCheckpointInvalid`, `ErrCheckpointCorrupt` with `domainError()` routing |
| context.Context propagation | Yes — all lifecycle functions take `ctx context.Context` as first parameter (D8) |
| CQRS: Markdown source of truth | N/A — checkpoints are ephemeral operational state, not source-of-truth artifacts |
| Agent context efficiency | Yes — `ListCheckpoints` returns summaries (not full content); `GetCheckpoint` returns single parsed struct |
| CLI/MCP parity | Yes — `backlogit checkpoint` command group mirrors all 4 MCP tools (D10) |
| Conventional commits | Yes — commits will use `feat(mcp):`, `test(contract):`, `docs(agents):` scopes |

## Plan Review

---
title: "Plan Review: Agent Session Disaster Recovery"
date: 2026-04-23
plan: "docs/exec-plans/2026-04-23-disaster-recovery-plan.md"
gate: pass
reviewers: [constitution-reviewer, go-quality-reviewer, scope-boundary-auditor, learnings-researcher, architecture-strategist, agent-native-parity-reviewer]
---

### Gate Decision: PASS (after revision)

Initial gate decision was **FAIL** with 1 P0 and 11 P1 findings. The plan was revised to address all P0 and P1 findings. Remaining P2/P3 findings are acknowledged as advisory.

**Revisions applied:**
- F-01 (P0): Defined all 5 supporting types with fields, JSON tags, and validator constraints
- F-02 (P1): Added path-traversal containment with basename validation and `filepath.Abs`
- F-03 (P1): Changed to `validate:"omitempty,gte=1"` with post-load defaulting
- F-04 (P1): Removed `Enabled` flag (YAGNI); retention always active
- F-05 (P1): Added sentinel errors (`ErrCheckpointNotFound`, `ErrCheckpointInvalid`, `ErrCheckpointCorrupt`) with `domainError()` routing
- F-06 (P1): Added `context.Context` to all lifecycle function signatures
- F-07 (P1): Changed `Progress` to `*CheckpointProgress` pointer for correct `omitempty` behavior
- F-08 (P1): Applied Windows-safe rename to archive move path
- F-09 (P1): Schema version persisted in checkpoint; lifecycle enforces on read/write
- F-10 (P1): Changed from skip-with-warning to quarantine with `ValidationErr` in results
- F-11 (P1): Added CLI command group `backlogit checkpoint list|get|resolve|cleanup`
- F-12 (P1): Defined deterministic recovery state machine with all paths specified

Plan hardening was assessed as not required (additive tools, non-destructive cleanup, opt-in schema). No `## Plan Hardening` section was expected or missing.

### Summary

| Severity | Count | Category breakdown |
|----------|-------|--------------------|
| P0 | 1 | type-safety (1) |
| P1 | 11 | error-handling (4), type-safety (2), security (1), architecture (2), agent-ux (1), cli-parity (1) |
| P2 | 14 | scope (3), testing (3), error-handling (3), naming (1), architecture (1), learnings (3) |
| P3 | 2 | documentation (1), naming (1) |

### Findings

#### P0 — Critical (must fix before proceeding)

**F-01: Five supporting types referenced but never defined** (GQ-1)
Units 1-2 reference `CheckpointContext`, `CheckpointProgress`, `CheckpointFilter`, `CheckpointSummary`, and `CleanupResult` but none are defined. Without them, `checkpoint_schema.go` and `checkpoint_lifecycle.go` will not compile. The field shapes of these types directly determine filter logic, test assertions, and MCP response contracts.
*Fix: Define all five types in Unit 1 with fields, JSON tags, and validator constraints.*

#### P1 — High (must fix before proceeding)

**F-02: Checkpoint filename operations lack path-traversal containment** (CR-1 + LR-11)
`GetCheckpoint` and `ResolveCheckpoint` accept caller-supplied `filename` but the plan never requires basename-only validation or `filepath.Rel` containment check. A `../` input could escape `.backlogit/checkpoints/`. Additionally, `filepath.Clean` alone does not make paths absolute — `filepath.Abs` must precede comparison.
*Fix: Reject path separators in filename input; enforce `.backlogit/checkpoints/` containment with `filepath.Abs` before comparison.*

**F-03: `validate:"gte=1"` on RetentionDays breaks existing configs** (GQ-2)
Existing `config.yaml` files without `checkpoint_retention` decode to `RetentionDays: 0`, failing `gte=1` validation and breaking `Load()` for all pre-existing installations.
*Fix: Use `validate:"omitempty,gte=1"` and add post-load defaulting: `if cfg.CheckpointRetention.RetentionDays == 0 { cfg.CheckpointRetention.RetentionDays = 7 }`, matching the existing `BugLevel` pattern.*

**F-04: Config `Enabled` zero-value is false, contradicting intended default** (GQ-3)
Go bool zero-value is `false`. Existing configs without the section will silently disable cleanup.
*Fix: Apply explicit post-load defaulting for the entire `CheckpointRetention` struct when it is at zero-value.*

**F-05: No sentinel errors for checkpoint domain** (GQ-4)
Without `ErrCheckpointNotFound` and `ErrCheckpointInvalid` in `internal/errors/errors.go`, `domainError()` will misroute file-not-found as `InternalError` (500 semantics for what is a 404).
*Fix: Add sentinel errors and corresponding `domainError()` switch cases.*

**F-06: Lifecycle function signatures omit `context.Context`** (GQ-5)
Every I/O function in `internal/events/` takes `ctx context.Context` as first parameter. The plan's `ListCheckpoints`, `GetCheckpoint`, `ResolveCheckpoint`, and `CleanupCheckpoints` all omit it, violating codebase convention and blocking cancellation propagation.
*Fix: All four signatures must be `func X(ctx context.Context, ...)`.*

**F-07: `omitempty` on struct value is a no-op in `encoding/json`** (GQ-6)
`Progress CheckpointProgress` with `json:"progress,omitempty"` is silently ignored — `encoding/json` never considers a concrete struct "empty". Every checkpoint will include the full progress object even when unpopulated.
*Fix: Use pointer field: `Progress *CheckpointProgress \`json:"progress,omitempty"\``.*

**F-08: Archive rename in CleanupCheckpoints fails on Windows** (GQ-7 + LR-2)
`os.Rename(src, dst)` fails on Windows when destination exists. The compound learning about Windows-safe atomic rename is not applied to the archive move path.
*Fix: Gate `os.Remove(dst)` on `runtime.GOOS == "windows"` before `os.Rename`, or suffix archive filenames with modification timestamp for uniqueness.*

**F-09: Schema validation opt-in creates a leaky boundary** (AS-5 + SB-5)
Making validation opt-in via `schema_version` parameter pushes correctness to callers. Different entrypoints can interpret the same checkpoint differently. The dual-mode `create_checkpoint` (raw legacy + validated V1) adds complexity beyond what V1 recovery needs.
*Fix: Persist schema version in every checkpoint; lifecycle enforces validation on read/write based on stored version. Narrow upgrade to V1 producers only.*

**F-10: Skip-unparseable-files weakens recovery guarantees** (AS-6)
"Warn and skip" can hide corruption in the newest checkpoint, producing silent data loss. For disaster recovery, this is particularly dangerous.
*Fix: Quarantine unreadable files to a known location; surface them through lifecycle/MCP results so operators can distinguish "no checkpoint" from "corrupted checkpoints".*

**F-11: No CLI parity for checkpoint management** (AP-2)
The plan adds 4 MCP tools without corresponding CLI commands, unlike every other major tool surface in the repo.
*Fix: Add `backlogit checkpoint list|get|resolve|cleanup` command group in Unit 4 scope.*

**F-12: Interactive recovery protocol under-specified and untestable** (AP-5 + SB-1)
"List at session start, show + ask, resolve on completion" does not define how options are derived, how invalid checkpoints are surfaced, how chosen actions are recorded, or how tests avoid hanging on prompts. The plan's only verification is a manual Ship test.
*Fix: Define a deterministic recovery state machine with observable outputs. Add explicit verification for Stage, Ship, resume, decline, no-checkpoint, and invalid-checkpoint paths.*

#### P2 — Moderate (user discretion)

**F-13: `ResolveCheckpoint` spec wording contradictory** (GQ-8)
"Reject if already resolved" vs "idempotent, no error" is contradictory. Decision D7 says no error. Update Unit 2 wording and add contract test asserting double-resolve returns nil.

**F-14: `CleanupCheckpoints` retentionDays has no guard for zero/negative** (GQ-9)
If `retentionDays <= 0` passes through, every checkpoint is archived regardless of age.
*Fix: Guard `retentionDays <= 0` with an explicit error return.*

**F-15: Unit 4 handlers written before tests exist (test-first violation)** (GQ-10)
Unit 4 delivers handlers as a complete unit before Unit 7 starts. The first commit has zero test coverage.
*Fix: Move contract test stubs into Unit 4 scope, or require Unit 4 and Unit 7 be developed test-first in same branch.*

**F-16: Atomic write inconsistent between V1 and legacy paths** (GQ-11)
The `syncWriteFileAtomic` upgrade applies only to the V1 code path; legacy `CreateCheckpoint` continues using `os.WriteFile`.
*Fix: Apply atomic write unconditionally to both paths.*

**F-17: Redundant `validate:"required"` on SchemaVersion** (GQ-12)
`required` is redundant when `eq=1` already rejects zero. Produces confusing error messages.
*Fix: Use `validate:"eq=1"` alone.*

**F-18: No concurrent-access test case for race detector** (GQ-13)
Unit 6 specifies `-race` but no tests exercise concurrent `ListCheckpoints` + `ResolveCheckpoint`.
*Fix: Add `TestListCheckpoints_ConcurrentResolve`.*

**F-19: Retention `Enabled` flag is YAGNI** (SB-2)
The requirement is configurable retention with a default. Adding enable/disable creates extra behavioral state without stated need.
*Consider: Remove `Enabled` field; use `RetentionDays: 0` as "disabled" if needed.*

**F-20: Cleanup response shape resolved prematurely** (SB-3)
Plan says response shape is deferred but Unit 4 hardcodes `archived_files`. For V1, return `archived_count` only.

**F-21: Context-window shutdown handling underspecified** (SB-4)
Treated as committed scope but lacks a reliable detection mechanism. Reframe as best-effort guidance.

**F-22: No integration test for end-to-end recovery flow** (CR-2)
The plan adds unit and contract tests but no `tests/integration/` scenario exercising the full create → list → get → resolve → cleanup flow with config-driven retention.
*Fix: Add one integration test.*

**F-23: `agent` filter naming inconsistent with conventions** (AP-3)
Existing tools use `consumer_id` for agent identity. Bare `agent` breaks naming conventions.
*Fix: Rename to `consumer_id` for consistency.*

**F-24: MCP response contracts not defined** (AP-4)
Tool responses need stable JSON contracts for agents and CLI wrappers to rely on.
*Fix: Define response shapes in the plan for all four tools.*

**F-25: Crash-safe rollback pattern not applied to archive move** (LR-3)
When `os.Remove` fails after a checkpoint archive operation, the temp file should be renamed back to its original path for recoverability. The compound learning about crash-safe delete-rename rollback is not referenced.

**F-26: Key scope decisions not contract-tested** (SB-6)
Contract tests do not verify cleanup defaulting to config when `retention_days` is omitted, or idempotent `ResolveCheckpoint`.

#### P3 — Low (advisory)

**F-27: `time.Time` `required` tag behavior should be documented** (GQ-14)
The `required` constraint on `CreatedAt`/`UpdatedAt` is correct but non-obvious. Add inline comments for maintainers.

**F-28: Checkpoint filename has 1-second granularity collision risk** (GQ-15)
Two creates in the same second produce the same filename. Consider sub-second precision (`20060102-150405.000`).

### Reviewer Attribution

| Finding | Reviewer(s) | Model |
|---------|-------------|-------|
| F-01 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-02 | Constitution Reviewer + Learnings Researcher | claude-opus-4.6 |
| F-03 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-04 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-05 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-06 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-07 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-08 | Go Quality Reviewer + Learnings Researcher | claude-opus-4.6 (go-engineer) |
| F-09 | Architecture Strategist + Scope Boundary Auditor | gpt-5.4 + claude-opus-4.6 |
| F-10 | Architecture Strategist | gpt-5.4 |
| F-11 | Agent-Native Parity Reviewer | gpt-5.4 |
| F-12 | Agent-Native Parity Reviewer + Scope Boundary Auditor | gpt-5.4 + claude-opus-4.6 |
| F-13 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-14 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-15 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-16 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-17 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-18 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-19 | Scope Boundary Auditor | claude-opus-4.6 |
| F-20 | Scope Boundary Auditor | claude-opus-4.6 |
| F-21 | Scope Boundary Auditor | claude-opus-4.6 |
| F-22 | Constitution Reviewer | claude-opus-4.6 |
| F-23 | Agent-Native Parity Reviewer | gpt-5.4 |
| F-24 | Agent-Native Parity Reviewer | gpt-5.4 |
| F-25 | Learnings Researcher | claude-opus-4.6 |
| F-26 | Scope Boundary Auditor | claude-opus-4.6 |
| F-27 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |
| F-28 | Go Quality Reviewer | claude-opus-4.6 (go-engineer) |

### Next Steps

**Gate: FAIL** — The plan must be revised to address P0 and P1 findings before proceeding to harvest. Recommended revision priority:

1. **F-01** (P0): Define all five supporting types with fields, tags, and constraints
2. **F-02** (P1): Add path-traversal containment to lifecycle functions
3. **F-03 + F-04** (P1): Fix config backwards-compatibility with post-load defaulting
4. **F-05** (P1): Add sentinel errors and `domainError()` cases
5. **F-06** (P1): Add `context.Context` to all lifecycle signatures
6. **F-07** (P1): Use pointer for optional struct fields with `omitempty`
7. **F-08** (P1): Apply Windows-safe rename to archive move
8. **F-09** (P1): Clarify schema validation boundary (persist version, enforce on read/write)
9. **F-10** (P1): Quarantine unparseable files instead of skip-with-warning
10. **F-11** (P1): Add CLI command group for checkpoint management
11. **F-12** (P1): Specify recovery state machine with testable paths

P2 findings should be addressed where practical but are at user discretion.
