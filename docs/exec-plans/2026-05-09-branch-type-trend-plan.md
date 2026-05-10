---
title: "Branch-Level Telemetry Metrics"
date: 2026-05-09
origin: ".backlogit/queue/048-DL.md"
status: approved
---

# Branch-Level Telemetry Metrics

## Problem Frame

The telemetry system groups metrics by session or by raw branch name. Operators
need branch as the **base aggregation unit** — each branch profile combines all
its sessions into a single row enriched with branch type classification, git PR
data, and backlogit artifact linkage.

This enables questions like: "How many tokens did the schema-discoverability
feature branch consume?", "What is the average cost of a stage branch?", and
"Which shipment had the most expensive feature branches?"

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | `DeriveBranchType(branch string) string` classifier | 048-DL Chosen Direction |
| R2 | `BranchProfile` struct aggregating sessions per branch | User direction: branch-level base aggregation |
| R3 | Git enrichment: branch → PR number via merge commit parsing | User direction: combine git data |
| R4 | Backlogit enrichment: extract shipment/feature IDs from branch name | User direction: combine backlogit archive data |
| R5 | New `telemetry branch` CLI subcommand | User direction: branch-level metrics command |
| R6 | `--by branch-type` trend grouping for roll-up | 048-DL Chosen Direction |
| R7 | Follow empty-string convention for missing data | 048-DL Notes |

## Scope Boundaries

### In Scope

- `DeriveBranchType` classifier in `internal/telemetry/records.go`
- `BranchProfile` type and `AggregateBranches` function in new `internal/telemetry/branch_metrics.go`
- `ExtractArtifactIDs` utility to parse shipment/feature IDs from branch names
- Git merge-commit parsing for branch → PR mapping
- `backlogit telemetry branch` CLI subcommand with table/json/markdown output
- `--by branch-type` grouping in `GenerateTrendReport`
- CLI reference regeneration

### Non-Goals

- Storing `branch_type` in JSONL records or SQLite schema
- Querying backlogit.db for enrichment (keep telemetry package decoupled from core)
- Adding enrichment to `telemetry report` or `telemetry trend` (those stay session-based)

### Deferred to Implementation

- Column width tuning for branch table output
- Whether `--type` filter flag is needed on `telemetry branch` (add if trivial)

## Implementation Units

### Unit 1: DeriveBranchType Classifier

