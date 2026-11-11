---
title: "212-S workflow duration and efficacy evidence: 2026-11-08"
description: "Durable measurement summary, elapsed phase windows, historical full-suite timings, and efficacy recommendations for the 212-S workflow through 2026-11-08."
doc_type: reference
source: docs/evidence/2026-11-08-212-s-workflow-duration-efficacy.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
---

## Scope and measurement methodology

This reference document records the verified workflow duration measurements,
historical full-suite execution timings, and efficacy findings for shipment
`212-S` through the audit cutoff at `2026-11-08T01:18:13Z`, followed by a
distinct post-cutoff continuity observation at `2026-11-08T01:20:35Z`.

All figures are derived from UTC-normalized backlog item log entries, Git
commit timestamps across `origin/main..HEAD` (`5f91f25f`, `25421faa`, and
`b40a2a9f`, full HEAD `b40a2a9fb9decbc9b808aaeaf5443ee2277ef475`), native
child-process receipts, review readiness records, and recovery pull request
metadata ([PR #644](https://github.com/softwaresalt/backlogit/pull/644)).
This audit performed no new test executions, no system configuration edits,
and no source code modifications. Measurements are diagnostic evidence only
and do not constitute budget resets or quality-gate waivers.

### Measurement boundaries and known versus unknown data

* **Elapsed phase windows, not CPU or model-active time**: Every phase and
  task interval in this document measures wall-clock elapsed time between
  recorded UTC timestamps, not active CPU execution or model inference time.
* **Historical implementation span**: From the first `WORK_STARTED` event at
  `2026-11-06T00:53:27.093542Z` to the `2026-11-08T01:18:13Z` cutoff, total
  elapsed wall time is `174,285.91s` (`48h 24m 45s`).
* **Task execution windows exclude surrounding phases**: The nine completed
  task execution windows (`WORK_STARTED` to task completion) sum to
  `21,369.15s` (`5h 56m 09s`). Because `WORK_STARTED` is logged after
  harness preparation and excludes later cross-task remediation and gate
  orchestration, this sum is not total productive CPU or agent active time
  and must not be added to overlapping phase tables.
* **Unmeasured and excluded intervals**: Initial checkpoint recovery prior to
  [PR #644](https://github.com/softwaresalt/backlogit/pull/644), whole-run
  durations for full-suite Runs 1-3, and per-agent token or active-hour
  totals are unmeasured and omitted rather than estimated. A separate
  side-actor initial RCA report duration (`54s`, which includes model and
  tool wall time rather than CPU time) comes from an unproven actor without
  a reproduction receipt, overlaps the main RCA window, and is excluded from
  phase totals.

## Latest repair tranche phase windows

The latest repair tranche spans `2026-11-07T20:48:03Z` to the audit cutoff at
`2026-11-08T01:18:13Z` across 11 contiguous, non-overlapping elapsed phase
windows totaling `16,210s` (`4h 30m 10s`).

| Phase | Start UTC | End UTC | Elapsed (`mm:ss` / `h:mm:ss`) | Seconds | Efficacy and evidence notes |
| --- | --- | --- | --- | --- | --- |
| 1. RCA and evidence report | `2026-11-07T20:48:03Z` | `2026-11-07T21:16:29Z` | `28:26` | `1706` | Consolidated blocker analysis in [delivery blockers RCA](../decisions/2026-11-07-212-s-delivery-blockers-rca.md). |
| 2. Options and operator selection | `2026-11-07T21:16:29Z` | `2026-11-07T21:24:02Z` | `07:33` | `453` | Operator selected Option 1 (212-only final full-suite cadence; no 213-215 exception; no budget reset). |
| 3. Stage scope/plan/review/handoff | `2026-11-07T21:24:02Z` | `2026-11-07T21:58:26Z` | `34:24` | `2064` | High planning overhead after completed RCA; recorded cadence in prose without executor-gate compatibility check. |
| 4. Ship intake/provenance commit | `2026-11-07T21:58:26Z` | `2026-11-07T22:05:37Z` | `07:11` | `431` | Ship intake and provenance recorded at commit `5f91f25f`. |
| 5. Repair harnesses and command-guard setup | `2026-11-07T22:05:37Z` | `2026-11-07T22:38:27Z` | `32:50` | `1970` | Heavy custom guard and proof scaffolding cost despite established writer seams. |
| 6. U1g implementation/tests/review/completion | `2026-11-07T22:38:27Z` | `2026-11-07T23:03:22Z` | `24:55` | `1495` | Native tests took `27s` RED, `39s` GREEN, and `9s` post-loop gate ([U1g proof](../memory/2026-11-07/212-S-U1g-green-proof.json), commit `25421faa`). |
| 7. Task transition | `2026-11-07T23:03:22Z` | `2026-11-07T23:06:58Z` | `03:36` | `216` | Sequential handoff transition from `U1g` (`211.013-T`) to `U1h` (`211.014-T`). |
| 8. U1h initial implementation and first GREEN commit | `2026-11-07T23:06:58Z` | `2026-11-07T23:18:39Z` | `11:41` | `701` | Fast initial code/test pass (~12m); power child `0.0565s` ([power proof](../memory/2026-11-07/212-S-U1h-green/native-python-result.json)) and LF child `20.524s` ([LF proof](../memory/2026-11-07/212-S-U1h-green/lf-result.json)). |
| 9. U1h review/remediation to last fix commit | `2026-11-07T23:18:39Z` | `2026-11-08T00:28:59Z` | `1:10:20` | `4220` | Caught 3 real C1 defects across 2 fix cycles; invalid helper evidence and missing Anchor ACK added churn (commit `b40a2a9f`). |
| 10. U1h final proof/review readiness/completion | `2026-11-08T00:28:59Z` | `2026-11-08T01:06:36Z` | `37:37` | `2257` | Readiness verified in [U1h review readiness](../memory/2026-11-08/212-S-U1h-review-readiness.json); Phases 9-10 totaled `1h 47m 57s` (~40% of tranche). |
| 11. Post-repair static gates/admission | `2026-11-08T01:06:36Z` | `2026-11-08T01:18:13Z` | `11:37` | `697` | Pinned Go 1.24 compile/vet and `v1.64.8` lint passed; surfaced Step 4.6 wave-convergence conflict before `U3`. |
| **Total (11 contiguous phases)** | `2026-11-07T20:48:03Z` | `2026-11-08T01:18:13Z` | `4h 30m 10s` | `16210` | **Contiguous elapsed phase windows (not CPU or model-active time).** |

## Historical task windows and full-suite evidence

### Nine completed task execution windows

Across the `48h 24m 45s` (`174,285.91s`) implementation span, the nine
completed tasks recorded `21,369.15s` (`5h 56m 09s`) of elapsed
`WORK_STARTED`-to-completion window time (excluding pre-task harness setup
and post-task orchestration):

* `U1` (`211.001-T`): `976.05s` (~16m).
* `U2` (`211.002-T`): `4,690.16s` (~1h 18m).
* `U1b` (`211.008-T`): `2,761.25s` (46m).
* `U1c` (`211.009-T`): `2,094.85s` (35m).
* `U1d` (`211.010-T`): `502.57s` (8m).
* `U1e` (`211.011-T`): `427.97s` (7m).
* `U1f` (`211.012-T`): `1,243.98s` (21m).
* `U1g` (`211.013-T`): `1,494.78s` (25m; overlaps Phase 6 above, do not sum).
* `U1h` (`211.014-T`): `7,177.54s` (~2h; overlaps Phases 8-10 above, do not sum).

### Historical recovery and seven full-suite runs

Seven full-suite convergence runs were executed across `212-S`, with six
unsuccessful outcomes; historical failure counters were never reset:

* **Recovery [PR #644](https://github.com/softwaresalt/backlogit/pull/644)**:
  Created at `2026-11-05T23:48:27Z` and merged at `2026-11-05T23:51:17Z`
  (`170s` = `2m 50s`); initial checkpoint recovery work before PR creation is
  unmeasured.
* **Runs 1-3 (partial package timings only)**: Whole-run elapsed times are
  unavailable. Recorded failing-package durations--CLI `311.753s`,
  `internal/core` `1,802.354s`, and `internal/core` `1,575.7s`--are partial
  package figures, not whole-suite runtimes.
* **Run 4 wrapper and approval wait**: Wrapper elapsed time from
  `2026-11-06T19:57:11.125Z` to `2026-11-06T20:58:37.386Z` was `61m 26s`
  wall-clock (not proven active test runtime) and failed with lost panic
  context. The subsequent interval to extra-review operator approval at
  `2026-11-07T03:16:59.656Z` was `22,702.27s` (`6h 18m 22s`) of blocked
  waiting time, not active agent work.
* **Runs 5 and 6 (CRLF-induced rerun)**: Run 5 took `2,551.59s` (`42m 32s`),
  passing all product code tests but failing solely due to 6 CRLF-encoded
  tool-owned JSON files. Run 6 took `2,499.457864s` (`41m 39s`) and passed.
  While Runs 5 and 6 combined span `84.19m` (`5,051.05s`), the avoidable
  increment attributable to missing LF preflight is the single rerun
  (~`41.66m` / `2,499.46s`), not the full `84.19m`.
* **Run 7 (host S4 hibernation)**: Reported wall-clock elapsed time was
  `36,266.58s` (`10h 04m 27s`). Verified host S4 hibernation accounts for
  `34,435.56s` (`9h 33m 55.6s`) alongside ~`1,828.05s` (`30m 28s`) of
  pre-sleep execution, proving it was not 10 hours of active CPU testing.

## Efficacy analysis and ranked recommendations

In the latest `4h 30m 10s` repair tranche, the single largest cost center was
`U1h` review, remediation, and final completion (`1h 47m 57s` = `6,477s`, or
~40% of the tranche), whereas initial `U1h` implementation to first GREEN
took `11m 41s` (~12m). Multi-agent review provided substantive quality value
by catching three real C1 defects:

* Ownership and summary field retention in readiness artifacts.
* Non-vacuous LF verification (`PASS` rather than `[no tests]` vacuity).
* Reparse-ancestor symlink and junction containment validation.

However, two remediation cycles, invalid helper evidence, and a missing
Anchor reviewer acknowledgement (`ACK`) generated avoidable churn. Standard
review personas plus the 3-model adversarial review pool also perform
duplicate file reads that merit investigation (without permission to drop
required coverage). Upstream orchestration remained heavy as well: Stage
planning/review/handoff (`34m 24s`), harness setup (`32m 50s`), and Ship
intake (`7m 11s`) consumed `1h 14m 25s` after the `28m 26s` RCA was already
complete.

### Ranked investigations

1. **Make approved cadence executable before plan review and acceptance**:
   Encode operator-approved cadence exceptions into truthful planned harness,
   green-maker, and closing-wave contracts before Ship intake so planning and
   executor gates agree without fake `PASS` or invented `RED` records.
2. **Scope and consolidate reviews and enforce upfront fingerprint/EOF ACK**:
   Investigate reducing duplicate file reads across standard personas and the
   3-model adversarial pool while preserving independent model coverage, and
   validate Anchor fingerprint/EOF `ACK` formatting upfront.
3. **Reuse stable verification tools and concise proofs**: Replace bespoke
   per-task source-copy, hash, and observer frameworks with a single tested
   verification launcher and compact structured receipts.
4. **Profile `internal/core` (`24-30m`) per-test histogram and evaluate safe
   package parallelism**: Measure per-test elapsed distributions and evaluate
   isolated package parallelism under governance, never via unilateral test
   skips or weakened final gates.
5. **Instrument code, test, reviewer, coordination, and blocking start/end
   timestamps separately**: Record distinct sub-phase boundaries before
   projecting active-time savings.

### Operational controls and investigation targets

* Pinned Go `1.24` and `golangci-lint` `v1.64.8`, native LF preflight
  (`20.52s`, now implemented), and awake-host execution requests are active;
  awake-host requests mitigate idle sleep only and cannot prevent manual host
  sleep or hibernation.
* Any proposed efficiency timebox figures must be treated strictly as
  **investigation targets, not measured SLAs**.

## Live post-cutoff observation (`2026-11-08T01:20:35Z`)

At `2026-11-08T01:20:35Z` (recorded in
[212-S-M14-forward-20261108T012035Z.md](../memory/2026-11-08/212-S-M14-forward-20261108T012035Z.md)),
`U1g` (`211.013-T`) and `U1h` (`211.014-T`) are `done` (`9/14` shipment tasks
`done`), original MCP tasks remain `2/7` `done`, and `U3` (`211.003-T`) has
not started. Ship identified that while the operator-approved Option 1
final-only full-suite cadence for `212-S` was recorded in plan prose, Ship
Step 4.6 ([_ship.agent.md](../../.github/agents/_ship.agent.md), lines
1114-1187) requires an intermediate unfiltered full suite when the canonical
`open_red` set is empty. This planning/executor compatibility gap blocked
`U3` admission, and Stage adjudication was requested independently. No status
beyond `2026-11-08T01:20:35Z` is claimed, no fake `PASS` or `RED` was
recorded, and no gate was waived.
