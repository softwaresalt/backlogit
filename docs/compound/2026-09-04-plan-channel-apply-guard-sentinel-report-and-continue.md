---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Report-and-continue plan channel with apply guard sentinel: the three-step pattern for preserving all-or-nothing writes when a planner relaxes from abort-on-error to report-and-continue"
source: docs/compound/2026-09-04-plan-channel-apply-guard-sentinel-report-and-continue.md
doc_type: learning
description: "When a planner (PlanMigration) is relaxed from hard-abort-on-error to report-and-continue, an apply path that previously aborted on the same errors now silently migrates the valid subset. The three-step sequenced fix: (U2a) add a durable findings channel to the plan so errors are visible to callers; (U2b) guard the apply entry point with a sentinel (ErrPlanHasFindings) that rejects any plan carrying findings before a single write; (U2c) relax the planner. This ordering is not optional: U2c without U2b is a silent partial-apply bug, and U2b without U2a produces a sentinel with no findings surface. The CLI and MCP adapters both call the shared planner and do NOT inspect plan.Findings, so the guard must live at the apply boundary, not in the adapters. Verified in 136-S (docline docs migrate --apply path)."
docline:
    date: 2026-09-04T00:00:00Z
    severity: high
    tags:
        - go
        - architecture
        - report-and-continue
        - plan-channel
        - apply-guard
        - sentinel-error
        - all-or-nothing
        - docline
        - 136-S
        - U2a
        - U2b
        - U2c
---

# Plan channel + apply guard sentinel: preserving all-or-nothing when a planner relaxes

## Context

`PlanMigration` was originally an abort-on-error planner: any `Normalize` error
caused the entire plan to fail. The plan was always either a complete set of
changes (no errors) or an error. `ApplyMigration` consumed the plan with no
additional validation.

U2c needed to relax `PlanMigration` to **report-and-continue**: frontmatter
decode errors should record a finding and continue with the remaining files,
rather than aborting the whole corpus.

## The hidden apply-path leak

`PlanMigration` is the **shared planner** for both dry-run and apply. The CLI
(`cli/docs.go`) and MCP (`mcp/docs_tools.go`) adapters call `PlanMigration`
then `ApplyMigration` and do **NOT** inspect `plan.Findings`. So relaxing
`PlanMigration` without a guard means:

- The planner drops the malformed file from `plan.Changes` (silently)
- The planner populates `plan.Findings` (the new channel)
- The adapters call `ApplyMigration(plan, opts)` — which has no knowledge of `plan.Findings`
- `ApplyMigration` happily migrates the valid subset

**Before the fix**: the corpus was all-or-nothing because the planner aborted.
**After U2c without U2b**: the corpus silently migrates the valid subset — the
malformed file is skipped, the rest is migrated. The operator has no visible
signal that part of the corpus was skipped.

This is the "shared planner for dry-run AND apply" trap: it looks safe to relax
the planner for dry-run, but dry-run and apply share the same call path.

## The three-step fix sequence (MANDATORY ORDER)

### Step 1 — U2a: Add a durable findings channel

Add `Findings []Finding` to `MigrationPlan`. Without this, there is nowhere to
carry the per-file errors — any guard in `ApplyMigration` would have nothing to
check. Both `MigrationPlan.Findings` and `MigrateReport.Findings` must be added
(transport shape), both always-array.

```go
type MigrationPlan struct {
    Changes  []Change
    Findings []Finding  // NEW: per-file decode errors
}
```

### Step 2 — U2b: Guard `ApplyMigration` BEFORE relaxing the planner

**Before** making `PlanMigration` report-and-continue, add the apply guard:

```go
func ApplyMigration(plan MigrationPlan, opts Options) (Result, error) {
    var res Result
    // Guard FIRST — before any preflight or write.
    if len(plan.Findings) > 0 {
        return res, fmt.Errorf("docline.ApplyMigration: %w", ErrPlanHasFindings)
    }
    // ... existing preflight, TOCTOU, write logic
}
```

