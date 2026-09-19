---
chunk_strategy: h1-h2-h3
description: Durable shipment-scoped operator authorization permitting shipment 156-S to retain exact baseline findings owned by unfinished remediation tasks until terminal convergence at U12.
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-17-baseline-convergence-authorization.md
title: "Baseline Convergence Operator Authorization (shipment 156-S)"
docline:
    feature_id: 175-F
    shipment_id: 156-S
    decision_status: decided
---

# Baseline Convergence Operator Authorization (shipment 156-S)

## Scope of authorization

The operator authorizes a narrow, shipment-scoped procedural allowance for the
baseline-convergence release unit (feature `175-F`, shipment `156-S`):

- Shipment `156-S` MAY retain the exact golangci-lint v2.13.2 baseline findings
  that are owned by not-yet-completed finding-remediation tasks, until terminal
  convergence at U12 (`175.012-T`).
- No new or unowned findings may be introduced. The set of tolerated findings is
  exactly the 489-identity governed inventory at `docs/decisions/baseline-lint-inventory-489.json`
  (SHA-256 `7330d6baecc93e9cac4f4f96438a21565994106984f50c239cd1e6d8f86d2b66`); each identity is owned by exactly one remediation task.
- Terminal global lint is mandatory and MUST be zero-warning: the terminal task
  U12 runs `golangci-lint run` across the whole repository and the release unit
  is not converged until that run is clean.
- This authorization is bounded to shipment `156-S` only. It expires when `156-S`
  closes or when the operator revokes this session authorization.

## What this authorization does NOT grant

- It does **not** approve U1's (`175.001-T`) destructive worktree
  renormalization / refresh. That actual worktree mutation still requires
  immediate operator-only approval at the moment of execution during Ship.
- It does not authorize any relaxation of the four mandatory quality gates at
  terminal convergence, nor any tolerance for findings without a remediation
  owner.

## Enforcement binding

The intermediate-wave verifier enforces exact remaining-baseline monotonicity
against the governed inventory:

```
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory-489.json -InventorySha256 7330d6baecc93e9cac4f4f96438a21565994106984f50c239cd1e6d8f86d2b66 -Shipment 156-S -TerminalTask 175.012-T
```

The terminal command is exactly `golangci-lint run` (zero-warning required).

## Authority

- Authorized by: `Derek Williams <42183845+softwaresalt@users.noreply.github.com>` (configured git user identity).
- Authorization time: `2026-09-19T01:01:22Z`.
- Basis: explicit operator instruction across the baseline-convergence rescope
  session directing full-baseline (489-finding) convergence with task-scoped lint
  per member and mandatory zero-warning global lint only at terminal convergence.
