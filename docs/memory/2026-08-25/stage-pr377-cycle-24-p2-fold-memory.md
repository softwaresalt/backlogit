---
chunk_strategy: h1-h2
description: "Stage cycle-24 bounded remediation: nine missing acceptance-criteria blocks authored, the cycle-23 appendix's own P-003 undercount corrected from two to nine, and three P2 advisories folded into the exempt-execution contract (positive completion marker, read-only command screening, U13 owner-status wording)"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-24 P2-fold and acceptance-criteria remediation memory
---

## Session frame

* Agent: Stage (prompt/policy/backlog-artifact remediation only; no Go source, no Go test edits, no
  production Go, no build loop, no push, no merge, no shipment claim, no subagent delegation)
* Worktree: dedicated, created fresh for this cycle at
  `.copilot/worktrees/cycle24-remediation`
* Branch: `chore/cycle-24-remediation`, created from `chore/stage-130-s` at HEAD
  `35aac6c097ec821243482265d04c78fb878b339c` (unchanged for the whole session — no commits landed
  on the base branch during this pass)
* Tooling: worktree-bound `go run ./cmd/backlogit --cwd .` CLI, `markdownlint-cli2@0.23.1` via
  `scripts/md-lint.ps1`, read-only `go test ./tests/integration/...`, direct `pwsh -NoProfile
  -Command` execution of each task's own declared `exempt_verification_command` /
  `harness_owner_command` for baseline re-verification, plus an independent Python
  frontmatter/topology recomputation (not the `sync` index) for cross-checking

## Baseline observed before any edit

