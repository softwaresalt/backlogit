---
chunk_strategy: h1-h2-h3
description: Spike evaluating feature 046-F and current telemetry capabilities against trend analysis, context-window analytics, and per-tool token attribution goals
doc_type: decision
docline:
    conclusion: pivot
    confidence: high
    date: 2026-05-05T00:00:00Z
    linked_parent_work_item: 046-F
    promoted_to:
        - none
    tags:
        - telemetry
        - copilot-cli
        - token-analytics
        - context-window
    time_box: 2h
    type: spike
ingested_at: "2026-06-26T02:33:47Z"
schema_version: "1.0"
source: docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md
title: Telemetry gap analysis for token-efficiency trends
---

## Goal

Determine whether feature `046-F` and backlogit's current telemetry system are sufficient to analyze `.copilot` session logs for token and context-window trends over time, identify which tools are used, estimate how much those tools contribute to context-window exhaustion, and support empirical comparisons of tools such as engram, graphtor, and backlogit.

## Success Criteria

* Evaluate what `046-F` does and does not cover.
* Inventory the telemetry data backlogit currently harvests, stores, and reports.
* Identify the gaps that block reliable time-series analysis, tool attribution, and before-versus-after efficiency comparisons.
* Produce a concrete recommendation for the next telemetry work beyond `046-F`.

## Scope Constraints

* Read-only investigation only.
* Evaluate repository source, current CLI behavior, and harvested telemetry artifacts.
* Do not implement telemetry changes in this spike.
* Do not redesign unrelated backlog or MCP surfaces.

## Investigation Approach

1. Review `046-F` and its reviewed plan to establish the intended telemetry scope.
2. Inspect the telemetry parser, recorder, reporter, attribution, and SQLite rehydration code.
3. Inspect the current telemetry CLI outputs, SQLite tables, and harvested JSONL data.
4. Compare the current system to the target outcome of trend analysis and tool-efficiency measurement.

## Findings

### What Was Discovered

1. `046-F` is necessary, but it is not sufficient for your goal. Its reviewed plan is explicitly limited to a parser fix, a `telemetry top` ranking fix, and two documentation gaps. It explicitly excludes adding new telemetry metrics or new context-window fields.

2. The current telemetry pipeline already harvests useful session-level data. The persisted session summary includes total, prompt, completion, and cached tokens, model and tool call counts, `tokens_by_model`, `tool_calls_by_server`, `tokens_per_task`, and derived context-window metrics such as `peak_utilization`, `remaining_capacity`, `depletion_rate`, and `max_context_tokens`.

3. The persisted history is too lossy for trend analysis inside a session. `SessionMeta` carries `StartedAt` and full compaction event detail during correlation, but `SessionSummaryRecord` persists only `CompactionCount` and does not persist `StartedAt`, per-turn timestamps, or the per-event compaction details. This means you can see a session peak, but not the context-window curve that led to it.

4. Tool attribution is only partial and only trustworthy at the server level in limited cases. `ToolUsageRecord` stores `session_id`, `server_name`, `tool_name`, `call_count`, and duration, but no prompt-token, completion-token, or total-token attribution per tool. The code still has `model_call_id` on raw `ToolCall`, but that linkage is discarded before persistence.

5. The current attribution registry is incomplete for the workflow you care about. The hardcoded prefix map covers some builtins and MCP prefixes, but it does not cover several tools observed in harvested data, including `powershell`, `sql`, `task`, `read_agent`, and hashed tool names. The current dataset shows `unknown` accounting for 1,568 calls, slightly more than `copilot_builtin` at 1,559 calls, while only `github` appears beyond that. This blocks any credible comparison of engram, graphtor, or backlogit because the attribution coverage is not complete enough.

6. The current harvested history is still noisy. The live telemetry tables contain 58 sessions, but 51 have `total_tokens = 0` and `tool_calls = 0`, and only 7 have any context-window metrics. That makes historical trend analysis and percentile-style comparisons unreliable without cleanup or a full re-harvest after the data-quality fixes land.

