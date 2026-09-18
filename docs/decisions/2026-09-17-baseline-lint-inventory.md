---
chunk_strategy: h1-h2-h3
description: Baseline golangci-lint inventory and one-file-per-task mapping for the 175-F file-owned lint DAG
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-17-baseline-lint-inventory.md
title: "Baseline Lint Inventory: 175-F File-Owned Lint DAG"
docline:
    feature_id: 175-F
    linter: golangci-lint
    linter_version: 2.13.2
---

# Baseline Lint Inventory: 175-F File-Owned Lint DAG

## Provenance

This inventory is generated directly from the read-only golangci-lint evidence captured for the baseline-convergence branch. It is the durable planning-evidence source of truth for the one-flagged-file-per-task decomposition of feature 175-F.

- Command: `golangci-lint run --issues-exit-code 0 --output.text.path=NUL --output.json.path=<file>`
- Linter: golangci-lint v2.13.2
- Baseline branch: `stage/baseline-convergence-main`
- Baseline HEAD at capture: `c80d7c6ba968604913614e90522e1380ba84ad99`
- Total findings: 56 (errcheck 50 + staticcheck 6)
- Flagged files: 36 (one file-owned lint task each)

gofmt/CRLF companion proof (`baseline-gofmt-files.txt` vs `baseline-crlf-go-json.txt`): the 518 gofmt-listed Go files are exactly the 518 tracked CRLF Go files (`gofmtNotCRLF=0`, `crlfGoNotGofmt=0`), so U1's line-ending migration resolves gofmt wholesale and no separate gofmt task is required. U1's wider renormalization scope is 548 tracked CRLF paths (518 `*.go` + 30 `*.json`, from live read-only `git ls-files --eol`); the 30 JSON paths are byte-only and outside gofmt's Go-only scope.

## Linter set disjointness (no-overlap proof)

- errcheck: 50 findings across 31 files / 9 packages
- staticcheck: 6 findings across 5 files / 4 packages
- errcheck files INTERSECT staticcheck files = 0 (EMPTY)

Because the errcheck and staticcheck file sets are disjoint, NO overlap-inventory artifact and NO overlap dependency edge are required. Each flagged file is owned by exactly one lint task under exactly one linter.

## One-file-per-task mapping (36 file-owned lint tasks)

