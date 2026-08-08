---
title: "F5 multi-store mutation recovery contract"
description: "Micro-decision for formal-gate unit F5: the simplest viable recovery contract for governed multi-store mutations, given that single-file replacement is already atomic on both platforms and exactly-once machinery was already descoped at the root."
source: docs/decisions/2026-08-07-f5-multi-mutation-recovery-contract-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "F5 (Q6): write-ahead journal versus idempotent replay versus snapshot-rollback for governed multi-store mutations"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-08-07-f5-idempotent-multi-mutation-envelope-plan.md"
  - "docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md"
  - "docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md"
tags:
  - "governance"
  - "formal-gate"
  - "reliability"
  - "atomicity"
  - "core"
  - "stage"
---

## Problem Frame

Formal-gate spike Q6 states there is no journaling or all-or-nothing guarantee
across a multi-file / multi-store mutation (frontmatter + SQLite + JSONL), and
records the journal-versus-replay choice as **unresolved** — "deferred to F5's
micro-decision". F5 is ranked **last** in the spike's recommended ordering and
flagged as non-trivial with medium confidence.

The question to settle before planning: **what is the smallest recovery contract
that makes a partial failure detectable and recoverable, without building
speculative transaction machinery?**

### Constraints

- **Operator policy** — reliability first, but simplicity over complexity;
  explicitly "deliberate the simplest viable recovery contract and avoid
  speculative transaction machinery".
- **Principle VI** — no speculative dependencies or abstractions.
- F5 executes **after** F1, F4, and F6 have landed, so it must compose with the
  authenticated evidence path rather than replace it.

### Success criteria

- A governed multi-store mutation that fails midway leaves state that is either
  fully applied, fully reverted, or **explicitly and typedly reported as partial**
  with the completed step set named.
- No compensating action is ever taken on an indeterminate write.
- Recovery is exercisable by re-running the operation or by an existing
  diagnostic, not by a bespoke recovery tool.
- No new persistent on-disk state format is introduced unless it is genuinely
  necessary.

## Research Findings

### The spike's central Q6 premise is now obsolete

The spike asserted that `internal/atomicfile` used a Windows
`os.Remove`-then-rename fallback that "is **not** atomic", and derived from that
"a formal rollback contract must treat single-file replacement as non-atomic on
that platform". **HEAD no longer does this.** Verified this session:

