---
chunk_strategy: h2
description: "Stage PR #377 cycle-17 formal decomposition — five DAG partitions, eleven new tasks, all eight cycle-16 blockers dispositioned"
doc_type: memory
schema_version: "1.0"
source: cycle-17-decomposition-session
title: "PR #377 Cycle 17 Formal Decomposition Memory"
---

# PR #377 Cycle 17 Formal Decomposition Memory

**Date**: 2026-08-25
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 17 (formal decomposition, executing the cycle-16 `restage_recommendation`)
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**Shipment**: `130-S` (queued — **not** claimed, **not** shipped)

## Outcome

Executed the formal decomposition the cycle-16 plan-review gate required. The remaining work is
re-partitioned into five DAG partitions, eleven new tasks were created, five were retitled, the
dependency graph was rewired, and every normative section of the plan was rewritten against the new
structure. All eight cycle-16 blockers (1 P0, 7 P1) are dispositioned as closed.

Stage's Role Boundary was observed throughout: no production Go, test, or configuration file was
touched; no build, test, or lint of Go code was run; no push, PR action, shipment claim, or Ship
handoff occurred.

**The cycle-16 gate is not cleared.** It remains `decision: FAIL` until a fresh, independent plan
review is dispatched against the decomposed plan.

## Partition mapping

| # | Partition | Units |
|---|---|---|
| 1 | Foundation diagnostics and conformance | U1, U1b, U1c, U1d, U2, U2b, U2c, U2d, U2e, U2g, U2h |
| 2 | Guarded rewrite seam | U11, U12, U13, U14, U2f |
| 3 | Declarations and genuine RED harness order | U15, U8b |
| 4 | Implementation plus MCP/CLI/instruction contracts | U3, U3b, U4, U17, U5, U6, U6d, U6b, U6c, U7, U7d, U7e, U7b, U7c, U8, U16, U8c, U9, U9b |
| 5 | Runtime verification and closure | U10, U10b, U10c |

Partition order is a hard execution order. The partitions are execution phases inside the single
`130-S` release unit, not separate release units: they share one branch, one merge commit, and the
U9/U9b hard merge gate, so splitting them would violate that gate.

## Blocker disposition

| ID | Severity | Disposition |
|---|---|---|
| H1 | P0 | Closed. `internal/events` publishes a structured `RemediationIntent` (U1d / `147.032-T`) and never a command string. The CLI renderer U16 (`147.039-T`) is the sole command surface and always binds `--cwd`, passes a bare filename, prints the A4c approval / preimage / no-clobber preamble, and refuses to render when quoting would be required. U9b gains a normative rule forbidding executable remediation, repair, restore, or sweep text plus an acceptance grep. `RemediationCommand` is deprecated in place; removal is stash `F1A47C02`. |
| H2 | P1 | Closed. U15 (`147.038-T`) declares `CheckpointReadResult` / `GetCheckpointResult` in partition 3; U8b (`147.016-T`) is rewritten as a declarations-only harness. Its five former implementation prerequisites were removed and seventeen partition-4 units now depend on it. The already-green schema-invalid `get` assertions were reclassified as declared regression guards. |
| H3 | P1 | Closed. U10c (`147.041-T`) owns context-duplicate cross-surface runtime verification, the abandoned-resolve MCP handler assertion, and scratch teardown. U10 and U10b stay at three rows each. All runtime evidence is persisted to the tracked `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`. |
| H4 | P1 | Closed. U1b returns `BoundedFieldPathSet{Paths, Truncated, OmittedPaths, TruncatedPaths}` with raw paths, no `strconv.Quote`, no `+N more` element, and UTF-8 rune-boundary-safe caps. U1c (`147.031-T`) owns the only quoted rendering. U7's `unknown_fields` gains three sibling truncation scalars. |
| H5 | P1 | Closed. U7 item 3 no longer says the two unreachable `domainError` rows were "moved to U7e" — they were deleted. U7e is retitled and carries a sole-ownership clause forbidding reintroduction. U7d's split note corrected the same way. |
| H6 | P1 | Closed. U7b and U7c key on the registered identifiers, verified against `mcplib.NewTool` literals at `internal/mcp/tools.go:177`, `:188`, `:195`, `:209`, `:218`. Description bodies corrected too. |
| H7 | P1 | Closed. U17 (`147.040-T`) changes the `AbandonCheckpoint` validation wrap to multi-`%w`. Constitution Check Principle I moves from `deviation (documented)` to `pass`, and the deviation row is withdrawn. |
| H8 | P1 | Closed. U11–U14 build a guarded rewrite seam (`events.RewriteCheckpointFile`) as the I1 mechanism. Quarantine's `moveNoReplace`, `CleanupCheckpoints`' rename, and `CreateCheckpoint`'s writes stay explicitly outside it. U2f is retitled as a supplemental caller-set regression guard with an honest bound; nothing depends on it and a blocked U2f no longer blocks the release unit. |

## Additional corrections

* **U2h** (`147.033-T`) closes a false negative: `encoding/json` routes every top-level key
  satisfying `strings.EqualFold(name, "context")` to the modeled field, so a lone `Context` was
  never inspected by U2g's literal-spelling walk.
* **U2g red timing** separated: the red gate is `-run '^TestU2g_Duplicate'` (two failing cases);
  the open-namespace preservation guard is green on landing and excluded from the red count.
* **A4c precondition split by operation class**: in-place rewrite uses preimage plus SHA comparison
  and must **not** assert absent-destination; archive move asserts absent destination and
  no-clobber; declared intentional-collision rows expect a refusal.
