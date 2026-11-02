---
title: "Dark Factory Mode Activation Record — Shipment 211-S (210-F)"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/orchestrator-211-s-dark-mode-activation.md
description: "Authoritative P-017 DARK_MODE_ACTIVE activation record, operator merge pre-authorization, and gate waivers for Shipment 211-S (Feature 210-F)."
chunk_strategy: h1-h2-h3
shipment_id: 211-S
feature_id: 210-F
---

## P-017 Dark Factory Activation Contract (`DARK_MODE_ACTIVE`)

* `DARK_MODE_ACTIVE`: `true`
* `activated_at_utc`: `2026-11-02T16:41:55Z`
* `resumed_at_utc`: `2026-11-02T18:41:46Z`
* `scope`: `["211-S"]`
* `covering_feature`: `210-F` (*Backlogit Workflow Custody, Pre-Existing Change Adoption, and Stage-to-Ship Handoff (1D48B514)*)
* `tasks`: `["210.001-T", "210.002-T", "210.003-T", "210.004-T", "210.005-T", "210.006-T", "210.007-T"]`
* `ordered_shipment_sequence`: `["211-S"]`
* `last_completed_shipment`: `"210-S"`
* `next_shipment_to_claim`: `"211-S"`
* `merge_approval_pre_authorized`: `true` (applies to both the `feat/211-s-workflow-custody-handoff-210-f` implementation PR and the `post-merge/211-s-closure` closure PR, subject to §1.9 5/5 PASS, P-018 `autoharness gate copilot-review` `SATISFIED: PASS`, all CI checks green, and `--merge --match-head-commit`)
* `admin_fallback_pre_authorized`: `false`

## Operator-Authorized Waivers & Decisions (`211-S`)

1. **Prune-on-Restore Waiver**: If `agent-engram` is unreachable or times out during checkpoint recovery (`checkpoint-20261102-222209.json`, `checkpoint-20261102-204757.json`), skip the engram prune-on-restore sweep, retain the active cursor and checkpoint pointer, record the waiver in the checkpoint decisions, and proceed with restore + resolve + execution (matching the `210-S` closure precedent in `docs/memory/2026-11-02/ship-210-s-post-merge-closure.md`).
2. **Windows Local Full-Suite Waiver (`Option 2`, Stash `F448AA52`)**: Because pre-existing `internal/core` tests (`TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation`, `TestReconcileResumeForgedPreparedEvent`, `TestUR1B_*`) flake or stall under Windows full-package `-race` runs (captured in P-021 C2 stash `F448AA52`), local wave and pre-PR gates are authorized to run full-repo compile verification (`go test -timeout=30m -run=^$ -count=1 ./...`), `go build ./cmd/backlogit`, `go vet ./...`, and scoped `-race` selectors covering all `211-S` tests (`TestU1_`, `TestU2_WorkflowCustody`, `TestU3_`, `TestU4_`, `TestU5_`, `TestU6_`, `TestU7_`, `TestRegistryParity*`), while relying on GitHub Actions CI on the pushed PR HEAD for the full CI suite (matching the `210-S` PR #638 precedent). Record `F448AA52` in `Follow-ups:` (`Outcome: READY_WITH_FOLLOWUPS`).
3. **Go 1.27 / `golangci-lint` Degraded Tool Waiver**: While `golangci-lint v1.64.8` (built with `go1.26.5`) cannot read `go1.27.0` export data locally, `go vet ./...` and `gofmt` are accepted as the local static-analysis/format substitute (`TOOL_DEGRADED: golangci-lint`), recorded in the readiness/closure notes.
4. **Closure Protocol**: Post-merge closure uses the `shipment-reconcile` safe-close flow (`mode: pre` -> governed `safe_close` / `backlogit_ship_shipment` -> `mode: post`).
5. **Wave 4 (`210.004-T`) Option B Contract Amendment & Scope Decision (`APPROVED`)**:
   * **Option B Authorized for `internal/core/custody_exec_test.go` (`78c8e937`)**: In the RED harness (`7f5fe59a`), line 242 called `custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))` immediately after line 238 rolled back and removed `workflow_custody` from that shipment (causing a self-contradictory fixture failure), and the AST lock-order helper check in `78c8e937` verifies the lock chain inside the single lock-acquiring function called by `OfferCustodyHandoff` and `AcceptCustodyHandoff`. Both test adjustments in `78c8e937` are explicitly authorized under P-004 (`Option B`).
   * **Committed-Journal Recovery & Nonce-Provider Scope**:
     1. Add `internal/core/shipment.go` to `210.004-T`'s file scope so `recoverPendingShipmentOperations` (`internal/core/shipment.go:2752-2759`) includes `isCustodyOperation(journal.Operation)` (`custody_offer` / `custody_accept`) when finalizing `phase == "committed"` journals via `removeShipmentOperationJournal(ctx, ws, record.path)`.
     2. Keep the `WithCustodyNonceConsumer` / `NonceConsumeProvider` wiring in `internal/core/custody_exec.go` (and allow `Workspace` or `CustodyAcceptOptions` to supply a `NonceConsumeProvider` for MCP/CLI callers in `U6`/`U7`), failing closed with `ErrNonceProviderUnavailable` when no consumer/provider is configured.
     3. Ensure `UpdateArtifactWithGate` and `writeArtifactFileGoverned` in `internal/core/artifacts.go` both refuse ungoverned `custom_fields["workflow_custody"]` mutations, pass `--no-replace-objects` to the default-branch git lookup in `custody_exec.go`, and include a Windows `case_only_rename` verification check in `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery`.
     4. For Waves 5 and 6 (`210.005-T`, `210.006-T`, `210.007-T`), any self-contradictory test fixture bug in a newly scaffolded RED test file may be repaired directly as part of that wave without halting to the Orchestrator, provided every behavioral subtest in the task contract is genuinely exercised and passes.
