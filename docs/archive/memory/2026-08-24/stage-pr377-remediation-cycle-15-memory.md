---
chunk_strategy: h2
description: "Stage PR #377 cycle-15 remediation — cycle-14 plan-review FAIL gate recorded, U2g contract unified, width normalized, topology regenerated"
doc_type: memory
schema_version: "1.0"
source: cycle-15-session
title: "PR #377 Cycle 15 Remediation Memory"
---

# PR #377 Cycle 15 Remediation Memory

**Date**: 2026-08-25
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 15 (operator-authorized extension)
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**Shipment**: `130-S` (queued — **not** claimed, **not** shipped)

## Outcome

Recorded the formal cycle-14 plan-review gate as `decision: FAIL` and remediated every P0 and P1
finding in one bounded planning pass. No Go source, test, or configuration file was modified.
Stage's Role Boundary was observed throughout: no push, no PR action, no shipment claim, no build,
no lint of Go code.

## Gate record

Appended a second `## Plan Review` section to the plan carrying the literal fields
`cycle: 14`, `dispatch_mode: multi-agent-dispatch`, `decision: FAIL`. All seven selected personas
returned: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher,
Architecture Strategist, Agent-Native Parity Reviewer, Security Lens Reviewer. The pre-existing
`## Plan Review` PASS record was annotated as historical and scoped to cycles 1-13, so it can no
longer be read as clearance for the cycle-14 plan state.

## Decisions and rationale

### D1 — one contract for U2g (the central P0)

`ParseCheckpoint` and `CheckpointContext.UnmarshalJSON` stay lenient. Context-member duplicate
detection lives in the caller-invoked read-boundary conformance helper, which already receives the
original bytes before any rewrite and already returns `*CheckpointNonConformingError`.
`ErrCheckpointContextDuplicateKey` is withdrawn; offenders report as `duplicate:context.<key>`.

Evidence that forced this choice:

* `ParseCheckpoint` wraps with `fmt.Errorf("%w: %v", ErrCheckpointCorrupt, err)`
  (`internal/events/checkpoint_schema.go:420-426`) — `%v` drops any parse-time sentinel, so
  `errors.Is` could never recover it.
* A parse-time refusal fires on non-mutating reads, so `list`, `get`, `resolve`, `abandon`, and
  `quarantine` would disagree about one file — the exact cross-surface incoherence R8 exists to
  prevent.
* `decodeTopLevelEntries` (`internal/events/checkpoint_strict.go:75-90`) already provides the
  ordered, duplicate-preserving walk the helper needs, so no new mechanism is required.

### D2 — decided duplicate semantics

| Case | Verdict | Grounding |
|---|---|---|
| Exact duplicate **decoded** member names (including `\u0066oo` vs `foo`) | Refuse universally | `map[string]json.RawMessage` collapses last-wins before caller code runs |
| Fold variant aliasing a modeled context field | Refuse | shadow decode picks one winner; `isModeledContextKey` filters **both** out of `Extra` (`checkpoint_schema.go:196-223`), destroying the loser |
| Fold-distinct unmodeled names (`foo` / `Foo`), NFC/NFD-distinct extensions | Conforming | distinct `Extra` map keys, lossless round-trip; refusing them would narrow the open namespace U2b protects |
| Algorithm | Go `strings.EqualFold`, no normalization | same relation `encoding/json` uses; `isFoldKeyIn`'s doc comment records why normalization reintroduces the closed bug |

The harvested task previously required refusing `foo` / `Foo`. That was wrong and directly
contradicted the U9b repair matrix row that already classifies distinct spellings as safely
movable. The task now matches the plan and the shipped decode behaviour.

### D3 — U7e retained, not archived

Scope called U7e YAGNI. Source inspection refutes that for at least one row:
`internal/mcp/errors.go:148-200` carries no case for `ErrCheckpointUseQuarantine`,
`ErrCheckpointNonConforming`, or `ErrCheckpointCannotResolveAbandoned`; all reach
`default: InternalError`. After U7d, `handleResolveCheckpoint` reroutes only `QuarantineIsRemedy`
matches, which the abandoned-resolve sentinel does not satisfy, so
`internal/mcp/tools.go:1224` still surfaces it as a 500.

The unit gained a **mandatory ordering constraint**: the three rows must precede the combined
`ErrValidation` / `ErrCheckpointInvalid` / `ErrCheckpointCorrupt` case, because U3's multi-`%w`
refusal satisfies both matchers and an appended row would be permanently shadowed. Tests now use
realistically wrapped errors, which is what makes the ordering defect detectable.

### D4 — width normalization

| Unit | Task | Before | After |
|---|---|---|---|
| U7d | 147.025-T | plan said 3 files, `Tests (4)` | 2 files, 3 scenarios (plan synchronized to the already-correct task) |
| U6 | 147.011-T | 4 scenarios | 3 — byte-identity recomposed as a postcondition of scenarios 1-3 |
| U5 | 147.009-T | 5 effective scenarios | 3 — absorbed state-conflict guards became extra rows of scenario 2 |
| U7 | 147.013-T | "four coupled defects" | three owned deltas plus one correction of record |

### D5 — security-lens scope split

