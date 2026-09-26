---
chunk_strategy: h1-h2-h3
description: "Final-gate review and verification record for shipment 155-S, including the single governed-suite docline failure."
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-09-25-155-S-final-review.md
title: "155-S Final Review Record"
---

# 155-S Final Review Record

> [!NOTE]
> The earlier sections preserve the review history at each point in time. Their blocked or incomplete verdicts are historical and are superseded by the final-gate Step 2 outcome below.

## Post-push review attempt — 2026-09-26

**Outcome:** `BLOCKED — REVIEW INCOMPLETE`
**Shipment:** `155-S` only
**Branch:** `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`
**Base:** `37a5cba4713953c855013fafd4fc05e479e5618c`
**Reviewed HEAD:** `98367132efac9de97ddf312f34e8afd163d45589`
**Remote HEAD at review start:** `98367132efac9de97ddf312f34e8afd163d45589`
**Diff:** `logs/review/155-s-final.diff`
**Changed-file list:** `logs/review/155-s-changed-files.txt`
**Diff SHA-256:** `4D499FFE00CAD35AEE08866997080E070A9D36FC40AF4F2BB18BCB3FBAAC74FC`
**PR:** None created.

### Standard review

The standard persona review returned candidate findings, but several reviewers
reported incomplete coverage of the 1.8 MB diff. The outputs were not
deduplicated into a complete, current-HEAD verdict. The following are
provisional candidate areas, not final finding counts or accepted dispositions:

* `internal/core/artifacts.go`: protected-status creation without canonical
  preimage evidence; pre-update hooks observing unsanitized blocked-envelope
  fields; and ambiguous errors after durable mutation with audit append failure.
* `internal/core/shipment.go` and
  `internal/core/shipment_recovery.go`: lock ordering and lock scope,
  normalizer lock-release error handling, and nil-workspace behavior.
* `.github/agents/_stage.agent.md`,
  `.github/agents/_orchestrator.agent.md`, and `.github/agents/_ship.agent.md`:
  role-boundary concerns about tool grants. The operator-approved
  server-qualified tool syntax does not by itself resolve the separate
  permission-scope question.
* `.autoharness/backlog-registry.yaml`,
  `internal/cli/shipment.go`, and
  `docs/cli-reference/backlogit_doctor.md`: MCP-only metadata, required CLI
  flags, and Doctor documentation parity.
* `.backlogit/queue/155-S.md` retains an `UNAPPROVED / BLOCKED` active-residual
  waiver statement, while the current Orchestrator directive identifies
  plan rev23.5 as operator-approved. The exact per-shipment waiver evidence
  was not reconciled in this review attempt.
* The review also questioned whether the W3 AC5 gate evidence is recorded in
  the archived task manifest. The selectors were run and their output is
  captured in the session diagnostics, but the manifest discrepancy was not
  resolved here.
* Unrelated archived-memory moves were raised as a scope concern. No files
  were restored or otherwise changed.

No finding from this partial pass is marked fixed, deferred, or invalidated.
No P-021 C1 disposition is final, and no out-of-scope candidate is treated as
closed. Consequently, there are no authoritative standard-review P0/P1/P2/P3
counts.

### Adversarial review

The requested four-slot review was not completed. The adversarial-review agent
declined before reading the diff or dispatching any reviewer because the
configured multi-provider dispatch would transmit repository source to
external model providers.

| Slot | Requested route | Result |
|---|---|---|
| Anchor | `openai / gpt-6-sol / high` | Not dispatched |
| Tier 1 | `openai / gpt-6-luna / xhigh` | Not dispatched |
| Tier 2 | `anthropic / claude-sonnet-5 / high` | Not dispatched |
| Tier 3 | `anthropic / claude-opus-5.5 / high` | Not dispatched |

