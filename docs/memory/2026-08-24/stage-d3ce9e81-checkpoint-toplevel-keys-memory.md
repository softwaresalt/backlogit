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
| Compaction roll-up | `docs/memory/compacted/2026-07-10-shipped-units-072S-087F-rollup-compacted.md` |

## Backlog state

* Feature **`147-F`** — "Refuse to rewrite checkpoints carrying unmodeled top-level keys", `queued`, `high`
* Tasks **`147.001-T` … `147.019-T`** (19 units, U1 … U10), all `queued`
* Shipment **`130-S`** — `queued`, `high`, 20 items, `covering_feature: 147-F`, `skipped: []`
* Stash **`D3CE9E81`** — `harvested`, archived to `.backlogit/archive/stash.jsonl`
  with `reason: harvested` and `harvested_artifact_id: 147-F`

Units were split from 11 to 19 to satisfy the NON-NEGOTIABLE 2-Hour Rule
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
