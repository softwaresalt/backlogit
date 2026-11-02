---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CT test-suite health: stabilize the gate-evidence counter concurrency test, fix the slog/log capture leak, stabilize the release client timeout test, spike the internal/core runtime budget, and verify the full suite green.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md
title: 'Implementation Plan: CT test-suite health'
---

# Implementation Plan: CT test-suite health

## Objective

Return `go test ./... -count=1` to green on the Windows host, with useful
diagnostics, and measure the internal/core runtime budget. Source:
`docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md`.

## Problem Frame

See the deliberation. The full-suite run on 2026-10-08 failed only on a
load-sensitive concurrency test. internal/core used 92% of the timeout, and
earlier diagnostics were lost to a log-capture leak.

## Requirements Trace

| Req | Source | Requirement | Units |
|---|---|---|---|
| R1 | 8F41D60D | The counter concurrency test asserts uniqueness, not universal success | T1 |
| R2 | D8EF5443 | Log capture restores slog and log state | T2 |
| R3 | 92AB6879 | The release timeout test is independent of load | T3 |
| R4 | 072-DL context | The internal/core runtime budget is measured and follow-ups are captured | T4 |
| R5 | DB48A817, FF1F3AC7, 885263D2 | A full-suite green run is recorded | T5 |

## Implementation Units

### T1: Stabilize the gate-evidence counter concurrency test

Domain: tests. Posture: characterization-first. Stash 8F41D60D. File:
`internal/core/gate_evidence_formal_test.go`
(`TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`).

Keep n=20. Treat `errors.Is(err, bkerrors.ErrGateInProgress)` (package
`internal/errors`) as a valid outcome; confirm the formal-required wrapper
keeps the `%w` chain. Any other error fails. Require at least one success and
a persisted `EventGatePassed` count equal to the number of successes, with no
duplicate counter.

AC:

1. `go test -count=30 -run TestAppendGateEvidence_ConcurrentSameItem ./internal/core`
   passes, and still passes with `-cpu 1`.
2. Reverting the 106-F unlock-after-append fix still fails the test on the
   duplicate-counter assertion (record this in the task).
3. No production file changes.

### T2: Restore slog and log state after log capture

Domain: tests. Posture: test-first. Harness: ready (red assertion is AC1).
Stash D8EF5443. Files:
`internal/core/gate_baseref_warn_test.go`,
`internal/core/shipment_shipped_event_durability_test.go`.

Add one helper in `gate_baseref_warn_test.go` (package `core`) that captures
`slog.Default()`, `log.Writer()`, `log.Flags()`, and `log.Prefix()` and
restores them in `t.Cleanup`: slog first, then `log.SetOutput`, `SetFlags`,
and `SetPrefix`, so a non-default previous slog handler can't re-redirect
`log`. Use it at the three sites (lines 40, 64, and 395 at d2f3ff97). These
are the only `slog.SetDefault` sites in internal/core tests at d2f3ff97.

AC:

1. After a capture site's cleanup, `log.Writer()`, `log.Flags()`, and
   `log.Prefix()` equal the values saved before the capture. This fails on
   `main`.
2. `go test -count=1 ./internal/core -run 'BaseRef|ShippedEventDurability'`
   passes.

### T3: Stabilize TestClientLatestRespectsContextTimeout

Domain: tests. Posture: test-first. Harness: ready (red assertion is the
`errors.Is(err, context.DeadlineExceeded)` check). Stash 92AB6879. File:
`internal/release/release_test.go`.

The server handler waits on `r.Context().Done()` or 2s. The test asserts
`errors.Is(err, context.DeadlineExceeded)` and `elapsed < 1s`.

AC:

1. `go test -count=200 -run TestClientLatestRespectsContextTimeout ./internal/release`
   passes.
2. Removing the context propagation in `Client.Latest` still fails the test.

### T4: internal/core runtime budget spike

Domain: spike (docs output). Context 072-DL. Output:
`docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md`.

Run `go test -json -count=1 ./internal/core` on the Windows host. Rank the 25
slowest tests and classify each (real subprocess, recovery-enabled fixture,
fsync-heavy, or sleep or poll). Recommend follow-ups and capture each as a
stash entry. Time box: 2 hours. No code changes.

AC:

1. The findings list total package time, the top 25 with classifications, and
   the stash IDs captured.

### T5: Full-suite green verification

Domain: verification-only. Depends on T1, T2, and T3. T4 is independent;
T4 may reuse T5's run by adding `-json` for internal/core instead of running
a second 27-minute package run.

Run `go test ./... -count=1 -timeout 30m` on the Windows host after CX has
merged, and record the per-package times in the CT closure. The closure
states that the DB48A817, FF1F3AC7, and 885263D2 isolation fixes held, which
is the evidence Stage uses to archive them.

Red-run policy: if one package fails on a test outside CT's scope, rerun that
package once. If it passes, record both runs and treat the suite as green
with the flake captured as a new stash entry. If it fails again, capture a
stash entry, record CT closure as verification-incomplete, and keep the three
entries active. CT does not widen to fix it.

AC:

1. EXIT=0 (or a green rerun under the red-run policy), with the internal/core
   time recorded.
2. The closure names the three stash IDs and the run evidence path.

## Dependency Graph

```text
T1, T2, T3 -> T5; T4 independent
Shipments: 198-S (CT) blocks-on 197-S (CX)
152-S retains blocks-on 197-S (CX) and 198-S (CT)
```

## Decisions

* D1-D6 are in the deliberation. queue_position is 150.
* The operator-authorized PR #487 finding-2 correction makes the existing
  T5 after-CX-merge requirement an explicit shipment prerequisite.
  queue_position alone is not evidence of dependency readiness.

## Risks

* T1 could weaken the test so that it no longer detects the duplicate-counter
  bug. Mitigation: T1 AC2 (revert check).
* T5 can fail on a new flake. Mitigation: capture it as a stash entry, and
  don't widen CT.

## Constitution Check

* One domain and the 2-hour rule per unit: yes.
* No production behavior change: yes.

Constitution Check: pass

## Plan Hardening Signals

* Concurrency test semantics (T1).

Requires plan hardening: yes

## Runtime Verification and Closure

T5 is the runtime verification. The closure attaches the run log path and
per-package times.

## Plan Hardening

### Context consulted

* `internal/core/gate_evidence_formal.go` `nextGateEvidenceCounter` and the
  106-F F1 finding in the test comment.
* 072-DL and 174.073-T.

### Protected invariants

* No duplicate gate-evidence counter is ever persisted.
* `ErrGateInProgress` remains the fail-closed result of a bounded-wait
  timeout.

### Risky actions

* T1 loosening an assertion. Mitigated by AC2.

### Added verification

* Repeat counts in the T1 and T3 AC.

### Closure, monitoring, and rollback

* Rollback is a revert of the CT merge.

### Review-gate capability risks

* None.

### Unresolved operator decisions

* None.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* attempt: 1
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* findings and revisions: T2 and T3 declare harness-ready red assertions; T2 helper location and restore order fixed, AC compares against saved values; T1 names bkerrors.ErrGateInProgress; T5 no longer waits on T4 and has a red-run policy; T5 runs after CX merges.
* residual advisory disposition: Architecture Strategist's CT blocks-on CX
  recommendation is now adopted under the operator's 2026-10-08 PR #487
  finding-2 correction authorization. The native `198-S blocks-on 197-S`
  edge withholds CT from dependency-aware queue results until CX is resolved,
  but queue filtering accepts done, accepted, archived, shipped, abandoned,
  and rejected. `ClaimShipment` does not check dependencies.
  Orchestrator/Ship MUST separately read 197-S from canonical Markdown
  (not the SQLite index, which does not project `archived_status`)
  immediately before claiming 198-S and require effective shipped provenance:
  `status: shipped`, or `status: archived` with `archived_status: shipped`
  (the normal post-`ShipShipment` form). Abandoned (live or
  archived-from-abandoned), rejected, any other terminal state, and missing
  or unparsable provenance FAIL CLOSED and do not satisfy T5's after-CX-merge
  requirement. Queue positions and shipment membership remain unchanged.
  No implementation scope is added.
* operator_authorization: approved (operator APPROVED this scope and directed autonomous work without routine confirmations, relayed by the Orchestrator on 2026-10-08)

## Amendment A1 (Stage, 2026-10-12): harness lifecycle, closed exempt set, and T1/T3 red-phase ruling

Appended; every line above is preserved. Trigger: `DARK_MODE_HALTED` at 198-S
wave-1 admission with a P-002 gap on T4 and a latent identical gap on T5
(`docs/memory/2026-10-12-ship-198s-wave1-halt.md`, commit 9a9efd1f), plus Ship's
request that Stage confirm the mutation-kill red evidence it used for T1 and T3.
A1 was first written as an escalation of that request. The operator then ruled
(Option A, recorded in A1.4) and answered nothing else, so a second revision on
the same day records the ruling and resolves the other open items inside policy.

Authority and landing. A1.1 through A1.3 are Stage planning-field amendments
under P-010 (Stage owns exemption classification, the closed set, and task
labels and bodies). A1.4 records the operator's ruling; Stage relaxes nothing
itself, and the relaxation is exactly what the operator's Option A names. Stage
ran no state-changing git operation (it ran read-only git commands in fixtures).
A1 is working-tree only, on Ship's feature branch, for
Ship to commit by explicit path, as in the 197-S precedent
(`docs/memory/2026-10-10/stage-197s-red-contract-amendment.md`); A1.5 lists the
paths. The rationale and rejected alternatives are in
`docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md`.

Review status. The first revision of A1 ended FAIL after three cycles, on the T1
and T3 hold. The operator's ruling and the 2026-10-12 revision were reviewed
afresh in a second review series (four cycles: three authorized by the Orchestrator's dispatch after the ruling, and a fourth the Orchestrator approved after the third returned ADVISORY; the first series' attempt marker is superseded); the final `## Plan Review` section at the end of
this plan governs.

