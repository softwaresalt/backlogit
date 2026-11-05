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
* Current operator and harness entry points: U3 through U11
* Installed harness manifest coherence: U12
* Historical evidence, event logs, checkpoint records, and original captured
  stash text remain unchanged

## Implementation Units

All units require an existing covering feature or chore with a configured
task-child edge, at most two implementation files per task, a verifiable
acceptance criterion, and Ship-owned harness or document-only exemption
evidence before execution. Ship must not label a task harness-satisfied without
the evidence required by P-002. The proposed selectors below are not claims
that a harness already exists.

### Covering release-unit red gate

Before **any** implementation under the covering feature or chore, Ship and
harness-architect must create a compiling assertion harness and observe a
genuine assertion failure. Use the existing `docline.Classify` API: assert
that `docline.Classify("docs/plans/x.md") == docline.DocTypePlan`; the current
classifier returns `guide`. A proposed root selector is
`go test -v -count=1 -run '^TestU1_' ./internal/docline`.
Record the exact test, compilation success, assertion-failure output, and
revision before proceeding. A compile error, missing test, `t.Skip`, source
shape check, constant-only assertion, or tool failure is not red evidence.
This same real regression may be U1's task harness; turn it green with U1.
This release-unit precondition does not replace each task's own P-002/P-004
or P-002.1 gate, pre-work check, and green evidence. The selector alone does
not fail closed on zero tests; Ship's executable task command and consumer
must reject a no-test result and require a matching named assertion result.

### U1: Classify canonical and legacy plan paths

* Domain: code; production file: `internal/docline/policy.go`
* Posture: test-first; Ship creates a compiling assertion harness before the
  production edit, separately from this plan
* Acceptance: canonical `docs/plans/x.md` and legacy
  `docs/exec-plans/x.md` classify as `plan`; a nonmatching
  `docs/plans-extra/x.md` remains `guide`
* Proposed red and green selector after harness creation:
  `go test -v -count=1 -run '^TestU1_' ./internal/docline`
* Harness evidence must show a compiling assertion failure before production
  implementation; no harness-satisfied status is implied by this plan. Ship
  must supply the executable non-vacuity guard before using the selector

### U2: Set canonical workspace plan directory

* Domain: config; file: `.autoharness/config.yaml`
* Acceptance: the active `docs.subdirectories.plans` value is `plans`, with
  unrelated configuration unchanged
* Posture: Ship provisions the applicable config validation harness first

### U3: Update top-level authoring guidance

* Domain: harness documentation; files: `AGENTS.md`,
  `.github/copilot-instructions.md`
* Acceptance: current plan-directory guidance points to `docs/plans/`

### U4: Update backlog guidance

* Domain: harness documentation; file:
  `.github/instructions/backlogit.instructions.md`
* Acceptance: current plan-path guidance and examples use `docs/plans/`
  without altering workflow gates. Do not edit the already-corrected Stage
  contract

### U5: Update architecture and compaction guidance

* Domain: harness documentation; files:
  `.github/instructions/architecture-doc.instructions.md`,
  `.github/skills/compact-context/SKILL.md`
* Acceptance: current plan-directory guidance uses `docs/plans/` without
  changing historical records or unrelated behavior

### U6: Update installed planning and harvest guidance

* Domain: harness documentation; files: `.github/skills/impl-plan/SKILL.md`,
  `.github/skills/harvest/SKILL.md`
* Acceptance: active plan paths use `docs/plans/`; existing validation and
  review gates remain intact. The green-reference pointer in impl-plan resolves
  to the relocated plan. Both skills describe migration previews accurately
  as metadata, not a frontmatter diff; do not claim a diff capability the CLI
  or MCP lacks. A safe alternative is manual inspection and correction of the
  frontmatter without applying migration. Preserve the already-corrected
  harvest hierarchy and its frontmatter gate

### U7: Update installed hardening and review guidance

* Domain: harness documentation; files: `.github/skills/plan-harden/SKILL.md`,
  `.github/skills/plan-review/SKILL.md`
* Acceptance: canonical inputs default to `docs/plans/`, while explicit legacy
  `docs/exec-plans/` inputs remain accepted; review requirements are unchanged

