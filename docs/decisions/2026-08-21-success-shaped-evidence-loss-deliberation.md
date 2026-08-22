---
title: "Deliberation: eliminate success-shaped evidence loss on governed diagnostic paths"
description: "Covering-feature scope decision for external source entries 3C7AAC71 (checkpoint context key drop) and 90F2A9F8 (docs lint whole-corpus abort), with the already-fixed disposition for 84D8E6AB"
source: "autoharness .backlogit/stash.jsonl entries 3C7AAC71, 90F2A9F8, 84D8E6AB"
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Status

Decided. Depth: `standard`. Promotion: `plan`.

Operator pre-selected three external source entries and authorized the full Stage pipeline. This
record captures the grouping validation, the per-entry disposition, and the design decision that the
implementation plan is built on.

## Provenance

All three source entries are read-only records in a different repository
(`C:\Source\GitHub\autoharness`, `.backlogit\stash.jsonl`). They were read but never mutated, and no
backlogit command was run against that workspace. External source IDs are carried verbatim into this
record and into every downstream backlog artifact.

| Source ID | Priority | Kind | Subject | Disposition |
|---|---|---|---|---|
| `3C7AAC71` | medium | bug | Checkpoint creation silently discards all context keys except the four modeled ones | In scope |
| `90F2A9F8` | medium | bug | `backlogit docs lint` hard-aborts the whole scan on the first frontmatter decode error | In scope |
| `84D8E6AB` | low | bug | Shipment audit-log completeness (missing intermediate `shipped` event) | Already fixed; not duplicated |

## Disposition for 84D8E6AB (already fixed)

`84D8E6AB` was already harvested into this repository and shipped. It must not be staged again.
Evidence gathered this session:

* The autoharness entry itself closes with `[DEFERRED - external Backlogit work tracked as 0115F71F]`.
* Local stash `0115F71F` produced deliberation `059-DL` and feature `143-F`
  (`Shipment shipped-event audit-log durability and doctor reconciliation`), archived `done` at commit
  `817d46794342f0747f381e4d42e899d75d01c3cf` under shipment `127-S`. `143-F` frontmatter records
  `source_stash_id: 0115F71F` and the original `84D8E6AB` text verbatim.
* The correlated torn-log case (archived emitted although the file move did not durably complete) is
  covered by the `shipped_unarchived_residue` doctor finding and the fail-closed shipped transition
  (`143.003-T`, `143.004-T`, `143.007-T`).
* The residual prevention gap that `143-F` explicitly deferred (non-`ShipShipment` producers, stash
  `47B48DB0`) shipped as feature `144-F` / shipment `128-S`, archived.
* Code confirms the surfaces exist today: `internal/core/doctor.go` defines
  `FindingMissingShippedEvent` and `FindingShippedUnarchivedResidue`; `internal/core/shipment.go:243`
  appends `shipment_status_changed` through the error-returning shipment envelope; CLI and MCP
  renderings are pinned by `internal/cli/doctor_test.go` and `internal/mcp/doctor_shipped_event_test.go`.
* Append-only audit integrity was preserved throughout: the shipped fix is report-only for historical
  residue and never synthesizes events. Nothing in this session synthesizes one either.

No new artifact is created for `84D8E6AB`. Nothing is written back to autoharness.

## Grouping validation

The operator proposed one reliability feature covering all three entries. Validation outcome:
**accept the theme, drop one member.** The coherent actionable remainder is `3C7AAC71` + `90F2A9F8`.

Considered groupings:

| Grouping | Members | Verdict |
|---|---|---|
| G1 — Success-shaped evidence loss on governed diagnostic paths | `3C7AAC71`, `90F2A9F8` | **Selected.** Same defect class, same CLI/MCP parity obligation, one coherent pull request |
| G2 — Original three-entry reliability feature | `3C7AAC71`, `90F2A9F8`, `84D8E6AB` | Rejected. `84D8E6AB` is already shipped as `143-F`/`144-F`; including it would duplicate closed work |
| G3 — Two solo features | `3C7AAC71` alone, `90F2A9F8` alone | Rejected. Two shipments for one defect class doubles review, harness, and closure overhead with no isolation benefit |

Coherence rationale for G1: both defects are cases where a governed backlogit operation produces a
**shaped result that hides lost evidence**. Checkpoint create returns `{"path": ...}` success and a
later read reports `valid: true` on a truncated record. Docs lint exits non-zero but emits no report
at all, so one malformed file makes conformance across the whole corpus unobservable. In both cases
the fix is the same shape: make the non-conforming input produce an explicit, per-unit, machine-readable
signal instead of disappearing. Both also require CLI/MCP parity, and both fix at a single shared core
seam (`internal/events`, `internal/docline`) rather than per surface.