**Unchanged by A1:** the shipment manifest S and the frozen wave set M
(`{198.001-T, 198.002-T, 198.003-T, 198.004-T, 198.005-T}`, tasks only per P-002.6; covering feature
198-F is in S, not M), the task set and task scope, dependencies, and queue positions. The
acceptance-criterion meaning is unchanged; two criteria carry errata in A1.1
(T4 AC1, where the stash IDs are recorded, and T5 AC2, which the evidence
artifact and the closure share). A1 adds classification metadata, evidence
manifests, errata, two run-parameter changes (the T4 and T5 `-timeout`, A1.3), a
stash-capture ordering change (A1.3), a clarified outcome for T5's non-green branches (A1.3 Erratum), the operator's ruling with its conditions,
and a `green-regression-contract` block on each of T1 and T3 (A1.5). No task is
added or removed. Ship passes the criterion-touching errata (T4 AC1, T5 AC2, T5's non-green outcome) to its Step
4.4 review so a reviewer reads each criterion against them.

### A1.1 Harness lifecycle by unit

| Unit | Task | Lifecycle | Red basis |
|---|---|---|---|
| T1 | 198.001-T | harness-required (`harness-ready`, Ship's label, retained under the ruling) | mutation-kill, with P-004 relaxed by the operator (A1.4 Option A, R1 to R6) |
| T2 | 198.002-T | harness-required (`harness-ready`) | genuine assertion red (`TestUT2_`), unchanged |
| T3 | 198.003-T | harness-required (`harness-ready`, Ship's label, retained under the ruling) | mutation-kill, with P-004 relaxed by the operator (A1.4 Option A, R1 to R6) |
| T4 | 198.004-T | `harness-exempt`, class `docs-only` | no harness; P-002.3 pre-work probe, A1.3 |
| T5 | 198.005-T | `harness-exempt`, class `verification-only` | no harness; P-002.3 pre-work probe, A1.3 |

Errata. None changes a task's scope or acceptance-criterion meaning; the two that
touch a criterion's wording say so, and T5's non-green outcome replaces a plan outcome (below) and is disclosed as such.

* T3's original text names `errors.Is(err, context.DeadlineExceeded)` as its red
  assertion. That check passes on `main` because the context propagation
  already shipped, so it is not red there. A1.4 records the mutation-kill basis
  the operator accepted instead.
* T4's text gives the run as `go test -json -count=1 ./internal/core` with no
  timeout. The package takes about 28 minutes under load (1656 s) and Go's default per-package
  timeout is 10, so the run needs `-timeout`; A1.3 pins `-timeout 40m`.
* T4's AC1 lists "the stash IDs captured" in the findings. A capture inside the
  task would put `.backlogit/stash.jsonl` in the gated delta (P-002.4), so Ship
  captures after the merge, in the post-merge closure, and records the IDs in the
  closure artifact and a task comment instead of the findings (A1.3). T5's red-run
  policy captures its flake the same way. T5's AC2 ("the closure names the three
  stash IDs and the run evidence path") is met by the evidence artifact plus the
  closure's citation of it. T5's text and reason say `-timeout 30m`; the run is
  `-timeout 40m` (A1.3).
* T5's "record the per-package times in the CT closure" and its AC2 "the closure
  names the three stash IDs and the run evidence path" are delivered by the T5
  evidence artifact (A1.3), which Ship's post-merge closure cites as the run
  evidence. A post-merge closure is written after the merge, so it cannot be T5's
  own deliverable, and P-002.4 admits only `*_test.go` files and `docs/closure/`
  artifacts for `verification-only`.
* T5's red-run policy says a second failure of the same package is recorded as
  "CT closure verification-incomplete". Because the exempt gate cannot exit 0 over
  a red suite, that branch is not a completion of T5 (A1.3 Erratum): Ship leaves
  T5 non-done, reports the failure through P-005, and halts with 198-S left active (A1.5), and
  Stage captures the policy's stash entry at triage. A package timeout, which
  names no failing test, takes the same branch instead of the rerun branch, as do a first-run failure that names no `--- FAIL:` test (a panic outside a test, a build failure), a first run that fails in two or more packages, and a failure in a CT-scope test or file (A1.5).
  This replaces the plan's original outcome for that branch (record a CT closure
  as incomplete, keep the three entries active) with a halt, which P-002.2
  requires of an exempt task. Neither acceptance criterion is met there, so no
  criterion's text
  changes, and the shipment's disposition then is the operator's (A1.7 item 6).
* T5's red-run policy says a rerun applies to a failure "on a test outside CT's
  scope". A1 reads that as requiring a named failing test, so a package timeout
  (which names none) is not a rerun case. The policy's "record CT closure as
  verification-incomplete" branch is not a completion of T5 (A1.3).
* "T4 may reuse T5's run" cannot apply, because T4 is in wave 1 and T5 is in
  wave 2. The original sentence is superseded (A1.8 E-7), and the 198.005-T body
  no longer carries it; T4 performs its own run.

### A1.2 Closed harness-exempt set (P-002.1 contract membership)

<!-- BEGIN:harness-exempt-set -->

The closed harness-exempt set for 198-F / 198-S is exactly:

**{198.004-T, 198.005-T}**

| Task | Plan unit | Class | Owner | Deliverable (sole changed file) |
|---|---|---|---|---|
| 198.004-T | T4 | docs-only | none | `docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md` |
| 198.005-T | T5 | verification-only | none | `docs/closure/198-S-ct-full-suite-green-evidence.md` |

No other task in this release unit may claim `harness-exempt`; adding a member
requires a reviewed amendment. 198.001-T, 198.002-T, and 198.003-T are
deliberately not members: each changes or adds test code that guards behavior,
and T1 and T3 edit pre-existing assertions, which `verification-only` forbids
(P-002.4 content pass).

<!-- END:harness-exempt-set -->

Objective classification (P-002.4). T4 changes zero `*.go` files: its single
changed file is Markdown, and its `go test -json` run is a read-only
measurement. T5 changes zero `*.go` files: its single changed file is an
evidence artifact under `docs/closure/`, it adds no test function, and it
records behavior already shipped by CX (197-S) and delivered by T1 through T3.
Neither unit changes production behavior, so neither needs a `covered-by` owner,
and fabricating a test for either is forbidden (P-002.1, P-004).

### A1.3 Exemption contracts

Roles of the two copies. The task-body block is what Ship's intake reads and so
what governs at run time (P-002.1). This section is the reviewed copy and the home
of set membership, deliverable paths, and the declared evidence manifests. The two
must be identical: Ship's A1.5 intake check compares them, and Step 4.1a item 1
halts as `EXEMPT_CONTRACT_INCOMPLETE` on any later drift between intake and claim
time. Stage applied each block below to its task body verbatim, inside the
P-002.1 `harness-exemption-contract` markers and with the `harness-exempt` label,
and read each task back through `backlogit_get_item`, comparing the whole block to
this section. A later edit to a task body is a contract change and needs its own
amendment. The 2026-10-12 revisions of this section (Orchestrator dispatch Q2 and Q3,
then the fresh review series) re-applied both blocks after each change.

Each command runs in pwsh 7.3 or later, invoked from pwsh, from the repository
root. It is read-only (P-002.5): it uses `Test-Path`, `Get-Content`,
`Resolve-Path`, `[regex]`, `git cat-file`, `git check-ignore`, `git diff --quiet`,
`git ls-files`, `go list`, and `backlogit docs lint`. It does not read
`.backlogit/stash.jsonl` (follow-up capture moved out of the gated window,
below). The `2>$null` redirections discard a stream and write no file. Each
command sets `$ErrorActionPreference = "Stop"`, so a failed cast or missing file
ends the run non-zero instead of skipping a check. Evidence paths in the
manifests must be repository-relative, resolve inside the repository, and be
git-ignored (Principle III; P-002.4's scratch-path rule). Each command prints its
`EXEMPT_VERIFY_OK:{task_id}` marker as its last statement, and each exits
non-zero with no marker while its deliverable is absent, which is the required
pre-work failure.

Test record. Stage exercised both commands in pwsh 7 with git only, and ran no Go
toolchain, linter, or build (P-010; the Orchestrator barred further probes in the
2026-10-12 dispatch): 85 cases in an ignored scratch path (T4: 5 positive and 35
negative; T5: 5 positive and 38 negative; plus the absent-deliverable pre-work case
for each), all as expected. The `go list ./...` call in T5 was answered by a
fixture-only `go` shim over a package list derived from `git ls-files`, and the
`backlogit docs lint` arm was stripped in the fixtures; neither arm has run for
real by Stage's hand. The scratch files stay in the ignored
`logs/stage-198s-scratch/` and are not committed. What Ship must therefore run
(A1.5 lists it, with expected results): the Step 4.1a pre-work probe, which exits
non-zero with no marker while the deliverable is absent but exits at the first
`Test-Path`, so it does not exercise those two arms, plus two stand-alone smoke
runs of the arms themselves, run before the first exempt dispatch; and the
post-deliverable gate, which is the first full execution. Outcomes and halt codes
follow P-002.2: a pre-work probe that is
not non-zero without the marker is `EXEMPT_FALSE_GREEN` or `EXEMPT_MARKER_MISSING`,
a post-deliverable difference is `EXEMPT_EVIDENCE_MISMATCH`, and contract drift is
`EXEMPT_CONTRACT_INCOMPLETE`. Stage's only remedy for any of them is to amend the
command text; the Orchestrator routes the halt to Stage for that or to the
operator.

Follow-up capture ordering (Orchestrator dispatch Q2). A stash capture writes the
tracked, non-Markdown `.backlogit/stash.jsonl`, which is outside the `docs-only`
and `verification-only` class surfaces (P-002.4), and `exempt_baseline_sha` is
captured before the task starts, so a capture made inside either task falls in the
gated delta. P-002.4 is not relaxed. Both gates therefore do not read the stash,
neither deliverable carries a stash ID, and the captures move to Ship's
post-merge closure (Ship Step 6, the item that stashes follow-ups identified by the
post-merge closure artifact; it is Ship's own follow-up stash grant, which P-021
C5 notes is separate from the Step 4.4a capture). Capturing there, rather than at
Step 5 item 10, keeps the capture after every gate and after the merge, so it
cannot advance the reviewed HEAD after the readiness and Copilot-review gates
(Step 5 items 4, 7b, and 7c), and the 197-S closure captured its follow-ups at the
same point. The procedure:

* Ship names the closure `docs/closure/198-S-198-F-post-merge-closure.md` (the
  pipeline-topology gate requires `{shipment}-{feature}-post-merge-closure.md`;
  stash BB669732 records the failure when it is misnamed), keeps the T5 evidence
  file `docs/closure/198-S-ct-full-suite-green-evidence.md` distinct, and cites
  that path in it.
* The closure lists T4's `FU-<n>` rows and, when the rerun branch was used, T5's
  flake, and any R3 detection-power finding (A1.4 R3), as follow-ups (the flake
  only when no active entry already tracks it: for 198-S, 63909DAE does, so Ship
  cites it and captures none; A1.8 E-8), so Step 6's follow-up item applies as written. The
  pre-merge closure artifact (Step 5 item 9) and readiness result must not list
  them (or an R3 detection-power finding) as follow-ups, or Step 5 item 10 would capture them early and duplicate
  the capture; they may be mentioned there only as "captured at Step 6".
* Ship creates one stash entry per item with `backlogit_create_item`
  (`artifact_type: "stash"`) and re-reads each. It does not retry after an MCP
  timeout: it waits at least 60 seconds, then checks `backlogit_fetch_stash` for
  the entry (a read that also times out is replaced by one direct read-only look at `.backlogit/stash.jsonl`, for example `Select-String` on the entry's text, because `backlogit stash list` and `backlogit_fetch_stash` can rewrite that file without a lock (stash DAEE9D03) and a rewrite racing a late-landing create can lose it; if that read cannot answer either, Ship halts through P-005 with the stash state recorded as unknown and creates nothing further; in 197-S reads timed out while the server was wedged, `docs/memory/2026-10-11/ship-197s-closure-partial-halt-memory.md`), because the first call can land after the check (in 197-S a
  timed-out `backlogit_stash` was retried after its marker looked absent, and the
  first call had landed as `BFB44A08`, a stale duplicate of `B53056C0`:
  `docs/closure/197-S-197-F-post-merge-closure.md`,
  `docs/memory/2026-10-11/ship-197s-closure-partial-halt-memory.md`;
  `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` records
  the same lesson for `ship_shipment`). A duplicate is recorded by ID for Stage,
  since Ship cannot remove entries. It does not triage or prioritize the entries
  (P-021 C5).
* Ship records the `FU-<n>`-to-stash-ID mapping in the closure artifact, which is
  authoritative (the closure archives 198.004-T and 198.005-T before this step, so
  a task comment is best-effort through `backlogit_append_comment`). Before it
  reports `DARK_MODE_COMPLETE`, and before the stash commit below, it re-reads each recorded ID through
  `backlogit_fetch_stash`, and the mapping must list exactly `SPIKE_FOLLOWUP_COUNT`
  IDs, plus one if T5 used the rerun branch and no active stash entry already
  tracks the flake, and one for each R3 detection-power
  finding. (When an active entry already tracks the flake, Ship cites it as the
  recurrence and creates none, as P-021 C2 asks; for 198-S that entry is 63909DAE,
  so the T5 term is zero: A1.8 E-8.) The check covers only those follow-ups: other closure captures (Step
  6's own follow-up item may stash more) and any duplicate Ship recorded are
  listed on a separate line and are outside the exact count. A mismatch, or a
  read that cannot answer, is a P-005 halt with the stash state recorded as
  unknown, not a retry.
* Ship commits the `.backlogit/stash.jsonl` write, and any hook-queue write and
  line-ending-only rewrite it causes, by explicit path on the post-merge closure
  branch, where it travels in the closure PR (the 197-S closure committed its stash write with its
  closure-docs commit). Ship makes no stash read after that commit, because a
  read can rewrite the file again; it checks `git status --porcelain` once
  afterwards, and a rewrite it shows is committed the same way.
* Until then the durable record is the deliverables committed on the shipment
  branch: T4's `FU-<n>` rows, T5's `CT_FLAKE_DESCRIPTION`, and the R6 note's
  `R3_FINDINGS` line. If the run halts
  before the closure, Stage reads them there. On T5's verification-incomplete
  branch Ship has no stash grant at a mid-wave halt, so it reports the failure
  (package, failed test names or panic text, run log path) through P-005 and
  halts with 198-S left active (A1.5), and Stage captures the entry when it triages the halt;
  the three isolation entries stay active, and Stage or the operator decides how
  the shipment then closes.

#### 198.004-T (T4), docs-only

```text
harness_exemption_class: docs-only
harness_exemption_reason: Adds only the new markdown spike findings document docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md that measures and classifies the runtime of already-shipped internal/core tests; changes no *.go file and has no behavior a failing test could pin.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$ErrorActionPreference = "Stop"; $f = "docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md"; if (-not (Test-Path -LiteralPath $f)) { Write-Error "T4 spike findings missing"; exit 1 }; $t = Get-Content -LiteralPath $f -Raw; $sc = { param($n) $ms = [regex]::Matches($t, "(?m)^" + $n + ": (\S.*?)\s*$"); if ($ms.Count -ne 1) { Write-Error "scalar $n must appear exactly once"; exit 1 }; $ms[0].Groups[1].Value }; $ct = { param($rel) if ([IO.Path]::IsPathRooted($rel)) { Write-Error "path must be repository-relative: $rel"; exit 1 }; $cwd = (Get-Location).Path; $full = [IO.Path]::GetFullPath((Join-Path $cwd $rel)); if (-not $full.StartsWith($cwd.TrimEnd([char]92) + [char]92, [StringComparison]::OrdinalIgnoreCase)) { Write-Error "path escapes the repository: $rel"; exit 1 }; git check-ignore -q -- $rel; if ($LASTEXITCODE -ne 0) { Write-Error "evidence path is not git-ignored: $rel"; exit 1 }; $full }; $rc = & $sc "SPIKE_RUN_COMMAND"; if ($rc -cnotmatch "^go test -json -count=1 -timeout 40m \./internal/core$") { Write-Error "run command malformed"; exit 1 }; if ((& $sc "SPIKE_HOST_CONDITION") -cnotmatch "^(alone|overlapped: \S.*)$") { Write-Error "host condition malformed"; exit 1 }; $tot = & $sc "SPIKE_TOTAL_PACKAGE_SECONDS"; if ($tot -cnotmatch "^[0-9]+(\.[0-9]+)?$") { Write-Error "total package seconds malformed"; exit 1 }; $total = [double]$tot; $rows = [regex]::Matches($t, "(?m)^\|[ \t]*([0-9]+)[ \t]*\|[ \t]*\x60?(\S+?)\x60?[ \t]*\|[ \t]*([0-9]+(\.[0-9]+)?)[ \t]*\|[ \t]*(real-subprocess|recovery-enabled-fixture|fsync-heavy|sleep-or-poll)[ \t]*\|[ \t]*\r?$"); if ($rows.Count -ne 25) { Write-Error "expected 25 ranked rows, found $($rows.Count)"; exit 1 }; for ($i = 0; $i -lt 25; $i++) { if ([int]$rows[$i].Groups[1].Value -ne ($i + 1)) { Write-Error "rank out of order at row $($i + 1)"; exit 1 }; if ($i -gt 0 -and [double]$rows[$i].Groups[3].Value -gt [double]$rows[$i - 1].Groups[3].Value) { Write-Error "durations not descending at row $($i + 1)"; exit 1 } }; if ($total -lt [double]$rows[0].Groups[3].Value) { Write-Error "total package time below slowest test"; exit 1 }; $names = @($rows | ForEach-Object { $_.Groups[2].Value }); if (@($names | Sort-Object -Unique -CaseSensitive).Count -ne 25) { Write-Error "ranked test names are not distinct"; exit 1 }; $jr = & $sc "SPIKE_RUN_EVIDENCE_PATH"; $jp = & $ct $jr; if ($jp -ieq (Resolve-Path -LiteralPath $f).Path) { Write-Error "run evidence cannot be the findings document"; exit 1 }; if (-not (Test-Path -LiteralPath $jp -PathType Leaf)) { Write-Error "run evidence json missing: $jr"; exit 1 }; $j = Get-Content -LiteralPath $jp -Raw; foreach ($n in $names) { $nj = $n.Replace("<", "\u003c").Replace(">", "\u003e").Replace("&", "\u0026"); if (-not [regex]::IsMatch($j, "\x22Test\x22\s*:\s*\x22" + [regex]::Escape($nj) + "\x22")) { Write-Error "ranked test absent from run evidence: $n"; exit 1 } }; $fc = & $sc "SPIKE_FOLLOWUP_COUNT"; if ($fc -cnotmatch "^(0|[1-9][0-9]?)$") { Write-Error "follow-up count malformed"; exit 1 }; $fr = [regex]::Matches($t, "(?m)^\|[ \t]*FU-([1-9][0-9]*)[ \t]*\|[ \t]*(\S[^|]*?)[ \t]*\|[ \t]*([0-9]+([ \t]*,[ \t]*[0-9]+)*)[ \t]*\|[ \t]*\r?$"); if ($fr.Count -ne [int]$fc) { Write-Error "follow-up rows do not match SPIKE_FOLLOWUP_COUNT"; exit 1 }; for ($i = 0; $i -lt $fr.Count; $i++) { if ([int]$fr[$i].Groups[1].Value -ne ($i + 1)) { Write-Error "follow-up rows are not numbered FU-1 upward"; exit 1 }; foreach ($rk in ($fr[$i].Groups[3].Value -split "[ \t]*,[ \t]*")) { if ([int]$rk -lt 1 -or [int]$rk -gt 25) { Write-Error "follow-up cites a rank outside 1 to 25"; exit 1 } } }; if ($fc -ceq "0") { $null = & $sc "SPIKE_NO_FOLLOWUP_REASON" } elseif ([regex]::IsMatch($t, "(?m)^SPIKE_NO_FOLLOWUP_REASON:")) { Write-Error "no-follow-up reason given alongside follow-ups"; exit 1 }; $l = backlogit docs lint --path $f --format json 2>$null | Out-String; if ($LASTEXITCODE -ne 0) { Write-Output $l; Write-Error "docs lint failed"; exit 1 }; if (-not (($l | ConvertFrom-Json).valid)) { Write-Output $l; Write-Error "docs lint not valid"; exit 1 }; Write-Output "EXEMPT_VERIFY_OK:198.004-T"; exit 0'
exempt_precondition: must-fail-before-deliverable
```

Declared evidence for T4 (maps to its acceptance criterion 1). The spike document
carries docline frontmatter like its sibling `docs/decisions/*.md` files
(`title`, `source`, `doc_type: decision`, `chunk_strategy: h1-h2-h3`, and `schema_version: "1.0"`; the last two are required of every tracked, in-scope `docs/**` Markdown file (`docline.Scope`; `docs/memory/` and `docs/archive/` are excluded) by `tests/integration/docline_soft_keys_test.go`, which neither `backlogit docs lint` nor this gate enforces, so a file without them would first fail in a full-suite run), one H1, and these column-0 manifest lines, each appearing
exactly once:

```text
SPIKE_RUN_COMMAND: go test -json -count=1 -timeout 40m ./internal/core
SPIKE_HOST_CONDITION: alone
SPIKE_RUN_EVIDENCE_PATH: <repository-relative, git-ignored path of the saved go test -json output>
SPIKE_TOTAL_PACKAGE_SECONDS: <decimal seconds for the package>
SPIKE_FOLLOWUP_COUNT: <n, 0 to 99, no leading zero>
```

plus a ranking table of exactly 25 rows shaped
`| <rank 1..25> | <test name> | <decimal seconds> | <class> |`, where `<class>` is
one of `real-subprocess`, `recovery-enabled-fixture`, `fsync-heavy`, or
`sleep-or-poll` (the four classes the T4 text names), and a follow-up table of
exactly `<n>` rows shaped `| FU-<k> | <one-line title> | <supporting ranks> |`,
where `<k>` runs 1 to `<n>` and the third cell is one or more ranks from 1 to 25
separated by commas. Rules the gate applies:

* The run command is exactly `go test -json -count=1 -timeout 40m ./internal/core`.
  Go's default per-package timeout is 10 minutes and the package takes about 28,
  so a `-timeout` is needed. The value follows the same reasoning as T5's (A1.3
  Timeout paragraph below): 40 minutes is task-local, and the earlier
  `-timeout 45m` equalled the 45-minute command limit, so a stall would have
  raced the circuit breaker.
* `SPIKE_HOST_CONDITION` is `alone` or `overlapped: <what ran alongside>`. The
  timings are load-sensitive (907.6 s quiet, per
  `docs/closure/197-S-197-F-post-merge-closure.md`, and 1656 s under suite load),
  so the document states which it measured. The gate checks only that the line is
  well-formed; it cannot verify the claim.
* The saved JSON must exist, be repository-relative and git-ignored, differ from
  the findings document, and contain `"Test":"<name>"` for every ranked name
  (names containing `"` or `\` are unsupported; `<`, `>`, and `&` are matched in
  their JSON-escaped form). It is a redirected run artifact in an ignored path
  (the circuit breaker caps diagnostic captures at 1 MiB, so it is never pasted
  into a transcript). The names are distinct, ranks are consecutive,
  seconds are non-increasing, and the package time is at least the slowest row.
  The gate proves shape, internal consistency, and that the ranked names are
  events in the saved output. It does not prove the timings, so a reviewer
  spot-checks the top rows against that output.
* The follow-up rows are numbered consecutively from `FU-1`, their count equals
  `SPIKE_FOLLOWUP_COUNT`, and each cites ranks that exist. A title cannot contain
  `|` (a row that does not match fails closed as a count mismatch). A count of `0`
  needs a `SPIKE_NO_FOLLOWUP_REASON: <one line>` line, which is rejected when
  follow-ups exist. The document carries no stash ID and the executor captures no
  stash entry inside the task.
* The table may list `TestUT1_ConcurrentSameItemCounterContract` next to the test
  it wraps; the duplicate is expected and is one finding.

Erratum for T4 acceptance criterion 1 (forced by P-002.4, which is not relaxed).
The criterion says the findings list "the stash IDs captured". A capture inside
the task would put `.backlogit/stash.jsonl` in the gated delta, so the findings
list the follow-up proposals (`FU-<n>`) and Ship records the stash IDs at the
post-merge closure, in the closure artifact and in a comment on 198.004-T (see the
ordering paragraph above). The capture obligation is unchanged and only moves
later and to Ship; the intent of the criterion (every recommended follow-up is
captured, with its ID on record) is met when that record exists, which Ship
confirms before it reports `DARK_MODE_COMPLETE`.

#### 198.005-T (T5), verification-only

```text
harness_exemption_class: verification-only
harness_exemption_reason: Executes go test ./... -count=1 -timeout 40m on the Windows host after CX merged and commits only one evidence artifact under docs/closure/ for behavior already shipped by CX and by prerequisite tasks 198.001-T through 198.003-T; never a new red assertion.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$ErrorActionPreference = "Stop"; $f = "docs/closure/198-S-ct-full-suite-green-evidence.md"; if (-not (Test-Path -LiteralPath $f)) { Write-Error "T5 evidence artifact missing"; exit 1 }; $t = Get-Content -LiteralPath $f -Raw; if ($t -cmatch "(?m)^[ \t]*(closure_status|compaction_status|gate_registration)[ \t]*:") { Write-Error "evidence artifact must not carry closure gate keys"; exit 1 }; $sc = { param($n) $ms = [regex]::Matches($t, "(?m)^" + $n + ": (\S.*?)\s*$"); if ($ms.Count -ne 1) { Write-Error "scalar $n must appear exactly once"; exit 1 }; $ms[0].Groups[1].Value }; $ct = { param($rel) if ([IO.Path]::IsPathRooted($rel)) { Write-Error "path must be repository-relative: $rel"; exit 1 }; $cwd = (Get-Location).Path; $full = [IO.Path]::GetFullPath((Join-Path $cwd $rel)); if (-not $full.StartsWith($cwd.TrimEnd([char]92) + [char]92, [StringComparison]::OrdinalIgnoreCase)) { Write-Error "path escapes the repository: $rel"; exit 1 }; git check-ignore -q -- $rel; if ($LASTEXITCODE -ne 0) { Write-Error "evidence path is not git-ignored: $rel"; exit 1 }; $full }; if ((& $sc "CT_RUN_COMMAND") -cne "go test ./... -count=1 -timeout 40m") { Write-Error "run command mismatch"; exit 1 }; $res = & $sc "CT_RUN_RESULT"; if ($res -cne "GREEN" -and $res -cne "GREEN-AFTER-RERUN") { Write-Error "run result is not green"; exit 1 }; $ex = & $sc "CT_RUN_EXIT"; $sha = & $sc "CT_RUN_HEAD_SHA"; if ($sha -cnotmatch "^[0-9a-f]{40}$") { Write-Error "run head sha malformed"; exit 1 }; git cat-file -e ($sha + "^{commit}") 2>$null; if ($LASTEXITCODE -ne 0) { Write-Error "run head sha is not a commit"; exit 1 }; git diff --quiet $sha -- "*.go" go.mod go.sum; if ($LASTEXITCODE -ne 0) { Write-Error "Go sources changed since the recorded run"; exit 1 }; if (@(git ls-files --others --exclude-standard -- "*.go").Count -gt 0) { Write-Error "untracked Go files present"; exit 1 }; $pp = & $ct (& $sc "CT_RUN_EVIDENCE_PATH"); if (-not (Test-Path -LiteralPath $pp -PathType Leaf)) { Write-Error "run evidence log missing"; exit 1 }; $log = Get-Content -LiteralPath $pp -Raw; if ($res -ceq "GREEN") { if ($ex -cne "0" -or $log -cmatch "(?m)^(FAIL|panic:|--- FAIL:)") { Write-Error "GREEN claim contradicted by exit code or log"; exit 1 } } else { if ($ex -cnotmatch "^[1-9][0-9]*$") { Write-Error "rerun result requires a failing first run"; exit 1 }; $rp = & $sc "CT_RERUN_PACKAGE"; if ((& $sc "CT_RERUN_COMMAND") -cne ("go test " + $rp + " -count=1 -timeout 40m")) { Write-Error "rerun command mismatch"; exit 1 }; $rx = & $sc "CT_RERUN_EXIT"; $rl = & $ct (& $sc "CT_RERUN_EVIDENCE_PATH"); $fd = & $sc "CT_FLAKE_DESCRIPTION"; $ff = @([regex]::Matches($log, "(?m)^FAIL[ \t]+(\S+)") | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique); if ($ff.Count -ne 1 -or $ff[0] -cne $rp) { Write-Error "first-run failures are not exactly the rerun package"; exit 1 }; if ([regex]::Matches($log, "(?m)^--- FAIL: \S+").Count -lt 1 -or $log -cmatch "(?m)^panic: test timed out") { Write-Error "first-run failure is a timeout or names no test"; exit 1 }; if (($rp -cmatch "/internal/core$" -and $log -cmatch "--- FAIL: (TestUT[12]_|TestAppendGateEvidence_ConcurrentSameItem)") -or ($rp -cmatch "/internal/release$" -and $log -cmatch "--- FAIL: (TestUT3_|TestClientLatestRespectsContextTimeout)")) { Write-Error "first-run failure is inside CT scope"; exit 1 }; if ($rx -cne "0" -or -not (Test-Path -LiteralPath $rl -PathType Leaf)) { Write-Error "rerun evidence missing or not green"; exit 1 }; $rlog = Get-Content -LiteralPath $rl -Raw; if ($rlog -cmatch "(?m)^(FAIL|panic:|--- FAIL:)" -or $rlog -cmatch "\[no tests to run\]" -or $rlog -cnotmatch ("(?m)^ok[ \t]+" + [regex]::Escape($rp) + "[ \t]")) { Write-Error "rerun log not green for package"; exit 1 }; foreach ($fn in @([regex]::Matches($log, "(?m)^--- FAIL: (\S+)") | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique)) { if (-not [regex]::IsMatch($fd, "(?<![A-Za-z0-9_])" + [regex]::Escape($fn) + "(?![A-Za-z0-9_])")) { Write-Error "flake description does not name failed test: $fn"; exit 1 } }; $log = $log + [Environment]::NewLine + $rlog }; if ($log -cmatch "(?m)^ok[ \t]+\S+[ \t]+\(cached\)") { Write-Error "run log contains cached results"; exit 1 }; if ($log -cmatch "\[no tests to run\]|testing: warning: no tests to run") { Write-Error "run log shows a filtered or partial run"; exit 1 }; $pk = @(go list -mod=readonly ./... 2>$null); if ($LASTEXITCODE -ne 0 -or $pk.Count -lt 1) { Write-Error "go list failed"; exit 1 }; foreach ($q in $pk) { if ($log -cnotmatch ("(?m)^(ok|\?|FAIL)[ \t]+" + [regex]::Escape($q) + "[ \t]")) { Write-Error "package missing from run log: $q"; exit 1 } }; $ic = [regex]::Match($log, "(?m)^ok[ \t]+\S*/internal/core[ \t]+([0-9]+(\.[0-9]+)?)s[ \t]*\r?$"); if (-not $ic.Success -or (& $sc "CT_INTERNAL_CORE_SECONDS") -cne $ic.Groups[1].Value) { Write-Error "internal/core time does not match the run log"; exit 1 }; $oks = [regex]::Matches($log, "(?m)^ok[ \t]+(\S+)[ \t]+([0-9]+(\.[0-9]+)?)s[ \t]*\r?$"); if ($oks.Count -lt 1) { Write-Error "no ok package lines in run log"; exit 1 }; foreach ($o in $oks) { if ($t -cnotmatch ("(?m)^\|[ \t]*" + [regex]::Escape($o.Groups[1].Value) + "[ \t]*\|[ \t]*" + [regex]::Escape($o.Groups[2].Value) + "[ \t]*\|")) { Write-Error "per-package row missing: $($o.Groups[1].Value)"; exit 1 } }; if ((& $sc "CT_ISOLATION_FIXES_HELD") -cne "DB48A817, FF1F3AC7, 885263D2") { Write-Error "isolation stash ids not named"; exit 1 }; $l = backlogit docs lint --path $f --format json 2>$null | Out-String; if ($LASTEXITCODE -ne 0) { Write-Output $l; Write-Error "docs lint failed"; exit 1 }; if (-not (($l | ConvertFrom-Json).valid)) { Write-Output $l; Write-Error "docs lint not valid"; exit 1 }; Write-Output "EXEMPT_VERIFY_OK:198.005-T"; exit 0'
exempt_precondition: must-fail-before-deliverable
```

Declared evidence for T5 (maps to its acceptance criteria 1 and 2). T5 runs
`go test ./... -count=1 -timeout 40m` once, after CX has merged (197-S is
shipped and is the branch base) and after T1 through T3 are done, with no other
`go test` run, build, or heavy host activity during it, and saves the
full output to a repository-relative, git-ignored path (for example under
`logs/`). The evidence artifact is the T5 deliverable. Its name must not end in
`-post-merge-closure.md` and it carries no `closure_status`, `compaction_status`,
or `gate_registration` key, so no closure gate can mistake it for the 198-S
closure. It carries docline frontmatter (`title`, `source`, `doc_type: closure`, `chunk_strategy: h1-h2-h3`, and `schema_version: "1.0"`; the last two are required by `tests/integration/docline_soft_keys_test.go`, which neither `backlogit docs lint` nor this gate enforces; as in
`docs/closure/195-S-claim-start-proof-positive.md`, with any `#` in a value
quoted) and these column-0 lines, each appearing exactly once:

```text
CT_RUN_COMMAND: go test ./... -count=1 -timeout 40m
CT_RUN_RESULT: GREEN
CT_RUN_EXIT: 0
CT_RUN_HEAD_SHA: <git rev-parse HEAD at run start, with no Go changes since>
CT_RUN_EVIDENCE_PATH: <repository-relative, git-ignored path of the saved run log>
CT_INTERNAL_CORE_SECONDS: <seconds exactly as the internal/core ok line prints them, trailing zeros kept, without the trailing s; from the rerun log when internal/core was the rerun package>
CT_ISOLATION_FIXES_HELD: DB48A817, FF1F3AC7, 885263D2
```

and one table row per package that printed an `ok` line, shaped
`| <import path as printed> | <seconds as printed, without the trailing s> | ok |`.
The committed rows are the durable record. The run log is session-local and
ignored, so it corroborates the rows but cannot be re-read from a fresh clone. It
is the command's complete output, not a summary or tail. The command carries no
`-v` and no `-json` (the rule's two allowed forms), which is a deliberate deviation from rule 2 of
`docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`: `-v` would put
every test's stdout at column 0 and could trip the gate's `panic:` and `FAIL`
checks, and `-json` would replace the `ok` lines the gate reads. A test panic usually prints `--- FAIL: <name>` and Go's timeout panic
lists the running tests, but a panic in a background goroutine can name no test,
as in the 197-S unnamed-panic case (stash `63909DAE`,
`docs/closure/197-S-197-F-post-merge-closure.md`, follow-up table). Such a run is not
rerunnable here and ends T5 as verification-incomplete after a run of about 28
minutes, with the failure reported and not classified; T4's `-json` run (wave 1) is an
earlier chance to name a recurring panic. What the gate checks beyond the scalars:
the run SHA is a commit, and no `*.go`, `go.mod`, or `go.sum` content differs
between it and the working tree and no untracked Go file exists; the log has no
`(cached)` result and no `[no tests to run]` or `testing: warning: no tests to run`
text, because an unfiltered run prints neither for any package in this repository
(`git grep` over every `*_test.go` directory finds no package with test files and
no test function except a `testdata` fixture; if one appears later, the gate halts
and the command text needs amending); every package that `go list ./...` reports
has an `ok`, `?`, or `FAIL` line in the log (so a partial or single-package log
fails); and every `ok` line has its row, with the `internal/core` and row matches
end-anchored. The three
isolation stash IDs are named literally and are not resolved against the stash,
because Stage archives them after this evidence exists. For a `GREEN` run the
artifact also states, in prose, that the DB48A817, FF1F3AC7, and 885263D2
isolation fixes held. For a `GREEN-AFTER-RERUN` run it must not state that: A1.8
E-8 takes precedence and says what is recorded instead. Either way it states
what else was running on the host during the run (the gate cannot check either
statement).
Ship's post-merge closure cites this artifact's path.

Under the existing red-run policy, a green rerun is recorded with
`CT_RUN_RESULT: GREEN-AFTER-RERUN`, a positive `CT_RUN_EXIT`, and the added
lines `CT_RERUN_PACKAGE: <import path>`,
`CT_RERUN_COMMAND: go test <import path> -count=1 -timeout 40m`,
`CT_RERUN_EXIT: 0`, `CT_RERUN_EVIDENCE_PATH`, and `CT_FLAKE_DESCRIPTION: <one
line naming the package and every failed test>`. There is no stash ID in the
artifact (see the ordering paragraph above). The gate accepts that branch only
when the first-run log shows exactly one failing package, equal to
`CT_RERUN_PACKAGE`; at least one named top-level `--- FAIL:` test and no timeout
panic; no failure in a T1 to T3 test (matched by name inside that test's own
package: `internal/core` for T1 and T2, `internal/release` for T3); a
`CT_FLAKE_DESCRIPTION` that names each failed top-level test as a whole
identifier (so `TestRunAll` does not satisfy a failed `TestRun`); and a rerun log
that is green for that package, with the pinned unfiltered command and no
`[no tests to run]`. In prose the artifact names the failed test and says whether
it falls in the scope of the three isolation fixes, because a package that fails
under suite load and passes alone is a load-sensitivity finding, not a waiver
(`docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`), and Stage
reads "held" only from that statement. The gate's CT-scope name match lists only the T1, T2 (`TestUT2_`), and T3 tests, not every test T2 edits (T2 edits three files, A1.8 E-2, and its AC2 pattern matched no test, A1.8 E-1), so the artifact also says whether the failed test is defined in a file that T1 to T3 edited (`git diff --name-only c5758918 HEAD`, the branch base that R2 names); a failure in such a file is a CT-scope failure, and Ship treats it as verification-incomplete (the Erratum path), not as a flake. Ship runs that check itself between the Step 4.3 gate and Step 4.4 (A1.5), because Step 4.4 is the report-only review skill; the command does not check it. Ship captures the flake as a stash entry, unless an active entry already tracks it (then Ship cites that entry: A1.8 E-8),
at the post-merge closure, as described above. A quiet-host attestation, an
`internal/core` exclusion, and an `Elapsed` cross-check against the saved
output were considered and not adopted: the gate cannot verify the first, the
red-run policy allows a rerun of any one package, and the third is unverified
against real output.

Timeout (Orchestrator dispatch Q3). The pinned `-timeout` is 40 minutes, not the 30
the plan text gave. The worst observed `internal/core` time is 1656 s (27.6
minutes, under suite load, from the stash 8F41D60D record). Against a 30-minute
budget that is 92% used and 8% headroom, which is the plan's own problem frame;
against 40 minutes it is 69% used and 31% headroom, which meets the Orchestrator's
direction (at least 30% headroom, at most 45 minutes). Two cautions. First, this
is a run parameter of T5's own verification, not a change to the governed
full-suite budget in P-002.6 (073-DL), which stays `-timeout=30m` for Ship's gates
and changes only through a Stage deliberation; the CT deliberation's D4 said CT
does not change the timeout, and T5's pass does not substitute for those gates.
Second, 40 minutes is 1.45 times the largest package, below the 1.5 times that
073-DL's adjustment criteria use; the value is Stage's choice within the Orchestrator's Q3 bounds (at
least 30% headroom, at most 45 minutes), and 073-DL's criteria govern the
governed budget, which this is not. A 42-minute value would meet 1.5 times but
leaves only 3 minutes under the command limit. `-timeout` is per test binary and starts
after the build, and `go test ./...` runs packages in parallel, so wall time is
roughly the build plus the slowest package: the 2026-10-08 run took 28m41s
against 1656 s for `internal/core`, about 65 s of warm overhead; a cold build is
unmeasured. 40 minutes plus an assumed build allowance of 4 minutes still fits the 45-minute
limit (`circuit-breaker.instructions.md`), and cmd/go arms a backup kill one minute after the timeout, so a hung run's worst case is 40 + 1 + 4 = 45 minutes, at the limit and not over it. Implication for stall detection: a
genuinely hung test is now cut at 40 minutes instead of 30, so a stall costs up
to 10 more minutes before Go's timeout panic lists the hung test. A timeout is
still not a rerun case (it names no failing test), so it still ends T5 as
verification-incomplete. The 30-minute value remains in two places this amendment
does not own: Ship Step 4.6 item 3 and Step 5 run `go test -timeout=30m ./...`,
and those gates keep their 92% exposure (stash F8936891).

Erratum. A red outcome that the red-run policy classifies as
verification-incomplete (a second failure of the same package, or a failure with
no named test such as a package timeout or a build failure, a first run that fails in two or more packages, or a failure inside CT's scope) cannot pass this gate, because a gate
that exits 0 over a red suite would be vacuous. The one case the gate accepts is a
failed test defined in a file T1 to T3 edited whose name is not in the gate's
CT-scope list (for example a `TestShipShipment_` test); Ship's A1.5 step 2 check
rejects it. The plan's "record CT closure as
verification-incomplete" branch is therefore not a completion of T5. Ship leaves
T5 non-done, reports the failed gate through P-005 with the package, the failed
test names or panic text, and the run log path, and halts with 198-S left active and unchanged (A1.5; no `backlogit_return_blocked` call); Stage
captures the policy's new stash entry when it triages the halt (Ship's stash grants
do not cover a mid-wave halt), the three isolation entries stay active, and Stage
or the operator decides how the shipment then closes, which is outside A1. Ship
Step 4.6
item 3 and Step 5 also run `go test -timeout=30m ./...` with no tolerated red and
no rerun allowance, so a flake that passes T5's rerun can still halt those gates;
CT does not widen to fix it. Two further errata follow from the ordering and
timeout changes. T5's text and `harness_exemption_reason` say `-timeout 30m`; the
run is now `-timeout 40m` (Q3), and the task body is edited to match. The
red-run policy says the flake is "captured as a new stash entry"; the capture
moves from inside T5 to Ship's post-merge closure (Q2), and the artifact's
`CT_FLAKE_DESCRIPTION` is the gated record of what is to be captured.

Delta surface. A1 does not change P-002.4, and each exempt task's deliverable is
exactly one named file. Backlog-state writes under `.backlogit/` that Ship's own
claim and completion steps make (claim state, labels, hook events) are not
deliverables and follow the 197-S practice: Ship commits `.backlogit/**`
carry-forward as separate commits before capturing the baseline, makes no tracked
backlog write between the baseline and the post-commit gate (A1.5), and keeps
evidence in task comments. Follow-up stash captures were the one new write class
(`.backlogit/stash.jsonl` is tracked and is not Markdown), and the ordering
paragraph above moves them out of the gated delta rather than relaxing the
policy, so neither gate can count them. The P-002.4 path pass still has no stated
`.backlogit/**` carve-out for the housekeeping writes above; stash 253E2505
tracks that general gap, and it no longer blocks CT.

### A1.4 T1 and T3 red phase: operator ruling recorded (Option A)

Governing text. P-004's Violation Action is "Do NOT apply `harness-ready` label.
Halt and report the failure", and its postcondition is a manifest recording
`Red Phase: CONFIRMED` for tests that fail with expected markers.
`docs/decisions/2026-08-29-p002-breach-incident-131s-148f.md` (INC-P002-131S-148F,
"Forward Control Amendment") makes FC-1 through FC-4 mandatory "for any future
Ship release unit until the operator explicitly relaxes them". FC-1 requires the
red harness in a separate commit ahead of the implementation commit. FC-2 step 4
requires the task's tests to fail non-zero at the red-harness commit, and a
failed preflight means the task "MUST NOT be claimed". FC-3 requires the closure
to confirm the red phase was observed to fail.
`docs/decisions/2026-08-30-p002-breach-incident-152f-134s.md` adds that its
acknowledgement cannot be cited as precedent for accepting or normalizing a
P-002 violation. P-017 says dark mode "is not a waiver of safety policies". The
Orchestrator's dispatch asked Stage to rule on Ship's deviation, and that request
was not the operator's explicit relaxation; the operator's ruling recorded below
is. FC-4 is not engaged (no CT task touches `syncWriteFileAtomic` callers). FC-5
(no production stub lands) is engaged and met by construction: R2 proves no
production file changes.

Facts that make the literal wording ("tests fail before implementation")
unsatisfiable for T1 and T3 here. It is not unsatisfiable for tests-only
stabilizations in general: 181.001-T shipped in 182-S under normal P-002 with a
reproduce-first red
(`docs/closure/182-S-181-F-post-merge-closure.md`,
`docs/decisions/2026-09-28-ur3-crash-ready-handoff-flake-deliberation.md`). Here:

1. T1 and T3 write no production code. T1 AC3 forbids it, and the plan's
   Constitution Check records no production behavior change.
2. The behavior each guards already shipped: the 106-F unlock-after-append fix
   (`appendGateEvidence` in `internal/core/gate_transition.go` defers the counter
   unlock obtained through `augmentDeltaWithFormalProof` so it covers the append)
   and the
   caller-context propagation in `Client.Latest` (`internal/release/release.go`).
   No assertion over the production tree can fail for the intended reason, and
   the pre-deliverable failure was host-load dependent.
3. Neither unit is `harness-exempt`. Both edit pre-existing assertions, which the
   `verification-only` content pass forbids, and both guard behavior, so
   `docs-only` does not fit. An exempt task with an empty delta also halts
   (P-002.4), and these deliverables are already committed.
4. A fabricated failing assertion is forbidden (P-002.1, harness-architect
   guardrails).
5. Commit `9762e75f` (the wave-1 harness scaffold) already contains the T1 and
   T3 test edits. Here the harness and the deliverable are the same edit, so both
   tasks start green, and T3 has no deterministic red without a production seam.

Ruling. The operator ruled on 2026-10-12 and the ruling is recorded verbatim on
the line below (a single column-0 line outside any code fence; the key appears at
column 0 nowhere else in this plan):

a1_4_operator_ruling: T1=A; T3=A: Ruling: option A (operator; 2026-10-12T17:35:08Z; Copilot CLI session a3f32a34-316b-4580-965d-f0a3e5ca2ab4)

How it was received and read. The operator's words are "Ruling: option A", sent
in the Copilot CLI session named above at 17:35:08Z (10:35 PDT) and relayed with
those words quoted by the Orchestrator in its dispatch to Stage. They answered
the joint question A1.7 item 1 put for T1 and T3 together, with no task
qualifier, and they chose neither B (which applies to T1 only) nor C. The
Orchestrator's reading is therefore that Option A applies to both tasks, which is
the only reading consistent with the options as written, and the line records it
as `T1=A; T3=A`. The operator can narrow or revoke it with a new ruling line and
a reviewed amendment. The retrievable record of the operator's message is that
session at that timestamp. A bare "as recommended", a P-017 dark-mode activation,
the 2026-10-08 `operator_authorization` above, and an Orchestrator paraphrase
without the operator's quoted words would not have counted; this ruling quotes
the operator's own words and names the option by letter.

Operator confirmation of 2026-10-13. Ship's pre-PR review halted on governance
items (`docs/memory/2026-10-13-ship-198s-prepr-review-halt.md`, decisions D-1 to
D-8). The operator answered in the same Copilot CLI session, in a message at
2026-10-13T03:26:01Z (2026-10-12 20:26 PDT), with exactly these words: "D1 yes,
D2 yes, D3 yes, D5 delete, D7 accept, D8 ratify". That is six answers. Read
against Ship's memo (the Orchestrator's reading, stated for the record), they
mean: D-1, the operator confirms the plan-review ADVISORY approval in the final
`## Plan Review`; D-2, Option A's wording reaches Principle II's ordering
requirement, Development Workflow item 1, and P-002's own red-phase statements,
for 198.001-T and 198.003-T only; D-3, A1.7 item 4(c) and 4(d) are within Ship's
boundary; D-5, the reply "D5 delete" answered a prompt that described both the
`%TEMP%\stage-198s` fixtures and Stage's first-revision linter, `backlogit docs
lint`, and `go list` runs (Ship's memo words it: "Stage wrote fixtures under
`%TEMP%\stage-198s`, outside the workspace (`stage-198s` directory retained, not
deleted). Stage also ran linters, `backlogit docs lint`, and `go list` in the
first revision. The plan's A1.6 records Principle IV as "pass". Operator
disposition needed; the Constitution Check should record a violation."); the
reply names deletion, and Stage reads nothing more about the P-010 runs into it
(the operator accepted those separately, below), and the directory was then found
already absent (A1.6); D-7, the 2026-10-11 in-tree production mutation is accepted
as a disclosed event; D-8, the second plan-review series is ratified. The
operator did not answer D-4 or D-6. They are the Orchestrator's in-scope judgment
calls under the dark-mode activation, not operator decisions (A1.8 E-1 and E-8).

Later on 2026-10-13, at 2026-10-13T14:20:54Z in the same session, the operator
answered two decisions of Ship's round-2 memo
(`docs/memory/2026-10-13-ship-198s-delta-review-and-readiness.md`, section 5)
with exactly "02: Accepted" and "03: Accepted". Read as that memo's O2 and O3 (the
Orchestrator's reading): O2, Stage's first-revision P-010 runs (linters,
`backlogit docs lint`, `go list`, and pwsh fixtures) are accepted as a disclosed
P-010 event with no re-execution; O3, the A1.8 errata as corrected by the
attribution fix of 2026-10-13 (O1) are accepted as residual risk. The
Orchestrator decided O1, O4, and O5 under the dark-mode activation (A1.7 item 9).
With these, no decision from Ship's O1 to O5 remains open. Each answer has its
own sentinel key, so the ruling line above stays the only column-0 line that
matches the ruling's acceptance rule:

a1_4_operator_confirmation: D-1=yes; D-2=yes (T1 and T3 only); D-3=yes; D-5=delete; D-7=accept; D-8=ratify: "D1 yes, D2 yes, D3 yes, D5 delete, D7 accept, D8 ratify" (operator; 2026-10-13T03:26:01Z; Copilot CLI session a3f32a34-316b-4580-965d-f0a3e5ca2ab4)

a1_4_operator_acceptance: O2=accepted (Stage's first-revision P-010 runs, no re-execution); O3=accepted (A1.8 errata as corrected by O1): "02: Accepted" and "03: Accepted" (operator; 2026-10-13T14:20:54Z; Copilot CLI session a3f32a34-316b-4580-965d-f0a3e5ca2ab4)

What the ruling covers. Option A, as quoted below from this section as it stood
when the operator chose it, relaxes for T1 and T3 only: P-004's literal wording
and its `Red Phase: CONFIRMED` postcondition token, FC-1's separate-commit
ordering, FC-2 step 4, and FC-3's red-observed and separate-harness-SHA
confirmations, and nothing else. FC-2 step 2 is not relaxed. The ruling does not
relax Constitution Principle II, FC-4, FC-5, P-002.1 or P-004 for any other task
(T2 keeps its genuine assertion red), or any other release unit (R1 makes it
non-precedent). The operator's confirmation of 2026-10-13T03:26:01Z (D-2, in the
confirmation line above) removes the one question this paragraph first left open.
It extends Option A's reach, for 198.001-T and 198.003-T only, to (i) Principle
II's ordering requirement, (ii) Development Workflow item 1 ("Harness before
code"), and (iii) P-002's own red-phase statements ("its tests fail in the red
phase", and the per-wave postcondition that all of a wave's tests fail with
expected markers), which state the same requirement as P-004's wording. It is not
a general relaxation: it waives nothing for T2, T4, T5, any other task, or any
other release unit, and Principle II stays in force for all production code,
which CT adds none of (R2). The conditions R1 through R6 below are the terms on
which the ruling and the confirmation apply.

Stage's determination after the ruling.

* The procedural hold from the first revision (the Orchestrator not dispatching
  Ship, and a shipment block if one was applied) lifts when the final
  `## Plan Review` is PASS (or an ADVISORY the operator accepts) and the
  Orchestrator unblocks shipment 198-S (A1.7 PA-2). If Ship reaches T1 or T3
  before then, the stop is `DARK_MODE_HALTED` with a P-005 event naming P-004,
  FC-2, and this section.
* Stage's assessment, now a recommendation the operator has accepted: mutation-kill
  is guard-retention evidence, which is what T1 AC2 and T3 AC2 ask for, and it is
  stronger than green on arrival. Stage's P-005 record of the gap is
  `docs/memory/2026-10-12-stage-198s-exempt-amendment.md`; no intercom is
  installed, so visibility is degraded.
* Ship's `harness-ready` labels on both tasks stay in place; the ruling is what
  supports them, not Ship's earlier claim. Stage withdrew nothing (A1.7 PA-3).

Principle II conflict record (Governance requires the principle, the
justification, and the simpler alternative rejected), with the VII and VIII
checks.

* Principle II (NON-NEGOTIABLE). Its operative rule is "never write production
  code before the corresponding test exists and has been observed to fail." CT
  writes no production code, and R2 proves that mechanically at three points. For
  a unit with no implementation the sequence "test, red, implement, green" has no
  implement step to order. What the principle protects is that a test is observed
  to detect the defect it guards, and R3 keeps that observation: the red is
  observed on a mutated copy, not on the committed harness, and not before the
  test existed. What Option A gives up is that ordering, which P-004 and FC-1 to
  FC-3 (the Enforcement Model's arm for this principle) enforce and the operator
  relaxed. The Development Workflow's item 1 ("Harness before code") states the same ordering as a MUST and is departed from as a consequence of the same relaxation, which the ruling did not name and the operator's confirmation of 2026-10-13 (D-2) covers for T1 and T3 only. That departure is an operator-authorized deviation, recorded in A1.6, not a carve-out. Simpler alternatives rejected: Option B (a genuinely red test-only
  helper harness) adds helper code the plan does not call for, and T3 has no
  deterministic red without a production seam; Option C (descope) changes M. No
  carve-out from the principle itself is recorded (P-002.1 withdrew
  `declaration-only` for being one), because no production behavior is added or
  changed.
* Principle VII. R6 creates files under an ignored scratch path and deletes
  nothing; the executor must not delete that directory, because deletion needs
  operator approval. It runs no destructive command.
* Principle VIII. The mutation experiment is elevated-risk work (it perturbs the
  behavior under test). R6 runs it by `go test -overlay`, so no production file
  changes, with `careful` mode then a `freeze-scope` boundary.

Options B and C were not chosen and have no effect; each remains available only
through a new ruling line and a reviewed amendment.

Option A as it stood when the operator chose it (quoted from this section's first
revision, which was never committed; the acceptance rule applied to the ruling line
above is: a single column-0 line outside any code fence matching
`^a1_4_operator_ruling: T1=(A|B|C); T3=(A|B|C): ` followed by the operator's words,
an ISO-8601 timestamp later than `2026-10-12T06:30:00Z`, and a retrievable
channel reference; text elsewhere that mentions the key is not a ruling, a
duplicate line is void, and `T3=B` is invalid):

> **Option A.** For that task only, relax every control A1.4's facts leave unmet:
> P-004's literal wording and its `Red Phase: CONFIRMED` postcondition token,
> FC-1's separate-commit ordering, FC-2 step 4, and FC-3's red-observed and
> separate-harness-SHA confirmations. The operator's words should name them.
> Principle II is not itself relaxed. Release: Stage adds the run mechanics (R6) in
> a recording amendment, which must pass plan review with a Principle II and VIII
> check (A1.6 covers A1 as written only); then Stage confirms `harness-ready` is
> on that task and re-applies it if it was removed.

Post-ruling note, not part of the quotation: this amendment is that recording; its
Principle II, VII, and VIII check is above, and its review is the final
`## Plan Review` section. The operator's words did not enumerate the controls, so
the enumeration above is the relaxation (adopted by choosing the option by letter),
and the operator's confirmation of 2026-10-13 (D-2) confirmed that reading and its
reach to Principle II's ordering, Development Workflow item 1, and P-002's red-phase
statements for T1 and T3.
Option B (T1 only: revert the T1 test edit and scaffold a genuinely red
outcome-classifier harness) and Option C (descope through a reviewed amendment
that changes M and 198-S) are recorded in the decision note.

Relaxation conditions, in force for T1 and T3 under the ruling. A failure of any is
a P-004 halt that Ship records through P-005 and the Orchestrator routes to the
operator (or to Stage when only plan text must change).

* **R1 Scope.** 198.001-T and 198.003-T only. Not precedent for another task or
  release unit; it lapses at CT closure (the 198-S post-merge closure). A policy
  fix promoted from stash 920BCA5B would supersede it.
* **R2 No production delta.** Ship runs, and records the result in a comment on
  each task, at three points: after the R6 experiment (R6 step 5), at the Step 4.6
  gate for each wave, and before the PR is created. Each of these exits 0 and
  prints nothing:
  `git diff --name-only c5758918 -- '*.go' go.mod go.sum ':(exclude)*_test.go'` and
  `git ls-files --others --exclude-standard -- '*.go' ':(exclude)*_test.go'`.
  `git diff --name-only c5758918 -- '*.go'` must list the `9762e75f` test files,
  as a positive control, so an erroring git cannot pass. `c5758918` is the branch
  base; use `git merge-base origin/main HEAD` if main is merged in later.
* **R3 Evidence of the intended failure.** A named executor (R6 step 1) runs the
  experiment once per task before the wave-1 dispatch, so the evidence exists when
  Ship Step 4.0 asks whether every task is harness-satisfied, and records the
  mutation, the exact mutant command (R6 step 3), the non-zero exit, and the
  `--- FAIL:` lines. T1's mutation is the 106-F fix: the deferred unlock in
  `appendGateEvidence` (`internal/core/gate_transition.go`) moved ahead of the
  append. T3's is `Client.Latest` passing `context.Background()` to
  `LatestRelease` (`internal/release/release.go`). The failure must be the
  intended assertion: T1 `duplicate counter <n> durably persisted`; T3
  `An error is expected but got nil.` (`release_test.go:102` at `9762e75f`). A
  build error, panic, timeout, or other assertion is not a kill. T1 asks for ten
  iterations that all fail, because it is a race (Ship's scaffold-time evidence was
  ten of ten); T3 asks for three. Zero intended-assertion failures, or a wrong
  assertion, halts. A ratio below all-fail is recorded with the observed count and
  is a detection-power finding that Ship reports without halting, because the T1
  window is probabilistic and a lost race is a possible cause; it is captured with
  the closure follow-ups. Each task is
  one invocation with `-count=N` rather than N invocations, because the circuit
  breaker counts every non-zero native exit and stops at three
  (`.github/instructions/circuit-breaker.instructions.md`); iterations share a
  process, so process-level independence is given up. The unmutated control (R6
  step 4) exits 0 with `-v` and prints
  `--- PASS: TestUT1_ConcurrentSameItemCounterContract` (T3:
  `TestUT3_ClientLatestRespectsContextTimeout`), and `go test -list` with the
  full anchored name lists exactly that one test (followed by its `ok` summary
  line).
* **R4 Evidence binding.** The evidence holds only while
  `git diff --quiet 9762e75f -- internal/core/gate_evidence_formal_test.go internal/release/release_test.go`
  exits 0. That one-argument form covers staged and unstaged edits. Ship runs it at
  the same three points as R2 and records it with them. A non-zero exit is a halt.
* **R5 Record (P-005 and FC-3).** The record for each task states
  `Compilation: PASS`,
  `Red Phase: MUTATION-KILL-CONFIRMED (P-004/FC-1/FC-2.4/FC-3 relaxed by the operator, a1_4_operator_ruling 2026-10-12T17:35:08Z)`
  and never the bare token `Red Phase: CONFIRMED`, and
  `Delivered-tree result: GREEN (expected; tests-only stabilization)`. If
  harness-architect's earlier manifest comment on either task used the bare
  `Red Phase: CONFIRMED` token for the mutation-kill, Ship appends a correcting
  comment saying that token meant mutation-kill under this ruling, not a red
  observed on the committed harness. The evidence goes in one task comment per
  task and in a committed `docs/memory/` note a reviewer can read (R6 step 6). The
  executor's identity and model are in the note (the dispatch pin, if the model
  is not exposed). The executor model could not be attested for 198-S; the
  Orchestrator accepted that as disclosed residual risk on 2026-10-13 under the
  dark-mode activation (O5), and an operator spot-check of the `--- FAIL:` lines
  in the T1 and T3 mutation runs is optional and available (the same model
  family wrote the tests). The PR body
  carries a compliance annotation naming 198.001-T,
  198.003-T, this section, both incidents (INC-P002-131S-148F and
  INC-P002-152F-134S), the Stage process events (A1.7 item 4: the Principle IV
  violation, found absent, and the first-revision P-010 event, accepted by the
  operator without re-execution), the 2026-10-11 in-tree production mutation
  (A1.7 item 7), D-4 and D-6 as the Orchestrator's in-scope calls, and the
  executor-model residual risk. Ship's
  closure P-002 compliance section names INC-P002-131S-148F, states, with SHA
  `9762e75f`, that the red phase was not
  observed on the committed harness and the basis was mutation-kill, and
  lists T4's and T5's exemption class and gate evidence (FC-3). Dark mode does not
  guarantee a human reads the annotation before a pre-authorized merge, so it is
  audit evidence, not an approval gate.
* **R6 Run mechanics (this amendment defines them; Stage ran none of it, P-010).**
  1. **Executor, mode, and scratch path.** Ship dispatches a general-purpose
     subagent (not build-feature, whose Commit step stages all changes and whose
     start-record ordering expects a task dispatch) or, if present, the operator.
     Ship's Role Boundary lists delegation of reads and writes "to build/fix
     skills", and this executor writes only ignored scratch files and no source;
     A1.7 item 4(d) asked the operator to confirm that reading, and the operator
     confirmed it on 2026-10-13 (D-3, A1.4 confirmation line), so the executor is
     within Ship's boundary. The executor invokes the `safety-modes` skill in
     `careful` mode (the risk
     list goes in the evidence note), then `freeze-scope` with write access to
     `logs/_ct-mutation/` only and read access to the two production files. Ship
     does not write the scratch files or any source. The leading underscore makes
     `./...` skip the directory, so `go list`, `go vet`, `go test`, and
     `golangci-lint` see no stray package; `logs/` is ignored in `.gitignore`, and
     `git check-ignore -v logs/_ct-mutation/x` must report it. Never delete
     `logs/_ct-mutation/` afterwards (Principle VII); it is ignored and stays.
  2. **Mutant copies, no production edit.** For T1, copy
     `internal/core/gate_transition.go` to `logs/_ct-mutation/gate_transition.go.mutant`
     and, in `appendGateEvidence`, delete the line `defer unlock()` and insert
     `unlock()` on its own line immediately before
     `return ws.appendGateEvent(ctx, id, eventType, delta)`. For T3, copy
     `internal/release/release.go` to `logs/_ct-mutation/release.go.mutant` and
     change `c.LatestRelease(ctx)` in `Latest` to
     `c.LatestRelease(context.Background())`. Write `t1-overlay.json` and
     `t3-overlay.json` in that directory without a byte-order mark, each
     `{"Replace": {"<absolute path of the original, forward slashes>": "<absolute
     path of the mutant copy, forward slashes>"}}`, with the original's path in
     the casing `go list -f '{{.Dir}}' ./internal/core` prints (Go compares overlay
     keys case-sensitively, and a mismatch leaves the overlay unapplied, which
     shows as no failure and halts safely). Stage has not run `-overlay` on this
     module. Only if Go rejects the non-`.go` replacement name may the executor
     rename the copy to `.go` inside the same underscore directory (still skipped
     by `./...`), and say so in the note; any other build error halts (step 3). In
     the renamed case `gofmt -l .` walks `_` directories, so Ship judges gofmt on
     LF-normalized content as it already does for CRLF working copies.
  3. **Mutant runs.** Run from pwsh. T1: one invocation of
     `go test -count=10 -run '^TestUT1_ConcurrentSameItemCounterContract$' -overlay <absolute path of t1-overlay.json> ./internal/core`,
     non-zero, printing ten
     `--- FAIL: TestUT1_ConcurrentSameItemCounterContract` lines, each with the
     `duplicate counter <n> durably persisted` assertion. T3: one invocation of
     `go test -count=3 -run '^TestUT3_ClientLatestRespectsContextTimeout$' -overlay <absolute path of t3-overlay.json> ./internal/release`,
     non-zero, printing three
     `--- FAIL: TestUT3_ClientLatestRespectsContextTimeout` lines with
     `An error is expected but got nil.` at `release_test.go:102`. Record the
     observed count. Zero intended-assertion failures means the overlay did not
     apply or the test has no detection power, and a count below all-fail may also
     be a lost race: R3 says what halts and what is only recorded. A build error
     from `-overlay` is a halt, not a reason to edit a production file instead.
  4. **Unmutated controls.** The same commands without `-overlay`, with `-v` and
     `-count=1`, exit 0 and print the `--- PASS:` line; `go test -list` with the
     same full anchored name lists exactly one test.
  5. **No production delta and an unchanged module.** The R2 commands run before
     and after the experiment and both times exit 0 and print nothing, and the
     output of `go list ./...` is captured before and after and is identical, with
     no `_ct-mutation` line (an exit code alone would not show the directory is
     invisible to Go).
  6. **Record.** Ship commits one note, `docs/memory/2026-10-12-198s-ct-mutation-kill-evidence.md`,
     by explicit path as a `docs(memory)` commit, carrying for each task the
     executor's identity and model, the mutation, the exact commands, the run
     counts, the `--- FAIL:` and assertion lines, the control and `-list` results,
     the R2 and R4 output, `9762e75f`, the R5 tokens, and one column-0 line
     `R3_FINDINGS: <n>` (the number of tasks whose kill ratio was below all-fail,
     which is the count A1.3 adds to the closure mapping). The same lines go in one
     comment on each task.
  7. **Regression contracts.** Stage applied a P-002.6 `green-regression-contract`
     block to each task body (A1.5 shows them), because Ship Step 4.3 runs only
     `harness_cmd` and the declared `green_regression_cmds`, and the plan's AC
     repetition commands would otherwise never run: T1 runs the anchored
     `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters` test with
     `-count=30` and again with `-count=30 -cpu 1`, both with `-timeout 30m`.
     Thirty iterations at up to 11.79 s each
     (`docs/evidence/2026-10-08-ct-full-suite-failure.md`) is about 6 minutes, and
     `internal/core` timed out at the 10-minute default in 154-S (a `-race`, multi-package run, so only an indication)
     (`docs/memory/2026-09-30-ship-154s-w4-race-timeout-halt.md`), so `-cpu 1` or
     load could cross it. T3 runs
     `TestClientLatestRespectsContextTimeout` with `-count=200`. The anchored
     selectors match the same tests as the AC's unanchored ones on the current
     test set. Each command names an explicit package and bypasses the result cache
     (any `-count` above zero does, because `-count` is outside Go's cacheable
     flags, so `-count=30` and `-count=200` meet P-002.6's cache rule). Their
     vacuity guard is Ship's rule that a `[no tests to run]` result from any of
     them is a failure. Ship
     re-runs Step 3 so these blocks are parsed (M is unchanged, so the freeze
     anchor is unaffected) and records each command's result in a comment on the
     task.
  8. **Completion path.** The harness is green on arrival and the delivering
     commit already exists, so Ship tells build-feature at Step 4.2 that a
     zero-delta, green-on-arrival completion is expected and that it must not edit
     a test or production file to create a delta. A "nothing to commit" report is
     success. If build-feature halts on that dispatch anyway (it has no stated
     zero-delta input, stash 5F5D7384), Ship runs `harness_cmd` and the declared
     regression commands itself, as it ran the 197-S red completions directly,
     records the output, and completes the task; it still writes no test or
     production code. A failure of an AC repetition command is a halt, because R4
     forbids changing the test files and CT writes no production code; it returns
     to the Orchestrator for routing, not to a fix attempt. Ship commits the
     `.backlogit/**` writes separately,
     completes the task, and associates `9762e75f` as the delivering commit
     (`backlogit_track_commit`). Any other halt is a P-005 event that the
     Orchestrator routes to the operator, or to Stage when only plan text must
     change; none is a reason to scaffold a substitute test.

Residual. T1's relaxed form (at least one success) no longer detects a stuck
counter lock, and mutation-kill covers only the unlock-after-append revert; stash
5A6F54BB records a deterministic-subtest follow-up.

### A1.5 Resume preconditions and notes for Ship

* Release. The operator ruled (A1.4, Option A for T1 and T3) and the 2026-10-12
  revision of A1 carries its own final `## Plan Review` section, which must read PASS (or ADVISORY with the approval marker) before Ship is dispatched: `_ship.agent.md` has no intake check of the review decision, so the hold is the Orchestrator's, backed by the Intake bullet's check below. Ship resumes at
  wave-1 admission, in the same Ship session (the Orchestrator's route; this is
  not a crash restore, so Ship does not select, restore, or resolve a checkpoint
  for it), after the Orchestrator unblocks shipment 198-S if it was blocked
  (A1.7 PA-2). Ship's active checkpoint
  `.backlogit/checkpoints/checkpoint-20261012-050053.json` has a `resume_hint`
  that says to reuse the `harness-ready` labels on 198.001-T through 198.003-T;
  that holds for T1 and T3 only on A1.4's terms (R1 through R6), which the hint
  omits. Stage resolves, restores, and resumes no checkpoint.
* Landing. Stage ran no state-changing git operation (it ran read-only git
  commands and fixture runs), and its uncommitted writes sit on Ship's
  branch only because Ship is halted. The Orchestrator's Step 1.5 continuity
  carve-out does not apply to this in-flight amendment, and Ship's commit is a
  pass-through of Stage's bytes. Ship commits these three paths by explicit path as
  one docs commit before Step 4.1a, similar to the 197-S amendment (`de75c673`, which combined docs and backlog records in one commit; A1 splits them),
  after running `backlogit docs lint` and `scripts/md-lint.ps1` on them (Stage
  could not, P-010; expected results are in the last bullet): the plan, the
  decision note `docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md`, and
  `docs/memory/2026-10-12-stage-198s-exempt-amendment.md`. A suitable message is
  `docs(docs): amend CT plan for 198-S exempt set and A1.4 ruling`. The
  `.backlogit/**` writes then go in a separate commit (for example
  `chore(config): commit 198-S backlog state after Stage amendment`; the repository
  history also uses `chore(backlogit)`) before Ship captures `exempt_baseline_sha`,
  similar to 197-S (`cb736fa6`, hook events only). Stage's writes there are
  `.backlogit/queue/198.001-T.md`, `198.003-T.md` (a green-regression block each),
  `198.004-T.md` and `198.005-T.md` (label, contract block, and amendment notes),
  `.backlogit/stash.jsonl` (seven Stage-created entries, listed in the decision
  note), and Stage's hook events in `.backlogit/hooks_queue.jsonl`. The rest of the
  dirty set is Ship's own: `198-F.md`, `198-S.md`, `198.002-T.md`, the claim and
  label changes already in `198.001-T.md` through `198.003-T.md`, hook events seq
  3722 through 3726, and the untracked checkpoint file above. Any other dirty path
  is a Ship halt. Review findings against A1 route to Stage, not to fix-ci.
* Intake. Re-run the Step 4.0 served-root attestation. Re-run Step 3 schedule
  construction so the new `green-regression-contract` blocks on 198.001-T and
  198.003-T are parsed, and confirm the re-frozen M equals the M in the halt note
  (`{198.001-T, 198.002-T, 198.003-T, 198.004-T, 198.005-T}`); before the Step 4.2
  dispatch of each of those two tasks, confirm the `green_regression_cmds` array
  passed to build-feature equals the one below, so a stale frozen array from before
  the amendment cannot silently drop the AC repetitions. Confirm through
  `backlogit_get_item` that 198.004-T and 198.005-T each carry `harness-exempt`
  and a contract block identical to A1.3, which Stage verified at application
  time, and that 198.001-T and 198.003-T each carry exactly one
  `green-regression-contract` block. Confirm A1.4 has exactly one column-0 line
  matching `^a1_4_operator_ruling: T1=A; T3=A: ` and record that in a comment on
  each of 198.001-T and 198.003-T. Read the final `## Plan Review` section of this
  plan and halt through P-005 unless its `decision:` is `PASS`, or `ADVISORY` with
  the approval marker (the label `operator_authorization`, a colon, and the word
  `approved`) in that section.
* T1 and T3 (A1.4). Ship follows the ruling: R1 through R6, with R6 steps 1 to 6
  done before Step 4.0 item 10 is evaluated for wave 1 and steps 7 and 8 governing
  the dispatch. The `green-regression-contract` blocks Stage applied are:

  ```json
  {
    "green_regression_cmds": [
      "go test -count=30 -timeout 30m -run '^TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters$' ./internal/core",
      "go test -count=30 -cpu 1 -timeout 30m -run '^TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters$' ./internal/core"
    ]
  }
  ```

  for 198.001-T, and

  ```json
  {
    "green_regression_cmds": [
      "go test -count=200 -run '^TestClientLatestRespectsContextTimeout$' ./internal/release"
    ]
  }
  ```

  for 198.003-T. A1.1 through A1.3 do not depend on A1.4.
* T4 and T5, the exempt tasks. T4 runs its own
  `go test -json -count=1 -timeout 40m ./internal/core` (about 15 minutes when
    alone and about 28 under load; A1.3 gives the margin) and saves the JSON to a
  git-ignored path. Its document is a
  task deliverable executed through `build-feature`, not a Ship planning artifact
  (Ship's Role Boundary lists spike artifacts as forbidden for Ship itself; A1.7
  item 4(c) asked the operator to confirm this reading, and the operator confirmed
  it on 2026-10-13 (D-3, A1.4 confirmation line), so Ship does not halt with
  P-010 at T4). Ship captures follow-ups at the post-merge closure as A1.3's
  ordering paragraph says.
* Dispatch constraints (T1 through T5). Ship passes these to build-feature at Step
  4.2 and owns them: no stash capture inside the task (quote this clause verbatim in the T4 dispatch, because T4's original text above its notes still says to capture each follow-up as a stash entry); no `gofmt -w .` (none of
  the five tasks changes Go source beyond test files, and T2's uncommitted helper edit may be formatted file by file; the
  `gofmt -l` gate is unchanged, and Ship's memo notes it reports false positives
  on CRLF working copies; build-feature's Post-Loop Format step runs `gofmt -l .`
  and, on any listing, `gofmt -w .`, which this constraint forbids, so the
  dispatch tells the executor to run `gofmt -l` on the files the task changed
  only, to judge them on committed-blob content (`git show HEAD:<path>`, because
  CRLF working copies list falsely), and to run `gofmt -w` on one named file,
  never `.`, only for a violation that content shows); a zero-delta, green-on-arrival completion is expected
  for T1 and T3 (R6 step 8). For T4 and T5 the deliverable is a new, untracked
  file, and build-feature's completion gate (Post-Loop item 4) measures
  `git diff --name-only {exempt_baseline_sha}` and
  `git diff --cached --name-only {exempt_baseline_sha}`, neither of which lists
  an untracked file, so the gate would halt on an empty delta
  (`EXEMPT_DELTA_EXCEEDS_CLASS`). Ship cannot stage the file between the
  executor's write and that gate, because Step 4.2 is one dispatch call. The
  dispatch therefore tells the executor to run `git add -- <deliverable path>`
  right after writing the file and before the gate, to run `scripts/md-lint.ps1`
  (it takes no path, lints the whole non-ignored corpus, and writes nothing)
  at the same point, because `backlogit docs lint`, which the exempt gate runs,
  does not imply markdownlint (P-008), and to write nothing else outside the
  ignored evidence log. Ship repeats the md-lint run before Step 4.5; findings
  only in files the task did not write are reported through P-005 and not edited.
  `git status --porcelain` is empty before the dispatch (Step 4.1a item 5 halts on
  a dirty tree after the baseline commit), shows only the staged deliverable
  before build-feature's gate, and is empty after its Commit step, before Ship's
  own Step 4.3 passes (which read `baseline..HEAD`). If the executor stops after
  writing the file, Ship does not re-dispatch (build-feature Step 0 would halt
  `EXEMPT_FALSE_GREEN` with the deliverable present); it halts through P-005.
  Ship makes no tracked backlog write between baseline capture and the
  post-commit Step 4.3 gate (comments go to the ignored item logs; status changes
  wait until after the gate). After every task completion, including the zero-delta
  T1 and T3, Ship commits by explicit path the queue-to-archive move,
  `.backlogit/hooks_queue.jsonl`, `.backlogit/stash.jsonl` after any Step 4.4a capture, its own checkpoint files and Step 4.5 memory
  writes, and any R2 or R4 appendix to the R6 note (the note itself was committed at R6 step 6), before the next baseline capture
  (`docs/memory/2026-10-10/ship-197s-wave1-converged-wave2-halt.md`: the archive
  file must be committed with the move or the next zero-delta check fails).
* T5 non-green runs and the CT-scope-file check (A1.1, A1.3 Erratum). These are
  Ship's own steps, not build-feature's and not Step 4.4's: Step 4.4 is the
  report-only `review` skill (READY, READY_WITH_FOLLOWUPS, or BLOCKED keyed to
  P0 and P1 findings), so it cannot make T5 incomplete. "Halts with 198-S left
  active" in A1.1, A1.3, and A1.7 means Ship halts through P-005, leaves 198-S
  active and unchanged, and makes no `backlogit_return_blocked` call, which would
  edit the shipment's items list. (1) The T5 dispatch tells the executor that on a
  non-green run (a second failure of the same package, a package timeout, a
  first-run failure that names no `--- FAIL:` test such as a panic outside a test
  or a build failure, a first run that fails in two or more packages, or a failure
  in a CT-scope test) it
  writes no deliverable, does not loop, does not mark the task BLOCKED
  (build-feature's generic loop ends in BLOCKED after five failures, which
  would route to Step 4.6 item 1 `WAVE_MEMBER_BLOCKED` and
  `backlogit_return_blocked`), and reports the package, the failed test names or
  panic text, and the run log path. On that report Ship skips Steps 4.4 and 4.5
  for T5 and halts through P-005. (2) After the Step 4.3 gate passes on a
  `GREEN-AFTER-RERUN` artifact and before Step 4.4, Ship takes each failed test
  name from the first-run log, finds its file with
  `git grep -l -E "func <TestName>\(" -- "*_test.go"`, and compares it with
  `git diff --name-only c5758918 HEAD` (pwsh; `c5758918` is the branch base that
  R2 names, and on this branch it equals `git merge-base main HEAD`, but `main`
  can move, so the pinned commit is the reference). A failed test defined
  in a file in that diff is a CT-scope failure and takes the same halt as (1),
  with the committed artifact left in place for Stage to read at triage. The
  command does not check this, so Ship's own run is the check, not the
  artifact's statement. The list of failed names must be non-empty and every name
  must resolve to exactly one test file; an empty list or an unresolved name takes
  the same halt, so the check cannot pass vacuously. (3) Ship's Step 4.4 inputs say that T4's follow-ups and
  T5's flake are "captured at Step 6", so a reviewer's follow-up list does not
  trigger an early Step 5 item 10 capture (A1.3).
* Green-regression validity. The two blocks use `-count=30`, `-count=200`, and
  `-cpu 1`, not the `-count=1` of P-002.6's example. P-002.6 asks the command to
  bypass result caches, which any `-count` above zero does, and to fail closed on a
  vacuous pass, which R6 step 7 covers with the `[no tests to run]` rule. The
  scheduler simulation's green-regression parser checks only structure (one key, a
  non-empty array of unique strings), and `WAVE_SIM_OK` is bound to 130-S, so it
  cannot corroborate these blocks; the intake checks above are the control. Run
  the commands from pwsh (the single-quoted `-run` patterns are literal in pwsh
  and bash, not in `cmd.exe`).
* Quiet host. T5 runs with no concurrent subagent, build, or review activity. Ship's own unfiltered gates (Step 4.6 for each wave and Step
  5) should too, as advice and not a gate (stash F8936891 records their 92% exposure, and no check can verify it).
  `docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md` rule 1 asks for
  a quiet workspace and rule 4 for the exit code and package list of the exact
  HEAD under gate. T5 runs in wave 2, after T1 through T3 are done and CX has merged. The
  evidence artifact states what else ran (the gate cannot check that). Its pinned
  timeout is 40 minutes (A1.3).
* Ship-side verification Stage could not run (P-010), with expected results. (1)
  Step 4.1a probes of both exempt commands against the absent deliverable: exit
  non-zero, no marker. (2) Before the first exempt dispatch, stand-alone smoke runs
  of the arms the probe never reaches: `go list -mod=readonly ./...` exits 0 and
  prints at least one package;
  `backlogit docs lint --path docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md --format json`
  exits 0 and prints JSON whose `valid` is true. (3) `backlogit docs lint` over the
  three docs, expecting `valid: true` and zero findings, and `scripts/md-lint.ps1`,
  expecting `0 issues`, before the docs commit. (4) `git check-ignore -v
  logs/_ct-mutation/x`, expecting a `.gitignore` rule line and exit 0, before R6
  step 1. (5) The `-overlay` runs, the R2 and R4 commands, and the before and after
  `go list ./...` comparison (R6 step 5). A difference from what this amendment
  states is a halt that the Orchestrator routes to the operator, or to Stage when
  only plan or command text must change.

### A1.6 Constitution Check (Amendment A1)

Numbering follows `.github/instructions/constitution.instructions.md`. This check
covers A1 as revised after the operator's ruling. Its closing line supersedes the
earlier `Constitution Check: pass` (above the amendment) for the plan as amended.

* I. Safety-First Go: pass. A1 changes no code.
* II. Test-First Development (NON-NEGOTIABLE): conflict, a documented deviation with operator authority (the Option A ruling and its 2026-10-13 confirmation, D-2), for T1 and T3 only, with the ordering relaxed by
  the operator and recorded as a conflict in A1.4 (the principle, the
  justification, and the simpler alternatives rejected, Options B and C). T2 keeps
  its genuine red. For T1 and T3 the red is observed on a mutated copy, not on the
  committed harness: CT writes no production code (R2 proves it at three
  points), the mutation-kill observation (R3) keeps what the principle protects,
  which is that a test is seen to detect the defect it guards, and what is given
  up is the ordering. P-004 and FC-1 to FC-3, the Enforcement Model's mechanism
  for this principle, are the controls the operator relaxed, and no carve-out from
  the principle itself is recorded (no production behavior is added or changed).
  The Development Workflow's item 1 ("Harness before code", a MUST) states the
  same ordering and is departed from on the same operator authority: the
  operator's confirmation of 2026-10-13 (D-2) covers it, and P-002's own red-phase
  statements, for 198.001-T and 198.003-T only.
* III. Workspace Isolation: pass for A1's design: evidence paths in the exempt
  commands are repository-relative, resolve inside the repository, and are
  git-ignored, and R6's scratch path is under the ignored `logs/`.
* IV. CLI Workspace Containment (NON-NEGOTIABLE): violation by Stage, recorded
  honestly. Stage's first fixture run wrote exempt-command fixtures under the user
  temp directory (`%TEMP%\stage-198s`), outside the workspace, which Principle IV
  forbids, and no exemption exists. It is a Stage process breach, not an A1 design
  defect. Disposition: the operator approved deleting the directory on
  2026-10-13T03:26:01Z (D-5, A1.4 confirmation line). The directory was found
  already absent at 2026-10-13T03:39Z: the operator's `Remove-Item` reported that
  the path does not exist, and the Orchestrator's read-only check found
  `stage-198s` absent from `$env:TEMP`, the 8.3 and long forms of the user temp
  directory, `C:\Windows\Temp`, and `C:\Temp`. It was removed by an unknown
  earlier step, and no further action is needed; this record does not say who or
  what removed it. The earlier wording that scored IV as "pass" is withdrawn.
  The breach is disclosed in
  `docs/memory/2026-10-12-stage-198s-exempt-amendment.md`.
* V. Structured Observability: pass. A1.4 R5 and R6 record evidence in task
  comments, a committed memory note, the PR annotation, and the closure.
* VI. Single Responsibility (dependencies): pass. No dependency is added.
* VII. Destructive Command Approval (NON-NEGOTIABLE): pass. Both exempt commands
  are read-only and P-002.5-screened, R6 deletes nothing and forbids deleting its
  scratch directory, and nothing alters history.
* VIII. Explicit Safety Modes: pass. The mutation experiment (A1.4 R6) runs by
  overlay under `careful` mode with a `freeze-scope` boundary, so no production
  file changes.
* IX. Git-Friendly Persistence and X. Agent Context Efficiency: pass. Every
  artifact is Markdown with frontmatter, and the evidence is committed rows
  rather than an ignored log.
* Task Granularity (NON-NEGOTIABLE): pass. No task changes size or scope, and
  T4 and T5 each remain one skill domain within the 2-hour bound. R6's experiment
  is evidence work before dispatch; it adds no file to either task's delta.
* XI. Merge Commit History Preservation (NON-NEGOTIABLE): pass. A1 ships inside
  the CT PR by merge commit.
* A1 records one operator-authorized deviation (A1.4, Option A with its
  2026-10-13 confirmation, T1 and T3 only, non-precedent under R1) and no
  Stage-granted one. It also records two Stage process events: the Principle IV
  violation above and a P-010 event (in the first revision Stage ran linters,
  `backlogit docs lint`, and `go list` while authoring the exempt commands). The
  P-010 event was accepted by the operator as a disclosed event with no
  re-execution on 2026-10-13T14:20:54Z ("02: Accepted", A1.4 acceptance line); the
  earlier "D5 delete" reply is not read as accepting it. Stage waives neither;
  the operator's answers are the authority. The 2026-10-11 in-tree production mutation during the wave-1
  scaffold is a Ship-side P-005 event, reverted, with R2 empty at HEAD, and the
  operator accepted it as disclosed (D-7, A1.7 item 7).

Constitution Check: documented-deviations

### A1.7 Plan hardening delta (Amendment A1)

* Protected invariants: CT commits no production code; no exemption without a
  contract block and closed-set membership; no red-phase substitute beyond the
  operator's Option A for T1 and T3, on A1.4's conditions; P-002.4 is not
  relaxed; M and task scope are unchanged.
* Risky actions (strict-safety):

  #### PA-1: Apply exemption labels and blocks to 198.004-T and 198.005-T

  * **Summary**: Add the `harness-exempt` label and the A1.3 contract block to
    each task body through `backlogit_update_item`, then read each task back
    through `backlogit_get_item` and compare the whole block to A1.3. The blocks
    were applied in the first revision and re-applied after each later change;
    T5's description line changed `-timeout 30m` to `-timeout 40m`, and each body
    gained a short note on the amended details (no stash capture inside the task).
  * **Targets**: `.backlogit/queue/198.004-T.md`, `.backlogit/queue/198.005-T.md`
  * **Change kind**: local planning-field edit
  * **ActionRisk**: low. It is metadata only, and Ship's intake fails closed on
    any drift.
  * **Rollback**: Stage removes the label and block and reverts the A1 text.
    After Ship commits A1, Ship commits a revert of the docs commit and of the
    backlog-state commit, with Stage supplying the inverse edits.
  * **Approval required**: no
  * **ActionResult**: applied and read back on 2026-10-12: each task's block equals
    A1.3 after line-ending normalization, and its command line equals the tested
    command file (`t4.v5.cmd.txt`, `t5.v6.cmd.txt` in the ignored scratch path)
    byte for byte. A1.5 says A1.1 through A1.3 do not depend on A1.4.

  #### PA-2: Block shipment 198-S (superseded recommendation)

  * **Summary**: While the T1 and T3 ruling was open, Stage recommended that the
    Orchestrator run `backlogit_block_shipment` on 198-S. The operator ruled, so
    the only remaining step is `backlogit_unblock_shipment` if it was blocked, once
    the final `## Plan Review` is PASS. Stage runs neither: Ship holds the active
    claim.
  * **Targets**: shipment 198-S (status only; the manifest is untouched)
  * **ActionRisk**: moderate (a blocked shipment reports a blocked member at Ship
    Step 4.0 item 3, and `backlogit_return_blocked` can edit the live items list).
  * **Rollback**: `backlogit_block_shipment`.
  * **Approval required**: Orchestrator or operator
  * **ActionResult**: abandoned (the recommendation is superseded; any unblock is
    the Orchestrator's)

  #### PA-3: Withdraw `harness-ready` from 198.001-T and 198.003-T (rejected)

  * **Summary**: Considered and rejected. Ship's Step 2 sends a task with neither
    label to harness-architect rather than halting (items 2 and 4), so withdrawal
    is not a mechanical hold, it invites the fabricated red that A1.4 rules out,
    and it exceeds the labels the exempt classification needs. Under the ruling the
    labels stand on A1.4's terms.
  * **ActionResult**: abandoned

  #### PA-4: Mutation experiment (A1.4 R3 and R6)

  * **Summary**: Scoped test runs against overlay copies of the guarded behavior
    (one `-count=10` invocation for T1 and one `-count=3` for T3), run by the R6
    executor, by `go test -overlay`. No production file is edited.
  * **Targets**: copies of `internal/core/gate_transition.go` and
    `internal/release/release.go` under the ignored `logs/_ct-mutation/`
  * **Change kind**: ignored scratch files and test runs
  * **ActionRisk**: moderate (it perturbs the behavior under test)
  * **Rollback**: nothing to revert; the production tree is unchanged (R2 proves
    it). The scratch directory is left in place, never deleted (Principle VII).
  * **Approval required**: given by the operator's Option A ruling, on R1 through
    R6, and effective when this amendment's review passes (Option A's release
    clause).
  * **ActionResult**: approved by the ruling; not executed by Stage (P-010). Ship
    runs it at wave-1 admission.

  #### PA-5: Apply green-regression contracts to 198.001-T and 198.003-T

  * **Summary**: Add one P-002.6 `green-regression-contract` block to each task
    body (A1.5 shows the JSON) so Ship Step 4.3 runs the plan's AC repetition
    commands, which `harness_cmd` alone would not. Each body also gained a note on
    the ruling.
  * **Targets**: `.backlogit/queue/198.001-T.md`, `.backlogit/queue/198.003-T.md`
  * **ActionRisk**: low. A malformed block halts Ship's schedule construction with
    `WAVE_GREEN_REGRESSION_INVALID`, which fails closed.
  * **Rollback**: remove the block.
  * **Approval required**: no
  * **ActionResult**: applied and read back on 2026-10-12; the JSON parses with
    exactly the one key and unique non-empty commands, equal to A1.5's. Ship
    re-runs Step 3.
* Added verification: Stage exercised both exempt commands (A1.3 test record: 85
  fixture cases, pwsh and git only). The observations of record are Ship's
  Step 4.1a pre-work probe, smoke runs, and post-deliverable gate; A1.5 lists what
  Ship must run that Stage could not, with expected results.
* Operator items. The operator answered item 1 on 2026-10-12, and on 2026-10-13
  answered item 1's open question, items 4(a) to 4(d), the plan-review approval,
  and items 7 and 8 below (A1.4 confirmation line). Stage resolved the rest, on the Orchestrator's directions,
  inside policy, as recorded here.
  1. **A1.4 (answered and confirmed; closed).** Option A for T1 and T3, recorded
     in A1.4 with the conditions R1 through R6. The question this item first left
     open (whether Option A's wording reaches P-002's own red-phase statements,
     Principle II's ordering, and Development Workflow item 1) was answered by the
     operator on 2026-10-13T03:26:01Z (D-2: yes, for 198.001-T and 198.003-T only
     and nothing else; A1.4 confirmation line).
  2. **P-002.4 and `.backlogit/**` (resolved without relaxing P-002.4; stash
     253E2505).** See A1.3's ordering paragraph. The general gap for housekeeping
     writes stays open in 253E2505, which no longer blocks 198-S.
  3. **T5 timeout (resolved).** See A1.3's Timeout paragraph. Ship's gates keep
     `-timeout=30m` and their 92% exposure (stash F8936891); four unfiltered runs
     still sit near that budget (Ship Step 4.6 for wave 1, T5, Step 4.6 for wave 2,
     and Step 5), and only T5 tolerates a rerun. A halt at one of Ship's gates goes
     to the operator; the 197-S final-gate halt
     (`docs/memory/2026-10-10/ship-197s-final-gate-halt-memory.md`) shows the path.
  4. **Stage process disclosures and role-boundary readings (closed by the operator
     on 2026-10-13: D-3, D-5, and the later O2 acceptance).** (a) Stage's first fixture run wrote under
     `%TEMP%\stage-198s`, outside the workspace (Principle IV; recorded as a
     violation in A1.6). The operator approved deleting it (D-5). It was found
     already absent at 2026-10-13T03:39Z (the operator's `Remove-Item` reported the
     path does not exist, and the Orchestrator's read-only check found it absent
     from `$env:TEMP`, its 8.3 and long forms, `C:\Windows\Temp`, and `C:\Temp`);
     it was removed by an unknown earlier step, and no further action is needed. (b) In the first
     revision Stage ran `backlogit docs lint`, `scripts/md-lint.ps1`, `go list`,
     and pwsh fixtures to test the exempt commands, a P-010 event (P-010 lists
     linters and build systems as forbidden for Stage). Disposition: the operator
     accepted it as a disclosed P-010 event with no re-execution on
     2026-10-13T14:20:54Z ("02: Accepted", A1.4 acceptance line). The earlier
     "D5 delete" reply answered a prompt that described these runs together with
     the `%TEMP%` fixtures, and Stage does not read it as accepting them; the
     separate "02: Accepted" does. The second
     revision's 85 fixture runs were pwsh and read-only git with a `go` shim (stash
     2462F1B0). (c) Ship's Role Boundary lists spike artifacts as forbidden for
     Ship, and T4's deliverable is a spike findings document produced through an
     executor under Ship's dispatch (T4, commit `a5a07af9`); the operator confirmed
     that this is within Ship's boundary (D-3). (197-S delegated its exempt tasks
     197.007-T and 197.008-T to a build-feature subagent, and neither was a spike
     document, so that was never a precedent; the confirmation is the authority.)
     (d) Ship's Role Boundary allows delegating writes "to build/fix skills", and
     R6's executor is a general-purpose subagent that writes only ignored scratch
     files under `logs/_ct-mutation/`; the operator confirmed that this is within
     Ship's boundary too (D-3).
  5. **Checkpoint selection (nothing to do).** The Orchestrator resumes the same
     Ship session, not a crash restore (A1.5).
  6. **T5 non-green outcome (a replaced outcome, disclosed for operator awareness).** The
     plan's third red-run branch ("record CT closure as verification-incomplete")
     cannot complete an exempt task whose gate exits 0 only over a green suite.
     A1.3's Erratum therefore has Ship leave T5 non-done, report through P-005,
     and halt with 198-S left active (A1.5), with Stage capturing the policy's entry at triage;
     a package timeout takes the same branch. No acceptance criterion is met on
     that branch, so no criterion's text changes, but the original outcome (a CT closure recorded as incomplete) is replaced by a halt, and what then becomes of the
     shipment is the operator's decision.
  7. **In-tree production mutation during wave-1 scaffolding (a disclosed P-005
     event; accepted).** On 2026-10-11 the wave-1 scaffold subagent temporarily
     mutated `internal/core/gate_evidence_formal.go` and `internal/release/release.go`
     in the working tree to demonstrate red, then reverted them. R6 forbids that
     fallback (it runs by `go test -overlay`, so no production file changes), and
     the event was not recorded when it happened; Ship disclosed it in
     `docs/memory/2026-10-13-ship-198s-prepr-review-halt.md` (D-7). R2 is empty at
     HEAD `d3aff2c5`: production Go files are unchanged against the base. The
     operator accepted the event as disclosed on 2026-10-13 (D-7, A1.4
     confirmation line); no re-validation is required.
  8. **The second plan-review series (ratified).** The first series ended FAIL
     after three attempts, with the line "a fourth cycle is not permitted". The
     second series then ran four cycles: three authorized by the Orchestrator's
     dispatch after the ruling, and a fourth the Orchestrator approved after
     cycle 3 returned ADVISORY. The operator ratified the series, including that
     cycle-4 authorization, on 2026-10-13 (D-8), and confirmed the ADVISORY
     approval in the final `## Plan Review` (D-1).
  9. **Decisions made by the Orchestrator within scope (not operator answers).**
     D-4: Ship's exact-name selector for 198.002-T AC2 is accepted as that
     criterion's disposition (A1.8 E-1). D-6: T5's result is GREEN-AFTER-RERUN,
     stash 63909DAE is the recurrence, and "the isolation fixes held" is not stated
     (A1.8 E-8). O1: D-4 and D-6 are the Orchestrator's calls, and the attribution
     in Stage's text was corrected on 2026-10-13. O4: the T5 scalar
     `CT_ISOLATION_FIXES_HELD` stays, as a gate-required token governed by A1.8
     E-8. O5: the unattested executor model for the R5 mutation runs is accepted
     as disclosed residual risk, and an operator spot-check of the `--- FAIL:`
     lines is optional and available. A1.8 also records the wording defects of
     stash FBD0BB95 and the fourth `slog.SetDefault` site of stash 8640432F.
  10. **Operator answers to Ship's round-2 memo (2026-10-13T14:20:54Z).** "02:
     Accepted" and "03: Accepted", read as O2 (the first-revision P-010 runs,
     accepted as a disclosed event without re-execution) and O3 (the A1.8 errata,
     as corrected by O1, accepted as residual risk). No decision from Ship's O1
     to O5 remains open.

