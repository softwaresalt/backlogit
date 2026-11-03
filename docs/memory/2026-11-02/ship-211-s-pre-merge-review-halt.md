---
title: "Ship 211-S Session Checkpoint: Pre-Merge Review Halt"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-211-s-pre-merge-review-halt.md
description: "Halt record for Shipment 211-S after Waves 1-6 and the pre-merge review. Readiness is BLOCKED for HEAD bfcfcb4d. Operator security-design decisions are required before any PR or merge."
shipment_id: 211-S
feature_id: 210-F
branch: feat/211-s-workflow-custody-handoff-210-f
reviewed_head: bfcfcb4d
status: halted
---

## Status

* All seven member tasks (`210.001-T` through `210.007-T`) and covering feature `210-F` are `done` and archived. Shipment `211-S` remains `active`.
* Local gates at `bfcfcb4d`: `go build ./cmd/backlogit` exit 0, `go vet ./...` exit 0, `gofmt -l` clean.
* No PR is open. No merge has occurred. Closure docs are not written.

## Wave Record

| Task | RED | GREEN | Notes |
|------|-----|-------|-------|
| 210.001-T, 210.002-T | 9ce93a70, 1ec397c5 | 226d60e4, ec59e47c | done, archived |
| 210.003-T | a38c0b51 | 9a5e6f75 | done, archived |
| 210.004-T | 7f5fe59a (+ Option B amendment 4849fd14) | 3805d4ad | done, archived. Scope: committed-journal recovery, nonce seam, guard, Windows casing assertion (dd726e3c) |
| 210.005-T | 28b9279b | ac4ea44c | done, archived |
| 210.006-T | 35ccd6a7 (+ fixture repair 5b69eee3) | 697eb88f | done, archived |
| 210.007-T | 6403ae47 | ffe17e42 | done, archived. CLI parity, registry promoted, docs regenerated, docs lint 0 violations |

## Pre-Merge Review: BLOCKED

Two independent reviews ran read-only against `bfcfcb4d`: a code review and an adversarial review with three reviewers. Both returned BLOCKED.

Verified directly by the orchestrator:

* **Receipt expiry is never enforced (P1).** `ExpiresAtUTC` is set at `custody_exec.go:411` and never compared. Expired accepted receipts still verify.
* **Default-branch refusal fails open (P1).** `custodyRefuseDefaultBranch` (`custody_exec.go:750-756`) compares against `config.ResolveDefaultBranch`, which returns `origin/<name>` when `origin/HEAD` resolves. A repo whose default branch is `develop` is not protected. Only the hard-coded names `main` and `master` are refused.

Reported by reviewers, not yet independently verified:

* **Trust anchors come from workspace config (P0 candidate, R1-2).** Verify needs no nonce provider, so an agent with workspace write access could add its own anchor, sign a token, and produce an operator-verified receipt. `custody_validate.go:649`, call site `custody_exec.go` near 470.
* **Profile selection trusts caller-supplied fields (P1, F-01 / R1-1).** `custodyRequiredProfile` (`custody_exec.go:980`) decides on `contributor` and `source_kind`, both caller-declared. An agent can self-classify an operator-owned file as agent-handoff.
* **Verify does not require redemption (P1, R1-3).** Lifecycle fields are unsigned. A copied unconsumed proof can be flipped to accepted.
* **Approval does not bind `receipt_id` or accepting role (P1, R1-5).**
* **Nonce is consumed before validation and commit (P2, F-02 / R2-2 / R3-9).** Fail-closed, but a late failure burns the operator's token.
* **Stale intent journal can be replayed after a failed removal (P1, R2-1).** `custody_exec.go:666`.
* **Adopt write-back can overwrite canonical envelope from the SQLite projection (P1, R3-1).** `shipment_lifecycle.go:1446`.
* **Windows path aliases bypass the `.git` exclusion (P2, F-03 / R2-4).** Trailing-dot and 8.3 forms.
* **Ancestor path components are not no-follow (P2, F-04 / R2-3).**
* Lower severity: unbounded reads, lock release without `defer`, recovery lock order B-before-C, missing `ShipmentID` binding at accept, unenforced idempotency keys, registry `accept_custody` template missing `--auth-token`.

Disclosed deviations: (a) nonce provider not wired, so live Profile A accept fails closed; (b) rollback posture of any file-backed provider; (c) raw `DELETE FROM items` leaves orphan `custody_receipts` rows, with no decision reading them; (d) `config.runGit` lacks `--no-replace-objects`.

## Decisions Required From Operator

These are security-model choices. Ship will not invent them.

* **D1, trust-anchor provenance.** Where do operator trust anchors come from, such that a workspace writer cannot add one? Options include a host-supervisor-managed path outside the workspace, or a pinned fingerprint held outside the agent write set.
* **D2, Profile eligibility.** How is operator-owned content identified authoritatively? Options include a path-ownership policy in config that agents cannot write, or a default of requiring Profile A for any path not owned by the producer role.
* **D3, redemption record.** Verify must bind an authoritative redemption record covering nonce, receipt, accepting role, and acceptance time. Approve the record format and where it is stored.

## Mechanical Fixes Ready Once D1 to D3 Are Decided

No further design choice is needed for these. Each needs a RED test first:

* Enforce `ExpiresAtUTC` on verify and on accept.
* Normalize the `origin/` prefix in the default-branch check and add a `develop` case.
* Move nonce consumption after validation, and surface the token-burn on a post-consume failure.
* Bind `receipt_id` and accepting role into signed claims.
* Use a terminal phase, or fail on removal error, in the rollback path.
* Reject Windows aliases in path segments, and make ancestor components no-follow.
* Make the adopt write-back read the canonical envelope, not the SQLite projection.
* Lower severity: size cap, `defer` for lock release, `ShipmentID` binding, idempotency comparison, registry template parity.

## Merge Gate

`merge_approval_pre_authorized` is conditioned on a §1.9 PASS for the current HEAD. The review outcome is BLOCKED, so the pre-authorization does not apply. No PR is to be opened or merged until the review is re-run on the fixed HEAD and returns READY or READY_WITH_FOLLOWUPS.

## Resume Hint

Operator decides D1 to D3. Then Ship dispatches RED-then-GREEN fixes for the mechanical list above, re-runs both reviews on the new HEAD, and only then opens the PR. Do not re-claim 211-S.
