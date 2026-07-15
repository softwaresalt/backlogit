---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Plan-review dispatch and fail-closed waiver governance'
source: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
doc_type: decision
description: 'Decision for exact-byte formal evidence, immutable private-token waiver lifecycle, lock-held governed mutation, contained paths, and fail-closed Stage/direct-harvest ownership.'
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

Stage must prefer real independent plan-review dispatch, prove the exact bytes reviewed, and prevent every Stage or direct-harvest mutation when evidence is missing or stale. A future waiver must be explicit, immutable, private-owner, single-use, and mode-scoped. The workflow also needs a final formal record, lock-held mutation, real-path containment, correct lifecycle ownership, and fail-closed ADVISORY behavior. Current plans have neither formal evidence nor an operator waiver.

## Evidence and Investigation

- Supported Copilot topologies can expose semantic subagent dispatch, but this invocation exposes no agent/task dispatch tool.
- Inline self-assessment and hosted review cannot impersonate required personas.
- Existing harvest copies and legacy Stage flags can otherwise bypass caller-only prose checks.
- `internal/atomicfile.WriteFileAtomic` is the shared repository writer. It intentionally provides atomic visibility without fsync because Git supplies document durability.
- Repository sync rehydrates stash promotion links from target artifact `source_stash_*` fields.
- `internal/docline.Scope()` is the production source of include/exclude truth for docline inventory.

## Options Considered

### A — Silent or reusable bypass

Rejected. It permits false attribution, stale PASS, waiver races, and unauditable mutation.

### B — Formal dispatch only, no contingency

Safe but unnecessarily blocks an explicitly authorized operator exception when runtime capability is absent.

### C — Exact formal evidence with a narrow immutable waiver contingency

Selected. Formal review is preferred. Unsupported dispatch blocks unless a new plan-scoped operator authorization is processed by the Stage/direct run that actually owns mutations.

## Decision

Use one Markdown-aware canonical terminal-record parser with two distinct final schemas:

1. `## Formal Plan Review Record` for attributed formal evidence and PASS/ADVISORY/FAIL.
2. `## Operator Waiver Ledger` for an operator-waived lifecycle.

Exactly one record may be the final H2 section. Each contains one fenced YAML mapping and EOF-only termination. Duplicate/non-final/malformed/unknown/trailing records and fenced-example confusion fail. Canonical UTF-8/LF SHA-256 removes only the uniquely parsed final record and hashes every other plan byte.

### Formal Evidence

The formal record contains `record_type`, schema version, unique review ID, exact workspace-relative plan path/digest, review time, required persona set, one attributed dispatch/result per persona, and verdict. Before Stage/direct harvest, recompute the digest and require exact match. Plan edits, appended content, stale digest, duplicate record, or missing persona/result block.

Formal ADVISORY is not direct-harvest evidence. Only Stage-managed flow may accept it, and only with a durable nested confirmation naming operator, authorization reference, confirmation time, exact plan path/digest, and `scope: stage_managed`. Direct harvest accepts formal PASS or valid direct waiver only.

### Waiver Authorization

The strict waiver schema fixes `verdict: WAIVED` and contains waiver ID, plan path/digest, authorizer/reference, missing capability, reason, issue/expiry, residual risk, `intended_disposition`, completion mode, `authorization_scope: exact_plan`, and authorized phases. All these non-transition fields are encoded deterministically and bound by `authorization_payload_sha256`. Unknown fields remain forbidden.

Reservation runs under a plan lock/CAS. It returns one cryptographically random raw token only to the winning long-lived in-process `GateSession` and persists only `reservation_token_sha256`; the token stays in process memory and never enters stdout/stderr, tool results, transcripts, persistence, or logs. Validation hashes the session-held token and compares with `crypto/subtle.ConstantTimeCompare`. Every immutable payload field is recomputed and compared. Only reserved/consumed transition fields may change. Tampering, loser or public-session-handle adoption, wrong owner/phase, expiry, or reuse fails closed.

`stage_managed` authorizes harvest plus shipment assembly and consumes afterward with exact IDs and required shipment ID. `direct_harvest` authorizes harvest only, consumes after its last mutation with exact IDs and no shipment ID, and grants no later shipment authority. `intended_disposition` must match the mode.

