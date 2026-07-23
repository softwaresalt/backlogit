---
chunk_strategy: h1-h2-h3
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-22-plan-review-dispatch-capability-deliberation.md
title: "Plan-review dispatch capability: capability-aware gate with declared-degradation fallback"
description: "Make the plan-review -> harvest gate satisfiable and never silently skipped across environments by preferring real reviewer-persona sub-agent dispatch and, where dispatch is unavailable, running a P-012 declared-degradation single-agent persona pass instead of silently skipping or requiring a per-plan operator waiver"
topic: "plan-review persona-dispatch path vs documented waiver (stash 8CD8F46A)"
depth: "standard"
decision_status: "decided"
promoted_to: "both"
linked_artifacts:
  - ".github/skills/plan-review/SKILL.md"
stash_ids:
  - "8CD8F46A"
tags:
  - "governance"
  - "plan-review"
  - "declared-degradation"
  - "P-012"
  - "harness"
  - "portability"
---

## Problem Frame

The Stage pipeline orders `impl-plan → plan-harden → plan-review → harvest`
("Release Unit Pipeline", `AGENTS.md`). The `plan-review` gate
(`.github/skills/plan-review/SKILL.md`) is defined to work by **spawning
independent reviewer persona sub-agents** (Constitution, Go, Scope Boundary,
Learnings Researcher, plus cross-model Architecture Strategist / Agent-Native
Parity / Security Lens).

In some execution environments Stage **cannot dispatch independent reviewer
persona sub-agents**. When shipments 091-S, 092-S, and 093-S were planned in
such an environment, Stage ran an honest **inline single-agent
self-assessment** instead of the formal multi-persona gate, and said so in their
closure records:

- `docs/closure/2026-07-13-091-S-spike-docline-closure.md` — inline
  self-assessment, "no formal plan-review gate evidence exists".
- `docs/closure/2026-07-13-092-S-item-writer-utc-closure.md` — "not the formal
  multi-persona `plan-review` skill"; "No formal plan-review gate was satisfied".
- `docs/closure/2026-07-14-093-S-ship-closure.md` — "an inline single-agent
  self-assessment".

The result is a gate that is **structurally unsatisfiable** in some environments
and is therefore skipped — honestly documented, but with no sanctioned protocol
that makes the skip a *legitimate* gate outcome rather than an ad hoc bypass.

Stash entry `8CD8F46A` asks to **establish a real persona-dispatch path OR a
documented operator waiver so the pre-harvest gate is satisfiable and not
silently skipped.**

### Success criteria

1. The gate is **satisfiable in every supported environment** — never left in a
   state where the only options are "silently skip" or "block Stage entirely".
2. Any degradation is **declared and auditable**, not silent (aligns with
   P-012).
3. **Portable**: the Orchestrator markets the harness as "Environment Agnostic"
   (VS Code Copilot, Copilot CLI, Codex, Cursor, Claude Code). The resolution
   must not assume one environment's sub-agent capability.
4. **DARK_MODE / AFK compatible**: must not require per-plan operator presence.
5. **Lightweight**: no reintroduction of machinery already rejected by prior
   decisions.

### Out of scope

- Retroactively re-running plan-review for the already-shipped 091-S/092-S/093-S.
  This decision does **not** reclassify their honest self-assessments as passed
  plan-review gates; it grants a one-time, explicitly-scoped **pre-policy
  grandfather exception** that preserves their nonconformant status (see Risks).
- Building a per-plan waiver/reservation/session contract (already rejected —
  see Research).
- Writing to the external autoharness template repository (Principle IV
  containment). This decision amends the **generated, committed** runtime
  `SKILL.md` in this repo, which is authoritative at runtime here.

## Research Findings

**Gate ordering is authoritative.** `AGENTS.md` Release Unit Pipeline places
`plan-review` before `harvest`; the deprecated-agents table records that
plan-review "dispatches persona subagents from `.github/agents/review/`".

**P-012 is the principle anchor, not the mechanism.** `.github/policies/workflow-policies.md`
has no policy authorizing a skip when persona sub-agents cannot be dispatched.
The nearest governing policy is **P-012 (Tool Availability and Declared
Degradation)**: "Agents must probe configured tools before relying on them and
must explicitly declare degraded mode when a configured tool is unavailable.
Silent fallback … is a policy violation." Note, however, that P-012's *mechanism*
is scoped to **backlog-registry** tools and their registry-declared `cli_command`
fallbacks (`.github/policies/workflow-policies.md:277-299`); reviewer sub-agent
dispatch is not a backlog-registry tool, so P-012 does not model it directly. This
decision therefore applies P-012's **declared-degradation principle** (probe /
declare / never-silent, `TOOL_OK` / `TOOL_DEGRADED` / `TOOL_UNAVAILABLE`) to a
non-registry capability and defines the permitted fallback and terminal states in
the skill itself. Generalizing P-012 into an explicit capability clause is a
recommended follow-up (see Unresolved Questions).

**The only existing fallback is model-diversity, not dispatch.** Both
`.github/skills/review/SKILL.md` and `.github/skills/plan-review/SKILL.md` say
"multi-model is preferred but not blocking" — that covers running personas on
the *caller's model*, but it still assumes personas can be *dispatched at all*.
There is no sanctioned path for "sub-agent dispatch unavailable".

