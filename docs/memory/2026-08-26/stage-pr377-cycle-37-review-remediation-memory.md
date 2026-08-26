---
chunk_strategy: h2
description: "PR #377 cycle-37 Copilot review remediation: earlier-wave open-red re-confirmation at the Step 4.6 convergence gate, the U14 / U14b caller-migration split, and the stale PR description refresh"
doc_type: memory
schema_version: "1.0"
source: cycle-37-review-remediation-session
title: "PR #377 Cycle 37 Review Remediation Memory"
---

**Date**: 2026-08-26
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 37
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `88ced429218f31ef424e24f149471522b771a6c6`
**Fixing commit**: `84d53b7919cf5061e887bd4482ff92b2af37fc6d`
**Shipment**: `130-S` (queued — not claimed, not shipped)
**Scope honored**: planning, prompt, policy, backlog, simulation-fixture, and script artifacts only;
no subagent, no merge, no shipment claim, no worktree deletion, no Go source change

## Threads handled

**Wave 1** — three unresolved Copilot threads raised against `88ced429`, all classified valid.

| Thread | Path | Disposition |
|---|---|---|
| `PRRT_kwDORzozKM6cjdUQ` | `.github/agents/.ship.agent.md` | Fixed — convergence gate widened to earlier-wave open reds |
| `PRRT_kwDORzozKM6cjdUy` | `.backlogit/queue/147.037-T.md` | Fixed — U14 caller migration split by verb into U14 and U14b |
| `PRRT_kwDORzozKM6cjdVm` | `docs/memory/2026-08-26/…cycle-35…-memory.md` | Fixed — PR description refreshed against the current HEAD |

**Wave 2** — five threads raised against `84d53b79`; four valid, one declined on a stale premise.

| Thread | Path | Disposition |
|---|---|---|
| `PRRT_kwDORzozKM6cmKT6` | `.github/policies/workflow-policies.md` | Fixed — new halt code plus two simulation scenarios make the new gate item executable |
| `PRRT_kwDORzozKM6cmKUm` | `.backlogit/queue/147.037-T.md` | Fixed — structural harness must also assert the `RewriteCheckpointFile` call |
| `PRRT_kwDORzozKM6cmKU9` | `.backlogit/queue/147.044-T.md` | Fixed identically |
| `PRRT_kwDORzozKM6cmKVX` | `tests/simulation/README.md` | Fixed — baseline scenario row recounted to 43 tasks / 106 edges |
| `PRRT_kwDORzozKM6cmKV6` | plan cycle-37 section | Declined — the PR description was already refreshed; the review's metadata snapshot predated the edit |

**Wave 3** — four threads raised against `44f4f078`, resolving to two defect classes.

| Thread | Path | Disposition |
|---|---|---|
| `PRRT_kwDORzozKM6cmdQ0`, `PRRT_kwDORzozKM6cmdRN`, `PRRT_kwDORzozKM6cmdRb` | policy, agent, runner | Fixed — the mirror hole: a newly closed entry is now re-run and required GREEN (`WAVE_GREEN_MAKER_UNVERIFIED`), with its own control scenario |
| `PRRT_kwDORzozKM6cmdQK` | `.github/agents/.ship.agent.md` | Fixed — Step 3's manifest census and green-regression array count recounted to 44 / 43 |

## Decisions and rationale

**Convergence gate (P1).** Step 4.6 item 2 re-confirmed RED only for red deliverables that
completed *in the current wave*. An entry carried into `open_red_deliverables_k` from an earlier
wave was never executed again, while the unfiltered suite stayed deferred for exactly as long as
that set was non-empty — so an earlier deliverable could go green before any declared green-maker
landed and still pass every convergence gate up to its closing wave. The always-run list in
`workflow-policies.md` P-002.6 gained a fourth item, mirrored in `.ship.agent.md` Step 4.6, and the
per-wave recomputation of `open_red_deliverables_k` moved into item 2 so item 3 consumes the same
set. An entry the wave *closed* is deliberately not run there — its selector is expected green and
is covered by the unfiltered suite the moment the set empties. Policy version 1.22.0 → **1.23.0**
with a changelog row; the simulation fixture's `policy_version` follows.

Deliberately **not** widened: no new `WAVE_*` halt code was introduced. `WAVE_RED_MAPPING_UNRESOLVED`
is scoped to schedule construction (Step 3), so reusing it at the convergence gate would have been a
scope mismatch. The new item mirrors item 3's existing code-free "report it rather than advancing"
language instead.

**Three-file width (P2).** `147.037-T` / U14 was the only task in the decomposition declaring three
files; every other task is at two or fewer, and this plan had already enforced the same heuristic
against `147.021-T` (cycle 3) and `147.014-T` (cycle 20). The caller migration was split by verb:
`147.037-T` / U14 keeps the resolve site in `internal/events`, and the new `147.044-T` / U14b takes
the abandon site in `internal/core`. Each unit is two files and two scenarios.

**Rejected alternative**: reducing U14 to two files by borrowing another unit's harness — the
pattern `147.036-T` / U13 already uses, where `147.035-T` / U12 owns the red. That would have left
the abandon migration with no red evidence of its own, which is dishonest rather than merely
inconvenient.

