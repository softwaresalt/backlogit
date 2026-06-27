---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 067-S — ArchiveItem archived_from integrity, invertible unarchive, and legacy record repair (PR #141, merge 41f6ff7d)'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-27T18:55:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-067-S-archived-from-integrity-closure.md
title: 067-S archived_from Integrity — Post-Merge Operational Closure
---

# Operational Closure — Shipment 067-S (archived_from Integrity)

- **Shipment**: 067-S — ArchiveItem archived_from Integrity
- **Feature**: 067-F (7 tasks: 067.001-T … 067.007-T, all done/archived)
- **PR**: #141 — *ArchiveItem archived_from integrity — invertible unarchive + legacy repair*
- **Merge commit**: `41f6ff7d309ccb7c388accd85d2c438205370a77` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide)
- **Closure branch**: `post-merge/067-archived-from-integrity`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-27-067-S-archived-from-integrity-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (the change is already merged; this artifact records monitoring + rollback for the shipped scope)

## Summary of the change

Fixed the `core.ArchiveItem` self-referential `archived_from` defect and restored
invertible unarchive for pre-archived items:

- **067.001-T** — canonical restore-path resolver (`canonicalRestorePath`, `.backlogit/`-prefixed queue path).
- **067.002-T** — `ArchiveItem` writes the canonical `.backlogit/queue/<id>.md` `archived_from` for pre-archived items (where current path == archive path).
- **067.003-T** — `UnarchiveItem` read-time self-heal of legacy self-refs + a pre-rename clobber-refuse guard (`TestUnarchiveItem_RefusesToClobberExistingQueueFile`).
- **067.004-T** — doctor `archived_from` invertibility audit (self-ref + malformed detection).
- **067.005-T / 067.006-T** — core `--fix-archived-from` body-preserving repair + CLI-only flag gating (MCP intentionally not wired, Principle VII).
- **067.007-T** — migration runbook (`docs/closure/2026-06-26-archived-from-migration-closure.md`).
- **Live legacy migration**: 130/130 self-referential records rewritten to canonical paths (byte-clean, idempotent); 2 malformed records (`038-DL`, `039-DL`, value `done`) flagged-only.

## Invariants to preserve

1. `archived_from` always names the **canonical queue restore path**, never the record's own archive path.
2. Archive → unarchive is invertible: a shipped/archived item can be restored to `.backlogit/queue/`.
3. `doctor --check-archived-from` reports **0 self-referential** records (only the 2 known malformed `done` records remain, flag-only).
4. A read-time self-heal never clobbers a distinct existing queue destination.
5. Body bytes of archive records are preserved across stamping/repair (only the `archived_from` field changes).

## Pre-deploy audits

Not applicable as a service deploy — this is a library + CLI + on-disk data change that ships with the merge. The data migration (130 records) was committed inside PR #141 and re-verified on `main` before this closure.

## Deployment / rollout path

**Merge-only.** No service deploy, canary, or feature flag. The fix takes effect for any
binary built from `main` at or after `41f6ff7d`. The repo-root `backlogit.exe` is already
rebuilt from this commit.

## Post-deploy checks (performed at closure)

- First `shipment ship` since the fix (067-S itself) re-stamped 9 records with canonical `archived_from` — **confirmed** (see runtime-verification E1).
- `doctor --check-archived-from` on the live workspace: **0 self-referential**, 2 malformed flag-only — **confirmed** (E2).
- Core round-trip / self-heal / clobber-guard test suite green — **confirmed** (E3).

## Healthy signals

- `doctor --check-archived-from` reports **0** `archived_from_self_ref`.
- Every archive→unarchive round trip restores the record to `.backlogit/queue/`.
- Newly archived records carry `archived_from: .backlogit/queue/<id>.md`.

## Failure signals

- Any new `archived_from_self_ref` finding from `doctor --check-archived-from`.
- `UnarchiveItem` leaving an item un-restorable (source == destination) or clobbering a distinct queue file.
- A body-byte change in a repaired/stamped archive record (only `archived_from` should change).

## Monitoring plan

This is a data-integrity guarantee, not a runtime service, so "monitoring" = a repeatable audit + CI guardrails:

- **Audit**: run `backlogit doctor --check-orphans=false --check-duplicates=false --check-archived-from` at each future shipment closure (now part of the Ship dogfooding check) and on demand. Expect 0 self-referential.
- **CI guardrails**: `test (1.24)` runs the core invariant suite (round-trip, canonical-stamp, self-heal, clobber-guard, doctor audit) on every PR; `Docline frontmatter gate` covers new docs.

## Rollback trigger

- Post-closure `doctor --check-archived-from` reports any self-referential record introduced after `41f6ff7d`, **or** an archive→unarchive round trip fails to restore to queue, **or** a stamped/repaired record shows a body-byte change.

## Rollback procedure

- Revert the merge commit: `git revert -m 1 41f6ff7d309ccb7c388accd85d2c438205370a77` and rebuild. Because the data migration only rewrote the `archived_from` field (body bytes preserved), reverting is byte-exact for code; the legacy-record rewrite is independently reversible per the migration runbook's Rollback section.

## Validation window & owner

- **Window**: through the next 1–2 shipment closures (each re-runs the doctor dogfooding check). No active service to watch.
- **Owner**: maintainer (softwaresalt) — the doctor audit is the standing check.

## Source artifact cleanup

- `067-F` carries no `custom_fields.source_stash_id` and no `custom_fields.source_deliberation_id` (custom_fields = `{harness_status: pending}` only). Per the Ship Step 6 protocol, **no heuristic search** was performed and no source stash/deliberation artifact was removed or archived. The source stash `53F22794` and the deliberation `docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md` are referenced only in the feature body text and are retained for traceability.
  - Archived source artifacts: **none**.
  - Skipped (no covering custom field): source_stash_id, source_deliberation_id.

## Follow-ups

- **Malformed `archived_from: done` records** (`038-DL`, `039-DL`): flag-only by deliberate operator decision; doctor surfaces them every run. Permanent disposition deferred → **stashed for Stage** during this closure.
- **Codec extraction** (stash `8863C6C8`, medium): extract the shared body-preserving frontmatter codec + atomic-write helper to a leaf package, removing the `internal/docline ↔ internal/core` duplication that the 067-S import-cycle workaround introduced. **Pre-existing open stash** (created in the feature session) — noted, not re-stashed; for Stage.
