---
closure_status: BLOCKED
compaction_status: degraded
description: "199-S (CY-A: harden lifecycle errors and blocked-envelope boundaries) post-merge closure for PR #498, merge commit 1f25d10daed74ce209a1f14ec7160041bb0a4b6e. Closure is blocked on the 192-F close-evidence record, which cannot be produced post-hoc."
doc_type: closure
docline:
  date: 2026-10-15T19:40:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 199-S
    - 199-F
feature_id: 199-F
shipment_id: 199-S
merge_commit: 1f25d10daed74ce209a1f14ec7160041bb0a4b6e
schema_version: "1.0"
source: docs/closure/199-S-199-F-post-merge-closure.md
title: "199-S post-merge closure"
conditions:
  - id: pr-498-merged
    description: "PR #498 is merged into main as a merge commit (P-009)."
    satisfied: true
    evidence: "gh pr view 498: state MERGED, mergedAt 2026-10-15T18:42:00Z, mergeCommit 1f25d10daed74ce209a1f14ec7160041bb0a4b6e; origin/main equals the merge commit; local main fast-forwarded to it with carry-forward state unchanged."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the merged head with zero unresolved threads, and every required check passed on that head."
    satisfied: true
    evidence: "copilot-review gate SATISFIED at e7961c8d (rounds 13, unresolved_thread_ids empty, COMMENTED review by copilot-pull-request-reviewer[bot] with zero inline threads); last-mile re-check SATISFIED before merge; checks test, Windows handle/lock tests, Docline frontmatter gate, CLI Reference Drift, Detect code changes, Markdown lint, pipeline-topology all pass."
  - id: governed-closure-archived
    description: "Shipment 199-S, covering feature 199-F, and the nine explicit task members are archived with merge provenance, and the shipment control record carries archived_status shipped."
    satisfied: true
    evidence: "archive/199-S.md archived_status shipped; archive/199-F.md and archive/199.001-T.md through archive/199.009-T.md status archived with commit 1f25d10daed74ce209a1f14ec7160041bb0a4b6e (corrected from a mistyped SHA by backlogit_track_commit, 11 of 11); item log shipped event present (shipment_status_changed to shipped); doctor shipped-event-completeness reports no missing shipped event for 199-S."
  - id: close-evidence-record
    description: "A 192-F close-evidence record docs/closure/evidence/199-S-199-F-close-evidence.json written by a governed cascade-close or safe-close run, selected close path recorded, and the closure-evidence gate passes."
    satisfied: false
    evidence: "No record exists. The governed close ran through the backlogit_ship_shipment path without the classify-only pre-close step required by the staged Ship protocol. autoharness shipment cascade-close --classify-only refuses post-hoc (shipment '199-S' is no longer open; no verdict record after the fact). The record is not backfilled or fabricated (staged operational-closure rule for post-192-F artifacts). Requires a Stage or operator decision."
  - id: compaction-p020
    description: "compact-context has run for this closure (P-020) and compaction_status is finalized to done or degraded."
    satisfied: true
    evidence: "compact-context ran as a bounded batch on 2026-10-15 (P-020). compaction_status is degraded: 35 originals archived, 17 skipped for queued or active references, closure and plan targets not assessed. See the Compaction section."
---

# 199-S (CY-A: harden lifecycle errors and blocked-envelope boundaries) post-merge closure

Status: BLOCKED. Every shipment-side condition is satisfied. The close-evidence record required for the 192-F closure-evidence gate cannot be produced for this shipment, and compact-context ran as a bounded batch (degraded; see Compaction).

## Summary of the shipped change

Nine tasks under covering feature 199-F, executed in three P-002.6 waves, all done and archived:

* Error classification: `domainError` maps `ErrWriteIndeterminate` to `write_indeterminate` ahead of the sibling sentinels. Compensation step and commit errors wrap with `%w`; the pre-compensation cause stays `%v`.
* Non-durable append (`internal/events/stream.go`): pre-write mkdir and open failures are `ErrWriteNotApplied`; post-write, short-write, and close failures are `ErrWriteIndeterminate`.
* BlockShipment accounting: `mutationApplied` is set before each member write; compensation restore failures in BlockShipment and UnblockShipment are `ErrWriteIndeterminate`.
* Blocked-envelope boundaries: one canonical key list; `guardBlockedShipmentArtifactTypeFlip` refuses ungoverned artifact_type flips into or out of `shipment` while blocked.

## Merge and verification facts

* PR #498 merged with a merge commit (no admin). Reviewed code at e7961c8d; internal tree identical to the reviewed 54573eee.
* Unfiltered `go test -count=1 -timeout=30m ./...` at wave 3 convergence: exit 0, 40 packages ok, zero failing tests. `go vet ./...` and `golangci-lint run` exit 0; gofmt clean on 899 committed Go blobs; `go build ./cmd/backlogit` exit 0.
* Adversarial multi-model review (report-only, HEAD-state) and the review skill (diff-level, READY_WITH_FOLLOWUPS, P0 0, P1 0).

