---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash FDEDE39A: exact-target administrative checkpoint disposition commands (abandon and quarantine) on both CLI and MCP, atomic and fail-closed audited, byte-preserving for malformed checkpoints, with listing made read-only, consuming the F6 governed-operation contract and the F5 mutation envelope.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md
title: 'Administrative checkpoint disposition — abandon and quarantine'
---

# Administrative checkpoint disposition — abandon and quarantine

Source deliberation:
`docs/decisions/2026-08-07-checkpoint-administrative-disposition-deliberation.md`.
Origin: stash `FDEDE39A` (high, `feature`).

This release unit ships **after** `120-S` and **before** `121-S`.

<!-- plan-review-attempt: 3 -->

## Problem Frame

The checkpoint surface offers `create`, `list`, `get`, `resolve`, and `cleanup`
(`internal/cli/checkpoint.go`, `internal/events/checkpoint_lifecycle.go`). An
operator who decides a valid checkpoint will never be resumed has no correct verb:
`resolve` asserts something untrue and records no reason, and `cleanup` is a broad
retention sweep over every `checkpoint-*.json` that cannot be aimed at one file
(`checkpoint_lifecycle.go:182-250`).

A **malformed** checkpoint is worse off. `GetCheckpoint` errors on it
(`:111-140`); `ListCheckpoints` **moves unparseable files as a side effect of
listing** and flags `Quarantined: true` (`:31-50`); and `CleanupCheckpoints`
**explicitly skips invalid files**. So a malformed checkpoint is unreachable
through every supported command, while a read-shaped command silently mutates the
filesystem.

`CheckpointV1.Status` is validated `oneof=active resolved`
(`internal/events/checkpoint_schema.go:17-30`), and **no checkpoint operation
emits an audit event today**.

### Success Criteria

* One named checkpoint file is dispositioned without touching any other file, and
  a test asserts the mutated set equals **exactly** the named target.
* A malformed checkpoint's bytes are **byte-identical** after quarantine.
* Every disposition records `disposition`, `disposition_reason`,
  `disposition_operator`, `disposition_at`, and `disposition_source` as exact
  labeled fields validated against closed allowlists.
* A failed audit append **refuses** the disposition; nothing is moved or rewritten.
* `checkpoint list` performs **no filesystem mutation** and cannot fail because of
  a disposition or audit failure.
* Neither verb consults retention or age policy.
* CLI and MCP produce identical observable state and are covered by the F6
  behavioral parity assertion.

### Scope Boundaries

**In scope:** the disposition metadata contract; the `abandoned` checkpoint
status; core `AbandonCheckpoint` and `QuarantineCheckpoint`; making
`ListCheckpoints` read-only; basename-only path confinement and clobber-refusal;
fail-closed audit append; CLI subcommands; MCP tools and handlers; registry rows;
documentation.

**Out of scope:** the checkpoint JSON HTML-escaping defect (stash `B5D7E401`,
**explicitly excluded from this dark scope**). No unit may touch `SetEscapeHTML`.
Any redesign of retention cleanup. A `checkpoint delete` verb. Checkpoint recovery
or resume-flow changes. Hook checkpoints under `.backlogit/runtime/hooks/`.
Re-implementing the F6 governed-operation contract or the F5 `MutationEnvelope` —
both are **consumed**.

### Consumed foundations (blocking dependencies, not to be duplicated)

| Foundation | Shipment | What this unit consumes |
|---|---|---|
| F6 governed-op parity | `119-S` (via `120-S`) | one shared core function per verb, both surfaces routed through it, `governed: true` registry marker, coverage by the existing behavioral parity assertion |
| F5 mutation envelope | `120-S` | ordered idempotent steps with compensation, gated on the two-class durable-write contract |

If either foundation is absent when Ship reaches this unit, **halt** — do not
reimplement.

### Package placement (settled — import-cycle safe)

Verified at HEAD: `internal/core` imports `internal/events` in 13 files;
`internal/events` imports `internal/core` in **zero**. The confinement helpers
(`internal/core/doctor_target.go`) and the audit-append shape
(`internal/core/gate_evidence.go`) both live in `internal/core`.

