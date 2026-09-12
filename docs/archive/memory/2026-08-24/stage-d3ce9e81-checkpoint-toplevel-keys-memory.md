---
chunk_strategy: h1-h2-h3
description: "Stage session memory for stash D3CE9E81 — checkpoint unmodeled top-level key disposition staged as 147-F / 130-S"
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
schema_version: "1.0"
source: docs/memory/2026-08-24/stage-d3ce9e81-checkpoint-toplevel-keys-memory.md
title: "Stage Session Memory — D3CE9E81 Checkpoint Top-Level Key Disposition"
---

# Stage Session Memory — D3CE9E81 Checkpoint Top-Level Key Disposition

**Date**: 2026-08-24
**Agent**: Stage
**Session scope**: stash item `D3CE9E81` only (bounded, single item)
**Dark mode**: `DARK_MODE_ACTIVE` under P-017
**Resumed from**: hard-stopped prior Stage turn, recovery point Step 1.8

## Summary

Staged stash item `D3CE9E81` end-to-end through the Stage pipeline: learnings
retrieval, deliberation, implementation plan, plan hardening, plan review gate,
harvest, shipment assembly, and stash archival.

The item asked whether checkpoint `abandon` / `resolve` rewrites should
**preserve** unmodeled top-level JSON keys or **explicitly refuse** to mutate.
The decision is **refuse**, with two mandatory companions that make refusal
safe rather than a dead end.

## Decision

**Refuse to mutate.** A stored checkpoint document carrying unmodeled top-level
keys is not safely rewritable, so neither `resolve` nor `abandon` may rewrite it.
`quarantine` is widened to accept that class so the refusal is a routed
remediation rather than a deadlock.

Three parts:

1. **Refuse on both mutation verbs.** `AbandonCheckpoint` already gates on
   `ParseCheckpoint` + `ValidateCheckpoint`; add a conformance gate.
   `ResolveCheckpoint` has **no validity gate at all** and must gain both.
2. **Close the `ResolveCheckpoint` validity gap.** This is the higher-severity
   half of the defect and was not what the stash text emphasised.
3. **Widen quarantine classification.** Without this, a valid-but-non-conforming
   file has no disposition path at all.

## Grounding

### Shipped contracts that constrained the answer

| ID | Contract | Source |
|---|---|---|
| C1 | Checkpoint top level is a **CLOSED** namespace at create; `context` is the OPEN counterpart | `internal/events/checkpoint_strict.go` (`checkClosedSchemaNamespace`), 146.011-T |
| C2 | A document that cannot be trusted to round-trip is **moved verbatim, never rewritten** | `docs/design-docs/checkpoint-administrative-disposition.md` |
| C3 | Legacy (non-V1) dumps are written **verbatim** at create | `internal/events/memory.go:55-121` |

C1 is the decisive one: preserving arbitrary top-level keys outbound while
rejecting them inbound would make the namespace closed on the way in and open on
the way out.

### Empirical findings

| ID | Finding |
|---|---|
| F1 | Abandon and resolve are **not symmetric** — abandon already refuses invalid documents with `ErrCheckpointUseQuarantine`; all nine live legacy files already fail abandon |
| F2 | `ResolveCheckpoint` (`internal/events/checkpoint_lifecycle.go:139-184`) has **no validity gate** and replaces a legacy document with a fabricated skeleton (`schema_version: 0`, `agent: ""`, `created_at: "0001-01-01T00:00:00Z"`, `context: {}`). Reachable from CLI `:228` and MCP `:1223`; both Stage and Ship session-start recovery call it |
| F3 | A naive "refuse" **deadlocks** — `QuarantineCheckpoint` refuses any `parse OK && validate OK` target with `ErrCheckpointUseAbandon` |

### Live corpus

`.backlogit/checkpoints/` holds 11 files. Nine are legacy documents carrying
unmodeled top-level keys and are schema-invalid. Two are conforming V1:
`checkpoint-20260822-064434.json` (resolved) and `checkpoint-20260822-212617.json`
(active, stale `129-S`).

## Cited learnings

Bounded search under `docs/compound/` only (93 files), self-performed. The prior
delegated Learnings Researcher stalled by recursing into unrelated repositories,
so delegation of that objective was prohibited for this turn.

