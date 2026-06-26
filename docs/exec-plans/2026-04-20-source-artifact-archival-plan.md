---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-20T00:00:00Z
    origin: .backlogit/queue/034-F.md (stash B155D9DA)
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-20-source-artifact-archival-plan.md
title: 'Workflow Hygiene: Source Artifact Archival Pattern'
---

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

## Scope Boundaries

### In Scope

* Update `.github/agents/ship.agent.md` post-merge closure protocol to add
  source artifact archival steps
* Update `.github/skills/operational-closure/SKILL.md` to add a source artifact
  cleanup section in the closure checklist
* Write a compound learning documenting the pattern and evidence
* Perform a one-time cleanup of currently stale harvested stash entries whose
  corresponding work has already shipped

### Non-Goals

* Automating source archival in Go code (the `ShipShipment` function) — the
  tools already exist; this is an agent protocol gap, not a code gap
* Modifying the harvest workflow to pre-mark stash entries for future archival
* Changing stash entry state machine (harvested → removed is already supported)
* Archiving stash entries that correspond to work still in progress

### Deferred to Implementation

* Exact wording of broadcast messages for the new closure steps
* Whether to log stash archival in the closure artifact's follow-up section or
  as a separate checklist item

## Implementation Units

### Unit 1: Ship Agent Protocol Update

**Files:** `.github/agents/ship.agent.md`
**Test files:** N/A (prompt engineering artifact — validated by plan-review)
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first — read existing protocol, then insert
**Patterns to follow:** Existing post-merge closure section structure (lines 235-250)
**Dependencies:** None

**Approach:**

Add a new step between the operational-closure invocation (step 1) and the
documentation evaluation (step 2) in the "Post-merge closure" section. The new
step instructs the ship agent to:

1. For each shipped feature in the shipment's release scope:
   a. Read `custom_fields.source_stash_id` — if present, call
      `backlogit_stash_remove` with the stash ID and reason
      `superseded by {shipment_id}`
   b. Read `custom_fields.source_deliberation_id` — if present, call
      `backlogit_archive_item` with the deliberation ID
2. Broadcast the count of source artifacts archived at info level
3. Log source artifact IDs in the closure artifact's follow-up section for
   traceability

The step must handle gracefully:
* Missing `source_stash_id` or `source_deliberation_id` (not all features
  originate from stash/deliberation)
* Stash entries already removed (idempotent — `backlogit_stash_remove` returns
  error for missing entries; skip and log)
* Deliberation artifacts already archived (idempotent — `backlogit_archive_item`
  returns error for already-archived items; skip and log)

**Verification:**

* Ship agent protocol contains explicit source artifact archival step
* Step references correct MCP tool names and custom_fields keys
* Step includes idempotent error handling guidance
* Step ordering preserves existing post-merge closure flow

### Unit 2: Operational-Closure Skill Update

**Files:** `.github/skills/operational-closure/SKILL.md`
**Test files:** N/A (prompt engineering artifact — validated by plan-review)
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first — read existing skill, then add section
**Patterns to follow:** Existing Step 2 checklist structure
**Dependencies:** Unit 1 (protocol alignment)

**Approach:**

Add a "Source Artifact Cleanup" checklist item to Step 2 (Build the Closure
Checklist). The item instructs the closure artifact to include:

* **Source artifacts archived** — list of stash IDs removed and deliberation IDs
  archived during this closure, with reason tags
* **Source artifacts not found** — list of source IDs that were referenced in
  custom_fields but no longer exist (already cleaned up)

This ensures the closure artifact documents the cleanup for auditability.

**Verification:**

* Operational-closure skill includes source artifact cleanup in the checklist
* Checklist items align with the ship agent protocol step from Unit 1

### Unit 3: Compound Learning

**Files:** `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`
**Test files:** N/A (documentation)
**Effort size:** small
**Skill domain:** docs
**Execution note:** documentation — capture evidence and pattern
**Patterns to follow:** `docs/compound/workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md` (structure template)
**Dependencies:** None (can be written in parallel with Units 1-2)

**Approach:**

Document the pattern using the compound template:

* Problem: shipped work leaves orphaned stash entries and deliberations
* Symptoms: stale harvested stash entries, orphaned deliberations in queue
* Root cause: ShipShipment archives items but ship agent protocol does not
  trace source artifacts
* Solution: post-merge closure protocol addition
* Prevention: protocol checklist item in operational-closure skill
* Evidence: 2026-04-19 stage session stale-scope sweeps, 15+ harvested stash
  entries in current DB

**Verification:**

* Compound doc follows template structure with valid frontmatter
* Cross-references the ship agent protocol change and closure skill update
* Evidence section includes specific data points

### Unit 4: One-Time Stale Harvested Stash Cleanup

**Files:** No permanent file changes — operational MCP tool calls only
**Test files:** N/A
**Effort size:** small
**Skill domain:** config (operational cleanup)
**Execution note:** migration-first — audit before removing
**Patterns to follow:** N/A
**Dependencies:** Units 1-3 (protocol should be in place before cleanup validates the pattern)

**Approach:**

1. Query all `stash_entries` with state `harvested`
2. For each, check whether the corresponding artifact (via `stash_links.item_id`)
   is archived or done
3. For entries whose work has shipped, call `backlogit_stash_remove` with reason
   `superseded — retroactive cleanup per 036-S`
4. For entries whose work is still active, leave them in `harvested` state
5. Document the cleanup in the session memory

