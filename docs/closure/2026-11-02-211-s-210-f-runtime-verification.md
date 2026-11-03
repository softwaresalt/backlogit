---
title: "211-S 210-F runtime verification"
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-11-02-211-s-210-f-runtime-verification.md
description: "Runtime and validator evidence for shipment 211-S (feature 210-F, workflow-custody/v1) at code HEAD ebdbb8d9, with blocked prerequisites and waived findings."
chunk_strategy: h1-h2-h3
shipment_id: 211-S
feature_id: 210-F
mode: runtime-verification
validation_status: PARTIAL
closure_status: READY_WITH_CONDITIONS
compaction_status: pending
---

## Scope

This document records the runtime and validator evidence for the native
workflow custody surface added by feature `210-F`: the `workflow-custody/v1`
envelope, the `preview`, `offer`, `accept`, and `verify` operations, the
derived `custody_receipts` SQLite projection, the `backlogit_custody_*` MCP
tools, and the `backlogit custody` CLI subcommands.

Code HEAD for the gates below is `ebdbb8d9`. Documentation commits made after
that point do not change code.

## Gate results at code HEAD

These results were observed in the Ship session that prepared this record.

* `go build ./cmd/backlogit`: exit 0.
* `go vet ./...` (repository-wide, as the operator required): exit 0.
* `go test -timeout=30m -run=^$ -count=1 ./...` (repository-wide compile): exit 0.
* `go test -timeout=30m -count=1 -race -run "TestRound2_|TestU1_|TestU2_WorkflowCustody|TestU3_|TestU4_|TestU5_|TestU6_|TestU7_|TestReview" ./internal/core/...`: `ok`. This includes the Option B docs test and the withdrawn R1 and R3 subtests, which are not run.
* `go test -timeout=30m -count=1 -race -run "TestRegistryParity" ./internal/cli/...`: `ok`.
* `go test -timeout=30m -count=1 -race -run "TestU7_|TestU6_" ./internal/cli/... ./internal/mcp/...`: `ok`.
* `go test -timeout=30m -count=1 ./internal/cli/...`: `ok`.
* `go test -timeout=30m -count=1 ./internal/mcp/...`: `ok`.
* `go test -timeout=30m -count=1 ./internal/db/...`: fails only `TestU175_075_LintFix`, `TestU175_076_LintFix`, `TestU175_077_LintFix`, and `TestU175_078_LintFix`. These are the golangci-lint incompatibility covered by the operator's lint waiver (see Blocked prerequisites). No other db test fails.

## Operator-surface probes

The probes below used the built binary and ran against the live workspace in
read-only mode. Neither command wrote any file. The working tree was checked
before and after.

* `backlogit custody --help` lists the `accept`, `offer`, `preview`, and `verify` subcommands and states that preview and verify are read-only.
* `backlogit custody verify 211-S --branch feat/211-s-workflow-custody-handoff-210-f --json` returned `verdicts: []` and the warning `no workflow custody envelope is recorded`. This is the correct result: no live offer has been made for this shipment, so there is nothing to verify.

These probes exercise the command surface and the fail-closed read path only.
The full offer-and-accept lifecycle is exercised by the package tests listed
above, not by a live handoff.

## Validator evidence

The envelope validator now refuses:

* more than one `accepted` receipt in one envelope (`ErrCustodyStaleRevision`, round 2 Option A, R2);
* an `accepted` receipt that is not the active receipt and whose revision does not exceed `custody_revision`;
* two `accepted` receipts that share a revision.

The write path enforces `custody_revision + 1` on accept (remediation F1,
`a67da9f5`). Approval-token claims are bound to the selected receipt's
candidate-set digest, allowed action, target revision, receipt ID, and
accepting role, each checked when both sides are set.

## Runtime surfaces not exercised live

* The MCP stdio server was not started in this session. The `internal/mcp` package tests exercise the four custody tools through `NewServer`, `NewServerForRoot`, `ListTools`, and `InvokeTool`.
* A live Profile A `accept` was not run. It fails closed by design when no nonce store or injected provider is configured (see Blocked prerequisites).

## Blocked prerequisites

* **golangci-lint.** `golangci-lint v1.64.8` cannot read go1.27 export data and cannot typecheck the repository under the installed toolchain. The operator waived lint. `go vet` and `gofmt` were used as the substitute. CI must run the real lint on the pull request head.
* **Operator approval channel and host key isolation (GSD OD-2).** Plan Section 10 makes this a mandatory owner or GSD bootstrap prerequisite that is not implemented in this repository. Until it exists, callers must never assume Profile A is available. Live Profile A accept fails closed with `ErrNonceProviderUnavailable` or `ErrApprovalChannelUnavailable` when neither an injected provider nor `BACKLOGIT_CUSTODY_NONCE_STORE_PATH` is configured. This is expected behaviour.
* **Production trust root.** When `BACKLOGIT_CUSTODY_OPERATOR_PINS_FILE` is unset, operator trust anchors come from workspace `config.yaml`. This is the waived finding D1 described in the pre-merge closure record.

## Residuals

* The envelope-level `CustodyRequestIdentityDigest` is retained because frozen fixtures call it. Profile A verification uses the per-receipt digest.
* `ActiveReceiptDigest` was removed. A search of this repository found no workspace data that contains the key. The custody decoder rejects unknown fields, so an envelope written by an earlier build of this unreleased branch that carries the key would fail to decode.
* The ancestor path check is a point-in-time check. A local writer that swaps an ancestor after the check and before the read is not closed. The window is documented in `custody_validate.go`.

## Compaction

`compaction_status` is `pending`. Post-merge closure runs `compact-context` under P-020 and updates this field.
