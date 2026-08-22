---
chunk_strategy: h1-h2-h3
description: 'Stage session memory for the resumed staging run that closed the plan-review gate and harvested feature 146-F and shipment 129-S.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-08-21/success-shaped-evidence-loss-stage-memory.md
title: 'Stage session memory — success-shaped evidence loss staging (Steps 4 through 6)'
---

## Session scope

Resumed an interrupted Stage session. Steps 0 through 3.3 were completed by a prior run; this session
resumed at Step 4 and completed Steps 4, 5, 5.5, 5.6, and 6. No deliberation or planning was redone.

Operator constraints honored throughout:

* Planning and backlog scope only. No source, test, config, workflow, template, or runtime code was
  modified. No build system, test suite, or linter was run.
* No branch, worktree, commit, push, or pull request was created. No shipment was claimed or shipped.
* `C:\Source\GitHub\autoharness` was treated as strictly read-only. **No command was run in or against
  it**, and its stash was neither edited nor archived.
* Unrelated untracked Azure DevOps artifacts in the root worktree were left untouched.
* The separate closure worktree was not touched and no new worktree was created.

## Degraded visibility (recorded, not silently skipped)

Intercom was unavailable for the entire session. No milestone was broadcast to a remote channel and no
approval was routed through it. Remote operator visibility is degraded; Ship must not assume any remote
operator observed these decisions. Only safe, non-destructive planning and backlog operations were
performed. This is recorded as a `documented-deviation` in the plan's Constitution Check.

## Step 4 — plan-review gate

Final record: **`dispatch_mode: multi-agent-dispatch`, `decision: PASS`** (gate run 8).

No `operator_authorization` was required or asserted, because the decision is PASS rather than
ADVISORY. **No P0 or P1 finding was waived at any point.**

All seven manifest personas were dispatched as independent sub-agents in every gate run and all seven
returned findings, so the `multi-agent-dispatch` label is valid under the skill's terminal-states rule.
Cross-model diversity was applied to the three cross-model personas (Architecture Strategist,
Agent-Native Parity Reviewer, Security Lens Reviewer), each on a different model from the caller and
from one another.

| Run | Decision | Merged findings |
|---|---|---|
| 1 | FAIL | 1 P0, 19 P1, 21 P2, 15 P3 |
| 2 | FAIL | 0 P0, 15 P1, 22 P2, 18 P3 |
| 3 | FAIL | 0 P0, 7 P1, 18 P2, 12 P3 |
| 4 | FAIL | 0 P0, 6 P1, 9 P2, 10 P3 |
| 5 | FAIL | 0 P0, 2 P1, 12 P2, 12 P3 |
| 6 | FAIL | 0 P0, 2 P1, 8 P2, 12 P3 |
| 7 | FAIL | 0 P0, 0 P1, open P2s |
| 8 | **PASS** | 0 P0, 0 P1, 0 open P2 |

Every run is recorded in full in the plan as its own `## Plan Review` section, so the remediation trail
is auditable. Stage Step 4's two-re-entry-cycle limit governs re-invocation after a FAIL verdict is
**returned to the operator**; no FAIL was ever returned — each run's findings were remediated in-session
and the gate re-run, which is the plan-review skill's own revise-and-re-review path.

### Highest-value defects the gate caught

1. **P0 (Learnings Researcher, run 1)** — U2's custom `UnmarshalJSON` on `CheckpointContext` changes the
   *observable* behavior of `ParseCheckpoint` on every read path even though the function itself is
   unmodified. Degenerate `context` values could flip an existing on-disk file from parseable to corrupt,
   tripping the plan's own primary rollback trigger. Fixed by restating R4 as observable leniency and
   adding pre-U2 golden-table guards (U3b, U3c).
2. **Containment downgrade (Go + Security, run 1)** — `ErrPathEscapesWorkspace` is raised **inside**
   `decodeDoc`, not only in `collectInScopeDocs`. The original "replace `return nil, err`" instruction
   would have converted a NON-NEGOTIABLE Principle III control into a success-shaped lint finding. Fixed
   with a sentinel-discriminated three-way split plus a dedicated containment guard unit.
