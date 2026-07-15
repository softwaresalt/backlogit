---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'PASS-only formal planning governance'
source: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
doc_type: decision
description: 'Decision to install exact-byte formal PASS-only Stage and harvest governance with one stateless governed mutation per CLI process and no machine waiver lifecycle.'
docline:
    date: 2026-07-14T18:30:00Z
    decision_status: decided
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
        - 062A67C0
    linked_deliberation: 052-DL
---

# PASS-only Formal Planning Governance

## Problem Frame

Stage and direct harvest must prove that independent personas reviewed the exact plan bytes before any backlog mutation. The previous design expanded into an unauthenticated waiver state machine and a process-private token session that the repository's one-command Cobra model cannot support. The smaller v1 needs a trustworthy admission rule using infrastructure the repository can implement now.

## Evidence and Constraints

- This Stage invocation exposes no semantic reviewer-subagent dispatch tool, so formal review is currently NOT RUN.
- Inline self-assessment and hosted Copilot PR review cannot impersonate required reviewer personas.
- The repository has no trusted local authority that can authenticate a claimed operator waiver.
- `backlogit` executes one Cobra command and exits; a private token cannot survive safely across later commands without new transport infrastructure.
- Stage and harvest currently have direct command and prose-readiness paths that must fail closed.
- Root commands are explicitly registered in `internal/cli/root.go`; generated reference docs and CLI-only parity must be updated with any new command.
- Workspace paths must satisfy lexical and real-path containment before evidence reads or lock derivation.

## Options Considered

### A — PASS-only formal evidence with stateless mutation enforcement

Selected. It has one auditable admission result and fits the existing process model.

### B — Machine-managed operator waiver lifecycle

Deferred as YAGNI. A safe future design first needs a trusted approval source and coherent lifecycle ownership.

### C — Caller prose, hosted review, or legacy flags

Rejected. These are not attributed exact-plan evidence and cannot authorize mutation.

## Decision

Install formal PASS-only governance for v1.

### Final Formal Record

A Markdown-aware parser permits exactly one `## Formal Plan Review Record` as the final H2 section outside fences. The section contains exactly one strict fenced YAML mapping and ends at EOF. Duplicate, non-final, malformed, unknown-field, or trailing content fails closed.

Canonicalization validates UTF-8 without BOM, normalizes CRLF to LF, rejects bare CR, removes only the uniquely parsed terminal formal-record block and its separator, and hashes every other byte with lowercase SHA-256. Every plan or review-content edit changes the digest; content appended after the formal record is rejected.

The record names a unique review, workspace-relative plan path, exact digest, review time, required persona set, one attributed dispatch result per persona, and final verdict. Represented failed/FAIL/ADVISORY results remain auditable, but only complete all-PASS evidence admits. Incomplete or unattributed results, stale digest, missing record, and malformed record also block.

### Stateless Governed Mutation

Expose one leaf command: `backlogit plan-apply --plan <workspace-relative-plan>`. One invocation accepts exactly one strict typed mutation request, resolves and contains the plan path, acquires a cooperative plan lease, re-reads and validates exact formal PASS, calls one allowlisted existing core mutation, then releases the lease after core returns.

Initial operations are create item, add dependency, create shipment, and add item to shipment.

There are no reserve, validate, consume, token, session, handle, waiver, confirmation, or plan-record-writer APIs. Dependency direction is `internal/cli -> internal/governed -> {internal/plangate, internal/core}`. `internal/plangate` imports no core; core imports no CLI.

### Workflow Admission

Both Stage and harvest copies require exact formal PASS and invoke `plan-apply` once per mutation. `skip_review`, `force_harvest_no_gates`, caller readiness prose, hosted review, inline assessment, direct legacy mutation, FAIL, ADVISORY, missing, stale, and malformed evidence yield zero mutation.

Plan-review produces evidence only. Impl-plan emits an explicit `## Constitution Check`; a dedicated contract test guards that output separately from gate evidence and runtime tests.

## Deferred Authenticated Waivers

Machine-managed waiver support is not part of shipment `094-S`. Stash `062A67C0` records the future possibility. Any future design must authenticate a live non-dismissed GitHub approval by a maintainer with repository permission and bind it to repository, plan path/digest, scope, intended disposition, and expiry. Prefer one process owning the full authorized-to-reserved-to-consumed manifest. No such authority, lifecycle, or implementation is planned now.

## Provenance and Intake Boundaries

Canonical harvested provenance remains:

- `8CD8F46A -> 105-F`
- `CA877CD1 -> 105.004-T`
- `A4BE2FAD -> 106-F`

Stash `823BADF4` carries the external PASS-only template handoff. Stash `D7B1B33D` remains active in PR #239; PR #240 was closed unmerged by operator correction. Neither deferred stash is harvested into the current shipments.

## Current Gate State

This simplification direction is not formal review evidence and is not a waiver. Shipment `094-S`, feature `105-F`, all sixteen member tasks, shipment `095-S`, feature `106-F`, and all nine member tasks remain BLOCKED.

The PASS-only design cannot authorize its own installation. Later unblocking requires either successful formal multi-persona evidence for the exact final plan, or a separate explicit operator bootstrap approval limited to installing PASS-only governance and recorded durably, such as an operator-authored GitHub PR comment. No bootstrap approval is inferred or currently present.

## Upstream Template Handoff

Active stash `823BADF4` covers:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`
- `templates/skills/harvest/SKILL.md.tmpl`

Closure requires regeneration parity for exact formal PASS, stateless plan-apply routing, non-PASS zero-mutation behavior, containment, bypass rejection, and Constitution Check output. External writes remain outside this workspace.

## Constitution Check

- **I:** Go production work is isolated into leaf parser, path, lease, broker, and CLI units.
- **II (NON-NEGOTIABLE):** bounded evidence, runtime, control-plane, and Constitution contracts start RED and end GREEN.
- **III/IV (NON-NEGOTIABLE):** lexical and real-path containment precedes every plan read and lease; external templates remain stash-only.
- **V:** exact digest, review ID, persona attribution, verdict, operation, and failure are observable.
- **VI:** every task has one concern, fewer than three files, fewer than five functions, and fewer than four scenario groups.
- **VII (NON-NEGOTIABLE):** no scratch-file mutation or unapproved destructive action is included.
- **VIII:** governance and concurrency risks receive focused negative and race tests without speculative infrastructure.
- **IX:** formal evidence remains human-readable, Git-backed Markdown/YAML.
- **X:** one stateless command avoids hidden session context.
- **XI:** Stage does not merge or perform Ship work.

No constitutional exception or current waiver exists.

## Rejected Alternatives

Rejected for v1: local claimed-authorizer hashes, waiver YAML states, reservation or consume lifecycles, bearer tokens or fingerprints, long-lived sessions, ADVISORY confirmation, gate-owned plan writers, check-then-command validation, and reusable bypasses.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-planning-governance-gates-plan.md`.
