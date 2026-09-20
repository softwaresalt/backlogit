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

## Scope extension to wave-aligned replacement shipments (2026-09-19)

This authorization applies to the 13 wave-aligned replacement shipments
`157-S`..`169-S` (RS-W00..RS-W12) that the operator-accepted decomposition
(`docs/decisions/2026-09-19-baseline-convergence-decomposition.md`) substituted
for the now-superseded single-shipment packaging `156-S`.

**This is a derived application of the existing authority, not a new or fabricated
grant.** Its basis is the conjunction of:

1. the original `156-S` authorization substance recorded below — permitting the
   exact governed baseline findings owned by not-yet-completed remediation tasks
   to persist until terminal convergence at U12 (`175.012-T`);
2. the operator-**accepted** decomposition decision, which changes only the
   *packaging* (shipment membership and shipment-level sequencing) and leaves the
   98 executable tasks, the 184-edge DAG, the terminal task `175.012-T`, and the
   governed 497-identity inventory (`docs/decisions/baseline-lint-inventory.json`,
   SHA-256 `6758f96b03170d242bb0a9407effa2ed66c0a68d0951ff81d29f385bc1d91b60`)
   **unchanged**; and
3. the operator's explicit directive for this correction cycle to restage /
   decompose and resolve the PR #449 review findings, which includes the request
   to record authorization for the replacement sequence.

Because the *substance* of what is permitted is unchanged and only the
operator-directed container changed, extending the shipment scope to
`157-S`..`169-S` faithfully carries the same permission across an operator-directed
repackaging. It does **not** grant any new tolerance, relax any of the four
mandatory quality gates, or approve U1's destructive worktree refresh (all
constraints in the sections below apply unchanged to each replacement shipment).
Each replacement shipment inherits: no new/unowned findings; every tolerated
identity owned by exactly one remediation task; and mandatory zero-warning global
lint at terminal convergence. Terminal convergence remains the single terminal
task `175.012-T` (carried by replacement shipment `169-S` / RS-W12).

A separate, stronger capability — machine-authenticated operator authorization
with an external trust anchor — remains out of scope and is tracked independently
(stash `B633E9B9`); this record uses the same configured-git-identity attribution
as the original grant below.

## Scope of authorization

The operator authorizes a narrow, shipment-scoped procedural allowance for the
baseline-convergence release unit (feature `175-F`, shipment `156-S`):

- Shipment `156-S` MAY retain the exact golangci-lint v2.13.2 baseline findings
  that are owned by not-yet-completed finding-remediation tasks, until terminal
  convergence at U12 (`175.012-T`).
- No new or unowned findings may be introduced. The set of tolerated findings is
  exactly the 497-identity supported-platform union governed inventory at
  `docs/decisions/baseline-lint-inventory.json`
  (SHA-256 `6758f96b03170d242bb0a9407effa2ed66c0a68d0951ff81d29f385bc1d91b60`);
  the union covers the Windows-primary 489 findings and the 8 Linux-only findings,
  with the 4 Windows-only findings encoded as Linux surface exclusions. Each
  identity is owned by exactly one finding-remediation task.
- Terminal global lint is mandatory and MUST be zero-warning: the terminal task
  U12 runs `golangci-lint run` across the whole repository and the release unit
  is not converged until that run is clean.
- This authorization originally named shipment `156-S`; under the accepted
  decomposition (see the scope-extension section above) it applies to the
  wave-aligned replacement shipments `157-S`..`169-S`. It expires when the
  replacement sequence completes terminal convergence at U12 (`175.012-T`, carried
  by `169-S`) or when the operator revokes this session authorization.

## What this authorization does NOT grant

- It does **not** approve U1's (`175.001-T`) destructive worktree
  renormalization / refresh. That actual worktree mutation still requires
  immediate operator-only approval at the moment of execution during Ship.
- It does not authorize any relaxation of the four mandatory quality gates at
  terminal convergence, nor any tolerance for findings without a remediation
  owner.

## Enforcement binding

The intermediate-wave verifier enforces exact remaining-baseline monotonicity
against the governed inventory. Under the decomposition it is invoked once per
active replacement shipment in RS-W00..RS-W12 order (`157-S`..`169-S`),
substituting `-Shipment` with that wave's replacement shipment id; the superseded
`156-S` is never used as the `-Shipment` argument:

```text
pwsh -NoProfile -File scripts/verify-baseline-lint.ps1 -Inventory docs/decisions/baseline-lint-inventory.json -InventorySha256 6758f96b03170d242bb0a9407effa2ed66c0a68d0951ff81d29f385bc1d91b60 -Shipment <replacement-shipment-id> -FeatureId 175-F -TerminalTask 175.012-T
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
