---
chunk_strategy: h1-h2-h3
description: 'Complete ErrWriteIndeterminate caller reconciliation and durable mkdir/append retry idempotency across five durable_writes sites before durable mode is promoted toward default/GA.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-28-durable-writes-second-layer-hardening-plan.md
title: 'durable_writes second-layer hardening: caller reconciliation and retry idempotency'
---

# durable_writes second-layer hardening plan

Source document: `docs/decisions/2026-07-28-durable-writes-second-layer-hardening-deliberation.md`
Prior art: `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`,
`docs/exec-plans/2026-07-27-durable-writes-fsync-protocol-plan.md`
Origin: Copilot review cycle-4 on PR #308 (feature 123-F, shipment 109-S), stash `50471E28`.

## Problem Frame

Feature 123-F landed the durable_writes fsync primitives and the two-class
outcome-based error contract in `internal/errors/durability_errors.go`:

- `ErrWriteNotApplied` — the mutation definitely did not apply (failure before
  the atomic rename/commit); the failed atomic write is safe to retry.
- `ErrWriteIndeterminate` — the mutation is possibly-applied (post-rename fsync
  or non-atomic append failure); callers MUST NOT blindly retry and MUST NOT
  roll back.

Five callers/retry paths were not fully reconciled to that contract. Because
`durable_writes` is opt-in (default `false`) and the failures are triple-gated
(durable ON + fsync failure + retry), with Markdown as source of truth and the
SQLite index self-healing on `sync`, these are P2 follow-ups — but they must be
closed before durable mode is promoted toward default/GA, where the triple gate
would no longer be the default posture.

Governing rule for all units: **commit-then-surface**. Surface post-mutation
fsync failures as `ErrWriteIndeterminate` and never roll them back; keep
pre-mutation failures as `ErrWriteNotApplied`; make retry paths re-attempt a
previously failed parent flush instead of early-returning past it.

## Requirements Trace

Each acceptance criterion from stash `50471E28` maps to exactly one
implementation unit, each scoped to a single production file plus its colocated
`_test.go`. Current locations verified by grep (line numbers drift).

| AC | Requirement | Site (verified) | Unit |
|---|---|---|---|
| AC1 | `UnarchiveItem` non-git branch: on `ErrWriteIndeterminate` from the restore write, complete-or-rollback instead of early return leaving archive copy + DB row + duplicate files unrecoverable | `internal/core/archive.go` `UnarchiveItem` non-git else branch (~L768 restore content write) | U1 |
| AC2 | `AddDependency` / `RemoveDependency` treat every `persistArtifact` error as file-untouched and roll back the DB edge; add explicit `ErrWriteIndeterminate` reconciliation so durable-mode fsync failure does not diverge MD from the live index (or document sync-rebuild reliance) | `internal/core/dependencies.go` (L46, L100) calling `persistArtifact` in `internal/core/shipment.go` (L366) | U2 |
| AC3 | `appendDurable` durable retry: after a parent-dir fsync fails, a retry early-returns via `EventWriter.mkdirAllDurable` and never re-fsyncs the parent; make the durable append retry re-attempt the parent flush (`ErrWriteNotApplied` safe-retry contract) | `internal/events/stream.go` `appendDurable` (~L133) + `EventWriter.mkdirAllDurable` (~L169) | U3 |
| AC4 | `mkdirAllDurable` durable retry: if mkdir succeeds but the parent fsync fails, the `os.Stat` early-return on retry skips the failed flush permanently; on the durable existing-dir path, unconditionally re-fsync the parent so a previously failed dirent flush is re-attempted | `internal/core/durable_fs.go` `mkdirAllDurable` (~L33) | U4 |
| AC5 | MCP `append_comment` maps every append failure to a generic internal error; map both durability classes (not-applied vs indeterminate) to explicit machine-readable MCP outcomes so agents do not duplicate comments on retry | `internal/mcp/tools.go` `handleAppendComment` (~L943) | U5 |

## Implementation Units

Every unit is **test-first** (write the failing regression test first, confirm
red, then implement to green). Every unit uses an existing in-process
fault-injection seam so the triple-gated failure is reachable without a real
power loss. No `t.Parallel` in tests that swap package-global seams.

### U1 — Reconcile ErrWriteIndeterminate in UnarchiveItem non-git branch