### A1.8 Errata after Ship's pre-PR review (2026-10-13)

Ship's pre-PR review halted
(`docs/memory/2026-10-13-ship-198s-prepr-review-halt.md`). The operator answered
six of its decisions (D-1, D-2, D-3, D-5, D-7, and D-8) with the words "D1 yes, D2
yes, D3 yes, D5 delete, D7 accept, D8 ratify" (A1.4 confirmation line). It did not
answer D-4 or D-6, which are the Orchestrator's in-scope calls. These errata were
written after that reply, so the reply does not accept them. Ship's round-2
delta review examined them (`docs/memory/2026-10-13-ship-198s-delta-review-and-readiness.md`),
this section was then corrected for attribution and wording (E-10), and the
operator accepted the errata as corrected, as residual risk, on
2026-10-13T14:20:54Z ("03: Accepted", A1.4 acceptance line). These errata are
text corrections and dispositions. No task, acceptance-criterion text,
dependency, or the manifest M changes. Classification, so each can be read for
what it is: E-1 is a disposition that substitutes the evidence for an
acceptance criterion (its text is unchanged), and E-8 is a disposition that
fixes what the T5 evidence and the closure may say; both are the Orchestrator's
in-scope calls. E-2, E-6, and E-7 are record-accuracy corrections that also edit
a task or feature body. E-3, E-4, and E-5 are wording corrections to Stage-owned
plan text. E-9 and E-10 are record-accuracy corrections. Bodies changed:
`.backlogit/archive/198.002-T.md` (E-1, E-2), `.backlogit/archive/198.005-T.md`
(E-4, E-7, E-8), `.backlogit/queue/198-F.md` (E-6), stash 253E2505 (E-9), and the
plan, the decision note, and the Stage memo. Each fix was checked against the
source it names.

