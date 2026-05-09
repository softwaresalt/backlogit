---
title: "Model-Aware Telemetry"
date: 2026-05-08
origin: "stash:5F0AAB28,stash:B68AED87,stash:6646ACA1"
status: draft
---

# Model-Aware Telemetry

## Problem Frame

Telemetry captures `TokensByModel` per session (a `map[string]int` of model name to token count), but this data is never surfaced in CLI output. Operators cannot see which models drove token consumption without querying raw JSONL. Additionally, exact model version strings (e.g. `claude-sonnet-4.6`) are too granular for trend analysis — operators need model-class grouping (e.g. `sonnet`, `opus`, `gpt`) and, for OpenAI reasoning models, a reasoning-level indicator.

Three stash entries form a layered capability:

1. **Model class derivation** (foundational) — parse model names to a class string, store in `SessionSummaryRecord`
2. **Model name surfacing** — show `PRIMARY_MODEL` in list/report views, add `--by model` report dimension
3. **Reasoning level derivation** — map OpenAI o-series model names to reasoning level indicators

### Scope Boundary

In scope: model classification, model-name display, reasoning-level heuristic. Out of scope: ghost session filtering and call-rate columns (covered by the Telemetry Accuracy plan).

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Derive `ModelClass` from model name string during harvest and store in `SessionSummaryRecord` | stash:5F0AAB28 |
| R2 | `ModelClass` derivation covers Anthropic (sonnet, haiku, opus), OpenAI (gpt, o3, o4), and unknown fallback | stash:5F0AAB28 |
| R3 | `--by class` option in `telemetry report` aggregates by model class | stash:5F0AAB28 |
| R4 | Model class column in `telemetry trend` output | stash:5F0AAB28 |
| R5 | `PRIMARY_MODEL` column in `telemetry list` and `telemetry report --by session` showing the model with highest token share | stash:B68AED87 |
| R6 | `--by model` option in `telemetry report` aggregates by model name | stash:B68AED87 |
| R7 | Derive `ReasoningLevel` from OpenAI o-series model names (`high` for o1/o3/o4, `low` for o3-mini/o4-mini) | stash:6646ACA1 |
| R8 | Non-reasoning models and Anthropic models have empty `ReasoningLevel` | stash:6646ACA1 |

## Scope Boundaries

### In Scope

- `DeriveModelClass` function with string-matching rules
- `DeriveReasoningLevel` function for OpenAI o-series models
- `ModelClass` and `ReasoningLevel` fields on `SessionSummaryRecord`
- `PrimaryModel` helper function (returns model with highest token share from `TokensByModel`)
- `PRIMARY_MODEL` column in session table and markdown outputs
- `--by model` and `--by class` report group-by options
- Unit tests for all new functions and output formats

### Non-Goals

- Changing the harvest log parser or `ModelCall` struct
- Adding model class to the `items` or `stash_entries` tables
- Querying external APIs for model metadata
- Supporting arbitrary provider model name formats beyond Anthropic and OpenAI

### Deferred to Implementation

- Exact model-class mapping rules for edge cases (implementation should use a switch/case with clear fallback)

## Implementation Units

### Unit 1: Model Class and Reasoning Level Derivation Functions

**Files:** `internal/telemetry/records.go`
**Test files:** `internal/telemetry/records_test.go` (new if not exists, or add to existing)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `ValidateSessionSummary` in `validate.go` for standalone predicate pattern
**Dependencies:** none

**Approach:**

1. Add `DeriveModelClass(model string) string` to `records.go`. Rules:
   - Contains `sonnet` → `"sonnet"`
   - Contains `haiku` → `"haiku"`
   - Contains `opus` → `"opus"`
   - Starts with `gpt` → `"gpt"`
   - Starts with `o1` or `o3` or `o4` → `"o-series"`
   - Fallback → `"other"`

2. Add `DeriveReasoningLevel(model string) string` to `records.go`. Rules:
   - Matches `o1`, `o3`, `o4` (not containing `-mini`) → `"high"`
   - Matches `o1-mini`, `o3-mini`, `o4-mini` → `"low"`
   - All other models → `""` (empty)

3. Add `PrimaryModel(tokensByModel map[string]int) string` helper — returns the model key with the highest token count, or `""` if the map is empty.

4. Add `ModelClass` and `ReasoningLevel` fields to `SessionSummaryRecord`:
   ```go
   ModelClass     string `json:"model_class,omitempty"`
   ReasoningLevel string `json:"reasoning_level,omitempty"`
   ```

**Verification:**

