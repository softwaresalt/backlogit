---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for formal-gate unit F5: a snapshot/compensate envelope for governed multi-store mutations, gated on the two-class durable-write error contract, with machine-readable partial-mutation results and doctor detection, and no write-ahead journal or exactly-once machinery.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-f5-idempotent-multi-mutation-envelope-plan.md
title: 'F5 — idempotent multi-store mutation envelope'
---

# F5 — idempotent multi-store mutation envelope

Source deliberation:
`docs/decisions/2026-08-07-f5-multi-mutation-recovery-contract-deliberation.md`.
Authoritative research inputs:
`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` (Q6, F5)
and `docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md`.

This is the **final** formal-gate release unit. It executes after F1, F4, and F6.

<!-- plan-review-attempt: 2 -->

## Problem Frame

There is no all-or-nothing guarantee across a multi-store mutation
(frontmatter + SQLite + JSONL). The ship path persists an artifact and then calls
`LinkCommit` with no wrapper (`internal/core/shipment_lifecycle.go:563-566`).
`LinkCommit` writes SQLite first, then appends JSONL best-effort, warning and
returning `nil` on failure (`internal/core/commits.go:27-56`). A caller is
therefore told a governed mutation succeeded when it partially failed.

**One spike premise is obsolete.** The spike asserted `internal/atomicfile` used
a Windows `os.Remove`-then-rename fallback. HEAD does not:
`atomicfile_windows.go` uses `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING`
unconditionally, and `atomicfile_other.go` uses plain `os.Rename`. Single-file
replacement is atomic on both platforms today, which removes the strongest
justification for a write-ahead journal. This correction is recorded in
documentation (U7); it is **not** asserted through tests that couple this unit to
`atomicfile` implementation details, since `atomicfile` is not being changed.

### Success Criteria

* A governed multi-store mutation that fails midway is either fully applied,
  fully compensated, or **explicitly reported as partial** with the completed
  step set and classification.
* No compensating action is ever taken on an `ErrWriteIndeterminate` write.
* Re-running a governed mutation converges to the same state.
* Agents receive a **machine-readable** partial result distinguishing retryable
  from non-retryable outcomes.
* `doctor` detects residual partial state for **both** governed paths, through
  CLI and MCP.
* No new persistent on-disk format is introduced.

### Scope Boundaries

**In scope:** the envelope primitive; applying it to commit association (post-F6)
and to the create-item + dependency path and the shipment-membership path;
removing warn-and-continue from the governed route; machine-readable MCP mapping;
`doctor` detection for both governed paths; documentation.

**Out of scope:** write-ahead journaling. `OpID`/`PrevOpID`/CAS/exactly-once
machinery — descoped at the root in 099-S cycle 3 and not re-litigated. Any new
persistent on-disk state. A bespoke recovery CLI. Cross-process locking. The
archive path, which already has git-aware move-and-rollback machinery. Automatic
`doctor` repair — detection ships first. Any change to `internal/atomicfile`.

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| Detectable and recoverable partial failure | Spike Q6 | U2, U3, U4 |
| Never compensate an indeterminate write | `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` | U2 |
| Indeterminate dominates the joined classification | Review F5-02 (Go) | U2 |
| Typed error carrying the completed-step payload | Review F5-01 (Go) | U2 |
| Envelope must not depend on a domain type | Review F5 (Architecture) | U2 |
| Snapshot + track; never a post-mutation read-back | `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md` | U2 |
| No warn-and-continue inside a governed envelope | Spike Q6; `internal/core/commits.go:50-56` | U3 |
| Machine-readable partial result on the MCP surface | Review F5-1 (parity) | U6 |
| Detection covers both governed paths, CLI **and** MCP | Review F5 P1 (scope), F5-2 (parity) | U6 |
| No speculative machinery | Deliberation; Principle VI | scope boundary |

## Implementation Units

### U1 — Characterization: silent partial success (tests)

Capture today's behavior before changing it: a governed mutation whose JSONL
append fails still reports success. Scope is limited to the governed mutation
path; no assertions are made about `atomicfile` internals.

Files: `internal/core/mutation_envelope_characterization_test.go`.
Scenarios: append failure returns `nil` today; SQLite write succeeded while JSONL
did not; the caller has no way to detect the split.
Posture: characterization-first.

### U2 — Envelope primitive (code)

`core.MutationEnvelope` registers ordered idempotent steps, each an
`Apply func(context.Context) error` and a `Compensate func(context.Context) error`
closure. **The envelope takes no domain types** — no `*events.EventWriter`, no
`*sql.DB`; callers capture whatever they need inside their own closures. It
snapshots the pre-state its caller supplies, tracks the exact step set applied,
and on failure branches **first** on classification:

* `ErrWriteNotApplied` → compensate the tracked set in reverse order;
* `ErrWriteIndeterminate` → **do not compensate**; finish remaining safe steps so
  the stores agree, then surface the accumulated error (commit-then-surface);
* any other error → compensate, then surface.

**Named invariant (classification precedence):** if any step returns
`ErrWriteIndeterminate`, the envelope's top-level classification is
`Indeterminate` regardless of any other error in the joined set. This is asserted
directly, not left as a code-level detail.

The result is a typed struct in `internal/errors/mutation_errors.go`:

```go
type MutationPartialError struct {
    Completed         []string
    FailedStep        string // name of the step that returned the classifying error
    CompensationState string // "compensated" | "not-compensated" | "unknown"
    Class             string // "not-applied" | "indeterminate" | "double-fault"
    Cause             error
}
```

`FailedStep` and `CompensationState` are populated by the envelope at the point
of classification — never reconstructed later by parsing `Error()` text — so
U6's MCP mapping can read them directly as typed fields.

with `Error() string` and `Unwrap() error`; callers use `errors.As`. Accumulation
uses `errors.Join` so `errors.Is` still finds the underlying durable-write
sentinels. All diagnostic values are logged on a double fault.

Files: `internal/core/mutation_envelope.go`,
`internal/errors/mutation_errors.go`.
Scenarios: not-applied → compensated; indeterminate → **not** compensated and the
original write is positively asserted still present; mixed joined errors →
classification is `Indeterminate`; double fault → typed error with diagnostics;
re-run is idempotent.
Posture: test-first.

### U3 — Wrap commit association (code)

Wrap `core.AssociateCommit` (delivered by F6 as an ordered list of discrete
idempotent steps, specifically so it can be wrapped without rewriting) in the
envelope. Remove the remaining warn-and-continue path from the governed route.
The `*events.EventWriter` is captured inside the commit-association Apply
closure, never passed to the envelope.

Files: `internal/core/commits.go`.
Scenarios: JSONL append failure compensates the SQLite and frontmatter writes;
indeterminate frontmatter write is not compensated; success path unchanged.
Posture: test-first.

### U4 — Wrap the create-item + dependency path (code)

Wrap the governed create-and-link path so a failure part-way through does not
leave an item without its dependency edges. `internal/core/dependencies.go`
alone is insufficient: `CreateArtifact` (`internal/core/artifacts.go:124`) is
the entry point both CLI (`internal/cli/add.go`, via `core.WithDependencies`)
and MCP (`internal/mcp/tools.go`) invoke directly to create an item with
at-creation dependency edges, and it performs that linking itself rather than
delegating to a separate call. Wrap `CreateArtifact`'s own item-creation-plus-
dependency-linking steps in the envelope (no new shared entry point — both
callers already route through this one function), in addition to the
standalone `AddDependency` path in `internal/core/dependencies.go` used by
`backlogit dep add` after creation.

Files: `internal/core/dependencies.go`, `internal/core/artifacts.go`.
Scenarios: dependency step fails → item creation compensated; indeterminate
dependency write → not compensated; existing `AddDependency` rollback tests
still pass; `CreateArtifact` with `WithDependencies` set and a failing edge
write leaves no orphaned item (via CLI and MCP entry points alike).
Posture: test-first.

### U5 — Wrap the shipment-membership path (code)

Wrap the governed shipment-membership mutation. Preserve `ClaimShipment`'s
existing snapshot/rollback rather than replacing it; the envelope generalizes
that shape and must not regress it.

Files: `internal/core/shipment_lifecycle.go`.
Scenarios: membership step fails → prior steps compensated; `ClaimShipment`
rollback tests pass untouched.
Posture: test-first.

### U6 — Machine-readable partial results and doctor detection for both paths (code)

Add a `CheckPartialMutations` boolean and finding types through the existing
`DoctorOptions` registration pattern (`internal/core/doctor.go:133-165`),
detecting residual partial state for **both** governed paths: a commit
association present in one representation but not the others, and an item whose
dependency or shipment-membership edges are inconsistent with its frontmatter.
Advisory, never blocking; detection only, no automatic repair.

Expose the check through **both** `internal/cli/doctor.go` and the MCP
`backlogit_doctor` schema/handler, so an agent that receives a partial result has
a recovery-discovery path.

Map `MutationPartialError` to a machine-readable MCP `mutation_partial` response
carrying `classification`, `completed_steps`, `failed_step`,
`compensation_state`, `retryable`, and `recovery` guidance, following the
existing precedent that maps durable-write outcomes to machine-readable flags.