1. **E-1, 198.002-T AC2 (D-4, the Orchestrator's in-scope call).** AC2 as written,
   `go test -count=1 ./internal/core -run 'BaseRef|ShippedEventDurability'`,
   matches no test name (the words are in the two file names, not in any test
   name), so it exits 0 with `no tests to run`, a vacuous pass that P-002.6 fails
   closed on. Ship ran an exact-name selector of the eight tests defined in the
   two named files: `TestGateBaseOverrideShadowed_WarnsAdvisory`,
   `TestGateBaseNoShadow_NoWarn`, and the six `TestShipShipment_` tests
   `DurabilityFixtureUsesFailOpenFakeGate`,
   `ProvenNotAppliedShippedEventCompensates`,
   `IndeterminateShippedEventNeverRollsBack`,
   `UnrestorableItemReportsPartialCompensation`,
   `FailClosedShippedAppendSuppressesMoveStatusPostHook`, and
   `ShippedEventAppendFailureLogsFixedShape`: 8 PASS, 0 FAIL, exit 0
   (`logs/ship-198s/t2-ac2-file-tests.log`, ignored). The Orchestrator accepted
   that run as AC2's disposition. AC2's text stands unchanged; the deviation is the
   substituted selector, and the intent AC2 states (the tests in the edited files
   still pass) is met. A selector in an acceptance criterion should name tests,
   not files.
2. **E-2, T2's files and the "only three sites" premise (stash 8640432F).** T2
   edits three files: `gate_baseref_warn_test.go`,
   `shipment_shipped_event_durability_test.go`, and `log_state_restore_test.go`
   (which holds `TestUT2_CaptureSitesRestoreStdlibLogState`); the T2 unit's Files
   list names only the first two. The T2 text says the three sites are "the only
   `slog.SetDefault` sites in internal/core tests at d2f3ff97". That was true at
   d2f3ff97 (commit `7505b740`, which adds the benchmark, is not an ancestor of
   it) and is false at the branch base `c5758918`, where `7505b740` is an
   ancestor. A fourth site exists there: `silenceBenchLogs` in
   `internal/core/shipment_gate_bench_test.go:61`, which is benchmark-only and is
   never run by a plain `go test`. It is outside T2's authorized change list
   (P-021 C1), so Ship deferred it (stash 8640432F), and Stage agrees with that
   call. The 198.002-T body now lists the third file and states the fourth site.
3. **E-3, the base reference.** A1.5 named the base as `git merge-base main HEAD`
   while R2 names `c5758918`. On this branch they are the same commit (checked),
   but `main` moves, so A1.3 and A1.5 now name `c5758918` (R2's note on
   `origin/main` still applies if main is merged in later).
4. **E-4, T5 non-green triggers.** The lists in A1.1, A1.3's Erratum, A1.5, and
   the 198.005-T body now carry one set: a second failure of the same package; a
   package timeout; a first-run failure that names no `--- FAIL:` test (a panic
   outside a test, a build failure); a first run that fails in two or more
   packages; a failure in a CT-scope test or in a file T1 to T3 edited. The gate
   rejects the first four cases and a failure in a test the CT-scope list names. It
   accepts a failure in a test defined in a file T1 to T3 edited whose name is not
   on that list (for example a `TestShipShipment_` test), and Ship's A1.5 step 2
   check is what rejects that one. The prose was incomplete.
5. **E-5, "returns the shipment".** A1.5 defined the phrase for A1.1, A1.3, and
   A1.7, which cycle 4 had already reworded to "halts with 198-S left active", so
   the definition was dangling. A1.5 now states what Ship does under that
   wording.
6. **E-6, the 198-F body.** It said "CT has no shipment prerequisite", while 198-S
   depends on 197-S (CX), which shipped. The 198-F body now says so.
7. **E-7, T4 reusing T5's run.** The original T5 text (above, preserved) and the
   198.005-T body said "T4 may reuse T5's run by adding `-json`". Wave order
   forbids it: T4 runs in wave 1 and T5 in wave 2, after T1 to T3 and CX, and T4
   ran its own `-json` run. The 198.005-T body no longer says it, and the
   original sentence is superseded by this erratum.
8. **E-8, T5's result and the isolation wording (D-6, the Orchestrator's
   call).** T5's result is GREEN-AFTER-RERUN, not GREEN. The first run (HEAD
   `0d1ee370`) failed in `internal/core` on
   `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`
   (expected "archived", got "queued", `artifact_mutation_lock_order_test.go:88`),
   and the rerun exited 0 (`678.504s`). Ship's CT-scope-file check found that file
   outside the branch diff, so the rerun branch stands. Stash 63909DAE is the
   recurrence of that test under load; under compound rule 3 a failure that
   passes alone is a load-sensitivity finding, not a waiver. "The DB48A817,
   FF1F3AC7, and 885263D2 isolation fixes held" must not be stated for this run:
   not in the T5 artifact, the closure, the PR body, Stage's archival decision, or
   anywhere else. The three entries stay active, and Ship cites 63909DAE and
   captures no new entry for the flake (P-021 C2; A1.3's closure count then has no
   T5 term). What Ship records for a GREEN-AFTER-RERUN run: the gate requires the
   scalar line `CT_ISOLATION_FIXES_HELD: DB48A817, FF1F3AC7, 885263D2` literally,
   so Ship keeps exactly that line, whose value is the three IDs and nothing more.
   It is a gate-required token that names the entries this run concerns. It is not
   a claim that the fixes held, and Ship's annotation in the artifact says so. The PR body
   must not assert that the isolation fixes held; it states GREEN-AFTER-RERUN, the
   first-run failure, and the 63909DAE recurrence. The T5 artifact's original
   "Isolation fixes" sentence (`docs/closure/198-S-ct-full-suite-green-evidence.md`)
   said they held; Ship's "Ship annotation" section in that artifact supersedes it,
   and Stage did not edit the artifact (Ship's deliverable; the gate does not check
   prose). The Orchestrator decided this on 2026-10-13 (O4); it is not an operator
   decision.