| Path | How it was used |
|---|---|
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | Direct preserve-vs-refuse precedent for a parse → mutate → re-marshal round trip dropping unmodeled state |
| `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | Deliberately **distinguished, not followed** — its preserve half does not transfer (arbitrary unowned keys vs. modeled owned frontmatter; namespace already closed at create; `CheckpointContext.Extra` is the sanctioned open carrier). Its **seam-enforcement** half **is** honoured: gate the mutation seam, not only create |

## Artifacts

| Artifact | Path |
|---|---|
| Deliberation | `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md` |
| Implementation plan | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| Compaction roll-up | `docs/archive/memory/2026-07-10-shipped-units-072S-087F-rollup-compacted.md` |

## Backlog state

* Feature **`147-F`** — "Refuse to rewrite checkpoints carrying unmodeled top-level keys", `queued`, `high`
* Tasks **`147.001-T` … `147.022-T`** (22 units, U1 … U10 plus U2e, U2f, U6c), all `queued`
* Shipment **`130-S`** — `queued`, `high`, 23 items, `covering_feature: 147-F`, `skipped: []`
* Stash **`D3CE9E81`** — `harvested`, archived to `.backlogit/archive/stash.jsonl`
  with `reason: harvested` and `harvested_artifact_id: 147-F`

Units were split from 11 to 19 at harvest, then to 22 during the PR #377 review
cycle, to satisfy the NON-NEGOTIABLE 2-Hour Rule
(< 3 files, < 5 functions, **< 4 test scenarios**). `147.005-T` (U2d),
`147.010-T` (U5b), and `147.016-T` (U8b) are pure regression guards that
explicitly declare exemption from the two-step red rule.

## Decisions and rationale worth carrying forward

* **Two-step red posture** is mandatory and stated at the head of the plan's
  Implementation Units: a declaration stub so the package *compiles*, then a
  harness that *fails on assertions*. A build error is not a red assertion.
* **U4 gate placement** — the conformance gate goes immediately after
  `ValidateCheckpoint` and **before** the already-abandoned short-circuit
  (authenticate-before-filter). Otherwise a non-conforming already-abandoned
  file returns `nil` from abandon while quarantine accepts it.
* **The predicate is round-trip safety, not "no unknown keys"** — `147.004-T`
  (U2c) makes `strings.EqualFold`-equal top-level keys non-conforming, reported
  as `duplicate:<key>`.
* **Invariant I3 is scoped totality** — disjoint and total only over
  `status: "active"`. A conforming `status: "resolved"` file is refused by
  **both** verbs (`ErrCheckpointNotActive` / `ErrCheckpointUseAbandon`); that is
  pre-existing and out of scope, pinned by `147.010-T`.
* **`147.018-T` (U9b) is a HARD MERGE GATE** — a PR containing `147.007-T`,
  `147.008-T`, or `147.009-T` MUST NOT merge without the
  `.github/instructions/backlogit.instructions.md` delta in the same merge
  commit. Shipping the refusal while the instruction file still tells agents to
  retry-and-nest strands every agent that hits it.
* **Halt condition** — if `147.009-T`'s paired accept/refuse assertion cannot be
  made to pass, halt rather than weakening it.
* **Constitution Check verdict is `documented-deviations`**, covering
  `AbandonCheckpoint`'s pre-existing `%v` wrap (Principle I), absence of refusal
  telemetry (V), and quarantine-as-source-moving-remedy plus U10 scratch
  teardown (VII).

## Failed and rejected approaches

* **Preserve raw top-level keys** — rejected. Contradicts C1 (closed namespace
  at create) and would require a raw-carrier field whose only consumer is the
  rewrite path.
* **Refuse without widening quarantine** — rejected. Produces F3's deadlock:
  valid-but-non-conforming files with no disposition path.
* **Delegating learnings retrieval** — prohibited this turn after the prior
  session's researcher stalled on out-of-repo recursion.

## Compaction outcome

`compact-context` executed as a mandatory prerequisite. `docs/memory/` was at
41 files / 215.3 KB, crossing the configured 40-file trigger.

* Built a verbatim roll-up of 15 already-compacted 2026-07-10 per-shipment
  summaries (`072-S` … `085-S`, `087-F`).
* `git mv`'d the 15 originals to `docs/archive/memory/` — **archive only, no
  deletion**.
* Result: **41 → 27 files**, 215.3 KB → 212.3 KB. `docs lint` clean.

No active-task checkpoints were compacted.

## Review outcome

Plan review gate: **PASS** (`dispatch_mode: multi-agent-dispatch`, two attempts).

* **Attempt 1** — all 7 personas dispatched. FAIL: 1 P0, 16 P1. (The
  Agent-Native Parity Reviewer failed on `gemini-3.1-pro-preview` and was
  retried successfully on `claude-sonnet-4.6`.)
* Remediation rewrote Requirements Trace, Resolved Design Questions, all
  Implementation Units (11 → 19), Dependency Graph, Decisions and Rationale,
  Risks and Caveats, Constitution Check, Runtime Verification and Closure, and
  Plan Hardening.
* **Attempt 2** — re-dispatched the 3 personas owning every P0/P1. Learnings
  Researcher **PASS**; Go Reviewer **ADVISORY** (no P0/P1); Constitution
  Reviewer **ADVISORY** with one new P1 (NF-3, U9b ordering was prose not a hard
  merge gate) and one P2 (NF-2). Both remediated immediately.

## Open follow-ups (not stashed — scope containment)

| Item | Note |
|---|---|
| Stale active checkpoint `checkpoint-20260822-212617.json` (129-S) | Superseded by completed closure. **Not disposed** — hygiene follow-up only |
| `CreateCheckpoint` same-second filename collision overwrite | Surfaced during grounding, deliberately not stashed |
| `AbandonCheckpoint` `%v` error wrap | Principle I deviation, recorded in the plan's Constitution Check |
| Refusal observability / telemetry | Principle V deviation, recorded in the plan |
| `CleanupCheckpoints` Windows `os.Remove` collision | Narrowed Ship-handoff follow-up |
| impl-plan skill `go run` self-lint entrypoint | Conflicts with Stage's no-build Role Boundary; deviation recorded |

## PR #377 review remediation cycle

Eight Copilot review threads were raised against the staging PR after CI passed.
All eight were confirmed against the frozen artefacts and the live source at
`540930d6`; none was a false positive. One (`147.018-T`) was partially valid on
its premise — plan units U9b and U10 shared a repair *mechanism* and differed only
in entry point, while the harvested task had invented a third procedure — and
fully valid on its remedy.

| Thread anchor | Category | Remediation |
|---|---|---|
| `147.003-T` nested `progress` | valid | New `147.020-T` (U2e): nested duplicate and fold-variant rule via an ordered token walk in the read-boundary helper only, so the shipped 146-F create boundary is untouched |
| `147.012-T` get result shape | valid | U6b now declares `CheckpointReadResult` + `GetCheckpointResult` with `GetCheckpoint` retained as a wrapper; new `147.022-T` (U6c) owns the MCP projection that removes the hardcoded `"valid": true` |
| `147.014-T` resolve error code | valid | `checkpoint_use_quarantine` for schema-invalid, `checkpoint_non_conforming` for unmodeled top-level keys, matching U7 |
| `147.016-T` parity fixtures | valid | Three-row matrix restored; `conforming` assertions moved to the `valid-but-non-conforming` row because a schema-invalid document never reaches a conformance verdict |
| `147.018-T` repair path | partial premise, valid remedy | One canonical body-preserving procedure with two entry points (direct repair, post-quarantine restore); creating a replacement checkpoint is explicitly rejected as the repair path |
| `147.019-T` verification hardening | valid | Scratch path pinned, containment asserted before first write, `.gitignore` rule owned by the task, binary built from branch HEAD, hash comparison over the whole checkpoint directory rather than a count-pinned subset |
| `.backlogit/memories.json` | valid | Continuity key populated with the 130-S / 147-F handoff record |
| `147.005-T` I1 bundling | valid | New `147.021-T` (U2f): I1 write-site enumeration or the gated `rewriteCheckpointFile` seam, split out because the fallback is a production seam inside a declared green-on-landing guard unit |

Net backlog delta: 19 tasks / 25 edges became 22 tasks / 34 edges. No task ID was
renumbered, no unit was removed, and the reviewed decision, scope, hierarchy, and
test-first ordering are unchanged. The ready set is still exactly `147.001-T`.

The SQLite index was **not** resynced: the edits were made in a dedicated Stage
planning worktree and a sync would have written to the polluted main worktree.
Refresh the index before trusting query output.

## Degraded visibility

Intercom tools were **unavailable** for the entire session. Remote operator
visibility is degraded; all dark-mode events are recorded locally in this memory
artifact and in the session summary rather than broadcast. No approval-dependent
destructive action was taken.

## Next steps

Ship agent picks up shipment `130-S`. Sequence is dependency-ordered from
`147.001-T`; `147.018-T` must land in the same merge commit as `147.007-T` /
`147.008-T` / `147.009-T`.

Stage's Role Boundary forbids feature branches and pull requests, so this
session stops at the committed-on-`main` staging handoff.

## Canonical status update (cycle 19, 2026-08-25)

The narrative above predates the branch `chore/stage-130-s` / PR #377 workflow
and the cycle 14-18 plan-review remediation cycles; it is left unedited as the
historical record of the original staging pass. This addendum is the current
canonical "next action" pointer for the initiative:

* Plan review is at **`cycle: 18`, `decision: ADVISORY`**
  (`dispatch_mode: single-agent-declared-degradation`,
  `TOOL_DEGRADED: reviewer-subagent-dispatch`, `operator_authorization: approved`,
  severity counts P0=0/P1=0/P2=1/P3=5, `push_allowed: yes`). Full record:
  `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
  (final `## Plan Review` section).
