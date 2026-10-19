---
schema_version: "1.0"
doc_type: plan
title: "Decided plan: Workflow Hygiene: Source Artifact Archival Pattern"
description: "Decided-plan consolidation of 2026-04-20-source-artifact-archival-plan.md: final decisions, implementation units, risks, and review verdict."
chunk_strategy: h1-h2-h3
source: docs/exec-plans/2026-10-18-source-artifact-archival-decided-plan.md
---

# Decided plan: Workflow Hygiene: Source Artifact Archival Pattern

Consolidated from `docs/archive/plans/2026-04-20-source-artifact-archival-plan.md` (P-020 compaction, 2026-10-18). The owning release units are archived. Full deliberation history stays in the archived original.

## Problem Frame

When the Ship agent completes post-merge closure for a shipment, it archives the
implemented features, tasks, and the shipment artifact itself. However, it does
not trace back to the **source artifacts** that originated the work: the stash
entry that was harvested into a feature, and the deliberation artifact that
shaped the approach.

This creates stale-scope accumulation:

* Harvested stash entries remain in `stash_entries` with state `harvested`
  indefinitely. Currently 15+ entries are in this state.
* Deliberation artifacts (when created) remain in the queue after their scope
  ships, potentially confusing future Stage triage.
* Source traceability fields (`source_stash_id`, `source_deliberation_id`)
  already exist in artifact `custom_fields` but are never consumed during
  closure.

Evidence from 2026-04-19 stage session: three stale-scope sweeps in one session
identified deliberations and stash entries that should have been archived when
their corresponding shipments closed.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Ship agent post-merge closure traces shipped features back to source stash entries and archives them | 034-F stash text |
| R2 | Ship agent post-merge closure traces shipped features back to source deliberations and archives them | 034-F stash text |
| R3 | The pattern is documented as a compound learning for institutional knowledge | 034-F stash text |
| R4 | No Go code changes required — leverage existing MCP tools (`backlogit_archive_item`, `backlogit_stash_remove`) | Research finding |
| R5 | Protocol additions must be consistent with existing compound learnings (stash discipline, follow-up verification) | docs/compound/workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md |

## Implementation units

* **Implementation Units**: 
* **Unit 1: Ship Agent Protocol Update**: **Files:** `.github/agents/ship.agent.md` **Test files:** N/A (prompt engineering artifact — validated by plan-review) **Effort size:** small **Skill domain:** docs **Execution note:** characterization-first — read existing protocol, then insert **Patterns to ...
* **Unit 2: Operational-Closure Skill Update**: **Files:** `.github/skills/operational-closure/SKILL.md` **Test files:** N/A (prompt engineering artifact — validated by plan-review) **Effort size:** small **Skill domain:** docs **Execution note:** characterization-first — read existing skill, then add ...
* **Unit 3: Compound Learning**: **Files:** `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md` **Test files:** N/A (documentation) **Effort size:** small **Skill domain:** docs **Execution note:** documentation — capture evidence and pattern **Patterns to ...
* **Unit 4: One-Time Stale Harvested Stash Cleanup**: **Files:** No permanent file changes — operational MCP tool calls only **Test files:** N/A **Effort size:** small **Skill domain:** config (operational cleanup) **Execution note:** migration-first — audit before removing **Patterns to follow:** N/A ...

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Agent protocol change, not Go code change | Source traceability fields and archival tools already exist. The gap is that the ship agent does not use them during closure. Adding Go code would duplicate tool-level logic that agents should orchestrate. | Add archival logic to `ShipShipment` Go function — rejected because it would couple shipment lifecycle code to stash cleanup and violate the CQRS boundary (stash is a transient queue, not a shipment concern). |
| D2 | Use `backlogit_stash_remove` not a new "archive stash" command | `stash_remove` already archives to `.backlogit/archive/stash.jsonl` and sets DB state to `removed`. This is the correct semantic for superseded entries. | Add a new `stash_archive` command — rejected because `stash_remove` already archives internally. |
| D3 | One-time retroactive cleanup as a separate unit | Separates the protocol fix (forward-looking) from the data cleanup (backward-looking). The protocol must be in place first to prove the pattern works. | Skip retroactive cleanup — rejected because 15+ stale entries will confuse future triage indefinitely. |
| D4 | Add to operational-closure skill, not just ship agent | The closure artifact should document what source artifacts were cleaned up for auditability. The ship agent does the work; the closure skill documents it. | Only update ship agent — rejected because the closure artifact would have no record of the cleanup, making it invisible to reviewers. |

## Risks and Caveats

* **Stash remove for already-removed entries**: `backlogit_stash_remove` will
  error if the entry is already removed. The protocol must instruct the agent
  to handle this gracefully (skip and log).
* **Deliberation archival for already-archived items**: Same pattern —
  `backlogit_archive_item` errors on already-archived items. Skip and log.
* **Missing source traceability fields**: Not all features have
  `source_stash_id` (e.g., features created directly without stash harvest).
  The protocol must check for field presence before attempting cleanup.
* **Compound learning from ship-agent-unrealized-follow-up-stash**: The same
  discipline applies here — the agent must actually call the tools, not just
  report that it did. The protocol should reference verification of the tool
  call result.

## Plan Review

**Date:** 2026-04-20
**Reviewers:** Constitution Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist (GPT-5.4)
**Gate Decision:** PASS

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| PR-001 | Learnings Researcher | claude-haiku-4.5 |
| PR-002 | Scope Boundary Auditor | claude-haiku-4.5 |
| PR-003 | Architecture Strategist | gpt-5.4 |
| PR-004 | Constitution Reviewer | claude-haiku-4.5 |

## Traceability

| Original path | Archive path | Disposition |
|---|---|---|
| `docs/exec-plans/2026-04-20-source-artifact-archival-plan.md` | `docs/archive/plans/2026-04-20-source-artifact-archival-plan.md` | archived (decided-plan) |