Estimated scope: 9 tasks, roughly 18 hours. Risk: moderate — both changes touch pinned JSON contracts
and one touches a shared read path used by every checkpoint consumer.

## Problem frame

### Defect 1 — checkpoint context keys are silently discarded

`events.CreateCheckpoint` (`internal/events/memory.go:44`) probes the state dump for
`schema_version == 1`. On a match it calls `ParseCheckpoint`, which is a plain `json.Unmarshal` into
`CheckpointV1` with no unknown-field rejection, then **re-marshals the parsed struct** and writes that
to disk. `CheckpointContext` (`internal/events/checkpoint_schema.go:44-49`) models exactly four keys:
`shipment_id`, `feature_id`, `task_ids`, `branch`. Every other caller-supplied key inside `context`,
nested or flat, is dropped at the re-marshal step. Unknown **top-level** keys are dropped the same way.

The failure is doubly masked. `handleCreateCheckpoint` (`internal/mcp/tools.go:1113`) returns
`{"path": ...}` on success, and `GetCheckpoint` later reports the truncated record as valid, so
neither the write path nor the validation path surfaces the loss. An agent that trusts either signal
believes it persisted recovery state it did not persist. The observed loss occurred on 2026-08-19
during a Stage recovery on shipment `143-S`; the workaround in use is to encode structured recovery
data as prose in `resume_hint`.

Two constraints found in the code that the source record did not state:

1. **`ParseCheckpoint` is shared by the read path.** `ListCheckpoints`, `GetCheckpoint`,
   `ResolveCheckpoint`, and the disposition verbs all call it. Adding `DisallowUnknownFields` there
   would reclassify existing on-disk checkpoints as parse failures and mark them
   `needs_quarantine: true`. Strictness must be applied at the create boundary only.
2. **The legacy on-disk corpus is free-form, not V1.** All nine files under `.backlogit/checkpoints/`
   lack `schema_version`, so they never enter the V1 branch and are written verbatim. Their top-level
   keys (`pr_number`, `pr_status`, `ci_status`, `decisions`, `next_steps`, `review_gate`,
   `items_blocked`, `follow_up_tasks`) are direct evidence of the structured recovery data agents
   actually need and that the V1 shape cannot carry. The corpus is inconsistent with the writer, but
   in the opposite direction from what the source record assumed.

### Defect 2 — docs lint aborts the whole corpus on the first decode error

`docline.LintTree` (`internal/docline/service.go`) returns `nil, err` the moment `decodeDoc` fails for
any file. The caller therefore gets no `LintReport` at all. `backlogit docs lint`
(`internal/cli/docs.go`) propagates the error and exits 1 with no findings printed; MCP
`backlogit_docs_lint` (`internal/mcp/docs_tools.go:48`) returns `InternalError`. One unquoted YAML
scalar containing `": "` therefore suppresses conformance reporting for every other document. The
observed workaround was per-file `--path` targeting.

The required direction from the source record is explicit and is adopted unchanged: **retain the
fail-closed non-zero exit, but emit a per-file `decode_error` finding and continue scanning the
remaining corpus.** The gate stays fail-closed; it stops being fail-blind.

## Research findings

Retrieved from `docs/compound/` with high confidence. The load-bearing prior art:

* `2026-07-17-backlogit-update-drops-archive-provenance.md` — near-exact structural precedent for
  Defect 1: a typed round-trip over a wider on-disk shape re-emits only enumerated keys and drops the
  rest while returning success. The recorded ruling is that **preserve or refuse are the only
  acceptable outcomes; silent drop is the defect.**
* `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` — the commit-then-surface
  pattern is exactly what Defect 2 needs: accumulate per-item failures, finish the operation, return
  the fully built result wrapped with the honest error rather than aborting with `nil, err`.
* `2026-07-21-omitempty-defeats-arrays-always-json-contract.md` — a collection field under an
  "always an array" contract must never use `omitempty`, and a byte-parity test between two surfaces
  that marshal the same struct **cannot** catch a shape defect, because both sides drift identically.
  `TestDocsTools_CLIParity` is necessary but not sufficient for the `decode_error` change.
* `2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md` — a red-first test must
  not bypass the transport when the transport is implicated. For Defect 1 the argument extraction and
  marshal steps are part of the loss path, so tests must run through the registered MCP handler and
  the CLI flag, not through `events.CreateCheckpoint` directly.
* `2026-08-18-shipment-shipped-prevention-envelope.md` — enforce an invariant at the single narrowest
  seam every caller traverses, not once per surface, so the surfaces cannot drift.
* `2026-07-23-machine-readable-governance-field-contract.md` — producers own the format, consumers
  validate it. `decode_error` must be a pinned literal rule value with a documented allowed-value set,
  and producer plus both consumers change in the same pull request.
