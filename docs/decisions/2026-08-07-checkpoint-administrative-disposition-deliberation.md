---
title: "Administrative checkpoint disposition: abandon and quarantine"
description: "Deliberation on stash FDEDE39A: how to represent exact-target administrative disposition of checkpoints so a valid-but-unwanted checkpoint and a malformed checkpoint are both retired atomically, auditably, and without destroying the original evidence."
source: docs/decisions/2026-08-07-checkpoint-administrative-disposition-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Exact-target checkpoint disposition commands (checkpoint abandon / checkpoint quarantine) across CLI and MCP, without reusing broad retention-cleanup semantics"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md"
tags:
  - "feature"
  - "checkpoints"
  - "cli"
  - "mcp"
  - "reliability"
  - "stage"
stash_ids:
  - "FDEDE39A"
---

## Problem Frame

Stash `FDEDE39A` (high priority, kind `feature`), captured verbatim:

> Add exact-target administrative checkpoint disposition:
> `checkpoint abandon FILE --reason REASON` for valid checkpoints intentionally
> not resumed, and `checkpoint quarantine FILE --reason REASON` for malformed
> checkpoints that cannot pass normal validation. Both CLI and MCP operations
> must be atomic, auditable, filename-scoped, preserve original evidence, record
> operator and reason metadata, and avoid broad retention cleanup.

Today the checkpoint surface offers `create`, `list`, `get`, `resolve`, and
`cleanup` (`internal/cli/checkpoint.go`, `internal/events/checkpoint_lifecycle.go`).
An operator who decides a valid checkpoint will never be resumed has only two
options, and both are wrong:

* `resolve` — semantically false. It means "this was picked up and dealt with",
  which is not what happened, and it leaves no reason.
* `cleanup` — a **broad retention sweep** over every `checkpoint-*.json`
  (`checkpoint_lifecycle.go:182-250`). It cannot be aimed at one file, and it
  archives on an age/status policy rather than an operator decision.

For a **malformed** checkpoint the situation is worse. `GetCheckpoint` errors on
it; `ListCheckpoints` silently auto-moves unparseable files and flags
`Quarantined: true` on the summary (`checkpoint_lifecycle.go:31-50`); and
`CleanupCheckpoints` explicitly **skips invalid files**, so a malformed
checkpoint can sit forever, un-actionable through any supported command.

### What must be decided

1. **Where disposition metadata lives** for a valid checkpoint versus a malformed
   one — the malformed file, by definition, cannot be round-tripped through the
   typed codec.
2. **How the new explicit quarantine composes with the existing implicit
   auto-quarantine** in `ListCheckpoints`, so the repository does not end up with
   two quarantine mechanisms.
3. **Whether both verbs belong on both surfaces**, given the standing rule that a
   fallback surface must never be more dangerous than the surface it mirrors.
4. **How the audit record is written**, given that two separate in-repo incidents
   trace to best-effort audit appends.

### Constraints

* **Operator requirements (non-negotiable):** atomic, auditable, filename-scoped,
  preserve original evidence, record operator and reason, avoid broad retention
  cleanup semantics.
* **Constitution III / IV** — the `FILE` argument is operator-supplied input that
  must resolve inside the workspace storage root; traversal must be rejected.
* **Constitution VII** — disposition is a mutating, evidence-moving operation.
* **Sequencing (operator-declared):** this release unit ships **after** `120-S`,
  so it must **consume** the F6 governed-operation contract and the F5 idempotent
  mutation envelope rather than duplicate them.

### Success criteria

* One named checkpoint file can be dispositioned without touching any other file.
* The original bytes of a malformed checkpoint survive disposition unmodified.
* Every disposition records operator and reason as **exact labeled fields**.
* A failed audit append **refuses** the disposition.
* Neither verb reuses retention/age policy.
* CLI and MCP produce identical observable state.

### Explicitly out of scope

Fixing checkpoint JSON HTML-escaping (`\u003e` / `\u003c` / `\u0026`) — that is
stash `B5D7E401`, deliberately excluded from this dark scope by the operator. Any
broad retention-cleanup redesign. Checkpoint recovery or resume-flow changes. A
`checkpoint delete` verb of any kind. Hook checkpoints under
`.backlogit/runtime/hooks/`, which are a separate per-consumer mechanism.