Files: `internal/core/doctor.go`, `internal/cli/doctor.go`,
`internal/mcp/tools.go`.
Scenarios: `commit_links` without a JSONL event → finding; inconsistent
dependency edges → finding; consistent state → no finding; MCP returns
`retryable: true` for not-applied and `false` for indeterminate.
Posture: test-first.

### U7 — Document the recovery contract (docs)

Document the envelope contract, the classification-precedence invariant, the
idempotent re-run rule, the `doctor` detection path, the machine-readable MCP
result shape, and — verbatim — the honest boundary from the deliberation
(in-process; does not survive a process kill). Record the corrected Q6
`atomicfile` premise so the stale conclusion is not re-derived.

Files: `docs/design-docs/governed-mutation-recovery-contract.md`, plus a pointer
from `docs/ARCHITECTURE.md`.
Posture: documentation.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──> U5 ──> U6 ──> U7
```

Strictly sequential. `U3` additionally depends on F6's `core.AssociateCommit`
having landed as discrete steps. `U2`'s classification test must be green before
any real call site is wrapped. `U7` last.

## Decisions and Rationale

* **In-process envelope over a write-ahead journal** — the journal's strongest
  justification no longer exists at HEAD. Building it now introduces a new
  persistent format and a new corruption class for no remaining benefit.
* **Envelope takes no domain types** — a generic coordination primitive that
  depends on `*events.EventWriter` leaks a domain concept into a
  general-purpose layer. Callers capture what they need in their closures.
* **Typed struct, not a bare sentinel** — a sentinel cannot carry the completed
  step list or the classification, which are exactly what a caller needs.
* **Indeterminate dominates** — a joined error set containing both classes must
  resolve conservatively, or compensation could run against an applied write.
* **Generalize the proven `ClaimShipment` shape** — already reviewed and shipped
  in this codebase.
* **No exactly-once machinery** — already descoped at the root; re-proposing it
  would reopen a closed decision.
* **Detection before repair** — `doctor` surfaces residual state; automatic
  repair waits for evidence that it is needed.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Envelope wrongly compensates an indeterminate write | **high** | Classification is the first branch; U2's test positively asserts the original write is still present |
| Joined errors produce an ambiguous classification | **high** | Named precedence invariant asserted directly in U2 |
| Compensation itself fails (double fault) | high | All diagnostic values logged; typed double-fault class; never swallowed |
| Envelope grows into a general transaction manager | medium | Scope fixed to governed operations; Scope Boundary Auditor is a review trigger |
| Removing warn-and-continue changes caller-visible behavior | medium | Intentional; characterized in U1; recorded in closure |
| Agents cannot distinguish retryable from non-retryable | **high** | U6 machine-readable `mutation_partial` mapping with a `retryable` flag |
| Agent has no recovery-discovery path | high | U6 exposes the doctor check through MCP, not only CLI |
| `ClaimShipment`'s existing rollback regresses | high | Its tests are a protected invariant and stay untouched |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. `MutationPartialError` in `internal/errors`; `errors.Join` accumulation; `errors.As` extraction. |
| II. Test-First | U1 characterization first; every code unit test-first. |
| III. Workspace Isolation | No new paths; no new persistent state. |
| IV. CLI Containment | No writes outside the workspace. |
| V. Structured Observability | Silent partial success becomes a typed, machine-readable error plus a doctor finding on both surfaces. |
| VI. Single Responsibility | No new dependencies. Journal and exactly-once machinery explicitly refused. |
| IX. Git-Friendly Persistence | No new on-disk format. |
| X. Context Efficiency | Doctor check is targeted and opt-in. |

No violations.

## Plan Hardening Signals

* high blast radius: wraps the ship path and governed create/link mutations — **yes**
* irreversible or partial-failure semantics — **yes**
* rollback and recovery behavior is the subject of the change — **yes**
* agent-facing error contract change — **yes**

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** commit association across CLI and MCP; the governed
  create-item + dependency path; the shipment-membership path; `doctor` on both
  surfaces.
* **Scenarios:** not-applied compensates; indeterminate does not; mixed joined
  errors classify as indeterminate; double fault reports diagnostics; re-run is
  idempotent; doctor detects residual state for both paths; MCP `retryable` flag
  is correct for each class.
* **Rollback:** plain revert; no persistent state introduced.
* **Closure artifact:** must record the honest in-process boundary and the
  corrected Q6 `atomicfile` premise.

## Plan Hardening

Hardening was required (four signals).

### Protected Invariants (must not regress)

1. `ErrWriteIndeterminate` is **never** compensated.
2. If any step is indeterminate, the top-level classification is `Indeterminate`.
3. `ClaimShipment`'s existing snapshot/rollback behavior and tests are untouched.
4. The envelope depends on no domain type.
5. Evidence and event appends keep their existing ordering relative to durable
   status writes.
6. Frontmatter remains the source of truth; SQLite remains a disposable
   projection.
7. `doctor` findings remain advisory and never block.
8. No new persistent on-disk format is introduced.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
* `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`
* `docs/decisions/2026-07-23-crash-window-exactly-once-size-mutation-spike.md`
* `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
* `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`
* `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
* `docs/decisions/2026-06-28-durable-highwater-counter-yagni-determination-deliberation.md`
* `.github/instructions/constitution.instructions.md` (I, VI),
  `.github/instructions/go.instructions.md`,
  `.github/instructions/strict-safety.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Wrap the governed create + link path in a compensating envelope | `internal/core/dependencies.go` | behavior change | **high** | Plain revert | **yes** |