**Files:** `internal/telemetry/records.go`
**Test files:** `internal/telemetry/records_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `DeriveModelClass` at `records.go:66-84`
**Dependencies:** none

**Approach:**

Add `DeriveBranchType(branch string) string` adjacent to `DeriveModelClass`.
Classification rules (applied in order):
1. empty string → `""`
2. `feature/` prefix → `"feature"`
3. `chore/stage-` prefix → `"stage"`
4. `ship/` prefix → `"ship"`
5. `post-merge/` prefix → `"post-merge"`
6. `chore/` prefix → `"chore"`
7. `main` or `master` exact → `"main"`
8. fallback → `"other"`

Rule 3 must precede rule 6 so `chore/stage-*` → `"stage"` not `"chore"`.
Rule 4 captures `ship/*` branches separately from generic chores.

**Verification:**

Test cases: `""` → `""`, `"feature/057-f-slug"` → `"feature"`,
`"chore/stage-056-s-slug"` → `"stage"`, `"ship/055s-slug"` → `"ship"`,
`"chore/fix-x"` → `"chore"`, `"post-merge/slug"` → `"post-merge"`,
`"main"` → `"main"`, `"master"` → `"main"`, `"release/v1"` → `"other"`

### Unit 2: BranchProfile Aggregation and Artifact ID Extraction

**Files:** `internal/telemetry/branch_metrics.go`
**Test files:** `internal/telemetry/branch_metrics_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `TrendGroup` aggregation pattern in `reporter.go:410-465`
**Dependencies:** Unit 1

**Approach:**

Create `internal/telemetry/branch_metrics.go` with:

```go
// BranchProfile holds aggregated metrics for a single branch, enriched with
// classification and artifact linkage.
type BranchProfile struct {
    Branch         string    `json:"branch"`
    BranchType     string    `json:"branch_type"`
    Sessions       int       `json:"sessions"`
    TotalTokens    int       `json:"total_tokens"`
    AvgTokens      float64   `json:"avg_tokens_per_session"`
    TotalModelCalls int      `json:"total_model_calls"`
    TotalToolCalls  int      `json:"total_tool_calls"`
    AvgPeakUtil    *float64  `json:"avg_peak_utilization,omitempty"`
    TaskCount      int       `json:"task_count"`
    PRNumber       string    `json:"pr_number,omitempty"`
    ShipmentID     string    `json:"shipment_id,omitempty"`
    FeatureID      string    `json:"feature_id,omitempty"`
    FirstSeen      time.Time `json:"first_seen"`
    LastSeen       time.Time `json:"last_seen"`
}
```

- `AggregateBranches(sessions []SessionSummaryRecord) []BranchProfile` — groups
  sessions by branch, computes totals/averages, derives branch type, extracts
  artifact IDs. Returns sorted by `LastSeen` descending. **Must filter ghost
  sessions via `IsGhostSession(s)` before aggregation** (see compound learning:
  `telemetry-ghost-session-filtering-pattern.md`).
- `ExtractArtifactIDs(branch string) (shipmentID, featureID string)` — parses
  known branch naming patterns:
  - `feature/{NNN}-f-*` or `feature/{NNN}-F-*` → featureID = `{NNN}-F`
  - `ship/{NNN}s-*` → shipmentID = `{NNN}-S`
  - `chore/stage-{NNN}-s-*` or `chore/stage-{NNN}s-*` → shipmentID = `{NNN}-S`
  - Others → empty strings
- `TaskCount` is the sum of `len(s.CompletedTasks)` across all sessions in the branch.
- `FirstSeen`/`LastSeen` are the min/max `HarvestedAt` timestamps.

**Verification:**

- Test `AggregateBranches` with fixture sessions across 3 branches, verify grouping,
  totals, averages, branch type, and sort order
- Test `ExtractArtifactIDs` with all known branch patterns and edge cases
- Test empty sessions returns empty slice
- Test ghost sessions are excluded from aggregation (fixture with 2 real + 1 ghost session → only 2 counted)

### Unit 3: Git PR Enrichment

**Files:** `internal/telemetry/branch_metrics.go` (add to same file)
**Test files:** `internal/telemetry/branch_metrics_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** standard `os/exec` + line parsing
**Dependencies:** none (independent of Unit 1-2 at code level, but used together)

**Approach:**

Add `ParseGitMergePRs(repoPath string) (map[string]string, error)` that:
1. Runs `git -C <repoPath> log --merges --oneline --all` via `os/exec`
2. Parses each line for pattern: `Merge pull request #N from <owner>/<branch>`
3. Returns `map[branchName]prNumber` (e.g. `{"feature/057-f-slug": "#109"}`)
4. Returns empty map (not error) when git is unavailable or repo has no merges

Internally, split into two functions for testability:
- `ParseGitMergePRs(repoPath string) (map[string]string, error)` — runs git, delegates to parser
- `parseMergeLines(r io.Reader) map[string]string` — pure parsing, testable without git

Also add `EnrichBranchProfiles(profiles []BranchProfile, prMap map[string]string)`
that fills in `PRNumber` on each profile from the map.

**Verification:**

- Test `parseMergeLines` with multi-line fixture input including edge cases
  (non-merge commits, malformed lines, branches with slashes)
- Test `EnrichBranchProfiles` with a mix of matched and unmatched branches
- Test graceful fallback when git is not available (ParseGitMergePRs returns empty map)

### Unit 4: `telemetry branch` CLI Command

**Files:** `internal/cli/telemetry.go`
**Test files:** `internal/cli/telemetry_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newTelemetryTrendCmd` at `telemetry.go:187-224`, `newTelemetrySchemaCmd`
**Dependencies:** Units 1, 2, 3

**Approach:**

Add `newTelemetryBranchCmd(cwd *string) *cobra.Command`:
1. Load sessions from JSONL via existing `telemetry` package functions
2. Call `AggregateBranches(sessions)` to produce `[]BranchProfile`
3. Call `ParseGitMergePRs(*cwd)` for PR mapping (graceful failure → empty map)
4. Call `EnrichBranchProfiles(profiles, prMap)`
5. Render in table/json/markdown format

Flags: `--format` (table|json|markdown), `--type` (filter by branch type),
`--limit` (restrict number of branches)

Table columns: `BRANCH | TYPE | SESSIONS | TOKENS | AVG/SESSION | TASKS | PR | FEATURE | SHIPMENT | LIFESPAN`

Where `LIFESPAN` is `LastSeen - FirstSeen` formatted as duration (e.g. "2h30m", "3d").

Register as `cmd.AddCommand(newTelemetryBranchCmd(cwd))` in `NewTelemetryCmd`.

**Verification:**

- Test command exists and accepts `--format` flag
- Test table output contains expected columns
- Test `--type feature` filters to only feature branches
- Test JSON output unmarshals to `[]BranchProfile`

### Unit 5: `--by branch-type` Trend Grouping

**Files:** `internal/telemetry/reporter.go`, `internal/cli/telemetry.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `case "class":` grouping at `reporter.go:425-432`
**Dependencies:** Unit 1

**Approach:**

1. Add `"branch-type"` to the `By` validation at `reporter.go:406`
2. Add `case "branch-type":` in both grouping switches:
   ```
   key = DeriveBranchType(s.Branch)
   if key == "" { key = "(unknown)" }
   ```
3. Update trend command `--by` flag help text to include `branch-type`

**Verification:**

- Test with JSONL fixture: sessions on `feature/a`, `feature/b`, `chore/stage-x`,
  `chore/fix-y`, `post-merge/z` → verify groups: feature(2), stage(1), chore(1), post-merge(1)
- JSON format test: unmarshal `[]TrendGroup`, verify aggregation correctness

### Unit 6: CLI Reference Regeneration and Quality Gates

**Files:** `docs/cli-reference/backlogit_telemetry*.md`
**Test files:** none (CI drift check validates)
**Effort size:** small
**Skill domain:** docs
**Execution note:** run `go run ./cmd/gen-docs` then all 4 quality gates
**Patterns to follow:** prior CLI reference regeneration
**Dependencies:** Units 4, 5

**Approach:**

Regenerate CLI reference docs via `go run ./cmd/gen-docs`. Run all quality gates.

**Verification:**

- New `docs/cli-reference/backlogit_telemetry_branch.md` exists
- Trend doc mentions `branch-type`
- All 4 quality gates pass

## Dependency Graph

```
Unit 1 (DeriveBranchType) ─┬── Unit 2 (BranchProfile + aggregation)
                            │       │
Unit 3 (Git PR enrichment) ─┤       │
                            ├── Unit 4 (CLI branch command)
                            │
                            └── Unit 5 (--by branch-type trend)
                                    │
                            Unit 6 (CLI ref regen + gates)
```

Units 1, 3 can run in parallel. Unit 2 depends on Unit 1. Units 4, 5 depend on
earlier units. Unit 6 runs last after all code changes.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Enrich at report time, not harvest time | YAGNI, no schema migration, data always current | Harvest-time storage adds migration complexity |
| D2 | `telemetry` package stays decoupled from `core`/`db` | Clean architecture — CLI layer does the joining | Adding core dependency to telemetry couples unrelated concerns |
| D3 | Git enrichment via `os/exec` + merge commit parsing | Simple, works everywhere git is available, no new deps | GitHub API (requires auth), git notes (non-standard) |
| D4 | Artifact ID extraction from branch name patterns | Convention-based, zero-cost, no DB lookup needed | Archive file scanning (slow, fragile), DB query (couples packages) |
| D5 | `ship/` gets its own branch type | Ship execution branches are distinct from general chores | Grouping with chore loses shipping visibility |
| D6 | `chore/stage-*` classified as `"stage"` | Stage planning is a distinct workflow phase | All chore/* uniform loses stage visibility |

## Risks and Caveats

- Branch naming conventions are repository-specific. The classifier and artifact
  ID parser assume backlogit conventions. Other repos may need customization.
- Git merge commit parsing assumes GitHub PR merge format. GitLab/Bitbucket
  use different formats — extend parsing if needed.
- `os/exec` git calls add ~100ms latency. Acceptable for a reporting command.
- Artifact ID extraction is best-effort. If branch naming deviates from
  convention, IDs will be empty (graceful degradation).

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **absent** — no JSONL or SQL schema change
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action: **absent**
* external integration, operator checkpoint, or external dependency: **absent** — git is local
* high runtime, rollout, or rollback risk: **absent** — read-only reporting

Requires plan hardening: no

## Runtime Verification and Closure

- **Unit 4** changes a CLI runtime surface (`backlogit telemetry branch`)
- **Unit 5** changes a CLI runtime surface (`backlogit telemetry trend --by branch-type`)
- Runtime verification: run both commands against the live workspace and confirm output
- Closure: no monitoring or rollback needed — these are read-only reporting features

## Learnings Applied

- `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`:
  Keep empty string for missing data, display labels at render boundary only
- `docs/compound/telemetry-ghost-session-filtering-pattern.md`:
  Filter ghost sessions via `IsGhostSession(s)` in all aggregation loops, not at harvest time

## Standards Check

- Go 1.22+ with GoDoc on all exported types and functions ✓
- Test-first development: tests before implementation ✓
- `golangci-lint` zero warnings: will verify ✓
- Conventional commits: `feat(telemetry):` prefix ✓
- No new external dependencies added (only `os/exec` from stdlib) ✓
- `telemetry` package stays decoupled from `core`/`db` ✓

## Plan Review

**Gate Decision: PASS** (after addressing P1 finding inline)

**Reviewers:** Constitution Reviewer, Go Quality Reviewer, Scope Boundary Auditor, Learnings Researcher

### Summary

5 findings total: 1 P1 (addressed inline), 1 P2 (acknowledged), 3 P3 (advisory).

### P0 — Critical

None

### P1 — High (addressed)

**F1 (Ghost Session Filtering)** — `AggregateBranches` must filter ghost sessions
via `IsGhostSession(s)` before counting, same as `GenerateTrendReport`. Without
this, zero-activity sessions inflate branch metrics. *Fixed: added to Unit 2 approach
and verification criteria.*

### P2 — Moderate

**F2 (os/exec testability)** — `ParseGitMergePRs` uses os/exec which is hard to
unit test. *Addressed: plan now specifies splitting into `ParseGitMergePRs` (exec
wrapper) + `parseMergeLines(io.Reader)` (pure parser, fully testable).*

### P3 — Low (advisory)

**F3** — `ExtractArtifactIDs` should handle case-insensitive ID segments (e.g.
`feature/057-f-slug` and `feature/057-F-slug` both → `057-F`). Implementation
should uppercase the type suffix.

**F4** — Ghost session filtering learning now referenced in Learnings Applied section.

**F5** — The `telemetry` package gains `os/exec` import but this is stdlib-only
and consistent with the package already importing `os` for file I/O. No
architectural concern.

### Next Steps

Plan is approved. Proceed to `harvest` to decompose into backlogit work items.
