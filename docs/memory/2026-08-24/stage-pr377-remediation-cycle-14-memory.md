---
chunk_strategy: h2
description: "Stage PR #377 remediation cycle 14 — context-duplicate safety, U7/U7d width splits, plan wording fix"
doc_type: memory
schema_version: "1.0"
source: cycle-14-session
title: "PR #377 Cycle 14 Remediation Memory"
---

# PR #377 Cycle 14 Remediation Memory

**Date**: 2026-08-25
**Agent**: Stage
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 14 (operator-authorized)
**Base HEAD**: `6e68ced585f88c56804fd6fbc857f42012bd18e9`

## Backlogit Discovery Evidence

- `backlogit_get_metadata_catalog`: workspace `.backlogit/`, 8 types, 4 statuses
- `backlogit_get_wit_metadata(task)`: confirmed task fields (id, title, status, priority, dependencies, labels, sections)
- `backlogit_list_types`: feature/task/subtask/deliberation/shipment/bug/epic/spike
- `backlogit_list_templates`: feature/task/subtask/deliberation/shipment templates confirmed
- MCP create/dependency/shipment ops failed for worktree artifacts (MCP bound to root workspace); used direct file editing

## Copilot Review Findings Addressed

### A. Context-member duplicate safety (PRRT_kwDORzozKM6b7YzW / 3849365620)

**Resolution**: Created **147.028-T / U2g** — context-member duplicate detection read boundary.
Performs ordered duplicate detection for immediate `context` members before `map[string]json.RawMessage` collapse. Refuses exact-duplicate and case-fold-equivalent member names. 3 scenarios, 2 files.

### B. U7 scenario width (PRRT_kwDORzozKM6b7Yzk / 3849365643)

**Resolution**: Moved domainError safety-net mappings (item 3, scenario 2) from **147.013-T / U7** to new **147.029-T / U7e**. U7 reduced to 3 scenarios, 2 files.

### C. U7d file/scenario width (PRRT_kwDORzozKM6b7Yzv / 3849365657)

**Resolution**: Moved `ErrCheckpointCannotResolveAbandoned` → `validation_failed` mapping (scenario 4, `errors.go`) from **147.025-T / U7d** to **147.029-T / U7e**. U7d reduced to 3 scenarios, 2 files.

### D. U7d wording (PRRT_kwDORzozKM6b7Yz9 / 3849365682)

**Resolution**: Plan U7d section corrected — `validation_failed` is not pre-existing; it is a genuine red delta created by U7e. `domainError` has no explicit case for `ErrCheckpointCannotResolveAbandoned` and falls to `default: InternalError`.

## Decomposition

Combined findings B and C into one U7e task because all three domainError mappings target the same function in the same file. Result: 2 new tasks instead of 3.

| New Task | Unit | Scenarios | Files | Dependencies |
|---|---|---|---|---|
| 147.028-T | U2g | 3 | 2 | U1 (147.001-T), U2b (147.003-T) |
| 147.029-T | U7e | 3 | 2 | U1 (147.001-T) |

## Dependency Rewiring

- 147.007-T (U3b): added dependency on 147.028-T (U2g)
- 147.008-T (U4): added dependency on 147.028-T (U2g)

## Files Modified

| File | Action |
|---|---|
| `.backlogit/queue/147.028-T.md` | Created (U2g) |
| `.backlogit/queue/147.029-T.md` | Created (U7e) |
| `.backlogit/queue/147.013-T.md` | Updated (removed domainError scope → U7e, 3 scenarios) |
| `.backlogit/queue/147.025-T.md` | Updated (removed scenario 4 → U7e, removed errors.go, 3 scenarios) |
| `.backlogit/queue/147.007-T.md` | Updated (added dep on 147.028-T) |
| `.backlogit/queue/147.008-T.md` | Updated (added dep on 147.028-T) |
| `.backlogit/queue/147.018-T.md` | Updated (U2g code enforcement reference for universal invariant) |
| `.backlogit/queue/147-F.md` | Updated (task inventory 26→28) |
| `.backlogit/queue/130-S.md` | Updated (added 147.028-T, 147.029-T; 29 members) |
| `docs/exec-plans/...plan.md` | Updated (U2g/U7e plan units, U7/U7d corrections, cycle-14 remediation) |
| `docs/memory/2026-08-24/stage-pr377-remediation-cycle-14-memory.md` | Created |

## Final Topology

- Queued tasks: 28
- Queued-to-queued edges: 47
- Historical total edges: 48 (47 + 1 archived)
- Shipment members: 29
- Ready set: {147.001-T} (sole root, unchanged)
- Archived: 147.010-T (U5b, unchanged)

## Width Audit (U7-family + context tasks)

| Task | Unit | Scenarios | Files | Compliant |
|---|---|---|---|---|
| 147.013-T | U7 | 3 | 2 | ✓ |
| 147.024-T | U7c | 2 | 2 | ✓ |
| 147.025-T | U7d | 3 | 2 | ✓ |
| 147.028-T | U2g | 3 | 2 | ✓ |
| 147.029-T | U7e | 3 | 2 | ✓ |

## Memory Footprint

- docs/memory: 39 files, 327.5 KiB (below 40-file and 500 KB thresholds)
- Compact-context: NOT required

## Decisions

- Combined B+C domainError splits into single U7e (all 3 mappings in same function/file)
- U2g depends on U2b (context open-namespace semantics) in addition to U1
- U2g blocks U3b and U4 (they must not silently last-wins on duplicate context members)
- MCP worktree limitation: all mutations done via direct file editing

## Next Steps

- Push and reply to Copilot review threads (operator action)
- Ship agent picks up 147.001-T (sole ready root) for implementation