## Incidents and corrections (disclosed)

1. Governed ship timeout: `backlogit_ship_shipment` returned a client timeout. The server-side transaction completed (shipped event at 11:45:51 local, archive completed later). The CLI fallback failed on the global shipment-lifecycle lock before writing anything.
2. Mistyped merge SHA: the governed ship and the first provenance writes used a 40-character SHA that is one character off (`1f25d10dae74...`). The correct SHA is `1f25d10daed74...`. All eleven archive records and the 199-S item-log commit event carried the wrong SHA. Corrected through `backlogit_track_commit` (eleven of eleven archive records now show the correct SHA; the wrong-SHA events remain in the append-only item logs with correcting events after them).
3. Shipment lock: the closure lock was acquired for `.backlogit/queue/199-S.md`. Its token was only partly echoed to the transcript (9 characters), so the lock was released with the operator-only force path after verifying no other process held the shipment file. Recorded in the post-mode reconcile report.

## Closure-evidence gate result

`autoharness gate closure-evidence --path docs/closure/199-S-199-F-post-merge-closure.md` fails. The artifact cannot declare a valid `close_path` and `close_evidence` pair because no 192-F record exists for this close. This is a process gap, not a fabricated pass.

## Decisions required (operator or Stage)

* Option A (preferred): accept this shipment's close as a documented exception to the 192-F evidence requirement, with the artifact status recorded as READY_WITH_CONDITIONS and the evidence condition named explicitly. This needs an explicit operator decision and a Stage-owned contract note; it must not be a backfilled record.
* Option B: keep the closure BLOCKED. The P-001 release closure stays open for this release unit until the evidence contract is decided. The next shipment's routing treats this closure as incomplete.
* In both options, the staged Ship protocol's classify-only pre-close step should be made mandatory in the live protocol before any future governed close, so the record exists before the ship.

## Residual risks (carried from PR #498)

* AdoptItem re-ID of blocked or shipped shipment artifacts (pre-existing; escalated P1 by the adversarial review; captured `34C23BE7`).
* New-path bypass of the artifact_type guard (captured `1D41F1C3`; no current caller).
* Mutation-applied restore trade-off on the first member failure (captured `8CB19B1F` for the missing test).

## Follow-ups (captured for Stage triage, P-021 C2)

`CC50FA47`, `8CB19B1F`, `4275BA19`, `843DE45B`, `AB174524`, `E411531A`, `BB80F0B2`, `4E7E1095`, `C04A0292`, `792C391E`, `19A6E9BB`, `34C23BE7`, `1D41F1C3`, `4399BF41`, `0C9FC425`, `FA754084`, `3B0B54A8`, `946B8775`, `D30D02CC`, `E29CFB5D`, `406AA1E0`, `14BF35E3`, `8624E9CB`, `1AC0EDBA`, `2CC696FE`, `85923985`, `DFD58CB2`, `54EEF1AA`. These are the follow-ups for Stage. No duplicate stash entries were created at closure.

## Runtime verification

Not required: `runtime_validation.validation_expectations.required` is false and `validator_manifest.surfaces` is empty in the workspace profile.

## Releasability

`READY` is not claimed. Releasability is BLOCKED on the close-evidence condition above. Monitoring plan, rollback procedure, and owner are not required by the profile (`releasability.required_evidence` marks them optional).

## Owner

Operator (dispatch owner) for the close-evidence decision. Ship for the remaining closure work once the decision is recorded.

## Compaction (P-020)

compact-context ran as a bounded batch on 2026-10-15, not a full target-all pass. Result: `compaction_status: degraded`. The batch moved 35 memory originals dated 2026-08-20 through 2026-09-30 to `docs/archive/memory/<date>/` by rename, with no deletions. It wrote one summary to `docs/memory/compacted/2026-10-15-memory-2026-08-20-to-2026-09-30-compacted.md`, which has a traceability table. It skipped 17 files that reference queued or active items (149-S, 073-DL, 141-S, 157-S, 182-F, 184-F, 156-S, 155-S, 168.001-T, 072-DL). The closure-record and plan targets were not assessed in this run. A full target-all pass remains a follow-up.

## Compound refresh

`docs/closure/2026-10-15-199-s-compound-refresh.md` records one in-place update to `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`. The update adds the non-durable append classification and compensation accounting from this shipment, and records the AdoptItem re-ID caveat. One related entry was kept unchanged.

## Closure PR hold

The closure PR is not yet open. P-001 keeps the release unit open until closure is complete. Opening and merging a closure PR while `closure_status` is `BLOCKED` would merge a closure artifact that the closure-evidence gate rejects. The branch `post-merge/199-f-cy-a-lifecycle-hardening` is pushed and ready to open once the operator records the close-evidence decision (Option A or Option B above).
