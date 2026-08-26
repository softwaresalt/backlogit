---
chunk_strategy: h2
description: "PR #377 cycle-33 remediation of four cycle-32 review findings: typed shipment-to-wave filtering, real-manifest and full red-contract verification with mutations, corrected descope citation, and canonical optional green-regression declarations; policy 1.21.0; gate remains FAIL pending independent review"
doc_type: memory
schema_version: "1.0"
source: cycle-33-review-remediation-session
title: "PR #377 Cycle 33 Review Remediation Memory"
---

# PR #377 Cycle 33 Review Remediation Memory

**Date**: 2026-08-26
**Agent**: Prompt Builder
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 33
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `c9fa17044cdd0585d0cfa7a2cca54f15adcb6f4d`
**Shipment**: `130-S` (queued — not claimed or shipped)
**Scope honoured**: planning, prompt, simulation, checkpoint, and memory artifacts only; no
subagents, push, merge, or Go source

## Baseline

Before edits:

```text
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue
WAVE_SIM_OK: 84/84 assertions PASS across 16 scenario(s)
```

The pass reproduced the review defect. `Get-QueueManifest` globbed `147.*-T.md` rather than reading
`130-S.md`; its count happened to be 42. It never proved that those tasks were shipment members,
never reported the explicit feature, and compared only the red flag inferred from raw text plus
`green_maker_tasks`. A selector, reason, close-wave, or shipment-membership mutation could pass.

## Findings and fixes

| ID | Sev | Finding | Fix |
|---|---|---|---|
| H1 | P1 | Normative `M` meant every shipment member, but execution used 42 tasks from a 43-member manifest | Define `S` as all explicit IDs and `M` as task-type IDs in `S`; report excluded IDs/types |
| H2 | P1 | Verification did not parse the shipment or all red-contract keys | Real manifest parse, exact filtered-ID comparison, all five keys, nine in-memory mutations, two scheduler negatives |
| H3 | P2 | Descope citation named the wrong abstraction | Cite `archivedFromDescopeEligibleStatus` |
| H4 | P2 | `green_regression_cmds` had no declaration grammar | Optional canonical JSON block; absent block is exactly `[]`; no current task needs one |

## H1 — typed shipment projection

The corrected contract has two explicit universes:

```text
S = every ID in shipment.custom_fields.items
M = { id in S : artifact_type(id) = task }
```

For `130-S`, `count(S)=43`, `count(M)=42`, and
`excluded_non_task_members=[147-F (feature)]`. Ship resolves every type from artifact metadata; it
does not infer type from an ID suffix. The covering-unit task enumeration is compared with filtered
`M`, not raw `S`. Wave budgets, status partitions, completion, and snapshot row counts all use
`count(M)`.

## H2 — manifest-backed verification and mutations

`scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` now:

1. reads `130-S.md` and its exact `custom_fields.items` list;
2. resolves each listed artifact from queue or archive and reads `artifact_type`;
3. compares exact shipment IDs, exact task-filtered `M`, and excluded `(id,type)` rows;
4. compares statuses, dependencies, exemption metadata, and the empty green-regression projection;
5. parses only the delimited canonical red block and compares all five keys:
   `red_deliverable`, `red_deliverable_reason`, `red_selector_command`,
   `green_maker_tasks`, and `green_maker_closes_wave`;
6. applies nine fixture-declared mutations to in-memory copies and requires each expected drift
   code.

The scheduler also computes a static wave map and fails
`WAVE_RED_MAPPING_UNRESOLVED` when a selector is empty or
`green_maker_closes_wave` differs from the actual latest green-maker wave.

## H3 — citation

The policy relies on archive provenance, not merely status eligibility. The correct helper is
`archivedFromDescopeEligibleStatus` in `internal/core/shipment_gate.go`. The cycle-32 memory,
checkpoint, plan, and live policy now use that citation.

## H4 — optional canonical green-regression contract

P-002.6 defines one declaration form: a
`<!-- BEGIN:green-regression-contract -->` / `<!-- END:green-regression-contract -->` block
containing a fenced JSON object with exactly one key, `green_regression_cmds`. A present block has
a non-empty unique string array and must satisfy scoped-command constraints. Absence means exactly
`[]`; an empty block is not authored. Ship freezes the array at Step 3 and `build-feature` runs it
unchanged. Malformed declarations halt with `WAVE_GREEN_REGRESSION_INVALID`.

No current `130-S` task needs a block, so all 42 defaults are empty.

## Validation

| Gate | Result |
|---|---|
| Scheduler + shipment drift + mutation checks | `WAVE_SIM_OK` 104/104 across 18 scenarios |
| Parsed census | S=43; M=42; excluded `147-F=feature` |
| Markdown P-008 | 0 issues |
| `backlogit docs lint` | `valid: true`, 0 violations |
| Integration | `go test ./tests/integration/ -count=1`, exit 0 |
| `backlogit sync` | 0 parse failures |
| Topology | 42 task members / 104 executable edges / 18 waves / acyclic |
| Go source modified | none |

## Gate state and next action

`cycle: 33`, `decision: FAIL`, `pending: independent-review-required`, `push_allowed: no`;
severity P0=0 / P1=2 (both remediated) / P2=2 (both remediated) / P3=0. Policy
`workflow-policies.md` is **1.20.0 → 1.21.0**.

The next action is an independent review of the cycle-33 diff. Push, PR-thread reconciliation,
shipment claim, and merge remain blocked. Operator merge approval has not been requested.