Therefore:

* **Types and schema stay in `internal/events`** — `CheckpointV1`, the disposition
  fields, the sidecar record type, and the allowlists.
* **All operational logic lives in `internal/core`** — the two verbs, target
  confinement, and the audit append.

Placing the verbs in `internal/events` would require `events` → `core` and create
a cycle. This is settled here so no implementer re-derives it.

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| Exact-target, filename-scoped | Stash `FDEDE39A`; P-015 scope-assertion precedent | U1, U4, U5 |
| Preserve original evidence | Stash; archive-only precedent | U1, U7 |
| Record operator and reason as labeled fields | Stash; machine-readable governance-field contract | U2, U3 |
| Operator provenance is explicit, never the tool | Review cycle 2, finding F3 | U10, U11 |
| Quarantine is malformed-only; abandon is valid-only | Stash text; review cycle 2, finding F2 | U6, U7 |
| Auditable, audit ordered before the move | Stash; two documented silent-audit-loss incidents | U8 |
| Audit failure classes respect the two-class contract | Review cycle 2, finding F4 | U8 |
| Atomic | Stash; two-class durable-write contract | U6, U7 |
| Avoid broad retention cleanup | Stash | scope boundary |
| Both CLI and MCP, with real parity fixtures | Stash; review cycle 2, finding F5 | U10, U11, U12 |
| Never round-trip a malformed record through the typed codec | `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | U7 |
| Clobber-refuse before a computed rename | `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md` | U7 |
| Listing must not mutate or become fallible | Review cycle 1, finding F4 | U9 |
| Naming boundary vs the artifact `abandoned` status | `docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md` | U2, U13 |

## Implementation Units

### U1 — Compiling-but-failing harness: disposition invariants (tests)

The RED harness for the two defining invariants. To **compile and fail** rather
than error out, this unit also lands the minimal exported signatures the harness
calls — `AbandonCheckpoint` and `QuarantineCheckpoint` — each returning an
`ErrNotImplemented` sentinel. That is the standard harness-before-code shape; the
stubs are replaced, never left, by U6 and U7.

Every test runs in an isolated `t.TempDir()` workspace so the hash-snapshot
assertions cannot be perturbed by concurrent writes. No `t.Parallel()`.

Files: `internal/core/checkpoint_disposition_test.go`,
`internal/core/checkpoint_disposition.go` (stubs only).
Scenarios: (1) mutated set equals exactly the named target — hash every checkpoint
before and after and assert exactly one changed; (2) quarantined bytes are
byte-identical, compared by hash; (3) a disposition that fails its audit leaves
the target unmoved.
Posture: test-first (RED).

### U2 — Metadata contract, `abandoned` status, sidecar type, listing fields (code)

Extend `CheckpointV1` with `disposition`, `disposition_reason`,
`disposition_operator`, and `disposition_at`, and widen status validation from
`oneof=active resolved` to include `abandoned`
(`internal/events/checkpoint_schema.go:17-30`). `disposition` validates against a
**closed allowlist**; an unrecognized value fails closed. There is deliberately
**no** `disposition_source` field: after U9 removes the automatic producer, every
disposition is operator-sourced and the field would be a constant.

Define the sidecar record type and its **exact** filename convention:
`<checkpoint-filename>.disposition.json`, written beside the quarantined file. The
suffix follows the extension so existing `checkpoint-*.json` scanners ignore it.

This unit also owns the `CheckpointSummary` additions that U9 needs —
`NeedsQuarantine bool` and a `RemediationCommand` string — because the schema file
belongs to this unit and split ownership of one file across units causes drift.

Files: `internal/events/checkpoint_schema.go`,
`internal/events/checkpoint_disposition_types.go`.
Scenarios: each allowlist value round-trips; an unknown `disposition` fails closed;
an empty or whitespace-only reason fails validation; an empty operator fails
validation.
Posture: test-first.

### U3 — Typed errors for disposition (code)

Declare the sentinels in the established errors package so both surfaces and the
MCP error mapper share one source:

* `ErrCheckpointUseQuarantine` — `abandon` attempted on a malformed target
* `ErrCheckpointUseAbandon` — `quarantine` attempted on a valid target
* `ErrCheckpointTargetUnsafe` — confinement refusal
* `ErrCheckpointReasonRequired`, `ErrCheckpointOperatorRequired`
* `ErrCheckpointDestinationOccupied`
* `ErrCheckpointAuditNotApplied`, `ErrCheckpointAuditIndeterminate`

Files: `internal/errors/checkpoint_errors.go`.
Scenarios: each sentinel is distinguishable via `errors.Is` after wrapping with
`fmt.Errorf("context: %w", err)`.
Posture: test-first.

### U4 — Failing confinement and refusal matrix (tests)

Red table-driven matrix for target resolution and precondition refusals, split
from U1 to keep each harness unit inside the two-hour boundary.

Files: `internal/core/checkpoint_target_test.go`.
Scenarios: bare filename accepted; path separator, `..`, absolute, volume-qualified
or UNC, and symlink targets each rejected; `abandon` on a malformed target refused
naming `quarantine`; `quarantine` on a valid target refused naming `abandon`;
empty reason and empty operator refused.
Posture: test-first (RED).

### U5 — Basename-only target resolution (code, thin adapter)

`ResolveDispositionTarget` accepts a **basename only** and rejects every form in
U4's matrix before any read or write. It is a **thin adapter** over the existing
helpers (`internal/core/doctor_target.go:228-283` `confineToStorageRoot`,
`internal/config/loader.go:71-89` `ensureContainedRelPath`) and must not
reimplement containment logic.

Files: `internal/core/checkpoint_target.go`.
Scenarios: U4's matrix turns green.
Posture: test-first.

### U6 — `AbandonCheckpoint` core operation (code)

Requires a parseable, schema-valid target whose status is `active`. Transitions to
`abandoned` and stamps the labeled fields using the existing atomic writer
(`syncWriteFileAtomic`, the path `ResolveCheckpoint` already uses at
`checkpoint_lifecycle.go:142-177`). A malformed target is refused with
`ErrCheckpointUseQuarantine`. Abandoning an already-`abandoned` checkpoint is a
successful no-op that reports the existing reason and does **not** overwrite it.
Registered as ordered idempotent steps on the F5 `MutationEnvelope`, honoring the
two-class durable-write contract. `operator` is an explicit non-empty parameter.

Files: `internal/core/checkpoint_disposition.go`.
Scenarios: valid active target transitions and stamps fields; malformed target
refused with the naming sentinel; already-abandoned is an idempotent no-op
preserving the original reason; an indeterminate write is not compensated.
Posture: test-first.

### U7 — `QuarantineCheckpoint` core operation, malformed-only, byte-preserving (code)

**Classifies without rewriting.** Reads the target and attempts parse plus schema
validation **in memory**. A target that is valid is refused with
`ErrCheckpointUseAbandon`; only a parse- or schema-invalid target proceeds. The
bytes on disk are never re-serialized, so evidence preservation is unaffected by
classification.

On a malformed target: move the bytes verbatim to the quarantine destination under
`WorkspaceStorageRoot(ws.RootPath)/archive/checkpoints` — the workspace's
already-resolved storage root helper (`internal/core/workspace.go`), never the
literal string `.backlogit/archive/checkpoints` — so this destination
continues to resolve correctly for both `.backlog` and legacy `.backlogit`
workspaces once shipment 121 changes the default without requiring split
state or a follow-up fix here. Guard the move with a **clobber-refuse guard**
before the rename (`ErrCheckpointDestinationOccupied`), then write the sidecar
as an **idempotent upsert** so an F5 step replay converges rather than
duplicating. On a failed move after a partial step, rename back to the
canonical path and log all diagnostics.

Files: `internal/core/checkpoint_disposition.go`.
Scenarios: valid target refused naming `abandon`; malformed bytes byte-identical
at the destination; existing destination refuses rather than overwrites;
re-quarantine upserts the sidecar only; double fault renames back and logs.
Posture: test-first.

### U8 — Audit append, ordered before the move, class-specific (code)

Append a JSONL audit event carrying target filename, verb, reason, and operator,
following `appendItemEventWithActorErr` (`internal/core/gate_evidence.go:25-58`).

**The append happens before any move or rewrite**, mirroring the gate path's
append-before-commit ordering. Failure handling is class-specific and consistent
with the F5 two-class contract:

| Audit failure class | Response |
|---|---|
| `ErrWriteNotApplied` | Refuse. Nothing moved. Retryable. |
| `ErrWriteIndeterminate` | **Do not compensate the audit.** Refuse the disposition, leaving the file unmoved, and surface the indeterminate class with recovery guidance. A duplicate audit on retry is acceptable; a missing one is not. |

Because the audit precedes the move, the invariant "a failed audit means nothing
moved" holds in **both** classes without ever compensating an indeterminate write.

**Both surfaces pass a real `*events.EventWriter`** — MCP passes the server's
shared instance; the CLI constructs a per-invocation writer via the existing
workspace writer constructor. **No caller passes `nil`**, because a fail-closed
audit and a nil writer are contradictory. The core function never mints one.

Files: `internal/core/checkpoint_audit.go`.
Scenarios: a successful disposition emits exactly one audit event with all four
fields; a not-applied append refuses and the file is **asserted unmoved**; an
indeterminate append refuses, does not compensate, and reports the class; the
passed writer instance is the one used.
Posture: test-first.

### U9 — Make `ListCheckpoints` read-only (code)

`ListCheckpoints` no longer moves unparseable files as a side effect
(`checkpoint_lifecycle.go:31-50`). It **reports** them: the summary keeps its parse
error in `ValidationErr` and sets the `NeedsQuarantine` and `RemediationCommand`
fields defined in U2, so an agent or operator can act without guessing.

This removes the second, implicit quarantine mechanism entirely rather than
converging it, keeping a read-shaped command free of both filesystem mutation and
any audit failure mode. The source deliberation was amended to record this
supersession.

Files: `internal/events/checkpoint_lifecycle.go`.
Scenarios: an unparseable file is reported with the remediation command and is
**not moved**; listing succeeds when the audit path is unavailable; existing
listing assertions unrelated to the move still pass.
Posture: test-first.

### U10 — CLI subcommands (code)

Add `checkpoint abandon <FILE> --reason <REASON> [--operator <NAME>]` and
`checkpoint quarantine <FILE> --reason <REASON> [--operator <NAME>]`, routed
through the shared core functions and constructing a real `EventWriter`.
`--reason` is **required** and must be non-empty after trimming. Operator
provenance resolves in a fixed order: `--operator`, then `BACKLOGIT_OPERATOR`,
then the OS user; it is **never** defaulted to `backlogit`, because attributing an
operator decision to the tool would be a false audit record. If none resolves, the
command fails with `ErrCheckpointOperatorRequired`.

Help text carries the naming-boundary statement distinguishing
`checkpoint abandon` from the artifact `abandoned` lifecycle status.

Files: `internal/cli/checkpoint.go`.
Scenarios: happy path emits a readable JSON result; missing `--reason` errors;
whitespace-only `--reason` errors; unresolvable operator errors. (Mirrors the
`TestCheckpointCreate_WritesReadableCheckpoint` / `_MissingStateDump` /
`_InvalidSchema` triple, the only checkpoint precedent in the library.)
Posture: test-first.

### U11 — MCP tools, handlers, and structured errors (code)

Register `backlogit_abandon_checkpoint` and `backlogit_quarantine_checkpoint`,
routed through the same shared core functions. Because MCP carries **no
authenticated operator identity**, `operator` is a **required** tool parameter on
both tools — it is never inferred.

Map each U3 sentinel to a structured machine-readable error: stable code, the
offending input, a `retryable` flag that is **class-derived** (`true` for
not-applied, `false` for a validation or confinement refusal, and a distinct
`indeterminate` outcome carrying recovery guidance rather than a bare
`retryable: false`), and operator remediation.

Files: `internal/mcp/tools.go`, `internal/mcp/errors.go`.
Scenarios: both tools produce state identical to their CLI counterparts; a missing
`operator` parameter is rejected; `abandon` on a malformed target returns a
structured error naming `quarantine`; `quarantine` on a valid target names
`abandon`; a confinement refusal returns a distinct non-retryable code; an
indeterminate audit returns the indeterminate outcome, not a plain failure.
Posture: test-first.

### U12 — Registry rows, governed markers, and parity fixtures (config)

Add honest registry rows for both operations with resolvable `cli_command`
templates and mark both `governed: true`.

A `governed: true` marker alone cannot establish behavioral parity for operations
that need distinct valid and malformed checkpoint inputs. This unit therefore also
**registers checkpoint-specific fixtures** — one valid checkpoint and one
malformed checkpoint — with the existing F6 behavioral parity assertion through
its declared fixture-registration seam. It must **not** build a second parity
framework; if F6 exposes no fixture seam at claim time, halt (see blocking
dependencies).

Record the naming-boundary statement in the operation descriptions.

Files: `.autoharness/backlog-registry.yaml`.
Acceptance: the existing surface-parity test passes with both new rows present;
both operations appear in the governed set; the F6 behavioral assertion executes
against both a valid and a malformed fixture for both operations; neither row
fabricates a `cli_command`.
Posture: configuration.

### U13 — Document the disposition contract (docs)

Document both verbs, the malformed-only versus valid-only split and why it exists,
the exact metadata field names and allowlist, the sidecar filename convention, the
audit ordering and its two failure classes, the exact-target guarantee, operator
provenance on each surface, the read-only listing change **and its impact on
agents that previously relied on auto-quarantine**, and — explicitly — the naming
boundary against the artifact `abandoned` status. Record the residual exposure that
this repository dogfoods a separately pinned `C:\Tools\backlogit.exe`, so a merge
does not make these commands operative here.

Files: `docs/design-docs/checkpoint-administrative-disposition.md`,
`.github/instructions/backlogit.instructions.md`.
Posture: documentation.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──> U5 ──┬──> U6 ──┐
                                 └──> U7 ──┴──> U8 ──> U9 ──> U10 ──> U11 ──> U12 ──> U13
```

