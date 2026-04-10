---
title: "008-S Harness Complete — Build Phase Starting"
date: 2026-04-10
origin: session-memory
status: active
---

## Context

Shipment 008-S (Governance and Archival Policies) is active on branch
`025-workspace-governance-integrity` (commit 8304e72).

## Harness Phase Complete

All harness files created and committed. Build order:

1. **Parallel** (no deps): 025.014-T ✅ done, 025.013-T, 025.016-T, 025.011-T, 025.012-T
2. **After Unit 2**: 025.015-T
3. **After Unit 1** (already done): 025.018-T
4. **After Unit 4**: 025.017-T

## Harness File Map

| Task     | Harness File(s)                                      | Cmd Pattern                                                |
|----------|------------------------------------------------------|------------------------------------------------------------|
| 025.014-T | `025_archive_harness_test.go`                       | DONE — all 3 GREEN                                         |
| 025.013-T | `025_hierarchy_harness_test.go`                     | `-run "TestCreateArtifact_RejectsLevel2|TestLevelForType_NilLayout"` |
| 025.015-T | `025_harvest_safety_harness_test.go`                | `-run "TestHarvestStashEntry_(Rejects|Preserves)"` |
| 025.016-T | `doctor.go` stub + `025_doctor_harness_test.go`     | `-run "TestDoctor_"` |
| 025.017-T | `025_doctor_contract_harness_test.go`               | `./tests/contract/... -run "TestDoctorTool_"` |
| 025.018-T | `shipment_verify.go` stub + `025_postship_verify_harness_test.go` | `-run "TestShipShipment_(Fails|Succeeds)WhenQueue"` |

## Key Technical Context

- `validateArtifactParent` (artifacts.go:265): extend to check `LevelForType >= 2` requires parent_id
- `LevelForType` (hierarchy.go:188): add nil-guard before `layout.Levels` iteration
- `HarvestStashEntry` (stash.go:265-276): move `removeStashEntry`+`writeStashEntries` AFTER `CreateArtifact`
- `Doctor` (doctor.go): implement orphan detection + duplicate detection; check event log for `returned_to_backlog`
- `VerifyPostShipConsistency` (shipment_verify.go): walk queue dir, check no file has prefix matching archived IDs
- After Unit 4 Doctor is done: register `backlogit_doctor` tool in `internal/mcp/tools.go`
- After all implementation: call `ShipShipment` → `VerifyPostShipConsistency` at end (Unit 6 wires it in)

## Stash State

Entry `02601E73` was harvested in prior session. No remaining relevant stash entries.

## Branch / Commit

- Branch: `025-workspace-governance-integrity`
- Last commit: 8304e72 (harness scaffold)
- Base: main