## Research Findings

### Substrate facts (HEAD, verified this session)

| Fact | Evidence |
|---|---|
| `CheckpointV1.Status` is validated `oneof=active resolved` | `internal/events/checkpoint_schema.go:17-30` |
| `CheckpointSummary` already carries `ValidationErr` and `Quarantined bool` | `internal/events/checkpoint_schema.go:56-72` |
| `ListCheckpoints` auto-quarantines unparseable files as a side effect of listing | `internal/events/checkpoint_lifecycle.go:31-50` |
| `GetCheckpoint` errors on a malformed or invalid checkpoint | `internal/events/checkpoint_lifecycle.go:111-140` |
| `ResolveCheckpoint` rewrites the file via `json.Marshal` + `syncWriteFileAtomic` | `internal/events/checkpoint_lifecycle.go:142-177` |
| `CleanupCheckpoints` sweeps all `checkpoint-*.json`, archives by age/status, **skips invalid files**, moves rather than deletes | `internal/events/checkpoint_lifecycle.go:182-250` |
| Archive destination is `.backlogit/archive/checkpoints` | `internal/events/checkpoint_lifecycle.go:182-250` |
| There is **no** audit event for any checkpoint operation today | repository-wide search, this session |
| Path-confinement helpers exist and are reusable | `internal/core/doctor_target.go:131-176`, `:228-283`; `internal/config/loader.go:71-89` |
| Registry rows exist for all five current checkpoint operations | `.autoharness/backlog-registry.yaml:117-143`, `:448-452` |

### Prior learnings applied

Confidence was **high** for every sub-area except checkpoint-specific prior art,
where the researcher recorded an explicit gap: *"There is NO compound learning and
NO decision record dedicated to checkpoint lifecycle, retention/cleanup,
quarantine, or validation errors."* The precedents below are therefore **analogs**
and are cited as such.

* **Never compute a mutation set by traversal; assert the scope** —
  `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`.
  The 114-S incident is the in-repo proof that unconditional set expansion
  corrupts the store. "Do not rely on 'the caller passed one filename' as the
  scope guarantee; assert it."
* **Archive-only, never delete** —
  `docs/decisions/2026-07-04-closure-docs-compaction-deliberation.md` states the
  rule three times, and every disposition precedent in the library (compaction,
  stash retirement, artifact archival) moves rather than unlinks.
* **Never round-trip a malformed record through the typed codec** —
  `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`. The
  typed writer enumerates recognized keys and silently drops everything else.
  Preserve bytes; record metadata out-of-band.
* **Machine-consumed governance fields must be exact labeled fields at the
  producer** — `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`.
  "Producers own the format; consumers own the validation."
* **Never make an audit append best-effort** — two documented incidents:
  `LinkCommit`'s `commit_tracked` and harvest's `appendToStashArchive` both
  warn-and-continue and return success, producing silent audit loss.
* **Thread the caller's `*events.EventWriter`** —
  `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`.
* **Clobber-refuse before renaming to a computed destination** —
  `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md`.
* **Rename back and log all diagnostics on double fault** —
  `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`.
* **Gate a mutating repair verb** — both in-repo repair verbs
  (`--fix-archived-from`, `--fix-malformed`) require an explicit opt-in and each
  has a named test enforcing it.
* **Naming collision warning** —
  `docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md`:
  `abandoned` is **already** a validated *artifact* lifecycle status defined in
  two places. `checkpoint abandon` acts on a different object and the boundary
  must be stated explicitly or reviewers will conflate them.

### The fact that shapes the whole decision

A **valid** checkpoint can be rewritten safely — `ResolveCheckpoint` already does
exactly that. A **malformed** checkpoint cannot: it may not parse at all, and even
when it does, the typed writer would drop the very bytes that make it evidence.
Any design that treats the two verbs symmetrically is either impossible for the
malformed case or destroys the evidence the operator asked to preserve.

