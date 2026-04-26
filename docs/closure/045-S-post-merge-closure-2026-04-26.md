# Operational Closure — Shipment 045-S

**Shipment**: 045-S  
**Feature**: 047-F — Telemetry Quality: Parser Fix and Documentation  
**Merge SHA**: b0d1d2986bc2b35a566cac77d09c1bd1a3e43a17  
**PR**: #73  
**Branch**: ship-045-telemetry-quality-fixes  
**Merged**: 2026-04-26  
**Author**: softwaresalt

---

## Release Summary

Four telemetry quality gaps closed in this shipment:

1. **Parser bug fix**: `bufio.Scanner` replaced with `bufio.NewReader.ReadString` in `internal/telemetry/parser.go`, `internal/telemetry/harvest.go`, and `internal/db/telemetry_schema.go`. Scanner had a hard 1MB token limit that silently aborted parsing on oversized log lines. The new reader has no limit.

2. **Partial-session filter**: `ValidateSessionSummary` in `internal/telemetry/validate.go` rejects sessions where `TotalTokens == 0 && ToolCalls > 0` (partial sessions caused by the scanner bug). These are now excluded from harvest output and their tool events are filtered from JSONL records.

3. **Token-ranked server reporting**: `formatServerTable`, `formatReportJSON`, and `formatServerMarkdown` now compute proportional token attribution per server (using `float64` arithmetic and `math.Round`) and sort by tokens descending. Previously the output sorted alphabetically by server name and showed raw tool-call counts.

4. **Documentation**: `docs/cli-reference/backlogit_telemetry_harvest.md` regenerated with Synopsis and Checkpoint sections. `docs/telemetry-fields.md` created as a telemetry field reference.

---

## Pre-Deploy Audit

| Check | Status | Notes |
|---|---|---|
| Feature flags / rollout gates | N/A | No feature flags required |
| Backward compatibility | ✅ | JSONL output format unchanged; SQLite schema unchanged |
| Data migration | ✅ | No schema changes; existing telemetry-sessions.jsonl unaffected |
| Rollback procedure | ✅ Documented below | Revert PR #73 |
| Dependent services | N/A | No cross-service boundaries |
| Monitoring plan | ✅ Documented below | Manual observation |

---

## Monitoring Plan

### Key signals

| Signal | Description | Expected range |
|---|---|---|
| `telemetry harvest` exit 0 | Harvest completes without error | Always |
| Session count in JSONL | New sessions appended on incremental runs | 0–N depending on log activity |
| Zero-token sessions | Sessions with `total_tokens=0` in output | Should be 0 after fix |
| `telemetry top --by server` output | Servers ranked by token attribution | Highest-token server at top |

### Observation window

- **Duration**: 2 weeks from merge
- **Owner**: softwaresalt
- **Method**: Manual — run `backlogit telemetry harvest` and `backlogit telemetry top --by server` on the next real harvest cycle and confirm no zero-token sessions appear and server ranking looks correct

### Alert threshold

If zero-token sessions appear in JSONL output after the first harvest post-merge, investigate whether a new log format introduced lines exceeding the old bufio.Scanner limit.

---

## Rollback Trigger and Procedure

### Trigger

Any of:
- Zero-token sessions reappear in JSONL output
- `telemetry top --by server` shows alphabetical ordering instead of token-ranked ordering
- `telemetry harvest` exits with error not present before this change

### Rollback Procedure

1. `git revert b0d1d2986bc2b35a566cac77d09c1bd1a3e43a17` (PR merge commit)
2. Create and merge a revert PR through the normal review pipeline
3. Delete `.backlogit/telemetry-checkpoint.json` and rerun harvest with `--force` if the JSONL became corrupted

---

## Source Artifact Cleanup

| Artifact | Action | Notes |
|---|---|---|
| `source_stash_id` on 047-F | None (not present) | No stash entry to remove |
| `source_deliberation_id` on 047-F | None (not present) | 041-DL was archived by `shipment ship` |
| 041-DL (deliberation) | ✅ Archived by `shipment ship` command | `.backlogit/archive/041-DL.md` |
| 045-S (shipment) | ✅ Archived | `.backlogit/archive/045-S.md` |
| 047-F, 047.001-T–047.004-T | ✅ Archived | Shipped tasks archived |
| 047.005-T, 047.006-T | ✅ Archived with shipment | P2 follow-ups; may be re-queued in a future shipment if prioritized |

Stash entries archived: 0  
Deliberation artifacts archived: 1 (041-DL, by shipment ship)

---

## Documentation Updates

| Document | Action |
|---|---|
| `docs/cli-reference/backlogit_telemetry_harvest.md` | Updated (regenerated from source) |
| `docs/telemetry-fields.md` | Created |
| `README.md` | No changes required; telemetry feature bullets accurate |
| `docs/ARCHITECTURE.md` | Does not exist; structural changes are internal quality fixes only |
| `docs/design-docs/` | No new design decisions to record |
| `docs/product-specs/` | No product spec changes |

---

## Session Learnings

Two compound learnings captured during this session:

- `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`  
  When fixing a bufio.Scanner bug in one file, the same pattern must be audited across all packages that independently open JSONL sources. Review gate P1 finding caught the `db/telemetry_schema.go` miss.

- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`  
  Files in `docs/cli-reference/` are generated artifacts. Never edit them directly. All content belongs in the Cobra command's `Long:` field; then run `go run ./cmd/gen-docs docs/cli-reference` and commit both.

---

## Validation Outcome

**Status**: ✅ Healthy  
**CI**: 3/3 checks passed on both PR #73 and PR #74  
**Copilot review**: 7/7 threads resolved across 2 review rounds  
**Tests**: All 20 packages passing including 4 new harness tests + 2 regression guards  
