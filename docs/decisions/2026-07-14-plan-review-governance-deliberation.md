---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Plan-review dispatch and fail-closed waiver governance'
source: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
doc_type: decision
description: 'Decision for real reviewer dispatch, uniquely parsed final-ledger waivers, and independent fail-closed enforcement at Stage and direct-harvest boundaries.'
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

Success means formal review is executable in supported environments, its provenance is inspectable, missing capability blocks every Stage-mediated or direct-harvest path by default, and any exception is operator-authorized, digest-bound, bounded, and durably consumed. Hosted Copilot review may supplement the gate but cannot impersonate a local reviewer persona.

## Research Findings

- `.github/skills/plan-review/SKILL.md` requires all always-on personas plus triggered personas and appends their findings to the plan.
- `.github/agents/.stage.agent.md` declares the abstract `agent` tool, `max_subagent_tier: 3`, and `subagent_depth: 2`.
- `plugin/agents/stage.agent.md` declares the concrete `agent/runSubagent` tool and `subagent_depth: 2`.
- `.github/agents/_orchestrator.agent.md` permits depth 3: Orchestrator → Stage → skill → reviewer persona.
- Reviewer definitions are read-only leaf executors with `subagent_depth: 0`; the depth and role policies therefore permit dispatch.
- This API invocation exposes no agent/task dispatch tool. The defect is invocation-tool-surface degradation, not a structurally impossible repository design.
- Existing Stage inputs `skip_review` and `force_harvest_no_gates` can bypass provenance unless removed or routed through the same fresh-waiver validation.
- Both harvest skill copies currently accept prose cleared/ready assertions and can mutate backlog state when invoked directly; Stage-only enforcement is insufficient.
- The Stage, plan-review, impl-plan, and harvest `.github` targets are generated from four external autoharness templates and require a tracked upstream handoff.

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

A future waiver is valid only when the plan gate record includes `review_mode: operator_waiver`, a unique `waiver_id`, exact plan path and digest, operator authorization reference and authorizer, missing capability, reason, UTC issue/expiry, acknowledged residual risk, completion mode, authorized phases, and intended disposition. Generic workflow commands and this refinement authorization are never waiver signals.

Waiver mode uses exactly one uniquely parsed `## Operator Waiver Ledger` as the final H2 section. The Markdown-aware parser ignores heading-like text inside fenced code, so examples do not count. The section contains exactly one fenced YAML mapping, only known unique keys, state-compatible required fields, a closing fence, one terminal line ending, and EOF. Duplicate headings or keys, a missing ledger in waiver mode, malformed YAML, unknown fields, another heading, prose, whitespace, or bytes after the allowed terminal ending all fail closed.

Canonical digest decodes valid UTF-8 without BOM, rejects invalid UTF-8 or bare CR, normalizes CRLF to LF, and requires exactly one terminal LF. Lowercase SHA-256 covers every canonical byte except the validated final ledger block and its one separator LF. Before reservation, hash the whole ledger-free canonical plan. After reservation, remove only the uniquely parsed final block and hash every remaining canonical byte. Windows and Unix checkouts therefore agree, while plan/review content inserted before the ledger changes the digest and requires a new explicit waiver; content appended after the ledger is rejected. Formal mode may have no ledger.

Atomic waiver lifecycle is repository-native, never instruction-level check-then-write. A plan-scoped cross-process lock plus compare-and-swap re-reads and validates the canonical plan under lock, atomically writes the final ledger, and returns one immutable reservation owner/run ID plus a cryptographically random opaque token. Concurrent Stage/direct callers for the same waiver yield exactly one winner; every loser conflicts before mutation. Validate checks waiver ID, path/digest, state, owner, token, expiry, completion mode, and authorized phase immediately before every backlog mutation. Consume is a locked CAS allowed only to the same owner/token. Any mismatch, concurrent conflict, stale/partial state, or crash fails closed.

The ledger has two explicit completion modes. `stage_managed` authorizes `[harvest, shipment_assembly]`: harvest returns exact IDs without consuming; Stage retains owner/token through shipment creation and consumes afterward with `consumed_at`, exact IDs, and required `shipment_id`. `direct_harvest` authorizes `[harvest]`: harvest consumes immediately after its last mutation with exact IDs and no `shipment_id`; that consumed waiver cannot later authorize shipment assembly. Reserved records forbid consumed fields. Consumed records require time and non-empty exact IDs. Invalid mode/phase/field combinations fail closed.

Both harvest copies independently re-read and validate durable formal evidence or the owner/token reservation immediately before every create, dependency, link, adoption, or shipment-membership mutation. Caller/Stage assertions and prose cleared/ready text are not proof. Direct invocation with missing/malformed evidence, wrong owner/token/phase, conflict, or consumed waiver must produce zero backlog mutations.

