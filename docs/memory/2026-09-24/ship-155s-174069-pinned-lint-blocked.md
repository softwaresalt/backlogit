---
title: Ship 155-S 174.069-T Pinned Lint Blocked
date: 2026-09-24
status: blocked
shipment: 155-S
task: 174.069-T
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 19428607862d224a5abcb44463b0402efcd9b181
---

## Outcome

The authorized same-contract helper remediation restored `injectBroker`
compatibility with existing specialized fake runners while retaining
pre-assignment rejection of the production exec-backed runners.

The exact inventory selector and all targeted ShipShipment, gate, durability,
archive, race, compile, and vet gates passed. The CI-pinned Go 1.24
`golangci-lint` gate then failed, so validation stopped without retrying lint
or running build and formatting gates.

## Task changes

* `internal/core/gate_transition_test.go`
* `internal/core/shipment_test.go`
* `internal/core/025_archive_harness_test.go`

The helper remediation changed only
`internal/core/gate_transition_test.go`. No production files changed.

## Passing evidence

The following bounded gates passed:

* `TestShipShipmentFixtureGateBrokerInventory`
* Ordinary shipment fixture callers, including the previously timing-out
  non-member feature restoration test
* Shipment gate, formal-gate, manifest, and completion-CAS selectors
* Durability and external archive ShipShipment selectors
* The bounded race selector
* `go test -run=^$ -count=1 ./internal/core`
* `go vet ./internal/core`

## Blocking evidence

The repository-supported pinned validation used Go `1.24.0` and
`golangci-lint v1.64.8` with:

```text
golangci-lint run --timeout=5m --new-from-rev 37a5cba4713953c855013fafd4fc05e479e5618c ./internal/core/...
```

It exited `1` with:

```text
internal\core\shipment_shipped_event_durability_test.go:359:29:
Error return value is not checked (errcheck)
    requireShippedAppendPartial(t, err)
```

That file is unchanged by `174.069-T`. The finding belongs to earlier
durability corrective work and is outside the active task's reviewed
test-fixture contract.

## Current state

* `174.069-T` remains active and uncommitted
* `155-S` remains active
* `154-S` remains queued and unmodified
* Current HEAD remains `19428607862d224a5abcb44463b0402efcd9b181`
* `go build ./cmd/backlogit`, scoped formatting, and `git diff --check` were
  not run after the lint failure
* `go test ./...` remains unauthorized and was not run

## Next action

Stage must disposition the out-of-scope pinned-lint finding before Ship can
finish `174.069-T`. After a governed correction, rerun the pinned lint gate
once, then execute the remaining build and scoped formatting gates.