9. **E-9, two Stage-owned text fixes.** The Stage memo's timestamp
   (`docs/memory/2026-10-12-stage-198s-exempt-amendment.md`) read 19:40Z against
   events recorded at 22:15Z; it now carries the time of its last write. Stash
   253E2505's status text cited a Step 5 item 10 capture, which A1.3 superseded
   with the post-merge closure (Step 6); it is corrected.
10. **E-10, corrections after Ship's round-2 delta review (2026-10-13; stashes
   7802D798 and 901C79CC).** Corrected, each checked against its source: the
   attribution of D-4 and D-6 (the Orchestrator's calls, not the operator's) in the
   plan, the decision note, and the Stage memo (O1); E-8's value rule and the PR
   body rule (O4); the executor-model gap recorded as disclosed residual risk
   (O5, A1.4 R5); the A1.4 reference to where D-4 and D-6 are recorded (A1.8 E-1
   and E-8, not A1.1 and A1.3); the A1.4 confirmation line, which no longer places
   the Orchestrator's 03:39Z observation under the 03:26:01Z reply; the A1.3
   Erratum and E-4 statements about what the gate rejects (it accepts a failure in
   a test defined in a file T1 to T3 edited whose name is off its CT-scope list,
   and Ship's A1.5 step 2 rejects it); the A1.3 T5 declared-evidence paragraph,
   which now says E-8 takes precedence over the "held" prose for a
   GREEN-AFTER-RERUN run; the A1.3 closure count (no T5 capture when an active
   entry already tracks the flake); the A1.1 T4-reuse bullet (superseded, as E-7
   says); the stale R5 PR-annotation list and the R6 step 1 P-010 conditional,
   which D-3 made moot; a non-empty and resolvable-name guard for A1.5 step 2; and
   the build-feature Post-Loop `gofmt -w .` clash with the exempt dispatch
   (A1.5). Declined, because the text is Ship's and not Stage's: stash 901C79CC
   items (b) and (d) concern Ship's own disposition record and memo (wrong erratum
   citations, the decision-ID mismatch with the operator's D-1 to D-8), which
   Ship corrects; Stage's records use the operator's D-1 to D-8 throughout. Already
   done before this round: the decision-note range (item (a): the FBD0BB95
   corrections are A1.8 E-2 to E-9) is now stated that way, and the Stage memo
   timestamp (item (e)) is set to its last write.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* attempt: 3
* scope: Amendment A1 (2026-10-12) only. This is the final `## Plan Review`
  section in document order, so it governs; the earlier section above covers the
  2026-10-08 plan and is superseded for A1's purposes only. A later recording
  amendment must carry its own review section, which would then govern.
* reviewers: six personas dispatched in parallel each cycle (Constitution
  Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist,
  Learnings Researcher, Correctness Reviewer).
* cycle 1, FAIL: Stage cannot ratify a P-004 deviation under dark mode (the
  INC-P002-131S-148F forward controls FC-1 through FC-4 are reserved to the
  operator, and INC-P002-152F-134S forbids treating a past acknowledgement as
  precedent); the T4 and T5 commands had false-pass and false-halt cases; T4's
  run command lacked `-timeout`; `backlogit stash get` is not read-only. Fixed:
  T1 and T3 escalated instead of ratified; commands hardened.
* cycle 2, FAIL (Constitution, Architecture, Correctness; Go, Scope, and
  Learnings ADVISORY): the T1 and T3 hold was prose-only while Ship's
  `harness-ready` labels remained. Attempted fix: withdraw the labels (PA-3), add
  a ruling sentinel line, contain evidence paths.
* cycle 3, FAIL: Go PASS, Learnings PASS, Constitution ADVISORY (P2 only), Scope
  FAIL (P1: PA-3's claim that intake would halt mechanically is false, and PA-3
  exceeds the labels the exempt classification needs), Architecture FAIL (P1,
  same root cause), Correctness FAIL (P1, same root cause, plus a P2 that a
  `-run`-filtered T5 log could pass). Root cause: Ship Step 2 items 2 and 4 send a
  task with neither label to harness-architect, so label withdrawal is not a hold.
* disposition after cycle 3 (these changes were not re-reviewed): PA-3 dropped and
  the hold stated as procedural; PA-2 added (recommend the Orchestrator block
  shipment 198-S); A1.5 per-outcome table; Option A and B release text; T5
  command v4 (end-anchored `ok` patterns, package-scoped CT-scope check; 33
  fixture cases pass); T4 non-overlap sentence removed; operator asks extended;
  the `harness-exempt` label and contract blocks applied to 198.004-T and
  198.005-T and read back identical to A1.3.
* gate outcome: FAIL after the maximum three attempts. The pipeline halts and
  operator intervention is required. A fourth cycle is not permitted. No
  `operator_authorization` is recorded.
* open P1 against A1.4: nothing mechanical holds T1 and T3. It closes only when
  the operator records the A1.4 ruling or the Orchestrator blocks shipment 198-S.
  A1.1 through A1.3 (T4 and T5) had no open P1 finding when review ended.

<!-- plan-review-attempt: 3 -->

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* operator_authorization: approved
* approval_basis: confirmed by the operator on 2026-10-13T03:26:01Z (decision D-1
  of Ship's memo `docs/memory/2026-10-13-ship-198s-prepr-review-halt.md`), in
  Copilot CLI session a3f32a34-316b-4580-965d-f0a3e5ca2ab4, with the words "D1
  yes, D2 yes, D3 yes, D5 delete, D7 accept, D8 ratify" (A1.4 confirmation line).
  History, kept as it happened: the marker was first supplied on
  2026-10-12T22:15:18Z (UTC) by the Orchestrator acting as dark-mode proxy under
  the P-017 activation for shipment 198-S, accepting this ADVISORY with no fifth
  review cycle. Ship's A1.5 intake accepted that marker textually, and Ship's
  pre-PR review then found that `.github/skills/plan-review/SKILL.md` allows the
  marker only where the user has explicitly confirmed, which the operator's D-1
  now supplies. The operator also ratified the second review series (D-8). The
  operator answered six decisions (D-1, D-2, D-3, D-5, D-7, and D-8) and not D-4 or
  D-6, which are the Orchestrator's in-scope calls. Residual risk accepted by the
  operator through D-1 and D-8: the six cycle-4 P2 text fixes and about 20 P3
  fixes were not re-reviewed. The 2026-10-13 errata (A1.8) postdate that reply:
  Ship's round-2 delta review examined them, Stage corrected their attribution and
  wording (A1.8 E-10), and the operator accepted them as corrected, as residual
  risk, on 2026-10-13T14:20:54Z ("03: Accepted", A1.4 acceptance line). No new
  Stage review cycle is required. Mitigation (kept): Ship's Step 4.4 adversarial review
  re-checks A1.5's dispatch constraints and its T4 and T5 bullets, and Ship
  captures any wording defects as stash entries (P-021 C2) instead of editing the
  plan (P-010). This is not a policy relaxation: no NON-NEGOTIABLE control is
  waived by the approval, and the T1 and T3 relaxation rests on the operator's
  explicit ruling and its D-2 confirmation (A1.4).
* attempt: 1 FAIL in this series (cycle 1); four cycles were used and a fifth is
  not authorized.
* scope: Amendment A1 (2026-10-12) as revised after the operator's ruling and
  after Ship's pre-PR review (A1.8, 2026-10-13), with its
  companion notes (`docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md`,
  `docs/memory/2026-10-12-stage-198s-exempt-amendment.md`) and the five task
  bodies. This is the final `## Plan Review` section in document order, so it
  governs and supersedes the first-series FAIL section above for A1's purposes.
* series: a second series of four cycles: three authorized by the Orchestrator's
  dispatch after the operator's ruling (`a1_4_operator_ruling` line in A1.4:
  "Ruling: option A", 2026-10-12T17:35:08Z, Copilot CLI session
  a3f32a34-316b-4580-965d-f0a3e5ca2ab4), and a fourth that the Orchestrator
  approved on 2026-10-12 after cycle 3 returned ADVISORY, scoped to the
  post-cycle-3 changes, on the operator's behalf under the dark-mode activation
  (that approval covers running the cycle; it is not an approval of any
  ADVISORY). On 2026-10-13 the operator ratified this second series, four cycles
  after the first series' three-attempt cap, including the Orchestrator's
  cycle-4 authorization (D-8, A1.4 confirmation line). The first series' attempt marker above
  (3) is superseded. Six personas ran in parallel each cycle (Constitution
  Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist,
  Learnings Researcher, Correctness Reviewer), read-only, with no Go, lint, or
  build run.
* cycle 1, FAIL (five personas) with Learnings ADVISORY. Common P1: R6's scratch
  copies at `logs/ct-mutation/*.go` would have broken every `./...` gate. Fixed:
  `logs/_ct-mutation/*.go.mutant`, one `-count=N` invocation per task, T5 rejects
  `[no tests to run]`, stash capture moved out of the exempt gated delta to the
  post-merge closure, dispatch constraints.
* cycle 2, ADVISORY (all six), no P1. Fixed: the closure capture procedure and
  its re-read and mapping, T5's second-failure branch, whole-identifier flake
  matching, pinned 40m timeouts with the stall-detection note, the Ship-side
  smoke runs, canonical ActionResult states.
* cycle 3, no P0 or P1. Constitution PASS (3 P3; it judged the Constitution Check
  line `documented-deviations` correct), Go PASS (2 P3), Learnings PASS (medium
  confidence, 7 P3), Scope ADVISORY (2 P2, 5 P3), Architecture ADVISORY (2 P2,
  2 P3), Correctness ADVISORY (1 P2, 3 P3). Gate rule (plan-review skill): P2
  findings only gives ADVISORY.
* P2 findings and what was done (text changes after cycle 3, not re-reviewed; the
  command strings in the T4 and T5 contract blocks are byte-unchanged):
  1. Scope: T5's non-green outcome (second failure or package timeout) changed
     from "record CT closure as verification-incomplete" to "Ship reports through
     P-005 and returns the shipment", while the header said no scope or
     acceptance-criterion meaning changed. Fixed by disclosing it as a third
     erratum (A1 header, A1.1, A1.3 Erratum, A1.7 item 6) and adding it to the
     errata Ship passes to its Step 4.4 review.
  2. Scope: T5's CT-scope name match omits tests that T2 edits
     (`BaseRef|ShippedEventDurability`), so such a failure could take the rerun
     branch. Fixed by a stated obligation, not a command change: the artifact says
     whether the failed test is in a file T1 to T3 edited, and Ship treats that as
     verification-incomplete at Step 4.4 (A1.3); cycle 4 found that Step 4.4 cannot
     enforce it and moved the check to a Ship-run step (cycle 4, item 3).
  3. Architecture: the closure mapping count did not cover R3 detection-power
     findings, and the "not captured early" rule did not name them. Fixed in A1.3:
     the count is `SPIKE_FOLLOWUP_COUNT` plus one for a T5 rerun plus one per R3
     finding; cycle 4 qualified "and nothing else" (cycle 4, A1.3 count text); the pre-merge exclusion names R3 findings.
  4. Architecture (and a Constitution P3): the dispatch bullet said `git status
     --porcelain` shows the deliverable before dispatch, which Step 4.1a item 5
     would halt on. Fixed: empty before dispatch, only the deliverable before the
     gate (cycle 4 corrected who stages the file, item 4).
  5. Correctness: both deliverable specs omitted `chunk_strategy: h1-h2-h3` and
     `schema_version: "1.0"`, which `tests/integration/docline_soft_keys_test.go`
     requires of every tracked, in-scope `docs/**` file and which `backlogit docs lint` and
     the exempt gates do not check. Fixed in both specs (A1.3) and the decision
     note; the commands are unchanged.
* P3 findings applied: A1.6 Principle II labeled a conflict, Development Workflow
  item 1 named in the conflict record, and A1.6's closing line stated to supersede
  the plan's earlier `pass`; "new authorization" wording corrected; M described as
  tasks only; the "27 minutes" figure qualified; 197-S commit citations and the
  197.007-T and 197.008-T description corrected; R6 note commit ordering; scoped
  `gofmt` for T2's uncommitted helper; quiet host limited to T5 as a requirement;
  `scripts/md-lint.ps1` added to the T4 and T5 constraints; the T4 dispatch quotes
  the no-stash clause; an MCP-timeout read fallback (replaced in cycle 4 by a
  direct file read); cmd/go's backup kill in the timeout arithmetic; the `-json` form named
  alongside `-v`; the `CT_INTERNAL_CORE_SECONDS` wording; the stash 63909DAE
  citation. Not applied, accepted as P3: capturing T4 and T5 logs from pwsh 7
  (the gates read either encoding), a floor for R3's lower kill ratio (compound
  rule 3), trimming the ruling-line intake check, and the three-copy drift note.
