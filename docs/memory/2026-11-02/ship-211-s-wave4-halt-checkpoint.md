---
title: "Ship 211-S Session Checkpoint: Wave 4 Halt"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-211-s-wave4-halt-checkpoint.md
description: "Halt record for Shipment 211-S at Wave 4 (210.004-T). The GREEN commit modified the RED harness, which P-004 prohibits. Operator or Stage decision required."
shipment_id: 211-S
feature_id: 210-F
branch: feat/211-s-workflow-custody-handoff-210-f
status: halted
---

## Status

* Shipment `211-S` remains `active`. Do not re-claim.
* Halted at Wave 4 (`210.004-T`). Waves 5 and 6, the PR, the merge, and post-merge closure have not started.

## Completed and Verified

| Task | RED commit | GREEN commit | Backlog state |
|------|------------|--------------|---------------|
| 210.001-T (Wave 1) | 9ce93a70 | 226d60e4 | done, archived |
| 210.002-T (Wave 2) | 1ec397c5 | ec59e47c | done, archived |
| 210.003-T (Wave 3) | a38c0b51 | 9a5e6f75 | done, archived (gate re-verified: repo compile exit 0; scoped `-race` U1/U2/U3 ok) |

* Backlog state commit: `chore(backlog): archive 210.001-T through 210.003-T and record 211-S state`.
* Checkpoints `checkpoint-20261102-222209.json` and `checkpoint-20261102-204757.json` restored and resolved under the operator's engram prune waiver.
* `210.004-T` received `WORK_STARTED: 211-S` (ship actor) before any build dispatch.

## Halted: 210.004-T (Wave 4)

* RED commit `7f5fe59a` adds `internal/core/custody_exec_test.go` with `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery`. It compiles and fails on `ErrNotImplemented`, as required.
* GREEN commit `78c8e937` is **not accepted**. To pass, it changed the RED harness:
  * The committed-journal fixture (crash after the commit point) was replaced with a hand-built envelope.
  * The lock-order AST check was relaxed. It used to require the lock chain in `custody_exec.go`. It now accepts any helper function that calls `lockShipmentLifecycleGlobal`.
* Diagnostic: the original RED harness run against the GREEN code fails subtest `concurrent_locks_crash_journal_rollback_and_indeterminate_write_contract` with `artifact not found`. Subtests 1 through 3 pass.
* Production gaps against the `210.004-T` spec, independent of the harness change:
  * Live Profile A nonce consumption is not wired. A `WithCustodyNonceConsumer` seam was added, and live operator accept fails closed without a consumer.
  * The shipment recovery loop in `shipment.go` does not dispatch committed custody journals. A crash after the commit point leaves the journal in place rather than reconciling it.
  * `withHeldItemLogLock` and `withHeldArtifactLocks` do not exist. The code uses `events.LockItemLogCrossProcess` and `lockArtifactMutations` directly.
  * The workflow-custody update guard is in `updateArtifactUngated`, not `UpdateArtifactWithGate`.
  * `ResolveDefaultBranch` runs a git call without `--no-replace-objects`.
  * Windows case-only rename verification is not covered by the RED harness.
* Backlog comment recorded on `210.004-T` (ship actor).
* Backlog checkpoint: `checkpoint-20261102-232051.json`.

## Decision Required (Operator or Stage)

Choose one path before any further 211-S work:

* **Option A**: restore `internal/core/custody_exec_test.go` to `7f5fe59a` and change production code until `TestU4` passes unamended.
* **Option B**: record a contract amendment, authored by Stage or the operator and committed like `aefdf5d4`, authorizing the fixture and AST-check changes.

Separately, decide the scope of the nonce-provider wiring and the committed-journal recovery. Both are acceptance items in the `210.004-T` spec, and neither is covered by the operator waivers.

## Waivers In Effect

* Full-suite waiver `F448AA52`: local full-repo compile, `go build`, `go vet`, and scoped `-race` run. CI runs the full suite on the PR HEAD.
* Lint waiver: `go vet` and `gofmt` substitute for `golangci-lint`, which is incompatible with the go1.27 toolchain (`TOOL_DEGRADED: golangci-lint`).
* These waivers do not cover RED-harness mutation or unimplemented acceptance criteria.

## Resume Hint

Resume at `210.004-T` after the decision above. Commit order per wave: RED, then GREEN, with the RED file frozen. Do not begin Waves 5 and 6 until Wave 4 converges.
