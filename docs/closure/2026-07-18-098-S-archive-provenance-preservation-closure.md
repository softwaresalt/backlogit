---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Post-merge closure for shipment 098-S — preserve archive provenance (archived_from / archived_status) through typed artifact update round-trips via a status-gated emit in the single WriteArtifactFile persist seam. Feature 111-F / task 111.001-T, harvested from stash bug 50C90A1B. Merged via PR #255 (merge commit 7767bc3) under P-017 dark-factory mode."
doc_type: closure
docline:
  ms.date: 2026-07-18T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-18-098-S-archive-provenance-preservation-closure.md
title: "098-S archive-provenance preservation closure"
---

## Scope

Post-merge closure for shipment **098-S** (archive-provenance preservation) —
feature **111-F**, task **111.001-T**, harvested from stash bug **50C90A1B**
during the dark-mode stage-next pass. Merged via **PR #255**, merge commit
`7767bc3cbdd96bb2a136225008eb98f28d3ee929`, at 2026-07-18T06:34:03Z.

## Problem

Typed artifact updates round-trip archived items through the generic frontmatter
codec, which modeled only recognized maps. The archive-provenance keys
`archived_from` and `archived_status` were **not modeled**, so any typed update
of an archived item (e.g. commit tracking, `migrate`, `references`, in-place
`move`) silently dropped both keys — losing the restore path and the original
pre-archive status.

## What shipped

| Deliverable | Item | Detail |
|---|---|---|
| Typed provenance fields | 111.001-T | `internal/models/artifact.go` — added `ArchivedFrom` / `ArchivedStatus string` with `omitempty` + doc comment. |
| Frontmatter parse | 111.001-T | `internal/models/frontmatter.go` — `ArtifactFromFrontmatter` now parses `archived_from` / `archived_status`. |
| Status-gated emit (the fix) | 111.001-T | `internal/core/artifacts.go` — `WriteArtifactFile` emits the two keys **only** when `Status == StatusArchived` and the field is non-empty. This is the single persist seam all typed paths funnel through, so emission is status-gated at one place: it preserves existing provenance on archived writes and suppresses the keys on any non-archived write. The guard is one-way — archived status is necessary for emission, so a non-archived write can never carry the keys — but it does **not** synthesize missing provenance for archived items that arrive without it (see the `CreateArtifact` and DB-sourced follow-ups below). |
| Regression tests | 111.001-T | `internal/core/archive_update_provenance_test.go` — 4 tests (see below). |

### Design decision: status-gate over clear-helper

The planned `clearStaleArchiveProvenance` helper (mirroring
`clearStaleBlockedReason` in `updateArtifactUngated`) was **dropped as
unreachable**. The `validate_status_transition` pre-update hook forbids
`archived → anything` ("status archived has no allowed transitions"), so the
typed update path can never move an item out of archived — a clear-helper on
that path would be dead code, and it would cover only the update path (leaving
`queue.go`, `move`-relocate, `migrate` free to emit stale keys). The
status-gate in the shared `WriteArtifactFile` seam is strictly more robust.

### Tests (4, all GREEN)

1. `TestUpdateArtifact_PreservesArchiveProvenanceOnArchivedItem` — commit update
   on an archived item preserves both provenance keys (integration via
   `core.UpdateArtifact`).
2. `TestUpdateArtifact_RejectsUnarchiveViaStatusUpdate` — hook-coupling guard:
   `archived → queued` typed update is rejected and provenance stays intact.
   Pins the transition-hook dependency that motivates the status-gate.
3. + 4. `TestWriteArtifactFile_ArchiveProvenanceIsStatusGated` (2 subtests) —
   emits keys when archived, omits them otherwise.

## Adversarial review (2 cross-model reviewers)

Reviewers: **Go Reviewer** (gpt-5.6-sol) and **Architecture Strategist**
(gemini-3.1-pro), both cross-model from the Ship caller. Outcome:
**P0=0, P1=1 (declined w/ rationale + guard test added), P2=3 (filed)**.

| Finding | Reviewer | Severity | Resolution |
|---|---|---|---|
| Remove the status-gate; use an explicit clear-helper on the update path | Arch | P1 | **DECLINED** with rationale — a clear-helper covers only the update path and leaves other writers able to emit stale keys; the status-gate is strictly more robust. Instead **ADDED** `TestUpdateArtifact_RejectsUnarchiveViaStatusUpdate` to pin the transition-hook coupling the reviewer flagged as fragile. |
| `queue.go MoveInQueue` persists DB-sourced clones that lack the typed provenance fields; reordering an archived-status queue view could strip provenance | Go | P2 | **FILED** `80DD65C4` — pre-existing (WriteArtifactFile never emitted the keys before this fix), low-reachability. |
| `CreateArtifact` accepts `status=archived` at creation | Go | P2 | **FILED** `7EEADCD3` — create-path serializer is separate; new items are never archived-with-provenance. |
| Serializer duplication (WriteArtifactFile vs inline `createArtifact` map builder) | Arch | P2 | **FILED** `12B5649E` — consolidation is future-scope. |

All three P2s are pre-existing, low-reachability, and **not regressions** from
this change. Local review readiness: **READY_WITH_FOLLOWUPS** (zero unresolved
P0/P1; three P2 follow-ups filed). Reviewed HEAD `e298084`.