* cycle 4, no P0 or P1. Six fresh personas over the post-cycle-3 changes:
  Correctness PASS (4 P3); Constitution ADVISORY (1 P2, 6 P3; it again judged the
  Constitution Check line `documented-deviations` correct); Go ADVISORY (1 P2,
  4 P3); Scope ADVISORY (1 P2, 5 P3); Architecture ADVISORY (2 P2, 2 P3);
  Learnings ADVISORY (1 P2, 5 P3; medium confidence). Gate rule: P2 findings only
  gives ADVISORY.
* cycle 4 P2 findings and what was done (text changes after cycle 4, not
  re-reviewed; the T4 and T5 command strings are still byte-unchanged):
  1. Constitution: decisions that came in the Orchestrator's dispatch (the Q2
     ordering, the Q3 timeout bounds, no more probes, no deletion) were
     attributed to the operator, whose recorded words are only "Ruling: option
     A". Fixed in the plan, decision note, and memory note: attributed to the
     Orchestrator's dispatch.
  2. Go: `backlogit stash list` and `backlogit_fetch_stash` rewrite
     `.backlogit/stash.jsonl` without the stash lock, so the A1.3 fallback could
     race a late-landing create and a re-read after the stash commit could
     re-dirty the tree. Fixed in A1.3: the MCP-timeout fallback is one direct
     file read, every re-read precedes the stash commit, none follows it, and
     `git status --porcelain` is checked once afterwards; stash DAEE9D03 was
     widened from `stash get` to the list and fetch paths.
  3. Scope: the T5 CT-scope-file check was assigned to Step 4.4, which is the
     report-only review skill and cannot make T5 incomplete (BLOCKED would start
     a fix loop). Fixed in A1.5: a Ship-run check between the Step 4.3 gate and
     Step 4.4, and the Erratum trigger list names it.
  4. Architecture: Ship cannot stage the deliverable before build-feature's
     completion gate (Step 4.2 is one dispatch call), and that gate's `git diff`
     and `--cached` passes do not list an untracked file, so it halts on an empty
     delta. Fixed in A1.5: the dispatch tells the executor to `git add --` the
     path after writing and before the gate, the status expectation is stated per
     gate, and there is no re-dispatch after a partial write.
  5. Architecture: T5's non-green path had no mechanism and could end in
     build-feature's BLOCKED path (`WAVE_MEMBER_BLOCKED`, then
     `backlogit_return_blocked`, which edits the shipment's items list). Fixed in
     A1.5: a T5 dispatch constraint (no deliverable, no loop, no BLOCKED mark, a
     report), Ship halts through P-005 with 198-S left active, and "returns the
     shipment" is defined as that.
  6. Learnings: this section's earlier claim that Ship's intake halts on an
     ADVISORY without the marker was false (`_ship.agent.md` has no such check;
     it is the same kind of false mechanical hold as the first series' PA-3).
     Fixed: the hold is procedural (the Orchestrator does not dispatch Ship
     until the gate closes), and A1.5's Intake bullet now tells Ship to read this
     section and halt through P-005 unless the decision is PASS or ADVISORY with
     the marker.
* cycle 4 P3 findings applied: A1.6 Principle II and Development Workflow item 1
  stated as following from the relaxation the ruling did name, with the open
  question widened in A1.7 item 1; docline scope wording (`docline.Scope`);
  "about 28 minutes (1656 s)" everywhere; the "Rejected alternatives:" lead-in
  restored in the decision note; the criterion-touching errata wording; the T5
  non-green outcome described as replacing the plan's original outcome (A1.7 item
  6); the closure count scoped to the FU, T5, and R3 follow-ups with duplicates
  and other closure captures listed separately and a mismatch or unanswered read
  a P-005 halt; `R3_FINDINGS` line in the R6 note and the durable-record bullet;
  `.backlogit/stash.jsonl` in Ship's commit list; Quiet host trimmed and the
  compound-rule citation corrected (rules 1 and 4); `CT_INTERNAL_CORE_SECONDS`
  trailing-zero and rerun-log wording; "earlier chance" for T4's `-json` run; the
  assumed 4-minute build allowance; 154-S timeout caveat; the 197-S stash-commit
  citation; the md-lint non-deliverable-findings rule; 197.007-T/197.008-T
  indentation. Not applied, accepted as P3: capturing T4 and T5 logs from pwsh 7,
  a floor for R3's lower kill ratio, trimming the ruling-line intake check,
  the `## Constitution Check` H2 near the top still ending `pass` (A1.6's
  closing line is stated to supersede it), and the three-copy drift note.
