---
chunk_strategy: h1-h2-h3
description: "Execution plan for S3: CLI/MCP structured-error parity, docs classify tool, and create_checkpoint governance"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s3-cli-mcp-parity-and-governance-plan.md
title: "S3 Execution Plan — CLI/MCP Parity & Governance"
---

# S3 Execution Plan — CLI/MCP Parity & Governance

**Covering feature**: CLI/MCP surface parity — structured errors, classify tool, and create_checkpoint governance
**Deliberation**: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md (5F4E0FC3 decision)
**Stash members**: 63E810D9, EB93E236, 5672D73E, 5F4E0FC3
**Tier**: composability/interoperability (shipment sequence S3)

## Problem Frame

The CLI and MCP surfaces diverge: unknown-key checkpoint rejections expose a
structured `unknown_fields` array on MCP but only a wrapped string on CLI; there
is no MCP equivalent of `backlogit docs classify`; and create_checkpoint is
ungoverned so registry-drift enforcement does not cover it. These are
interoperability/parity gaps staged ahead of feature work.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; errors wrapped |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | n/a |
| IV. CLI Containment | n/a |
| V. Observability | Structured error envelope improves machine legibility |
| VI. Single Responsibility | Shared response builder factored once |
| VII. Destructive Approval | none |
| VIII. Safety Modes | additive |
| IX. Git-Friendly | registry YAML + fixture |
| X. Context Efficiency | parseable errors reduce agent guesswork |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Shared transport-agnostic structured error builder (EB93E236)
* Scope: factor a shared response builder from *CheckpointUnknownFieldError so both surfaces can expose unknown_fields structurally; add an optional `data` field to internal/cli/format.JSONRPCError per JSON-RPC 2.0 so structured domain errors do not degrade to a string on the CLI JSON-RPC surface.
* Acceptance: builder yields identical {error, message, unknown_fields} payload for CLI and MCP; JSONRPCError carries optional `data`; unit tests cover both surfaces.

### U2 — CLI structured JSON error envelope (63E810D9)
* Scope: route CLI validation failures through the U1 builder so an unknown-key checkpoint rejection prints a parseable {error, message, unknown_fields} envelope mirroring the MCP shape.
* Acceptance: CLI `checkpoint create` unknown-key rejection emits the structured envelope; agent can read unknown_fields without parsing message text. Depends on U1.

### U3 — Add backlogit_docs_classify MCP tool (5672D73E)
* Scope: add a `backlogit_docs_classify` MCP tool wrapping docline.Classify for CLI/MCP parity with the existing docs lint/migrate/scope tools.
* Acceptance: MCP `backlogit_docs_classify` returns the same classification as CLI `backlogit docs classify <path>`; cross-surface parity test passes.

### U4 — Govern create_checkpoint (5F4E0FC3 decision)
* Scope: add `governed: true` + `governed_name: checkpoint_create` to create_checkpoint in .autoharness/backlog-registry.yaml and author the named behavioral fixture that dispatches the authoritative registry (compound: 2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md). No new result-shape/flag semantics.
* Acceptance: registry drift test now covers create_checkpoint; fixture dispatches the authoritative registry and passes on both surfaces.

## Dependency Graph

U2 depends on U1 (shared builder). U3 and U4 independent. Order: U1, U2, U3, U4.

## Runtime Verification and Closure

U1-U3 change CLI/MCP runtime surfaces (error envelopes, new tool); U4 changes
governance metadata. Verification: cross-surface parity tests + registry drift
test. Closure: parity fixtures are the durable closure artifact.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Add a new `backlogit_docs_classify` MCP tool | Medium — new MCP surface changes the public tool catalog | Keep the tool additive, mirror CLI classification semantics, and add cross-surface parity tests |
| Add structured CLI JSON-RPC error `data` for checkpoint unknown fields | Medium — agent-facing error envelopes become more structured | Preserve the existing `WrapError` API; ensure legacy errors omit `data` instead of emitting `null`; add byte-level JSON tests for both legacy and structured paths |
| Reflect caller-supplied `unknown_fields` values | Medium — unbounded key echo can amplify output or expose sensitive key names | Bound count and size, JSON-escape/control-character-safe output, and avoid logging raw secret-like values beyond field names |

Rollback: disable the new MCP registration and keep the structured `data` field
behind the shared builder call path if compatibility issues appear. Compatibility:
all surfaces remain additive and backward-compatible when `data` is omitted for
legacy errors. Ownership: transport-neutral DTOs belong in a leaf package that can
consume `internal/errors`, not in an MCP- or CLI-owned presentation package.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — new MCP tool + CLI error envelope + JSONRPCError data field (all additive/backward-compatible).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent — additive parity work.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Security Reviewer (`gpt-5.6-terra`) when risk-triggered for the plan
* Learnings Researcher over `docs/compound/`

Controlling P1 findings:
* P-006 hardening was missing despite a new MCP tool, CLI error envelope, and `JSONRPCError.data` public contract signal.
* The transport-neutral builder package owner was unspecified and must not couple MCP to CLI presentation internals.
* `unknown_fields` reflects attacker-controlled key names and needs count, size, and control-character bounds.