* `2026-07-29-durable-writes-test-seam-patterns.md` — package-level `var fooFn = realImpl` seams with
  `t.Cleanup` restore, path-selective mocks, and no `t.Parallel()` anywhere in a package that uses the
  seam.
* `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` — normalize output shape at the
  handler boundary so every caller inherits the guarantee; names
  `internal/cli/checkpoint_create_test.go` as the existing red-first home for checkpoint create tests.
* `2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` — prefer a denylist-of-known-exceptions
  parity test over an allowlist, or the next added member drifts silently. Place cross-surface tests in
  `internal/cli`, which already imports `internal/mcp`.

## Options evaluated

### Defect 1

#### Option A1 — Preserve arbitrary context keys

Give `CheckpointContext` a raw carrier for unmodeled keys and round-trip them losslessly.

* Pros: zero caller-visible breakage; matches observed agent need; matches the archive-provenance
  precedent's "preserve" branch.
* Cons: the `context` object becomes unbounded; nothing tells the caller which keys were understood.
* Effort: medium.

#### Option A2 — Reject unknown keys loudly at create

Strict-decode the state dump at the create boundary and fail with a validation error naming the
unknown keys.

* Pros: no silent loss; smallest schema surface; loud and immediate.
* Cons: forces every agent back into `resume_hint` prose encoding, which is the exact operational
  problem the source record complains about. Rejecting `context.pr_number` gives the caller nowhere
  to put it.
* Effort: low.

#### Option A3 — Namespace split: open caller namespace, closed schema namespace

Treat `context` as the **caller-owned** namespace and preserve every key in it verbatim; treat the
**top level** as the schema-owned namespace and reject unknown keys there loudly at the create
boundary. Report the preserved context key names in the create result so the caller can verify
persistence without a read-back.

* Pros: satisfies both halves of the source record's suggested fix without picking a losing side; gives
  structured recovery data a real home; keeps the schema contract closed and enforceable; the create
  result becomes machine-readable rather than a bare success envelope.
* Cons: largest of the three; requires custom marshal/unmarshal on `CheckpointContext` plus a
  create-boundary strict decode that must not leak into `ParseCheckpoint`.
* Effort: medium-high.

#### Option A4 — Document `resume_hint` as the only free-text carrier

Rejected without detailed evaluation: it ratifies the defect.

### Defect 2

#### Option B1 — Report and continue with a `decode_error` finding

Accumulate a per-file `decode_error` finding, continue scanning, return the complete report, and keep
the non-zero exit.

* Pros: removes whole-corpus masking; preserves fail-closed exit; identical shape on both surfaces;
  matches the commit-then-surface precedent.
* Cons: `LintTree`'s signature semantics change from all-or-nothing to partial-with-findings; the
  empty and degraded report shapes must be locked explicitly.
* Effort: medium.

#### Option B2 — Keep the abort, improve the message

* Pros: trivial.
* Cons: the corpus stays masked. Rejected.

#### Option B3 — Continue and exit zero on decode errors

* Pros: never blocks CI.
* Cons: converts a fail-closed gate into a fail-open one. Rejected; the source record explicitly
  requires the non-zero exit to be retained.

## Trade-off comparison

| Criterion | A1 preserve | A2 reject | A3 namespace split |
|---|---|---|---|
| Silent loss removed | Yes | Yes | Yes |
| Caller keeps a home for structured recovery data | Yes | No | Yes |
| Schema contract stays closed and enforceable | No | Yes | Yes |
| Caller can verify persistence without a read-back | No | Yes | Yes |
| Risk to the shared read path | Low | High if applied to `ParseCheckpoint` | Low, by construction |
| Effort | Medium | Low | Medium-high |

| Criterion | B1 report-and-continue | B2 better message | B3 continue and exit zero |
|---|---|---|---|
| Whole-corpus masking removed | Yes | No | Yes |
| Gate stays fail-closed | Yes | Yes | No |
| Matches the required direction in the source record | Yes | No | No |

## Decision

**Defect 1: adopt Option A3.** The two namespaces get opposite rules, and that asymmetry is the whole
point:

* `context` is caller-owned and **open**. Unmodeled keys are preserved verbatim through the
  create round-trip. The four modeled fields keep their typed accessors so filters
  (`CheckpointFilter.ShipmentID`, `FeatureID`) and summaries keep working unchanged.
* The **top level** is schema-owned and **closed**. Unknown top-level keys are rejected at the create
  boundary with an explicit validation error naming them, so an agent that mistakenly puts
  `pr_number` beside `phase` instead of inside `context` learns immediately.
* The create result stops being a bare `{"path": ...}`. It reports the persisted context key names so
  the caller can verify without a read-back. This is the machine-readable outcome pattern from
  `2026-07-23-machine-readable-governance-field-contract.md`.
