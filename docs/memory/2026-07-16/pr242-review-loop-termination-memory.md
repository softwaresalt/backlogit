---
description: "Ship-lane session memory for PR #242 (post-merge PR #241 local-review-fix follow-up) — review cycles 9-12, adversarial complete-class enumeration to terminate a Copilot whack-a-mole loop, merge 3d2ebda, and closure. Docs/planning/backlog-only; no code changed."
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-16T00:00:00Z
  ms.topic: memory
source: docs/memory/2026-07-16/pr242-review-loop-termination-memory.md
title: "PR #242 review-loop termination — Ship session memory"
---

## Outcome

PR #242 merged to `main` at merge commit `3d2ebda` (two parents — P-009
merge-commit compliant; squash/rebase disabled in repo settings). Remote branch
`chore/post-merge-pr241-local-review-fixes` deleted. This PR was the docs /
planning / backlog-only follow-up to the already-merged PR #241 (docline
soft-key regression guard shipment). No production code changed, so no runtime
verification surface applied.

This closes the original operator restaging directive in full:

1. Formal-gate work as a time-boxed architecture spike —
   `docs/decisions/2026-07-14-formal-gate-architecture-spike.md` (merged).
2. Docline guard as a separate shipment — landed as PR #241.
3. D7B1B33D in the next single staging PR — harvested as `108-F` (size
   estimation), staged blocked/unmanifested with traceability preserved.

## Review cycle arc (9-12)

The session ran under `careful` safety mode with explicit operator authorization
to exceed the 3-cycle auto limit ("finish resolving all copilot review issues...
anticipate additional issues through your own adversarial round of reviews.
Repeated rounds of copilot reviews is inefficient").

- Cycle 9 (`fb649f2`): date-time attribution scoped to JSON schema only (not the
  Go validator); CRLF reframed to canonical LF-normalized body equivalence.
- Cycle 10 (`13f20a6`): two-path split — `core.SetArtifactSize` IS a
  body-preserving backlog-writer path (via `internal/mdfront`); only the generic
  rebuild route LF-normalizes and drops.
- Cycle 11 (`0c2dff8`): enumerated the COMPLETE docline schema vs Go-validator
  divergence set in one pass (whitespace-only rejection via `strings.TrimSpace`,
  closed `doc_type` vocab, unknown-profile guard, no `ingested_at` date-time
  check) to end the "except X" enumeration-gap loop.
- Cycle 12 (`d83c83a`): terminal body-preservation class sweep — exhaustively
  corrected size-plan inventory/synthesis/changelog/summary + `109.007-T` +
  `109-F` so nested-`custom_fields`-survival is never conflated with top-level
  extension-graph preservation.

## Two-path preservation model (code-verified truth)

- Generic rebuild route (`models.ParseFrontmatter` -> `ArtifactFromFrontmatter`
  -> `core.WriteArtifactFile`): LF-normalizes the whole document; re-emits only
  struct-backed fields; nested `custom_fields` SURVIVES
  (`internal/models/frontmatter.go:74-76`); unknown TOP-LEVEL extension keys are
  DROPPED (no carrier at HEAD).
- `core.SetArtifactSize` (`internal/core/artifact_size.go:35-95`): decodes/encodes
  via `internal/mdfront` directly; preserves EXACT body bytes + the FULL
  frontmatter map including unknown top-level keys. It is the size-write path;
  it is NOT the only mdfront-based body/map-preserving mutator — the doctor
  archived_from repair codec (`rewriteArchivedFromField` /
  `removeArchivedFromField`, `internal/core/doctor.go:695,720`) likewise
  preserves the full top-level map + exact body bytes. What is unique to the
  generic rebuild route is the DROP of unknown top-level keys; the mdfront-based
  mutators preserve them. (SetArtifactSize stores size under nested
  `custom_fields.size` today, while 109-F proposes size* as top-level — a
  documented reconciliation tension.)

## Loop termination

Cycle 12 was the first fresh Copilot review of the current HEAD to return ZERO
new findings. The §1.9 readiness gate passed clean: FRESH review (oid == HEAD),
0 pending requests, 0 unresolved Copilot threads. The recurring size-plan/109
whack-a-mole class is now fully enumerated across all three files; no cycle 13
was needed.

## Deferred (backlog — intentionally NOT patched)

These remain the time-boxed architecture-spike items, out of scope for a
docs/planning follow-up: formal evidence trust/forgery model, mutation-manifest
replay/binding, exact-byte CRLF *implementation*, and partial core-mutation
rollback guarantees. Tracked via the `109-*` spike tasks (109.001-T..109.007-T,
109.004-T synthesis) and the blocked `108-F` (depends on 109.004-T proceed exit,
107.009-T docline pass-through codec, 107.011-T schema opening).

## Files modified this session

- `fb649f2`: hooks_queue.jsonl, 107.011-T, 109-F, 109.007-T, decision.md,
  guard-plan.md, size-plan.md
- `13f20a6`: hooks_queue.jsonl, 109-F, 109.007-T, size-plan.md
- `0c2dff8`: hooks_queue.jsonl, 107.011-T, decision.md, guard-plan.md
- `d83c83a`: hooks_queue.jsonl, 109-F, 109.007-T, size-plan.md

## Next steps

- `108-F` stays blocked/unmanifested until a later Stage restaging moves it
  blocked->active after `109.004-T` records a proceed decision and all three
  prerequisite edges (109.004-T, 107.009-T, 107.011-T) complete.
- Recommended follow-up (not yet filed): a dedicated repo-wide `gofmt`
  remediation item for the pre-existing unrelated 26-file debt scoped out of
  this shipment.