## Options Evaluated

### Option A — Symmetric in-file metadata for both verbs

Both verbs write `status`, `disposition_reason`, and `disposition_operator` into
the checkpoint file.

* **Pros:** one code path; one place to look; mirrors `resolve` exactly.
* **Cons:** **impossible for the malformed case** — the file may not parse, and a
  typed round-trip would drop unmodeled bytes, destroying the evidence. Directly
  violates the operator's "preserve original evidence" requirement.
* **Effort:** low. **Fit:** none.

### Option B — Asymmetric: in-file transition for `abandon`, byte-preserving move plus sidecar for `quarantine`

`abandon` requires a parseable, schema-valid checkpoint and performs a status
transition to `abandoned` with labeled reason and operator fields, exactly as
`resolve` does today. `quarantine` requires only that the file exists, moves its
bytes **verbatim** to the quarantine destination, and writes disposition metadata
to a **sidecar** record beside it. Both are exact-target, both are audited, both
refuse on audit failure.

* **Pros:** each verb matches the actual state of its input; evidence is preserved
  byte-for-byte where preservation matters; reuses the existing `resolve`
  precedent for the valid case; the sidecar is the only shape that can carry
  metadata for an unparseable file; converges the existing implicit
  auto-quarantine onto one destination and one metadata format.
* **Cons:** two code paths and two metadata locations; the asymmetry must be
  documented or it looks like an inconsistency.
* **Effort:** medium. **Fit:** high.

### Option C — Sidecar metadata for both verbs

Neither verb touches the checkpoint file; both write a sidecar.

* **Pros:** perfectly uniform; never rewrites any checkpoint.
* **Cons:** creates **two sources of truth** for a valid checkpoint's status — the
  file says `active` while a sidecar says abandoned. Every reader
  (`ListCheckpoints`, `GetCheckpoint`, the Stage recovery state machine) would
  have to learn to consult the sidecar or silently report stale state. It also
  diverges from `resolve`, which already writes status in-file, so the checkpoint
  surface would carry two contradictory conventions.
* **Effort:** medium. **Fit:** medium — the closest rejected alternative.

### Option D — Extend `cleanup` with a `--file` flag

Add exact-target selection to the existing retention sweep.

* **Pros:** no new verbs; smallest surface change.
* **Cons:** directly contradicts the operator requirement to *avoid broad
  retention cleanup*. It fuses an age/status **policy** with an operator
  **decision** in one command, so a future retention change silently alters
  disposition behavior. `cleanup` also skips invalid files, which is precisely the
  case `quarantine` exists to handle.
* **Effort:** low. **Fit:** low.

## Trade-off Comparison

| Criterion | A (symmetric in-file) | B (asymmetric) | C (sidecar both) | D (extend cleanup) |
|---|---|---|---|---|
| Works for a malformed file | **no** | **yes** | yes | **no** (skipped today) |
| Preserves original bytes | **no** | **yes** | yes | n/a |
| Single source of truth for status | yes | **yes** | **no** | yes |
| Consistent with existing `resolve` | yes | **yes** | no | no |
| Avoids retention-policy coupling | yes | **yes** | yes | **no** |
| Converges the existing auto-quarantine | no | **yes** | partial | no |
| Exact-target by construction | yes | **yes** | yes | bolted on |
| Code paths | 1 | 2 | 1 | 1 |
| Meets every stated operator requirement | no | **yes** | no | no |

## Decision

**Adopt Option B — asymmetric disposition, with one shared core seam, one
destination, and one metadata contract.**

### `checkpoint abandon <FILE> --reason <REASON>`

* **Precondition:** the target parses and passes schema validation, and its status
  is `active`. A malformed target is **refused** with a typed error that names
  `quarantine` as the correct verb. Refusing rather than silently redirecting
  keeps the two operator intents distinct.
* **Effect:** status transition `active` → `abandoned`, plus the labeled fields
  `disposition_reason`, `disposition_operator`, and `disposition_at`. Written with
  the existing atomic writer.
