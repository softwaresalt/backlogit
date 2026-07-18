---
chunk_strategy: h1-h2-h3
description: 'Session memory — DARK_MODE dark-factory pipeline, stage-next pass: harvested stash bug 50C90A1B into feature 111-F / task 111.001-T / shipment 098-S and shipped the archive-provenance preservation fix (PR #255, merge 7767bc3). Records the TDD design pivot (status-gated emit over an unreachable clear-helper), the transition-hook coupling constraint, the two-model adversarial review (P1 declined with a guard test, three P2 follow-ups filed), the clean Copilot review, and post-merge closure.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-18/098-S-archive-provenance-dark-mode-memory.md
title: 098-S archive-provenance ship (dark-mode) — session memory
---

## Scope

DARK_MODE (P-017) dark-factory pipeline, operator AFK. Ordered scope: ship
095-S ✅ → ship 096-S ✅ → **stage-next** (triage 5 stash entries) — in progress.
The primary ship deliverable of stage-next was bug **50C90A1B**, harvested as
feature **111-F** / task **111.001-T** / shipment **098-S**.

## What shipped

Archive-provenance preservation: typed artifact updates dropped the
`archived_from` / `archived_status` frontmatter keys on archived items because
the generic codec did not model them. Fix = model the two keys as typed fields
and emit them in the single `WriteArtifactFile` persist seam **gated on
`Status == StatusArchived`**.

- `internal/models/artifact.go` — typed `ArchivedFrom` / `ArchivedStatus`.
- `internal/models/frontmatter.go` — parse both keys.
- `internal/core/artifacts.go` — status-gated emit in `WriteArtifactFile`.
- `internal/core/archive_update_provenance_test.go` — 4 tests (green).

Merged via **PR #255**, merge commit `7767bc3`, at 2026-07-18T06:34:03Z.

## Key decisions

- **Status-gate over clear-helper**: the `validate_status_transition` pre-update
  hook forbids `archived → anything`, so a `clearStaleArchiveProvenance` helper
  on the update path would be unreachable dead code AND would leave other
  writers (queue.go, move-relocate, migrate) able to emit stale keys. The
  status-gate makes emission one-way at the shared seam: archived status is
  necessary for the keys to be written, and non-archived writes suppress them.
  It does not backfill provenance for archived items that arrive without it
  (see the CreateArtifact / DB-sourced follow-ups).
- **Declined the arch P1** (remove the gate for an explicit clear-helper) with
  that rationale; instead **added** `TestUpdateArtifact_RejectsUnarchiveViaStatusUpdate`
  to pin the transition-hook coupling the reviewer called fragile.

## Adversarial review (2 cross-model reviewers)

Go Reviewer (gpt-5.6-sol) + Architecture Strategist (gemini-3.1-pro).
P0=0, P1=1 (declined + guard test), P2=3 filed to stash:
`80DD65C4` (queue.go MoveInQueue DB-sourced persist), `7EEADCD3` (CreateArtifact
accepts status=archived), `12B5649E` (serializer consolidation). All P2s
pre-existing, low-reachability, not regressions.

## Gates + review

Build/test/vet/lint all green on HEAD `e298084`; gofmt clean for the 4 changed
files. CI all green (`test` 2m52s). Copilot review COMMENTED, fresh, 9/9 files,
**0 comments, 0 threads**. §1.9 gate PASS on all 3 checks →
`DARK_MODE_MERGE_AUTHORIZED` → merge commit (P-009).

## Reconciliation

Pre: 098-S active, members done. `ship_shipment 098-S` → shipped; archived
111.001-T + 098-S + 111-F. Post: 098-S archived, queue file removed.

## Next steps

1. Finish this 098-S closure PR (chore/098-S-closure): closure doc + memory +
   backlog-state commit → Copilot + §1.9 gate → merge (§1.10).
2. Finish remaining stage-next dispositions:
   - Archive stash `A4BE2FAD` (resolved by 095-S).
   - Leave stashed with rationale: `7F0A6E89` (Principle IV external-repo —
     blocked), `CA877CD1` (low-pri prompt-artifact governance — deferred),
     `8CD8F46A` (governance policy — needs operator input). Plus the 3 new
     follow-ups `80DD65C4` / `7EEADCD3` / `12B5649E`.
   - Note 108-F restaging (096-S `proceed`) as out-of-scope 14h follow-up.
3. `DARK_MODE_COMPLETE` + final session summary.

## Watch

No admin fallback (halt on branch-protection review block). Two parked
untracked files must NOT be committed: `docs/decisions/2026-07-13-scratch-spike.md`,
`docs/memory/2026-07-17/094-S-ship-closure-memory.md`. Stage backlog files
individually (multi-pathspec abort + parked-file sweep risk).