* Strictness lives in a **create-only** decode. `ParseCheckpoint` stays lenient. This is a hard
  constraint, not a preference: tightening `ParseCheckpoint` would mark the existing on-disk corpus as
  quarantine candidates through `ListCheckpoints`.

**Defect 2: adopt Option B1.** `LintTree` accumulates rather than aborts. A file that fails to decode
yields one `Finding` with `Rule: "decode_error"` and `Severity: SeverityError`, carrying the
structured cause in `Fix` as actionable remediation text, and the scan continues. The change lands in
`internal/docline` so the CLI and MCP surfaces inherit identical behavior; neither consumer gets its
own fix-up. `LintReport.Findings` must remain a present, non-null array on every path, including a
corpus whose only finding is a `decode_error`.

Both defects ship as one covering feature and one shipment.

## Rejected alternatives

* A1 alone: preserves data but leaves the caller unable to distinguish "persisted" from "understood",
  and leaves the top-level schema silently lossy.
* A2 alone: honest but operationally worse than the status quo workaround, because it removes the only
  place structured recovery data could live without adding a replacement.
* B2 and B3: one keeps the masking, the other breaks fail-closed. Both contradict the source record.
* Extending the fix to `PlanMigration` / `ApplyMigration`: deliberately **out of scope**. Those are
  write paths whose all-or-nothing abort is a documented safety invariant (`ErrConcurrentEdit`,
  `ErrBodyMutated` preflight, zero partial writes). Converting them to report-and-continue would
  weaken a real guarantee to satisfy a symmetry argument.

## Scope boundaries

Explicitly **not** in scope for this feature:

* Any change to `ParseCheckpoint` read leniency, or to `ListCheckpoints` / `GetCheckpoint`
  quarantine classification.
* Migrating, rewriting, or backfilling the existing `.backlogit/checkpoints/` corpus.
* Widening `CheckpointProgress`, or adding new modeled top-level checkpoint fields.
* `PlanMigration` / `ApplyMigration` error handling.
* `84D8E6AB` / `0115F71F` shipment audit-log work — already shipped as `143-F` and `144-F`.
* Autoharness stash entry `F73BA065` (missing `source` / `doc_type` across `docs/compound/`). It was
  visible in the same source file but was not selected by the operator, and it is a corpus-content
  question rather than a linter-behavior question.

## Unresolved questions

1. Should the create result report *rejected* top-level keys as well as *persisted* context keys? The
   plan resolves this as: rejection is an error, so the key names belong in the error message, not in a
   success envelope.
2. Should a `decode_error` finding suppress the other findings for the same file? No — a file that
   fails to decode produces no other findings anyway, since field validation cannot run on it.
3. Is a depth limit needed on preserved `context` values? Deferred. No observed payload approaches a
   size where this matters, and a limit would reintroduce a silent-truncation class.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Strict create decode leaks into the shared read path and quarantines the legacy corpus | Strictness is implemented as a create-boundary-only decode; a regression test asserts every existing legacy checkpoint shape still parses through `ParseCheckpoint` and is not marked `needs_quarantine` for unknown keys |
| Custom `CheckpointContext` marshalling breaks the four modeled fields or their filters | Round-trip test over the modeled fields plus filter tests through `ListCheckpoints`; assert on-disk bytes, not the returned struct |
| Byte-parity test gives a false green on the new report shape | Add an explicit degraded-corpus assertion that inspects the marshalled JSON for a present, non-null `findings` array, per `2026-07-21-omitempty-defeats-arrays-always-json-contract.md`; do not rely on `TestDocsTools_CLIParity` alone |
| A red-first test bypasses the transport and proves nothing about the real loss path | Checkpoint tests dispatch through the registered MCP handler and the CLI `--state-dump` flag |
| Continuing after a decode error changes CI exit behavior unnoticed | An explicit test asserts a malformed-only corpus still exits non-zero and still emits a report |
| Package-global test seams cause flaky parallel runs | No `t.Parallel()` in any package that introduces a seam, per the recorded seam pattern |

## Traceability

* External source IDs: `3C7AAC71`, `90F2A9F8` (in scope); `84D8E6AB` (already fixed, not duplicated).
* Origin chain for `90F2A9F8`: autoharness stash `395EBE60`, deliberation
  `docs/decisions/2026-08-20-docline-lint-hard-abort-malformed-frontmatter-deliberation.md` question Q2.
* Origin chain for `3C7AAC71`: autoharness
  `docs/memory/2026-08-19/circuit-break-stage-pr372-review-fix-143-S.md`.
* Origin chain for `84D8E6AB`: autoharness `docs/closure/114-S-109-F-post-merge-closure.md` and
  `docs/decisions/2026-08-05-114s-closure-preactivation-fixes-deliberation.md`; already tracked locally
  as `0115F71F` to `059-DL` to `143-F` / `127-S`, with prevention in `144-F` / `128-S`.
