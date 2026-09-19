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
  exactly the 497-identity supported-platform union governed inventory at
  `docs/decisions/baseline-lint-inventory.json`
  (SHA-256 `6cd1d3468d6fe334b165763b33dc03aacdcfad04d369dda3c88057459b998f3a`);
  the union covers the Windows-primary 489 findings and the 8 Linux-only findings,
  with the 4 Windows-only findings encoded as Linux surface exclusions. Each
  identity is owned by exactly one finding-remediation task.
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

```text
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory.json -InventorySha256 6cd1d3468d6fe334b165763b33dc03aacdcfad04d369dda3c88057459b998f3a -Shipment 156-S -FeatureId 175-F -TerminalTask 175.012-T
```

The terminal command is exactly `pwsh -NoProfile -File scripts/verify-terminal-lint.ps1 -FeatureId 175-F`,
which runs `golangci-lint run` across the whole repository (zero-warning
required) and covers both declared supported surfaces (`windows`, `linux`).

## Authority

- Authorized by: `Derek Williams <42183845+softwaresalt@users.noreply.github.com>` (configured git user identity).
- Authorization time: `2026-09-19T01:01:22Z`.
- Basis: explicit operator instruction across the baseline-convergence rescope
  session directing full-baseline supported-platform union convergence (497
  findings: 489 Windows-primary plus 8 Linux-only) with task-scoped lint per
  member and mandatory zero-warning global lint only at terminal convergence.