3. **HTML-escaping regression (Go, run 6)** — once `CheckpointContext` implements `json.Marshaler`, a
   `json.Marshal`-based implementation would ship an escaped ampersand inside `context`, regressing the
   guarantee shipped as 137-F / 123-S. The existing guard could not catch it, because it exercises only a
   top-level field still encoded by the outer escape-free encoder. Fixed by pinning `emit()` to
   `jsonutil.MarshalReadable` and adding an escape-guard scenario.
4. **Committed live-corpus fixture (Constitution + Security)** — the plan briefly directed an
   implementer to commit a redacted copy of the live checkpoint corpus. `git revert` cannot purge a
   committed blob and Principle XI forbids history rewriting, so a missed secret would have been
   unrecoverable. Replaced with hand-written synthetic fixtures.
5. **Non-compiling red harnesses (Constitution + Go, runs 3–5)** — several harness units asserted
   identifiers their green units create, making the Principle II red *unobservable* rather than merely
   weak. Fixed with two declaration-only prelude units (U0a, U0b), split by track so the two source
   entries share no commit.
6. **Process defect worth remembering** — at run 4 several run-3 remediations had landed in the plan's
   summary tables but silently **not** in the unit bodies, because a scripted multi-line PowerShell
   `.Replace()` mismatched CRLF line endings and no-opped without error. The Constitution Reviewer
   caught it. Subsequent rounds verified every edit landed and switched to the `edit` tool for
   multi-line changes.

## Step 5 — harvest

P-003 validation passed before any item was created: the source document exists, the plan references it,
the covering feature references both, every task references its parent feature, and every task carries at
least one concrete acceptance criterion.

A duplicate check was run immediately before harvest: the backlog queue was empty and a SQL search for the
source IDs and title returned nothing.

* **Covering feature: `146-F`** — "Eliminate success-shaped evidence loss on governed diagnostic paths",
  `queued`, priority `high`. Carries `Provenance`, `Gate`, `Boundaries`, and `Approvals` sections plus
  labels `src-3C7AAC71` and `src-90F2A9F8`.
* **22 tasks: `146.001-T` through `146.023-T`** (`146.003-T` was a duplicate from a failed script run and
  was deleted). Each maps 1:1 to a plan unit, is width-isolated to a single skill domain, is scoped inside
  the two-hour envelope, and carries red-first acceptance criteria.

| Unit | Task | Domain | Unit | Task | Domain |
|---|---|---|---|---|---|
| U0a | `146.001-T` | code | U5a | `146.013-T` | tests |
| U0b | `146.002-T` | code | U5b | `146.014-T` | tests |
| U1a | `146.004-T` | tests | U6 | `146.015-T` | code |
| U1b | `146.005-T` | tests | U7a | `146.016-T` | tests |
| U2 | `146.006-T` | code | U7b | `146.017-T` | tests |
| U3a | `146.007-T` | tests | U8 | `146.018-T` | code |
| U3b | `146.008-T` | tests | U8b | `146.019-T` | code |
| U3c | `146.009-T` | tests | U9a | `146.020-T` | tests |
| U3d | `146.010-T` | tests | U9b | `146.021-T` | tests |
| U4 | `146.011-T` | code | U10a | `146.022-T` | docs |
| U4b | `146.012-T` | code | U10b | `146.023-T` | docs |

Domain split: 12 tests-only, 8 code-only, 2 docs-only.

## External source disposition

| Source ID | Disposition |
|---|---|
| `3C7AAC71` | **Harvested** into Track A of `146-F` (U0a, U1a, U1b, U2, U3a, U3b, U3c, U3d, U4, U4b, U5a, U5b, U6) |
| `90F2A9F8` | **Harvested** into Track B of `146-F` (U0b, U7a, U7b, U8, U8b, U9a, U9b) |
| `84D8E6AB` | **NOT harvested. No duplicate item created.** Already shipped as archived feature `143-F` / shipment `127-S`, with prevention hardening in `144-F` / shipment `128-S`. The deliberation proved this; creating an item would have duplicated shipped work. |

