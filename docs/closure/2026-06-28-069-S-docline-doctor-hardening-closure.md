---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 069-S — docline + doctor robustness hardening (PR #152, merge 1dd4e69a). doctor --fix-malformed cleared 038-DL/039-DL malformed archived_from (2 to 0); docline ApplyMigration apply-time TOCTOU re-read (ErrConcurrentEdit); ValidateFields full pattern+minLength schema enforcement (ErrSchemaViolation, 0 new deps). Monitoring = doctor archive audit + docline migration gate.'
doc_type: closure
docline:
    ms.date: 2026-06-28T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T22:36:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-28-069-S-docline-doctor-hardening-closure.md
title: 069-S Docline + Doctor Robustness Hardening — Post-Merge Operational Closure
---

# Operational Closure — Shipment 069-S (Docline + Doctor Robustness Hardening)

- **Shipment**: 069-S — docline + doctor robustness hardening
- **Feature**: 069-F (3 tasks: 069.001-T, 069.002-T, 069.003-T — all done/archived)
- **PR**: #152 — *docline + doctor robustness hardening*
- **Merge commit**: `1dd4e69a8fcefdf18f13efe90f49031d865c95db` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide)
- **Closure branch**: `post-merge/069-docline-doctor-hardening`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-28-069-S-docline-doctor-hardening-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (already merged; this artifact records monitoring + rollback for the shipped scope)

## Summary of the change

Three independent, narrow hardening tasks shipped under one feature:

- **069.001-T** — `doctor --fix-malformed`: gated behind `--check-archived-from`, clears malformed `archived_from` records that have no restore target via a body-preserving rewrite. Cleared `038-DL` and `039-DL` (2 to 0 malformed). Self-ref repair path unchanged. (commit 36926448)
- **069.002-T** — docline `ApplyMigration` apply-time re-read: before writing, the target is re-read; if on-disk bytes differ from the change `Before`, all writes abort with the `ErrConcurrentEdit` sentinel (TOCTOU clobber prevention). Target deletion is also treated as `ErrConcurrentEdit`. (commits 48e4d042, 03005fdb)
- **069.003-T** — `ValidateFields` full schema enforcement: hand-rolled pattern + minLength + additionalProperties checks (no JSON-schema dependency added — 0 new deps), mapped to `ErrSchemaViolation`. Presence + doc_type vocab preserved. (commit e6d5231f)
- **Dogfood**: `doctor --check-archived-from` now reports 0 self-ref + 0 malformed on `main` — this very fix cleared the archive it audits.
- **Plan**: `docs/exec-plans/2026-06-28-docline-doctor-hardening-plan.md`. **Deliberation**: `docs/decisions/2026-06-28-docline-doctor-hardening-deliberation.md`.

## Invariants to preserve

1. `doctor` reports "No issues found" on `main`; `--check-archived-from` = 0 self-ref + 0 malformed.
2. `archived_from` repairs stay body-preserving — only the target field changes, never body bytes.
3. docline `ApplyMigration` aborts atomically on concurrent edit — zero partial writes when on-disk bytes drift.
4. Unchanged files still apply normally; idempotent re-apply is still a no-op.
5. `ValidateFields` reports pattern/minLength/additionalProperties violations and passes valid frontmatter; no new module dependency.

## Pre-deploy audits

- Merge-only change; no migrations, flags, config, or access changes. None required.

## Deployment / rollout path

- Merge-only. Already merged to `main`; `backlogit.exe` rebuilt from `main`. No deploy step.

## Post-deploy checks

- `backlogit doctor` = "No issues found". ✅
- `backlogit doctor --check-archived-from --format json` = 0 findings. ✅
- `go test ./internal/core/... ./internal/docline/...` green. ✅
- `backlogit docs lint` clean. ✅

## Healthy signals

- doctor archive audit stays at 0 malformed + 0 self-ref.
- docline migration apply remains body-preserving and idempotent.
- No `ErrSchemaViolation` regressions on previously-valid frontmatter.

## Failure signals

- doctor surfaces new malformed/self-ref records after archive churn.
- docline writes partial output despite concurrent edit (TOCTOU guard regressed).
- valid docs newly rejected by ValidateFields, or invalid docs slipping through.

## Monitoring plan

- doctor archive audit (`doctor --check-archived-from`) run during each post-merge closure.
- docline migration gate (`docs lint`) on every doc-touching PR.

## Rollback trigger

- A regression in archive integrity or a false-positive validation failure that blocks legitimate doc edits.

## Rollback procedure

- `git revert 1dd4e69a8fcefdf18f13efe90f49031d865c95db` (revert the merge commit); rebuild binary.

## Validation window

- One closure cycle. No deploy surface; low blast radius.

## Owner

- Ship pipeline operator (softwaresalt).

## Source artifact cleanup

- See post-merge closure log. Stash sources `9685B1AA`, `AE53BC5C`, `B349CBED`; deliberation `069-DL`-equivalent at `docs/decisions/2026-06-28-docline-doctor-hardening-deliberation.md`.

## Follow-up

- Stashed: ValidateFields explicit empty-string-vs-absent-key threading (minLength parity for empty vs missing key). Source: this artifact.

## Readiness

**READY** — change is merged; monitoring = doctor audit + docline gate; rollback = revert 1dd4e69a.