**Heavyweight waiver machinery was already rejected.** The formal-gate
architecture spike (`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md`)
concluded PIVOT and explicitly discarded "waiver / reservation / session
machinery" and decided "not to reintroduce a plan-digest + waiver contract".

**Persona definitions exist and are dispatchable where supported.** The reviewer
personas are real, committed agent definitions under `.github/agents/review/`
(constitution-reviewer, go-quality-reviewer, scope-boundary-auditor,
architecture-strategist, security-lens-reviewer, template-integrity-reviewer,
concurrency-reviewer) and `.github/agents/research/learnings-researcher.agent.md`
(the Learnings Researcher lives under `research/`, not `review/`).
In dispatch-capable environments (e.g. the Copilot CLI `task` tool, VS Code
Copilot agents) these can be spawned today; the "cannot dispatch" premise is
**environment-specific, not a universal repo limitation**.

## Options Evaluated

### Option A: Mandatory real persona-dispatch only

Make dispatch of the `.github/agents/review/` personas mandatory; the gate is
satisfied only by a genuine multi-persona sub-agent run.

- **Pros**: Highest review fidelity; uses existing persona definitions; removes
  the "skip" path entirely.
- **Cons**: In environments that lack sub-agent support, the gate becomes
  impossible and Stage is **blocked outright** — it relocates the portability
  problem instead of solving it. Not "Environment Agnostic".
- **Effort**: low. **Fit**: fails success criteria 1 and 3.

### Option B: Capability-aware gate + P-012 declared-degradation fallback (chosen)