* Topology is **SOUND** and unchanged this cycle: 40 queued tasks under `147-F`,
  98 queued-to-queued executable edges, 41 members in shipment `130-S`, ready
  set exactly `{147.001-T, 147.032-T}`.
* **NEXT REQUIRED ACTION: push branch `chore/stage-130-s` and reconcile
  GitHub PR #377.** `operator_authorization: approved` records that the
  operator explicitly directed autonomous continuation until this bounded
  cycle is fully complete, so the advisory corrections are accepted — **this is
  not merge approval**, not a shipment claim, and not a Ship handoff. Ship must
  not claim `130-S` until its own build, review, and PR-lifecycle gates run
  against the pushed branch.
* Session checkpoint of record: `checkpoint-20260825-190220.json`
  (`phase: cycle-19-advisory-closure-complete`).

## Canonical status update (cycle 21, 2026-08-25)

This addendum supersedes the cycle-19 addendum above as the canonical "next action" pointer; the
cycle-19 addendum is left unedited as history, consistent with how it treated the narrative above
it. Between cycle 19 and cycle 21, **cycle 20** ran a full test-lifecycle remediation of PR #377's
20 unresolved Copilot threads (17 P1, 3 P2): it withdrew the "declared regression guards in the
harness" device plan-wide in favour of a three-step declaration → red-harness → green lifecycle,
made `147.032-T` / U1d and `147.038-T` / U15 `harness-exempt: declaration-only`, made `147.007-T` /
U3b `harness-exempt: verification-only` behind new unit U3c (`147.042-T`), added `147.043-T` / U6e
to make the `RemediationIntent` contract total, corrected `147.021-T`'s halt path, and grew the
topology to 42 tasks / 104 edges / 43 shipment members. Cycle 20 closed the `harness-exempt` set at
ten enumerated units but recorded its own gate as `FAIL` pending a fresh local plan review before
push.