Absorbed because they are directly required for safe disposition: diagnostic key disclosure
(quoting plus a 16-path / 128-byte cap on `Error()`, added to U1) and archive-restore no-clobber
(added to the recovery-procedure contract). Deferred with named stash follow-ups:
`E429A031` (create-boundary `context` duplicates) and `35A27CD0` (symlink / no-follow containment
and the read-to-write CAS race). Rollback safety needs no follow-up — with no data migration, no
schema change, and no on-disk format change, revert-the-merge is already complete.

### D6 — rejected review claims

* **Architecture: "U2g and U7e are absent."** Stale — both sections exist. Removing them would
  have deleted the cycle-14 remediation itself. No regression applied.
* **Constitution: "a linked git worktree is a P-016 or containment violation."** Not valid. P-016
  requires one dedicated implementation branch in one active worktree; a linked worktree is a
  mechanism for that, not a second branch. All work ran inside one worktree on
  `chore/stage-130-s`, and the root checkout was never touched.
* **Constitution: "the U9b same-merge constraint means a merge per backlog task."** Misreading.
  Ship builds one release-unit branch and one PR, so the constraint binds the implementation units
  to each other inside that PR. Wording clarified in the U9b section.

## Files modified

| File | Change |
|---|---|
| `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` | U2g rewritten; U7e rewritten; U7, U7d, U6, U5, U8, U1 corrected; Requirements Trace R1/R7 updated; dependency graph, edge table and execution order regenerated; A4c/A4d risky actions and read-only live-observation rule added; recovery-procedure contract added; ldflags provenance corrected; cycle-15 runtime verification row added; eight-gate sequence added; follow-ups extended; cycle-14 `FAIL` gate record and cycle-15 appendix appended; prior PASS annotated historical |
| `.backlogit/queue/147.028-T.md` | U2g rewritten to the conformance-helper contract; refusal rule decided; case 3 reclassified as a regression guard |
| `.backlogit/queue/147.029-T.md` | U7e reachability proven; ordering constraint added; wrapped-error acceptance criteria |
| `.backlogit/queue/147.011-T.md` | U6 normalized to 3 scenarios with byte-identity as a postcondition |
| `.backlogit/queue/147.009-T.md` | U5 normalized to 3 scenarios; absorbed guards folded into scenario 2 |
| `.backlogit/queue/147.018-T.md` | U9b context-member enforcement wording synchronized to the helper placement and the decided fold rule |
| `.backlogit/archive/147.010-T.md` | prominent RETIRED / DO NOT IMPLEMENT caution added; acceptance criteria marked VOID |
| `.backlogit/checkpoints/checkpoint-20260824-191617.json` | `plan_review` set to the cycle-14 FAIL / cycle-15 remediated state; topology and compaction footprint refreshed |
| `.backlogit/memories.json` | canonical Stage handoff entry refreshed for cycle 15 |
| `docs/memory/2026-08-24/stage-pr377-remediation-cycle-15-memory.md` | this file |

## Topology

Measured with `backlogit --cwd . query` after `backlogit --cwd . sync`.

| Measure | Cycle 14 | Cycle 15 |
|---|---|---|
| Queued tasks under `147-F` | 28 | 28 |
| Queued-to-queued executable edges | 47 | **48** |
| Shipment `130-S` members | 29 | 29 |
| Ready set | `{147.001-T}` | `{147.001-T}` (sole root) |
| Historical total edges | 48 | **49** (48 executable + archived `147.010-T -> 147.009-T`) |

One edge added: `147.028-T -> 147.004-T`, because U2g reuses U2c's `duplicate:` reporting form,
mirroring the existing `U2c -> U2e` edge. Graph verified acyclic by Kahn topological sort — all 28
nodes ordered, sole root `147.001-T`.

## Validation

* `backlogit --cwd . dep add 147.028-T 147.004-T --type blocks` — applied
* `backlogit --cwd . sync` — 1188 artifacts indexed, 0 parse failures
* `backlogit --cwd . doctor` — no orphans, no duplicate IDs
* `backlogit --cwd . docs lint` — 0 violations on changed docs
* `make md-lint` (markdownlint-cli2, P-008 MD001/MD025/MD041) — 0 issues
* Kahn topological sort over the 48 executable edges — acyclic, 28/28 ordered
* `docs/memory/` footprint after adding this file: recomputed and recorded below

## Compaction check

`docs/memory/` held 39 files / 327.5 KiB before this cycle. Adding this artifact takes it to
**40 files / 337.9 KiB**. The mandatory triggers are **more than** 40 files or **more than** 500 KB;
exactly 40 is not more than 40, and 337.9 KiB is far below 500 KB. **No compaction required.**
`compact-context` was deliberately not invoked.

## Open items

No P0 or P1 finding remains open for this bounded scope. Two P2 security-lens items are recorded as
stash follow-ups (`E429A031`, `35A27CD0`) with explicit non-blocking rationale.

## Next safe action

The plan is ready for a **fresh, independent cycle-15 review**. Ship or the operator must query
live PR #377 state — head, checks, reviews, unresolved threads — before any merge-readiness claim.
Stage stopped at its Role Boundary: push, review replies, thread resolution, shipment claim, and
merge remain forbidden to it. The hard merge gate is unchanged: `147.018-T` must land in the same
merge commit as `147.007-T`, `147.008-T`, and `147.009-T`, inside the single `130-S` PR. The halt
condition is unchanged: if `147.009-T`'s paired accept/refuse assertion cannot pass, halt rather
than weaken it.