| Measure | Value |
|---|---|
| Queued tasks under `147-F` | 42 |
| Queued tasks carrying no `acceptance-criteria` block | 9 (`147.031-T`, `147.033-T`, `147.034-T`, `147.035-T`, `147.036-T`, `147.037-T`, `147.039-T`, `147.040-T`, `147.041-T`) — the cycle-23 appendix had named only 2 of these |
| `harness-exempt` tasks | 10, all already carrying a `harness-exemption-contract` block (cycle 23) |
| Exempt commands carrying a positive completion marker | 0 |
| Exempt/harness-owner commands screened for destructive operations before execution | 0 (no screening step existed) |
| Workflow policy `Version` header | `1.0.0` (stale; Amendment Log's own last row already said `1.16.0`) |

## What this session did

### Nine acceptance-criteria blocks (P-003)

A full audit of all 42 queued task files (not a trust of the prior cycle-23 claim) found nine
carrying no `<!-- BEGIN:acceptance-criteria -->` block at all: `147.031-T`, `147.033-T`,
`147.034-T`, `147.035-T`, `147.036-T`, `147.037-T`, `147.039-T`, `147.040-T`, `147.041-T`. Each
gained a block authored strictly from that task's own already-declared contract — files, test
scenarios, harness/exempt lifecycle classification, dependencies, and evidence paths — with no
invented scope and no more than three scenarios per task. Style matches the existing convention
used by `147.001-T`/`147.030-T`/`147.007-T`/`147.042-T`/`147.043-T`/`147.032-T`/`147.038-T`: the
comment-delimited bullet list is appended directly after the task's closing paragraph with **no**
`## Acceptance Criteria` heading — the 147-series tasks never use that heading from the generic
`task.md` template, and "use existing task style" means matching what the sibling tasks in this
same series actually do. Every new block includes a measurable `Width: <files>, <scenarios>`
bullet, mirroring `147.007-T` / `147.042-T` / `147.043-T` and the constitution's Task Granularity
2-Hour Rule (fewer than 4 test scenarios). `147.036-T` and `147.041-T` (the two harness-exempt
members among the nine) additionally restate their own exemption class, owner, and evidence
grammar rather than a generic red-harness shape.

All 42 queued tasks — 43 counting the archived `147.010-T`, which already carried one — now carry
an `acceptance-criteria` block, independently re-verified by parsing every queue file rather than
trusting the count.

### Cycle-23 appendix correction (2 → 9)

The cycle-23 plan appendix's own "Still outstanding" paragraph said only `147.036-T` and
`147.041-T` carried no acceptance criteria. Corrected in place to name all nine and explain that
cycle 23's review scope was bounded to the harness-exemption contract and never re-audited every
task's `acceptance-criteria` block, so it undercounted by seven. The correction follows the same
precedent cycle 22 set for cycle-16/17's stale "current gate state" pointers (C4): a factual error
in a prior cycle's own appendix text is corrected in place, while cycles' *decisions* and
*root-cause remediation* stay historical record.

### Exempt-execution contract hardened (P-002.3, new P-002.5)

**Positive completion marker.** Every `exempt_verification_command` must now print
`EXEMPT_VERIFY_OK:{task_id}` as its last statement, reached only after every preceding assertion
has already passed, immediately before its own `exit 0`. A pass is now exit 0 **and** the exact
marker; exit 0 alone is `EXEMPT_MARKER_MISSING` (new P-002.2 code), not a pass. This closes the
cmd/shell-inversion and no-op gap the plain `exit 0` left open: a wrapper that negates the real
exit code, a shell that swallows a failure, or a no-op stand-in can each manufacture an unearned
`exit 0`, but none can also emit the one task-specific string the command only reaches after its
own guard clauses have already returned zero.

All ten exempt tasks' `exempt_verification_command` values were updated consistently — a
`Write-Output 'EXEMPT_VERIFY_OK:<task-id>'` statement inserted immediately before each command's
final `exit 0`, verified by locating the line via `^exempt_verification_command:` and confirming
the marker is present exactly once with the correct task ID in each of the ten files. `147.036-T`'s
`harness_owner_command` is **deliberately unmarked**: it is the `build-feature` loop command for
`covered-by`, checked by its own named `--- PASS: TestU12_` count against U12's harness, not by the
exempt completion gate — marking it would have made 11 marker commands instead of the declared 10.

**Read-only command screening (new P-002.5).** Before Ship (`Step 4.1a`) or `build-feature`
(`Step 0`) executes either an `exempt_verification_command` or a `harness_owner_command`, the
command text must first be statically screened for a Constitution Principle VII
destructive-operation pattern (deletion, force-overwrite, config/history mutation, data drops,
package install/removal, untrusted code execution). A match is rejected outright — never executed
by the gate — and routed to Principle VII operator approval; new halt code
`EXEMPT_COMMAND_DESTRUCTIVE`. All eleven live commands (the ten `exempt_verification_command`
values plus `147.036-T`'s `harness_owner_command`) were re-audited against a destructive-pattern
list (`Remove-Item`, `rm`, `del`, `Set-Content`, `Out-File`, `git push`/`commit`/`reset`/`rebase`,
`Move-Item`, `-Force`, `Install-`, and related verbs) and matched none — every one is a read-only
probe (`Test-Path`, `Get-Content`, `Select-String`, `go doc`, `go test -run`, `go run … docs lint`).

### Consumers updated

* `.github/policies/workflow-policies.md` — P-002.3 gains the marker mandate and an updated
  false-green signal table row; the "Post-deliverable gate" and "Pre-work probe" prose reference
  the marker; new **P-002.5** section added (statement, screening outcome, authoring rule,
  violation action); P-002.2's halt-code table gains `EXEMPT_MARKER_MISSING` and
  `EXEMPT_COMMAND_DESTRUCTIVE`; P-002's summary `Gate Point` row mentions the P-002.5 screen;
  `Version` header corrected `1.0.0` → `1.16.0`; the existing `1.16.0` Amendment Log row extended
  in place to describe this cycle's additions rather than minting a new version number.
* `.github/agents/.ship.agent.md` — Step 4.1a gains a screening bullet (new bullet 3) before the
  must-fail-before-deliverable probe (renumbered bullet 4), and the probe's failure/success
  criteria now reference the marker; Step 4.3's exempt completion gate requires the marker and can
  halt with `EXEMPT_MARKER_MISSING`.
* `.github/skills/build-feature/SKILL.md` — Step 0 gains a screening sub-step covering both
  `exempt_gate_cmd` and, when `covered-by` makes it distinct, `harness_cmd`; its failure/success
  criteria reference the marker; Step 2's evaluation rule distinguishes marker-checked
  `exempt_verification_command` runs from PASS-count-checked `harness_owner_command` runs; the
  post-loop completion gate, `Output`, `Behavioral Constraints`, and `Quality Criteria` sections
  updated to match.
* `harness-architect/SKILL.md` — intentionally **not** modified. It never executes either command
  (Step 1a is static-only), so the marker and screening requirements — both execution-time
  concerns — do not apply to it.

### `147.036-T` / U13 owner-status wording (U13 correction)

Three normative locations described U12 as "carries `harness-ready` rather than an exemption" in
the present tense, contradicting the same two-gate split cycle 23 itself introduced (static intake
never observes red evidence; only Ship Step 4.1a at claim time does). Corrected in `147.036-T`'s
own body, the plan's U13 unit section, and the plan's "Ship ready-selection contract" bullets to
"must carry `harness-ready` at claim time — eligible only after harness generation runs — rather
than an exemption." The cycle-22 historical appendix cell (`C2`) that used the original wording is
left unmodified: it is historical record of what cycle 22 wrote at the time, and rewriting it would
misstate history rather than correct a live defect.

### Optional: closure-evidence filename convention

The not-yet-created planned path `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`
did not follow the `YYYY-MM-DD` convention every other `docs/closure/` artifact uses. Renamed to
`docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md` (dated to the governing
plan document) across every **live** reference: `147.019-T` (5 occurrences), `147.026-T` (4),
`147.041-T` (5, including this cycle's own new acceptance-criteria bullet), and the plan's U10 /
U10b / U10c unit sections (3). The two historical mentions — the cycle-17 `H3` blocker-disposition
row and the cycle-23 `D4` finding row — keep the name as originally written; renaming history would
misstate what those cycles actually named. No topology, dependency, or task-count change, and the
file still does not exist.

## No fabricated REDs; no behaviour change

This session touched no `*.go` file. The nine acceptance-criteria blocks restate each task's own
already-declared contract; none extends scope beyond what the task body already specified. The
marker and screening additions change the *shape* of ten declared commands and two consumer
skill/agent files, not any Go behaviour.

## Baseline re-verification after the marker/screening changes

All ten `exempt_verification_command` values — each now carrying its `EXEMPT_VERIFY_OK:{task_id}`
marker — were extracted from the task files and executed directly (`pwsh -NoProfile -Command`)
against this worktree's pre-work tree:

```text
147.007-T  exit=1  marker printed=false
147.017-T  exit=1  marker printed=false
147.018-T  exit=1  marker printed=false
147.019-T  exit=1  marker printed=false
147.021-T  exit=1  marker printed=false
147.026-T  exit=1  marker printed=false
147.032-T  exit=1  marker printed=false
147.036-T  exit=1  marker printed=false
147.038-T  exit=1  marker printed=false
147.041-T  exit=1  marker printed=false
```

All ten still exit 1 and none reaches the marker statement, because none of the ten deliverables
exists in this worktree — the declared baseline is unchanged by adding the marker.
`147.036-T`'s `harness_owner_command` was executed the same way and also exited 1.

## Gate status — unchanged

Cycle 24 is **remediation evidence, not a gate**. No `## Plan Review` record was appended, no
`PASS` is claimed, and the current gate state remains `cycle: 21` `FAIL` with
`restage_recommendation: confirmatory-review-of-cycle-21-fixes` (now widened to require review of
cycles 21 through 24 together). The plan carries a "PR #377 plan remediation, cycle 24" appendix
under the cycle-23 appendix that says so explicitly.

## Topology (unchanged, re-verified independently)

Recomputed from the queue frontmatter via a standalone script (not from the `sync` index, and not
by trusting the prior cycle's claim):

* 42 queued tasks under `147-F`, 104 queued-to-queued executable edges, 43 shipment members
* Ready roots unchanged: `{147.001-T, 147.032-T}`
* Kahn topological sort: 42/42 ordered, **acyclic**
* Exempt set unchanged: exactly the same ten enumerated members
* No dependency edge, task count, shipment membership, or unit definition changed

## Validation

| Gate | Result |
|---|---|
| All 10 `exempt_verification_command` values, re-executed at this worktree's HEAD | 10/10 exit 1, 0/10 marker printed |
| `147.036-T` `harness_owner_command`, re-executed | exit 1 |
| Destructive-operation pattern audit, 10 exempt commands + 1 owner command | 0 matches — all read-only |
| `sync` | 1208 artifacts indexed |
| `doctor` | 23 issues, all pre-existing orphans (`106.0xx-T`, `016.001-R`) outside `147-F`; 0 new |
| `docs lint` | `valid: true, violation_count: 0` |
| `scripts/md-lint.ps1` (markdownlint-cli2 0.23.1, repo-wide) | 2286 files, 0 issues, exit 0 |
| `go test ./tests/integration/... -count=1` | `ok` — existing structural prompt/plugin/CI guards pass |
| Frontmatter YAML validity, 21 changed Markdown files | 21/21 valid mapping |
| Balanced `BEGIN:`/`END:` section markers, 21 changed Markdown files | balanced (one raw-text false-positive hit inside a historical table cell's backtick-quoted example, not a real marker) |
| Exemption-class vocabulary (`declaration-only`, `docs-only`, `verification-only`, `covered-by`) | present and consistent across policy, `.ship.agent.md`, `harness-architect/SKILL.md`, `build-feature/SKILL.md` |
| Halt-code cross-references (12 codes total, 2 new) | all 12 in policy; the 5 execution-time codes (including both new ones) in `.ship.agent.md` and `build-feature/SKILL.md`; the 7 static-intake-only codes in `.ship.agent.md` only (pre-existing, correct asymmetry) |
| Topology: tasks / edges / members / roots / exempt | 42 / 104 / 43 / `{147.001-T, 147.032-T}` / 10 |
| Acceptance criteria | 42/42 queued (43/43 with archived `147.010-T`) |

## Open questions and next steps

1. **The independent confirmation review is still outstanding and now wider.** It must cover the
   cycle-21, cycle-22, cycle-23, and cycle-24 changes together. The gate stays `FAIL` until it runs.
2. Only after that review should either worktree's branch be pushed and PR #377 reconciled.
3. No push, merge approval, or shipment claim was performed, and no subagent was invoked.
4. **No P-003 gap remains**: all 42 queued tasks (43 counting the archived `147.010-T`) now carry
   at least one acceptance criterion.
5. **Deferred by scope (carried from cycle 22/23)**: no structural Go test asserts the
   P-002.1/P-002.3/P-002.5 contract grammar or the marker/screening requirements. A follow-up
   should add a guard that every `harness-exempt` task's `exempt_verification_command` contains
   the literal string `EXEMPT_VERIFY_OK:` followed by its own task ID, and that the class
   vocabulary matches across policy, agent, and skills.
6. `plugin/agents/ship.agent.md` and `plugin/skills/` were again deliberately not modified — the
   frozen standalone bundle carries no P-002 vocabulary at all and is not a coupled surface.
7. This session's worktree (`chore/cycle-24-remediation`) is separate from the pre-existing
   `chore/stage-130-s` worktree the cycle-17 through cycle-23 sessions used. Both currently point
   at the same HEAD (`35aac6c0`); reconciling which branch carries the final, pushed history is
   part of the still-outstanding confirmation review, not this session's decision to make.
