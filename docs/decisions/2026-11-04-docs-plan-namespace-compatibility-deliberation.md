---
chunk_strategy: h1-h2-h3
description: 'Decision for the authorized plan documentation namespace relocation and direct compatibility work.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-11-04-docs-plan-namespace-compatibility-deliberation.md
title: 'Plan Documentation Namespace Compatibility Decision'
docline:
  decision_status: decided
  depth: standard
  promoted_to: plan
---

## Problem Frame

The operator approved moving the remaining tracked plan from `docs/exec-plans/`
to `docs/plans/` and updating directly dependent references. The relocation is
under way on the existing administrative staging branch. The current docline
classifier recognizes the legacy directory as `plan` but classifies the
canonical directory as the default `guide`. Authoring and runtime contracts
must agree without changing the prior MCP runtime design or its shipments.

The operator explicitly requested "newcanonicaldocs/plans recognition,
intentionallegacycompatibility" and authorized a separate, narrowly scoped
maintenance shipment. The operator also required that Stage not run linters,
builds, or tests and that Ship supply required source-entrypoint evidence.

## Research Findings

* `internal/docline/policy.go` has a `docs/exec-plans/` plan rule and no
  `docs/plans/` rule
* `internal/core/custody_exec.go` already accepts both prefixes; its existing
  acceptance must remain intact
* The old namespace occurrence in `internal/core/doctor.go` is a historical
  comment, not an active path contract
* `.autoharness/config.yaml` and installed harness guidance still direct new
  plans to the legacy path
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` notes that source
  presence checks do not establish self-path correctness and that migrations
  must preserve document bodies
* The related learnings on path validation and CLI/MCP parity favor retaining
  existing normalization and containment behavior

## Options Evaluated

### A: Canonical rule with legacy recognition

Recognize `docs/plans/` as `plan`, keep `docs/exec-plans/` recognition for
historical compatibility, and correct active authoring pointers. This keeps
existing readers compatible and directs new work to the canonical namespace.

### B: Canonical rule only

Remove legacy recognition when adding the new prefix. This is smaller but can
misclassify historical references and conflicts with the requested compatibility.

## Trade-off Comparison

| Criterion | A: dual recognition | B: canonical only |
|---|---|---|
| Current classification | Correct | Correct |
| Legacy compatibility | Preserved | Broken |
| Migration scope | Bounded | Bounded, with compatibility risk |

## Decision

Use option A, matching the operator's explicit canonical-namespace and legacy
compatibility direction. Limit implementation to the classifier, the active
workspace configuration, and directly coupled authoring guidance. Do not alter
historical evidence, existing MCP plan scope, or existing shipment membership.

## Rejected Alternatives

Option B does not satisfy the requested legacy compatibility. Rewriting all
historical paths would destroy provenance and is outside the authorized scope.

## Unresolved Questions

The implementation plan requires source-entrypoint authoring validation by
Ship, independent plan review, and harness evidence before implementation.

## Risks and Mitigations

Keep the legacy rule and existing path normalization unchanged. Preserve
historical event and checkpoint records. Require a compiling red classifier
harness before the production rule is changed.
