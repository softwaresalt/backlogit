---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 073-S — create/update write-path nil-HeaderDef hardening (feat/073-artifacts-write-nil-headerdef). Records the READY verdict, invariants to preserve (fail closed on nil ws.HeaderDef in CreateArtifact/UpdateArtifact via the shared requireHeaderDef helper; error wraps blerrors.ErrConfig not ErrValidation → MCP internal/500 never validation_failed/422; the requireHeaderDef check precedes ApplyFieldDefaults/ValidateArtifactFields to avoid a nil-panic in ResolveFieldSchema; loaded-HeaderDef create/update regression still succeeds; no artifact mutation persisted on either nil path; CLI/MCP consistency via the single shared core functions), merge-only rollout path, healthy/failure signals, no monitoring or rollback required for the zero-blast-radius defensive edge fix (revert the merge commit if ever needed), validation window, ownership, and the deferred advisories recorded (not stashed). Third and final recurrence site of the nil-precondition-fail-open shape named in the exported-cache zero-value compound learning; sibling of the shipped 072-S doctor --target fix.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-closure.md
title: 073-S create/update nil-HeaderDef — Pre-Merge Operational Closure
---

# Operational Closure — 073-S create/update write-path nil-HeaderDef hardening (pre-merge)

- **Date**: 2026-07-02
- **Mode**: `pre-merge`
- **Shipment**: `073-S` · Feature `073-F` · Task `073.001-T`
- **Branch**: `feat/073-artifacts-write-nil-headerdef` → `main`
- **Verification report**: `docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-runtime-verification.md`
- **Readiness**: **READY** (pending operator P-014 merge approval + branch-protection approving review)

## Change summary

`CreateArtifact` and `UpdateArtifact` now fail closed when `ws.HeaderDef == nil`: a shared
`requireHeaderDef(ws)` helper returns an error wrapping `blerrors.ErrConfig` before any default
application or field validation, replacing the prior fail-open `if ws.HeaderDef != nil` guards that
silently skipped required-field validation and persisted the write. No new MCP error type, no
schema change — reuses the existing `internal` (500) mapping and the `ErrConfig` sentinel.

## Invariants to preserve

1. Nil `ws.HeaderDef` on the create/update write paths returns a non-nil error wrapping
   `blerrors.ErrConfig` (NOT `blerrors.ErrValidation`) — so MCP surfaces `internal` (500), never
   `validation_failed` (422), and the CLI exits non-zero.
2. `requireHeaderDef(ws)` runs **before** `ApplyFieldDefaults`/`ValidateArtifactFields` at both call
   sites (load-bearing: `headerDef.ResolveFieldSchema` dereferences its receiver with no nil-guard).
3. No artifact mutation is persisted on either nil path (create writes no file; update leaves the
   on-disk artifact unchanged) — the guard precedes `persistArtifact`.
4. Loaded-`HeaderDef` create and update still succeed and persist (regression guard tests).
5. CLI and MCP remain behaviorally consistent for this fault — both fail closed and both surface
   `internal` (mechanisms differ: `create_item` hard-maps via `InternalError`; `update_item`/
   `move_item` route via `domainError`'s default case).

## Pre-deploy audits

- None required. No migration, config, flag, or access change. Pure precondition-gating logic.

## Deployment / rollout path

- **Merge-only** (library/CLI + MCP binary). No deploy, canary, or migration.
- Consumed cross-repo by autoharness via the MCP create/update tools, but the changed branch is
  unreachable in a normally-initialized workspace, so real behavior is unchanged.

## Post-merge checks

- After merge, a source build `backlogit add`/`update` against a normally-initialized workspace
  still succeeds with exit 0 (spot-checked pre-merge). No further runtime observation required.

## Healthy vs failure signals

- **Healthy**: existing create/update behavior unchanged on schema-present workspaces; new tests
  remain green.
- **Failure**: any real workspace unexpectedly getting an `internal` "header definition not loaded"
  error on `add`/`update` would indicate a genuinely absent `header-def.yaml` (a real config fault
  the fix is designed to surface) — investigate the workspace init, not this change.

## Monitoring plan

- None. Zero-blast-radius defensive edge fix; the unit tests are the durable regression guard.

## Rollback

- **Trigger**: none anticipated.
- **Procedure** (if ever needed): `git revert` the merge commit; one shared helper + two adjacent
  call-site blocks, fully reversible.

## Validation window / owner

- **Window**: n/a (merge-only, no rollout).
- **Owner**: Ship agent handed to operator at P-014.

## Follow-up

- **No new stash.** `internal/core/artifacts.go` was the third and final recurrence site of the
  nil-precondition-fail-open shape named in
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`; the other
  nil-HeaderDef seams (`validateSizeValue`, `doctor_target.go`, `metadata_catalog.go`) already fail
  closed.
- **Deferred advisories (recorded, not stashed — mirrors 072-S)**: (1) a distinct agent-facing MCP
  error type for schema-absent (e.g. `workspace_not_initialized` / `precondition_failed`) instead of
  generic `internal`; (2) MCP tool-description enrichment documenting the new fail-closed mode. Both
  deferred as YAGNI tied to unreachability in a normal workspace; expanding either into a cross-layer
  MCP-contract change is disproportionate for a zero-blast-radius edge fix. See the plan Decisions/
  Risks for full rationale.
- **Knowledge graduation (post-merge)**: reinforce the compound learning with the third-instance
  resolution (create/update write paths closed) via `compound-refresh` during Step 6.

## Readiness recommendation

**READY** — merge can proceed once the operator grants P-014 approval and the branch-protection
approving review is satisfied. Ship halts at P-014; no self-merge.
