---
title: "155-S Final Review Halt Record"
description: "Standard and adversarial report-only review results for shipment 155-S at the governed-suite HEAD."
doc_type: closure
source: docs/closure/2026-09-25-155-S-final-review.md
---

# 155-S Final Review Halt Record

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