- Test: `DeriveModelClass` with known Anthropic and OpenAI model strings
- Test: `DeriveModelClass` returns `"other"` for unknown models
- Test: `DeriveReasoningLevel` returns correct levels for o-series models
- Test: `DeriveReasoningLevel` returns `""` for non-reasoning models
- Test: `PrimaryModel` returns model with highest token count
- Test: `PrimaryModel` returns `""` for empty map

### Unit 2: Populate Model Class and Reasoning Level During Harvest

**Files:** `internal/telemetry/harvest.go`
**Test files:** `internal/telemetry/harvest_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `writeTelemetryJSONL` record construction in `harvest.go`
**Dependencies:** Unit 1 (uses `DeriveModelClass`, `DeriveReasoningLevel`, `PrimaryModel`)

**Approach:**

1. In `writeTelemetryJSONL` (or the record-building step before it), after constructing `SessionSummaryRecord`, call `PrimaryModel(s.TokensByModel)` to get the dominant model, then `DeriveModelClass` and `DeriveReasoningLevel` on it.
2. Set `ModelClass` and `ReasoningLevel` on the record before writing to JSONL.
3. For existing harvested data, these fields will be empty until a `--force` re-harvest. This is acceptable — the fields are `omitempty`.

**Verification:**

- Test: harvest of sessions with known model names produces correct `ModelClass` and `ReasoningLevel` in JSONL records
- Test: sessions with no model calls produce empty `ModelClass` and `ReasoningLevel`

### Unit 3: Surface PRIMARY_MODEL in List and Report Session Views

**Files:** `internal/telemetry/reporter.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `formatSessionTable` column layout (reporter.go:165-181)
**Dependencies:** Unit 1 (uses `PrimaryModel`)

**Approach:**

1. In `formatSessionTable`, add a `PRIMARY_MODEL` column. Compute it via `PrimaryModel(s.TokensByModel)` for each row. Display `"-"` when empty.
2. In `formatSessionMarkdown`, add a `Primary Model` column.
3. Truncate long model names to 20 characters for table display to prevent column overflow.

**Verification:**

- Test: session table output includes `PRIMARY_MODEL` column with correct values
- Test: session with multiple models shows the one with highest token count
- Test: session with no models shows `"-"`
- Test: markdown output includes the column

### Unit 4: Add `--by model` and `--by class` Report Dimensions

**Files:** `internal/telemetry/reporter.go`, `internal/cli/telemetry.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `--by server` group-by in `GenerateReport` (reporter.go:74-84) and `--by branch` in `GenerateTrendReport`
**Dependencies:** Units 1, 2 (needs `ModelClass` populated in records)

**Approach:**

1. Extend `ReportOptions.GroupBy` validation to accept `"model"` and `"class"` in addition to `"session"` and `"server"`.
2. For `--by model`: aggregate sessions by `PrimaryModel(s.TokensByModel)`. Produce rows with model name, total tokens, session count, avg tokens/session.
3. For `--by class`: aggregate sessions by `ModelClass`. Same output structure.
4. Add formatting functions `formatModelTable`, `formatModelMarkdown` following the pattern of `formatServerTable`.
5. Update CLI flag help text for `--by` in `newTelemetryReportCmd` to include `model` and `class`.

**Verification:**

- Test: `--by model` produces one row per distinct primary model
- Test: `--by class` produces one row per distinct model class
- Test: JSON output marshals correctly for both new group-by values
- Test: invalid `--by` value still returns error

### Unit 5: Add `--by class` Grouping to Trend Report

**Files:** `internal/telemetry/reporter.go`, `internal/cli/telemetry.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `--by branch` grouping in `GenerateTrendReport` (reporter.go:362-398)
**Dependencies:** Units 1, 2 (needs `ModelClass` populated in records)

**Approach:**

1. Extend `TrendOptions.By` validation to accept `"class"` in addition to `"date"` and `"branch"`.
2. In `GenerateTrendReport` aggregation loop, when `by == "class"`, use `s.ModelClass` as the group key (falling back to `DeriveModelClass(PrimaryModel(s.TokensByModel))` when `ModelClass` is empty for pre-existing records).
3. Update CLI flag help text for `--by` in `newTelemetryTrendCmd` to include `class`.
4. Reuse existing trend formatters — the `GROUP` column naturally holds the class name.

**Verification:**

- Test: `telemetry trend --by class` groups sessions by model class
- Test: records with empty `ModelClass` derive it on-the-fly
- Test: invalid `--by` value still returns error

## Dependency Graph

```text
Unit 1 (derivation functions + record fields)
  ├── Unit 2 (populate during harvest)
  ├── Unit 3 (PRIMARY_MODEL display — uses PrimaryModel helper)
  ├── Unit 4 (--by model/class report dimensions — needs ModelClass populated)
  └── Unit 5 (--by class trend grouping — needs ModelClass populated)
```

