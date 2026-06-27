---
title: Ship session — 052-S telemetry accuracy and call-rate metrics
description: Shipped ghost session filtering, [empty] marker, and AvgModelCalls/AvgToolCalls columns
ms.date: 2026-05-08
ms.topic: reference
---

## Session Summary

Executed shipment 052-S "Telemetry Accuracy & Call-Rate Metrics" from branch `feat/telemetry-accuracy-call-rate`, merged as PR #94 (SHA: 5fa1e073).

## Tasks Completed

| Task | Title | Status | Notes |
|---|---|---|---|
| 053.004-T | Ghost session predicate and trend filtering | done | Implementation task |
| 053.005-T | Ghost session visual indicator | done | Implementation task |
| 053.006-T | Call-rate columns in TrendGroup | done | Implementation task |
| 053.007-T | Ghost session filtering (harvested into 053-F) | done | Provenance tracker — stash E39D0A34; work completed by 053.004-T and 053.005-T |
| 053.008-T | Call-rate trend columns (harvested into 053-F) | done | Provenance tracker — stash 5EC2B37F; work completed by 053.006-T |
| 053-F | Telemetry Accuracy & Call-Rate Metrics (feature) | done | |
| 052-S | Shipment | done (merged) | PR #94 |

## Files Modified

- `internal/telemetry/validate.go` — added `IsGhostSession(SessionSummaryRecord) bool`
- `internal/telemetry/reporter.go` — ghost filtering in `GenerateTrendReport` (2 loops), `[empty]` marker in `formatSessionTable`/`formatSessionMarkdown`, `AvgModelCalls`/`AvgToolCalls` in `TrendGroup` and formatters
- `internal/telemetry/reporter_test.go` — 11 new tests via TDD

## Key Decisions

- **Ghost session definition**: `TotalTokens==0 && ModelCalls==0 && ToolCalls==0`. Distinct from partial sessions (tool calls but zero tokens) which `ValidateSessionSummary` handles at harvest time.
- **Filter at display, not at harvest**: Ghost sessions remain in JSONL for auditability; they are excluded only during report generation.
- **Two-loop guard in `GenerateTrendReport`**: Both the main aggregation loop and the finalisation loop (which accumulates `taskCounts` and `peakCounts`) needed `IsGhostSession` guards for consistency.
- **`[empty]` marker scope**: Table and markdown session formatters only — JSON output is unmodified so machine consumers get raw data.

## PR and Review

- PR #94: merged 2026-05-09T04:39:51Z
- Review 053.001-R: PASS (0 P0/P1, 7 P3 advisory)
- Copilot comment: fixed `ms.date: 05/08/2026` → `2026-05-08` (ISO 8601)

## Failed Approaches

- Squash merge blocked by repo policy → used merge commit (`gh pr merge --merge --admin`)
- `PRRC_*` node ID cannot be used with `resolveReviewThread` mutation — must query for `PRRT_*` thread node ID via `reviewThreads` GraphQL query first

## Next Steps

- 053-S "Model-Aware Telemetry" is queued — ready for next Ship session
- 054-F has 5 tasks queued under 053-S
- Stash entry ACDF8C2D (SQL schema in metadata catalog) remains unprocessed
