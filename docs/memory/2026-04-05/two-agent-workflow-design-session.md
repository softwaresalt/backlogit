---
title: "Session Memory: Two-Agent Workflow Design"
description: "Captures the full design deliberation for refactoring backlogit from N agents to a two-agent (groomer + shipper) model with four-stage pipeline."
ms.date: 2026-04-05
ms.topic: reference
stash_id: 84CAE804
related_stash_ids:
  - D7CF4B20
  - F5FC7303
  - 3C7BCC11
---

## Session Context

This session developed the design for a fundamental workflow refactor of the
backlogit agent harness. The user identified that the current multi-agent model
(backlog-harvester, harness-architect, build-orchestrator, pr-review, fix-ci,
doc-ops) requires excessive manual handoffs. The design converged on a two-agent
model with a four-stage storage pipeline.

## Stash Entry

Created stash entry `84CAE804` (priority: high, kind: feature) titled
"Two-agent workflow refactor".

## Deliberation Status

A deliberation artifact was attempted via `backlogit deliberate 84CAE804` but
failed because the deliberation template does not exist in
`.backlogit/templates/`. This needs to be resolved before the deliberation
artifact can be created. The full design content below should be captured as the
deliberation artifact once the template issue is fixed.

## Decisions Made

### 1. Four-Stage Pipeline (AGREED)

```
STASH ──→ BACKLOG ──→ SHIPMENT ──→ SHIPPED
 stash.jsonl   queue/*.md     queue/S*.md    archive/*.md
               status:queued  status:active  status:done
```

- **Stash**: Raw ideas in `.backlogit/stash.jsonl` (JSONL format, rehydrated into
  SQLite)
- **Backlog**: Groomed artifacts with full F/T/ST hierarchy in
  `.backlogit/queue/` with `status: queued` or `blocked`
- **Shipment**: Active work grouped in a shipment manifest (`S` prefix artifact),
  one shipment = one branch = one PR
- **Archive**: Completed items moved to `.backlogit/archive/` with terminal
  statuses

### 2. Two Agents (AGREED)

**Groomer agent**: Transforms stash ideas into groomed backlog items.
Orchestrates: deliberate → impl-plan → plan-review → harvest skills.
Output: F/T/ST artifacts with status `queued`.

**Shipper agent**: Takes a shipment from `queued` through every gate to a merged
PR. Orchestrates: harness-architect → build-feature → review → fix-ci →
pr-lifecycle → compound skills.
Input: Shipment ID.

### 3. Stash Format (AGREED)

JSONL, not markdown. Each entry is one JSON line. Format:
```jsonl
{"id":"84CAE804","kind":"feature","priority":"high","text":"Two-agent workflow refactor","created_at":"2026-04-05T17:28:25Z"}
```
Rehydrated into SQLite for query access. Aligns with existing events.jsonl and
telemetry.jsonl conventions.

### 4. Shipment Artifact (AGREED)

New artifact type `shipment` with prefix `S`. Groups N ready items into one
shippable unit. 1:1 mapping to branch and PR. When an item is blocked mid-
shipment, it is removed from the manifest and returned to the backlog with
`status: blocked` plus `blocked_reason` in frontmatter.

### 5. Shipment Granularity (AGREED)

Keep it 1:1 — one shipment per PR/branch. Be granular and iterative rather than
batching too many items.

### 6. Merge Approval (AGREED)

Require user confirmation to merge. Auto-merge deferred to future configurable
setting when process confidence is established.

### 7. File Naming Convention (AGREED, from stash 3C7BCC11)

File naming follows the ID convention based on hierarchy level with no prefix
usage in file names. Examples:
- Feature (level 1): `001-FA.md`
- Task under feature (level 2): `001.001-T.md`
- Second task: `001.002-T.md`
- Subtask (level 3): `001.001.001-S.md`
- Bug: `001.001.001-B.md`
- Review: `001.001-R.md`
- Event log: `001.001-T.jsonl` (same root, different extension)

ID = filename. Type suffix at end of ID/filename. Hierarchy expressed by dot-
separated level numbers. Archive should also be refactored to match.

### 8. No Migration Code Needed (AGREED)

Project is in alpha. No public users need migration support. Only this repo's
own data needs to be converted.

### 9. Bootstrap Strategy (NEEDS RESOLUTION)

The current agents and skills would need to be refactored using a workflow that
relies on those same agents and skills. Options discussed:

**Option A: Sidecar approach** — Build the new groomer and shipper agents as new
files alongside the existing agents. Build the new harvest, harness-architect
(skill version), and pr-lifecycle skills alongside their agent counterparts.
Once the new harness is validated, deprecate and remove the old agents. This
avoids the bootstrapping paradox.

**Option B: Ad hoc manual implementation** — Implement the refactor manually
without using the current agent pipeline, since the pipeline is the thing being
refactored.

