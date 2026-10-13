---
title: 'Decision: CT harness lifecycle, exempt set, and the operator-ruled T1/T3 red phase'
doc_type: decision
source: docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: 'Stage decision after the 198-S wave-1 P-002 halt: 198.004-T is harness-exempt docs-only and 198.005-T is harness-exempt verification-only, with follow-up stash captures moved out of the gated delta and both runs pinned to a 40-minute timeout; Ship''s mutation-kill red evidence for 198.001-T and 198.003-T was not ratified by Stage because P-004 and the INC-P002-131S-148F forward controls reserve relaxation to the operator, and the operator then ruled Option A for both tasks, recorded with conditions R1 to R6.'
topic: test-suite-health
depth: standard
decision_status: "decided (Decision 1 accepted by Stage; Decision 2 ruled by the operator on 2026-10-12: Option A for T1 and T3, confirmed on 2026-10-13 to reach Principle II ordering, Development Workflow item 1, and P-002's red-phase statements for those two tasks only)"
promoted_to: docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md
linked_artifacts:
  - docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md
  - docs/decisions/2026-08-29-p002-breach-incident-131s-148f.md
  - docs/decisions/2026-08-30-p002-breach-incident-152f-134s.md
  - docs/memory/2026-10-12-ship-198s-wave1-halt.md
---

# Decision: CT harness lifecycle, exempt set, and the operator-ruled T1/T3 red phase

## Subject