The exact source IDs are preserved and queryable in three places: the feature's `Provenance` section, the
feature's labels (`src-3C7AAC71`, `src-90F2A9F8`), and each task's `Provenance` section. The disposition is
additionally recorded as a `stage`-authored comment on `146-F`.

Because the source workspace is read-only, the normal Step 5.6 stash-archive action was **deliberately not
performed**. The comment on `146-F`, the feature's `Provenance` section, and this memory file are the
durable record instead.

## Step 5.5 — shipment

* **Shipment `129-S`** — "Eliminate success-shaped evidence loss on governed diagnostic paths",
  status `queued`, priority `high`. **Not claimed, not shipped.**
* Manifest: 23 items — covering feature `146-F` first, then all 22 tasks in dependency-safe topological
  order. `covering_feature` resolves to `146-F`. `skipped` is empty.
* No competing shipment existed: `list_shipments` showed no `queued` or `active` shipment; every prior
  shipment is `done`.

## Dependency graph

33 `blocks` edges were registered through the backlogit dependency operation, transcribed from the plan's
graph. Verified readiness: exactly the two prelude tasks have zero unmet upstream dependencies, and every
other task has at least one — which both matches the plan and proves the graph is acyclic.

```text
U0a --> U1a, U1b, U3a, U3b, U3c, U3d, U5a, U5b
U0b --> U7a, U7b, U9b
U1a, U1b, U3b, U3c --> U2
U3a, U2 --> U4 --> U4b (also <-- U3d)
U4b, U2, U5a, U5b --> U6
U7a, U7b, U9b --> U8 --> U8b --> U9a
U6, U9a, U8b --> U10a
U6, U9a --> U10b
```

## Approval gates Ship must clear

Four ProposedActions carry an approval or block condition. **None was approved by Stage.**

| Action | Risk | Condition |
|---|---|---|
| PA-3 | `high` | Approval required. Blocks U8 (`146.018-T`) and transitively U8b (`146.019-T`). |
| PA-5 | `destructive` | **Operator-only, `ActionResult: blocked`.** Refreshing the pinned out-of-tree binary. No agent may perform it. |
| PA-6 | `high` | Approval required before any rollback revert. |
| PA-8 | `high` | Approval required. Blocks U2 (`146.006-T`). |

With intercom down, approvals must be obtained by direct operator prompt and recorded by flipping each
`ActionResult` to `approved` in the plan.

## Files changed this session

* `docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md` — extensively remediated across six
  rounds and appended with eight `## Plan Review` sections.
* `.backlogit/queue/` — feature `146-F`, tasks `146.001-T` through `146.023-T`, shipment `129-S`,
  dependency edges, and two `stage` comments on `146-F`.
* `docs/memory/2026-08-21/success-shaped-evidence-loss-stage-memory.md` — this file.

Not touched: the deliberation record, any source or test file, the Azure DevOps artifacts under
`docs/decisions/2026-08-20-*`, `docs/exec-plans/2026-08-20-*`, and `docs/memory/2026-08-20/`, the closure
worktree, and the entire `autoharness` workspace.

## Handoff to Ship

Ship should start from **shipment `129-S`**. The two ready entry points are `146.001-T` (U0a) and
`146.002-T` (U0b); everything else is dependency-gated behind them. Before starting U2 or U8, obtain
operator approval for PA-8 and PA-3 respectively. Verification must run through
`go run ./cmd/backlogit` from the feature-branch HEAD — never the pinned `C:\Tools\backlogit.exe`, which
would execute pre-fix code on both sides of the before/after comparison the plan calls its primary
rollback signal.

## Open items

* Nothing was published to `origin/main`; Stage does not own that gate.
* Eight scope-boundary follow-ups are recorded in the plan's `Scope boundaries and recorded follow-ups`
  table, owned by Stage and to be created as backlog items in a future staging session.
* Four advisory P3 items from gate run 8 remain unactioned and are listed in the final `## Plan Review`
  section. None affects executability.
