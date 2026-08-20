---
title: "128-S / 144-F — Runtime Verification and Operational Closure"
doc_type: closure
schema_version: "1.0"
chunk_strategy: h1-h2-h3
ingested_at: "2026-08-20T04:15:00Z"
source: docs/closure/2026-08-19/128-s-144-f-runtime-verification-and-closure.md
---

## Summary

Feature 144-F ("shipped-transition prevention hardening") implements two
runtime guards preventing shipment lifecycle corruption:

* **Guard 1** — rejects any attempt to move a shipment artifact to `shipped`
  status through the generic artifact-update path. Only `ShipShipment` may
  perform this transition.
* **Guard 2** — rejects archiving a shipment whose status is `shipped` unless
  a durable `shipped` event is present in its JSONL item log, protecting
  against archive stamping without a proper release record.

PR #370 implemented and tested both guards across `internal/core`,
`internal/mcp`, and `internal/cli`. This record documents the merge,
post-merge backlog reconciliation, and targeted runtime verification of the
prevention behavior.

## Merge

* PR: [#370](https://github.com/softwaresalt/backlogit/pull/370)
* Reviewed HEAD (implementation): `db1300f80c5f03636cbb8b6ac8ca81fdc87115cb`
* Merge commit: `461b670c3602ce54fa5e24635f4e2abc50c2b36c`
* Merge strategy: merge commit (P-009 compliant; squash/rebase disabled at
  repo level: `allow_squash_merge=false`, `allow_rebase_merge=false`,
  `allow_merge_commit=true`)
* Pre-merge gate (re-verified at merge time, not relying on stale
  `docs/memory/2026-08-19/ship-128s-pr370-readiness.md`):
  - PR HEAD == db1300f8 ✓
  - CI: 6/6 checks `success` (CLI Reference Drift, Docline frontmatter gate,
    test, copilot-pull-request-reviewer, Detect code changes, Markdown lint) ✓
  - Latest Copilot review (2026-08-19T21:24:07Z / repo-local
    2026-08-19T14:24:07-07:00) covers commit db1300f8 (current HEAD at merge
    time) ✓
  - No pending Copilot review requests ✓
  - 8/8 review threads resolved, pagination complete (`hasNextPage: false`) ✓
  - `mergeStateStatus: CLEAN`, `mergeable: MERGEABLE` ✓
  - Single implementation worktree confirmed:
    `.worktrees/stage-47b48db0` on `feat/144-shipped-transition-prevention`
    (P-016 topology safe) ✓

## Post-Merge Workspace Topology

* Root worktree (`D:\Source\GitHub\backlogit`) remained on
  `post-merge/127-s` with its pre-existing carried/dirty state untouched, per
  instruction not to absorb or disturb unrelated root changes.
* A dedicated closure worktree was created at `.worktrees/128-s-closure` on
  new branch `post-merge/128-s`, checked out from `origin/main` at the merge
  commit (461b670c), per the "fresh-binary post-merge lifecycle" protocol.
* A fresh `backlogit` CLI binary (`backlogit-closure.exe`, gitignored) was
  built from this merged HEAD via `go build ./cmd/backlogit` and used
  exclusively for all backlog lifecycle mutations below.

## Shipment Reconciliation (shipment-reconcile skill)

* Pre-mode report: `.backlogit/reconcile/128-S-pre-20260819-204955.md`
  — 12/12 manifest items `matched`/`pre-archived` at `expected_status: done`,
  zero orphans, recommendation `PROCEED`.
* Backlog state transitions applied via the fresh CLI (`move`, then
  `shipment ship`):
  - 144.001-T: already `done`/archived from prior implementation work
    (unchanged).
  - 144.002-T through 144.005-T: `active` → `done` (direct transition
    permitted).
  - 144.006-T through 144.011-T: `queued` → `active` → `done` (direct
    `queued`→`done` is rejected by the `validate_status_transition` pre-hook;
    required the intermediate `active` step).
  - 144-F: `active` → `done`.
* `backlogit shipment ship 128-S --sha 461b670c3602ce54fa5e24635f4e2abc50c2b36c
  --message "Merge pull request #370 ..." --author "softwaresalt"` executed
  successfully:
  - `shipment_status: shipped`
  - `archived_ids`: all 13 items (144.001-T…144.011-T, 128-S, 144-F)
  - `returned_ids`: `[]`
  - `commit_sha` recorded: `461b670c3602ce54fa5e24635f4e2abc50c2b36c`
* Post-mode report: `.backlogit/reconcile/128-S-post-20260819-211430.md`
  — ship result classified as a normal, fully-applied archival (not
  `mutation_partial`/`indeterminate`); all 13 archive files present; no
  deletions detected under `.backlogit/archive/` (P-007 guard clean); no
  restore required. Recommendation: `PROCEED`.
* Final state (verified via `backlogit query` against a re-synced index):
  128-S, 144-F, and 144.001-T through 144.011-T are all `status: archived`.
  No active/queued residue remains for this release scope.
* No physical file-lock was acquired: the `file-lock` skill's
  `acquire_lock.ps1`/`.sh` scripts are referenced by its `SKILL.md` but are
  not present under `.github/skills/file-lock/scripts/` in this checkout.
  Per `concurrency.instructions.md`, per-file locking is required only under
  concurrent-access conditions; this was a single-agent, single-worktree
  closure session, so proceeding without a physical lock was policy-compliant.
  Noted here for traceability.

## Full Test Suite (informational)

`go test ./...` was run once in the closure worktree at merged HEAD. All
packages passed except a single panic in `internal/core` during
`TestShipmentGate_NoHeadDriftBeforePersist_ShipsCleanly` after ~616s of
suite runtime (Windows `os.OpenFile` call inside the hook-event writer).
Re-running that test in isolation (`go test ./internal/core/... -run
TestShipmentGate_NoHeadDriftBeforePersist_ShipsCleanly`) passed cleanly in
7.1s. This is treated as an environment-specific flake (resource/file-handle
contention from the very large parallel suite on Windows), not a merge
regression, because:

1. CI already validated this exact test at the reviewed HEAD (db1300f8) as
   part of the green `test` check before merge.
2. The merge commit introduces no new diff beyond what CI validated
   (clean, fast-forwardable merge).
3. The isolated re-run reproduced a clean pass with no code changes.

No remediation action taken; recorded here as an observed flake for future
reference, not a residual risk requiring backlog follow-up.

## Targeted Runtime Verification of Prevention Behavior

Performed in an isolated scratch `backlogit` workspace (`backlogit init`
under a throwaway `runtime-verify-scratch/` directory, removed after
verification) using the same fresh closure-worktree binary — production
backlog state was never touched by these checks.

### Guard 1 — generic shipped-transition rejection

* Created a test shipment (`001-S`), moved `queued` → `active`.
* Attempted `backlogit move 001-S --status shipped` (the generic path).
* **Result**: rejected, exit code 9:
  `move shipment 001-S to shipped via generic path: backlogit: shipment must
  be shipped via ShipShipment, not a direct status update`
* Confirms `gate_transition.go`'s unconditional refusal
  (`bkerrors.ErrShipmentShippedRequiresEnvelope`) is active in the merged
  binary.

### Guard 2 — archive-stamping-without-event rejection

* Created a second test shipment (`002-S`, `queued`).
* Directly archiving a non-`shipped` shipment succeeded normally (guard is
  scoped to `status: shipped` only) — confirms the guard does not
  over-block ordinary archival of non-released artifacts.
* Manually edited `002-S.md` frontmatter to `status: shipped` (simulating a
  bypassed/corrupted state with no durable shipped event in the item log).
* Attempted `backlogit archive 002-S`.
* **Result**: rejected, exit code 9:
  `archive shipment 002-S: backlogit: archiving a shipped shipment requires a
  durable shipped event in the item log`
* Confirms `archive.go`'s `archiveShippedEventPreflight`
  (`bkerrors.ErrArchiveShippedRequiresEvent`) is active in the merged binary.

### Legitimate path confirmation

The real 128-S shipment above was shipped and archived successfully via
`backlogit shipment ship`, which records the shipped event before archival —
demonstrating Guard 2 does not block the legitimate, fully-governed path.

## Residual Risks (from PR readiness report, carried into closure)

* MCP parity tests (`internal/mcp/144_prevention_parity_test.go`) exercise
  direct handler calls rather than a full MCP protocol round-trip.
* Some CLI tests (`internal/cli/144_prevention_cli_test.go`) validate the
  error-mapper layer directly rather than executing every affected Cobra
  command end-to-end with process exit-code assertions.
* Both are P2, pre-existing test-design choices scoped to this feature, not
  functional defects. No durable follow-up stash entry was found during PR
  readiness review, and none is created here: the residual risk is narrow
  (test-depth choice, not a behavior gap — the live runtime verification
  above independently exercises the actual guard behavior through the CLI
  process boundary, exit code 9 included), and does not block closure.

## Closure Decision

**A separate closure PR is required**, matching the established repository
pattern: every prior shipment's post-merge closure (127-S/#368, 125-S/#356,
124-S/#352, 123-S/#349, 122-S/#344, 121-S/#346, 120-S/#341, 119-S/#339,
118-S/#336, 117-S/#334, 116-S/#331, 113-S/#322, 110-S/#313, 108-S/#306,
107-S/#304, 106-S/#301, 103-S/#291, 102-S/#288, 092-S/#236) was submitted
and merged as its own dedicated `chore(ops)`/`chore(harness)`/`chore(docs)`
PR from a `post-merge/*` or `closure/*` branch, even though the diff is
confined to `.backlogit/` archival state plus closure/memory docs. Shipment
128-S follows the same protocol: branch `post-merge/128-s` is pushed and a
PR opened against `main`, Copilot review requested and polled, and the
§1.9 pre-merge readiness gate satisfied before presenting for merge. Per
P-014, this closure PR's merge requires its own explicit operator approval
— the approval already granted for implementation PR #370 does not carry
over.

**Final state (pending closure PR merge)**: shipment 128-S —
`SHIPPED`/`ARCHIVED`. Feature 144-F and tasks 144.001-T through 144.011-T —
`ARCHIVED`. No active or queued items remain for this release scope. This
state is durable in the `post-merge/128-s` branch/PR pending merge.

## Closure PR Status (final, awaiting operator approval)

* **Closure PR**: [#371](https://github.com/softwaresalt/backlogit/pull/371)
  — "chore(harness): post-merge closure for shipment 128-S / feature 144-F"
* **Branch**: `post-merge/128-s`
* **Reviewed HEAD**: `0ba2fa9cd113d43c9085a841151b8dac8e971281`
* **CI**: 5/5 checks `success` (test, CLI Reference Drift, Detect code
  changes, Docline frontmatter gate, Markdown lint)
* **Copilot review**: covers HEAD `0ba2fa9c` (submitted
  2026-08-20T05:17:37Z); no pending review requests
* **Review threads**: 3/3 resolved (all were the docline
  `chunk_strategy` frontmatter finding raised against an earlier commit
  `4b2cbaf3`, fixed in `0ba2fa9c`, replied to, then resolved; the fresh
  review at `0ba2fa9c` raised zero new findings)
* **Mergeable**: `MERGEABLE`, `mergeStateStatus: CLEAN`
* **Merge strategy**: repo-level `allow_merge_commit=true`,
  `allow_squash_merge=false`, `allow_rebase_merge=false` (unchanged from
  implementation PR #370's verification)
* **§1.9 gate**: PASS (all 3 checks — no pending review, review covers
  current HEAD, zero unresolved threads)

**STOP — P-014 gate.** This closure PR is fully ready for merge but was
**not merged** in this session. The operator's approval `PR 370: Merge
approved` (2026-08-19T20:10:01-07:00) was explicitly scoped to
implementation PR #370 only and does not extend to this closure PR. Merging
PR #371 requires a new, explicit operator approval naming PR #371 (e.g.
`PR 371: Merge approved`).