`U5` (confinement) is green before `U6` or `U7` touches any file on disk. `U6` and
`U7` may land in either order; both precede `U8`. `U13` last.

## Decisions and Rationale

* **Operational logic in `internal/core`, types in `internal/events`** — verified
  import direction makes any other placement a cycle.
* **Asymmetric verbs** — a valid checkpoint can be rewritten safely (`resolve`
  already does); a malformed one cannot, because a typed round-trip drops
  unmodeled bytes and destroys the evidence the operator asked to preserve.
* **Listing becomes read-only** — a read-shaped command must not mutate the
  filesystem, and it must not inherit a fail-closed audit failure mode. Removing
  the implicit mechanism is cleaner than converging two mechanisms.
* **No caller passes a nil `EventWriter`** — a nil writer and a fail-closed audit
  are contradictory; both surfaces pass a real instance.
* **Refuse rather than auto-redirect** — `abandon` on a malformed target names
  `quarantine` rather than silently doing it, keeping operator intents distinct.
* **Fail-closed audit** — two documented in-repo incidents trace to best-effort
  audit appends that returned success.
* **Basename-only argument** — eliminates the traversal class outright.
* **Not built on `cleanup`** — fusing retention policy with an operator decision
  means a future retention change silently alters disposition behavior, and
  `cleanup` already skips exactly the files `quarantine` exists to handle.