### U8: Update installed and plugin spike guidance

* Domain: harness documentation; files: `.github/skills/spike/SKILL.md`,
  `plugin/skills/spike/SKILL.md`
* Acceptance: active plan path references use `docs/plans/`

### U9: Update plugin planning guidance

* Domain: harness documentation; file: `plugin/skills/impl-plan/SKILL.md`
* Acceptance: active plan paths use `docs/plans/`; gate semantics are unchanged.
  Do not edit the already-corrected plugin harvest contract

### U10: Update plugin hardening and review guidance

* Domain: harness documentation; files: `plugin/skills/plan-harden/SKILL.md`,
  `plugin/skills/plan-review/SKILL.md`
* Acceptance: canonical inputs default to `docs/plans/`, while explicit legacy
  `docs/exec-plans/` inputs remain accepted; review requirements are unchanged

### U11: Correct the docline authoring guide

* Domain: documentation; file: `docs/docline-frontmatter-authoring-guide.md`
* Acceptance: the active taxonomy identifies canonical and legacy plan
  namespaces, and the standardization-plan pointer resolves under
  `docs/plans/`. Historical narrative is not rewritten

### U12: Reconcile harness manifest namespace records

* Domain: configuration; file: `.autoharness/harness-manifest.yaml`
* Acceptance: the active `DOCS_PLANS_DIR` is `plans`, checksums for artifacts
  changed by this maintenance match their final contents, the `config_hash`
  matches the final LF-normalized config, and unrelated manifest records and
  historical notes are preserved. Reject duplicate YAML keys
* Posture: Ship applies the harness workflow and records evidence without
  overwriting the independently completed hierarchy correction

### Task verification contracts

The closed `docs-only` exempt set is U3 through U11, and no other unit is
exempt. Each member changes only the documentation or harness guidance named
above. U1, U2, and U12 are non-exempt and require their own compiling,
assertion-red harnesses in their eligible wave. The covering release-unit
assertion remains an additional precondition before any implementation.

For every docs-only unit, the harvested task must carry a P-002.1
`harness-exemption-contract` body block with exactly these five keys, in
order: `harness_exemption_class: docs-only`, a specific
`harness_exemption_reason`, `harness_owner: none`, an exact read-only
`exempt_verification_command`, and
`exempt_precondition: must-fail-before-deliverable`. After creation resolves
the actual task ID, bind its command to the unit's real remaining deliverables
and exact `EXEMPT_VERIFY_OK:{task_id}` success marker. Do not write a
placeholder command or claim `harness-exempt` readiness without the complete
contract. The command must establish the unit's concrete acceptance criteria
without treating missing files, tool errors, or vacuous output as success.
It must fail the P-002.3 pre-work probe and pass non-vacuously after delivery.
Ship must screen it under P-002.5, validate static and claim-time conditions,
and verify the P-002.4 delta. Do not scaffold a Go or assertion-failing harness
for an eligible docs-only task.

For non-exempt units, the task's own `harness_cmd` is separate from all
regression declarations. Ship and harness-architect record an executable
task-scoped command and prove compilation and named assertion RED before its
implementation, then named PASS afterward. The command must name an explicit
package, use `-count=1`, anchor `^TestU<unit>_`, and fail closed on a zero-test
result. No task is labeled harness-satisfied merely by this plan.

The optional `green-regression-contract` is independent of those commands.
Only a task with a genuinely required additional scoped regression declares
the canonical JSON block; never copy its own harness or exemption command into
that block merely to populate it. Ship freezes the exact declared arrays at
Step 3; an absent block means exactly `[]`, with no inferred command. Ship
executes and verifies each current wave and its convergence under P-002.6.

Only remaining deliverables against the current branch enter the shipment.
Do not create a future exempt task for already-delivered 6bb4e451, 756df591,
or 9c028c85 changes, or accept a pre-work command that already passes. If
a proposed unit's specific deliverable is already satisfied, omit or rescope
it without fabricating a failure. Verify changed guidance and actual links.

## Dependency Graph

