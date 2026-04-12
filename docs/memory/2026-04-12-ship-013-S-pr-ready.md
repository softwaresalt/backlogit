---
title: "Ship 013-S: Correctness & Safety Fixes - SHIPPED"
description: "Shipment 013-S: merged at aeee58e, all items archived, post-merge closure complete"
ms.date: 2026-04-12
shipment_id: 013-S
---

## Shipment Status

- **ID**: 013-S
- **Title**: Correctness & Safety Fixes
- **Status**: shipped → archived
- **Branch**: ship/013-S-correctness-safety-fixes
- **PR**: https://github.com/softwaresalt/backlogit/pull/29 (merged)
- **Merge SHA**: `aeee58e`
- **Commits**: `170ae28` (implementation), `4c80ccd` (wave 1 review fixes), `573b8a9` (wave 2 review fixes)

## Completed Items

| Item ID | Title | Status |
|---|---|---|
| 029-F | Correctness & Safety Fixes (parent feature) | archived |
| 029.001-T | Constrain export_command_map path to .backlogit/ | archived |
| 029.002-T | Atomic adopt item with ID rewrite | archived |
| 029.003-T | Fix query gate semicolons and section-write error handling | archived |
| 029.004-T | Fix stash harvest TOCTOU and terminal status gaps | archived |
| 029.005-T | Fix stale index.db references in instructions | archived |

## Build Verification

- `go build ./...` passed (all 3 commits)
- `go test ./...` all 16 packages pass (all 3 commits)
- `go vet ./...` clean
- CI: test (1.23) ✅ SUCCESS, test (1.24) ✅ SUCCESS (all 3 commits)

## Copilot Review Remediation

### Wave 1 (commit 4c80ccd) — 6 findings

| # | File | Finding | Fix |
|---|---|---|---|
| 1 | queries.go | UpsertItemTx missing columns/formatting | Aligned with UpsertItem |
| 2 | gate.go | Inline regex + brittle index check | Extracted semicolonGuard var |
| 3 | gate.go | Unicode smart-quote in comment | Fixed to ASCII |
| 4 | tools.go | No whitespace validation for section names | Added ContainsAny check |
| 5 | lifecycle.go | Hard-coded newID+".md" | Uses ResolveFileName |
| 6 | lifecycle.go | Ancillary references not rewritten | Added RewriteAncillaryReferences |

### Wave 2 (commit 573b8a9) — 10 findings

| # | File | Finding | Fix |
|---|---|---|---|
| 7 | tools.go | Non-deterministic map iteration | Sorted section names |
| 8 | lifecycle.go | log_path stored as absolute, not relative | Fixed to .backlogit/-relative |
| 9 | queries.go | Comment says delete+insert; unused param | Fixed comment, removed oldLogPath |
| 10 | queue.go | Comment lists only (done, accepted) | Lists full TerminalStatuses |
| 11 | tools.go | No regression test for mixed sections | Added test |
| 12 | test.go | HasPrefix false positive risk | Uses filepath.Rel |
| 13-15 | docs/ | False positive: || table formatting | Confirmed no || on disk |
| 16 | lifecycle.go | Markdown frontmatter edge rewrite | Acknowledged as follow-up |

## Closure

- Operational closure: `docs/closure/2026-04-12-013-S-correctness-safety-fixes.md`
- No documentation updates needed (changes are internal bug fixes)
- Follow-up filed: cross-artifact Markdown frontmatter reference rewrite on adoption
