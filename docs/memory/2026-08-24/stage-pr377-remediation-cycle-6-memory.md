---
chunk_strategy: h1-h2-h3
description: "Stage session memory for PR #377 Copilot review remediation cycle 6 — an operator-authorized extension past the three-cycle limit that corrected the U7d predicate-package reference to the backlogiterrors alias, made U7's checkpointDispositionError remediation string operation-aware to prevent U7d's resolve routing from advertising the wrong originating verb, and tightened U9b's canonical repair procedure so entry point (a) is scoped to schema-valid non-conforming stored documents while schema-invalid stored documents route to quarantine"
doc_type: memory
docline:
    date: 2026-08-24T00:00:00Z
    status: accepted
    tags:
        - session-memory
        - D3CE9E81
        - 147-F
        - 130-S
        - checkpoint
        - dark-mode
        - review-remediation
schema_version: "1.0"
source: docs/memory/2026-08-24/stage-pr377-remediation-cycle-6-memory.md
title: "Stage Session Memory — PR #377 Review Remediation Cycle 6"
---

# Stage Session Memory — PR #377 Review Remediation Cycle 6

**Date**: 2026-08-24
**Agent**: Stage
**Session scope**: PR #377 review remediation only, bounded to stash `D3CE9E81` / feature `147-F` / shipment `130-S`
**Dark mode**: `DARK_MODE_ACTIVE` under P-017, `admin_fallback_pre_authorized=false`
**Branch / worktree**: `chore/stage-130-s` in the dedicated Stage planning worktree
**Reviewed HEAD**: `72dd789048171c62070706e638b217e179d1625e`
**Cycle position**: sixth cycle, **explicitly authorized by the operator** as an extension past
the standard three-cycle review-fix limit — recorded as an extension, not as a silent counter reset

## Summary

Sixth Copilot review remediation cycle on staging PR #377. The fresh review
covering the current head raised exactly three unresolved threads. All three
were confirmed against live Go source and the staged artifacts, and all three
are **valid**. Two of the three (Comments 1 and 2) attach to `147.025-T`; the
third (Comment 3) attaches to `147.018-T`. The corrections are internally
coupled — Comment 2's operation-aware remediation is the mechanism that lets
Comment 1's routing fix advertise the correct originating verb on resolve — so
the cycle addresses them as one bounded consistency pass rather than three
independent point fixes.

This cycle **does not change the backlog shape**. Task count remains **27**,
declared `blocks` edges remain **43**, and `130-S` shipment membership remains
**28**. No stash operations, no shipment mutations, no new tasks, no
renumbering, no dependency edges added or removed. The corrections are
tightening the *content* of three plan units (U7, U7d, U9b) and their three
mirrored task cards (`147.013-T`, `147.025-T`, `147.018-T`), plus small
consistency clarifiers on the U10b downstream row (`147.026-T` and the plan
U10b section) so the restore verification aligns with the tightened U9b scope.

## Comment 1 — U7d references the wrong package for the U1 predicate

**Reviewer text (verbatim)**: "This call cannot compile as planned. U1 adds
`QuarantineIsRemedy` to `internal/errors/checkpoint_errors.go`, not the
`events` package, and `tools.go` already imports that package as
`backlogiterrors`. Route through `backlogiterrors.QuarantineIsRemedy(err)` and
make the same correction in authoritative plan U7d."

**Valid.** U1 places the predicate in the `internal/errors` package
(`internal/errors/checkpoint_errors.go`), and `internal/mcp/tools.go` line 21
imports that package under the alias `backlogiterrors`. The plan U7d code
block and its mirror in `147.025-T` both wrote `events.QuarantineIsRemedy(err)`,
which is the wrong package (`internal/events`) and would not compile against
the alias `tools.go` actually carries.

