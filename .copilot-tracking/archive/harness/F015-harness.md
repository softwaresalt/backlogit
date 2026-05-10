# F015 Harness Manifest

**Feature**: Two-agent workflow refactor
**Generated**: 2026-04-05
**Branch**: 015-two-agent-workflow-refactor
**Import Check**: PASS
**Red Phase**: CONFIRMED

## Test Files

| Tier | Path | Test Count |
|------|------|------------|
| unit | internal/config/shipment_defaults_test.go | 6 |
| unit | internal/errors/shipment_errors_test.go | 2 |
| unit | internal/core/shipment_test.go | 9 |
| unit | internal/cli/shipment_test.go | 6 |
| unit | internal/stash/jsonl_test.go | 7 |
| contract | tests/contract/shipment_tools_test.go | 8 |
| integration | tests/integration/shipment_workflow_test.go | 6 |

## Stub Files

| Path | Symbols |
|------|---------|
| internal/core/shipment.go | ShipmentStatus, ShipmentQueued, ShipmentActive, ShipmentShipped, ShipmentAbandoned, CreateShipment, GetShipment, MoveShipmentStatus, AddItemToShipment, ReturnBlockedItem |
| internal/cli/shipment.go | NewShipmentCmd, newShipmentCreateCmd, newShipmentGetCmd, newShipmentListCmd, newShipmentClaimCmd, newShipmentReturnBlockedCmd |
| internal/stash/jsonl.go | WriteJSONL, ReadJSONL, MigrateStashMDToJSONL |
| internal/errors/errors.go | ErrShipmentNotFound, ErrItemAlreadyAssigned, ErrShipmentConflict, ErrCannotReturnItem (added to existing file) |

## Work Item Mapping

| Item ID | Title | Test Function(s) | Harness Command | Status |
|---------|-------|-------------------|-----------------|--------|
| T001 / ST001 | Add shipment to DefaultArtifactTypes | TestDefaultConfig_ContainsShipmentType, TestDefaultConfig_ShipmentInQueueLayout | `go test ./internal/config/... -run TestDefaultConfig -v` | RED |
| T001 / ST002 | Add shipment headerdef schema | TestDefaultHeaderDef_ContainsShipmentSchema | `go test ./internal/config/... -run TestDefaultHeaderDef -v` | RED |
| T001 / ST003 | Add shipment default template | TestDefaultTemplates_ContainsShipment | `go test ./internal/config/... -run TestDefaultTemplates -v` | RED |
| T001 / ST005 | Prefix uniqueness test | TestDefaultConfig_PrefixUniqueness, TestDefaultConfig_ShipmentPrefixIsS | `go test ./internal/config/... -run TestDefaultConfig_Prefix -v` | RED |
| T002 / ST011 | Sentinel errors | TestShipmentSentinelErrors_ErrorsIs, TestShipmentSentinelErrors_AreDistinct | `go test ./internal/errors/... -run TestShipmentSentinel -v` | GREEN |
| T002 / ST012 | Create and status transitions | TestCreateShipment_Success, TestMoveShipmentStatus_QueuedToActive, TestMoveShipmentStatus_ActiveToShipped, TestMoveShipmentStatus_InvalidTransition | `go test ./internal/core/... -run "TestCreateShipment|TestMoveShipment" -v` | RED |
| T002 / ST013 | Item association and blocked return | TestAddItemToShipment_Success, TestAddItemToShipment_AlreadyAssigned, TestReturnBlockedItem_Success, TestReturnBlockedItem_NotInShipment | `go test ./internal/core/... -run "TestAddItem|TestReturnBlocked" -v` | RED |
| T002 / ST014 | Rehydration consistency | TestShipment_RehydrationConsistency | `go test ./internal/core/... -run TestShipment_Rehydration -v` | RED |
| T003 / ST016 | CLI commands | TestNewShipmentCmd_HasSubcommands, TestShipmentCreateCmd_HasFlags, TestShipmentReturnBlockedCmd_HasFlags, TestShipmentSubcommands_HaveDocumentation, TestShipmentListCmd_HasStatusFilter, TestShipmentCmd_HelpOutput | `go test ./internal/cli/... -run "TestNewShipmentCmd|TestShipment" -v` | RED |
| T003 / ST018 | Contract tests | TestCreateShipment_*, TestGetShipment_*, TestListShipments_Success, TestClaimShipment_Success, TestReturnBlocked_* | `go test ./tests/contract/... -run "Shipment|ReturnBlocked" -v` | RED |
| T004 / ST019 | JSONL read/write | TestJSONL_RoundTrip, TestWriteJSONL_OneLinePerEntry, TestReadJSONL_SkipsEmptyLines, TestReadJSONL_MalformedLine | `go test ./internal/stash/... -run TestJSONL -v` | RED |
| T004 / ST020 | Migration function | TestMigrateStashMDToJSONL_Success, TestMigrateStashMDToJSONL_EmptyStash, TestMigrateStashMDToJSONL_AtomicWrite | `go test ./internal/stash/... -run TestMigrateStash -v` | RED |
| T008 / ST029 | Stash migration | TestShipmentWorkflow_StashMigration | `go test ./tests/integration/... -run TestShipmentWorkflow_StashMigration -v` | RED |
| T008 / ST031 | Integration tests | TestShipmentWorkflow_StashJSONLRoundTrip, TestShipmentWorkflow_CreateShipmentFromBacklog, TestShipmentWorkflow_ReturnBlockedItem, TestShipmentWorkflow_RehydrationConsistency, TestShipmentWorkflow_StashJSONLRehydration | `go test ./tests/integration/... -run TestShipmentWorkflow -v` | RED |