**Verification:**

* Stash entries for shipped work are removed
* Stash entries for active work are preserved
* Cleanup count matches expectation (approximately 10-15 entries)

## Dependency Graph

```text
Unit 1 (Ship agent protocol) ──┐
                                ├──> Unit 4 (One-time cleanup)
Unit 2 (Closure skill update) ─┘
Unit 3 (Compound learning) ────────> independent
```

Units 1-3 can be implemented in any order. Unit 4 depends on Units 1-2 being
complete so the protocol is in place before validating through cleanup.

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

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change: **absent** — no Go code or MCP tool changes
* Security, auth, permission, or compliance-sensitive behavior: **absent**
* Migration, backfill, destructive data/config action, or irreversible step: **present** — Unit 4 removes stash entries (but they are archived, not deleted, so reversible)
* External integration, operator checkpoint, or external dependency: **absent**
* High runtime, rollout, or rollback risk: **absent** — prompt engineering changes only; rollback is a git revert

Requires plan hardening: no

The stash removal in Unit 4 is the only destructive action, but `backlogit_stash_remove`
archives entries to `.backlogit/archive/stash.jsonl` before removing them from the
active file, so the action is reversible. The protocol changes in Units 1-2 are
purely additive prompt engineering updates.

## Runtime Verification and Closure

* **Units 1-2**: No runtime surface change. These are agent instruction files
  validated by the next Ship cycle that exercises the updated protocol.
* **Unit 3**: Documentation only — no runtime verification needed.
* **Unit 4**: Operational cleanup — verify via `backlogit_fetch_stash` and
  `backlogit_query_sql` that harvested entries for shipped work are removed and
  active entries are preserved.

Operational closure for this shipment should confirm:
* Ship agent protocol includes source artifact archival step
* Operational-closure skill includes source artifact cleanup checklist item
* Compound learning is searchable in `docs/compound/`
* Stale harvested stash entries for shipped work are cleaned up

## Learnings Applied

* `docs/compound/workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md`:
  Reinforces that agents must actually call tools, not just report actions. Applied
  to the protocol by requiring verification of tool call results.
* `docs/compound/workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md`:
  Reinforces that workflow gates exist for a reason. Applied by ensuring the
  archival step is a mandatory post-merge closure step, not optional.

## Standards Check

* **Prompt engineering standards**: Updates follow `prompt-builder.instructions.md`
  authoring standards for agent and skill files.
* **Writing style**: Updates follow `writing-style.instructions.md` conventions.
* **Markdown**: Updates follow `markdown.instructions.md` structure requirements.
* **No Go code**: No Go conventions apply — this is purely workflow documentation.
* **Constitution compliance**: No principles are violated. The changes align with
  Principle IV (workspace containment — all archival goes through tool surface)
  and Principle VII (CQRS — stash is a transient queue, archival is the correct
  lifecycle transition).

## Plan Review

**Date:** 2026-04-20
**Reviewers:** Constitution Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist (GPT-5.4)
**Gate Decision:** PASS

### Summary

4 findings total: 0 P0, 0 P1, 0 P2, 4 P3. Plan is sound and ready for harvest.

### P0 — Critical

None.

### P1 — High

None.

### P2 — Moderate

None.

### P3 — Low (advisory)

**PR-001** (Learnings Researcher, Unit plan-level, P3): The `Learnings Applied`
section references two compound docs. `docs/compound/go-patterns/f015-shipment-stash-patterns.md`
documents stash storage format patterns (JSONL dual-reader, SQLite JSON array
normalization) that provide useful background context for Unit 4's cleanup
operations. Not critical since no Go code is written, but the implementer should
be aware of stash storage mechanics.
*Recommendation:* Implementer awareness only — no plan revision needed.

**PR-002** (Scope Boundary Auditor, Unit 4, P3): Unit 4's verification criteria
state "approximately 10-15 entries" but the actual count depends on which
harvested entries have corresponding shipped work. The verification should query
the current count before cleanup to set a concrete baseline.
*Recommendation:* During implementation, query harvested entry count and
cross-reference against archived items before executing removals.

**PR-003** (Architecture Strategist, Unit 2, P3): Adding backlogit-specific
source artifact cleanup to the generic `operational-closure` skill template
slightly couples a generic skill to backlogit-specific custom_fields. This is
acceptable because backlogit is both the product and the workflow system, but if
the skill is ever extracted to other projects, this section would not apply.
*Recommendation:* Acceptable coupling for this workspace. No change needed.

**PR-004** (Constitution Reviewer, Unit 1, P3): The protocol should clarify
what `source_deliberation_id` references — it may point to a deliberation
artifact ID (an `.md` file in `.backlogit/`) or to a `stash_entries.deliberation_id`
link that was never promoted to a full artifact. The archival step should verify
the deliberation artifact exists before calling `backlogit_archive_item`.
*Recommendation:* Add existence check guidance to Unit 1's error handling notes.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| PR-001 | Learnings Researcher | claude-haiku-4.5 |
| PR-002 | Scope Boundary Auditor | claude-haiku-4.5 |
| PR-003 | Architecture Strategist | gpt-5.4 |
| PR-004 | Constitution Reviewer | claude-haiku-4.5 |

### Next Steps

Gate: **PASS**. All findings are P3 advisory. Proceed to `harvest` to decompose
this plan into backlogit feature, task, and subtask items.