**Fix**: rewrote the code block in both surfaces to use
`backlogiterrors.QuarantineIsRemedy(err)`, and added prose to U7d that names
the alias, the source file, and the package boundary explicitly so this cannot
drift again through casual reading — the reference now cites both
`internal/errors/checkpoint_errors.go` and the existing `tools.go` import.

**Files touched**:
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
  (U7d section)
- `.backlogit/queue/147.025-T.md` (mirrored code block and prose)

## Comment 2 — checkpointDispositionError remediation names the wrong originating verb on resolve

**Reviewer text (verbatim)**: "Routing resolve through
`checkpointDispositionError` reuses remediation text that currently says to
call quarantine 'instead of backlogit_abandon_checkpoint.' A resolve failure
would therefore name the wrong originating verb. Make the U7 formatter
remediation operation-neutral or operation-aware, and add a resolve-handler
assertion for the remediation field."

**Valid.** The shipped `checkpoint_use_quarantine` remediation string in
`internal/mcp/errors.go` line 324 is `"this target is malformed; call
backlogit_quarantine_checkpoint instead of backlogit_abandon_checkpoint"`. The
`instead of backlogit_abandon_checkpoint` clause is hardcoded to a specific
originating verb. That is correct today because only the abandon and quarantine
handlers reach the formatter, but once U7d routes `handleResolveCheckpoint`
through the same formatter a resolve refusal would advertise
`backlogit_abandon_checkpoint` as the operator's original verb — a lie that
would send the operator to the wrong entry point.

**Design choice**: the formatter already receives `op` (`"abandon
checkpoint"`, `"quarantine checkpoint"`, and after U7d `"resolve checkpoint"`).
The smallest coherent fix is **operation-aware**, not operation-neutral:
derive the operator-facing verb from the first word of `op` and interpolate
`backlogit_<verb>_checkpoint` into the "instead of" clause of each of the
three disposition-class remediations (`checkpoint_use_quarantine`,
`checkpoint_use_abandon`, and the new `checkpoint_non_conforming`). This
keeps the formatter operation-aware without adding a new parameter and
without weakening the operator-facing prose to something like "instead of
whichever verb you called". The remedy verb (`quarantine` for the two
"target is malformed / non-conforming" classes, `abandon` for the "target is
valid" class) is **unchanged**; only the wronged verb becomes op-derived.

**Ownership split**: formatter logic stays with U7 / `147.013-T`; handler-side
assertions stay with U7d / `147.025-T`. Width isolation is preserved — U7 owns
`errors.go`, U7d owns `tools.go` and the handler-side test assertions.

**Assertions added**:
- U7 / `147.013-T` — the existing abandon-side test now also asserts the
  `remediation` string names `backlogit_abandon_checkpoint` as the originating
  verb. The assertion holds because abandon is the caller here, but its
  purpose is a regression guard against a silent revert to a hardcoded
  wronged verb.
- U7d / `147.025-T` — cases 1 and 2 now additionally assert that
  `handleResolveCheckpoint`'s payload names
  `backlogit_resolve_checkpoint` (not `backlogit_abandon_checkpoint`) as the
  originating verb. Together with U7's assertion this pins the op-derived
  interpolation from both directions.

**Files touched**:
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
  (U7 change note sub-item 4 added, U7 test list clarified, U7d test list
  extended with the resolve-side verb assertion)
- `.backlogit/queue/147.013-T.md` (change note sub-item 4 mirrored,
  acceptance criterion extended)
- `.backlogit/queue/147.025-T.md` (acceptance criteria extended)

## Comment 3 — U9b entry point (a) does not distinguish validity from conformance

**Reviewer text (verbatim)**: "This repair only fixes conformance, but the
feature explicitly includes schema-invalid stored documents. After moving
their unmodeled keys, `checkpoint get` still returns `ErrCheckpointInvalid`
before producing `conforming: true`, so the promised normal resolve/abandon
step remains unreachable. Scope this procedure to schema-valid non-conforming
files and route schema-invalid files to quarantine, or define how validation
defects are repaired and verified too."