No fallback retry was attempted. No reviewer accessed the diff, so there are
no adversarial observations, consensus/majority/plurality/unique counts, or
P-021 dispositions to report. The required minimum reviewer count was not
met; this is a blocking review gate, not a clean or degraded review verdict.

### Process deviations

Commit `98367132efac9de97ddf312f34e8afd163d45589` contains the U20C5 source
change made outside the `build-feature` workflow. The Orchestrator accepted
this as a procedural deviation and directed that the commit stand; it was not
rewritten, reverted, or re-routed. The harness-correction comment on
`174.081-T` was recorded by the Orchestrator.

### Gate disposition

* Local review readiness is `BLOCKED`; the standard review is incomplete and
  the adversarial review did not run.
* No code remediation or post-remediation review was performed after this
  push.
* No PR, CI, P-018 Copilot-review, or merge gate was started.
* The governed full suite remains withheld and was not rerun.
* Do not create a PR until a complete review has been performed through an
  approved review route, findings have final P-021 dispositions, and all
  blocking in-scope findings are resolved.

**Review date:** 2026-09-25  
**Shipment:** `155-S` only  
**Branch:** `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`  
**Reviewed HEAD:** `3a240fe091422e96d22b75b9af0854f66b60340f`  
**Review range:** `37a5cba4713953c855013fafd4fc05e479e5618c..3a240fe091422e96d22b75b9af0854f66b60340f`  
**PR:** None created for this branch.

## Persistence and provenance

Neither report was persisted to a repository file when originally produced.
The standard review ran in report-only mode and returned its findings inline;
the adversarial review also ran report-only and returned its incomplete
aggregate inline. Neither wrote an artifact. This document now records those
returned results. Finding IDs below (`STD-P2-01` through `STD-P2-03`) are
assigned here for reference; the original standard-review output did not
provide finding IDs.

The original adversarial aggregate assigned three reviewers:

| Route | Model |
|---|---|
| Anchor | `openai / gpt-5.6-sol` |
| Tier 1 | `claude-haiku-4.5` |
| Tier 3 | `claude-opus-4.8` |

It says the Tier 3 reviewer (`claude-opus-4.8`) could not access the exact diff
and returned a non-authoritative empty result. The aggregate did not preserve
the underlying error text or code. It also did not attribute each candidate
observation to an individual reviewer. It only states that the two reviewers
whose observations were returned produced the candidate observations below.
The report identifies the Anchor and Tier 1 routes as the other two assigned
reviewers, but does not say which of those two raised any particular row.
Attributing a specific row to one of them would be unsupported.

## Standard review

**Outcome:** `NOT READY`  
**Counts:** P0 0, P1 0, P2 3, P3 0  
**Runtime verification follow-up:** Not run; each finding's reviewer text
recommended validation after remediation.

### STD-P2-01 — Generic update may invalidate a blocked shipment envelope

**File and lines:** `internal/core/artifacts.go:640-641`  
**Severity / confidence:** P2 / 0.93  
**P-021 C1 disposition in the review:** In scope, same blocked-lifecycle
contract surface.

**Defect:** The exported core update path accepts `updates["custom_fields"]`
and replaces shipment custom fields other than reserved sizing keys. An
unchanged-status write can therefore leave a shipment in `blocked` status
while removing or changing canonical envelope values such as its reason,
timestamp, or member snapshot. `UnblockShipment` then rejects the envelope
and normalization may be needed to recover.

**Why classified in scope:** The reviewed change surface is the blocked
shipment lifecycle. The reported repair would preserve that lifecycle's
canonical blocked envelope against a generic update; it does not describe a
new contract surface. P-021 C1 requires that a fix complete only the exact
authorized change on the same contract surface; file, function, PR, or
subsystem proximity alone would not establish scope.

**Suggested fix:** Protect lifecycle-reserved fields from generic updates
while blocked, or reject updates that would invalidate the canonical
envelope. After remediation, verify generic updates cannot invalidate a
blocked envelope and valid unblock/normalization paths still work.