* **Both surfaces** — no more dangerous than the already-MCP-exposed
  `cleanup_checkpoints`, so the blast-radius rule is satisfied.
* **Abandon leaves the file in place** — moving it would re-couple disposition to
  retention.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Malformed evidence rewritten or lost | **high** | `quarantine` never parses or re-serializes; byte identity asserted by hash in U1 and U6 |
| Disposition expands beyond the named file | **high** | Basename-only argument, thin-adapter confinement, and a hash-snapshot assertion that exactly one file changed |
| Import cycle between `core` and `events` | **high** | Placement settled and justified against the verified import direction |
| CLI permanently refuses because of a nil writer | **high** | Both surfaces construct a real `EventWriter`; asserted in U7 |
| `checkpoint list` gains a new failure mode | **high** | U8 makes listing read-only; it neither moves nor audits |
| Audit lost while the mutation lands | **high** | Fail-closed append; refusal asserted to leave the file unmoved |
| Destination collision overwrites evidence | **high** | Clobber-refuse guard before any rename |
| Failed move strands evidence | high | Rename back on double fault and log all diagnostics |
| `abandoned` conflated with the artifact status | medium | Boundary stated in help text, docs, and registry |
| Sidecar shape guessed by the implementer | medium | Filename convention and record type fixed in U2 |
| F5/F6 machinery duplicated | medium | Declared blocking dependency; halt rather than reimplement |
| Collision with the excluded `B5D7E401` defect | medium | `SetEscapeHTML` named out of scope; no unit touches it |
| Commands look operative here before the pinned CLI updates | low | Residual exposure recorded in U12 and closure |
| Half-built tree if abandoned mid-way | medium | U1 lands stubs that fail loudly; the widened status enum has a writer as of U5 |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. Five sentinels in `internal/errors`; all errors wrapped with `%w`. |
| II. Test-First | U1 is a compiling-but-failing RED harness; every code unit is test-first. |
| **III. Workspace Isolation** | **Load-bearing.** Basename-only resolution, `Lstat` symlink rejection, realpath confinement, ordered before any disk access. |
| IV. CLI Containment | All writes stay inside the workspace storage root. |
| V. Structured Observability | A previously unaudited surface gains a fail-closed audit event and structured MCP errors. |
| VI. Single Responsibility | No new dependencies; consumes F5/F6; U4 is a thin adapter, not a reimplementation. |
| VII. Destructive Approval | Quarantine relocates evidence; classified `destructive` below with approval required. Archive-only — nothing is deleted. |
| VIII. Safety Modes | Full `ProposedAction` / `ActionRisk` / `ActionResult` table below. |
| IX. Git-Friendly Persistence | Sidecar is JSON beside the artifact; no new format family. |
| X. Context Efficiency | Exact-target operations; listing becomes strictly cheaper. |