Unit 1 is foundational. Units 2–5 depend on Unit 1. Units 2 and 3 are independent of each other. Units 4 and 5 benefit from Unit 2 (to have `ModelClass` in records) but can derive on-the-fly from `TokensByModel` when records lack the field.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Derive model class from model name string, not from an external lookup table | Model names follow predictable patterns; an external registry adds unnecessary complexity | Config-file-based model registry — over-engineered for the current model landscape |
| D2 | Use `PrimaryModel` (highest token share) rather than first/last model | Token share reflects actual cost driver, not an arbitrary ordering | Alphabetical first model — meaningless; list all models — too wide for table columns |
| D3 | `ReasoningLevel` only for OpenAI o-series, empty for all others | Anthropic does not expose reasoning as a model-name dimension; inventing one would be misleading | Heuristic-based reasoning detection for all providers — no reliable signal |
| D4 | Add `ModelClass` and `ReasoningLevel` to `SessionSummaryRecord` (JSONL schema) | Avoids re-deriving on every report render; makes the data queryable via SQL after rehydration | Derive at display time only — would prevent SQL queries and add latency on large JSONL |

## Risks and Caveats

- **Model name format changes**: If Anthropic or OpenAI change their naming conventions, `DeriveModelClass` will need updating. The `"other"` fallback prevents crashes.
- **JSONL schema evolution**: Adding `model_class` and `reasoning_level` to JSONL records is additive (`omitempty`). Existing records without these fields will unmarshal with zero values. A `--force` re-harvest populates them for all sessions.
- **SQLite rehydration**: The `telemetry_sessions` table schema may need new columns. Check `EnsureTelemetrySchema` in `internal/db/` during implementation.

## Plan Hardening Signals (REQUIRED)

- Public API, schema, or contract change: **Yes** — `SessionSummaryRecord` gains two new fields; JSONL schema expands; `telemetry report --by` gains two new values. All additive.
- Security, auth, permission, or compliance-sensitive behavior: **No**
- Migration, backfill, destructive data/config action, or irreversible step: **No** — old JSONL records remain valid; `--force` re-harvest is optional.
- External integration, operator checkpoint, or external dependency: **No**
- High runtime, rollout, or rollback risk: **No**

Requires plan hardening: **no** — all changes are additive with no data migration or destructive actions.

## Runtime Verification and Closure

- **Changed runtime surface**: `backlogit telemetry list`, `backlogit telemetry report`, `backlogit telemetry harvest`
- **Verification**: After `--force` harvest, run `telemetry list` and confirm `PRIMARY_MODEL` column appears. Run `telemetry report --by model` and `--by class` and confirm grouping works. Verify JSONL records contain `model_class` and `reasoning_level`.
- **Closure**: No monitoring or rollback needed — CLI display changes and additive JSONL schema.

## Learnings Applied

No directly applicable learnings from `docs/compound/` — this is a new feature area. The `2026-05-07-build-attributor-pattern.md` learning about the attribution registry pattern may be useful as a reference for the string-matching approach in `DeriveModelClass`.

## Standards Check

- Go quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
- Test-first development per constitution Principle III
- Struct fields use typed Go with JSON tags per Principle I
- JSONL schema evolution is additive per Principle VIII (Git-friendly persistence)
- No new external dependencies required

## Plan Review

**Gate Decision: PASS**

**Date:** 2026-05-08
**Reviewers:** constitution-reviewer, go-quality-reviewer, scope-boundary-auditor, learnings-researcher

### Summary

4 personas reviewed. 1 P1 finding resolved during review (Unit 5 added), 1 P2 advisory remaining. No P0 findings.

### P0 — Critical

None

### P1 — High

**F1 (scope-boundary-auditor) — RESOLVED:** Requirement R4 ("Model class column in telemetry trend output") had no implementation unit. The plan covered `--by class` for the report command (Unit 4) but not for the trend command. **Resolution:** Unit 5 added to extend `GenerateTrendReport` with `--by class` grouping support.

### P2 — Moderate

**F2 (go-quality-reviewer):** Unit 4 and Unit 5 should handle the case where `ModelClass` is empty in pre-existing JSONL records (not yet re-harvested with `--force`). When `--by class` is used, records with empty `ModelClass` should derive it on-the-fly from `PrimaryModel(TokensByModel)` to avoid a confusing empty-string group in output. Unit 5 approach already notes this fallback; implementers should apply the same pattern in Unit 4.

### P3 — Low (advisory)

None

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F1 | scope-boundary-auditor | claude-opus-4.6 |
| F2 | go-quality-reviewer | claude-opus-4.6 |

### Next Steps

Plan passes review (P1 resolved, P2 advisory noted for implementers). Proceed to `harvest` to decompose into backlogit work items.