| Task | Linter | File | Lines | Findings | Package | Affected funcs | Reserved harness path |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 175.002-T | errcheck | `internal/atomicfile/atomicfile_bench_test.go` | 48 | 1 | `internal/atomicfile` | BenchmarkFsyncFile | `internal/atomicfile/errcheck_atomicfile_bench_test_175_test.go` |
| 175.003-T | staticcheck | `internal/canonical/canonical.go` | 228 | 1 | `internal/canonical` | encodeString | `internal/canonical/staticcheck_canonical_175_test.go` |
| 175.004-T | errcheck | `internal/cli/add.go` | 68, 120 | 2 | `internal/cli` | newAddCommand | `internal/cli/errcheck_add_175_test.go` |
| 175.005-T | errcheck | `internal/cli/adopt.go` | 31 | 1 | `internal/cli` | newAdoptCommand | `internal/cli/errcheck_adopt_175_test.go` |
| 175.006-T | errcheck | `internal/cli/archive.go` | 30, 41, 53, 58 | 4 | `internal/cli` | newArchiveCommand | `internal/cli/errcheck_archive_175_test.go` |
| 175.008-T | staticcheck | `internal/cli/checkpoint_remediation_test.go` | 168 | 1 | `internal/cli` | runCLICombined | `internal/cli/staticcheck_checkpoint_remediation_test_175_test.go` |
| 175.007-T | errcheck | `internal/cli/checkpoint.go` | 347, 348 | 2 | `internal/cli` | RenderCheckpointRemediationBlock | `internal/cli/errcheck_checkpoint_175_test.go` |
| 175.009-T | staticcheck | `internal/cli/jsonrpc_test.go` | 126 | 1 | `internal/cli` | TestExecute_FunctionExists | `internal/cli/staticcheck_jsonrpc_test_175_test.go` |
| 175.010-T | errcheck | `internal/cli/list.go` | 261 | 1 | `internal/cli` | newListCommand | `internal/cli/errcheck_list_175_test.go` |
| 175.011-T | errcheck | `internal/cli/migrate.go` | 171, 197 | 2 | `internal/cli` | runSourceMigration | `internal/cli/errcheck_migrate_175_test.go` |
| 175.013-T | errcheck | `internal/cli/registry_parity_test.go` | 489, 538, 579 | 3 | `internal/cli` | observeGovernedState, observeGovernedCommentState, observeGovernedDependency | `internal/cli/errcheck_registry_parity_test_175_test.go` |
| 175.015-T | errcheck | `internal/cli/tty_test.go` | 19, 20 | 2 | `internal/cli` | TestIsTerminal_PipeReturnsFalse | `internal/cli/errcheck_tty_test_175_test.go` |
| 175.016-T | errcheck | `internal/cli/update_sync_test.go` | 65 | 1 | `internal/cli` | TestUpdateCommand_SectionWrite_SyncsDB | `internal/cli/errcheck_update_sync_test_175_test.go` |
| 175.017-T | errcheck | `internal/core/archive_durable_write_test.go` | 77 | 1 | `internal/core` | setupDurableArchiveWorkspace | `internal/core/errcheck_archive_durable_write_test_175_test.go` |
| 175.018-T | errcheck | `internal/core/doctor_test.go` | 189, 244 | 2 | `internal/core` | TestDoctor_FixOrphansArchivesOrphanedTask, TestDoctor_FixOrphansSkipsReturnedToBacklog | `internal/core/errcheck_doctor_test_175_test.go` |
| 175.019-T | errcheck | `internal/core/shipment_reconcile_append_windows.go` | 90 | 1 | `internal/core` | appendShipmentReconcileEventHandleRelative | `internal/core/errcheck_shipment_reconcile_append_windows_175_test.go` |
| 175.020-T | errcheck | `internal/core/shipment_reconcile_evidence_windows.go` | 56 | 1 | `internal/core` | readShipmentReconcileClosureEvidenceFile | `internal/core/errcheck_shipment_reconcile_evidence_windows_175_test.go` |
| 175.021-T | errcheck | `internal/core/shipment_reconcile_fs_windows.go` | 154 | 1 | `internal/core` | writeShipmentReconcileArchiveFileHandleRelative | `internal/core/errcheck_shipment_reconcile_fs_windows_175_test.go` |
| 175.022-T | errcheck | `internal/core/shipment_reconcile_snapshot_windows.go` | 66 | 1 | `internal/core` | readShipmentReconcileArchiveSnapshotFile | `internal/core/errcheck_shipment_reconcile_snapshot_windows_175_test.go` |
| 175.023-T | errcheck | `internal/core/shipment_test.go` | 36, 1454, 1483 | 3 | `internal/core` | setupShipmentWorkspace, TestNewWorkspace_RecoversPendingReturnBlockedJournal, TestShipment_RehydrationConsistency | `internal/core/errcheck_shipment_test_175_test.go` |
| 175.024-T | errcheck | `internal/core/workspace.go` | 151 | 1 | `internal/core` | NewWorkspace | `internal/core/errcheck_workspace_175_test.go` |
| 175.025-T | errcheck | `internal/db/connection.go` | 88 | 1 | `internal/db` | Open | `internal/db/errcheck_connection_175_test.go` |
| 175.026-T | errcheck | `internal/db/dependencies.go` | 58, 69, 196 | 3 | `internal/db` | GetDependencies, GetDependents, detectCycleTx | `internal/db/errcheck_dependencies_175_test.go` |
| 175.027-T | errcheck | `internal/db/telemetry_schema.go` | 129 | 1 | `internal/db` | RehydrateTelemetry | `internal/db/errcheck_telemetry_schema_175_test.go` |
| 175.028-T | errcheck | `internal/events/hook_events.go` | 126 | 1 | `internal/events` | (*HookEventWriter).ReadHookEvents | `internal/events/errcheck_hook_events_175_test.go` |
| 175.029-T | staticcheck | `internal/gateevidence/formal.go` | 223 | 1 | `internal/gateevidence` | envelopeFromEvent | `internal/gateevidence/staticcheck_formal_175_test.go` |
| 175.030-T | errcheck | `internal/hooks/webhook.go` | 172 | 1 | `internal/hooks` | (*WebhookNotifier).dispatchToEndpoint | `internal/hooks/errcheck_webhook_175_test.go` |
| 175.031-T | errcheck | `internal/mcp/contract_consistency_test.go` | 196 | 1 | `internal/mcp` | TestMCP_ConcurrentEnsureWorkspace_NoRace | `internal/mcp/errcheck_contract_consistency_test_175_test.go` |
| 175.032-T | errcheck | `internal/mcp/reliability_test.go` | 42 | 1 | `internal/mcp` | TestEnsureWorkspace_ConcurrentCallsShareWorkspace | `internal/mcp/errcheck_reliability_test_175_test.go` |
| 175.033-T | errcheck | `internal/mcp/server_init_test.go` | 67, 106 | 2 | `internal/mcp` | TestEnsureWorkspace_CachesSuccess, TestEnsureWorkspace_ConcurrentSafe | `internal/mcp/errcheck_server_init_test_175_test.go` |
| 175.035-T | errcheck | `internal/stash/jsonl_test.go` | 124 | 1 | `internal/stash` | TestMigrateStashMDToJSONL_Success | `internal/stash/errcheck_jsonl_test_175_test.go` |
| 175.034-T | errcheck | `internal/stash/jsonl.go` | 100, 101, 105, 109 | 4 | `internal/stash` | MigrateStashMDToJSONL | `internal/stash/errcheck_jsonl_175_test.go` |
| 175.036-T | errcheck | `internal/telemetry/checkpoint_test.go` | 112 | 1 | `internal/telemetry` | TestHarvestTelemetry_Force_ReprocessesAllLogs | `internal/telemetry/errcheck_checkpoint_test_175_test.go` |
| 175.037-T | errcheck | `internal/telemetry/events_harvest.go` | 304, 310 | 2 | `internal/telemetry` | HarvestEventsFacts | `internal/telemetry/errcheck_events_harvest_175_test.go` |
| 175.038-T | staticcheck | `internal/telemetry/reporter.go` | 196, 198 | 2 | `internal/telemetry` | formatSessionTable | `internal/telemetry/staticcheck_reporter_175_test.go` |
| 175.039-T | errcheck | `internal/telemetry/session_store.go` | 28 | 1 | `internal/telemetry` | ReadSessionStore | `internal/telemetry/errcheck_session_store_175_test.go` |

## Bounds verification

- Every task modifies at most 2 files (1 flagged file + 1 reserved harness file).
- Maximum affected functions in any single file: 3 (< 5).
- No file carries >= 4 distinct affected functions, so no file requires a further same-file split.
- Every reserved harness path is unique (exactly-one-owner, collision-proof naming `<pkg>/<linter>_<basename>_175_test.go`).
- Each harness file's test function follows the P-002.6 selector convention `TestU175_<NNN>_...` (deterministic sanitized unit token, e.g. `TestU175_002_` for `175.002-T`); the scoped verification command is `go test ./<pkg> -run '^TestU175_<NNN>_' -v -count=1`, preserving native exit and failing fail-closed when zero matching `--- PASS: TestU175_<NNN>_` lines appear. Errcheck findings are resolved by checked propagation/wrapping, named-return `errors.Join`, test assertion, or an existing safe helper — never by an ignored `_ =` return.