**Valid.** `checkpoint get` on a schema-invalid document refuses with
`ErrCheckpointInvalid` **before** producing any conformance verdict (U6b, U6c,
U8c). Moving unmodeled keys under `context.legacy_top_level` cannot fix a
validation defect — `legacy_top_level` relocates keys byte-for-byte, it does
not repair shapes. The instruction file has no in-place repair path defined
for arbitrary validation defects, and does not want one: the whole point of
U3/U4/U5 is that a schema-invalid stored document is refused and its remedy
is quarantine.

**Fix**: added a **validity precondition** to entry point (a) that explicitly
separates validity from conformance:
- Entry point (a) is scoped to a stored document `checkpoint get` reports as
  `valid: true, conforming: false`.
- A schema-invalid stored document is routed to `quarantine_checkpoint`
  directly. Moving unmodeled keys is stated to be incapable of fixing a
  validation defect, and this instruction file promises **no** in-place
  repair for arbitrary validation defects.
- The two entry points are stated as disjoint conditions before the
  procedure body: (a) is the active-file valid-but-non-conforming repair,
  (b) is the post-quarantine restore.
- The exact-duplicate modeled-key and nested `progress` fold-variant
  behaviours in entry point (a) are **unchanged** — operator choice or
  quarantine, never a silent value selection.

Entry point (b) is extended with a matching **restore-abort rule**: quarantines
routed by U3 (validity gate on resolve) or U4/U5 (validity gate on abandon)
preserve schema-invalid bytes, so the archived evidence may fail validation.
If `checkpoint get` refuses the restored bytes with `ErrCheckpointInvalid`,
entry point (a) is inapplicable; if entry point (a)'s termination rule fires
(the restored file is valid-but-non-conforming with a move-untouchable
offender such as a nested `progress` duplicate), the same abort applies. In
either case the operator removes the active copy at
`.backlogit/checkpoints/<filename>` and the file remains quarantined; the
renamed archive evidence at
`archive/checkpoints/<filename>.quarantined-<disposition_at>` and its
`.disposition.json` sibling are **untouched** by the abort. The restore
succeeds only when `checkpoint get` reports the active copy as **both**
`valid: true` **and** `conforming: true`.

**Mirrored downstream consistency** (U10b / `147.026-T`): the plan U10b row
2 and its mirror in `147.026-T` step 2 now explicitly identify the quarantine
archive as **valid-but-non-conforming**, because 147.019-T step 1's `get`
already reported `conforming: false` on the same fixture — which under U6b/U6c
is only possible on a schema-valid document. This makes the fixture-class
selection unambiguous: entry point (a)'s schema-valid precondition holds so
its classified moves converge; a schema-invalid archive would land on U9b's
restore-abort rule and the row would fail.

**Merge gate preserved.** `147.018-T` remains a HARD MERGE GATE with
`147.007-T` / `147.008-T` / `147.009-T` (U3b / U4 / U5). The paired assertion
halt condition and the U9 dependency stay intact.

**Files touched**:
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
  (U9b validity preamble + restore-abort rule; U10b row 2 fixture-class
  clarifier)
- `.backlogit/queue/147.018-T.md` (mirrored validity preamble +
  restore-abort rule; acceptance criteria extended with the two new
  guarantees)
- `.backlogit/queue/147.026-T.md` (step 2 fixture-class clarifier;
  acceptance criterion extended to require both `valid: true` and
  `conforming: true` before the `resolve` verb)

## Counts and manifest — unchanged

| Metric | Cycle 5 (start) | Cycle 6 (end) | Delta |
|---|---|---|---|
| Feature `147-F` tasks | 27 | 27 | 0 |
| Declared `blocks` edges | 43 | 43 | 0 |
| Shipment `130-S` members | 28 (147-F + 27 tasks) | 28 | 0 |

