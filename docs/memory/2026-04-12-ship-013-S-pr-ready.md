---
title: "Ship 013-S: Correctness & Safety Fixes - PR Ready"
description: "Shipment 013-S execution complete, PR #29 awaiting merge approval"
ms.date: 2026-04-12
shipment_id: 013-S
---

## Shipment Status

- **ID**: 013-S
- **Title**: Correctness & Safety Fixes
- **Status**: active (PR created, awaiting user merge approval)
- **Branch**: ship/013-S-correctness-safety-fixes
- **PR**: https://github.com/softwaresalt/backlogit/pull/29

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
- `go test ./...` all packages pass
- `go vet ./...` clean
- All new tests pass (25 targeted tests)
- Zero remaining `index.db` references in .github/

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
- Post-merge: run operational-closure skill, update docs