### Governed Mutation and Ownership

Plan-review owns dispatch and evidence production only. It never reserves, validates lifecycle credentials, mutates backlog state, or consumes.

Stage starts, owns, and drives one long-lived `GateSession` for `stage_managed`; direct harvest does so for `direct_harvest`. The raw token remains private to that process while callers use a non-secret handle. Harvest operating under Stage uses the Stage session but never consumes it.

No workflow may call validate and then a separate backlog command. One typed governed operation resolves/contains the plan, holds the cross-process lock, validates record/digest/expiry/owner/token/payload/phase, stages an allowlisted Markdown-first mutation, revalidates at the exact commit boundary, and publishes while locked. Drift or expiry aborts with zero durable mutation. Race tests pause between initial validation and commit to prove no plan/ledger change can slip through.

### Path and Write Policy

Every lifecycle path is workspace-relative under `docs/exec-plans/`. Reject absolute, UNC/volume, traversal, missing/non-regular, and symlink/junction/reparse escapes before read, lock, or write. Resolve real target and require containment inside the configured workspace.

Reuse `internal/atomicfile.WriteFileAtomic`; do not duplicate temp/rename code. Fsync is not required for these Git-backed records because the correctness requirement is atomic visibility, not power-loss persistence.

### Bypass Reconciliation

`skip_review` and `force_harvest_no_gates` are never bypasses. If retained, they can only request the same evidence/lifecycle route. Both harvest copies independently accept exact formal PASS or a mode-valid reservation; direct ADVISORY, inline/hosted review, caller assertion, and prose cleared/ready text produce zero mutation.

## Provenance Repair

The bounded pass repaired canonical promotion provenance:

- `8CD8F46A -> 105-F`
- `CA877CD1 -> 105.004-T`
- `A4BE2FAD -> 106-F`

Target artifacts now carry source stash ID/kind/priority/path/text and applicable deliberation. Archived stash records carry harvested reason and artifact ID. Two repository-native syncs rebuilt all three harvested links exactly; repair stash `3E12DC97` was retired only afterward.

## Current Gate State

This operator direction authorizes one bounded refinement pass, not a review waiver. Formal reviewer dispatch remains NOT RUN and waiver authorization remains NONE. Shipment `094-S`, feature `105-F`, tasks `105.001-T` through `105.013-T`, shipment `095-S`, feature `106-F`, and tasks `106.001-T` through `106.007-T` remain BLOCKED.

Shipment requeue remains separately tracked by `DB1F9026`. Generic `blocked -> active` is forbidden. Stash `D7B1B33D` remains active and independently isolated in PR #240.

## Upstream Template Handoff

Stash `823BADF4` tracks external updates to:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`
- `templates/skills/harvest/SKILL.md.tmpl`

Closure requires regeneration parity for final formal/waiver schemas, exact digest checks, ADVISORY behavior, lifecycle ownership, governed mutation, legacy bypass rejection, and Constitution Check output. External work is not part of either shipment.

## Constitution Check

- **I:** implementation is split into small leaf Go/schema/lease/path units, a core broker, focused tests, and thin CLI.
- **II (NON-NEGOTIABLE):** formal/waiver/race/containment contracts begin RED and end GREEN.
- **III/IV (NON-NEGOTIABLE):** real-path containment precedes file access; external templates remain stash-only.
- **V:** persona evidence, exact digest, payload hash, token fingerprint, owner, and transitions are durable and queryable.
- **VI:** each task has one concern, at most two files, and fewer than five functions.
- **VII (NON-NEGOTIABLE):** no deletion or scratch mutation is authorized.
- **VIII:** governance/concurrency/security risk requires hardening and adversarial tests.
- **IX:** canonical evidence remains human-readable and Git-backed.
- **X:** compact records avoid transcript reliance.
- **XI:** implementation remains merge-commit-only; Stage does not merge.

No constitutional exception or current waiver exists.

## Rejected Alternatives

Silent skip, self-certification, hosted-review substitution, stale formal PASS, check-then-mutate, raw persisted bearer token, mutable authorization payload, duplicate atomic writer, unconstrained caller paths, direct ADVISORY, and reusable/global waivers are rejected.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`.