**Executable contract, not prose (wave 2, P1).** The review's strongest wave-2 finding was that the
tracked simulation would still report success with the new always-run item deleted, because
`Invoke-WaveScheduler` only computed `open_red_after_wave` and never ran an open selector. Fixed by
giving the early-green outcome a named fail-closed code, `WAVE_RED_DELIVERABLE_EARLY_GREEN`, used
by both the in-wave and carried-in cases; by recording `open_red_reconfirmed_at_wave` in the runner;
and by adding an `open_red_early_green` fixture mutation with two scenarios —
`open_red_early_green_carried_in` (halt expected) and `open_red_closed_entry_not_reconfirmed`
(completion expected, proving the gate re-confirms exactly the still-open set). Load-bearingness was
verified empirically: a throwaway copy of the runner with the new block deleted failed 6 assertions
in the control scenario (`early_green` `[147.042-T]` → `[]`, `compile_gate_waves` running to 18
instead of halting at 5). The copy was deleted after the probe.

**Structural harnesses strengthened (wave 2, P2).** Both migration harnesses asserted only that the
direct atomic-write call disappeared, which any other helper would satisfy. Both now require the
`RewriteCheckpointFile` call as well, in the task text and in the matching plan unit section.

**The mirror hole (wave 3, P1).** Wave 2's fix re-ran every *still-open* selector but let an entry
that closed at the gate leave the set unverified. Because the unfiltered suite stays deferred while
any other entry is open, a green-maker that landed without turning its selector green would not be
caught at the wave that was supposed to prove it — for `147.035-T` / U12 that is a six-wave gap.
Item 5 now re-runs every entry in `newly_closed_k` and requires GREEN, halting with
`WAVE_GREEN_MAKER_UNVERIFIED`. Items 4 and 5 partition the pre-recomputation open set exactly, so
no entry is skipped in either direction. The `green_maker_lands_but_selector_stays_red` control
halts at wave 7 as designed.

**Runner bug found while adding the control**: `$stayRedAfterClose = @(ConvertTo-List $x)` produced
a nested array because `ConvertTo-List` already returns `,@(...)`, so `-contains` never matched and
the new scenario silently passed as `COMPLETE`. Fixed by dropping the redundant `@()`, matching the
existing `$dropped = ConvertTo-List ...` pattern. Worth remembering: in this runner, always assign
`ConvertTo-List` output directly.

**Edge retargeting.** `147.008-T` / U4 edits `internal/core/checkpoint_disposition.go`, so its
prerequisite moved from `147.037-T` to `147.044-T`. `147.021-T` / U2f enumerates the post-migration
allow-list across both packages, so it now depends on both migrations. `147.006-T` / U3,
`147.007-T` / U3b, and `147.042-T` / U3c are resolve-verb units and kept their `147.037-T` edges;
U3c's green-maker is still `147.037-T`, closing at wave 8.

## Files modified

* `.backlogit/queue/147.044-T.md` (new — U14b)
* `.backlogit/queue/147.037-T.md`, `147.008-T.md`, `147.021-T.md`, `147.034-T.md`, `147.040-T.md`,
  `147.019-T.md`, `147-F.md`, `130-S.md`
* `.github/agents/.ship.agent.md`, `.github/policies/workflow-policies.md`
* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
* `tests/simulation/wave-scheduler-contract.json`, `tests/simulation/README.md`
* `.backlogit/hooks_queue.jsonl` (tool-managed)

All dependency and shipment mutations went through the worktree-bound `backlogit` CLI
(`dep add`, `dep remove`, `shipment add`, `add`, `sync`) rather than hand-edited frontmatter.

## Topology delta

| Measure | Before | After |
|---|---|---|
| Queued tasks in `M` | 42 | 43 |
| Executable edges | 104 | 106 |
| Shipment `130-S` members | 43 | 44 |
| Dependency waves | 18 | 18 |
| Wave 8 membership | `147.014-T`, `147.037-T` | `147.014-T`, `147.037-T`, `147.044-T` |
| Ready roots | `{147.001-T, 147.032-T}` | unchanged |

U14b depends only on U13 (wave 7), so it lands in wave 8 beside U14 and no dependent's wave moves.

## Validation

| Gate | Result |
|---|---|
| Wave-scheduler simulation, live-queue verification | `WAVE_SIM_OK` 150/150 across 21 scenarios |
| Markdown P-008 | 0 issues across 2300 files |
| Docline frontmatter | `valid: true`, 0 violations |
| Index sync | 1210 artifacts, 0 parse failures |
| `backlogit doctor` | 23 pre-existing orphans under `016-` / `106-`; none under `147-F` |
| Go source modified | none |

The simulation caught the schedule delta exactly: it failed on `wave_sizes[8]` and `scheduled`
before the fixture expectations were updated, which is the intended direction — the fixture was
corrected to the computed truth only after the wave-8 placement was reasoned about independently.

## Open questions and next steps

* §1.9 readiness requires a Copilot review whose `commit.oid` equals the final `headRefOid`. Any
  further push — including this memory artifact — re-opens Check 2 until a fresh review lands.
* Merge approval remains unrequested and ungranted. `130-S` stays `queued`.
* P-016 warning stands: `.copilot/worktrees/cycle24-remediation` and the dark-factory worktree on
  `chore/121-s-closure` still exist and must be resolved or exempted by their owner before Ship
  claims `130-S`. Nothing was deleted.