No violations.

## Plan Hardening Signals

* mutating operation that relocates evidence — **yes**
* operator-supplied path input on a Principle III surface — **yes**
* new public CLI and MCP contract plus a schema/status enum change — **yes**
* behavior change to an existing command (`ListCheckpoints`) — **yes**

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** `checkpoint abandon` and `checkpoint quarantine` on
  both CLI and MCP; `checkpoint list` (now read-only); `checkpoint cleanup` (must
  be unaffected).
* **Scenarios:** valid abandon; abandon on malformed (refused); quarantine of a
  malformed file (bytes identical); re-quarantine; destination collision;
  separator / `..` / absolute / symlink targets; empty reason; audit-append
  failure; listing an unparseable file (reported, not moved).
* **Rollback:** plain revert. Nothing is deleted, so a reverted build still reads
  every file; a quarantined file is recoverable by moving it back and removing the
  sidecar.
* **Closure artifact:** must record the metadata contract, the asymmetry rationale,
  the read-only listing change, the naming boundary, and the pinned-CLI residual
  exposure.

## Plan Hardening

Hardening was required (four signals).

### Protected Invariants (must not regress)

1. A quarantined file's bytes are **byte-identical** to the original.
2. The mutated set equals **exactly** the named target.
3. The audit append happens **before** any move, so a failed audit — in either
   error class — means nothing moved. An indeterminate audit is **never**
   compensated.
