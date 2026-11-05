---
chunk_strategy: h1-h2-h3
description: 'Bounded plan for canonical plan-path classification and directly coupled authoring references.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-04-docs-plan-namespace-compatibility-plan.md
title: 'Plan Documentation Namespace Compatibility Plan'
docline:
  source_decision: docs/decisions/2026-11-04-docs-plan-namespace-compatibility-deliberation.md
  status: draft
---

## Objective

Complete the user-approved plan namespace relocation by recognizing
`docs/plans/` as a plan namespace while preserving legacy recognition and
correcting directly coupled authoring pointers. This maintenance plan does not
replan or reharvest the multi-session MCP work.

## Requirements Trace

* Canonical classification and legacy compatibility: U1
* Canonical workspace setting: U2
* Current operator and harness entry points: U3 through U10
* Historical evidence, event logs, checkpoint records, and original captured
  stash text remain unchanged

## Implementation Units

All units require an existing covering feature, at most two implementation
files per task, a verifiable acceptance criterion, and Ship-owned harness or
document-only exemption evidence before execution. The proposed selector names
below are not claims that a harness already exists.

### U1: Classify canonical and legacy plan paths

* Domain: code; production file: `internal/docline/policy.go`
* Posture: test-first; Ship creates a compiling assertion harness before the
  production edit, separately from this plan
* Acceptance: canonical `docs/plans/x.md` and legacy
  `docs/exec-plans/x.md` classify as `plan`; a nonmatching
  `docs/plans-extra/x.md` remains `guide`
* Proposed red and green selector after harness creation:
  `go test ./internal/docline -run '^TestDoclinePlanNamespace$' -count=1`
* Harness evidence must show a compiling assertion failure before production
  implementation; no harness-satisfied status is implied by this plan

### U2: Set canonical workspace plan directory

* Domain: config; file: `.autoharness/config.yaml`
* Acceptance: the active `docs.subdirectories.plans` value is `plans`, with
  unrelated configuration unchanged
* Posture: Ship provisions the applicable config validation harness first

### U3: Update top-level authoring guidance

* Domain: harness documentation; files: `AGENTS.md`,
  `.github/copilot-instructions.md`
* Acceptance: current plan-directory guidance points to `docs/plans/`

### U4: Update Stage and backlog guidance

* Domain: harness documentation; files: `.github/agents/_stage.agent.md`,
  `.github/instructions/backlogit.instructions.md`
* Acceptance: current plan-path guidance and examples use `docs/plans/`
  without altering workflow gates

### U5: Update architecture and compaction guidance

* Domain: harness documentation; files:
  `.github/instructions/architecture-doc.instructions.md`,
  `.github/skills/compact-context/SKILL.md`
* Acceptance: current plan-directory guidance uses `docs/plans/` without
  changing historical records or unrelated behavior

### U6: Update installed harvest and planning guidance

* Domain: harness documentation; files: `.github/skills/harvest/SKILL.md`,
  `.github/skills/impl-plan/SKILL.md`
* Acceptance: active plan paths use `docs/plans/`; existing validation and
  review gates remain intact

### U7: Update installed hardening and review guidance

* Domain: harness documentation; files: `.github/skills/plan-harden/SKILL.md`,
  `.github/skills/plan-review/SKILL.md`
* Acceptance: active plan inputs use `docs/plans/`; review requirements remain
  unchanged

### U8: Update installed and plugin spike guidance

* Domain: harness documentation; files: `.github/skills/spike/SKILL.md`,
  `plugin/skills/spike/SKILL.md`
* Acceptance: active plan path references use `docs/plans/`

### U9: Update plugin harvest and planning guidance

* Domain: harness documentation; files: `plugin/skills/harvest/SKILL.md`,
  `plugin/skills/impl-plan/SKILL.md`
* Acceptance: active plan paths use `docs/plans/`; gate semantics are unchanged

### U10: Update plugin hardening and review guidance

* Domain: harness documentation; files: `plugin/skills/plan-harden/SKILL.md`,
  `plugin/skills/plan-review/SKILL.md`
* Acceptance: active plan inputs use `docs/plans/`; review requirements remain
  unchanged

For U3 through U10, Ship must obtain the applicable harness-satisfaction or
document-only exemption evidence before execution, and validate the changed
paths with the repository's applicable documentation checks.

## Dependency Graph

U1 and U2 are independent. U3 through U10 have no implementation dependency on
one another. Validate the integrated namespace after U1 and U2 are complete.
Do not invent dependency edges for a preferred execution order.

## Decisions and Rationale

Keep the legacy classifier rule as a compatibility rule. The custody allowlist
already includes both namespaces; the doctor occurrence is a historical comment.
Neither requires a production edit solely for this relocation. Preserve
dated narratives, original stash text, and immutable lifecycle evidence.

The registry exposes `feature` as hierarchy level 1 and `chore` as level 0.
Use a feature as the covering parent for task hierarchy if the tool cannot
parent tasks to a chore; label and describe it as maintenance.

## Risks and Caveats

The classifier is an active CLI/MCP-visible contract. A path-prefix change
must not weaken normalization, containment, or old-path compatibility.
Workspace configuration and installed guidance may temporarily disagree while
the shipment is in progress; complete the integrated validation before
declaring the migration complete. The already observed five SDK anchor warnings
are pre-existing optional reference-check findings, not authoring-gate passes.

