---
title: "Azure DevOps Sync Design Session Memory"
doc_type: memory
schema_version: "1.0"
ingested_at: "2026-08-20T00:00:00Z"
---

## Task

Assess the `azd-backlogbldr` and `azd-backlogloader` proof-of-concept
workspaces, determine how their useful behavior can be absorbed into backlogit,
and design an Azure DevOps-only first synchronization release. Jira and GitHub
Issues or Projects remain future extensions.

## Completed

1. Investigated both Python reference workspaces and the relevant backlogit
   configuration, artifact, migration, mutation-recovery, CLI, MCP, and SQLite
   contracts.
2. Produced a proceed recommendation with medium confidence in
   `docs/decisions/2026-08-20-azure-devops-sync-spike.md`.
3. Produced the test-first implementation plan in
   `docs/exec-plans/2026-08-20-azure-devops-sync-plan.md`.
4. Completed a seven-persona plan review. All P0 and P1 findings were
   remediated; lower-severity follow-ups remain advisory.
5. Completed a focused architecture consistency pass and corrected package
   purity, key-format, retry, frozen-plan, and orphan-reporting
   inconsistencies.
6. Aligned the spike's package boundary and provider interface with the final
   plan while preserving the history of superseded decisions.

## Decisions and Rationale

* V1 is an explicit, one-way, plan-then-apply push from backlogit to Azure
  DevOps; no pull, bidirectional merge, lifecycle-hook writes, or remote
  deletion
* Backlogit Markdown remains authoritative, and shipments select the artifacts
  to publish without becoming remote work items
* Configurable local artifact types map to arbitrary Azure DevOps work item
  types through `integrations/azure-devops.yaml`
* Durable remote identity uses an immutable 128-bit random key serialized as
  32 lowercase hexadecimal characters in `custom_fields.external.key`
* Frozen content-addressed plans contain all write semantics; apply cannot
  synthesize actions or introduce a drift override
* `internal/extsync` owns core-independent, filesystem-free computation and a
  translation-only Azure DevOps adapter; `internal/ado` owns transport;
  `internal/core` owns orchestration and persistence
* Jira and GitHub receive no implementation units until the Azure DevOps
  workflow is implemented and validated

## Files Modified

* `docs/decisions/2026-08-20-azure-devops-sync-spike.md`
* `docs/exec-plans/2026-08-20-azure-devops-sync-plan.md`
* `docs/memory/2026-08-20/azure-devops-sync-design-memory.md`

## Failed or Rejected Approaches

* Porting either Python proof of concept wholesale
* Reusing inbound `.backlogit/migration.yaml` for outbound synchronization
* Correlating remote items by mutable artifact ID
* Combining planning and applying in one command
* Retrying outcome-unknown writes blindly
* Hardcoding Feature, User Story, and Task hierarchy semantics

## Open Questions

* Confirm the exact Azure DevOps status and `typeKey` returned by a failed
  JSON Patch `/rev` test against a scratch organization
* Confirm the conservative REST defaults documented in the spike against a
  live Azure DevOps project
* Determine whether a scratch organization and credential will be available
  for runtime verification

## Next Steps

1. Obtain operator approval for the spike and implementation plan.
2. On explicit instruction, harvest the plan into backlog items and a queued
   shipment.
3. Begin implementation test-first with configuration, REST transport, and
   pure mapping units.

## Compaction Assessment

`docs/memory/` is below the configured compaction thresholds at 35 files and
approximately 162 KB. The current plan is active design work and was preserved
in full. Existing older plans and closure records were not moved because they
are outside this task's scope and archive moves require a separate,
operator-approved maintenance pass.