The sentinel `ErrPlanHasFindings` is exported so CLI/MCP can map it to
structured error responses.

### Step 3 — U2c: Relax the planner

Only now, make `PlanMigration` report-and-continue. The guard is already in
place, so the relaxed planner never silently migrates a partial corpus.

## Why the order is non-optional

If U2c lands before U2b:
- `plan.Findings` is populated (the planner continues past the malformed file)
- `ApplyMigration` has no guard, applies the valid subset
- This is the silent partial-apply bug

### If U2b lands without U2a

Without U2a, `MigrationPlan` has no `Findings []Finding` field. The U2b
guard references `len(plan.Findings)`, which is a **compile error** — the
guard cannot compile, let alone run. This makes the dependency concrete:
U2b is syntactically impossible without U2a, not merely a logical no-op.

The canonical order is U2a → U2b → U2c. In the backlog, U2b depends on U2a,
and U2c depends on U2b (explicit `depends_on` edges).

## Adapter pattern: guard at the boundary, not in adapters

The adapters (`cli/docs.go`, `mcp/docs_tools.go`) **should not** check
`plan.Findings` before calling `ApplyMigration`. The guard belongs at the
`ApplyMigration` boundary for two reasons:

1. **Single point of enforcement**: any caller of `ApplyMigration` gets the
   guarantee without needing to remember to check `plan.Findings` first.
2. **Shared planner**: if the guard were in the adapters, adding a new adapter
   would silently bypass it.

The adapters only need to handle the sentinel error (`ErrPlanHasFindings`)
returned by `ApplyMigration` and surface it appropriately (text render + exit
code for CLI, structured `plan_has_findings` IsError response for MCP).

## Render-before-return on CLI

When `ApplyMigration` returns `ErrPlanHasFindings`, the CLI should render the
report (including `plan.Findings`) BEFORE returning the sentinel, so the operator
sees the findings output first, then the non-zero exit. Mirror the existing
`errLintViolations` render-then-signal pattern:

```go
if errors.Is(err, docline.ErrPlanHasFindings) {
    // Render first (dryRun=true: nothing was written)
    if renderErr := writeMigrateResult(cmd, format, plan, nil, true); renderErr != nil {
        return renderErr
    }
    return docline.ErrPlanHasFindings
}
```

Note: `dryRun=true` on the rejection path, not `false`. Nothing was applied;
rendering as an apply would misrepresent the outcome in `dry_run: false` JSON.

## MCP structured error

The MCP adapter must map `ErrPlanHasFindings` to a **distinct** error type
(`plan_has_findings`, not `validation_failed`), with a discrete top-level
`findings` array (not flattened into `message`). The distinct type lets agents
disambiguate a corpus-content rejection from a `--path` validation failure.

```go
type planHasFindingsResponse struct {
    Error    string                  `json:"error"`    // "plan_has_findings"
    Message  string                  `json:"message"`
    Findings []docline.FindingReport `json:"findings"` // discrete array
}
```

## Test coverage

- Unit guard: `TestU2bApplyGuard_FindingsBearingPlanReturnsError` — plan with
  findings → `ApplyMigration` returns non-nil error
- Zero-write invariant: `TestU2bApplyGuard_FindingsBearingPlanWritesZeroFiles`
- CLI Cobra path: `TestU2bCLIApply_MixedCorpus_RejectsWithFindingsAndZeroWrites`
  — exercises `newDocsMigrateCommand` (not `writeMigrateResult` directly) so
  `SilenceErrors`, `dryRun=true`, and render-before-return are all covered
- MCP end-to-end: `TestU2cMCP_MixedCorpus_ApplyReturnsPlanHasFindings`
- Full integration: `TestU2cEndToEnd_MixedCorpus_ApplyRejectedViaErrPlanHasFindings`

Refs: 136-S / 154.002-T (U2a), 154.003-T (U2b), 154.004-T (U2c),
`internal/docline/service.go`, `internal/cli/docs.go`, `internal/mcp/docs_tools.go`.