## Constitution Check

* Safety-first Go: pass; the production change is bounded to the Go policy
* Test-first development: pass; Ship must capture red then green evidence
* Workspace isolation and CLI containment: pass; operations stay in the
  existing workspace and validation caches must be workspace-contained
* Structured observability and single responsibility: pass; each unit has a
  bounded outcome and traceable evidence
* Destructive approval and safety modes: pass; the operator approved the
  relocation, and no unrelated destructive operation is proposed
* Merge history: N/A; Stage will not merge

Constitution Check: pass

## Plan Hardening Signals

* Public classification contract: present
* Security or permission change: absent; preserve existing containment
* Migration or configuration change: present
* External integration or operator checkpoint: absent
* High runtime or rollout risk: absent; compatibility still needs verification

Requires plan hardening: yes

## Runtime Verification and Closure

Ship validates canonical and legacy classifier behavior, nonmatching prefix
behavior, source-entrypoint plan authoring, current configuration, and relevant
guidance references with workspace-contained caches. The Go harness must be
observed red before U1 implementation and green afterward. Record evidence in
the shipment execution record; if compatibility or path containment regresses,
stop and revert the affected change through the approved Ship workflow.

## Plan Hardening

* **Triggers**: path classification is an agent-facing contract, and the
  canonical workspace configuration is changing
* **Invariants**: recognize both plan prefixes; leave nonmatching paths,
  containment, normalization, custody behavior, and unrelated document types
  unchanged. Do not rewrite historical paths in immutable records
* **Evidence**: Ship reported that
  `go run .\cmd\backlogit docs lint --path docs\plans\2026-11-04-docs-plan-namespace-compatibility-plan.md --format json`
  ran with workspace-contained caches and temporary directories, exited 0, and
  returned `valid: true` and `violation_count: 0`. This was the authoring
  profile on the draft before this body-only hardening record; no frontmatter
  fields changed afterward
* **Pre-implementation**: Ship must establish the actual compiling red harness
  for U1 and any required harness or exemption evidence for U2 through U10.
  Proposed selector names are not evidence of harness satisfaction
* **Verification**: exercise canonical, legacy, and near-prefix paths and
  confirm CLI/MCP-visible scope parity through the shared policy. Validate the
  workspace setting and touched guidance without widening to historical records
* **Rollout**: retain the legacy classifier rule throughout the transition;
  verify the integrated configuration and guidance after both U1 and U2
* **Rollback**: if classification, compatibility, or containment regresses,
  Ship stops execution and reverts the affected change through its approved
  workflow; Stage does not run code or alter shipment lifecycle
* **ProposedAction**: add a canonical classifier rule and update active
  namespace settings and guidance. `ActionRisk: moderate`; operator approved
  the namespace relocation. `ActionResult: planned`
* **Review contract**: capability-aware persona review must cover the classifier,
  config, harness-reference scope, and task sizing, and record a recognized
  `dispatch_mode` and gate `decision` before harvest
* **Learnings**: `docs/compound/2026-06-26-docline-frontmatter-contract.md`;
  `docs/compound/2026-09-04-bounded-error-and-cli-mcp-parity-patterns.md`;
  `docs/compound/2026-07-27-absolutize-filepath-rel-base-for-absolute-walked-paths.md`

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `decision: FAIL`
* `TOOL_DEGRADED: reviewer-subagent-dispatch` — the bounded review evidence
  fits a direct read, so this pass uses the complete sequential persona rubric
  rather than independent reviewer dispatch
* **Constitution**: no P0/P1 findings; test-first, containment, safety mode,
  and role boundaries are explicitly preserved
* **Go**: no P0/P1 findings; the change is limited to the policy prefix table
  and requires an observed compiling red assertion before implementation
* **Scope**: P1 — the required Stage decomposition chain includes sub-epics
  under the covering feature, but the live registry exposes no epic type and
  the feature's allowed children are only task and review. The proposed direct
  feature-to-task hierarchy does not satisfy that Stage chain
* **Learnings**: no P0/P1 findings; source-path self-reference and path
  containment learnings are incorporated
* **Architecture**: no P0/P1 findings; classification uses the existing shared
  policy and custody already recognizes both paths
* **Agent-native parity**: no P0/P1 findings; integrated verification includes
  CLI/MCP-visible scope behavior and active harness guidance
* **Security lens**: not triggered; no auth, secrets, or trust-boundary change
  is proposed, and existing containment must remain unchanged
* **Hardening**: required and present. The source-entrypoint authoring result
  from Ship is recorded above; it preceded body-only hardening and review edits
* **Runtime and closure**: Ship must record red/green and integrated namespace
  evidence, stop on regressions, and use its approved rollback workflow
* **Required resolution**: reconcile the required sub-epic hierarchy with the
  live WIT parent rules before harvest; do not create a partial hierarchy or
  assume an unsupported parent-child relationship
* **Remaining gate**: after the hierarchy issue is resolved and review passes,
  harvest's source-entrypoint authoring check must be obtained on the reviewed
  plan before any backlog item is created

<!-- plan-review-attempt: 1 -->