The release-unit red gate precedes all implementation; it is a gate, not a
synthetic task. U1 through U11 are otherwise independent implementation units.
U12 depends on U2, U3, U4, U5, U7, and U8: those units change the configuration
or active installed-artifact checksums represented in the manifest. U6 and
plugin-only U9/U10, as well as guide U11, are not active manifest inputs.
Validate the integrated namespace after U1 and U2 are complete. Suspend
canonical migration writes until U1 is green. Do not invent dependency edges
for a preferred order.

## Decisions and Rationale

Keep the legacy classifier rule as a compatibility rule. The custody allowlist
already includes both namespaces; the doctor occurrence is a historical comment.
Neither requires a production edit solely for this relocation. Preserve
dated narratives, original stash text, and immutable lifecycle evidence.

The active work-item configuration allows tasks directly beneath a covering
feature or chore. Select a supported parent after refreshing the live type and
template metadata; plan headings are not backlog items.

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
* Destructive approval and safety modes: Ship enters freeze-scope mode for
  classifier, configuration, and current authoring references; destructive
  operations still require the applicable approval
* Git-friendly persistence and context efficiency: pass; use versioned
  artifacts, bounded work items, and targeted queries
* Merge history: Ship must verify and use a merge commit if the release later
  reaches merge; Stage does not merge

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
guidance references with workspace-contained caches and Go 1.24. Test POSIX
and backslash forms, absolute, escaping and dot-segment rejection through
`ValidateClassifyPath`, and both CLI and MCP classify surfaces for canonical,
legacy, near-prefix and invalid inputs. Check CLI and MCP migration planning
on canonical and legacy fixtures, compare report metadata and verify no writes.
Because those reports do not show before/after content, separately assert the
shared `docline.PlanMigration` fixture's planned `After` has `doc_type: plan`,
the correct self-source, and unchanged body bytes. Preserve pre-existing
behavior and do not weaken assertions. The release-unit harness must be observed red before any
implementation and green after U1. Record command, exit and assertion evidence.

Before execution Ship must freeze the explicit shipment task IDs and evaluate
P-001, P-011, P-002/P-004, P-002.6, and applicable topology and served-root
gates. The current branch is `stage/212-S-multi-session-mcp-runtime` with
uncommitted work; neither branch suitability nor a clean-worktree gate has
been established. P-011 requires a matching shipment branch and otherwise
halts with `BRANCH_MISMATCH`; do not infer an exception for the existing PR
or claim that an origin-main/clean-branch gate passed. No new worktree or
branch is authorized by this plan. Resolve any real gate conflict through the
owning workflow before implementation.

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
* **Pre-implementation**: before any implementation Ship establishes the
  actual compiling release-unit red harness, then task-scoped harness or valid
  exemption evidence for U1 through U12. Proposed selectors are not evidence
  of harness satisfaction
* **Verification**: exercise canonical, legacy, and near-prefix paths and
  confirm CLI/MCP-visible scope parity through the shared policy. Validate the
  workspace setting and touched guidance without widening to historical records
* **Rollout**: retain the legacy classifier rule throughout the transition;
  verify the integrated configuration and guidance after both U1 and U2
* **Rollback**: if classification, compatibility, or containment regresses,
  Ship stops. Retain minimal canonical `docs/plans/` and intentional legacy
  recognition while files, live configuration, or active references use the
  canonical directory. Never blindly revert compatibility support while those
  consumers remain. Full namespace rollback requires fresh approval and a
  coordinated file, configuration, and reference migration; verify document
  types in both namespaces before removing canonical support. Do not relocate
  the pre-existing plans without that approval. Stage does not execute rollback
* **ProposedAction**: add canonical classification and update active namespace
  configuration and guidance. Targets are U1 through U12; change kind is a
  migration and shared-contract/config update. `ActionRisk: high`.
  `approval_required: true`; the operator approved this bounded relocation.
  `ActionResult: approved` and not yet applied. Rollback must follow the
  compatibility-preserving conditions above; additional relocation requires
  separate approval
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

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `decision: FAIL`
* `TOOL_DEGRADED: reviewer-subagent-dispatch` — six leaf personas were
  dispatched, but the learnings researcher could not assess the plan because
  its role is restricted to the compound library. A complete sequential
  persona-adapter pass was therefore performed; the partial dispatch was not
  treated as a complete multi-agent gate