### STD-P2-02 — Unblock audit event omits the prior reason and follows metadata clearing

**File and lines:** `internal/core/shipment.go:714-720, 775-815`  
**Severity / confidence:** P2 / 0.99  
**P-021 C1 disposition in the review:** In scope, same blocked-lifecycle
audit contract surface.

**Defect:** The unblock event delta carries `unblocked_by` and
`resume_checkpoint_ref`, but omits the validated envelope's `reason`.
Persistence clears `blocked_reason` at lines 778-780 before the correlated
`shipment_status_changed` event is appended at lines 808-815. The review says
the unblock audit contract requires actor, reason, and prior checkpoint
reference before blocked metadata is cleared.

**Why classified in scope:** This concerns the audit evidence emitted by the
authorized blocked-to-unblocked lifecycle operation itself. Including the
validated prior reason and preserving the specified event ordering completes
the same lifecycle/audit contract rather than adding a different feature.

**Suggested fix:** Include the validated prior reason and record the required
correlated audit evidence before clearing blocked metadata, while preserving
the existing recovery protocol. After remediation, verify event fields and
ordering for both unblock targets.

### STD-P2-03 — Compensation can terminalize a journal before its audit event is durable

**File and lines:** `internal/core/shipment.go:476-490, 745-760`; recovery skip
reported at `internal/core/shipment.go:2600-2602`  
**Severity / confidence:** P2 / 0.99  
**P-021 C1 disposition in the review:** In scope, same lifecycle
transaction/recovery contract surface.

**Defect:** Both compensation paths persist journal `Phase = "compensated"`
before appending the terminal lifecycle event. If event append fails or the
process stops between those operations, `recoverPendingShipmentOperations`
skips the journal because it is no longer in `intent` phase. The intent can
remain without terminal event evidence, and Doctor may report a torn lifecycle
intent that recovery does not repair.

**Why classified in scope:** The reported defect is in transaction completion
and recovery evidence for the same shipment lifecycle operation being
reviewed. Making terminal journal state recoverable until its required status
and terminal evidence are durable completes that lifecycle/recovery contract.

**Suggested fix:** Keep compensation recoverable until required status and
terminal evidence are durable, or make recovery repair terminal journals
whose required evidence is incomplete. After remediation, inject failure
between terminal-journal persistence and event append, then verify recovery
and Doctor converge.

## Adversarial review

**Outcome:** Incomplete; no valid three-reviewer consensus, majority,
plurality, or confidence classification. These rows are unverified candidate
observations, not authoritative findings. The aggregate's provisional P-021
classifications are retained as provisional only.

| Candidate severity | File and line | Candidate observation and suggested fix | Reviewer attribution |
|---|---|---|---|
| CRITICAL / candidate P0 | `.backlogit/backlogit.db-shm:0` (no valid source line) | **Invalid, dismissed.** `git ls-files .backlogit` showed no tracked database files; `.gitignore` lines 32-35 ignore the backlogit DB, SHM, and WAL files. No repository change is warranted. | Not attributed per observation. |
| MAJOR / candidate P1 | `.github/agents/_stage.agent.md:6` | **Invalid, dismissed.** The committed Stage agent at HEAD uses `backlogit/*`; the operator approved the server-qualified wildcard form in `3a240fe0`, and `<server>/*` is documented as valid for VS Code and Copilot CLI. | Not attributed per observation. |
| MAJOR / candidate P1 | `internal/core/shipment.go:562` (also cited at `:825`) | Alleged committed lifecycle evidence can be appended before journaling the committed phase; a journal-write failure could then allow contradictory committed and compensated terminal events. Proposed avoiding compensation after committed evidence is appended and letting recovery reconcile. Provisionally in scope as lifecycle ordering/recovery. | Not attributed per observation. |
| MAJOR / candidate P1 | `internal/mcp/tools.go:2082` | Alleged the recovery tool uses the recovery path only when workspace initialization fails, so a warmed server may be blocked by a later poison journal. Proposed using the recovery path regardless of initialization state while retaining the target-shipment guard. Provisionally in scope as the blocked-shipment recovery tool contract. | Not attributed per observation. |
| MAJOR / candidate P1 | `internal/core/shipment_recovery.go:822` | **Confirmed behavior; out of scope for this fix cycle under P-021 C1.** `removeShipmentOperationEvents` rewrites JSONL and is called by claim recovery and return-blocked rollback. A complete fix would change shared event-history and claim/return-blocked recovery semantics, beyond the exact authorized SBLK blocked-lifecycle surface. No code change was made. The capture-first deferred entry is `388C586D`; Stage deliberation is required. | Not attributed per observation. |
| MINOR / candidate P2 | `internal/mcp/tools.go:469` | **Confirmed, in scope.** The normalize tool declares `by` optional, while core `normalizeBlockedShipment` rejects an empty actor. Proposed making `by` required on the normalize tool only; Block/Unblock actor parameters remain advisory and optional. | Not attributed per observation. |