* **Idempotency:** abandoning an already-`abandoned` checkpoint is a successful
  no-op that reports the existing reason rather than overwriting it.

### `checkpoint quarantine <FILE> --reason <REASON>`

* **Precondition:** the target exists inside the storage root **and fails parse or
  schema validation**. A valid target is refused with a typed error naming
  `abandon`, mirroring `abandon`'s refusal of a malformed target. Classification is
  performed in memory; the bytes on disk are never rewritten.
* **Effect:** the file's bytes are moved **verbatim** to the quarantine
  destination. Nothing is parsed-and-rewritten, re-serialized, or normalized.
  Disposition metadata is written to a **sidecar** record beside the quarantined
  file.
* **Idempotency:** quarantining an already-quarantined file updates the sidecar
  reason and operator as an idempotent upsert rather than erroring or moving twice.

### Convergence with the existing implicit auto-quarantine

**Superseded by plan review — see the amendment below.**

`ListCheckpoints` already moves unparseable files as a side effect. This
deliberation originally proposed routing that path through the same core function
with `operator: "backlogit"` and a reserved automatic reason value.

### Amendment (2026-08-07, after plan-review cycle 1)

The convergence above was **rejected during plan review** and is superseded. Plan
review established that routing the implicit auto-quarantine through the
fail-closed audit would give `checkpoint list` — a read-shaped command — a new
failure mode, and would leave a filesystem mutation inside a read path.

**Superseding decision: remove the implicit auto-quarantine entirely.**
`ListCheckpoints` becomes strictly read-only. It reports an unparseable file with
its parse error plus a `NeedsQuarantine` signal and the exact remediation command,
and moves nothing. Disposition happens **only** through the explicit,
operator-invoked verb.

Consequences of the amendment, all reflected in the plan:

* There is no automatic disposition producer, so `disposition_source` and its
  `auto` value are removed from the metadata contract as unused.
* `disposition_operator` is always a real operator, never the tool.
* The "two quarantine mechanisms" problem is eliminated rather than converged.
* The change to `ListCheckpoints` is a behavior change to an existing command and
  is carried as a risky action requiring approval.

### Quarantine is malformed-only (clarified after plan-review cycle 2)

`quarantine` accepts **only** a target that fails parse or schema validation. A
valid target is refused with a typed error naming `abandon`, mirroring `abandon`'s
refusal of a malformed target. Classification is performed by reading and
validating **in memory**; the bytes on disk are never rewritten, so the
evidence-preservation guarantee is unaffected. This keeps the two verbs disjoint
and matches the stash text, which scopes `quarantine` to "malformed checkpoints
that cannot pass normal validation".

### Operator provenance (clarified after plan-review cycle 2)

`disposition_operator` is an explicit input, never inferred and never defaulted to
`backlogit` — attributing an operator decision to the tool would be a false audit
record. The CLI resolves it from an explicit `--operator` flag, falling back to
`BACKLOGIT_OPERATOR` and then the OS user. MCP has no authenticated identity, so
`operator` is a **required** tool parameter there. The shared core function takes
it as an explicit non-empty parameter.

### Metadata contract (exact labeled fields, fixed here)

`disposition` (`abandoned` | `quarantined`), `disposition_reason` (required,
non-empty, trimmed), `disposition_operator` (required, explicit — see operator
provenance above), `disposition_at` (UTC), and `original_filename`. Consumers
validate `disposition` against a **closed allowlist**; an unrecognized or missing
value fails closed rather than defaulting.

`disposition_source` was in the original draft and is **removed** by the
amendment: with no automatic producer, every disposition is operator-sourced and
the field would be a constant.

### Audit contract

Every disposition appends a JSONL audit event carrying the target filename, verb,
reason, and operator. The append happens **before** any move or rewrite, mirroring
the append-before-commit ordering the gate path already uses. Consequently:

* audit failure classified `ErrWriteNotApplied` → refuse; nothing moved;
* audit failure classified `ErrWriteIndeterminate` → **do not compensate the
  audit**; refuse the disposition, leaving the file unmoved, and surface the
  indeterminate class with recovery guidance. A duplicate audit event on a later
  retry is acceptable; a missing one is not.