The user leaned toward Option A (sidecar) if feasible, falling back to Option B.

## Skills Reclassification Summary

| Current | Current Role | Proposed | Rationale |
|---|---|---|---|
| deliberate | skill | skill (unchanged) | Leaf executor |
| impl-plan | skill | skill (unchanged) | Leaf executor |
| plan-review | skill | skill (unchanged) | Spawns persona subagents |
| build-feature | skill | skill (unchanged) | Leaf executor |
| review | skill | skill (unchanged) | Spawns persona subagents |
| fix-ci | skill | skill (unchanged) | Leaf executor |
| compound | skill | skill (unchanged) | Spawns analyzer subagents |
| compact-context | skill | skill (unchanged) | Leaf executor |
| backlog-harvester | **agent** | **harvest skill** | Phase 3 only; planning + review handled by groomer |
| harness-architect | **agent** | **harness-architect skill** | Mechanical code gen |
| build-orchestrator | **agent** | **retired** | Absorbed by shipper |
| pr-review | **agent** | **pr-lifecycle skill** | Leaf PR creation operation |
| doc-ops | agent | skill (optional) | Invoked by shipper for docs |
| memory | agent | agent (unchanged) | Cross-cutting |
| go-engineer | agent | agent (unchanged) | Domain expert |
| go-mcp-expert | agent | agent (unchanged) | Domain expert |

## Blocked Item Flow

When shipper hits a blocker mid-shipment:
1. Remove item from shipment manifest
2. Set `status: blocked` with `blocked_reason` in frontmatter
3. Item stays in `.backlogit/queue/` — still a groomed artifact
4. Queryable via `SELECT * FROM items WHERE status IN ('queued', 'blocked')`
5. When blocker resolves, set back to `queued` for re-shipment

## Status Semantics

| Status | Stage | Meaning |
|---|---|---|
| *(stash)* | Stash | Raw idea, pre-artifact |
| `queued` | Backlog | Groomed, planned, reviewed, ready to ship |
| `active` | Shipment | Claimed by shipment, being built |
| `blocked` | Backlog | Removed from shipment, awaiting resolution |
| `review` | Shipment | Code complete, in review/CI |
| `done` | Shipped | Merged, acceptance criteria met |
| `archived` | Archive | Terminal |

## Constitution Compliance

| Principle | Impact |
|---|---|
| I. Type-Safe Go | Shipment struct with validator tags; JSONL stash parsed into typed Entry structs |
| II. MCP Protocol Fidelity | New tools: backlogit_create_shipment, backlogit_get_shipment, backlogit_list_shipments |
| III. Test-First | Contract tests for shipment CRUD; integration tests for groomer-to-shipper handoff |
| IV. Workspace Containment | stash.jsonl and shipment manifests inside .backlogit/ |
| VII. CQRS | Stash JSONL is append-only source of truth; shipments are markdown+frontmatter; both rehydrate into SQLite |
| VIII. Git-Friendly | JSONL is line-per-entry; shipment manifests use stable YAML ordering |
| IX. Agent Context Efficiency | Shipment manifest gives shipper minimal scope boundary |

## Open Items / Next Steps

1. **Fix deliberation template** — `.backlogit/templates/` is missing the
   deliberation template. Create it so `backlogit deliberate` works.
2. **Create deliberation artifact** — Run `backlogit deliberate 84CAE804` once
   the template exists. Populate with the design content from this session.
3. **Decide bootstrap approach** — Confirm sidecar vs. ad hoc for implementing
   the refactor.
4. **Plan the work** — Feed the deliberation artifact through impl-plan (or do
   it manually if using ad hoc approach) to produce the implementation plan.
5. **Implement stash JSONL migration** — Convert `.backlogit/queue/.stash.md` to
   `.backlogit/stash.jsonl`.
6. **Implement shipment artifact type** — Add to config.yaml, header-def.yaml,
   and core logic.
7. **Implement file naming convention** — Per stash 3C7BCC11 decisions.
8. **Create groomer agent** — New `.github/agents/groomer.agent.md`.
9. **Create shipper agent** — New `.github/agents/shipper.agent.md`.
10. **Extract harvest skill** — From backlog-harvester agent Phase 3.
11. **Extract harness-architect skill** — From harness-architect agent.
12. **Extract pr-lifecycle skill** — From pr-review agent.
13. **Validate new pipeline end-to-end** — Test groomer → backlog → shipment →
    shipper flow.
14. **Deprecate old agents** — Mark backlog-harvester, harness-architect,
    build-orchestrator, pr-review as deprecated.

## Files Modified This Session

- `.backlogit/queue/.stash.md` — Added stash entry 84CAE804

## Failed Approaches

- `backlogit deliberate 84CAE804` failed due to missing deliberation template in
  `.backlogit/templates/`. The deliberation artifact type may not be configured
  in config.yaml either.