Ship claimed 198-S and halted at wave-1 admission (`DARK_MODE_HALTED`, P-002
gap). 198.004-T (docs-only spike) and 198.005-T (verification-only) carried
neither `harness-ready` nor `harness-exempt`, and the plan listed no closed
exempt set. Ship also asked Stage to confirm that it labelled 198.001-T and
198.003-T `harness-ready` on mutation-kill evidence because their tests already
pass on `main`. The normative text is Amendment A1 of
`docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md`. This note holds the
reasoning. A1 was first written as an escalation; the operator then ruled
("Ruling: option A", 2026-10-12T17:35:08Z, Copilot CLI session
a3f32a34-316b-4580-965d-f0a3e5ca2ab4) and a second revision recorded the ruling
and resolved the other open items inside policy. After Ship's pre-PR review halted
(`docs/memory/2026-10-13-ship-198s-prepr-review-halt.md`), the operator answered
its decisions on 2026-10-13T03:26:01Z ("D1 yes, D2 yes, D3 yes, D5 delete, D7
accept, D8 ratify"); Decision 3 below records the dispositions and Amendment A1.8
of the plan holds the errata.

## Decision 1: T4 and T5 are harness-exempt

Policy text consulted: P-002, P-002.1 through P-002.5, and P-004 in
`.github/policies/workflow-policies.md`; harness-architect Step 1a;
`build-feature` Step 0 and the completion gate; Ship Steps 2a, 4.1a, and 4.3.

* 198.004-T changes one Markdown file and zero `*.go` files. `docs-only` is the
  class for that, and the class forbids fabricating a test (P-002.1). Its
  command probes required content and then runs the doc lint gate, which is the
  P-002.3 authoring rule for `docs-only`.
* 198.005-T executes the full suite once and commits one evidence artifact for
  behavior that already shipped (CX) or is delivered by prerequisites (T1 to
  T3). That is the `verification-only` definition. Its only admissible
  deliverable path is `docs/closure/`, so the evidence artifact lives there, and
  Ship's post-merge closure cites it. A post-merge closure cannot be T5's
  deliverable, because T5 completes before the merge.
* Both need a five-key contract block, a closed exempt set in the governing plan,
  and a command that fails before the deliverable exists. A1 supplies all three.
  Review changed the commands materially: `$ErrorActionPreference = "Stop"` (a
  failed cast had let a check be skipped and the marker print); run-evidence
  bindings for T4 and T5 (names as JSON events, a run SHA, `go list ./...`
  coverage, no cached results); repository-contained, git-ignored evidence
  paths; and tighter conditions on T5's rerun branch.
* Follow-up capture ordering (Orchestrator dispatch Q2, P-002.4 not relaxed). The
  first revision had the gates read `.backlogit/stash.jsonl` to confirm captured
  stash IDs. A capture inside either task writes a tracked, non-Markdown file
  after `exempt_baseline_sha`, so it falls in the gated delta and could halt T4
  after its 28-minute run. The Orchestrator's dispatch declined to relax P-002.4, so the
  captures moved to Ship's post-merge closure: T4's findings list `FU-<n>`
  follow-up proposals under a count the gate checks, T5's artifact carries a
  `CT_FLAKE_DESCRIPTION` that must name each failed test, and neither gate reads
  the stash. An earlier draft put the capture at Ship Step 5 item 10; review showed
  that step follows the readiness and Copilot-review gates, so a capture commit
  there would advance the reviewed HEAD, and the 197-S closure captured at the
  post-merge point anyway. A1.3 holds the procedure. The change moves the location
  of T4 AC1's "stash IDs captured" evidence to Ship's closure record and a task
  comment; the obligation and its intent are unchanged, and A1.1 and A1.3 record
  the erratum.
* Timeout (Orchestrator dispatch Q3). Both runs pin `-timeout 40m`: the worst observed
  `internal/core` time is 1656 s (27.6 minutes), so 40 minutes leaves 31% headroom
  against 8% at 30, within the 45-minute command limit. A1.3's Timeout paragraph
  holds the reasoning, the stall-detection cost, and two cautions: this is a
  task-local run parameter and not the governed 073-DL budget (Ship's gates keep
  `-timeout=30m`, stash F8936891), and 40 minutes is 1.45 times the largest
  package, below 073-DL's 1.5 times, with the Orchestrator's Q3 direction as the
  authority. T4's earlier `-timeout 45m` equalled the command limit, so it moved
  to 40m too. cmd/go also arms a backup kill one minute after the timeout, so a
  hung run's worst case is 40 + 1 + 4 (build) = 45 minutes, at the limit and not
  over it.
* Deliverable frontmatter. Both specs name `title`, `source`, `doc_type`,
  `chunk_strategy: h1-h2-h3`, and `schema_version: "1.0"`. The last two are
  required of every tracked, in-scope `docs/**` Markdown file (`docline.Scope`;
  `docs/memory/` and `docs/archive/` are excluded) by
  `tests/integration/docline_soft_keys_test.go`, and neither `backlogit docs lint`
  nor the exempt gates check them, so a deliverable without them would first fail
  in a full-suite run (T4's at Ship's wave-1 Step 4.6, after the 28-minute run).
  The commands are unchanged; the spec is the control.
* T5's non-green outcome. The plan's third red-run branch ("record CT closure as
  verification-incomplete") cannot complete an exempt task whose gate exits 0 only
  over a green suite. Ship leaves T5 non-done, reports the failure through P-005,
  and halts with 198-S left active (A1.5); Stage captures the policy's entry at
  triage. A package timeout takes the same branch. This replaces the plan's
  original outcome for that branch (a CT closure recorded as incomplete) and no
  acceptance criterion's text changes; the plan discloses it (A1.1, A1.3, A1.7
  item 6). The gate's CT-scope name match lists only the T1, T2 (`TestUT2_`), and
  T3 tests, so the artifact also states whether a failed test is defined in a file
  T1 to T3 edited, and Ship runs that check itself between the Step 4.3 gate and
  Step 4.4 (A1.5), treating a hit as verification-incomplete, because Step 4.4 is
  the report-only review skill (the command does not check it).

Rejected alternatives:

* Scaffold a file-existence test: forbidden fabrication (P-002.1) and a vacuous
  red (P-002.3).
* Descope T4 or T5: changes M and task scope, which this dispatch forbids.
* Put the exempt set in feature 198-F as 197-F did: workable, but 197-F did so
  only because its plan had no set. Two enumerations would be two sources of
  truth, so the plan is the single home, and Stage reads each task body back
  against the plan when it applies the blocks.
* A quiet-host attestation, an `internal/core` exclusion on T5's rerun branch,
  and an `Elapsed` cross-check of T4's total against the saved JSON: the gate
  cannot verify the first, the plan's red-run policy allows a rerun of any one
  package, and the third was not validated against real `go test -json` output.
  T5 and T4 instead require the artifact to state host condition and the failed
  test, which a reviewer can check.
* Relax P-002.4 for stash captures made during an exempt task (the first
  revision's open question): the Orchestrator's dispatch did not relax it, and a carve-out in a
  gate that exists to be unfalsifiable is not Stage's to grant. Reordering the
  captures needs no relaxation.
* Capture the stash entries inside the task before the gate, or pre-capture
  guessed follow-ups now: the first falls in the delta, the second invents
  results before the spike runs.
* Capture at Ship Step 5 item 10 (an earlier draft): it follows the readiness and
  Copilot-review gates, so the capture commit would advance the reviewed HEAD
  and could re-arm P-018.
* Raise only T5's timeout and leave T4's `-timeout 45m`: 45 minutes equals the
  command limit, so a stall would race the circuit breaker.
* Pin `-timeout 42m` to meet 073-DL's 1.5 times criterion: it leaves 3 minutes
  under the command limit for build and dump, and 073-DL governs Ship's gates, not
  T5's own run.
* Add `-v` to T5's run (the compound lesson's rule 2): it puts every test's
  stdout at column 0 and could trip the gate's `panic:` and `FAIL` checks. A1.3
  records the deviation and its cost (a nameless background-goroutine panic, as in
  the 197-S unnamed-panic case, stash 63909DAE, ends T5 as verification-incomplete).
  `-json` is the compound rule's other allowed form and is not used either: it would
  replace the `ok` lines the gate reads.

## Decision 2: T1 and T3 red phase: the operator ruled Option A

Stage's first draft ratified Ship's mutation-kill evidence under conditions.
Review overturned that, and Stage escalated. A1.4 holds the governing text, the
facts, the ruling line, the Principle II conflict record and the VII and VIII
checks, the relaxation conditions R1 to R6, and the run mechanics; this section
keeps only what A1.4 does not.

Why Stage could not ratify. Relaxing P-004 and the INC-P002-131S-148F forward
controls is reserved to the operator, a request from the Orchestrator is not the
operator's relaxation, and dark mode is not a waiver (P-017). The forward
controls are not embedded in the Ship agent or skills, so a hold that lives in
prose would not have stopped a silent deviation; the hold was procedural, and A1
said so.

The options the operator saw. Option A: relax, for that task only, P-004's literal
wording and its `Red Phase: CONFIRMED` token, FC-1's separate-commit ordering,
FC-2 step 4, and FC-3's red-observed and separate-harness-SHA confirmations
(chosen; A1.4 quotes it). Option B (T1 only): a new revert commit removes the T1
test edit (`internal/core/gate_evidence_formal_test.go`), harness-architect
scaffolds a genuinely red test-only outcome-classifier helper fed synthetic
inputs, as in the ur3 flake deliberation, and build-feature implements it; it adds
helper code the plan does not call for, so it needs a reviewed Stage scope
amendment, T3 has no deterministic red without a production seam, and Stage would
remove T1's `harness-ready` label. Option C: descope the task through a reviewed
amendment that changes M and 198-S. B and C were not chosen and stay available
only through a new ruling line and a reviewed amendment.

The ruling. The operator wrote "Ruling: option A", which the Orchestrator read as
applying to both T1 and T3 (B is T1-only and no descope was chosen). Stage
recorded it as `T1=A; T3=A` and relaxed exactly the controls Option A listed, for
these two tasks only and as no precedent. P-002's own red-phase statements state
the same requirement as P-004's wording but were not named in the quoted option,
so Stage did not extend the ruling to them and left the question to the operator
(A1.7 item 1). The operator answered it on 2026-10-13 (D-2, yes): Option A's
wording reaches Principle II's ordering requirement, Development Workflow item 1,
and P-002's red-phase statements, for 198.001-T and 198.003-T only, and nothing
else. That is recorded as a documented Constitution-Check deviation with operator
authority (A1.6), not a general relaxation. Principle II stays in force for all
production code: CT writes none (R2 proves it), and the mutation-kill experiment
preserves the observation the principle exists for.

How Stage made the ruling runnable without running it. P-010 bars Stage from
running Go. R6 therefore specifies a `go test -overlay` experiment on mutant
copies under an ignored scratch directory that `./...` skips (an underscore name,
after review found a plain `logs/` directory would have broken every repo-wide Go
command), one invocation per task so the circuit breaker is not tripped, with
unmutated controls and a before-and-after no-production-delta check, for Ship to
run through a general-purpose subagent under `careful` mode then `freeze-scope`;
the commit-free completion path; and a `green-regression-contract` block per task
so that the AC's `-count=30`, `-cpu 1`, and `-count=200` repetitions actually run
at Ship's Step 4.3. Stage has not run `-overlay` on this module, so a build error
from it halts rather than falling back to a production-file edit.

Rejected alternatives:

* Withdraw the `harness-ready` labels (PA-3): an earlier draft did this on the
  claim that intake would then halt mechanically. Ship Step 2 sends a task with
  neither label to harness-architect, so it does not halt, and it invites the
  fabricated red that A1.4 rules out. It also goes beyond the labels the exempt
  classification needs. Review cycle 3 failed it on both grounds. The labels
  stay as Ship's claim, and under the ruling they stand on A1.4's terms.
* Source-shape (`go/ast`) harness over the old test text: that mechanism is
  specified for declarations and would test source text rather than behavior.
* Reclassify T1 and T3 as `verification-only`: inadmissible, since both weaken
  existing assertions, and an empty delta halts.
* Ratify and ask for operator confirmation at PR review: pre-authorized merge
  means no human is guaranteed to read the annotation first, so it would be a
  self-granted waiver with an audit trail.
* Rely on a task comment alone: the comment lands in an ignored item log and no
  gate reads it.
* Mutate the production file in the working tree instead of an overlay: the
  first revision kept this as a fallback; it puts a production file in the delta
  and was dropped, so a failed overlay returns to Stage instead.

## Decision 3: dispositions after Ship's pre-PR review (2026-10-13)

The operator's six answers, in exactly these words, at 2026-10-13T03:26:01Z: "D1 yes,
D2 yes, D3 yes, D5 delete, D7 accept, D8 ratify". Read against Ship's memo (the
plan's A1.4 confirmation line records them): D-1 yes, the operator confirms the
plan-review ADVISORY approval (the marker was first supplied by the Orchestrator
as dark-mode proxy on 2026-10-12T22:15:18Z; the confirmation makes it genuine);
D-2 yes, as in Decision 2 (T1 and T3 only); D-3 yes, a spike
deliverable produced by an executor under Ship's dispatch and an R6 executor
writing only ignored scratch files are within Ship's boundary; D-5 delete, which
answered a prompt that described both the `%TEMP%\stage-198s` fixtures and Stage's
first-revision linter, `backlogit docs lint`, and `go list` runs. The operator
approved deleting the directory (it was then found already absent at
2026-10-13T03:39Z, removed by an unknown earlier step, so no action was needed;
this record does not say who or what removed it). A1.6 records a Principle IV
violation honestly. Stage reads nothing more about the P-010 runs into "D5
delete"; the operator accepted those separately (below). D-7 accept, the
2026-10-11 in-tree production mutation during wave-1 scaffolding (reverted, R2
empty at HEAD `d3aff2c5`) is a disclosed P-005 event; D-8 ratify, the second
plan-review series of four cycles after the first series' three-attempt cap,
including the Orchestrator's cycle-4 authorization.

Later, at 2026-10-13T14:20:54Z, the operator answered two decisions of Ship's
round-2 memo with exactly "02: Accepted" and "03: Accepted", read as that memo's O2
and O3: Stage's first-revision P-010 runs (linters, `backlogit docs lint`, `go
list`, pwsh fixtures) are accepted as a disclosed P-010 event with no
re-execution, and the A1.8 errata, as corrected by the attribution fix, are
accepted as residual risk.

Decided by the Orchestrator within scope, not by the operator: D-4, Ship's
exact-name selector for 198.002-T AC2 (eight PASS) stands as that criterion's
disposition, because the criterion's pattern matched no test (A1.8 E-1); D-6, T5's
result is GREEN-AFTER-RERUN, stash 63909DAE is the recurrence, and "the isolation
fixes held" is not stated (A1.8 E-8); O1, D-4 and D-6 are the Orchestrator's
calls and the attribution in Stage's text was corrected; O4, the T5 scalar
`CT_ISOLATION_FIXES_HELD` stays as a gate-required token whose recorded value is
the three IDs and not a claim, and the PR body must not assert the fixes held
(A1.8 E-8); O5, the unattested executor model for the R5 mutation runs is accepted
as disclosed residual risk, with an optional operator spot-check of the `--- FAIL:`
lines. Stage recorded these and edited plan text and the 198.002-T and 198.005-T
body notes. With the operator's O2 and O3 answers, no decision from Ship's O1 to
O5 remains open.

Errata (A1.8): E-1 the AC2 disposition; E-2 T2's third file and the fourth
`slog.SetDefault` site (stash 8640432F), the plan's "only three sites" premise
being true at `d2f3ff97` and false at the branch base `c5758918`; E-3 the base
reference pinned to `c5758918`; E-4 one T5 non-green trigger set; E-5 the dangling
"returns the shipment" definition; E-6 the 198-F prerequisite text; E-7 T4 does not
reuse T5's run; E-8 the T5 result and isolation wording; E-9 the Stage memo
timestamp and stash 253E2505's status text; E-10 the corrections after Ship's
round-2 delta review. E-1 and E-8 are dispositions (an evidence substitution for
an acceptance criterion whose text is unchanged, and a rule for what the T5
evidence and closure may say), not text-only corrections; E-2, E-6, and E-7 also
edit a task or feature body. The wording defects were captured by Ship in stash
FBD0BB95 (P-021 C2) and are Stage's to correct. Each fix was checked against its
source, and none changes a task, an acceptance-criterion text, a dependency, or M.
The errata postdate the operator's six answers, so those answers do not accept
them; the operator accepted them as corrected, as residual risk, with "03:
Accepted".

## Follow-ups (captured as stash entries, not acted on)

* Stash FBD0BB95 (task, low): the wording defects corrected by A1.8 E-2 to E-9
  (E-9 records the stash 253E2505 correction); Stage's correction is recorded
  there and the entry can be archived at triage.
* Stash 7802D798 (task, medium) and 901C79CC (low): Stage-owned plan and record
  text from Ship's round-2 delta review, corrected by A1.8 E-10. Declined, because
  the text is Ship's: 901C79CC items (b) and (d), Ship's disposition record and
  its decision-ID mismatch with the operator's D-1 to D-8.
* Stash 8640432F (bug, low): the fourth `slog.SetDefault` site, benchmark-only
  (`silenceBenchLogs`), deferred by P-021 C1.
* Stash 63909DAE: the T5 first-run failure is its recurrence.

* Stash 920BCA5B (task, medium): P-002.1 and P-004 give no admissible path to a
  tests-only stabilization whose red cannot be observed before the deliverable
  (narrowed after review: 181.001-T in 182-S shipped under normal P-002 with a
  reproduce-first red, so this is not a general impossibility).
* Stash 253E2505 (task, medium), related to stash 7AA35A39: P-002.4 exempt
  completion-gate delta versus `.backlogit/**` backlog-state writes made during
  a task. No longer blocks 198-S; the general gap stays open.
* Stash 5A6F54BB (task, low): T1's relaxed assertion no longer detects a stuck
  counter lock; consider a deterministic subtest.
* Stash F8936891 (task, medium): Ship's wave and final gates pin
  `-timeout=30m` while `internal/core` ran at 92% of it.
* Stash DAEE9D03 (bug, medium): `backlogit stash get` is not read-only.
* Stash 5F5D7384 (task, low): build-feature has no stated zero-delta handling for
  a green-on-arrival harness, and harness-architect hard-codes
  `Red Phase: CONFIRMED`.
* Stash 2462F1B0 (task, low): P-010 forbids Stage from exercising the commands it
  must author under P-002.3 and P-002.5.

## Residual risk

* T1's relaxed form no longer detects a stuck counter lock, and mutation-kill
  covers only the unlock-after-append revert (stash 5A6F54BB).
* Stage could not run the Go-dependent or lint arms of the exempt commands, the
  `-overlay` experiment, or the markdown and docs linters (P-010 and the
  operator's no-more-probes direction). The first real run of each is Ship's, and
  A1.5 lists them. A difference is a halt returned to Stage.
* Ship's own gates keep a 30-minute test timeout, with a four-run exposure near
  the `internal/core` budget (stash F8936891). A halt there goes to the operator.
* Stage process events: the first fixtures written under `%TEMP%\stage-198s`
  (a Principle IV violation, recorded in A1.6; the operator approved deleting the
  directory, D-5, and it was found already absent at 2026-10-13T03:39Z, removed by
  an unknown earlier step, no further action; this note does not say who or what
  removed it) and the first revision's P-010 authoring probes (the operator
  accepted them as a disclosed event without re-execution, "02: Accepted",
  2026-10-13T14:20:54Z).
* The R5 mutation executor's model could not be attested. The Orchestrator
  accepted that as disclosed residual risk (O5); an operator spot-check of the
  `--- FAIL:` lines in the T1 and T3 mutation runs is optional and available.
* The T5 artifact's original "Isolation fixes" prose said the three isolation
  fixes held; that sentence cannot stand for a GREEN-AFTER-RERUN result (A1.8 E-8).
  Ship's "Ship annotation" section in the artifact supersedes it.
* Landing is working-tree only, for Ship to commit by explicit path, because the
  Orchestrator's concurrency rule keeps Stage and Ship off the same branch and
  P-010 names the default or an admin branch as Stage's commit venue. Rollback is
  in A1.7 PA-1.