* **Constitution, P1**: the plan allows document-only exemption evidence for
  U3 through U10 and makes those units independent of U1, but does not require
  the covering feature or chore's compiling, observed-failing test harness
  before implementation begins. Principle II and the development workflow
  require that release-unit precondition. State the precondition explicitly
  and preserve P-002.1 task-level exemption conditions where applicable
* **Go, P2**: the proposed classifier examples do not independently establish
  the stated normalization, containment, and CLI/MCP parity invariants. Require
  focused existing or new checks for both interfaces and invalid paths
* **Scope**: no independent P0 or P1 finding; direct tasks under the configured
  feature or chore are supported and the earlier hierarchy blocker is resolved
* **Learnings**: no P0 or P1 contradiction found. Prior art on source-path
  correctness, lexical validation, containment, and CLI/MCP parity is relevant
* **Architecture, P2**: the active docline authoring guide still lists only the
  legacy plan namespace. Rollback must coordinate classifier, configuration,
  and guidance rather than leaving canonical authoring misclassified
* **Agent-native parity, P2**: the guide, the relocated green-reference link,
  and affected harness manifest values/checksums need explicit bounded
  reconciliation. Verify both classify entry points and non-writing
  migration planning, not scope output alone
* **Constitution, P2/P3**: record the applicable explicit safety posture,
  Git-friendly persistence, context efficiency, and Ship's merge-commit rule
* **Security lens**: not triggered; the work does not introduce authentication,
  secrets, an external integration, or a new trust boundary
* **Hardening**: required and present, but the release-unit red precondition
  and coordinated rollback require clarification before the next gate
* **Remaining gate**: after a later review passes, Ship must validate the
  final reviewed plan through the source-entrypoint authoring check before
  harvest. No backlog items may be created from this failed review

<!-- plan-review-attempt: 2 -->

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `decision: FAIL`
* **Architecture, P1**: the rollback permits removing canonical classification
  while canonical documents still exist under `docs/plans/`. Reverting the
  authoring guidance does not protect those documents: they would classify as
  guides and migration could rewrite their type. Keep canonical classification
  while such documents exist, or require an explicitly authorized relocation or
  equivalent compatibility mechanism before removal
* **Constitution, P2**: the proposed U1 selector does not use the P-002.6
  task-unit prefix. Task selectors and canonical task-body
  `green-regression-contract` blocks must be fixed before wave scheduling,
  rather than deferred until after harness generation
* **Go and architecture, P2**: specify fixture-based migration-plan assertions
  for derived `doc_type`, `source`, and unchanged body bytes; compare
  non-writing CLI/MCP reports on canonical and legacy fixtures. Prevent
  canonical migration writes until U1 is green
* **Scope and architecture, P2**: include the LF-normalized manifest
  `config_hash`, limit U12 dependencies to actual manifest inputs, and remove
  currently corrected guidance files from edit scope unless a specific stale
  reference remains
* **Agent-native parity, P2**: retain explicit legacy input support in review
  and hardening; correct any misleading diff-first migration guidance without
  inventing a tool capability
* **Learnings**: relevant prior art covers non-vacuous assertions, source-path
  correctness, relative-root parity, body preservation, and duplicate YAML
  keys. No independent P0/P1 contradiction was established
* **Hardening**: required and present, but rollback remains unsafe for existing
  canonical documents. The release-unit compiling assertion-red gate is now
  explicit and does not rely on a build or tool failure
* **Decision consequence**: attempt 3 exhausts this plan's review budget.
  Halt and escalate; do not start a fourth review or harvest a partial hierarchy

<!-- plan-review-attempt: 3 -->

## Operator-Authorized Review Extension

At `2026-11-05T05:07:18Z`, the operator answered **"Yes"** to one additional
remediation and review cycle for this namespace compatibility maintenance
plan only. The maximum is review attempt 4 and remediation cycle 3. Attempts
1, 2, and 3 remain `FAIL`; their findings and history are not reset.