* **U10b evidence-pair claim narrowed**: `moveNoReplace` is genuinely no-clobber, but the sidecar
  uses `atomicfile.WriteFileAtomic` (an idempotent upsert) and therefore replaces. Sidecar
  hardening is stash `9C4B10D7`.
* **Live-corpus contradiction removed**: blocked-path handling no longer tells an operator to
  quarantine one of the nine live legacy files under A4c. A5 stays `abandoned` and forbidden.
* **Closure signals scoped** to conforming, active, undisposed checkpoints, so pre-existing
  `ErrCheckpointNotActive` and `ErrCheckpointCannotResolveAbandoned` refusals are not misread as
  incidents.
* **Final gates rewritten** as executable Windows PowerShell, branch-wide, with explicit
  empty-output assertions. Gate 4b adds `go run golang.org/x/tools/cmd/goimports@v0.39.0 -l .`
  (version pinned from `go.sum`, module-independent, halts rather than skips on a cold offline
  cache). Gate 9 captures `git rev-parse --short HEAD`, reproduces the Makefile `LDFLAGS` shape
  (`Makefile:6-8`), and throws on SHA inequality.
* **P-016** recorded as a Ship execution precondition, not a plan defect: an unrelated linked
  worktree on `chore/121-s-closure` must be finished or removed by its owner before Ship claims
  `130-S`. Stage did not touch it.
* **Adjacent intake linked, not absorbed**: stash `B2657A3E` (malformed / truncated V1 JSON treated
  as legacy and written) and stash `E429A031` (create-boundary duplicates) are both create-boundary
  concerns; this plan governs the stored-document read and rewrite boundaries.

## Topology

| Measure | Cycle 16 | Cycle 17 |
|---|---|---|
| Queued tasks under `147-F` | 29 | 40 |
| Queued-to-queued executable edges | 52 | 98 |
| Shipment `130-S` members | 30 | 41 |
| Ready roots | `{147.001-T}` | `{147.001-T, 147.032-T}` |
| Historical total edges | 53 | 99 |

Eleven tasks created (`147.031-T` … `147.041-T`), five retitled (`147.015-T`, `147.016-T`,
`147.021-T`, `147.029-T`, `147.030-T`), none archived, no ID renumbered. Eight superseded edges
removed, fifty-four added.

## Files modified

| File | Change |
|---|---|
| `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` | Requirements trace, Implementation Units (partition table, red-verification table, eleven new unit sections, ten rewritten unit sections), Dependency Graph regenerated, Decisions, Risks, Constitution Check, Runtime Verification and Closure, gate sequence, I1/I2 invariants, risky actions, and the cycle-17 remediation appendix |
| `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md` | Scope boundary synchronized with the guarded seam, structured remediation intent, and the create-boundary deferral |
| `.backlogit/queue/147-F.md` | Plan-review state, cycle-17 decomposition summary, partition list, P-016 precondition, task inventory, and topology paragraph |
| `.backlogit/queue/147.031-T.md` … `147.041-T.md` | Eleven new tasks |
| `.backlogit/queue/147.011-T.md`, `147.012-T.md`, `147.013-T.md`, `147.014-T.md`, `147.015-T.md`, `147.016-T.md`, `147.019-T.md`, `147.021-T.md`, `147.022-T.md`, `147.024-T.md`, `147.026-T.md`, `147.027-T.md`, `147.028-T.md`, `147.029-T.md`, `147.030-T.md` | Bodies and acceptance criteria resynchronized; five of these also retitled |
| `.backlogit/queue/130-S.md` | Eleven new members |
| `.backlogit/checkpoints/checkpoint-20260824-191617.json` | `phase`, `updated_at`, `task_ids`, `topology`, `plan_review`, `review_remediation`, `stash_state`, `memory_path`, new `partitions` and `p016_precondition` context keys, and `resume_hint` |
| `.backlogit/memories.json` | Canonical Stage handoff entry `stage-d3ce9e81-checkpoint-toplevel-keys` refreshed |
| `.backlogit/logs/*.jsonl` | Append-only mutation history for every touched artifact — written by the tool but **git-ignored** (`.gitignore:3`), so it does not appear in the commit |
| `.backlogit/hooks_queue.jsonl` | Tool-managed hook event queue, appended by the backlog mutations |
| `docs/memory/2026-08-25/stage-pr377-cycle-17-formal-decomposition-memory.md` | this file |

## Validation

* `backlogit --cwd . sync` — indexed with 0 parse failures
* `backlogit --cwd . query` — 40 queued tasks, 98 queued-to-queued executable edges, ready set
  exactly `{147.001-T, 147.032-T}`
* `backlogit --cwd . shipment get 130-S` — 41 members, covering feature `147-F` present
* Kahn topological sort over the executable subgraph — 40/40 nodes ordered, acyclic
* `backlogit --cwd . checkpoint get checkpoint-20260824-191617.json` — `valid: true`
* `backlogit --cwd . doctor` — no new orphans or duplicate IDs introduced
* `backlogit --cwd . docs lint` — in-scope frontmatter clean
* `markdownlint-cli2` over the changed Markdown files — clean

## Compaction

`compact-context` was evaluated and **not** triggered. Thresholds after this file lands: 5 PR-377
memory artifacts for this feature (limit 10, cycles 2-14 were already compacted in cycle 16),
32 files under `docs/memory/` (limit 40), and roughly 270 KB total (limit 500 KB). No mandatory
trigger is met, and cycle 16 already ran the archive-only consolidation.

## Next safe action

Dispatch a **fresh, independent plan-review gate** against the decomposed plan. Do **not** push this
branch. Do **not** hand shipment `130-S` to Ship until that review passes. Before Ship claims the
shipment, verify the P-016 precondition: exactly one active implementation worktree for this
release unit.
