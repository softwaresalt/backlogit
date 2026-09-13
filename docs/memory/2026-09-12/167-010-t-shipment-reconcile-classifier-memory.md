---
title: 167.010-T shipment reconcile classifier memory
doc_type: memory
created_at: 2026-09-12T13:58:00Z
---

## Task

* Implement task 167.010-T on branch `feat/148-s-governed-archived-shipment-reconciliation-to-shipped`
* Leave changes uncommitted for operator review

## Files changed

* `internal/core/shipment_reconcile.go`
* `internal/core/shipment_reconcile_classifier.go`
* `internal/core/shipment_reconcile_classifier_test.go`

## Decisions

* Branch 0 unreadable or path-unsafe log remains caller-owned because the classifier only accepts already-read `[]byte`; nil or blank log is treated as readable-and-empty, not unreadable
* Persisted request markers are read from `custom_fields.shipment_reconciliation_idempotency_key` plus top-level `shipment_reconciliation_request_identity_digest`
* Prepared resume state is read from top-level `shipment_reconciliation_prepared_event` and `shipment_reconciliation_event_digest`
* Branch 2b maps to external outcome `no_op`, matching the existing `ShipmentReconcileOutcomeNoOp` branch comment in `shipment_reconcile.go`; the eventual transaction must re-derive 1b vs 2b from the same persisted inputs when it needs append-only resume behavior
* Invalid or torn resume state returns `indeterminate` with `ErrValidation`; request mismatches and unsupported durable history return `conflict` with nil error

## Validation

* RED observed before implementation: `TestClassifyShipmentReconcileState_Totality` failed because `classifyShipmentReconcileState` panicked with `not implemented: classifyShipmentReconcileState (167.010-T)`
* GREEN: targeted classifier test passed
* GREEN: `go build ./...`
* GREEN: `go vet ./...`
* GREEN: `golangci-lint run ./internal/core/...` with v1.64.8
* GREEN for changed files only: `gofmt -l internal/core/shipment_reconcile.go internal/core/shipment_reconcile_classifier.go internal/core/shipment_reconcile_classifier_test.go`
* BLOCKED on unrelated suite noise: repeated `go test ./internal/core/... -count=1` runs failed in pre-existing flaky test `TestShipShipment_UnrestorableItemReportsPartialCompensation`; a focused rerun of that test passed in isolation

## Follow-up note for 167.008-T

* The transaction must re-derive the richer internal distinction between 1b durable-no-op and 2b append-only resume from persisted state, because the fixed public classifier signature returns only the external four-value outcome enum