Probe sub-agent dispatch capability at gate start. **Prefer** real persona
sub-agent dispatch when supported (this is Option A's strength). **When dispatch
is unavailable**, run a sanctioned **single-agent persona pass** — the caller
sequentially adopts each persona's lens using the same manifest definitions
(`.github/agents/review/`, plus `.github/agents/research/learnings-researcher.agent.md`)
as rubrics — and **record the degradation explicitly** in the appended
`## Plan Review` section with a `dispatch_mode` marker and a
`TOOL_DEGRADED: reviewer-subagent-dispatch` line, applying P-012's
declared-degradation principle. Coverage of every selected persona is mandatory
in both modes; a partial dispatch failure forces a complete rubric pass, and if
neither a complete dispatch nor a complete rubric pass can run the gate halts
(`TOOL_UNAVAILABLE`) rather than deciding on partial coverage. The single-agent
pass is a *legitimate, auditable* gate outcome, not a silent skip and not a
per-plan operator waiver.

- **Pros**: Portable across all environments; gate is always satisfiable;
  honest and auditable (P-012 declared-degradation principle); lightweight
  (skill-doc only, no new machinery); DARK_MODE/AFK compatible; analogous in
  principle to the review skill's "preferred but not blocking" stance; and it
  removes the silent-skip path that 091-S/092-S/093-S fell into.
- **Cons**: A single-agent persona pass loses independent-agent (and any
  cross-model) execution, a *stronger* degradation than the review skill's
  same-model persona spawn — an accepted limitation, made explicit by the
  `dispatch_mode` record so a degraded PASS is honestly interpretable.
- **Effort**: low. **Fit**: satisfies all five success criteria.

### Option C: Per-plan documented operator waiver

Require the operator to sign a waiver artifact/field per plan when the gate
cannot run.

- **Pros**: Explicit operator accountability.
- **Cons**: **Already rejected** by the 2026-07-17 formal-gate spike (waiver /
  reservation / session machinery discarded). Requires operator presence per
  plan → breaks DARK_MODE / AFK. Heavyweight relative to the problem.
- **Effort**: medium-high. **Fit**: fails success criteria 4 and 5; contradicts
  a prior decision.

## Trade-off Comparison

| Criterion | A: Mandatory dispatch | B: Capability-aware + declared degradation | C: Per-plan waiver |
|---|---|---|---|
| Gate always satisfiable | No (blocks Stage) | **Yes** | Yes |
| Portable / environment-agnostic | No | **Yes** | Partial |
| Degradation declared, not silent | n/a | **Yes (P-012 principle)** | Yes |
| DARK_MODE / AFK compatible | Yes | **Yes** | No |
| Review fidelity when dispatch is available | Highest | **Highest (prefers dispatch)** | Highest |
| Consistency with prior decisions | Neutral | **Consistent** | Contradicts 2026-07-17 spike |
| Weight / machinery | Low | **Low** | High |

## Decision

**Adopt Option B — a capability-aware plan-review gate with a P-012
declared-degradation fallback.** DARK_MODE, operator AFK, decided autonomously
with the operator's standing "sound, well-reasoned judgement" authorization and
the p3 routing directive ("real plan-review persona-dispatch path vs documented
waiver").

Concretely, amend `.github/skills/plan-review/SKILL.md` to:

1. **Probe dispatch capability** before spawning any reviewer (at the start of
   Step 2), logging `TOOL_OK` / `TOOL_DEGRADED` per P-012's declared-degradation
   principle. Because P-012's registry mechanism does not model sub-agent
   dispatch, the skill defines the permitted fallback and terminal states locally.
2. **Prefer real persona sub-agent dispatch** from the manifest when
   the environment supports it — this is the "real persona-dispatch path" the
   stash asks for, and it is exercised natively wherever sub-agent tooling
   exists (e.g. Copilot CLI, VS Code Copilot agents).
3. **Sanction a single-agent persona-pass fallback** when dispatch is
   unavailable: the caller sequentially applies each persona's rubric, and the
   appended `## Plan Review` section MUST record `dispatch_mode:
   multi-agent-dispatch` or `dispatch_mode: single-agent-declared-degradation`
   plus, in the degraded case, a `TOOL_DEGRADED: reviewer-subagent-dispatch —
   single-agent persona pass` line. A silently skipped gate (no dispatch_mode
   record) remains a plan-review gate-integrity (contract) violation. Because
   P-012's mechanism does not yet model sub-agent dispatch, this is surfaced as a
   local plan-review contract violation rather than a `POLICY_VIOLATION: P-012`
   P-005 event, until P-012 is generalized to capabilities (see follow-up).
4. **Enforce full-coverage terminal states**: a `multi-agent-dispatch` result is
   valid only when every selected persona completes; a mid-gate dispatch failure
   forces a complete sequential rubric pass emitting
   `single-agent-declared-degradation`; if neither a complete dispatch nor a
   complete rubric pass can run, halt with `TOOL_UNAVAILABLE` rather than deciding
   on partial coverage.

This makes the **plan-review gate itself** satisfiable in every environment and
self-declaring — it can no longer silently skip its own dispatch step — which
establishes the persona-dispatch path and declared fallback that `8CD8F46A` asks
for. End-to-end enforcement that a plan can never *reach* `harvest` without a
valid review record (tightening Stage `skip_review` and `harvest` acceptance) is
out of scope here and tracked as the end-to-end enforcement follow-up.

## Rejected Alternatives

- **Option A (mandatory dispatch)** — rejected because it blocks Stage entirely
  in sub-agent-incapable environments, contradicting the harness's
  "Environment Agnostic" contract.
- **Option C (per-plan operator waiver)** — rejected because the 2026-07-17
  formal-gate spike already discarded waiver/reservation/session machinery, and
  because per-plan operator sign-off is incompatible with DARK_MODE / AFK
  autonomous operation.

## Unresolved Questions

- **P-012 capability clause (recommended follow-up)**: P-012's mechanism is
  scoped to backlog-registry tools with `cli_command` fallbacks. This decision
  applies P-012's *principle* to sub-agent dispatch and defines the fallback in
  the skill. Formalizing a general "capability with declared fallback" clause in
  P-012 (or a referenced sub-policy) would give this and future non-registry
  capabilities a policy-level home. Deferred to a governance work item.
- **End-to-end gate enforcement (recommended follow-up)**: the `dispatch_mode`
  record is currently a skill-level contract; it is not yet enforced downstream.
  Stage still permits `skip_review: true`, and `harvest` accepts a plan described
  as reviewed without validating a `dispatch_mode` record. A follow-up should wire
  the hand-off contract (reject `skip_review` without a valid prior review record;
  have `harvest` halt on a missing/invalid `dispatch_mode`). Tracked as a
  follow-up stash entry.
- **Automated dispatch-capability probe**: this decision specifies a
  behavioral/self-declared probe in the skill, not a programmatic capability
  API. A future enhancement could expose an environment capability signal the
  skill can query deterministically. Deferred; not required to satisfy the gate.
- **Upstream template parity**: the amendment lands on the committed runtime
  `SKILL.md`. Propagating the same change into the external autoharness template
  (`plan-review/SKILL.md.tmpl`) is out of scope here under Principle IV and is a
  separate upstream concern.

## Risks and Mitigations

- **Risk**: teams treat the declared-degradation fallback as the default and
  skip real dispatch even where it is available.
  **Mitigation**: the protocol makes dispatch the *preferred* path and requires
  an explicit `dispatch_mode` record; a `single-agent-declared-degradation`
  marker in an environment known to support dispatch is a visible review signal.
- **Risk**: single-agent persona passes miss issues a diverse panel would catch.
  **Mitigation**: the degradation is *declared* so reviewers know the fidelity
  level (a stronger degradation than the review skill's same-model spawn, and
  called out as such); full-coverage terminal states forbid a partial-coverage
  PASS; P0/P1 blocking semantics are unchanged.
- **Risk (retroactive)**: 091-S/092-S/093-S shipped without the formal gate and
  without a `dispatch_mode` / `TOOL_DEGRADED` record.
  **Mitigation**: this decision does **not** reclassify their self-assessments as
  passed plan-review gates — doing so would contradict the new "no dispatch_mode =
  violation" rule. Instead it grants a one-time, explicitly-scoped **pre-policy
  grandfather exception**: those shipments predate this contract, their
  nonconformant status is preserved in their closure records, and the exception
  **cannot** satisfy the gate for any future plan. No rework is required precisely
  because they are grandfathered, not certified.
