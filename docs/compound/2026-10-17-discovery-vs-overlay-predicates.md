---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Keep discovery-exclusion and overlay-eligibility predicates separate"
description: "A widened path predicate reused for a second purpose caused a regression on the red-test honesty gate (PR #518); use one predicate per purpose and test the cross-purpose cases first."
doc_type: learning
source: docs/compound/2026-10-17-discovery-vs-overlay-predicates.md
docline:
    date: 2026-10-17T00:00:00Z
    severity: medium
    tags:
        - review
        - redhonesty
        - 143-S
---

# Discovery exclusion vs overlay eligibility

## Problem

PR #518 (143-S, `internal/redhonesty`) took eleven Copilot cycles. Cycle 9 widened `isTestdata` to cover Go-ignored `_`/`.` names so discovery would skip them. The same predicate also decided which files the baseline overlay replaces with head contents, so hidden production inputs such as `p/.config` were overlaid, which could certify an already-passing test as an honest red (cycle 10). Cycle 11 found the same class through the `_test.go` suffix check.

## Learnings

* **One predicate per purpose.** "What the go tool ignores" (discovery) and "what may be overlaid with head state" (overlay) differ. Name them separately (`isTestdata`, `isTestdataDir`, `isOverlayPath`, `isTestFile`).
* **Overlay is a whitelist.** Overlay only real test files and explicit `testdata` paths. Everything else is production state and stays at baseline.
* **Test cross-purpose cases first.** When widening a shared helper, grep every caller and write the regression for each caller before the change.
* **Audit by class.** Before pushing a Go-discovery fix, check hidden and underscore names, `*_test.go` basenames in ignored directories, build constraints, symlinks and path containment together.