* Plan review is now at **`cycle: 21`, `decision: FAIL`**
  (`dispatch_mode: single-agent-declared-degradation`,
  `TOOL_DEGRADED: reviewer-subagent-dispatch`, `operator_authorization: pending`,
  severity counts P0=0/P1=1/P2=1/P3=2, topology **SOUND**, `push_allowed: no`). Full record:
  `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` (final `## Plan
  Review` section, `cycle: 21`).
* Cycle 21 **is** the fresh local plan review cycle 20 required. It found and fixed, in the same
  pass: (P1) the closed ten-unit `harness-exempt` set was enumerated in the plan but applied to
  only three of its ten member tasks' frontmatter (`147.007-T`, `147.032-T`, `147.038-T`), with no
  plan-local rule telling Ship's ready-selection query to treat `harness-exempt` as an alternative
  to `harness-ready` — fixed by labelling the remaining seven (`147.017-T`, `147.018-T`,
  `147.019-T`, `147.021-T`, `147.026-T`, `147.036-T`, `147.041-T`) and adding a machine-readable
  Ship ready-selection adapter (a documented P-002 enforcement deviation, not a Principle II
  waiver) to the plan's Documented deviations section and `147-F.md`; (P2) residual ambient
  `backlogit docs lint` references in `147.017-T` (3 occurrences) and `147.018-T` (4 occurrences),
  replaced with `go run ./cmd/backlogit --cwd . docs lint`; (P3 x2) a cycle-19 numbering
  continuity gap and a stale current-gate-state pointer chain, both corrected.
* Topology is **SOUND** and unchanged this cycle: 42 queued tasks under `147-F`, 104
  queued-to-queued executable edges, 43 members in shipment `130-S`, ready set exactly
  `{147.001-T, 147.032-T}`, verified acyclic by an independent Kahn topological sort (42/42
  ordered).
* **NEXT REQUIRED ACTION: an independent confirmation review of the cycle-21 fixes, then push
  branch `chore/stage-130-s` and reconcile GitHub PR #377.** `operator_authorization: pending` —
  this session did **not** push, did not request merge approval, and did not claim shipment
  `130-S`. Ship must not claim `130-S` until Stage's confirmation review passes and its own build,
  review, and PR-lifecycle gates run against the pushed branch.
* Session checkpoint of record: see the newest `checkpoint-2026*.json` entry with
  `phase` starting `cycle-21-` in `.backlogit/checkpoints/` (created and validated via
  `go run ./cmd/backlogit --cwd . checkpoint create` / `checkpoint get`).
