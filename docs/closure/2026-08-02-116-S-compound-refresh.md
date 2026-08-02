---
title: "116-S Compound Refresh — Shipment Sequencing Primitives"
source: docs/closure/2026-08-02-116-S-compound-refresh.md
doc_type: closure
description: "Compound-refresh pass for PR #330 / shipment 116-S — assesses compound library entries for drift, reinforcement, or new learning candidates"
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-08-02T06:32:00Z
    severity: low
    tags:
        - compound-refresh
        - shipment
        - priority
        - blocking
        - cli
        - mcp
        - 116-S
---

# 116-S Compound Refresh Report

Scope: recent compound entries potentially affected by PR #330 (shipment sequencing primitives — priority and blocking order).  
Mode: propose (entries assessed; no files edited in this pass; one new learning created).

## Entries Reviewed

### `2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`

**Classification: keep**  
Evidence: PR #330 applies the same denylist pattern for `create_shipment` params (in `internal/cli/shipment_test.go`: `TestShipmentCreateCLIMCPParityLock`). The denylist pattern is now used for two separate CLI commands. The existing entry accurately describes the approach; no update needed. Pattern reinforced.

### `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`

**Classification: keep**  
Evidence: `CreateShipment` with variadic opts still commits to disk (Markdown) before returning — consistent with the durable-writes contract. No drift introduced.

### `2026-07-21-omitempty-defeats-arrays-always-json-contract.md`

**Classification: keep**  
Evidence: PR #330 does not change JSON serialization of the `items` field in shipments. No drift.

### `2026-07-06-ancestor-aware-shipment-gate-staleness.md`

**Classification: keep**  
Evidence: `AddShipmentBlock` delegates to existing `AddDependency` which does not touch the gate/ancestor logic. No drift.

## New Learning Warranted

A new compound entry should be created for the variadic-options pattern used in `CreateShipment`. This pattern allows backward-compatible extension of core factory functions without changing callers.

**File created**: `docs/compound/2026-08-02-variadic-options-backward-compatible-shipment-creation.md`

## Summary

| Entries reviewed | 4 |
|---|---|
| keep | 4 |
| update | 0 |
| consolidate | 0 |
| replace | 0 |
| delete | 0 |
| New learning created | 1 |

No existing compound entries require modification. One new institutional learning added to capture the variadic-options pattern for future use.
