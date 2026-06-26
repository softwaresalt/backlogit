---
chunk_strategy: h1-h2-h3
description: Documents why backlogit should incubate the groomer and shipper workflow internally while autoharness consumes only stable backlogit capabilities.
doc_type: learning
docline:
    category: workflow_issue
    component: task_manager
    date: 2026-04-05T00:00:00Z
    file_path: docs/memory/2026-04-05/two-agent-workflow-design-session.md
    message: Do not promote backlogit's experimental two-agent workflow into autoharness until the external contract is proven.
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: true
    root_cause: schema_mismatch
    severity: medium
    tags:
        - backlogit
        - autoharness
        - workflow
        - groomer
        - shipper
        - operating-model
        - contract-boundary
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md
title: Keep the stable backlogit contract separate from the emerging two-agent workflow
---

## Problem

The main problem is not whether the proposed two-agent `groomer` and `shipper`
workflow is promising. It is. The problem is deciding what should be treated as
stable platform contract versus what should remain incubating inside
`backlogit` until it is implemented and validated.

`autoharness` already integrates with `backlogit` through a stable overlay
contract: query-driven backlog lookup, queue-aware work selection, dependency
operations, memory and checkpoints, comments, and commit traceability. The new
two-agent model changes internal orchestration, storage stages, artifact types,
and likely tool expectations. Pulling that model into `autoharness` before it is
proven would couple a global harness framework to a moving target.

## Symptoms

The current multi-agent model in `backlogit` creates handoff pressure across
planning, harnessing, implementation, review, and CI workflows.

The design session in
[`docs/memory/2026-04-05/two-agent-workflow-design-session.md`](../../memory/2026-04-05/two-agent-workflow-design-session.md)
captures the intended replacement:

* a four-stage pipeline: stash → backlog → shipment → shipped
* two top-level agents: `groomer` and `shipper`
* shipment artifacts that scope one branch and one pull request
* sidecar-versus-ad-hoc bootstrap work that is still unresolved

That same session also shows the workflow is not done yet:

* the deliberation artifact could not be created at the time because the
  deliberation template was missing
* the bootstrap strategy was still open
* shipment artifacts, tool surface, and end-to-end validation were still listed
  as next steps

## What Did Not Work

Two incorrect reactions became clear.

First, flattening `backlogit` into generic CRUD-only backlog behavior would lose
the higher-leverage workflow surface that already exists today. That would throw
away query, queue, dependency, memory, checkpoint, and traceability gains that
the harness can use immediately.

Second, promoting the planned two-agent workflow into `autoharness` now would be
premature. The current design is a strong direction, but it is still an internal
backlogit evolution rather than a stable external contract.

## Solution

Treat the boundary between `backlogit` and `autoharness` as a contract boundary.

Use `backlogit` as the incubator for the next operating model:

* implement and validate `groomer` and `shipper` inside `backlogit`
* settle shipment artifacts, stash storage shape, and new tool semantics there
* prove the workflow with tests and real runs before exporting it

Use `autoharness` as the consumer of the stable contract:

* keep the generic backlog abstraction in place
* keep the `backlogit` capability pack focused on proven features
* promote only stable surfaces such as MCP and CLI operations, artifact
  contracts, status semantics, overlay behavior, and tuner detection rules

This lets `autoharness` move forward now on documentation, compatibility
contracts, and first-party backlogit guidance without freezing experimental
workflow details into templates.

## Why This Works

This works because it matches the architecture of both repositories.

`backlogit` is the right place to evolve agent workflow state, queue semantics,
and orchestration boundaries because it owns the work-item store, query engine,
and runtime tool surface.

`autoharness` is the right place to consume only the durable parts of that
surface because it is a global harness composer intended to stay reusable across
multiple backlog tools and workspace shapes.

That separation lowers risk in both directions:

* `backlogit` can evolve quickly without destabilizing installed harnesses
* `autoharness` can deepen backlogit support without overfitting to an
  unfinished design
* future promotion into `autoharness` becomes a deliberate graduation step, not
  an accidental copy of in-progress repository behavior

## Prevention

Use these guardrails for future workflow changes:

* Treat new backlogit workflow ideas as internal until their external contract is
  clear.
* Promote only stable surfaces into `autoharness`: tools, artifact types,
  registry mappings, status semantics, verification checks, and tuning signals.
* Keep experimental agent names and phase choreography out of `autoharness`
  templates until backlogit proves them end to end.
* Use sidecar rollout when refactoring the harness engine itself so the current
  workflow is not forced to disappear before its replacement works.
* Revisit the promotion decision only after backlogit completes implementation,
  contract coverage, and end-to-end validation of the two-agent flow.

## Related Solutions

* [`docs/memory/2026-04-05/two-agent-workflow-design-session.md`](../../memory/2026-04-05/two-agent-workflow-design-session.md)
  records the current design direction and open implementation tasks.
* [`docs/rationale.md`](../../rationale.md) explains why backlogit already acts
  as an agent operating substrate rather than a simple markdown task tracker.
* [`docs/configuration.md`](../../configuration.md) documents the metadata
  catalog and stable discovery surfaces that agents can consume today.
* [`docs/workflow.md`](../../workflow.md) records the current stable workflow
  surface that remains safe for autoharness integration.