- `internal/atomicfile/atomicfile_windows.go` uses `MoveFileEx` with
  `MOVEFILE_REPLACE_EXISTING` set **unconditionally** ("removing the destination
  first was a real data-loss defect"), adding `MOVEFILE_WRITE_THROUGH` only in
  durable mode. `ReplaceFileW` was deliberately rejected. The destination is
  never removed.
- `internal/atomicfile/atomicfile_other.go` uses a plain POSIX `os.Rename`.
- On failure in both files the destination is untouched, and the error is
  classified `ErrWriteNotApplied`.

**Single-file replacement is atomic on both platforms today.** The strongest
stated motivation for a write-ahead journal has already been removed by shipped
work. Planning a journal on the spike's stale premise would be building for a
defect that no longer exists.

### A two-class durable-write error contract already exists

`docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
records the shipped contract: `ErrWriteNotApplied` (failure before rename —
safe to retry) versus `ErrWriteIndeterminate` (rename happened, durability flush
failed — **must not** retry blindly and **must not** roll back). Its explicit
warning: "rolling back an indeterminate write is the primary hazard — the rename
likely persisted, so a rollback diverges FS from DB or destroys an applied
change." Any envelope must be gated on this classification.

### A proven snapshot-rollback pattern already exists in-repo

`ClaimShipment` (`internal/core/shipment_lifecycle.go:43-128`) snapshots
pre-mutation state, tracks the exact set it mutates, and calls
`rollbackShipmentClaim` on any mid-flight failure, restoring in reverse order.
The fallible post-activation read-back was **deleted** because "the read can fail
the same way the mutation did"
(`docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`).
`AddDependency` / `RemoveDependency` (`internal/core/dependencies.go:29-57,77-123`)
already implement a narrower version of the same shape, including the
indeterminate-write special case.

### Exactly-once machinery was already tried and descoped at the root

`docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md`
confirms none of the three prerequisites exist: no `OpID` / idempotency key on
either transport, no deterministic multi-orphan ordering, and no reachable
offline reconciliation. `OpID`, `PrevOpID`, exactly-once, and CAS reconcile were
all **dropped at the root** in 099-S cycle 3, because internally generated OpIDs
make retry dedup impossible and CAS reconcile is nondeterministic when two crash
residues share a `PrevOpID`.

### The real remaining defect

`LinkCommit` (`internal/core/commits.go:27-56`) writes SQLite first, then appends
the JSONL event **best-effort**: on append failure it logs a warning and still
returns `nil`. It also does not rewrite markdown. Meanwhile CLI
`update --commit` writes only the frontmatter scalar
(`internal/cli/update.go:194-196`). So a "commit association" leaves one, two, or
three of the three representations updated depending on which surface ran and
whether the append succeeded — and the caller is told it succeeded either way.
That silent partial success is the concrete, demonstrable Q6 defect.

### Transaction surface available today

`internal/db` exposes `*sql.Tx` helpers (`UpsertItemsTx`, `upsertItemTx` in
`internal/db/upsert_tx.go:12-26`) but no `BeginTx` facade; callers use
`(*sql.DB).BeginTx` directly. `Rehydrate` already wraps clear-and-rebuild in one
transaction. There is no `journal` or `wal` package anywhere under `internal/`.

## Options Evaluated

### Option A — Write-ahead journal

A durable journal file records intent before each step; an opener replays or
rolls back unfinished journal entries.

- **Pros:** survives process kill mid-sequence; classic and well understood.
- **Cons:** introduces a new persistent on-disk format under the storage root,
  a replay path that must itself be crash-safe, and a new corruption class. The
  motivating Windows non-atomicity is gone. The high-water-counter YAGNI
  determination refused a much smaller durable state file on exactly this
  reasoning. Cannot be decomposed into bounded ~2h units without leaving
  half-built machinery in the tree.
- **Effort:** high. **Fit:** low.

### Option B — Idempotent replay with an internally generated operation id

Each mutation gets an id; steps are idempotent; a reconciler dedups on replay.

- **Pros:** no rollback hazard; naturally retry-safe.
- **Cons:** **already tried and descoped at the root**. Without transport-visible
  id ingress a client retry submits a *new* id, so the orphan is never
  deduplicated. Re-proposing it re-opens a closed decision.
- **Effort:** high. **Fit:** very low.

### Option C — Snapshot / track / compensate envelope, gated on the two-class error contract

Generalize the proven `ClaimShipment` shape into one small reusable envelope:
snapshot the pre-state of every representation the operation will touch, execute
ordered steps, and on failure compensate **only** the steps whose errors classify
as `ErrWriteNotApplied`. Any `ErrWriteIndeterminate` short-circuits compensation
and is surfaced as a typed partial-application error naming exactly which steps
completed. Steps are written to be idempotent so a plain re-run converges.

- **Pros:** reuses a pattern already proven in this codebase; no new on-disk
  format; no new corruption class; directly honors the indeterminate-write
  hazard; decomposes cleanly into bounded units; makes the real `LinkCommit`
  defect impossible to reintroduce.
- **Cons:** does **not** survive a process kill between steps — an in-memory
  envelope cannot. Requires an honest statement of that boundary.
- **Effort:** medium. **Fit:** high.

### Option D — Declare multi-store mutations advisory-only

Take the spike's own escape hatch: the formal gate simply refuses to reason over
multi-store mutations.

- **Pros:** zero code.
- **Cons:** leaves `LinkCommit`'s silent partial success in place, which is a real
  reliability defect independent of the gate. Fails the operator's
  reliability-first policy.
- **Effort:** none. **Fit:** low.

## Trade-off Comparison

| Criterion | A (journal) | B (OpID replay) | C (snapshot/compensate) | D (advisory) |
|---|---|---|---|---|
| Complexity | high | high | medium | none |
| New on-disk state | yes | yes | **no** | no |
| New corruption class | yes | yes | **no** | no |
| Reuses proven in-repo pattern | no | no | **yes** | n/a |
| Survives process kill mid-sequence | yes | partial | **no** | n/a |
| Honors indeterminate-write hazard | must be added | must be added | **by construction** | n/a |
| Already-closed decision reopened | no | **yes** | no | no |
| Fixes the concrete `LinkCommit` defect | yes | yes | **yes** | **no** |
| Decomposes into bounded ~2h units | no | no | **yes** | n/a |
| Speculative under Principle VI | **yes** | **yes** | no | no |

## Decision

**Adopt Option C — a snapshot / track / compensate envelope, gated on the
two-class durable-write error contract, with idempotent steps.**

### The contract

1. **Scope.** The envelope covers *governed* multi-store mutations only:
   commit association, and the create-item + dependency + shipment-membership
   path. It is not applied repo-wide.
2. **Snapshot before mutate.** Capture the pre-state of every representation the
   operation will touch. Never use a post-mutation read-back as the integrity
   guarantee.
3. **Ordered idempotent steps.** Each step is safe to re-run. Re-running the whole
   operation converges to the same state.
4. **Classified failure handling.**
   - `ErrWriteNotApplied` → compensate the tracked set in reverse order.
   - `ErrWriteIndeterminate` → **do not compensate**. Finish the remaining steps
     where safe so the stores agree, then surface the accumulated error
     (commit-then-surface).
   - Any other error → compensate, then surface.
5. **Typed partial result.** When the operation cannot be made whole, return a
   typed error naming the completed step set and the classification. No
   warn-and-continue inside the envelope; `LinkCommit`'s current best-effort
   JSONL append is removed as part of F5.
6. **Reconciliation via `doctor`.** Detection of a residual partial state is added
   to the existing `doctor` check-registration pattern
   (`internal/core/doctor.go:133-165`) rather than a bespoke recovery tool.
7. **EventWriter threading.** The envelope threads the caller's
   `*events.EventWriter` (MCP passes the server's; CLI passes `nil`). It must
   never mint one internally — that silently drops the MCP server's append
   serialization.

### Explicitly NOT built

No write-ahead journal. No `OpID` / `PrevOpID` / CAS / exactly-once machinery.
No new persistent on-disk format. No bespoke recovery CLI. No cross-process
locking. Each of these is either descoped by a prior decision or refused here as
premature.

### Honest boundary (must be documented)

> This envelope makes a *failed* governed mutation either fully applied, fully
> reverted, or explicitly reported as partial with the completed steps named. It
> is an in-process construct and therefore does **not** survive a process kill
> between steps. Crash-window exactly-once semantics were evaluated separately
> and descoped at the root; residual crash residue is detected by `doctor`, not
> prevented by this envelope.

## Rejected Alternatives

- **Option A (write-ahead journal)** — its strongest justification, Windows
  non-atomic replacement, no longer exists at HEAD. Building it now would be
  exactly the premature abstraction the high-water-counter YAGNI determination
  and the exactly-once spike both refused.
- **Option B (OpID replay)** — already descoped at the root in 099-S cycle 3 for
  reasons that have not changed: no transport-visible id ingress.
- **Option D (advisory-only)** — leaves a real silent-partial-success defect in
  `LinkCommit`, which violates the operator's reliability-first policy.

## Unresolved Questions

- Whether the envelope should later be extended to the archive path is deferred;
  archival already has its own git-aware move/rollback machinery
  (`planArtifactMove` / `performArtifactMove` / `rollbackGitArtifactMove`).
- Whether `doctor` should gain an automatic fix for a detected partial commit
  association is deferred to a follow-up; detection-only ships first.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Compensation itself fails | Log all diagnostic values on double fault (crash-safe delete precedent) and surface a typed double-fault error; never silently swallow |
| Envelope wrongly compensates an indeterminate write | Classification gate is the first branch of the failure path and is covered by a dedicated test using the existing injectable write seams |
| Envelope grows into a general transaction manager | Scope is fixed to governed operations in the plan; a scope-boundary check is a named review persona trigger |
| Removing `LinkCommit`'s best-effort append changes behavior | The behavior change is the point and is stated in the plan; a characterization test records the old behavior before the change |
| Package-global seam tests race | The existing rule applies: no `t.Parallel()` in any package that overrides a package-global write seam |
