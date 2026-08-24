---
chunk_strategy: h1-h2-h3
description: "Ship session memory for 129-S formal closure — repair-evidence implementation and shipment ship"
doc_type: memory
docline:
    date: 2026-08-24T00:00:00Z
    status: accepted
    tags:
        - session-memory
        - 129-S
        - 146-F
        - repair-evidence
schema_version: "1.0"
source: docs/memory/2026-08-24/ship-129-s-closure-memory.md
title: "Ship Session Memory — 129-S Closure"
---

# Ship Session Memory — 129-S Closure

**Date**: 2026-08-24
**Session scope**: Formally close shipment 129-S (feature 146-F, 23 tasks)
**Dark mode**: DARK_MODE_ACTIVE under P-017

## Summary

Shipment 129-S was formally closed by:
1. Implementing a new `backlogit shipment repair-evidence` command (PR #375)
2. Running the repair on `146.006-T`'s stale gate evidence SHA
3. Running `backlogit shipment ship 129-S` successfully

## Root Cause of Prior Blocker

`146.006-T`'s `pre_task_completion_gate_passed` evidence recorded
`head_sha: 52f1983516a65ef2d782f75d137426ae5a63dc21`, which was orphaned by
a git history rewrite during PR #373's 13-round review-fix cycle. The commit
object still existed locally but was not an ancestor of `main`, causing
`validateMemberGateEvidence` to fail with "gate evidence is stale (recorded
at a divergent head)".

## Files Modified / Created

| File | Change |
|---|---|
| `internal/core/shipment_repair.go` | NEW: `RepairShipmentMemberEvidence` |
| `internal/core/shipment_repair_test.go` | NEW: 9 TDD tests |
| `internal/cli/shipment.go` | Added `repair-evidence` subcommand |
| `.autoharness/backlog-registry.yaml` | Added `repair_member_evidence` CLI-only op |
| `docs/cli-reference/backlogit_shipment*.md` | Regenerated reference docs |
| `.backlogit/archive/129-S.md` | MOVED from queue/archive |
| `.backlogit/archive/146-F.md` | Updated with shipped commit SHA |
| `.backlogit/archive/146.*.md` | All 23 tasks updated with shipped SHA |
| `.backlogit/logs/146.006-T.jsonl` | Repair event appended |

## Key Commits

| SHA | Description |
|---|---|
| `4579f45c` | feat(core): add shipment repair-evidence command |
| `90911050` | fix(core): restrict repair-evidence to task/subtask members only |
| `f00fb5d0` | docs(cli): regenerate CLI reference |
| `ea0b3724` | fix(core): address Copilot review (3 findings: MCP removal, stale-evidence check, formal-gate guard) |
| `5e86657e` | Merge PR #375 (merge commit on main) |
| `d182b119` | chore: archive 129-S backlog artifacts |

## Decisions

1. **CLI-only**: `repair-evidence` follows the existing `force_cli_only` contract — no MCP tool exposed
2. **Stale-evidence guard**: Only repairs if existing evidence is confirmed stale (not missing, not current)
3. **Formal-gate guard**: Fails early when formal gate enforcement is active (EventGateForced not admissible under FormalAdmit)
4. **Repair event type**: Uses `EventGateForced` with `repair: true` marker — matches existing break-glass contract, no new event type needed

## Shipment Result

```text
{
  "shipment_id": "129-S",
  "shipment_status": "shipped",
  "archived_ids": ["146.001-T", "146.002-T", "146.004-T", "146.005-T",
    "146.006-T", "146.007-T", "146.008-T", "146.009-T", "146.010-T",
    "146.011-T", "146.012-T", "146.013-T", "146.014-T", "146.015-T",
    "146.016-T", "146.017-T", "146.018-T", "146.019-T", "146.020-T",
    "146.021-T", "146.022-T", "146.023-T", "146.024-T", "129-S", "146-F"],
  "commit_sha": "15ab30a2a394439f52e5338fc94d1c50e3f395ae"
}
```

## Open Items

- Remaining follow-up stash items from 146-F/129-S scope are independent (`D3CE9E81`, `EA1F5912`, etc.)
- Closure PR #376 (post-merge/129-s-ship-closure) needs operator review and merge

## Completed This Session

- Stash `DD957688` archived (blocker resolved by repair-evidence + ship)

## Next Steps for Operator

1. Review and merge closure PR (on `post-merge/129-s-ship-closure` branch)
2. Stage agent can now proceed with DD957688 triage (repair-evidence compound learning and stash harvest)
3. P-001 is cleared — no active release unit remains