## Regression trace

`move` / force paths funnel through `WriteArtifactFile` → the status-gate
auto-preserves (archived) or auto-clears (non-archived). `migrate` /
`references` / in-place `move` now **preserve** provenance where they previously
dropped it — a net improvement, no regression. `ArchiveItem` / `UnarchiveItem`
use the raw write path (untouched): `ArchiveItem` writes the keys raw,
`UnarchiveItem` deletes them raw — both unaffected.

## Quality gates (HEAD e298084)

`go build ./...` ✅ · `go test ./...` ✅ (all packages) · `go vet ./...` ✅ ·
`golangci-lint run` ✅ (0) · gofmt (4 changed files clean; pre-existing LF debt
out of scope) ✅.

## CI + Copilot review (PR #255)

CI: all checks green (`test` 2m52s, `Docline frontmatter gate`, `CLI Reference
Drift`, `Detect code changes`). Copilot review completed **COMMENTED**, fresh
(review `commit.oid` == HEAD `e298084`), reviewed **9/9 files** and **generated
no comments**. §1.9 pre-merge gate: Check 1 (no pending review request) ✓,
Check 2 (freshness — review covers HEAD) ✓, Check 3 (zero unresolved Copilot
threads) ✓.

## GI/GR reconciliation (shipment-reconcile)

- **Pre-mode**: 098-S `active` in queue; members 111-F + 111.001-T `done`. ✓
- **`ship_shipment 098-S`**: `shipment_status: shipped`; archived 111.001-T +
  098-S + 111-F; `returned_ids: []`. ✓
- **Post-mode**: 098-S present in archive with `status: archived`; absent from
  queue (`.backlogit/queue/098-S.md` removed). ✓

## DARK_MODE (P-017)

- `DARK_MODE_ACTIVE` scope: ship 095-S → 096-S → stage-next (operator AFK).
  `merge_approval_pre_authorized=TRUE`, `admin_fallback_pre_authorized=FALSE`.
  098-S is the primary ship deliverable of the stage-next pass.
- `LOCAL_REVIEW_READY`: HEAD `e298084`, READY_WITH_FOLLOWUPS, P0/P1=0.
- `DARK_MODE_MERGE_AUTHORIZED`: PR #255, HEAD `e298084`, checks green,
  merge-commit strategy (P-009), approval source = activation record, scope
  match = 111-F/098-S. Normal merge path (`NORMAL_MERGE_READY`); no admin
  fallback.

## Residual risk and follow-ups

Three P2 follow-ups filed to stash for future triage:

- `80DD65C4` — `queue.go MoveInQueue` DB-sourced persist drops provenance
  (reload-from-Markdown-before-persist, or add fields to the DB codec).
- `7EEADCD3` — `CreateArtifact` accepts `status=archived`.
- `12B5649E` — serializer consolidation (WriteArtifactFile vs `createArtifact`
  inline map builder).

None block the shipped behavior; all are pre-existing, low-reachability.

## Release-observability (operational evidence)

The shipped change (PR #255) modifies the `WriteArtifactFile` serializer, a
runtime-affecting persist path. Operational evidence for the release:

| Item | Value |
|---|---|
| Monitoring signal | **Manual** — no runtime metrics system for the local backlogit index. Health signal: archived artifacts under `.backlogit/archive/*.md` retain `archived_from` / `archived_status` after subsequent typed updates (e.g. `track_commit`, `migrate`). The 4 regression tests in `archive_update_provenance_test.go` are the automated guard. |
| Owner | Ship agent (dark-mode session); operator on return. |
| Observation window | Next backlog mutation cycle that touches an archived item (commit tracking, migrate, references). No timed window — surfaced by the regression suite on every `go test ./...`. |
| Baseline | Pre-fix: typed updates of archived items dropped both provenance keys. Post-fix: keys preserved on archived writes, suppressed on non-archived writes. |
| Rollback trigger | An archived item losing `archived_from` / `archived_status` after a typed update, OR a non-archived item emitting the keys, OR any new `archive_update_provenance_test.go` failure. |
| Rollback procedure | Revert merge commit `7767bc3` (PR #255) via `git revert -m 1 7767bc3`. The revert is clean at the code level (isolated to `artifact.go` / `frontmatter.go` / `artifacts.go`, no schema or data migration). **It is not behaviorally safe on its own**: reverting re-exposes the original defect, where the next typed update of an archived item can erase `archived_from` / `archived_status`. Containment before updates resume: (1) freeze typed updates of archived items (avoid `track_commit` / `migrate` / `references` on archived artifacts), (2) back up `.backlogit/archive/*.md` so provenance can be restored if lost, and (3) re-run `go test ./internal/core -run ArchiveProvenance` to confirm the regression guard state before re-enabling archived-item mutations. Prefer a roll-forward fix over leaving the revert in place. |

## Compound-refresh

The existing memory fact "The generic Artifact codec preserves only recognized
maps such as custom_fields; an unmodeled top-level docline map is dropped on
parse/write" (frontmatter.go / artifact.go) is the root-cause pattern this fix
addresses for the archive-provenance keys specifically. No compound learning was
invalidated; the status-gate is a localized robustness fix rather than a
cross-cutting change warranting a new compound entry beyond this closure record.
