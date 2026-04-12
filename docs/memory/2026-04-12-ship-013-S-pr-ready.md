---
title: "Ship 013-S: Correctness & Safety Fixes - CI Green, Awaiting Merge"
description: "Shipment 013-S: all Copilot review findings fixed, CI green, PR #29 awaiting merge approval"
ms.date: 2026-04-12
shipment_id: 013-S
---

## Shipment Status

- **ID**: 013-S
- **Title**: Correctness & Safety Fixes
- **Status**: active (PR created, CI green, Copilot review addressed, awaiting user merge approval)
- **Branch**: ship/013-S-correctness-safety-fixes
- **PR**: https://github.com/softwaresalt/backlogit/pull/29
- **Commits**: `170ae28` (implementation), `4c80ccd` (Copilot review fixes)

## Completed Items

| Item ID | Title | Status |
|---|---|---|
| 029-F | Correctness & Safety Fixes (parent feature) | In shipment |
| 029.001-T | Constrain export_command_map path to .backlogit/ | Implemented |
| 029.002-T | Atomic adopt item with ID rewrite | Implemented |
| 029.003-T | Fix query gate semicolons and section-write error handling | Implemented |
| 029.004-T | Fix stash harvest TOCTOU and terminal status gaps | Implemented |
| 029.005-T | Fix stale index.db references in instructions | Implemented |

## Build Verification

- `go build ./...` passed
- `go test ./...` all 15 packages pass (both commits)
- `go vet ./...` clean
- CI: test (1.23) ✅ SUCCESS, test (1.24) ✅ SUCCESS

## Copilot Review Remediation (commit 4c80ccd)

All 6 Copilot comments addressed and replied to:

| # | Comment ID | File | Finding | Fix |
|---|---|---|---|---|
| 1 | 3070117102 | queries.go | UpsertItemTx missing columns/formatting | Aligned with UpsertItem: hierarchy_path, RFC3339Nano, NULL-safe JSON |
| 2 | 3070117113 | gate.go | Inline regex + brittle index check | Extracted semicolonGuard var, decoupled from loop index |
| 3 | 3070117118 | gate.go | Unicode smart-quote in comment | Fixed to ASCII representation of SQL escaped quotes |
| 4 | 3070117133 | tools.go | No whitespace validation for section names | Added strings.ContainsAny check for space/tab |
| 5 | 3070117140 | lifecycle.go | Hard-coded newID+".md" for filename | Now uses ResolveFileName with file_name_format config |
| 6 | 3070117143 | lifecycle.go | Ancillary references not rewritten | Added RewriteAncillaryReferences and wired into AdoptItem tx |

## Review Gate Findings

- P2 (advisory): AdoptItem crash-inconsistency window between file renames and
  DB commit. Mitigated by rehydration architecture (DB is ephemeral cache,
  Markdown files are source of truth). Current ordering follows the reviewed
  execution plan: reversible DB tx before harder-to-reverse file ops.

## Decisions

1. Followed execution plan ordering for AdoptItem: DB tx → file ops → commit
   (per reviewed plan, not commit-then-compensate)
2. Extended scope of index.db fix to include copilot-instructions.md and
   go-mcp-expert.agent.md (2 additional references found)
3. Used NextTypedHierarchicalID for adoption ID generation to match CreateArtifact pattern

## Next Steps

- Awaiting user merge approval for PR #29
- Post-merge: run operational-closure skill, update docs, write final memory