- **File**: `internal/core/archive.go` (+ `archive_durable_write_test.go`).
- **Change**: In the non-git `else` branch of `UnarchiveItem`, when
  `replaceFileWithOptions` returns `ErrWriteIndeterminate` (rename committed,
  parent fsync uncertain), do NOT return early leaving both the archive copy and
  the restored file. Complete the restore per commit-then-surface: proceed to the
  archive-file removal + DB upsert so FS and DB agree, and surface the
  indeterminate signal wrapped, rather than aborting with a duplicate on disk.
  Preserve current behavior for `ErrWriteNotApplied` (file untouched → the
  existing early return is correct).
- **Seam**: `replaceFileWriteFn` (package-global in `archive.go`); precedent test
  `archive_durable_write_test.go`. **Seam-fidelity requirement**: the test
  override MUST perform a real write of the restored bytes to `originalPath`
  (mirroring the production write) and *then* return the wrapped
  `ErrWriteIndeterminate`, so the real indeterminate state (rename committed →
  file present) is genuinely reproduced. A whole-function stub that returns the
  sentinel *without* writing the file would let the "no duplicate remains"
  assertion pass trivially and is NOT acceptable.
- **Depends on**: U4 (consumes shared `core.mkdirAllDurable` at
  `archive.go` restore-dir creation; U4 changes that function's durable retry
  behavior — see Dependency Graph).
- **Tests (test-first)**: (a) indeterminate on restore write (seam writes then
  returns indeterminate) → exactly one canonical file remains, archive copy
  removed, DB row present, function surfaces `IsWriteIndeterminate`, no rollback;
  (b) not-applied on restore write → unchanged safe-abort behavior; (c)
  durable-off happy path unchanged.
- **Posture**: test-first. **Runtime surface**: none (internal core).

### U2 — Explicit indeterminate reconciliation for dependency callers

- **File**: `internal/core/dependencies.go` (+ colocated
  `dependencies_indeterminate_test.go`).
- **Change**: In `AddDependency` and `RemoveDependency`, distinguish the two
  classes of `persistArtifact` error. For `ErrWriteNotApplied` keep the existing
  DB-edge rollback (file untouched). For `ErrWriteIndeterminate` do NOT roll back
  the DB edge (the MD write likely persisted; rolling back diverges MD from the
  live index); surface the indeterminate signal so the caller sees the honest
  durability state. Model the caller pattern on the existing cross-ref reference
  `artifact_references.go` (`applyCrossRefWriteFn`) +
  `artifact_references_indeterminate_test.go`. If explicit reconciliation proves
  disproportionate, the documented fallback is a comment noting reliance on
  `sync`-rebuild — default to reconciliation.
- **Seam (CORRECTED)**: `AddDependency`/`RemoveDependency` call
  `persistArtifact(..., relocate=false)`. On that same-path (`currentPath ==
  targetPath`) rewrite the cross-directory source-dir fsync block in
  `persistArtifact` (gated on `srcDir != dstDir`) is **skipped**, so the
  `mkdirDirSyncFn` source-dir seam does NOT fire on this path. The only
  indeterminate source on the `relocate=false` path is the post-rename parent
  fsync inside `WriteArtifactFileWithOptions` (atomicfile). Therefore U2 MUST
  either (i) add a `persistArtifact` write seam mirroring
  `applyCrossRefWriteFn`, or (ii) inject the failure through the atomicfile
  write seam. **Precondition assertion**: add a test asserting `persistArtifact`
  propagates `blerrors.ErrWriteIndeterminate` on the `relocate=false` write, so
  U2 does not silently regress if that cross-file contract changes.
- **Tests (test-first)**: (a) indeterminate persist → DB edge NOT rolled back,
  indeterminate surfaced; (b) not-applied persist → DB edge rolled back
  (unchanged); (c) durable-off path unchanged; plus the `persistArtifact`
  propagation precondition assertion above.
- **Depends on**: U4 (the `relocate=false` persist path creates/uses dirs via
  shared `core.mkdirAllDurable`).
- **Posture**: test-first. **Runtime surface**: none (internal core).

### U3 — Re-attempt parent flush on durable append retry

