---
title: "Compacted memory: 2026-08-20 Azure DevOps sync design session"
date_range: "2026-08-20 to 2026-08-20"
doc_type: compacted-memory
source_count: 1
---

# Compacted memory: 2026-08-20

Compacts 1 source file. The original was moved to docs/archive/memory/ (not deleted).

## Decisions made

- Azure DevOps sync V1 is an explicit, one-way, plan-then-apply push from backlogit. No pull, no bidirectional merge, no lifecycle-hook writes, no remote deletion.
- Backlogit Markdown stays authoritative. Shipments select which artifacts to publish and never become remote work items.
- Local artifact types map to arbitrary Azure DevOps work item types through integrations/azure-devops.yaml.
- Durable remote identity is an immutable 128-bit random key, serialized as 32 lowercase hex characters in custom_fields.external.key.
- Frozen, content-addressed plans hold all write semantics. Apply cannot synthesize actions or add a drift override.
- Package boundaries: internal/extsync holds core-independent, filesystem-free computation and a translation-only Azure DevOps adapter; internal/ado holds transport; internal/core holds orchestration and persistence.
- Jira and GitHub get no implementation units until the Azure DevOps workflow is implemented and validated.

## Files modified or created

- docs/decisions/2026-08-20-azure-devops-sync-spike.md (spike; proceed recommendation at medium confidence)
- docs/exec-plans/2026-08-20-azure-devops-sync-plan.md (test-first implementation plan)
- docs/memory/2026-08-20/azure-devops-sync-design-memory.md (source record)

## Key learnings

- A seven-persona plan review found P0 and P1 items; all were remediated. Lower-severity follow-ups stayed advisory.
- A focused architecture pass corrected package purity, key format, retry, frozen-plan, and orphan-reporting inconsistencies.
- The spike's package boundary and provider interface were aligned with the final plan; superseded decisions were kept as history.

## Failed or rejected approaches

- Porting either Python proof of concept wholesale.
- Reusing inbound .backlogit/migration.yaml for outbound synchronization.
- Correlating remote items by mutable artifact ID.
- Combining planning and applying in one command.
- Retrying outcome-unknown writes blindly.
- Hardcoding Feature, User Story, and Task hierarchy semantics.

## Outcomes and open items

- Design only. Operator approval of the spike and plan was pending. No implementation had started.
- Open: confirm the Azure DevOps status and typeKey returned by a failed JSON Patch /rev test against a scratch organization; confirm REST defaults against a live project; determine whether a scratch organization and credential will be available for runtime verification.
- Compaction assessment at the time: 35 files, about 162 KB, below thresholds. Older plans and closure records were not moved.

## Sources

- docs/memory/2026-08-20/azure-devops-sync-design-memory.md
