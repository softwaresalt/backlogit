---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Plan-review dispatch and fail-closed waiver governance'
source: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
doc_type: decision
description: 'Decision to use real reviewer subagent dispatch when available and permit only a new explicit, auditable, consumable operator waiver when it is not.'
docline:
    date: 2026-07-14T18:30:00Z
    decision_status: decided
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
    linked_deliberation: 052-DL
---

# Plan-review Dispatch and Fail-closed Waiver Governance

## Problem Frame

Stage must run `plan-review` before harvest, but recent API-launched Stage sessions had no agent or task dispatch tool. Inline single-agent assessment cannot satisfy a skill that requires independent persona outputs. The same planning surface also omits the constitution's required labeled `## Constitution Check`.

Success means formal review is executable in supported environments, its provenance is inspectable, missing capability blocks harvest by default, and any exception is operator-authorized, bounded, and durably consumed. Hosted Copilot review may supplement the gate but cannot impersonate a local reviewer persona.

## Research Findings

- `.github/skills/plan-review/SKILL.md` requires all always-on personas plus triggered personas and appends their findings to the plan.
- `.github/agents/.stage.agent.md` declares the abstract `agent` tool, `max_subagent_tier: 3`, and `subagent_depth: 2`.
- `plugin/agents/stage.agent.md` declares the concrete `agent/runSubagent` tool and `subagent_depth: 2`.
- `.github/agents/_orchestrator.agent.md` permits depth 3: Orchestrator → Stage → skill → reviewer persona.
- Reviewer definitions are read-only leaf executors with `subagent_depth: 0`; the depth and role policies therefore permit dispatch.
- This API invocation exposes no agent/task dispatch tool. The defect is invocation-tool-surface degradation, not a structurally impossible repository design.
- Existing Stage inputs `skip_review` and `force_harvest_no_gates` can bypass provenance unless removed or routed through the same fresh-waiver validation.
- The three `.github` targets are generated from external autoharness templates and require a tracked upstream handoff.

## Options Evaluated

### Option A: Dispatch only, no contingency

Use `agent/runSubagent` where present and halt everywhere else. Strongest, but it provides no explicit exceptional path for an operator who understands and accepts a plan-specific residual risk.

### Option B: Treat inline or hosted review as equivalent

Allow one caller model or GitHub Copilot review to stand in for personas. Rejected because provenance would be false and independent focus would be lost.

### Option C: Dispatch first with a narrow waiver contingency

Run formal personas whenever dispatch is exposed. When it is absent, fail closed unless the operator supplies a new plan-scoped, auditable waiver that Stage can reserve and consume exactly once. Record waiver mode distinctly from PASS.

## Decision

Choose Option C. Supported Copilot environments already have a viable dispatch topology; implementation must make that path explicit and observable rather than silently falling back.

A formal gate record must include:

1. `review_mode: formal`;
2. every required persona name and agent definition path;
3. dispatch/return evidence or invocation identifier;
4. model/provider when the runtime exposes it, otherwise `unknown`;
5. each persona's structured findings and disposition;
6. merged severities and final PASS, ADVISORY, or FAIL.

A missing, failed, or unattributed required persona makes the gate FAIL. Inline self-assessment must be labeled `informal_single_agent`; it never counts as formal evidence. Hosted review remains supplemental.

A future waiver is valid only when the plan gate record includes `review_mode: operator_waiver`, a unique `waiver_id`, exact plan path and digest, operator authorization reference and authorizer, missing capability, reason, UTC issue time, expiry, acknowledged residual risk, and intended disposition. Generic workflow commands are never waiver signals.

Before any harvest mutation, Stage must append a durable waiver reservation to the same plan: `waiver_id`, `state: reserved`, `reserved_at`, `reserved_by_stage_session`, and plan digest. Existing reserved or consumed records reject reuse. After successful harvest, Stage updates that record to `state: consumed` with `consumed_at`, `consumed_by_harvest_ids`, and `shipment_id`. A crash after reservation remains fail-closed and requires a new explicit operator decision; it never silently frees the waiver.

The legacy `skip_review: true` and `force_harvest_no_gates: true` values are not bypasses. If retained for compatibility, they only request the operator-waiver path and cannot reach harvest without a fresh valid waiver and successful reservation.

## Current Gate State

No waiver was authorized for either current plan. The generic `stage next` command is workflow routing only. Formal reviewer dispatch was unavailable, so both plans and shipments are BLOCKED pending either successful formal multi-persona evidence or a new explicit, plan-scoped operator waiver. No waiver reservation or consumption record exists.

## Upstream Template Handoff

Stash `823BADF4` tracks the Principle IV-bounded external work for all generated sources:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`

The in-repo governance work is not operationally closed until those external templates land and regeneration verifies all three `.github` targets retain the behavior. The external work is not part of either in-repo shipment.

## Constitution Check

- **I — Safety-First Go:** no production code is changed during staging; implementation requires normal Go gates for contract tests.
- **II — Test-First:** contract tests fail before skill/agent instruction changes and pass after them.
- **III/IV — Isolation and containment:** all current writes stay in this repository; upstream work is stash-only.
- **V — Observability:** dispatch, waiver reservation, consumption, and verdict evidence are durable.
- **VI — Single Responsibility:** planning governance stays separate from docline regression work and external template execution.
- **VII — Destructive approval:** no destructive action is authorized.
- **VIII — Elevated risk:** governance changes require plan hardening before review.
- **IX — Git-friendly persistence:** evidence travels with the plan.
- **X — Context efficiency:** structured evidence replaces transcript claims.
- **XI — Merge history:** implementation remains merge-commit-only; Stage will not merge.

No constitutional violation or standing exception is requested. No current workflow waiver exists.

## Rejected Alternatives

Silent skip, inline self-certification, hosted-review substitution, generic-command-as-authorization, reusable textual waivers, and global/permanent waivers are rejected.

## Risks and Mitigations

- **Runtime tool names vary:** describe capability semantically and list known `agent`/`agent/runSubagent` names.
- **Fabricated attribution:** require actual returned outputs and identifiers; absence fails.
- **Waiver reuse:** reserve before harvest and persist consumed IDs afterward; reject existing reservation/consumption.
- **Bypass drift:** negatively test `skip_review` and `force_harvest_no_gates`.
- **Regeneration drift:** require upstream closure through stash `823BADF4`.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`.
