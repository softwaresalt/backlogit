---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Stage memory: blocked planning governance and docline shipments'
source: docs/memory/2026-07-14/stage-governance-docline-memory.md
doc_type: memory
description: 'Corrected session continuity for blocked shipments 094-S and 095-S after authorization and Copilot review findings.'
docline:
    ms.date: 2026-07-14T22:13:07Z
    ms.topic: memory
---

# Stage Memory: Blocked Planning Governance and Docline Shipments

## Current State

- Shipment `094-S`, feature `105-F`, and tasks `105.001-T` through `105.008-T` are blocked.
- Shipment `095-S`, feature `106-F`, and tasks `106.001-T` through `106.007-T` are blocked.
- No shipment, feature, or task was unblocked, claimed, implemented, removed, or archived.
- Both plans remain `review_state: blocked`; formal plan-review provenance is NOT RUN and waiver authorization is NONE.
- The operator authorized one additional refinement cycle only. That authorization is not formal evidence and is not a plan-review waiver.
- Shipment `blocked` remains non-resumable through current `ClaimShipment`; stash `DB1F9026` tracks atomic hold/requeue support and generic `blocked -> active` is forbidden.

## Refinement Decisions

- Task `105.006-T` isolates the canonical final-ledger parser and digest; `105.007-T` isolates lock/CAS reservation ownership; `105.008-T` exposes the repository-native reserve/validate/consume CLI.
- Reservation is atomic under a plan-scoped cross-process lock. Concurrent Stage/direct callers yield exactly one immutable owner plus opaque token; every loser blocks before mutation.
- Owner, token, exact digest, state, expiry, mode, and phase are validated immediately before every mutation. Wrong or stale state fails closed.
- `stage_managed` mode retains ownership through shipment assembly and consumes with exact harvested IDs plus required shipment ID.
- `direct_harvest` mode consumes after its last harvest mutation with exact IDs and no shipment ID; it grants no later shipment authority.
- Both harvest copies independently validate provenance. Missing evidence, prose readiness, conflict, wrong owner/token/phase, malformed ledger, or consumed state produces zero mutations.
- Canonical lowercase SHA-256 normalizes valid UTF-8 CRLF to LF, removes only the uniquely parsed final ledger block, and hashes every other byte. Missing, duplicate, malformed, unknown-field, non-final, or trailing ledgers fail closed.
- Docline task `106.001-T` retains the live-corpus guard and adds hermetic temporary-Git cases for tracked missing keys and untracked exclusion plus positive/edge cases.

## Upstream Handoff

Active high-priority stash `823BADF4` names all four external Principle IV-bounded templates:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`
- `templates/skills/harvest/SKILL.md.tmpl`

Closure requires all four external changes to land, regeneration of all four `.github` targets, and parity verification. The stash remains outside in-repo shipments.

## Fresh Review Follow-up

- Fixed the literal newline escape in this memory.
- Corrected `105.004-T` to G1 scenario 4.
- Added missing-ledger, unknown-field, concurrency, owner/token/phase, mode, and bypass negatives to `105.001-T`.
- Split atomic lifecycle implementation into `105.006-T` through `105.008-T`; `105.002-T` now depends on the CLI task.
- Reconciled Stage-managed and direct-harvest shipment timing in the plan, decision, and tasks.
- Harvested provenance remains a supported-tooling blocker: current CLI cannot stamp `custom_fields.source_stash_id` or amend archived stash records. Stash `3E12DC97` tracks an atomic repair command for `8CD8F46A -> 105-F`, `CA877CD1 -> 105.004-T`, and `A4BE2FAD -> 106-F`; raw backlog edits are prohibited.

## Stash and Integrity

- Consumed source entries `8CD8F46A`, `CA877CD1`, and `A4BE2FAD` remain archived but lack rehydratable harvested provenance.
- `7F0A6E89`, `823BADF4`, `DB1F9026`, and `3E12DC97` remain active/deferred; none is in an in-repo shipment.
- Use `backlogit.exe --cwd .` exclusively; MCP remains bound to the installed-plugin workspace.
- Repository-native sync indexed 854 artifacts; docline lint reports zero violations.
- Doctor reports only pre-existing orphan `016.001-R`; zero fixes were applied.
- `docs/decisions/2026-07-13-scratch-spike.md` remains untracked and must not be edited, deleted, or committed.

## Next Steps

1. Keep every plan, shipment, feature, and task blocked.
2. Commit/push this bounded follow-up and resolve only review threads actually fixed.
3. Leave harvested-provenance review open unless a supported repair becomes available.
4. Require successful formal multi-persona evidence for each exact final plan or a separate explicit waiver naming its path and digest.
5. Require supported requeue from `DB1F9026`, or separately authorized artifact-preserving replacement shipment assembly, before Ship intake.
6. Do not merge the staging PR.