The legacy `skip_review: true` and `force_harvest_no_gates: true` values are not bypasses. If retained, they only request the same atomic operator-waiver validation and cannot reach Stage or direct harvest without valid durable provenance.

## Current Gate State

The operator authorized one additional plan/backlog refinement cycle only; that authorization is not formal review evidence and is not a waiver. No waiver was authorized for either current plan. The generic `stage next` command is workflow routing only. Formal reviewer dispatch was unavailable, so both plans and shipments are BLOCKED pending either successful formal multi-persona evidence or a new explicit, plan-scoped operator waiver. No waiver reservation or consumption record exists. Shipment `094-S`, feature `105-F`, and tasks `105.001-T` through `105.008-T` remain blocked; shipment `095-S`, feature `106-F`, and tasks `106.001-T` through `106.007-T` remain blocked.

The repository's generic artifact transition accepted shipment `blocked`, but shipment lifecycle code has no `blocked -> queued` transition and `ClaimShipment` accepts only queued manifests. Stash `DB1F9026` tracks atomic hold/requeue support. Until it lands, approval alone does not make `094-S` or `095-S` claimable; generic `blocked -> active` is forbidden because it bypasses atomic member activation.

Archived source stash links for `8CD8F46A -> 105-F`, `CA877CD1 -> 105.004-T`, and `A4BE2FAD -> 106-F` are not rehydratable because current artifacts lack `custom_fields.source_stash_id` and the CLI has no supported repair command. Stash `3E12DC97` tracks that atomic repair path. Raw backlog files must not be edited, so this remains an explicit traceability blocker.

## Upstream Template Handoff

Stash `823BADF4` tracks the Principle IV-bounded external work for all generated sources:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`
- `templates/skills/harvest/SKILL.md.tmpl`

The in-repo governance work is not operationally closed until all four external templates land and regeneration/parity verification proves all four `.github` targets retain formal dispatch, canonical final-ledger digest, atomic owner/token/phase validation, both completion modes, legacy bypass, direct-harvest pre-mutation, and Constitution Check behavior. External work is not part of either in-repo shipment.

## Constitution Check

- **I — Safety-First Go:** no production code is changed during staging; implementation isolates the leaf parser/reservation and thin CLI adapter and requires normal Go gates.
- **II — Test-First:** integration contracts fail first; parser, atomic reservation, and CLI units are test-first; all gates pass before instruction work completes.
- **III/IV — Isolation and containment:** all current writes stay in this repository; upstream work is stash-only.
- **V — Observability:** dispatch, atomic owner/token reservation, phase validation, mode-specific consumption, and verdict evidence are durable.
- **VI — Single Responsibility:** parser, reservation, CLI, Stage, harvest, contract-test, docline, and external-template concerns remain width-isolated.
- **VII — Destructive approval:** no destructive action is authorized.
- **VIII — Elevated risk:** governance changes require plan hardening before review.
- **IX — Git-friendly persistence:** evidence travels with the plan.
- **X — Context efficiency:** structured evidence replaces transcript claims.
- **XI — Merge history:** implementation remains merge-commit-only; Stage will not merge.

No constitutional violation or standing exception is requested. No current workflow waiver exists.

## Rejected Alternatives

Silent skip, inline self-certification, hosted-review substitution, prose cleared/ready harvest, caller-trusted direct harvest, generic-command/refinement-as-authorization, suffix-excluding digests, check-then-write reservations, shipment-before-ID consumption, reusable textual waivers, and global/permanent waivers are rejected.

## Risks and Mitigations

- **Runtime tool names vary:** describe capability semantically and list known `agent`/`agent/runSubagent` names.
- **Fabricated attribution:** require actual returned outputs and identifiers; absence fails.
- **Reservation race/reuse:** plan lock plus CAS yields one owner/token; validate ownership/phase before each mutation and reject every conflicting or consumed record.
- **Shipment timing:** distinct completion modes keep Stage ownership through shipment assembly while direct harvest consumes without granting shipment authority.
- **Ledger suffix attack:** parse exactly one final fenced-YAML ledger, hash every other byte, and reject any trailing content.
- **Direct harvest bypass:** both harvest copies independently validate before every mutation and prove zero-mutation negatives.
- **Bypass drift:** negatively test `skip_review` and `force_harvest_no_gates`.
- **Regeneration drift:** require four-template upstream closure through stash `823BADF4`.
- **Missing stash provenance:** use only the future supported repair from stash `3E12DC97`; do not raw-edit archived records.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`.
