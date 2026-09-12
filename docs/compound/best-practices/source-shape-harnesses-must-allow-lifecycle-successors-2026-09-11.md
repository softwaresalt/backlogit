---
chunk_strategy: h1-h2-h3
description: "Source-shape harnesses should enforce permanent contracts or an explicit set of authorized lifecycle states, not temporary scaffold bodies and placeholders."
doc_type: learning
docline:
  category: best-practices
  citations:
    - ".backlogit/archive/158.003-T.md"
    - ".backlogit/archive/158.008-T.md"
    - ".backlogit/archive/158.009-T.md"
    - ".backlogit/archive/158.010-T.md"
    - ".backlogit/archive/158.011-T.md"
    - "docs/memory/2026-09-10-140s-declaration-behavior-split.md"
    - "docs/compound/2026-09-08-parity-harness-design-patterns.md"
    - "docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md"
    - "https://github.com/softwaresalt/backlogit/pull/436"
    - "commit:87a9f23fa0e073a445f23aeefa6a236edbb1e961"
    - "commit:d82669208640d52fa394ad60fc13bc4f86a48057"
    - "commit:22cc341ce00167e9357dcde75ed2f3e7cd672ab9"
    - "commit:eb3705aeb9220dd115dffda1b71b6b3bfe682524"
  component: tdd-source-shape-harness
  date: 2026-09-12T02:31:00Z
  file_path: internal/faultline
  message: "A scaffold test rejects the authorized behavior or integration state that replaces its temporary no-op body or placeholder."
  problem_type: lifecycle-frozen-test-contract
  resolution_type: design_change
  root_cause: "The harness encoded an implementation phase as a permanent invariant instead of separating stable declaration shape from temporary scaffold state."
  severity: high
  source_pr: 436
  source_shipment: 140-S
  tags:
    - tdd
    - source-shape
    - scaffolding
    - lifecycle
    - placeholders
    - ast
schema_version: "1.0"
source: docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md
title: "Source-shape harnesses must allow authorized lifecycle successors"
---

## Problem

A declaration-first TDD split used AST source-shape tests so prerequisite tasks
could compile before behavior existed. The initial scaffold harness permanently
required the analyzer run function to return `nil, nil` and required later
multichecker analyzers to remain commented placeholders. Those assertions were
correct only during the scaffold task. They would reject the authorized
behavior and final integration tasks that followed.

The same risk applies when a permanent test pins a transient dependency state,
an empty fixture directory, or any other red-phase-only shape.

## Root Cause

The harness mixed two contracts:

* Stable declarations and wiring rules that must survive the release
* Temporary implementation state used only to establish the red phase

Source-shape tests are persistent repository tests. Unless removed in the same
authorized transition, every assertion must remain valid after successor tasks
replace the scaffold.

## Resolution

Assert permanent shape wherever possible:

* Analyzer variable type, name, documentation, and run signature
* Public types, fields, methods, and function signatures
* Exactly one direct `multichecker.Main` call
* Final ordering and uniqueness rules

When a serialized lifecycle intentionally has multiple valid states, enumerate
the complete authorized state set. Shipment 140-S accepted either:

* Scaffold state: FL001 live with four exact reserved comments
* Final state: FL001-FL005 live with no reserved comments

It did not freeze the no-op run body. It also moved historical
no-version-change verification out of the permanent shape contract instead of
pinning unrelated transitive dependencies forever.

## Prevention

* Label every source-shape assertion as permanent or transition-only
* Keep permanent tests focused on contracts, not temporary function bodies
* Encode all authorized successor states when staged integration is intentional
* Verify historical diff properties against the implementation base during the
  owning task, not through a forever test
* Remove or replace transition-only assertions in the same serialized task that
  advances the lifecycle
* Use the signature checks in
  [Cross-surface parity harness design patterns](../2026-09-08-parity-harness-design-patterns.md)
  as the baseline, then add lifecycle-state handling separately
