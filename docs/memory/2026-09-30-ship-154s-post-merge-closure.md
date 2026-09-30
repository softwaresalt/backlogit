---
doc_type: memory
schema_version: "1.0"
source: docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md
title: "Ship 154-S post-merge closure checkpoint"
---

# Ship 154-S post-merge closure checkpoint

## Resumption and branch state

- Resumed the operator-selected `checkpoint-20260930-153707.json` after
  validating its Ship ownership and integrity. Engram was available. Resolved
  that checkpoint only after successful resume. `checkpoint-20260930-152943.json`
  and `checkpoint-20260930-080656.json` were already resolved; older
  `045011`, `044842`, `043858`, `042027`, and `030939` remain untouched.
- PR #466 is merged at `6d233d21162a072ddbdfecb52ec62a8fb8a63793`; the merge
  SHA is in `origin/main`. Local `main` was synchronized to that SHA before
  creating `post-merge/154-s-closure`.
- Current closure branch: `post-merge/154-s-closure`. No closure commit or
  closure PR exists yet. Operator approval is required before any merge.
- Preserve the operator-approved allowlist, backlogit-managed shipment/member
  archive and hook changes, and the closure outputs. No `.lock` file was
  touched.

## Shipment closure

- Stage resolved explicit shipment member `173-F` from `active` to `done`.
  Backlogit relocated it from queue to archive and recorded hook sequence
  `3352`; Ship did not perform the status move.
- Fresh reconciliation `.backlogit/reconcile/154-S-pre-20260930T154233Z.md`
  returned `PROCEED`; all eight explicit members were done and pre-archived.
- `backlogit_ship_shipment` was invoked with merge SHA
  `6d233d21162a072ddbdfecb52ec62a8fb8a63793`. Its MCP request timed out after
  shipment status/event persistence. Ship did not retry the governed mutation.
  After a later CLI workspace-open with recovery enabled, read-only
  reinspection found the shipment and all eight explicit members archived with
  merge traceability.
- Fresh post reconciliation `.backlogit/reconcile/154-S-post-20260930T155230Z.md`
  returned `CLOSED`. P-007 archive check passed; no archive deletions were
  restored. A doctor audit still reports 18 unrelated advisory legacy
  missing-shipped-event findings; none concern 154-S.

## Verification and operational evidence

- `go build ./cmd/backlogit` passed. CLI and MCP reads succeeded, and both
  active-shipment queries returned no active shipments.
- Runtime profile has no configured runtime-validator surfaces and marks
  validation optional. Local probes do not prove that the external scheduler
  consumed the 154-S marker. Do not route a successor until the external
  scheduler owner attests to marker consumption; existing stash `AF1E5075`
  tracks this condition.
- Relevant compound learning was reviewed and retained (`keep`) in
  `docs/closure/2026-09-30-154-s-compound-refresh.md`.
- Operational closure is `READY_WITH_CONDITIONS`; closure PR review/CI and
  external scheduler attestation remain outstanding.

## P-020 compaction

- Invoked `compact-context` with `target: all`; result is `degraded`.
- One completed 139-S memory record was compacted, with its original archived;
  507 bytes were recovered. The full target set was not processed. The
  154-S checkpoint was preserved because the closure PR is still pending.
- See `docs/closure/2026-09-30-154-s-compaction-report.md`. Do not describe
  compaction as complete.

## Residual risks and next steps

Existing residual stash entries remain untouched: `AD5AECAF`, `4DB1DFF1`,
`5A1C4D3F`, `D116AF58`, `67F17B6B`, `E52607F5`, `8B52A5F1`, and `AF1E5075`.
The incomplete compaction follow-up was captured as `A17EE897` through the
registry-declared CLI fallback after the MCP create-item surface rejected
`artifact_type: stash`; the entry was re-read and verified.

Next: finish changed-Markdown lint and relevant closure/reconcile tests; run
the required report-only local review; commit only approved paths with the
requested conventional message, push, open a closure PR with current-HEAD
readiness evidence, confirm required CI and the P-018 Copilot gate, then stop
for explicit operator merge approval.