4. Nothing is ever deleted — disposition is archive-only.
5. `cleanup` retention semantics are untouched; no disposition path consults age
   or retention policy.
6. `ListCheckpoints` performs **no** filesystem mutation and cannot fail because of
   audit or disposition.
7. `internal/events` never imports `internal/core`.
8. No caller passes a `nil` `EventWriter`; the core function never mints one.
9. `disposition_operator` is never defaulted to `backlogit`; an unresolvable
   operator is a refusal.
10. The two verbs stay disjoint: `abandon` is valid-only, `quarantine` is
    malformed-only, and each refuses by naming the other.
11. `SetEscapeHTML` is not touched anywhere — that is the excluded `B5D7E401`.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`
* `docs/decisions/2026-07-04-closure-docs-compaction-deliberation.md`
* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`
* `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
* `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md`
* `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
* `docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md`
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
* `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
* `.github/instructions/constitution.instructions.md` (III, IV, VII),
  `.github/instructions/strict-safety.instructions.md`,
  `.github/instructions/go.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Relocate a checkpoint file's bytes off its canonical path | `internal/core/checkpoint_disposition.go` | evidence relocation | **destructive** | Move the file back and remove the sidecar; nothing is deleted | **yes** |
| A2 | Widen the checkpoint status enum and add schema fields | `internal/events/checkpoint_schema.go` | schema/contract change | **high** | Plain revert; older files remain readable | **yes** |
| A3 | Accept an operator-supplied file target on a Principle III surface | `internal/core/checkpoint_target.go` | security surface | **high** | Plain revert | **yes** |
| A4 | Remove the implicit filesystem mutation from `ListCheckpoints` | `internal/events/checkpoint_lifecycle.go` | behavior change to an existing command | **high** | Plain revert | **yes** |
| A5 | Add two new CLI and MCP operations | `internal/cli/checkpoint.go`, `internal/mcp/tools.go` | new public contract | moderate | Plain revert | no |
`ActionResult` for every entry starts `planned`. **A1 is classified
`destructive`** rather than `high`: it removes operator evidence from its
canonical discoverable path, which is the most conservative reading, even though
the operation is archive-only and rename-back-guarded.

