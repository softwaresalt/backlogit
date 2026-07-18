# 095-S Ship + Closure Memory — docline soft-key regression guard

**Date**: 2026-07-18 (UTC) · **Mode**: P-017 Dark Factory (operator AFK)
**Shipment**: 095-S · **Feature**: 107-F · **PR**: #250 · **Merge**: `ede77ed`

## Outcome

Shipped 095-S end-to-end autonomously under dark mode. Live-corpus docline
soft-key regression guard + hermetic value test + 12-doc backfill + CI docs-lint
enforcement. Merged via merge commit (P-009); post-merge closure complete.

## Task IDs completed

- 107.001-T (live-corpus guard), 107.008-T (hermetic value test),
  107.002–007-T (corpus backfill) — all done + archived.
- Feature 107-F — done + archived (feature-scope-root derivation at ship time).
- Shipment 095-S — shipped + archived.

## Files modified (merged in PR #250, commit 9f11df7)

- NEW `tests/integration/docline_soft_keys_test.go` (LF) — live corpus guard;
  eager parent-scope computation (fixes vacuous filtered subtests); constant
  equality asserts vs `docline.DefaultChunkStrategy`/`DefaultSchemaVersion`.
- NEW `tests/integration/docline_soft_key_values_test.go` (LF) — hermetic value test.
- `.github/workflows/ci.yml` — guard step in docs-lint job (gated on `docline_required`).
- `tests/integration/ci_compliance_test.go` — protect guard-step wiring in
  `TestHeavyStepsAreFailSafeGated`.
- `docs/docline-frontmatter-authoring-guide.md` — required-in-source note.
- 12 doc frontmatter backfills (closure/compound/decisions/design-docs/exec-plans).

## Closure state (this branch: chore/close-095-S)

- `.backlogit/archive/095-S.md` (RM from queue), member + 107-F archive stamps,
  hooks_queue.jsonl — from `ship_shipment 095-S`.
- NEW `docs/closure/2026-07-18-095-S-docline-soft-key-guard-closure.md`.
- Commit tracked on 095-S: 9f11df7.

## Decisions + rationale

- **9→12 backfill variance**: corpus drifted; 3 extra docs had wrong value
  `chunk_strategy: h1-h2`. Guard correctly named 12. Backfilled all.
- **CI wiring included in scope**: judged closing the docs-only bypass as
  completing the guard deliverable (aligned with ci.yml's "gate expensive steps
  inside always-reporting jobs" invariant), not scope expansion. Duck flagged
  it as blocking; independently verified real via ci.yml paths-filter analysis.
- **Adversarial consensus**: single-reviewer blocking finding treated as must-fix
  after independent verification. All actionable findings fixed in one pass.
- **Merge**: NORMAL_MERGE_READY; normal merge path, no admin fallback (not
  authorized). Copilot reviewed 28/28 files, zero comments.

## Gotchas carried forward

- **CRLF/gofmt**: working-tree files are CRLF; `gofmt -l .` false-flags them.
  `.gitattributes` `* text=auto` stores LF blobs (CI-clean). Verify gofmt on the
  cmd-redirected LF blob, NOT the working-tree file or a PowerShell `Out-File`
  copy (adds BOM → false gofmt flag).
- **schema_version MUST be quoted** `"1.0"` (unquoted → YAML float → guard rejects).
- **Closure/compound/decision docs are in guard scope** — any new doc under
  docs/ (except docs/memory, docs/archive, .github) MUST carry both soft keys or
  CI docs-lint fails. `docs/memory/` is excluded.
- **Copilot review request** via `gh api --method POST .../requested_reviewers
  -f "reviewers[]=copilot-pull-request-reviewer[bot]"` (not auto-requested here).

## Next steps

1. Commit + push closure branch; open post-merge closure PR (§1.10 — Copilot
   review + §1.9 gate required); merge.
2. **096-S** spike (109-F + 109.001–007-T) — size-extension architecture spike;
   produces findings artifact + proceed/pivot/defer decision (likely no prod code).
3. **stage-next** — triage 5 active stash entries → backlog + shipments.

## Parked / leave-alone

- Untracked `docs/decisions/2026-07-13-scratch-spike.md` — scratch, do NOT commit
  (docs/decisions is in guard scope; committing without soft keys breaks CI).
- Untracked `docs/memory/2026-07-17/094-S-ship-closure-memory.md` — prior session
  artifact; leaving as-is.
- Pre-existing `git stash` entries (start.ps1 reformat, etc.) — restore for
  operator at session end; do NOT touch `stash@{1}`.