7. The current reporting surface is narrower than your goal. `telemetry report` can emit JSON and group by `session` or `server`, but `telemetry list` is table-only, `telemetry top` is table-only, and there is no first-class grouping by tool, model, branch, task, day, week, or capability pack. There is also no reporting filter for time windows other than harvest-time slicing during ingestion.

8. There is a runtime-versus-source discrepancy around `telemetry top`. The checked-in `internal/telemetry/reporter.go` computes proportional token attribution by server, but the current executable output still shows server call counts, not token estimates. That means either the local binary is stale or the feature is not actually delivered in the runtime you are using. Either way, the capability is not dependable yet.

9. backlogit can already help you build historical metric data over time, but not the experiment-grade evidence you want. The current model can answer broad questions such as total tokens per session, rough server usage, and whether compaction happened. It cannot yet answer questions such as which specific tool consumed the most prompt budget in a session, which tool mix drove peak context utilization, or whether engram or backlogit reduced prompt growth relative to a comparable baseline.

### What Was Tried and Failed

* Attempted to use `backlogit telemetry list --format json`, but the subcommand has no format flag even though machine-readable reporting exists on `telemetry report`.
* Expected `backlogit telemetry top` to reflect token-based ranking, but the current executable still returned server call counts. That prevented direct validation of the intended `046-F` behavior at runtime.

### Remaining Unknowns

* Whether the `telemetry top` mismatch is caused by a stale binary or by incomplete implementation delivery.
* What graphtor tool names look like in harvested logs, since no graphtor attribution mapping is present in the current registry.
* Which attribution model will be most defensible for per-tool token accounting: proportional allocation by model-call descendants, exact log-derived token binding if available, or a hybrid model.

## Recommendation

**Conclusion**: pivot
**Confidence**: high

Ship `046-F` as a data-quality prerequisite, but treat it as only the first layer. The next telemetry feature should pivot from parser and docs repair to attribution and history analytics.

The highest-value follow-on work is:

1. Persist a real time axis for analysis. Store `started_at`, an end or last-seen timestamp, and per-model-call timestamps so sessions can be trended by day, week, branch, and experiment window.
2. Preserve detailed context-window evidence. Persist per-model-call prompt and total tokens, plus compaction events with their token counts, instead of reducing them to session-level peaks and counts.
3. Add per-tool token attribution. Keep `model_call_id` or another join key through persistence so prompt and completion tokens can be apportioned to tools or tool groups within a session.
4. Make attribution coverage configurable and complete. Replace the hardcoded-only prefix registry with a configurable mapping that covers builtins, MCP servers, graphtor, engram, backlogit, and any runtime-specific tool names observed in logs.
5. Add history and comparison reports. Provide grouping and filtering by tool, server, model, task, branch, and time window, plus comparison outputs that can answer questions such as "sessions with engram versus without engram" or "before backlogit rollout versus after backlogit rollout".
6. Re-harvest and backfill after the parser and rehydration fixes are confirmed. The current dataset has too many zero-token sessions to serve as a trustworthy baseline.

With those additions, backlogit could move from coarse telemetry to evidence you can use to show that tool-assisted workflows reduce prompt growth, delay compaction, and improve token efficiency over time.

## Next Steps

1. Finish and verify `046-F`, then force-reharvest telemetry so the historical baseline is clean.
2. Create a follow-on feature for telemetry attribution and history analytics, linked to `046-F`.
3. Scope the follow-on feature into separate tasks for persistence, attribution mapping, reporting, and historical comparison queries.
4. Use the cleaned dataset to define one or two baseline comparison views, such as sessions with `engram` usage versus sessions without it.

## References

* `docs/exec-plans/2026-04-25-telemetry-quality-plan.md`
* `internal/telemetry/parser.go`
* `internal/telemetry/records.go`
* `internal/telemetry/reporter.go`
* `internal/telemetry/context_window.go`
* `internal/telemetry/types.go`
* `internal/telemetry/attribution.go`
* `internal/telemetry/harvest.go`
* `internal/cli/telemetry.go`
* `internal/db/telemetry_schema.go`
* `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`