No task added or renumbered. No dependency edge added or removed. No shipment
member added or removed. Architecture and backlog shape are unchanged from
cycle 5's freeze, exactly as directed.

## Constraint compliance

- **Role boundary**: only backlog (`147.013-T`, `147.018-T`, `147.025-T`,
  `147.026-T`) and planning (`docs/exec-plans/…` and `docs/memory/…`) files
  were written. No source (`internal/**`), no tests (`*_test.go`), no
  configuration (`go.mod`, `.autoharness/**`, `.github/instructions/**`), and
  no CI or lint config touched. Ship is the owner of the source-code
  implementations these corrections describe.
- **No shipment operations**: no `claim_shipment`, no `add_to_shipment`, no
  `create_shipment`, no `ship_shipment`. Shipment `130-S` is unchanged.
- **No PR/GitHub interaction**: no push, no comment/thread response, no
  resolve, no merge, no re-request-review.
- **No amending existing commits**: this cycle is a **new** conventional
  commit on top of `72dd7890`. Prior cycle commits (through
  `72dd7890`) are not touched.
- **Bounded scope**: only the three unresolved comments are addressed. No
  drive-by edits, no re-litigation of resolved cycle-4 or cycle-5 threads,
  no template updates.
- **Dark mode visibility**: this memory record is the visibility artifact for
  the cycle; no operator broadcast is emitted from this session (Stage runs
  local-only under P-017).

## Validation

- `.backlogit/queue/147.013-T.md` — YAML frontmatter unchanged, acceptance
  criterion count extended by one sub-clause; docs lint expected clean.
- `.backlogit/queue/147.018-T.md` — YAML frontmatter unchanged, acceptance
  criterion list extended by two entries; docs lint expected clean.
- `.backlogit/queue/147.025-T.md` — YAML frontmatter unchanged, acceptance
  criteria list extended; code block corrected to the `backlogiterrors`
  alias; docs lint expected clean.
- `.backlogit/queue/147.026-T.md` — YAML frontmatter unchanged, step 2
  narrative and one acceptance criterion extended; docs lint expected clean.
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` —
  U7, U7d, U9b, U10b sections extended; dependency graph unchanged; docs
  lint expected clean.
- Task/edge/manifest counts spot-checked: `147-F` still parents 27 numbered
  tasks (`147.001-T` through `147.027-T`); no dependency edges added or
  removed by these edits; `130-S` manifest still lists 28 members.
- Package alias parity: plan U7d code block and `147.025-T` code block both
  read `backlogiterrors.QuarantineIsRemedy(err)`.
- Plan-vs-task mirror parity: U9b classification table + validity preamble +
  restore-abort rule in the plan match the mirrored text in `147.018-T`.

## Follow-ups

None. The three unresolved review comments are addressed within Stage's
role boundary. Ship inherits the corrected plan and task cards when
implementation resumes:
- `147.013-T` — implement the operation-aware remediation interpolation in
  `checkpointDispositionError` and the mirrored abandon-side assertion.
- `147.025-T` — route `handleResolveCheckpoint` through
  `checkpointDispositionError` using the `backlogiterrors.QuarantineIsRemedy`
  predicate, and add both resolve-side remediation-verb assertions.
- `147.018-T` — apply the tightened U9b delta to
  `.github/instructions/backlogit.instructions.md`, including the validity
  precondition on entry point (a) and the restore-abort rule on entry point
  (b).

## Session hand-off

- **Reviewed HEAD after this cycle** will be the SHA of the cycle 6 commit
  (recorded when the commit lands, not amended in). Prior HEAD
  `72dd789048171c62070706e638b217e179d1625e` is unchanged.
- **Branch**: `chore/stage-130-s`, dedicated Stage worktree, not pushed by
  this session per the operator's directive.
- **Next Stage action**: none. Await operator direction on next steps
  (Copilot re-review of the cycle 6 head, or handoff to Ship when the
  operator judges the plan/task cycle stable).