### Deepened Verification and Rollback (for Ship)

* **Order of work is a safety property.** U5 (confinement) must be green before U6
  or U7 touches any file on disk, and U8's audit ordering must be green before any
  disposition is exposed on a surface.
* **Negative-path first.** Land and observe failing tests for every refusal —
  `abandon` on malformed, `quarantine` on valid, separator, `..`, absolute,
  symlink, empty reason, unresolvable operator, destination collision, and both
  audit failure classes — before any happy path.
* **Byte-identity is asserted, not assumed.** Hash before and after quarantine.
* **Scope is asserted, not assumed.** Hash every checkpoint before the operation
  and assert exactly one changed.
* **Both audit failure classes are tested.** Not-applied must refuse and leave the
  file unmoved; indeterminate must refuse, **not** compensate, and report the
  class. Assert the file is unmoved in both.
* **Listing must be proven inert:** assert no file moved and no audit emitted
  during a list over a corpus containing an unparseable file.
* **Parity fixtures must actually execute.** Assert the F6 behavioral assertion
  runs against both a valid and a malformed checkpoint for both operations; a
  governed marker with no fixture is a vacuous pass.
* **Isolated fixtures.** Every disposition test runs in `t.TempDir()`; no
  `t.Parallel()` in any package overriding a package-global write seam.
* **Verify F5/F6 availability first.** If `MutationEnvelope`, the governed-op
  contract, or the F6 fixture-registration seam is absent at claim time, halt
  rather than reimplement.
* **Rollback trigger:** any checkpoint byte mutation during quarantine, any
  sibling file modified, any disposition landing without an audit event, or any
  listing failure in the first validation window → revert immediately.
* **Validation window:** one full disposition cycle across CLI and MCP plus one
  `checkpoint list` and one `checkpoint cleanup` run, owned by the operator.

### Unresolved Operator Decisions

* Whether `abandon` should eventually archive the file rather than leave it in
  place. Left in place to avoid re-coupling disposition to retention.
* Whether the sidecar is later projected into the SQLite index. Deferred — no
  consumer needs it and the index is a disposable projection.