- **File**: `internal/events/stream.go` (+ `stream_durable_test.go`).
- **Change**: `EventWriter.mkdirAllDurable` early-returns `nil` when `logsDir`
  already exists, so a retry after a prior parent-fsync failure never re-fsyncs
  the parent — a later append reports success while the logs dirent is not
  power-loss durable. Make the durable path **unconditionally re-fsync the parent
  of the existing `logsDir`** before returning (idempotent, and safe under the
  `ErrWriteNotApplied` pre-append contract — nothing is appended yet). This is a
  stateless function with no per-dir "confirmed" flag, so the fix is an
  unconditional re-flush, not a conditional one; note the added per-append
  dir-fsync cost. Do not double-append the event.
- **Seam**: `EventWriter.fsyncDirImpl` / `fsyncFileImpl` (`WithDurableWrites`);
  `stream_durable_test.go`. The `fsyncDirImpl` seam fires for BOTH the
  mkdir parent-of-logsDir flush and the per-append logsDir flush, so the U3 test
  MUST fail the seam **selectively by path argument** to isolate the parent-flush
  retry from the append-dir flush.
- **Tests (test-first)**: (a) first durable append: parent fsync fails (seam
  fails on the parent path only) → error; retry append: parent fsync re-attempted
  and succeeds → durable, and the event line count is exactly one (no duplicate);
  (b) durable-off path byte-for-byte unchanged.
- **Posture**: test-first. **Runtime surface**: item-log JSONL append path.

### U4 — Re-fsync existing dir in core mkdirAllDurable durable retry

- **File**: `internal/core/durable_fs.go` (+ `durable_fs_test.go`).
- **Change**: `mkdirAllDurable` returns `nil` at `os.Stat(dir); err == nil`,
  so if a prior call created `dir` but its parent fsync failed, the retry skips
  the failed flush permanently. This is a stateless `(dir, durable)` function
  with no per-dir confirmation flag, so the fix is: when `durable` and the target
  dir already exists, **unconditionally re-fsync its parent** before returning
  (idempotent, `ErrWriteNotApplied`-safe pre-write step); note the added per-call
  dir-fsync cost. Keep `durable == false` exactly `os.MkdirAll`. Wrap any re-fsync
  failure with `%w` around `blerrors.ErrWriteNotApplied` for parity with the
  events variant. Also fix the stale doc-comment cross-reference in this file
  (it currently says it mirrors "the U5 events level-by-level durable-mkdir";
  the events counterpart is the U3 change).
- **Cross-consumer regression (blast radius)**: `core.mkdirAllDurable` is shared,
  consumed at runtime by `UnarchiveItem` (U1) and the `persistArtifact` path (U2).
  The new unconditional re-fsync of an existing dir MUST NOT falsely surface
  indeterminate for a confirmed/happy-path dir. Add an assertion that the
  durable happy path (existing dir, fsync succeeds) returns `nil` and does not
  regress U1/U2 behavior. Landing U4 before U1/U2 (dependency edges below) keeps
  those consumers building on the final shared behavior.
- **Seam**: `mkdirDirSyncEnabled` / `mkdirDirSyncFn` package-globals;
  `durable_fs_test.go` (no `t.Parallel`).
- **Tests (test-first)**: (a) mkdir succeeds, parent fsync fails on first call →
  error wrapped with `ErrWriteNotApplied`; retry (dir now exists) → parent fsync
  re-attempted; (b) durable-off → plain MkdirAll, seam never invoked; (c)
  already-existing durable dir, fsync succeeds → `nil` (no false indeterminate).
- **Posture**: test-first. **Runtime surface**: none (internal core).

### U5 — Map durability classes to explicit MCP outcomes in append_comment

- **File**: `internal/mcp/tools.go` `handleAppendComment` (~L943) plus a small
  shared mapper helper in the `mcp` package (colocated with `domainError` /
  `gateErrorResult`), not a new file; `append_comment_test.go`.
