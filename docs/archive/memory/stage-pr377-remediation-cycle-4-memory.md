---
chunk_strategy: h1-h2-h3
description: "Stage session memory for PR #377 Copilot review remediation cycle 4 — an operator-authorized extension past the three-cycle limit that added the missing CLI get projection unit, moved scratch-workspace teardown ownership, published the ListCheckpoints filter exemption, and split checkpoint lifecycle metadata"
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
source: docs/memory/2026-08-24/stage-pr377-remediation-cycle-4-memory.md
title: "Stage Session Memory — PR #377 Review Remediation Cycle 4"
---

# Stage Session Memory — PR #377 Review Remediation Cycle 4

**Date**: 2026-08-24
**Agent**: Stage
**Session scope**: PR #377 review remediation only, bounded to stash `D3CE9E81` / feature `147-F` / shipment `130-S`
**Dark mode**: `DARK_MODE_ACTIVE` under P-017, `admin_fallback_pre_authorized=false`
**Branch / worktree**: `chore/stage-130-s` in the dedicated Stage planning worktree
**Reviewed HEAD**: `bb4879237ca2ada40cf3416530563acbbecd6ac9` (CI green, six checks)
**Cycle position**: fourth cycle, **explicitly authorized by the operator** as an extension past
the standard three-cycle review-fix limit — recorded as an extension, not as a silent counter reset

## Summary

Fourth Copilot review remediation cycle on staging PR #377. The fresh review
`PRR_kwDORzozKM8AAAABKsXFuA` covers the current head and raised five unresolved
threads under the summary "the plan omits the required CLI get projection unit,
relies on disposable cross-task state, and contains inconsistent lifecycle
metadata". Thread pagination completed in one page (36 threads total,
`hasNextPage: false`); all five unresolved threads are authored by
`copilot-pull-request-reviewer` and none is outdated.

All five were confirmed against live Go source and the staged artifacts. All five
are **valid**. One (the checkpoint metadata thread) is valid on the remedy while
carrying a partly stale premise, and the correction is recorded rather than
silently adopted. One (the `hooks_queue.jsonl` thread) is only **partially
remediable** without violating the tool-managed data-ownership rule; the residual
is recorded in the plan, in the checkpoint, and here.

This cycle changes the backlog shape by exactly one unit: `147.027-T` (plan U8c).
The graph goes from 26 tasks and 40 edges to 27 tasks and 43 edges, and the
`130-S` manifest grows from 27 to 28 members.

## The missing CLI projection (`147.027-T` / U8c)

Cycle 3 split `147.013-T` and `147.014-T` for granularity, and in the process U8's
change note lost the CLI `checkpoint get` projection. The loss was real, not
cosmetic: `newCheckpointGetCmd` (`internal/cli/checkpoint.go:180-210`) still calls
`events.GetCheckpoint` and prints a literal `valid: true`, so after `147.012-T`
lands the CLI would keep answering the superseded question while its MCP twin
(`147.022-T` / U6c) answers the new one — and `147.016-T` (U8b) asserts the two
agree. The parity unit would have been guaranteed to fail with no unit able to fix
it.

Folding the projection back into `147.015-T` (U8) was rejected: U8 already carries
three scenarios (resolve accept, resolve refuse, abandon), and a fourth breaches
the task-granularity limit the reviewer explicitly cited. So U8c is a separate,
width-isolated unit: one skill domain (CLI presentation), one production file plus
its test, three scenarios, one dependency (`147.012-T` / U6b).

Both sides carry reciprocal exclusion notes so the split cannot silently re-merge:
`147.015-T` must not touch `newCheckpointGetCmd`, `147.022-T` must not change the
CLI, and `147.027-T` must not change the MCP handler. `147.022-T` also names its
CLI twin explicitly.

## Teardown ownership moves from U10 to U10b

`147.026-T` (U10b) consumes `147.019-T` (U10)'s scratch workspace — the
branch-built binary, the copied fixtures, and the quarantine archive its second
row restores from. U10 owned an operator-approved teardown of that directory at
its own completion. The ordering was left to chance and the losing order silently
destroys U10b's inputs.

Fix: U10 hands the workspace over intact; U10b owns teardown and performs it only
after all three of its rows pass. The `ActionRisk: destructive` classification,
the Constitution VII approval requirement, and the withheld-approval fallback are
unchanged — only the owner and the timing moved. Re-attributed in five places: the
U10 change note, the new U10b *Inherited inputs and teardown* bullet, the
Constitution VII deviation row, the conflict-resolution row, risky action **A4b**,
and the Teardown bullet in the verification section.

A missing workspace now explicitly **blocks** U10b on re-running U10. It is not a
licence to hand-rebuild the inputs, because a hand-rebuilt binary is not the
branch-built binary the verification exists to exercise.

## The `ListCheckpoints` filter exemption becomes a published contract

`147.023-T` (U6d) makes `ListCheckpoints` stop applying its documented Agent /
Status / ShipmentID / FeatureID / MaxAge filters to quarantine candidates. That is
the right behaviour — a quarantine candidate must not be filtered out of the very
listing that advertises its remedy — but cycle 3 shipped it as a silent internal
change. An exported function that ignores its own documented options is a trap for
every caller that is not this feature.

U6d now updates the exported `ListCheckpoints` doc comment to state the exemption
and its consequence: filtered results may include entries mismatching status,
agent, feature, shipment, or age. A sixth acceptance criterion covers the doc
comment, and the expected-red note records that it is a **contract obligation, not
a fourth test scenario**, so the three-scenario budget is untouched.

The agent-facing description in `147.014-T` (U7b) carries the same sentence, which
creates a genuine ordering constraint: a published description must not promise
behaviour no landed unit implements. New edge `U6d → U7b` (`147.014-T` now depends
on `147.023-T`).

