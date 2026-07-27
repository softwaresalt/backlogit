---
chunk_strategy: h1-h2-h3
description: "Ship post-merge closure memory for shipment 107-S ('Fix docline collectInScopeDocs relative-root filepath.Rel error', stash EF4C0EC6): PR #303 merged (8a757d5e), shipment + 127-F + 127.001-002-T archived, compound learning graduated on absolutizing the filepath.Rel base for absolute walked paths."
doc_type: memory
docline:
  date: 2026-07-27T00:00:00Z
  ms.topic: closure
schema_version: "1.0"
source: docs/memory/2026-07-27/107-S-post-merge-closure-memory.md
title: "107-S Post-Merge Closure — Ship Session Memory"
---

# 107-S Post-Merge Closure — Ship Session Memory

## Shipment Outcome

- **Shipment**: 107-S — "Fix docline collectInScopeDocs relative-root
  filepath.Rel error"
- **Status**: SHIPPED and ARCHIVED
- **Merge commit**: `8a757d5ed4ffdd4fcb782c2c295e2a660e90994e` on `origin/main`
  (merge-commit strategy, P-009)
- **Implementation branch**: `fix/107-s-docline-relative-root` (single branch,
  P-016) — RED test, GREEN fix, backlog commit-tracking
- **PR**: #303 — <https://github.com/softwaresalt/backlogit/pull/303>
- **Closure branch**: `chore/close-107-s` (this session)
- **Origin stash**: EF4C0EC6 (bug, medium) — harvested during Stage, now
  fully closed

## Items Done / Archived

| Item | Type | Terminal status | Location | Archive provenance |
|---|---|---|---|---|
| 127-F | feature | archived | `.backlogit/archive/127-F.md` | 8a757d5e |
| 127.001-T | task (RED) | archived | `.backlogit/archive/127.001-T.md` | 8a757d5e |
| 127.002-T | task (GREEN) | archived | `.backlogit/archive/127.002-T.md` | 8a757d5e |
| 107-S | shipment | archived | `.backlogit/archive/107-S.md` | 8a757d5e |

The **Archive provenance** column is the terminal `commit` stamped by
`ShipShipment` on every archived artifact — the merge commit `8a757d5e`, not the
per-task implementation commits. The original implementation commits (RED
`8bc78cc5`, GREEN `55571675`) were tracked pre-ship via `backlogit update
--commit` and are recorded in "The Fix" and "Files Modified" below.

`backlogit shipment ship 107-S --sha 8a757d5e` succeeded cleanly (exit 0) and
archived all four members in one call — the 106-S "refusing to write archived
artifact without provenance" quirk did **not** recur this time (the `--sha`
provenance stamp satisfied it).

## The Fix

- **Bug**: `docline.collectInScopeDocs` walked an absolute base (from
  `core.SafeResolve`) but computed `filepath.Rel(root, p)` with the raw,
  possibly-relative `root`. Under the MCP server default `RootPath == "."`,
  `filepath.Rel(".", absPath)` errors on Windows:
  `can't make C:\Source\GitHub\backlogit relative to "."`.
- **Fix** (`internal/docline/service.go`): absolutize the Rel base via
  `absRoot, err := filepath.Abs(root)` after the `SafeResolve` block and use
  `filepath.Rel(absRoot, p)` in the WalkDir callback. Behaviorally equivalent for
  absolute-root callers (`Abs` returns the same logical path; `Rel` cleans
  internally); mirrors the existing `ValidateApplyPath` idiom.
- **Test** (`internal/docline/service_test.go`):
  `TestCollectInScopeDocs_RelativeRootDoesNotErrorOnRel` — parallel-safe,
  derives a relative root, asserts no `Rel` error.

## Files Modified

- `internal/docline/service.go` — the production fix (GREEN, `55571675`).
- `internal/docline/service_test.go` — the failing-then-passing test (RED,
  `8bc78cc5`).
- `.backlogit/` — lifecycle transitions (claim, task/feature done, ship).
- `docs/compound/2026-07-27-absolutize-filepath-rel-base-for-absolute-walked-paths.md`
  — graduated compound learning (this closure).
- `docs/memory/2026-07-27/107-S-post-merge-closure-memory.md` — this file.

## Runtime Verification

Definitive proof on the real Windows repo (`C:\Source\GitHub\backlogit`):

- **CLI**: `go run ./cmd/backlogit docs lint` and `... docs lint --path docs` →
  `valid: true, 0 violations` (CLI already absolutized; confirms no regression).
- **MCP** (the actual bug path, `RootPath == "."`): drove the stdio server with
  `backlogit_docs_lint` → `{"valid":true,"violation_count":0,"findings":[]}` —
  no `Rel` error. The defect is fixed end-to-end at the exact failing condition.

## Quality Gates (Ship PR #303)

All four, in order, passed: `go test ./...` (all `ok`), `go vet ./...`,
`golangci-lint run`, `gofmt -l .` (changed files clean; repo-wide list is
pre-existing Windows CRLF noise, unaffected on Linux CI). P-008 `make md-lint`:
0 issues in 1846 files.

## Decisions

- **Right-sized ceremony**: a small, well-understood bug with a known fix
  direction — no deliberation; straight to a concise impl-plan (Stage) and a
  two-task RED/GREEN shipment (Ship).
- **Fix direction**: absolutize the base (not "make both relative") to match the
  already-absolute walked paths and the sibling `ValidateApplyPath` idiom.
- **Compound supersession scan**: three compound docs mention Windows/absolute
  paths (`2026-07-06-ancestor-aware-shipment-gate-staleness`,
  `2026-07-06-exec-binary-config-must-be-bare-path-validated`,
  `2026-07-07-empty-head-fail-closed-repo-presence-probe`) but none concerns
  `filepath.Rel` relativization — the new learning is novel; nothing superseded.

## Next Steps

- Await operator approval to merge the closure PR (`chore/close-107-s` → `main`).
  P-014: the closure PR needs its own approval; the PR #303 approval does not
  carry over. STOP before merge.
- No follow-up stashes recorded — the fix was complete and self-contained.
