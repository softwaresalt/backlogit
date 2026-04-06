---
description: "Rules for maintaining architecture documentation and progressive disclosure in backlogit"
applyTo: '**/ARCHITECTURE.md,**/docs/**,**/AGENTS.md'
---

# Architecture Documentation Instructions

Use these rules when maintaining architecture and durable knowledge artifacts in this workspace.

## Progressive disclosure

1. `AGENTS.md` is a map, not a manual. Keep it short and use it to point to deeper sources of truth.
2. `docs/` is the durable knowledge root for this repository.
3. Depth should match complexity. Small areas need light mapping. Cross-cutting systems need more.

## Directory roles

Use the durable docs tree by purpose:

* `docs/ARCHITECTURE.md` for top-level domain and dependency direction
* `docs/design-docs/` for graduated design decisions and rationale
* `docs/product-specs/` for product requirements and acceptance criteria
* `docs/compound/` for durable implementation learnings
* `docs/exec-plans/` for implementation plans
* `docs/decisions/` for long-lived decision and spike artifacts
* `docs/memory/` for session memory and checkpoints
* `docs/closure/` for runtime verification and closure records

## Boundary between docs and backlog

The docs tree and `.backlogit/` serve different lifecycles.

* `.backlogit/` holds active work items and stash state.
* `docs/` holds durable knowledge that should outlive a single queue item.

Do not duplicate active backlog content into `docs/`. Distill long-lived insight instead.

## Knowledge graduation

When backlog work finishes, graduate only durable knowledge:

* architectural decisions into `docs/design-docs/`
* domain maps into `docs/ARCHITECTURE.md`
* product requirements into `docs/product-specs/`
* implementation learnings into `docs/compound/`
* spike findings into `docs/decisions/`

Graduation is distillation, not copying execution logs.

## Staleness rules

Documentation is stale when it references paths, commands, modules, or workflows that no longer exist.

When code changes invalidate architecture docs, update the docs in the same change.