* verification after the fixes (Stage, read-only): `verify-plan.v5.ps1` and
  `verify-tasks.v5.ps1` report DRIFT COUNT 0 (plan commands equal the tested
  command files equal the task bodies; the T1 and T3 JSON equals A1.5's); exactly
  one column-0 `a1_4_operator_ruling:` line. Not run, by P-010: `backlogit docs
  lint`, `scripts/md-lint.ps1`, `go list`, and every Go arm (A1.5 assigns them to
  Ship with expected results).
* gate outcome after cycle 4: ADVISORY, approved and now confirmed by the
  operator (see `approval_basis` above; the decision stays ADVISORY). Stage's
  Step 4 treats ADVISORY as satisfied only when the section carries the
  machine-readable approval marker (the label `operator_authorization`, a colon,
  then the word `approved`), which it does, on the operator's confirmation of
  2026-10-13T03:26:01Z (D-1). Stage did not write the marker on its own
  authority; the Orchestrator supplied it on 2026-10-12 and the operator
  confirmed it. All six cycle-4 P2s are fixed in text but were not re-reviewed,
  and the operator ratified that (D-1, D-8). Ship's intake has no check of the
  review decision of its own, so A1.5's Intake bullet is the check, with the
  Step 4.4 mitigation in `approval_basis`.
* closed on 2026-10-13: `%TEMP%\stage-198s` (the operator approved deletion, D-5;
  the directory was found already absent at 2026-10-13T03:39Z, no further action;
  A1.6 and A1.7 item 4(a)). The P-002 mirror
  question and the two Ship Role Boundary readings are closed by D-2 and D-3
  (A1.7). The first-revision P-010 runs were accepted by the operator as a
  disclosed event without re-execution on 2026-10-13T14:20:54Z ("02: Accepted"),
  and the A1.8 errata, as corrected by the attribution fix (O1), were accepted as
  residual risk at the same time ("03: Accepted"; A1.4 acceptance line). No
  decision from Ship's O1 to O5 remains open (O1, O4, and O5 were decided by the
  Orchestrator under the dark-mode activation, A1.7 item 9).

<!-- plan-review-attempt: 1 -->