The approved remediation must retain canonical classification while files
remain in `docs/plans/`, or coordinate an explicitly authorized file and
configuration rollback before removing canonical support. This is not
acceptance of residual risk. No further attempt beyond 4 is authorized
without a new operator decision.

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `decision: FAIL`
* **Constitution, P1**: the plan incorrectly places each task's own selector
  inside the `green-regression-contract` and requires test-prefixed selectors
  even for potentially exempt tasks. P-002.6 treats a task's own harness or
  exemption command separately from the optional block of additional
  regression commands. An exempt task may have no harness test function, so
  this conflation can make its wave gate vacuous or unsatisfiable
* **Go and constitution, P2**: the proposed U1 command alone can pass without
  selecting tests and lacks verbose named-test evidence. Its eventual executable
  verification must fail closed on a vacuous pass
* **Constitution, P2**: review the stated risk level and explicit approval
  fields against strict-safety's migration classification; the extension is
  not authority for additional writes
* **Scope and architecture, P2**: the manifest's impl-plan references are
  historical rather than active checksum records; U12's dependency on U6 is
  not justified by a current manifest input
* **Agent-native parity, P2**: the installed harvest skill still describes
  migration as diff-first even though the public reports expose metadata,
  not before/after content. Correct that bounded guidance without weakening
  the harvest frontmatter gate
* **Learnings**: relevant prior art covers honest non-vacuous red evidence,
  source/body preservation, path containment, and manifest integrity
* **Hardening**: required and present. The attempt 3 rollback P1 is resolved:
  the revised contract retains canonical support while canonical consumers
  remain and requires approval before a full namespace rollback
* **Decision consequence**: the sole operator-authorized extra cycle is
  exhausted. Halt and escalate; do not conduct a fifth review or harvest a
  partial hierarchy

<!-- plan-review-attempt: 4 -->

## Additional Operator-Authorized Review Extension

At `2026-11-05T05:48:16Z`, the operator stated **"Additional review cycle
approved"** for this namespace compatibility maintenance plan only. The
maximum is review attempt 5 and remediation cycle 4. Attempts 1 through 4
remain `FAIL`; their findings and history are not reset. No sixth attempt or
residual-risk acceptance is authorized.

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `decision: ADVISORY`
* **Constitution, P2**: each docs-only exemption command must check the
  required content and then run the documentation lint gate, as required by
  P-002.3. The plan does not state the latter part explicitly
* **Go, P2**: shared migration-plan verification should independently assert
  canonical and legacy fixtures each appear without findings and have the
  correct planned `doc_type`, self-source, and preserved body bytes
* **Architecture, P2**: U5 must address the compaction skill's existing plan
  relocation behavior and require applicable approval and reference
  preservation before moving pre-existing canonical plans
* **Agent-native parity, P2**: U5 must retain discovery of legacy plans while
  making the canonical directory the primary target; otherwise routine
  compaction may omit legacy plans
* **Scope**: no independent P0 or P1 findings
* **Learnings**: relevant prior art covers honest red evidence, body
  preservation, path containment, manifest integrity, and exemption
  boundaries. No independent P0 or P1 contradiction was established
* **Hardening**: required and present. The attempt 4 command-contract P1 is
  resolved: task harness or exemption commands are separate from optional
  additional regression commands
* `operator_authorization: approved`
* **Operator decision**: at `2026-11-05T07:13:15Z`, the operator stated
  **"Yes, proceed"** with all four controls below. This authorizes proceeding
  with the advisory; it does not waive P-002.3 or any other gate
* **Applied control — U3 through U11**: before creating any exempt task,
  require its exact P-002.3 command to check the required deliverable content
  and then run the documentation lint gate. Preserve the pre-work failure,
  post-deliverable success, evidence marker, and other exemption checks
* **Applied control — migration verification**: independently verify
  canonical and legacy fixtures for expected type, self-source, preserved body
  bytes, and absence of findings
* **Applied control — U5 relocation**: preserve affected references and
  obtain applicable approval before relocating existing canonical plans
* **Applied control — U5 discovery**: retain legacy plan discovery alongside
  the primary canonical scan
* **Current-baseline guard**: do not create exempt tasks for changes already
  delivered by 6bb4e451, 756df591, or 9c028c85; their pre-work checks would
  start green

<!-- plan-review-attempt: 5 -->