| A2 | Add compensating (reverting) writes to governed mutations | `internal/core/mutation_envelope.go` | potentially destructive on the failure path | **destructive** | Plain revert; compensation gated on `ErrWriteNotApplied` only | **yes** |
| A3 | Wrap the shipment-membership path | `internal/core/shipment_lifecycle.go` | behavior change on a release-critical path | **high** | Plain revert | **yes** |
| A4 | Remove warn-and-continue from the governed path | `internal/core/commits.go` | behavior change | moderate | Plain revert | no |
| A5 | Add a doctor check and an MCP error mapping | `internal/core/doctor.go`, `internal/mcp/tools.go` | additive, advisory | low | Plain revert | no |

`ActionResult` for every entry starts `planned`. **A2 is destructive**: a
compensating write is a write, and mis-gating it on an indeterminate error would
destroy an applied change. Its classification test is mandatory before any call
site is wrapped.

### Deepened Verification and Rollback (for Ship)

* **Order of work is a safety property.** U2's classification test must be green
  before U3, U4, or U5 wrap any real call site.
* **Inject failures at the existing seams** (`persistArtifactWriteFn`,
  `appendCommentFn`, `mkdirDirSyncFn`) with path-selective mocks; a fail-all mock
  breaks the "directory was created" assertion.
* **Assert non-compensation positively.** The indeterminate case needs an
  assertion that the original write is still present, not merely that no error
  was returned.
* **Assert the precedence invariant directly** with a mixed joined error set.
* **Keep `ClaimShipment` tests untouched** as the regression witness.
* **No `t.Parallel()`** in any package overriding a package-global write seam.
* **Rollback trigger:** any observed loss of an applied change, or any
  compensation on an indeterminate write, in the first validation window →
  revert immediately.
* **Validation window:** one full shipment cycle plus one `doctor` run over the
  live corpus, owned by the operator.

### Unresolved Operator Decisions

* Whether `doctor` later gains automatic repair. Deferred; detection ships first.
* Whether the envelope later covers the archive path. Deferred; archival already
  has git-aware rollback machinery.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (Constitution Reviewer, Scope Boundary
  Auditor, Architecture Strategist, Go Reviewer, Agent-Native Parity Reviewer,
  Learnings Researcher — cross-model).
* **Cycle 1 decision: FAIL.** P1: `MutationEnvelope` took `*events.EventWriter`,
  coupling a generic primitive to a domain type (Architecture); the error model
  was ambiguous between sentinel and typed struct so callers could not extract
  the completed-step payload (F5-01); `errors.Join` interaction with the
  two-class contract left the classification ambiguous (F5-02); detection covered
  only commit associations while the envelope covered a second governed path
  (scope P1); `ErrMutationPartial` had no MCP mapping, so agents could not
  distinguish retryable from non-retryable outcomes (parity F5-1). P2: the doctor
  check was CLI-only, leaving agents without a recovery-discovery path (parity
  F5-2). P3: U1 coupled tests to `atomicfile` implementation details in a unit
  that does not change `atomicfile` (scope).
* **Resolutions:** the envelope signature now takes no domain types and callers
  capture the writer in their own closures; `MutationPartialError` declared as a
  typed struct in `internal/errors/mutation_errors.go` with `Error`/`Unwrap` and
  `errors.As` extraction; a named classification-precedence invariant added and
  asserted; U4 split into U4 (create + dependency) and U5 (shipment membership)
  to restore width isolation; detection extended to both governed paths and
  exposed through MCP as well as CLI; a machine-readable `mutation_partial`
  response added; the `atomicfile` reconciliation moved from test assertions to
  documentation; A1/A2/A3 upgraded to `approval_required: yes`.

### Cycle 2 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups.