The aggregate's raw candidate mapping was P0 1, P1 4, P2 1, P3 0. These are
not validated finding counts. Exact HIGH- and MEDIUM-confidence P0/P1 counts
were unavailable because the required third reviewer did not return a valid
review.

## Gate disposition

The standard review was `NOT READY` with three unresolved P2 findings. The
adversarial review was incomplete. A governed remediation attempt stopped in
cycle 1 before the expected assertion RED: the test command failed to compile
because a newly added test declared but did not use `root`. No production code
was changed, and cycles 2 and 3 were not started. The compile failure is not
accepted as red-phase evidence and no code fix was applied.

### Governed candidate validation and remediation halt

Read-only source validation confirmed the following remaining candidates:

* `internal/core/shipment.go:558-563, 821-826`: `shipment_lifecycle` committed
  evidence is appended before the journal is marked committed; a journal-write
  error takes the compensation path. This is in scope for lifecycle
  transaction/recovery ordering.
* `internal/mcp/tools.go:2057-2087`: the recovery normalizer is selected only
  when `requireWorkspace` fails. A warm server uses ordinary normalization,
  which auto-recovers pending operations; the recovery normalizer skips
  unrelated journals while retaining the target-shipment intent guard. This
  is in scope for blocked-shipment normalization.
* `internal/mcp/tools.go:465-470` and
  `internal/core/shipment_recovery.go:1063-1092`: the normalize MCP schema
  leaves `by` optional, while the core normalizer rejects an empty actor. This
  is in scope for the same MCP normalization contract.

The standard P2s and the confirmed in-scope candidates remain unremediated.
The delegated Go Engineer added test-only code to
`internal/core/shipment_blocked_recovery_harness_test.go` and ran:

```text
go test ./internal/core -run '^(TestBlockedShipmentGenericCustomFieldsPreserveEnvelope|TestUnblockShipmentStatusEventPrecedesEnvelopeClear)$' -count=1
```

It exited 1 with:
`internal/core/shipment_blocked_recovery_harness_test.go:1498:2: declared and not used: root`.
No expected assertion RED was observed. No further tests, vet, build, lint,
format check, final review, or full-suite execution followed. The draft test
diff remains uncommitted; all acquired file locks were released.

The pre-PR, threadless P-021 C2 capture for the out-of-scope event-history
expansion is `388C586D`. Active and archived stash discovery found no
positively matching entry; the entry was captured rather than suppressed.
Its six-field payload cites `174.076-T` as final-review context, not as a
unique originating code owner, plus feature `174-F`, shipment `155-S`, and
`PR=N/A` / `review_thread=N/A`. Stage must deliberate on that entry before
planning a separate task for the wider event-history change.