- **Change**: Replace the blanket `InternalError` mapping with explicit,
  machine-readable outcomes. Add a reusable `durabilityOutcomeResult(op, err)`
  helper that classifies `core.AppendComment` errors via
  `blerrors.IsWriteNotApplied` and `blerrors.IsWriteIndeterminate` and returns a
  structured result **reusing the existing `gate_errors.go` envelope shape**
  (stable `"error"` type key + `"retryable"` boolean + `"message"`, built via
  `marshalGateError` and returned as an MCP **error result** through
  `NewToolResultError`, exactly like `gateErrorResult`) so agents branch on the
  same fields across tools. Contract (pinned, so the red test binds to a stable
  shape):
  - `ErrWriteNotApplied` → `"error": "write_not_applied"`, `"retryable": true`
    (the comment was NOT appended; a scoped retry is safe).
  - `ErrWriteIndeterminate` → `"error": "write_indeterminate"`,
    `"retryable": false` (the comment MAY have been appended; the agent MUST NOT
    blindly retry or it duplicates). This outcome MUST be machine-distinguishable
    from the generic `internal` error an agent would auto-retry — do NOT collapse
    it into `internal`. Both durability classes return via the error-result path
    (not a success-shaped payload), mirroring `gateErrorResult`.
  - Non-durability errors keep the generic `internal` error mapping.
  - Success response stays backward-compatible; optionally add an additive
    `"retryable": false` marker WITHOUT removing `{"ok":true}`.
  Evaluate `IsWriteNotApplied` before `IsWriteIndeterminate` (mutually exclusive
  classes) and assert that ordering in tests.
- **CLI parity decision (recorded)**: U5 is MCP-only by design. The CLI
  `comment add` path shares `core.AppendComment` but surfaces the wrapped
  sentinel as plain error text. This user/agent asymmetry is **accepted and
  intentional**: a human reads the wrapped "indeterminate" text and decides,
  whereas an agent needs the structured marker to avoid auto-retry duplication.
  No CLI machine-readable marker is added in this feature.
- **Seam**: behavioural test surface in `append_comment_test.go`; inject the
  durability error via the events writer seam or a stubbed `AppendComment`. Also
  add a cross-package propagation assertion that `IsWriteIndeterminate` returns
  true on the error the MCP handler receives (locking the `errors.Is` chain
  through `core.AppendComment`).
- **Tests (test-first)**: (a) indeterminate append → `write_indeterminate` /
  `retryable:false` outcome, distinct from generic internal error; (b)
  not-applied append → `write_not_applied` / `retryable:true` outcome; (c)
  generic error → unchanged internal-error mapping; (d) success → unchanged
  `{"ok":true}`; (e) **retry-idempotency**: after a not-applied outcome, a retry
  produces exactly ONE comment event (mirrors the `TestSizeSeam_*` / U3
  exactly-once assertion); the indeterminate outcome carries the do-not-retry
  marker.
- **Posture**: test-first. **Runtime surface**: MCP `append_comment` tool
  (agent-facing contract).

## Dependency Graph

U4 changes the **shared** `core.mkdirAllDurable` function, which is consumed at
runtime by U1 (`UnarchiveItem` restore-dir creation in `archive.go`) and by U2's
`persistArtifact` path. So U4 is NOT independent of U1/U2 at runtime — its new
unconditional re-fsync of an existing dir alters behavior for those consumers.
U4 is therefore sequenced first, and U1/U2 depend on it so they build and test
against the final shared behavior.

U3 (the `internal/events` `EventWriter.mkdirAllDurable`) and U5 (the MCP handler)
touch code with no shared runtime state with the others and are independent.
U3 and U4 fix the same bug class in two separate `mkdirAllDurable`
implementations that do not share code today (consolidation is a deferred
follow-up, not this feature).

```text
U4 ──► U1
  └──► U2
U3            (independent)
U5            (independent)
```

Dependency edges to wire in the backlog: `U1 depends on U4`, `U2 depends on U4`
(`dep_type: blocks`). U3 and U5 have no upstream dependencies.

## Decisions and Rationale

- **One unit per file, test-first, existing seams**: satisfies the 2-Hour Rule
  and Width Isolation, keeps five distinct regressions independently bisectable,
  and reuses proven fault-injection seams rather than inventing new ones (U2 adds
  a small `persistArtifact` write seam mirroring the existing
  `applyCrossRefWriteFn` where the documented seam does not fire on the
  `relocate=false` path).
- **Commit-then-surface is mandatory**: rolling back an indeterminate write is
  the primary hazard (diverges FS from DB or destroys an applied change), per the
  compound learning; every unit's tests assert no rollback and no duplicate on
  the indeterminate path.
