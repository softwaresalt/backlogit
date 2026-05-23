---
title: "Post-merge closure: 063-S Schema Discoverability"
description: "Operational closure record for shipment 063-S — sql_schema in MetadataCatalog and backlogit telemetry schema CLI"
ms.date: 2026-05-22
ms.topic: reference
shipment: 063-S
pr: "123"
merge_sha: ca26d887232214596392b0af848b48ee360eb010
status: READY
---

## Post-Merge Closure: 063-S Schema Discoverability

**Shipment**: `063-S` — Ship: Schema Discoverability
**PR**: [#123](https://github.com/softwaresalt/backlogit/pull/123)
**Merge SHA**: `ca26d887232214596392b0af848b48ee360eb010`
**Branch**: `feature/063-f-schema-discoverability` → `main`
**Closure branch**: `post-merge/063-f-schema-discoverability`
**Date**: 2026-05-22

## Change Summary

Two programmatic schema discovery surfaces shipped:

1. **SQL schema in metadata catalog** — `backlogit_get_metadata_catalog` now includes a
   `sql_schema` array with live SQLite table definitions extracted at runtime via
   `db.IntrospectSchema` (PRAGMA introspection). Agents no longer need pre-loaded
   instruction files to construct correct SQL queries.

2. **`backlogit telemetry schema` CLI subcommand** — Prints field definitions for all
   JSONL fact tables and SQLite telemetry tables in `text`, `json`, or `markdown` format.
   Schema maintained as a typed Go registry with drift-detection tests.

## Scope

| Task | Title | Status |
|---|---|---|
| 063-F | Schema Discoverability (covering feature) | ✓ archived |
| 063.001-T | SQL schema introspection in db package | ✓ archived |
| 063.002-T | SQL schema in metadata catalog | ✓ archived |
| 063.003-T | Telemetry schema reference types | ✓ archived |
| 063.004-T | Telemetry schema CLI subcommand | ✓ archived |
| 063.005-T | CLI reference regeneration and quality gates | ✓ archived |

## Quality Gates (at merge)

| Gate | Result |
|---|---|
| `go test ./...` | ✓ all 20 packages pass |
| `go vet ./...` | ✓ no issues |
| `golangci-lint run` | ✓ zero warnings |
| Schema contract tests | ✓ drift-detection test validates field registry against struct JSON tags |
| CLI reference | ✓ `docs/cli-reference/backlogit_telemetry_schema.md` exists |
| CI (GitHub Actions Go 1.23 + 1.24 matrix) | ✓ passed |
| Copilot review | ✓ reviewed; no unresolved threads |

## Invariants to Preserve

* `backlogit_get_metadata_catalog` must always include a valid `sql_schema` array when
  the database is accessible; `omitempty` ensures backward compatibility when nil.
* `PRAGMA table_info`, `PRAGMA index_list`, and `PRAGMA index_info` must remain in the
  `allowedPragmas` gate in `internal/db/gate.go` or schema introspection will fail.
* The telemetry schema drift-detection test in `internal/telemetry/schema_ref_test.go`
  must pass — it guards against registry/struct divergence.
* `backlogit telemetry schema` output must match struct JSON tags exactly (enforced by drift test).

## Pre-Deploy Checks

This is a merge-only release (no separate deployment step). The following were
verified before merge:

* [x] `db.IntrospectSchema` validates table names from `sqlite_master` before PRAGMA
  interpolation — no SQL injection surface.
* [x] `allowedPragmas` set in `gate.go` updated to include `index_list` and `index_info`.
* [x] `sql_schema` field uses `omitempty` — existing callers receiving no schema see no
  change in behavior.
* [x] No new external dependencies introduced.
* [x] All tests pass on Go 1.23 and Go 1.24 matrix.

## Deployment Path

Merge-only. No separate deployment or migration step. The SQLite index is rebuilt
on demand via `backlogit sync`; schema introspection runs at call time. No data
migration required.

## Post-Deploy Checks

* Call `backlogit_get_metadata_catalog` on an initialized workspace and verify
  `sql_schema` is present in the response.
* Run `backlogit telemetry schema` and `backlogit telemetry schema --format json`
  to confirm output matches expected field counts.
* Run `backlogit telemetry schema --format markdown` to confirm markdown table
  formatting is correct.

## Risky Action Record

| Action | Risk | Approval Path | Result |
|---|---|---|---|
| Adding `allowedPragmas` entries (`index_list`, `index_info`) | moderate — widens PRAGMA allowlist | reviewed in PR #123 | applied |
| Runtime PRAGMA introspection on live DB | moderate — read-only but DB-dependent | table name validation gate added | applied |

## Healthy Signals

* `sql_schema` field present with ≥ 10 tables (items, item_deps, commit_links,
  stash_entries, item_links, stash_links, item_logs, item_log_entries, and two FTS
  virtual tables) in `backlogit_get_metadata_catalog` responses.
* `backlogit telemetry schema` exits 0 with output for all three fact tables.
* No new test failures in `internal/db/` or `internal/telemetry/` packages.

## Failure Signals

* `sql_schema` field absent or empty — likely `allowedPragmas` regression or DB
  not initialized before catalog call.
* `backlogit telemetry schema` panics or exits non-zero — check `schema_ref.go`
  registry for field count mismatch vs drift-detection test.
* CI failure on schema tests after a struct field rename in `internal/telemetry/records.go`
  — update `schema_ref.go` registry to match.

## Monitoring Plan

This feature has no runtime SLI targets (it is a developer/agent tool, not a
user-facing service). Key observability:

* Drift-detection test in CI (`TestDescribeFactTables_DriftDetection`) guards the
  telemetry schema registry automatically on every PR.
* Schema introspection errors are logged via `slog.Debug` in `db.IntrospectSchema` —
  review CI log output if catalog calls start returning empty `sql_schema`.

## Rollback Trigger

* If `backlogit_get_metadata_catalog` consistently returns errors or malformed
  responses after this merge, revert `internal/db/schema.go` and
  `internal/core/metadata_catalog.go` changes and pin the PRAGMA allowlist back to
  the pre-063-S set.

## Rollback Procedure

```bash
git revert ca26d887232214596392b0af848b48ee360eb010 --no-edit
git push origin main
```

If a targeted revert is preferred, revert only the PRAGMA gate change in
`internal/db/gate.go` and the `IntrospectSchema` call in `internal/mcp/metadata.go`.

## Validation Window

72 hours post-merge. Owner: engineering on-call. Criterion: no schema-related errors
in CI or agent-reported MCP failures.

## Source Artifact Cleanup

| Artifact | Action | Result |
|---|---|---|
| Stash `ACDF8C2D` (SQL schema in catalog) | harvested at feature creation | already consumed |
| Stash `1D5578B5` (telemetry schema CLI) | harvested at feature creation | already consumed |
| Deliberation `047-DL` | archived by `backlogit shipment ship` | `.backlogit/archive/047-DL.md` |

## Follow-up Items

None required. The deferred MCP `backlogit_telemetry_schema` tool (Option B.3 from
deliberation 047-DL) remains out of scope — stash if needed in a future pass.

## Readiness Status

**READY** — All quality gates passed. CI clean. Source artifacts archived. No
unresolved review threads. Monitoring plan in place. Closure PR requires operator
approval before merge.