No remediation commit was created or pushed. HEAD remains
`3a240fe091422e96d22b75b9af0854f66b60340f`; no PR, CI, P-018, merge,
shipment archival, or post-merge closure was reached. The governed full suite
was not rerun. A new operator authorization will be required for a governed
full-suite run after any future production-code change.

## Final-gate Step 2 — Wave 20 review and remediation

* **Outcome:** `READY` for final-gate Step 2
* **Shipment:** `155-S` only
* **Review scope:** Wave 20 surfaces, per plan final-gate step 2
* **PR:** None created.

### Scope and review coverage

The initial whole-branch diff was approximately 1.8 MB. The standard review
could not complete that scope. The final-gate Step 2 review was therefore
re-scoped to Wave 20:
`git diff 3a240fe0..HEAD -- internal cmd tests`, captured at the round-1
review HEAD `eeecf01d` in `logs/review/155-s-wave20.diff` (approximately
90 KB).
The file contained the Wave 20 implementation and harness surfaces. The
adversarial-review custom agent refused to dispatch, citing external
transmission; the Orchestrator determined that refusal was invalid and
replaced it with direct dispatch to four independent code-review agents.

### Standard review

Eight personas reviewed the Wave 20 diff: Go Reviewer, Constitution Reviewer,
Learnings Researcher, Architecture Strategist, Concurrency Reviewer, Scope
Boundary Auditor, Agent-Native Parity Reviewer, and Security Reviewer.
The review returned three P2 findings, no P0/P1 findings, and no other
actionable findings.

| ID | Severity | Finding | P-021 C1 disposition |
|---|---|---|---|
| STD-W20-01 | P2 | Fractional `blocked_at` values compare equal after `canonicalBlockedTimestamp` formats to whole seconds. | Out of scope: §20.3.1 prescribes this shared helper. Deferred as `DEFERRED SCOPE EXPANSION` stash `AC6B669D`. |
| STD-W20-02 | P2 | Cold workspace initialization may run auto-recovery before the target-guarded normalize operation. | Out of scope: §20.3.4 preserves workspace or diagnostic-workspace acquisition order. Deferred as `DEFERRED SCOPE EXPANSION` stash `A4B62D86`, related to `09D06A75`. |
| STD-W20-03 | P2 | The low-level artifact writer has a check/write race for blocked-envelope changes. | Out of scope; already deferred under stash `8AF55264`. No new entry or code change. |

The candidates at `internal/core/shipment_recovery.go:1201` (scan-error
classification) and `:1274` (unrecognized journal filenames) were dismissed
against §20.3.6: the specified behavior wraps scan errors as
`ErrShipmentConflict`, and only return-blocked validation errors attributed
by the journal filename are considered by the aggregate guard.

### Adversarial review — round 1

All four slots received the Wave 20 diff file.

| Slot | Route | Result |
|---|---|---|
| Anchor | `gpt-6-sol / openai / high` | `NOT READY`; P2 finding `ADV-W20-01`. |
| Tier 1 | `gpt-6-luna / openai / xhigh` | `READY`; no findings. |
| Tier 2 | `claude-sonnet-5 / anthropic / high` | `READY`; no findings. |
| Tier 3 | `claude-opus-5.5 / anthropic / high` | `READY`; P3 observation on the same issue as `ADV-W20-01`. |

Consensus classified `ADV-W20-01` as **MEDIUM confidence / plurality
(2/4 reviewers)**, P2, in scope under P-021 C1, and `gated_auto`.
The finding was that function 2 of
`TestU20C3_CompensationJournalFailureAfterTerminalEvidenceRecovers` called
`realWrite` before returning the injected error. That tested landed-then-failed
behavior rather than the §20.3.3 required failed-before-writing behavior; the
`error_shape_wrapping_write_not_applied` row therefore contradicted its
intended case.

### Remediation

