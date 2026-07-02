---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 072-S — doctor --target nil-HeaderDef hardening (PR #158). Records the READY verdict, invariants to preserve (versioned doctor --target exit-code contract 0/1/2/3/4 unchanged, DoctorTargetResult schema_version 1.0.0 unchanged, OK==true iff Kind==pass, loaded-HeaderDef pass regression guard, CLI/MCP consistency via the single shared function), merge-only rollout path, healthy/failure signals, no monitoring or rollback required for the zero-blast-radius defensive edge fix (revert the merge commit if ever needed), validation window, ownership, and the stashed follow-up 266816CE for the internal/core/artifacts.go write-path fail-open shape.'
doc_type: closure
docline:
    ms.date: 2026-07-01T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-01T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-01-072-S-doctor-nil-headerdef-closure.md
title: 072-S doctor --target nil-HeaderDef — Pre-Merge Operational Closure
---

# Operational Closure — 072-S doctor --target nil-HeaderDef hardening (pre-merge)

- **Date**: 2026-07-01
- **Mode**: `pre-merge`
- **Shipment**: `072-S` · Feature `072-F` · Task `072.001-T`
- **PR**: #158 (`feat/072-doctor-target-nil-headerdef` → `main`)
- **Verification report**: `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-runtime-verification.md`
- **Readiness**: **READY** (pending operator P-014 merge approval + branch-protection approving review)

## Change summary

`ValidateDoctorTargetResolved` now fails closed when `ws.HeaderDef == nil`: returns
`kind=DoctorTargetIO` (exit 3) with message "header definition not loaded; cannot perform
required-field validation" instead of the prior fail-open `kind=pass`. Reuses the existing
io/exit-3 bucket — no new kind, no schema-version bump, no exit-code table change.

## Status

- CI (PR #158): all 4 checks green (`test 1.23`, `test 1.24`, Docline gate, CLI Reference Drift).
- Review: Copilot `COMMENTED`, 11/11 files reviewed, **zero comments**; code-review agent
  (report-only) found **no P0/P1**. §1.9 readiness gate: **PASS** (review covers HEAD `30e0a35`,
  zero unresolved Copilot threads).
- Unresolved review items: none.

## Invariants to preserve

1. Versioned exit-code contract unchanged: `0` pass / `1` validation / `2` timeout /
   `3` scope\|io / `4` busy.
2. `DoctorTargetResult` schema (`schema_version 1.0.0`) unchanged.
3. `OK == true` iff `Kind == pass` on every return path.
4. Loaded-`HeaderDef` valid artifact still returns `kind=pass` (regression guard test).
5. CLI and MCP remain behaviorally consistent (single shared function).

## Pre-deploy audits

- None required. No migration, config, flag, or access change. Pure classification logic.

## Deployment / rollout path

- **Merge-only** (library/CLI + MCP binary). No deploy, canary, or migration.
- Consumed cross-repo by autoharness as the `doctor --target` gate, but the changed branch is
  unreachable in a normally-initialized workspace, so real gate behavior is unchanged.

## Post-merge checks

- After merge, source build `backlogit doctor --target <valid artifact>` still returns exit 0
  (spot-checked pre-merge). No further runtime observation required.

## Healthy vs failure signals

- **Healthy**: existing pass/validation/scope/io/busy outcomes unchanged; new test remains green.
- **Failure**: any real workspace unexpectedly returning `kind=io` "header definition not loaded"
  for `doctor --target` would indicate a genuinely absent `header-def.yaml` (a real config fault
  the fix is designed to surface) — investigate the workspace init, not this change.

## Monitoring plan

- None. Zero-blast-radius defensive edge fix; the unit test is the durable regression guard.

## Rollback

- **Trigger**: none anticipated.
- **Procedure** (if ever needed): `git revert` the merge commit; single-function, fully reversible.

## Validation window / owner

- **Window**: n/a (merge-only, no rollout).
- **Owner**: Ship agent handed to operator at P-014.

## Follow-up

- Stashed for Stage: `internal/core/artifacts.go:224,514` write paths share the same fail-open
  shape (`ValidateArtifactFields` behind `if ws.HeaderDef != nil`). Out of scope for 072-S.

## Readiness recommendation

**READY** — merge can proceed once the operator grants P-014 approval and the branch-protection
approving review is satisfied. Ship halts here; no self-merge.
