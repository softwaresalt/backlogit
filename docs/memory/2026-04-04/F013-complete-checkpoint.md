---
title: "Memory Checkpoint: Feature 013 - Release Pipeline Fix"
description: "Long-lived memory checkpoint summarizing Feature 013 completion state, decisions, and validation outcomes."
ms.date: 2026-04-04
ms.topic: reference
---

## Feature Memory: Release Pipeline Fix

Feature `F013` aligned the release workflows with the repository's current Go and
GitHub Actions policy baseline.

## Durable Outcomes

* `.github/workflows/release.yml` now uses a valid tag glob and pinned third-party
  actions with `persist-credentials: false`
* `.github/workflows/ci.yml` now matches the supported Go toolchain policy and the
  same action-pinning and credential rules
* `tests/integration/ci_compliance_test.go` captures the workflow invariants so
  future workflow edits fail fast when they drift

## Key Decisions

1. Tag matching uses `v*.*.*` so release triggers stay compatible with GitHub
   Actions glob rules.
2. Workflow Go versions are aligned with `go.mod` and the supported compatibility
   window rather than preserving an outdated matrix.
3. Third-party GitHub Actions are pinned by full SHA and checkout disables
   credential persistence by default.

## Operational Takeaways

* Treat workflow policy as code: when GitHub Actions rules change, update the
  characterization tests first.
* Keep release and CI workflow hardening in lockstep so one pipeline does not lag
  the other.
* Preserve queue and review artifacts separately from long-lived memory. Durable
  memory should capture decisions and outcomes, not session telemetry.