## The partially remediable hooks queue

The committed `hooks_queue.jsonl` stops at the original 19-task shipment creation,
so consumers polling the durable queue never saw creation signals for `147.020-T`
through `147.026-T`.

What was done: every cycle-4 mutation was executed through the **supported
backlogit CLI surface** (`backlogit.exe --cwd <worktree>`) rather than by editing
markdown, so nine genuine events landed at seq 2305-2313 — one `create_artifact`
for `147.027-T` and eight `update_artifact` events for `147-F`, `147.014-T`,
`147.015-T`, `147.016-T`, `147.019-T`, `147.022-T`, `147.023-T`, and `147.026-T`.

What was rejected and why:

* **Hand-appending retroactive `create_artifact` rows.** Forbidden by the
  data-ownership rule, and the events would be lies — the artifacts already exist,
  so no creation happens at append time.
* **Reverting the file (the reviewer's option b).** The existing rows are genuine
  tool-emitted history. Reverting while the worktree runtime keeps appending forks
  the sequence space against main's copy of the same file, which is a worse defect
  than the one being fixed.

**Residual**: `147.020-T`, `147.021-T`, `147.024-T`, and `147.025-T` still carry no
lifecycle event. Mitigation: all four are discoverable through the SQLite index and
`queue view`, and each emits a genuine `update_artifact` the moment Ship claims it.

## Checkpoint lifecycle metadata

The handoff labelled PR #377 "CI green" while `push_state` said the cycle-3
remediation was local-only, so Ship could resume treating unvalidated code as
validated.

Premise correction: the branch **is** pushed (`origin/chore/stage-130-s ==
bb487923`) and CI **is** green there — all six checks (Docline frontmatter gate,
test, CLI Reference Drift, copilot-pull-request-reviewer, Detect code changes,
Markdown lint (P-008)). The artifact was nonetheless internally inconsistent, and
it goes stale the instant cycle 4 commits.

Fixed in the **`context` namespace only**. The top-level namespace is closed — this
is precisely the feature that refuses unmodeled top-level keys, so writing one into
its own handoff checkpoint would be self-refuting. Changes:

* `context.ci_state` (new) — last verified green SHA plus the six named checks, and
  an explicit statement that the cycle-4 commit is **not** covered by that run.
* `context.push_state` — names the remote SHA and marks the cycle-4 commit local-only.
* `context.pr` — separates the reviewed/green remote head from the unpushed local tip.
* `context.review_remediation` — cycle-4 entry appended, including the residual.
* `context.task_ids` — 26 → 27 with `147.027-T`.
* `resume_hint` (modeled top-level field) — requires push plus fresh CI **and** a
  fresh Copilot review covering the new head before any merge-readiness claim.

## Backlog shape after this cycle

| Measure | Before cycle 4 | After cycle 4 |
|---|---|---|
| Implementation units in the plan | 26 | 27 |
| Harvested tasks under `147-F` | 26 | 27 |
| `blocks` edges in the task graph | 40 | 43 |
| `130-S` manifest members | 27 | 28 |

New edges: `U6b → U8c` (`147.027-T` depends on `147.012-T`), `U8c → U8b`
(`147.016-T` depends on `147.027-T`), `U6d → U7b` (`147.014-T` depends on
`147.023-T`). No unit removed, no ID renumbered, no dependency retargeted.

## Files changed

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — R8 trace
  row, U6d, U7b, U8, new U8c section, U8b, U10, U10b, Constitution VII row,
  conflict-resolution row, A4b risky-action row, Teardown bullet, dependency-graph
  ASCII, edge table, execution order, and the appended cycle-4 remediation record.
* `.backlogit/queue/147.027-T.md` — new (U8c).
* `.backlogit/queue/147.014-T.md`, `147.015-T.md`, `147.016-T.md`, `147.019-T.md`,
  `147.022-T.md`, `147.023-T.md`, `147.026-T.md`, `147-F.md` — updated through the CLI.
* `.backlogit/queue/130-S.md` — manifest 27 → 28.
* `.backlogit/hooks_queue.jsonl` — nine genuine tool-emitted events (seq 2305-2313).
* `.backlogit/checkpoints/checkpoint-20260824-191617.json` — `context` keys and
  `resume_hint` only.
* `docs/memory/2026-08-24/stage-pr377-remediation-cycle-4-memory.md` — this file.

## Not done by Stage

Role Boundary forbids all of it, and dark mode does not relax it:

* No push, no PR mutation, no review replies, no thread resolution, no review request.
* No merge, and no admin fallback (`admin_fallback_pre_authorized=false`).
* No Go source, test, or config changes — every finding was remediated in planning
  and backlog artifacts, which is where the defects actually lived.
* No `go build`, `go test`, `go vet`, or `golangci-lint` runs. Validation was
  artifact-level: JSON parse, frontmatter parse, dependency existence, cycle check,
  ready-set computation, shipment-vs-inventory parity, and plan-task parity.
* No changes outside the Stage worktree; the dirty main checkout and the third
  worktree were left untouched.

## Next steps

1. Operator or Ship pushes `chore/stage-130-s`; the cycle-4 commit is local-only.
2. Fresh CI must pass on the new head, and a fresh Copilot review must cover it —
   the `bb487923` green run does **not** transfer.
3. Post the five prepared replies and resolve the five threads (Ship or operator).
4. P-014 operator merge approval is still outstanding.
5. Ship claims `130-S`; a genuine `update_artifact` event then lands for each of the
   four residual tasks (`147.020-T`, `147.021-T`, `147.024-T`, `147.025-T`).
6. Any fifth review with new P0/P1 findings needs another explicit operator decision;
   cycle 4 consumed the authorized extension.