## Package Structure

All packages compile:

* `internal/config/` — shipment defaults test (no new source files)
* `internal/errors/` — 4 new sentinel errors added to existing errors.go
* `internal/core/` — shipment.go stub with 5 exported functions
* `internal/cli/` — shipment.go stub with 6 command constructors
* `internal/stash/` — jsonl.go stub with 3 exported functions
* `tests/contract/` — MCP contract tests for 5 shipment tools
* `tests/integration/` — 6 end-to-end workflow tests

## Test Helpers

```go
// internal/core/shipment_test.go
func setupShipmentWorkspace(t *testing.T) *Workspace {
    t.Helper()
    root := t.TempDir()
    ws := filepath.Join(root, ".backlogit")
    require.NoError(t, os.MkdirAll(ws, 0o755))
    require.NoError(t, config.WriteDefaults(ws))
    ctx := context.Background()
    workspace, err := NewWorkspace(ctx, root)
    require.NoError(t, err)
    t.Cleanup(func() { workspace.Close() })
    return workspace
}
```

```go
// tests/integration/shipment_workflow_test.go
func setupShipmentIntegrationWorkspace(t *testing.T) (string, *core.Workspace) {
    t.Helper()
    root := t.TempDir()
    backlogitDir := filepath.Join(root, ".backlogit")
    require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
    require.NoError(t, config.WriteDefaults(backlogitDir))
    ctx := context.Background()
    ws, err := core.NewWorkspace(ctx, root)
    require.NoError(t, err)
    t.Cleanup(func() { ws.Close() })
    return root, ws
}
```

## Notes

* T001 / ST004 (metadata catalog update) has no dedicated test in this harness because its verification depends on the shipment type appearing in defaults first. The existing `metadata_catalog_test.go` will catch regressions once ST001-ST003 are implemented.
* T002 / ST015 (artifact model fields) has no dedicated test because the existing `models.Artifact` struct already has `CustomFields` for items list storage. The shipment-specific field testing is covered by ST012 and ST013 tests.
* T004 / ST021 (rehydration update) is tested implicitly by T008's `TestShipmentWorkflow_StashJSONLRehydration`.
* T005, T006, T007 are docs-only tasks — no Go test harness generated.
* T008 / ST028 (workspace config update) is tested implicitly by the integration workspace setup which calls `config.WriteDefaults`.
* Sentinel error tests (ST011) are GREEN because the harness architect added the error definitions directly as part of stub scaffolding.
* Agent-intercom was unavailable for this run. No remote Slack notifications were sent.