Either way the invariant "a failed audit means nothing moved" holds, because the
audit precedes the move. This deliberately departs from `LinkCommit`'s
best-effort append, which the library records as a source of silent audit loss.

### Scope guarantee

Filename-scoped by construction: the argument is a **basename only**. Path
separators, `..`, absolute paths, and symlinked targets are rejected before any
work begins, reusing the existing confinement helpers. A test asserts the mutated
set equals **exactly** the named target — the caller's argument is not accepted as
the guarantee.

### Surfaces

Both verbs exist on **both** CLI and MCP, routed through one shared core function
per verb. This satisfies the blast-radius rule: the operations are no more
dangerous than the already-MCP-exposed `cleanup_checkpoints`, which archives files
in bulk today. Unlike `--force-gates`, there is no human-at-a-terminal
justification for a CLI-only asymmetry here, and the operator explicitly requires
both.

### Consumption of the F5 and F6 foundations (not duplication)

This unit ships after `120-S`. It therefore **consumes**:

* the **F6** governed-operation contract — one shared core function per verb, both
  surfaces routed through it, registered `governed: true`, and covered by the
  behavioral parity assertion rather than a bespoke parity test; and
* the **F5** `MutationEnvelope` — the move-plus-sidecar-plus-audit sequence is
  registered as ordered idempotent steps with compensation, gated on the two-class
  durable-write contract, instead of hand-rolling rollback.

If either foundation is unavailable when Ship reaches this unit, that is a
blocking dependency, not a licence to reimplement.

### Naming boundary (must be stated in help text, docs, and the registry)

> `checkpoint abandon` disposes of a **session checkpoint file**. It is unrelated
> to the `abandoned` **artifact lifecycle status** used for work items. The two
> operate on different objects and neither implies the other.

## Rejected Alternatives

* **Option A** — cannot work for the malformed case and destroys evidence.
* **Option C** — creates two sources of truth for a valid checkpoint's status and
  contradicts the existing `resolve` convention. Closest rejected alternative.
* **Option D** — fuses retention policy with operator decision, and `cleanup`
  already skips exactly the files `quarantine` must handle.
* **A `checkpoint delete` verb** — every disposition precedent in this repository
  is archive-only. Not proposed and not accepted.
* **Removing the implicit auto-quarantine** — it is load-bearing for
  `ListCheckpoints` resilience. It is converged onto the shared path instead.

## Unresolved Questions

* Whether `abandon` should also move the file to the archive directory or leave it
  in place with an `abandoned` status. The plan leaves it **in place**, because
  `cleanup` already archives resolved and stale files on its own policy and moving
  here would re-couple disposition to retention. Revisit only if operators report
  clutter.
* Whether the sidecar should later be projected into the SQLite index. Deferred —
  no consumer needs it yet, and the index is a disposable projection.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| A malformed file is rewritten and its evidence lost | `quarantine` never parses or re-serializes; a byte-identity assertion is a protected invariant |
| Disposition silently expands beyond the named file | Basename-only argument, confinement helpers, and a test asserting the mutated set equals exactly the target |
| Audit record lost while the mutation lands | Fail-closed append; the disposition refuses if the audit does not land |
| Two quarantine mechanisms drift apart | The implicit path is routed through the same core function, destination, and sidecar format |
| `abandoned` conflated with the artifact lifecycle status | Explicit boundary statement in help text, docs, and the registry; the enum pair is pinned by a test |
| Reason stored as unvalidated prose | Required, trimmed, non-empty, and stored as an exact labeled field validated against a closed allowlist for `disposition` and `disposition_source` |
| Destination collision overwrites an existing quarantined file | Clobber-refuse guard before any rename |
| Failed move leaves evidence at an unscannable path | Rename back on double fault and log all diagnostics |
| Duplicating F5/F6 machinery | Consumption is a declared dependency; the shipment is ordered after `120-S` |
| New commands appear usable in this workspace before the pinned CLI is updated | Recorded as residual exposure; the repository dogfoods a separately pinned `C:\Tools\backlogit.exe` |
