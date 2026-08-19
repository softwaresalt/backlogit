---
title: "PR #367 — shipped-event audit durability (127-S)"
---

## Summary

Implements feature 143-F (shipped-event audit durability) as part of shipment 127-S. Adds durability guarantees to the shipped-event append path so that failures are correctly classified (not-applied, indeterminate) and the doctor audit surface can detect and report residue after a partially-shipped shipment.

## Changes

### Core implementation (143.001-T – 143.012-T)
- **`internal/core/shipment_events.go`**: Adds `appendShipmentEventErr` — a shipment-scoped, error-returning event appender that locks the item log, classifies append failures as `ErrWriteNotApplied` or `ErrIndeterminate`, and refuses item IDs that resolve outside the workspace logs directory
- **`internal/core/shipment_lifecycle.go`**: Governs the shipped-event append during `ShipShipment`; classifies indeterminate failures and halts with compensation; corrects defer order so compensation fires before the indeterminate guard
- **`internal/core/doctor.go`**: Adds `CheckShippedEventCompleteness` to detect unarchived residue after shipment, enumerates feature descendants (not just manifest members), labels the enumeration as approximate
- MCP tool: exposes `check_shipped_event_completeness` on `backlogit_doctor` with recovery guidance keyed to the producer
- CLI flag: `--check-shipped-event-completeness` on `backlogit doctor`

### Review follow-ups (143.002-T, 143.007-T)
- Path containment check: refuses an item ID whose log resolves outside the workspace logs directory before any lock or write, tags it `ErrWriteNotApplied` so compensation is safe
- Feature descendant expansion: doctor residue finding now enumerates descendants under feature-typed manifest members, not just the member itself; labelled 'approximate'

### Tests
- RED-then-GREEN harness for governed shipped-event durability
- RED-then-GREEN harness for shipped-event reconciliation audit
- `TestAppendShipmentEventErr_RefusesUncontainedItemID`: pins path containment
- `TestDoctorShippedEventCompleteness_ResidueEnumeratesFeatureDescendants`: pins descendant expansion

### Backlog lifecycle
- Tasks 143.001-T through 143.012-T moved queue to archive (all done)
- Feature 143-F and shipment 127-S lifecycle state updated
- Pre-reconcile snapshot artifact added

## Quality Gates

| Gate | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `golangci-lint run ./internal/core/...` | PASS |
| `gofmt -l` (changed files) | PASS (pre-existing format drift in other packages not caused by this PR) |
| Scope tests (new + affected) | PASS |

## Scope

Bounded to shipment 127-S / feature 143-F. Stash item 47B48DB0 is explicitly excluded.

## Dark Mode

DARK_MODE_ACTIVE: true. Merge approval pre-authorized for this PR once all gates pass (CI, Copilot review, merge-commit-only). Admin fallback NOT authorized.
