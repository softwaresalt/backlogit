---
chunk_strategy: h1-h2-h3
description: "When prose governance documents have machine consumers that match literal field values, the producer contract must specify exact labeled-field formats — not free-form prose — or downstream validation fails silently or rejects valid records."
doc_type: learning
docline:
  date: 2026-07-23T00:00:00Z
  severity: high
  tags:
    - governance
    - harness
    - plan-review
    - stage
    - harvest
    - contract
    - ship
schema_version: "1.0"
source: docs/compound/2026-07-23-machine-readable-governance-field-contract.md
title: "Machine-readable governance fields require exact labeled-field format at the producer"
---

# Machine-Readable Governance Fields Require Exact Labeled-Field Format at the Producer

## Context

Surfaced during shipment 102-S (PR #287, merge commit `130a10a1`). The shipment
added a plan-review enforcement contract to Stage Step 4 and `harvest/SKILL.md`,
requiring both `dispatch_mode` and `decision` in the plan's `## Plan Review`
section. The downstream consumers (Stage and harvest) check for literal field
values (`decision: PASS`, `dispatch_mode: multi-agent-dispatch`, etc.).

The original plan-review/SKILL.md Step 5 only said "Gate decision and rationale"
— free-form prose. Copilot review (thread 6, `PRRT_kwDORzozKM6TKdWd`) flagged
this: the producer contract allows outputs like "Gate decision: PASS" or any prose
that mentions PASS without a literal `decision:` marker, which the downstream
absent/unrecognized checks would reject.

## Problem

A governance rule's **producer** (plan-review) and its **consumers** (Stage Step 4,
harvest Phase 1) were written independently with incompatible field-format
expectations:

- **Consumer assumption**: literal `decision: PASS` field in the `## Plan Review`
  section, matchable by field-name scan.
- **Producer output**: free-form "Gate decision and rationale" prose, where the
  value might appear as `**PASS**` (Markdown bold), `Gate decision: PASS` (prose
  label), or just mentioned in a sentence.

The mismatch means:
1. A PASS review from plan-review might be rejected by Stage/harvest because the
   field name is not an exact match.
2. The downstream check cannot distinguish a FAIL mentioned in a rationale
   sentence from `decision: FAIL` as an authoritative gate field.

## Solution

**Define the exact labeled-field format at the producer, not just at the consumers.**

In plan-review/SKILL.md Step 5 (the producer contract), specify:

```
decision: PASS         # or ADVISORY or FAIL — write as a labeled field, not prose
dispatch_mode: multi-agent-dispatch   # or single-agent-declared-degradation
operator_authorization: approved      # for ADVISORY+confirmed only
```

At the consumer side (Stage Step 4 skip_review gate, harvest Phase 1 step 3),
the check matches the literal `decision:` label, making the scan unambiguous.

**The ADVISORY authorization marker pattern applies the same principle**: "explicit
recorded authorization" written as prose cannot be distinguished from a finding that
merely mentions authorization. A concrete labeled field `operator_authorization: approved`
is unambiguous and scannable.

## Procedure for Future Governance Rules with Machine Consumers

When adding a governance gate that will be evaluated by an agent or script:

1. Identify every **producer** — the skill or step that writes the artifact field.
2. Identify every **consumer** — the skill or step that reads and validates the
   field.
3. Specify the **exact field name, allowed values, and format** in the producer's
   append or output spec (not just in the consumer's validation logic).
4. Write the consumer validation to match the producer's specified format exactly.
5. Review both producer and consumer in the same PR (a one-sided change creates
   the mismatch).

## Bypass-Gate Exception Documentation

A related pattern from the same shipment: when a new gate validates unconditionally,
check all documented bypass paths in the same agent/skill definition. Step 3.0 of
Stage documents a triple override (`skip_plan: true` + `skip_review: true` +
`force_harvest_no_gates: true`) that bypasses all planning and review gates. The
new skip_review gate contradicted that without an explicit bypass note.

Fix: add a bypass block at the top of the new gate specifying which override paths
are exempt, and verify it is consistent with the published event table and all other
references to the bypass in the same document. If the new gate is not supposed to
honor a bypass, remove or redefine the bypass everywhere before shipping.

## Evidence

- PR #287 Copilot thread 6 (`PRRT_kwDORzozKM6TKdWd`): "The downstream gates treat
  `decision` as a machine-readable field, but the producer contract still only asks
  for a free-form 'Gate decision and rationale.'"
- PR #287 Copilot thread 5 (`PRRT_kwDORzozKM6TKVlm`): "`plan-review` does not
  currently require an explicit authorization field or marker … prose scanning cannot
  reliably distinguish authorization from a finding that merely mentions it."
- PR #287 Copilot thread 4 (`PRRT_kwDORzozKM6TKVlF`): "This unconditional validation
  conflicts with the existing `force_harvest_no_gates` path … the agent receives
  contradictory instructions."
- Resolved in commits `d7e34dca`, `7ac40ae3` on PR #287.

## Applicability

Applies to any governance rule in a prompt-artifact (agent, skill, instruction) that:
- writes a field to a document that another agent or skill will later read
- distinguishes between outcome values (PASS/FAIL/ADVISORY) by field match
- uses an authorization or override pattern that must be machine-distinguishable

The principle is: **producers own the format; consumers own the validation. Define
the format at the producer and match it exactly at the consumer. Never leave the
format as "prose with the value somewhere."**