- **AC2 defaults to reconciliation over the sync-rebuild escape hatch**: explicit
  do-not-roll-back on indeterminate keeps MD and the live index consistent
  without depending on a later `sync`.
- **AC5 reuses the `gate_errors.go` outcome envelope**: the durability classes
  map to the existing `"error"` type key + `"retryable"` envelope (built via
  `marshalGateError`, returned as an MCP error result) so agents branch on the
  same fields across MCP tools, rather than introducing a third outcome
  convention.
- **U3/U4 duplication is deferred, not consolidated here**: extracting a shared
  durable-mkdir primitive into the `fsutil` leaf that both `events` and `core`
  consume is the correct long-term fix, but doing it now would widen scope and
  couple the two package edits. Recorded as a follow-up so the two copies cannot
  silently diverge again.
- **Unconditional re-fsync over a "confirmed" flag (U3/U4)**: both
  `mkdirAllDurable` variants are stateless functions with no per-dir confirmation
  state, so the only idempotent, `ErrWriteNotApplied`-safe fix is an
  unconditional parent re-flush on the existing-dir path, accepting a small
  per-call dir-fsync cost.

## Risks and Caveats

- **Rolling back an indeterminate write** (diverges FS/DB): mitigated by the
  mandatory commit-then-surface contract and per-unit assertions that the
  indeterminate path neither rolls back nor duplicates.
- **Retry double-applies** (duplicate audit event / duplicate comment): U3 and U5
  each include an explicit test asserting event/comment count stays exactly one
  across a retry, mirroring the existing `TestSizeSeam_*` idempotency assertions.
- **U4 shared-function blast radius**: the new unconditional re-fsync must not
  falsely surface indeterminate for a confirmed dir; U4 test (c) and the U1/U2
  durable happy-path assertions guard this, and the U4→U1/U2 sequencing keeps
  consumers on the final behavior.
- **U2 seam reachability**: the named cross-directory source-dir fsync seam does
  not fire on the `relocate=false` dependency path; U2 injects at the atomicfile
  write seam (or a new `persistArtifact` write seam) and asserts `persistArtifact`
  propagates `ErrWriteIndeterminate` as a precondition.
- **Windows/POSIX seam**: dirent durability is best-effort on Windows; tests swap
  the package-global seams (no `t.Parallel`) to exercise POSIX ordering
  in-process on a Windows host.
- **AC5 MCP contract-shape change**: additive on an already-failing path and
  unreachable in the default config (`durable_writes=false`); the exact outcome
  shape is pinned in U5; see Plan Hardening.

## Constitution Check

- **Safety-First Go (NON-NEGOTIABLE)** — pass. All units are Go; errors wrap with
  `%w` and route through `blerrors` sentinels; no `unsafe`.
- **Test-First Development (NON-NEGOTIABLE)** — pass. Every unit lands a failing
  regression test (red) before implementation (green), using named existing
  seams (U2 adds a small write seam mirroring `applyCrossRefWriteFn`).
- **Workspace Isolation and Security Boundaries** — pass. All file operations stay
  within `.backlogit`; no new path construction escapes the workspace root; no
  secrets.
- **CLI Workspace Containment (NON-NEGOTIABLE)** — pass. No writes outside the
  current working tree; the three non-stageable stash entries requiring
  out-of-tree writes are explicitly excluded.
- **Structured Observability** — pass. Existing `slog` and error-wrapping paths
  are preserved; no observability regressions.
- **Single Responsibility** — pass. No new dependencies; reuses existing seams and
  the `blerrors` contract.
- **Destructive Command Approval (NON-NEGOTIABLE)** — N/A. No destructive terminal
  commands in the implementation units.
- **Explicit Safety Modes (VIII)** — pass. The moderate-risk actions (U5 MCP
  contract refinement, U1/U2 reconciliation behavior) are classified in the
  `## Plan Hardening` section using the strict-safety `ProposedAction` /
  `ActionRisk` vocabulary.
- **Git-Friendly Persistence (IX)** — pass. The commit-then-surface contract
  preserves atomic-write integrity for Git-mergeable Markdown/JSONL workspace
  state; no format changes.
- **Agent Context Efficiency (X)** — pass. U5 replaces a blanket internal error
  with a structured, machine-readable outcome so agents avoid redundant retries
  and duplicate comments.
