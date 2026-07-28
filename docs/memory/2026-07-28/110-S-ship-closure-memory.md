---
chunk_strategy: h1-h2-h3
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-28/110-S-ship-closure-memory.md
title: "Shipment 110-S — Ship Closure Memory"
---

# Shipment 110-S — Ship Closure Memory

**Date**: 2026-07-28  
**Agent**: Ship  
**Shipment**: 110-S ("Archive/shipment re-persist field-drop fix")  
**Status**: COMPLETE — all items archived, closure written

---

## Session Summary

Executed shipment 110-S end-to-end: TDD red→green, quality gates, PR #312,
Copilot review cycle, operator-approved merge, post-merge closure.

---

## Work Items Completed

| ID | Title | Final Status |
|----|-------|-------------|
| 110-S | Archive/shipment re-persist field-drop fix | archived |
| 129-F | Fix archive/shipment re-persist field-drop (links + provenance) | archived |
| 129.001-T | Skip already-archived linked deliberations in shipment archive flow | archived |
| 129.002-T | Reload from Markdown before re-persist so links and provenance survive | archived |

---

## Files Modified / Created

| File | Change |
|------|--------|
| `internal/core/shipment_lifecycle.go` | `attachCommitToItems` rewritten: single `findArtifact` (Markdown) load for both archived-skip guard and mutate-then-persist |
| `internal/core/shipment_repersist_test.go` | NEW: two regression tests (TDD RED→GREEN) |
| `.backlogit/queue/129.001-T.md` | Archived (done) |
| `.backlogit/queue/129.002-T.md` | Archived (done) |
| `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | NEW: compound learning |
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | Updated: `attachCommitToItems` note reflects PR #312 reload-from-Markdown fix |

---

## Key Decisions

1. **Chosen fix**: `findArtifact` (Markdown reload) at the `attachCommitToItems`
   re-persist seam. NOT widening DB projection. Precedent: `MoveInQueue`.

2. **Single load for guard + mutate**: After Copilot review (cycle 1), the
   split-load approach (`loadArtifact` for guard, `findArtifact` for persist) was
   replaced with a single `findArtifact` call. Split-load has a race window where
   DB and Markdown disagree on status.

3. **Skip archived items**: `attachCommitToItems` skips items where
   `artifact.Status == StatusArchived` — re-stamping an archived artifact is
   wrong; `ArchiveItem` already stamped commit + provenance.

4. **Test assertion for skip semantics**: The 129.001-T test checks that the
   deliberation's commit SHA is UNCHANGED after ship. Without this, the test
   would false-green under a reload-only fix (reload would restore provenance,
   letting the stamp succeed while being semantically wrong).

---

## Merge + Closure Milestones

| Step | Result |
|------|--------|
| Branch | `fix/129-archive-repersist-field-drop` |
| PR #312 | Merged @ `f32c9f7847f9cee9428ff8b76f0d7778748f0944` |
| CI | All checks green (test, Docline frontmatter gate, Markdown lint, CLI Reference Drift) |
| Copilot review | 1 comment (fix applied: consolidated to single findArtifact), thread resolved |
| §1.9 gate | PASSED — 0 unresolved Copilot threads, review covers HEAD |
| ship_shipment 110-S | archived_ids: [129.001-T, 129.002-T, 110-S, 129-F] |
| GI/GR reconcile | pre-mode PROCEED, post-mode PROCEED |

---

## Closure Artifacts

- Reconcile pre-mode: `.backlogit/reconcile/110-S-pre-20260728T141511.md`
- Reconcile post-mode: `.backlogit/reconcile/110-S-post-20260728T141707.md`
- Compound learning: `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
- Updated: `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`

---

## Environment Quirks Noted

- `git reset --hard origin/main` after merge reverted `.backlogit/` state (shipment
  re-claim + feature re-done transitions required after reset).
- `backlogit shipment ship` flag is `--sha` (not `--commit`).
- Copilot bot GraphQL login: `copilot-pull-request-reviewer` (no `[bot]` suffix).
- `gofmt -l .` flags ~96 pre-existing files on Windows CRLF; only changed files
  need to be clean (see 2026-07-28-durable-writes compound doc for details).

---

## Next Steps / Follow-Ups

- None from this shipment. The DB projection gap for `item_links` is now documented
  in the new compound learning; a product-level fix (widening `selectCols` or
  refusing to re-persist DB-loaded artifacts) is tracked as a follow-up concern but
  not a blocker.
