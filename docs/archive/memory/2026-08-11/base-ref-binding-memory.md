---
type: session-memory
date: 2026-08-11
title: Base Ref Binding
---

## Outcome

Implemented formal-gate proof schema 2 with MAC-bound `base_ref` verification while preserving schema 1 compatibility.

## Files Changed

* `internal/gateproof/gateproof.go`
* `internal/gateevidence/formal.go`
* `internal/core/gate_evidence_formal.go`
* `internal/core/shipment_gate_manifest.go`
* `internal/core/shipment_gate.go`
* Focused formal-gate tests and design documentation

## Decisions

* Schema 1 remains valid without `base_ref`.
* Schema 2 requires a non-empty `base_ref` and a verifier-supplied matching value.
* No-repository evidence falls back to schema 1 because no resolved base ref exists.
* Shipment signing and self-verification receive the resolved base ref explicitly.

## Verification

* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `go build ./cmd/backlogit`