- **Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ship will merge
  via a merge commit; no squash/rebase.

Constitution Check: pass

## Plan Hardening Signals

- public API, schema, or contract change — **present**. AC5/U5 refines the
  agent-facing MCP `append_comment` error-outcome contract (additive, on an
  already-failing path).
- security, auth, permission, or compliance-sensitive behavior — absent.
- migration, backfill, destructive data/config action, or irreversible step —
  absent.
- external integration, operator checkpoint, or external dependency — absent.
- high runtime, rollout, or rollback risk — absent (opt-in default `false`,
  triple-gated, P2; MD source of truth; index self-heals on `sync`).

Requires plan hardening: yes

## Runtime Verification and Closure

- **U1, U2, U4** — internal core; no runtime surface. Verification is the
  regression tests plus `go test ./internal/core/...`.
- **U3** — item-log JSONL append path. Verify durable append remains durable
  after a retry and does not duplicate log lines (`go test ./internal/events/...`).
- **U5** — MCP `append_comment` tool. Verify the tool returns distinct
  machine-readable outcomes (`write_not_applied`/`retryable:true` vs
  `write_indeterminate`/`retryable:false` vs generic `internal`) via
  `go test ./internal/mcp/...`, confirm the retry-idempotency test (exactly one
  comment event after a not-applied retry) passes, and confirm the default
  (`durable_writes=false`) `{"ok":true}` response is unchanged. The CLI
  `comment add` parity asymmetry (plain wrapped text, no structured marker) is an
  accepted, documented decision — no CLI verification of a machine-readable
  marker is required.
- **Closure**: no monitoring/rollback infrastructure required — durable_writes is
  opt-in default `false`, so the changed paths are inert in the default config.
  Closure evidence is: full `go test ./...` green, `go vet ./...` clean,
  `golangci-lint run` clean, `gofmt -l .` clean (verify formatting on
  LF-normalized BOM-free copies per the compound learning's gofmt-on-Windows
  gotcha). Rollback is a revert of the merge commit; no data migration to unwind.

## Plan Hardening

Hardening required: **yes** — a single hardening signal is present (AC5/U5
refines the agent-facing MCP `append_comment` error-outcome contract). All other
signals are absent; the work is opt-in (default `false`), triple-gated, and P2.

Learnings and instructions consulted:

- `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  — the two-class contract, commit-then-surface pattern, POSIX/Windows seam, the
  opt-in-default severity heuristic, and the gofmt-on-Windows verification gotcha.
- `.github/instructions/go-mcp-server.instructions.md` and
  `.github/instructions/go.instructions.md` — MCP result conventions and Go error
  handling for U5.
- `docs/exec-plans/2026-07-27-durable-writes-fsync-protocol-plan.md` — the 123-F
  primitives and seam layout.

Protected invariants:

- Never roll back an `ErrWriteIndeterminate` write (commit-then-surface).
- A durable append or mkdir retry must re-attempt a previously failed parent
  flush and must never double-append the audit event.
- `durable_writes=false` behavior is byte-for-byte unchanged in every unit.
- The MCP change is additive: success responses and non-durability error
  responses are unchanged; only the two durability classes gain explicit
  machine-readable outcomes, and the indeterminate outcome is
  machine-distinguishable from the auto-retryable generic `internal` error.
- The U5 outcome shape is pinned to the existing `gate_errors.go` envelope
  (`"error"` type key + `"retryable"`, error-result path) so it does not
  introduce a third outcome convention.

Deferred follow-up (recorded, not in scope): extract a shared durable-mkdir
primitive into the `fsutil` leaf so `internal/events` and `internal/core` stop
maintaining two copies of the level-by-level durable-mkdir algorithm. Ship
should stash this as a new backlog item during closure.

ProposedAction / ActionRisk (strict-safety vocabulary):

- **ProposedAction**: refine MCP `append_comment` error-outcome mapping (U5).
  - `change_kind`: agent-facing contract refinement (additive).
  - `ActionRisk`: **moderate** — changes a contract shape on shared agent-facing
    surface, but additive and unreachable in default config; not destructive.
  - `rollback`: revert the merge commit; no data or schema to unwind.
  - `approval_required`: no (moderate, additive, opt-in-gated) — carried into
    review for visibility per the reviewability rule.
- **ProposedAction**: change indeterminate reconciliation behavior in
  `UnarchiveItem` (U1) and dependency callers (U2).
  - `ActionRisk`: **moderate** — alters error-path behavior on durable writes
    only; inert when `durable_writes=false`.
  - `rollback`: revert the merge commit.

Added verification / closure / rollback detail:

- Verification depth: each unit asserts all three of (indeterminate path,
  not-applied path, durable-off path); U3/U5 additionally assert
  exactly-once event/comment across a retry.
- Rollback trigger: any post-merge regression in the MCP `append_comment`
  outcome shape or a duplicated audit event → revert the merge commit.
- Owner / validation window: Ship owns runtime verification and post-merge
  closure; validation window is the CI run plus the closure test sweep. No
  production monitoring is required because the paths are opt-in and default-off.
- Unresolved operator decisions: none block execution. The AC2 sync-rebuild
  escape hatch is a documented fallback only if explicit reconciliation proves
  disproportionate; the default is reconciliation.

<!-- plan-review-attempt: 1 (FAIL: 2 P1 findings on U5 outcome contract + missing retry test; corroborated P2s on U1/U2 seam fidelity, U3/U4 framing, U4 blast radius) -->

<!-- plan-review-attempt: 2 (PASS) -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Plan hardening: required (yes) and satisfied — the `## Plan Hardening` section
classifies the moderate-risk actions (U5 MCP contract refinement, U1/U2
reconciliation) with strict-safety `ProposedAction` / `ActionRisk` vocabulary and
records rollback, verification depth, and the accepted CLI-parity asymmetry.

