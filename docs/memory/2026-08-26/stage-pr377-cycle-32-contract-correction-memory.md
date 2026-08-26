---
chunk_strategy: h2
description: "Stage PR #377 cycle-32 contract correction — the fresh local review cycle 31 blocked on, run against cycle 31's own 1.19.0 contract: two P1 defects (Step 4.6's empty tolerated-red set was unsatisfiable from wave 4; the status model cited a five-token set that exists in no source) and three P2 defects (wrong M freeze anchor, unverifiable scheduler validation, discretionary green-package selection), all remediated in-pass; workflow-policies 1.20.0; new tracked read-only scheduler simulation at 84/84 assertions; gate set to FAIL pending fresh local review"
doc_type: memory
schema_version: "1.0"
source: cycle-32-contract-correction-session
title: "PR #377 Cycle 32 Contract Correction Memory"
---

# PR #377 Cycle 32 Contract Correction Memory

**Date**: 2026-08-26
**Agent**: Stage (prompt-builder scope)
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 32 (the fresh local review cycle 31's `FAIL` blocked on, plus in-pass remediation)
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `d57bbd8a456383c0a98283f2637642b34ffa5c54`
**Shipment**: `130-S` (queued — **not** claimed, **not** shipped)
**Scope honoured**: no subagents, no push, no merge, no production Go

## Mandate and outcome

Cycle 31 closed six PR #377 P1 threads and set the gate to `FAIL` with
`pending: fresh-local-review-required`, on the honest grounds that a cycle rewriting the harness
lifecycle, the wave scheduler's completion semantics, and the per-task green gate cannot certify
itself. This cycle **is** that review. It found **two P1 and three P2 defects in the 1.19.0
contract cycle 31 produced** and remediated all five in-pass.

| ID | Sev | Defect | Fix |
|---|---|---|---|
| G1 | P1 | Step 4.6's "tolerated-red set is empty" is false from wave 4 | `open_red_deliverables_k` + conditional unfiltered suite + bounded deferral |
| G2 | P1 | Status model cited a five-token set that exists nowhere | Configured executable / terminal-success / unsupported partition |
| G3 | P2 | `M` frozen at Step 0.5, which never enumerates members | Anchored to Step 3 + defined non-shipment fallback |
| G4 | P2 | Mandatory scheduler validation was a session claim | Tracked read-only fixture + PowerShell runner, 84/84 |
| G5 | P2 | "any package that was already green" was discretionary | Task-declared closed list `green_regression_cmds` |

## G1 — open red across waves

Cycle 31 relocated the full suite to a new Ship Step 4.6 and justified an **empty** tolerated-red
set with "every current-wave harness is now green because every member has been built". That
premise holds only for waves whose members are all build-to-green tasks. Three members of this
release unit are **red deliverables** — tasks whose declared deliverable *is* a failing harness,
turned green by a later task — and two of them, `147.016-T` / U8b and `147.042-T` / U3c, complete in
**wave 4**. An unfiltered `go test ./...` at wave 4's convergence gate would fail on exactly the
artifacts the plan asked those tasks to produce, and Step 4.6 forbids advancing on a failing gate,
so the release unit would deadlock at wave 4. That is the same shape of unsatisfiable gate cycle 31
fixed one level up — the fifth instance of composition failure in the P-002.6 design.

**The correction.** `open_red_deliverables_k` is the closed set of `(task, selector)` pairs for
completed red deliverables at least one of whose declared green-makers is not yet `done`. It is
derived **mechanically** from a new `red-deliverable-contract` block in the task body
(`red_deliverable`, `red_deliverable_reason`, `red_selector_command`, `green_maker_tasks`,
`green_maker_closes_wave`) and fails closed with `WAVE_RED_MAPPING_UNRESOLVED` when the mapping is
missing, empty, unknown, self-referential, or not strictly later. Convergence now **always** runs
the repo-wide compile check, `go vet`, lint, format, and the wave's closed list of declared scoped
commands — with a red deliverable's selector required to still be **RED**. The **unfiltered** suite
runs if and only if the open-red set is empty, and is mandatory at final closure.

**Defer, not classify.** Both options in the finding were evaluated and deferral is the simpler
robust one. A classified full run would have to admit failures matching the open-red selectors — but
Go aborts a package on a build error, panic, or timeout, so a genuine unrelated failure inside a
package that also holds an open red produces **no** `--- FAIL:` line to classify and is silently
absorbed. That is precisely the hidden-unexpected-failure mode the gate exists to prevent. Deferral
has no classification step: what stays repo-wide (compile, vet, lint, format) cannot be reddened by
a designed red at all, every declared scoped command runs and must pass, the deferral is recorded
with its selectors and unclosed green-makers, and it is bounded by `green_maker_closes_wave` and
`WAVE_OPEN_RED_UNCLOSED`. On this schedule it spans waves 4–12 and discharges at **wave 13**.

A sub-finding closed alongside: `archived` satisfies a *dependency* but does **not** discharge a
*green-maker* obligation, because a member may be archived as a descope
(`isDescopeEligibleStatus`). A descoped green-maker leaves its entry open and the loop halts rather
than retiring a red no one made green.

## G2 — the status model was cited, not read

P-002.6 1.19.0 said `status(t)` "is exactly one of the five recognized tokens `queued`, `active`,
`blocked`, `done`, `archived` (`internal/models/artifact.go:17-22`)". Those lines declare **ten**
`ArtifactStatus` constants, and `.backlogit/config.yaml` `fields.status.values` enumerates the same
ten. Five real lifecycle tokens — `review`, `accepted`, `rejected`, `shipped`, `abandoned` — had no
stated disposition, and the single clause about unknown tokens folded them into
`WAVE_MEMBER_BLOCKED`, whose report promises a `blocked_reason` and a `return_blocked` record that
none of them has.

The model is now **read from the configured sources at Step 3**: the workspace catalog
(`.backlogit/config.yaml`), the registry mapping (`.autoharness/backlog-registry.yaml`
`status_values` = `queued`, `active`, `done`, `blocked`), and the actual archive lifecycle
(`ArchiveItem` writes `status: archived`). `M` partitions into configured **executable**
(`queued`, `active`, `blocked`), configured **terminal-success** (`done`, `archived`), and
**`unsupported`** — defined as the *complement*, so the partition is total by construction rather
than by enumeration and no future token can occupy an unexamined state. Unsupported halts with
`WAVE_STATUS_UNSUPPORTED` naming each ID and token; an unreadable, empty, or self-inconsistent
model halts with `WAVE_STATUS_CATALOG_UNAVAILABLE` rather than falling back to a built-in list.

## G3 — freeze anchor and the non-shipment path

`M` was said to be frozen "at Ship Step 0.5 shipment intake". Step 0.5 loads the shipment,
validates membership, creates the branch, and claims — it never enumerates members at every status,
and it is conditional on `features.shipments: true`. The anchor moves to **Step 3**, the step that
does the enumeration; Step 0.5 now states in one sentence that it does *not* freeze `M`; and the
non-shipment path is defined rather than left implicit — freeze from the covering release unit's
declared child enumeration, recorded verbatim, with `WAVE_MANIFEST_UNAVAILABLE` when neither source
is available or the two disagree.

## G4 — the validation became an artifact

P-002.6 requires a blocked-injection replay before a wave schedule may be relied on, but cycle 31
satisfied it with an in-session simulation described only in a review record. A paragraph cannot be
re-run. The replay is now `tests/simulation/wave-scheduler-contract.json` (a tracked fixture
mirroring the live queue) plus `scripts/wave-scheduler-sim.ps1` (a pure, read-only runner). No Go
was added — the test tree is Go-only and this cycle's scope forbids production Go — so the artifact
uses the PowerShell already present under `scripts/` and a JSON fixture any reader, CI step, or
`jq` invocation can parse. `-VerifyAgainstQueue` re-derives members, statuses, edges, exemption
labels, and red-deliverable contracts from `.backlogit/queue` and fails on drift.

## G5 — discretionary package selection withdrawn

Ship Step 4.3 and `build-feature`'s post-loop gate both allowed "any package that was already green
before this wave began". That set was never enumerated, never recorded, and could be narrowed to
nothing or widened into a sibling's red with no artifact showing it — implementer judgement inside
a gate that had just been tightened elsewhere. Replaced by `green_regression_cmds`: a
task-declared, closed, diffable list, empty unless the task declares otherwise.

## Simulation evidence (84/84 assertions PASS, 16 scenarios)

`pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` → `WAVE_SIM_OK`, exit 0.

* **Baseline**: `COMPLETE`, **18 waves**, 42/42, sizes 2,2,4,5,4,4,3,2,3,3,1,2,2,1,1,1,1,1, **0**
  stalls, **0** compile-order violations, one snapshot per wave.
* **Persistent red mapping**: wave 4 **advances**; open red after wave 4 = `{147.016-T, 147.042-T}`,
  after wave 6 = `{147.016-T, 147.035-T, 147.042-T}`; entries close at waves 7 / 8 / 13; unfiltered
  suite at waves 1,2,3,13-18; deferred at 4-12; compile gate at all 18; **0** hidden failures.
* **Blocked injection**: `WAVE_MEMBER_BLOCKED` at wave 1, impact **26**, member retained, no
  completion. Mid-run variant halts at wave 5.
* **Unsupported status**: `review`, `abandoned`, and an off-catalog token each halt with
  `WAVE_STATUS_UNSUPPORTED`; catalog-unavailable and catalog-disagrees halt with
  `WAVE_STATUS_CATALOG_UNAVAILABLE`.
* **Active residual**: `WAVE_NO_PROGRESS` (detail `active residual`).
* **Cycle injection**: `WAVE_CYCLE_DETECTED` at schedule construction.
* **Sibling-red wave 4**: old repo-wide gate **0/5**, new scoped gate **5/5**, **0** full-suite runs
  inside per-task loops.
* **Non-frozen-`M` control**: `WAVE_NO_PROGRESS` at wave 10 with the member dropped.
* **Mapping fail-closed**: missing and ambiguous green-makers both halt
  `WAVE_RED_MAPPING_UNRESOLVED`; a descoped green-maker halts `WAVE_OPEN_RED_UNCLOSED` at wave 8.

## Validation evidence

| Gate | Result |
|---|---|
| Wave scheduler simulation | `WAVE_SIM_OK` 84/84, exit 0, fixture verified against the live queue |
| Markdown P-008 repo-wide | 0 issues |
| `backlogit docs lint` | `valid: true`, 0 violations |
| `backlogit sync` | 0 parse failures |
| Topology | 42 tasks / 104 executable edges / 43 shipment members — unchanged |
| `go build ./...`, `go vet ./...` | exit 0, exit 0 |
| `go test ./...` | exit 0 |
| Production Go modified | **none** |

## Gate state

`cycle: 32`, `decision: FAIL`, `pending: fresh-local-review-required`, `push_allowed: no`,
severity P0=0 / P1=2 (both remediated in-pass) / P2=3 (all remediated in-pass) / P3=0, topology
SOUND and unchanged. `workflow-policies.md` **1.19.0 → 1.20.0**.

**Why FAIL again.** The same argument cycle 31 made now applies to cycle 32's own changes: this
cycle rewrote the scheduler's status model, its manifest freeze anchor, and its convergence
semantics, and the corrected contract has been reviewed only by the session that authored it. The
difference is that the review obligation is now partly mechanized — the simulation re-checks the
behavioural claims on demand, so the next review can concentrate on whether the *contract* is right
rather than on whether the *simulation* was really run.

## Lessons

* **A fix that composes a new gate needs a composition check against the gates already in the
  path.** This is the fifth defect of that shape in P-002.6 (vacuous delta gate, missing `blocked`
  disposition, unsatisfiable wave order, unsatisfiable convergence gate, and the status set that
  examined only half the vocabulary). Cycle 31 even *named* this lesson and then reproduced it one
  level down, because it validated the new Step 4.6 against the wave it was invented for rather
  than against every wave the schedule actually contains.
* **Cite the configuration, not the constant you remember.** "Five recognized tokens
  (`artifact.go:17-22`)" was written with a file-and-line citation that made it look verified. The
  file declares ten. A citation is evidence only if the reader can see the claim in the cited text.
* **Define residual classes as complements, not as lists.** `unsupported = M \ (executable +
  terminal_success)` cannot go stale when a status is added; an enumerated list of "other tokens"
  can, and would fail open exactly when the vocabulary grows.
* **Anchor a freeze to the step that produces the thing being frozen.** Step 0.5 was named because
  it is where the shipment is claimed, not because it enumerates members. An anchor that points at
  a step which never computes the value is a contract that cannot be executed as written.
* **A validation you cannot re-run is a claim, not a validation.** The blocked-injection replay was
  mandatory in policy and evidenced only in prose for two cycles. Turning it into a tracked fixture
  plus a read-only runner cost one file each and converted a recurring assertion into a
  re-checkable fact.
* **"Already green" is a judgement, not a scope.** Any gate clause that lets the executor choose
  the set it will be judged against is not a gate. Closed, declared lists are the only form that
  survives review.

## Next actions

1. **Fresh local plan review of the cycle-32 contract** — required before push; this is what the
   FAIL blocks on.
2. Push `chore/stage-130-s` (blocked on 1).
3. Reply to and resolve the PR #377 threads against the new HEAD (blocked on 2).
4. Operator merge approval (P-014) — not requested.