`ADV-W20-01` was fixed in commit
`d9a01e6fb242a9d73824b2e8f0df865e4d227d69`. Function 2 now arms the obstruction
and returns its injected error without calling `realWrite`; its injected-error
messages consistently identify the pre-write failure. Contract assertions
were retained.

Verification passed: the U20C3 harness reported 2/2 top-level PASS; the core
regression command reported 17/17; the CLI regression command reported 3/3;
build, `go vet ./internal/core`, pinned golangci-lint v1.64.8, and the
LF-normalized gofmt check passed. The full repository suite was not run.

### Post-remediation re-review — cycle 1 of maximum 2

The four slots reviewed `logs/review/155-s-wave20-remediation.diff` on the
post-remediation commit:

| Slot | Route | Result |
|---|---|---|
| Anchor | `gpt-6-sol / openai / high` | `READY`; zero findings. |
| Tier 1 | `gpt-6-luna / openai / xhigh` | `READY`; zero findings. |
| Tier 2 | `claude-sonnet-5 / anthropic / high` | `READY`; zero findings. |
| Tier 3 | `claude-opus-5.5 / anthropic / high` | `READY`; zero findings. |

**Residual findings:** None.

### Final-gate disposition and process deviations

Final-gate Step 2 is `READY`: zero P0/P1 findings and no unresolved in-scope
P2 findings remain. Final-gate Step 1 was subsequently run once with operator
authorization; its result is documented below. No full-suite rerun is claimed.

The following process deviations and harness corrections are recorded:

* Commit `98367132` includes a 174.081-T production edit made directly rather
  than through the build-feature skill. The Orchestrator accepted the
  procedural deviation because all required gates passed.
* Harness corrections `f7db30a3` (U20C1 over-assertion) and `b78ce24b`
  (U20C5 presence-based count) were made to match the reviewed plan evidence.
* The adversarial-review custom agent's refusal was invalid for this
  workspace; the Orchestrator replaced it with direct four-slot review
  dispatch.

No PR, CI, P-018 Copilot-review, merge, shipment archival, or post-merge
closure was started. The governed full-suite outcome is recorded below; no
full-suite rerun is claimed.

## Final-gate Step 1 — governed full suite

The operator authorized one governed full-suite run at HEAD `0b11459f` on
2026-09-26T10:24. The Orchestrator ran
`go test -timeout=30m ./...` once. It exited 1 after 2029 seconds. Every
package passed except
`tests/integration:TestDoclineSoftKeys_LiveTrackedCorpus/deterministic_path_report`,
which found this document missing the soft keys `chunk_strategy: h1-h2-h3`
and `schema_version: "1.0"`. This was a documentation-only failure; no code
failure was reported. `internal/core` passed in 791.8 seconds, as did the
other packages (`internal/mcp`, `internal/cli`, `tests`, and
`tests/contract`). The suite was not rerun.

The full-suite capture is
`logs/diagnostics/155-s-go-test-governed-30m-20260926-1024.txt` with metadata
in the adjacent `.meta.txt` file.

**Fix commit SHA:** Pending; a follow-up documentation commit will record the
SHA of the commit that adds these soft keys.

**Targeted re-verification:**

* `go test -count=1 -timeout=10m -run '^TestDoclineSoftKeys_' ./tests/integration`
  — PASS, exit 0; package time 13.287 seconds.
* `go test -count=1 -timeout=15m ./tests/integration` — PASS, exit 0; package
  time 15.888 seconds.
* `.\bin\backlogit.exe docs lint --path docs/closure/2026-09-25-155-S-final-review.md`
  — PASS, `valid: true`, zero violations.

The targeted outputs and command metadata are captured in
`logs/diagnostics/155-s-docline-softkeys-targeted-20260926.txt`,
`logs/diagnostics/155-s-integration-targeted-20260926.txt`, and
`logs/diagnostics/155-s-closure-docs-lint-fix-20260926.txt` with corresponding
`.meta.txt` files.