### Gate rationale

Attempt 1 (multi-agent dispatch of Go Reviewer, Constitution Reviewer, Scope
Boundary Auditor, Architecture Strategist, Agent-Native Parity Reviewer, and
Learnings Researcher) returned two P1 findings from the Agent-Native Parity
Reviewer plus corroborated P2s on U1/U2 seam fidelity, U3/U4 retry framing, and
U4 shared-function blast radius — a FAIL. The plan was revised to resolve every
P1 and P2. Attempt 2 re-dispatched the three personas that raised the blocking
and corroborated findings (Agent-Native Parity, Go, Architecture); all three
confirmed every prior P1 and P2 RESOLVED with no new blocking findings. The
non-blocking P2/P3 field-name and result-kind precision items were then folded in
(the U5 envelope is pinned to the real `gate_errors.go` `"error"`/`"retryable"`
keys via `marshalGateError`). Constitution, Scope, and Learnings raised no P0/P1
in attempt 1; their P3 advisories (Constitution VIII/IX/X line-items, U5
test-count/inline-helper, compound-learning alignment) were incorporated. Every
selected persona is covered; no P0 or P1 findings remain. Gate: PASS.

### Findings by severity

- **P0**: none.
- **P1**: none remaining. (Attempt 1: 2 P1 on U5 outcome contract + missing
  retry test — both RESOLVED and re-confirmed.)
- **P2** (all resolved in the revision): U2 seam does not fire on the
  `relocate=false` path (corrected to the atomicfile write seam + propagation
  precondition); U1 seam must write-then-return-indeterminate; U3/U4 unconditional
  re-fsync framing; U4 shared-function blast radius (U4→U1/U2 edges + happy-path
  regression); U5 pinned `gate_errors.go` envelope + shared `durabilityOutcomeResult`
  helper + CLI-parity decision.
- **P3** (acknowledged / incorporated): Constitution VIII/IX/X line-items added;
  U5 result-kind stated as error-result path; AC4 trace-row wording aligned to
  unconditional re-fsync; U3/U4 shared-`fsutil` consolidation recorded as a
  deferred follow-up; stale core `mkdirAllDurable` doc-comment fix folded into U4.

### Runtime verification and closure

No gaps. U3 (item-log append) and U5 (MCP `append_comment`) runtime surfaces have
explicit verification steps and retry-idempotency tests; U1/U2/U4 are internal
core covered by `go test`. Closure evidence is the full quality-gate sweep
(`go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` on
LF-normalized BOM-free copies). Rollback is a revert of the merge commit; no
data migration to unwind (opt-in, default `false`).
