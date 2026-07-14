---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Plan-review dispatch and fail-closed waiver governance'
source: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
doc_type: decision
description: 'Decision to use real reviewer subagent dispatch when available and permit only an explicit, auditable bootstrap waiver when it is not.'
docline:
    date: 2026-07-14T18:30:00Z
    decision_status: decided
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
    linked_deliberation: 052-DL
---

# Plan-review Dispatch and Fail-closed Waiver Governance

## Problem Frame

Stage must run `plan-review` before harvest, but recent API-launched Stage sessions had no agent or task dispatch tool. Inline single-agent assessment was honestly reported, yet it cannot satisfy a skill that requires independent persona outputs. The same planning surface also omits the constitution's required labeled `## Constitution Check`.

Success means formal review is executable in supported environments, its provenance is inspectable, missing capability blocks harvest by default, and any exception is operator-authorized and bounded. Hosted Copilot review may supplement the gate but cannot impersonate a local reviewer persona.

## Research Findings

- `.github/skills/plan-review/SKILL.md` requires all always-on personas plus triggered personas and appends their findings to the plan.
- `.github/agents/.stage.agent.md` declares the abstract `agent` tool, `max_subagent_tier: 3`, and `subagent_depth: 2`.
- `plugin/agents/stage.agent.md` declares the concrete `agent/runSubagent` tool and `subagent_depth: 2`.
- `.github/agents/_orchestrator.agent.md` permits depth 3: Orchestrator → Stage → skill → reviewer persona.
- Reviewer definitions are read-only leaf executors with `subagent_depth: 0`; the depth and role policies therefore permit dispatch.
- This API invocation exposes PowerShell, file reads, and backlogit tools, but no agent/task dispatch tool. The defect is an invocation-tool-surface degradation or omission, not a structurally impossible repository design.
- The impl-plan copies describe a standards check but do not require the constitution's exact labeled section.

## Options Evaluated

### Option A: Dispatch only, no contingency

Use `agent/runSubagent` where present and halt everywhere else. This is strongest but makes the staging pipeline unusable in otherwise supported non-dispatch invocations, including the bootstrap needed to install the guard itself.

### Option B: Treat inline or hosted review as equivalent

Allow one caller model or GitHub Copilot review to stand in for personas. Rejected because provenance would be false and independent focus would be lost.

### Option C: Dispatch first with a narrow waiver contingency

Run formal personas whenever the dispatch tool is exposed. When it is absent, fail closed unless the operator supplies a plan-scoped, auditable, single-use waiver. Record waiver mode distinctly from PASS. This preserves the real gate while allowing an explicit bootstrap path.

## Decision

Choose Option C. Supported Copilot environments already have a viable dispatch topology; implementation must make that path explicit and observable rather than silently falling back.

A formal gate record must include:

1. `review_mode: formal`;
2. every required persona name and agent definition path;
3. dispatch/return evidence or invocation identifier;
4. model/provider when the runtime exposes it, otherwise `unknown`;
5. each persona's structured findings and disposition;
6. merged severities and final PASS, ADVISORY, or FAIL.

A missing, failed, or unattributed required persona makes the gate FAIL. Inline self-assessment is useful preparation only and must be labeled `informal_single_agent`; it never counts as formal evidence. Hosted review remains supplemental.

A waiver is valid only when the appended plan gate record includes all of: `review_mode: operator_waiver`, exact plan path, operator authorization reference and authorizer, missing capability, reason, UTC timestamp, single-use/expiry boundary, acknowledged residual risk, and harvest disposition. Missing fields, scope mismatch, expiry, or reuse blocks harvest. A waiver verdict is `WAIVED`, never `PASS`.

## Bootstrap Authorization

The operator's 2026-07-14 `stage next` instruction explicitly requests planning, review, harvest, and queued shipments while identifying this invocation's inability to dispatch. That instruction authorizes one bootstrap waiver for the two plans created in this staging session only. The waiver expires after those plans are harvested and cannot be reused by future Stage sessions. This is not evidence that formal personas ran.

## Constitution Check

- **I — Safety-First Go:** no production code is changed during staging; the implementation plan requires normal Go gates for its contract tests.
- **II — Test-First:** contract tests must fail before skill/agent instruction changes and pass after them.
- **III/IV — Isolation and containment:** all planned writes remain inside this repository; upstream autoharness work is excluded.
- **V — Observability:** formal dispatch and waivers require attributed, appended evidence.
- **VI — Single Responsibility:** review governance and impl-plan constitution output share the planning-gate concern; docline regression work remains separate.
- **VII — Destructive approval:** no destructive action is authorized.
- **VIII — Elevated risk:** governance changes require plan hardening before review.
- **IX — Git-friendly persistence:** evidence is durable Markdown traveling with the plan.
- **X — Context efficiency:** compact structured evidence replaces transcript claims.
- **XI — Merge history:** implementation remains subject to merge-commit-only delivery; Stage will not merge.

No constitutional violation or standing exception is requested. The single-use review waiver is a workflow bootstrap exception, not a waiver of constitutional safety principles.

## Rejected Alternatives

Silent skip, inline self-certification, hosted-review substitution, and a global/permanent waiver are rejected.

## Risks and Mitigations

- **Runtime tool names vary:** describe capability semantically and list known `agent`/`agent/runSubagent` names.
- **Fabricated attribution:** require actual returned outputs and identifiers; absence fails.
- **Waiver normalization:** label WAIVED, scope it to one plan, expire it, and require operator authorization each time.
- **Mirror drift:** add regression checks across `.github` and plugin copies.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`.