* Whether a future maintenance verb should bulk-quarantine every reported
  `NeedsQuarantine` file. Deliberately not built; exact-target is the requirement.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** — Constitution Reviewer
  (`claude-opus-4.6`), Go Reviewer with an architecture lens
  (`gemini-3.1-pro-preview`), Scope Boundary Auditor with an agent-native parity
  lens (`gpt-5.6-sol`), plus the Learnings Researcher pass that fed the
  deliberation.

### Cycle 1 — decision: FAIL

* **P0** — placing the verbs, target confinement, and audit in `internal/events`
  while reusing `internal/core` helpers creates an import cycle. Verified at HEAD:
  `internal/core` imports `internal/events` in 13 files; `internal/events` imports
  `internal/core` in zero.
* **P0** — a fail-closed audit combined with "the CLI passes `nil`" for the
  `EventWriter` is self-contradictory and would make every CLI disposition fail.
* **P1** — routing the implicit auto-quarantine through the fail-closed audit would
  give `checkpoint list`, a read-shaped command, a new failure mode, unclassified.
* **P1** — the sidecar filename convention was unspecified.
* **P2** — sentinels had no declared home; U2 mixed production code with pinning
  tests; the re-quarantine sidecar write was not stated as an idempotent upsert;
  hash-snapshot tests were not required to run in an isolated fixture; `A1` was
  rated `high` where `destructive` is the conservative reading.
* **P3** — U1's compile-and-fail mechanism was unexplained; U4's thin-adapter
  intent was implicit; the config unit lacked a named acceptance criterion.

**Resolutions:** package placement settled against the verified import direction
and recorded as a protected invariant; both surfaces construct a real
`EventWriter`; the implicit auto-quarantine **removed** rather than converged, with
`ListCheckpoints` made strictly read-only and the change classified as risky action
A4; sidecar filename fixed to `<checkpoint-filename>.disposition.json`; sentinels
given their own unit; pinning tests moved out of the code unit; sidecar write
specified as an idempotent upsert; `t.TempDir()` required; `A1` upgraded to
`destructive`; stub-based compile-and-fail made explicit; thin-adapter intent and
config acceptance criteria stated.

### Cycle 2 — decision: FAIL

* **P1** — the deliberation still recorded the superseded "converge the implicit
  auto-quarantine" decision while the plan removed it; the authoritative artifacts
  disagreed.
* **P1** — `quarantine` accepted **every** existing checkpoint, expanding the
  stash's malformed-only verb and overlapping `abandon`.
* **P1** — `disposition_operator` provenance was undefined on both surfaces, and
  MCP has no authenticated operator identity.
* **P1** — "a failed audit means unmoved" conflicted with the F5 rule that an
  indeterminate write must not be compensated.
* **P1** — a `governed: true` marker alone cannot establish behavioral parity for
  operations needing valid and malformed fixtures.
* **P2** — `disposition_source` became YAGNI once the automatic producer was
  removed; U8's listing fields lived in a schema file owned by another unit; U1
  exceeded the two-hour boundary.

**Resolutions:** the deliberation was **amended** with an explicit supersession
record plus malformed-only and operator-provenance clarifications; `quarantine`
now classifies in memory and refuses a valid target with `ErrCheckpointUseAbandon`
while never rewriting bytes; operator provenance fixed — CLI resolves
`--operator` → `BACKLOGIT_OPERATOR` → OS user and never defaults to `backlogit`,
MCP requires `operator` as a tool parameter; the audit was reordered to run
**before** the move so "failed audit ⇒ nothing moved" holds in both error classes
without ever compensating an indeterminate write, with class-derived MCP
retryability; U12 now registers valid and malformed parity fixtures with the
existing F6 assertion and halts rather than building a second framework;
`disposition_source` removed; the `CheckpointSummary` listing fields moved into the
schema-owning unit; U1 split into U1 (disposition invariants) and U4 (confinement
and refusal matrix). Unit count 12 → 13.

### Cycle 3 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups below.
* Cycle count: 2 remediation cycles used, within the 3-attempt circuit breaker.
* Accepted advisory follow-ups (not blocking): whether `abandon` should later
  archive rather than leave in place; whether the sidecar is projected into the
  index; whether a bulk maintenance verb is ever added for reported
  `NeedsQuarantine` files.
