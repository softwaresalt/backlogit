# Stage session — D23DFA0B Pre-Task-Completion Gate Broker

- Date: 2026-07-06
- Agent: Stage
- Mode: operator-attended, planning-only (P-010 role isolation: no code/build/PR)
- Entry point: single targeted feature stash `D23DFA0B` (deliberation COMPLETE)

## Tooling status (Step 0.0 / 0.1)
- Registry present: `.autoharness/backlog-registry.yaml` (backlogit, shipments=true,
  dependencies=true, checkpoints=true, stash=true).
- MCP tools not directly exposed this session -> operating via `backlogit.exe` CLI
  fallback (documented CLI-fallback mode). Logged DEGRADED_MODE for MCP; CLI OK.
- Index sync OK via `backlogit sync` (721 artifacts).
- Installed capability packs: backlogit, strict-safety, agent-intercom, agent-engram,
  adversarial-review, release-observability. continuous-learning NOT installed
  (skip observe/learn/evolve). strict-safety active -> plan-harden must classify
  risky actions (ProposedAction/ActionRisk) or plan-review FAILs.

## Classification / routing (Step 1)
- D23DFA0B: feature-shaped, medium. Deliberation complete (design doc + requeue
  decision). Route straight to impl-plan -> plan-harden -> plan-review -> harvest.
- Grouping analysis (Step 1.5) skipped: single feature-shaped entry, not task-shaped.

## Authoritative sources
- Design doc (UNCOMMITTED): docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md
  (section "Deliberation outcomes and autoharness coordination (2026-07-05)" is
  authoritative). 5-unit plan + test matrix + security considerations.
- Requeue decision (UNCOMMITTED): docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md
- Both must be committed as part of harvest (they are this feature's design/decision
  artifacts). Do NOT sweep operator WIP (.github/agents/*, .gitignore, start.ps1,
  .backlogit/hooks_queue.jsonl). Path-scoped git add only.

## Repo grounding (for impl-plan)
- Transition path: core.UpdateArtifact(ctx, ws, id, updates map[string]any) @ internal/core/artifacts.go:463;
  pre-hook FirePre(HookUpdateArtifact) @ :476-490 = insertion point.
- Hooks: internal/hooks/hooks.go (HookPoint, HookContext OldValues/NewValues map[string]any,
  HookFunc, HookRunner.FirePre first-error); builtin_pre.go ValidateStatusTransition pattern.
- Config: internal/config/schema.go:126 LifecycleHooksConfig; defaults.go:473-509; consumed
  internal/core/workspace.go:96-115.
- Task lock: internal/core/task_lock.go lockTaskFile (non-blocking, ErrTaskBusy, NO bounded-wait
  -> broker must ADD bounded-wait retry ~2-5s).
- CLI exit codes: internal/cli/exit_error.go ExitError/ExitCodeFor (add code 6). move.go/update.go.
- MCP: internal/mcp/tools.go handleMoveItem :511, handleUpdateItem :715; errors.go structured errors.
- Shipment ship: internal/core/shipment_lifecycle.go ShipShipment :136; NormalizeShipmentItems.
- NO command-runner seam exists (exec.Command direct in commits.go:132) -> Unit 2 adds GateRunner interface.
- Evidence append: internal/core/shipment.go appendItemEvent/appendItemEventWithCommit :279-315
  (events.NewEventWriter). Mirror for gate evidence (logs-only per Q3 decision).
- Tests: table-driven testify, t.TempDir(), setupTestWorkspace / setupShipmentWorkspace.

## Locked decisions (encode in plan; do NOT re-deliberate)
- Two-level gating (task->done; shipment->shipped both member-evidence aggregate + shipment-diff gate).
- Base-ref = DEFAULT-BRANCH ref passed as --base; autoharness does 3-dot merge-base. Order:
  config base_ref -> --gate-base -> origin/HEAD -> origin/main -> main.
- Exit trichotomy: binary-not-found=setup err; exit2=config err; exit1=GateBlockedError(backlogit exit 6);
  exit0=pass+record evidence (exit0 is pass even if stdout not JSON).
- Three-valued enabled: auto(default, fail-open if unresolvable), true(strict fail-closed),
  false(disabled).
- Min autoharness >=1.4.7 (repeated_failure {count,threshold,reached,action}; --no-count advisory).
- Requeue ownership: BACKLOGIT sole executor. reached+block->queued; reached+escalate->blocked;
  below-threshold blocked->refuse(stay active); pass->done. Honor autoharness breaker, no second breaker.
- Lock: no partial completion; hold lock through gate; bounded-wait then fail-fast retryable; re-read after acquire.
- Force: operator-only CLI --force-gates --force-reason; pass --force to autoharness; backlogit
  pre_task_completion_gate_forced event. No MCP force.
- Evidence storage: item logs only (indexed read-model column is separate follow-up stash 7ED9CE1A).
- Doctor: advisory --check-gate-evidence in first release.
- Security: argv-array exec only; no untrusted-path shell interpolation; force CLI-only+audited;
  stderr truncation for human output, full JSON preserved for machine callers.

## Plan-review cycle state (Step 4) — HALT AT CAP, AWAITING OPERATOR

- Plan: docs/exec-plans/2026-07-06-pre-task-completion-gate-broker-plan.md (docline lint: 0 violations).
- plan-harden: appended `## Plan Hardening` (strict-safety PA-1..PA-5; all high/moderate, none destructive, all planned).
- **Attempt 1 = FAIL** (6 personas). P0 import cycle + P1 pre-hook-can't-hold-lock/redirect +
  gate god-object + ref-error fail-open + trust boundary + non-transactional evidence +
  gate-in-progress class + response contract. ALL resolved via inline-core-service redesign
  (one-way core->gate, TransitionOptions, typed errors in internal/errors leaf).
- **Attempt 2 = FAIL** (5 personas). Constitution -> PASS. 4 NEW P1s: (Go) lock-lifetime vs
  60s stale-TTL cross-process reap; (Arch) sole-mutation-path under-specified at CLI/MCP;
  (Security) unaudited ref-override (`head_ref`/`--gate-base`) empty-diff bypass; (Parity)
  MCP config/setup+timeout collapse to `internal`. ALL resolved (sidecar heartbeat + TTL
  validation; sole-durable-mutation-owner preamble + adapter no-mutation asserts; head pinned
  to HEAD + base-override break-glass audit; distinct MCP gate_config/gate_setup/gate_timeout).
- **Attempt 3 = FAIL on 1 narrow P1** (Go/Security/Parity all PASS). (Arch) below-threshold
  block hard-coded retained status to `active`, but model has `queued|active|blocked|review`
  (verified internal/models/artifact.go:15-19) and `review->done` is valid. RESOLVED in-plan:
  block retains reread `old_status`; core contract carries `old_status` + `new_status ==
  old_status` on block; CLI/MCP report actual retained status; tests cover block-from-active
  AND block-from-review. Re-lint: 0 violations.
- **Cap reached**: `<!-- plan-review-attempt: 3 -->` = max 2 re-entry cycles. Operator
  (12:08 PT) selected Option B: authorize ONE confirmation re-review (attempt 4).
- **Attempt 4 = confirmation pass** (operator-authorized). Go/Security/Parity -> PASS
  (retained-status fix verified Go-safe, security-neutral, symmetric). Architecture -> FAIL
  but PARTIAL on the SAME retained-status finding (NOT a new P0/P1): ST2.2 pure-mapper
  description still said "stays `active`" on below-threshold block, contradicting the
  corrected ST2.3/ST2.5/ST3.2/R7. Reviewer prescribed exact fix; "New P0/P1 introduced: none."
  RESOLVED in-plan: ST2.2 block row now status-agnostic (refuse/no-write; retained status =
  ST2.3 reread old_status via ST2.5 contract). Full-file sweep: no other hard-coded `active`.
  Re-lint: 0 violations. Marker bumped to `<!-- plan-review-attempt: 4 -->`.
- **Per operator attempt-4 mandate**: harvest ONLY on clean PASS; on any non-PASS report and
  do NOT run a 5th attempt. Attempt 4 was not a clean PASS (Architecture FAIL on the stale
  line, now fixed). Stage does NOT self-harvest. Reported to operator; awaiting decision on
  the completed ST2.2 fix. NO 5th review auto-run. Harvest still BLOCKED.
- Non-blocking advisories recorded in the plan's `## Plan Review` (attempt 3) for Ship/Unit 2:
  heartbeat goroutine lifecycle bound to lock release; injectable TTL for fast concurrency
  test; JSONL write-ahead-intent vs "rollback" semantics; variadic-option shim for the
  UpdateArtifact signature change; CLI --json error_class enum parity + read state_changed.

## OPERATOR DECISION REQUIRED (before harvest)
- Option A (recommended): accept the revised plan as the gate outcome; proceed to harvest.
  Rationale: 3/4 personas PASS on attempt 3; the sole remaining P1 is a deterministic,
  status-model-grounded fix already applied and re-linted clean.
- Option B: authorize ONE confirmation re-review (attempt 4) of just the retained-status fix.
- Option C: request further plan changes.

## Original next steps (post-gate, unchanged)
1. impl-plan -> docs/exec-plans/2026-07-06-pre-task-completion-gate-broker-plan.md  [DONE]
2. plan-harden (REQUIRED: high blast radius, security-sensitive, external exec)  [DONE]
3. plan-review (must PASS; Constitution Check + security lens)  [SATISFIED — operator Option A]
4. harvest -> feature 082-F + 5 tasks + subtasks + dependency edges  [DONE]
5. queued shipment (Step 5.5 scope guard)  [DONE — 082-S]
6. commit design doc + requeue decision + plan artifacts + backlog state (path-scoped)  [DONE]

## OPERATOR DECISION (12:14 PT) — Option A: gate satisfied, harvest authorized
- Rationale (operator): attempt 4 returned Go/Security/Parity PASS; the Architecture FAIL was
  the SAME retained-status finding with the reviewer's own prescribed correction applied verbatim
  (no new substantive P0/P1). Another pass on a reviewer-dictated one-line consistency edit is
  thrashing against the cap. Design is substantively sound across all lenses; implementation
  correctness verified downstream by Ship (TDD build + standard + adversarial + Copilot review).
  Treat the plan-review gate as SATISFIED.

## Harvest (Step 5) — DONE
- Feature: 082-F "Pre-Task-Completion Gate Broker" (queued, medium; refs design doc + requeue
  decision + plan; labels gate/autoharness/core-lifecycle/security-sensitive).
- Tasks (5): 082.001-T (U1 config/resolver/probe), 082.002-T (U2 core transition broker),
  082.003-T (U3 CLI/MCP mapping), 082.004-T (U4 evidence/shipment/doctor), 082.005-T (U5 docs).
- Subtasks (17): U1 ST1.1-1.3 (082.001.001..003); U2 ST2.1-2.5 (082.002.001..005);
  U3 ST3.1-3.4 (082.003.001..004); U4 ST4.1-4.3 (082.004.001..003); U5 ST5.1-5.2 (082.005.001..002).
- Dependency edges (17 total): task-level (5) 082.002-T<-082.001-T, 082.003-T<-082.002-T,
  082.004-T<-082.002-T, 082.005-T<-082.003-T, 082.005-T<-082.004-T; intra-unit subtask chains (12)
  sequencing each unit's subtasks. All verified present in item_deps.

## Step 5.5 Shipment — DONE
- Shipment 082-S "Pre-Task-Completion Gate Broker" (status queued; covering_feature 082-F).
- 23 items, scope-guarded to harvest-emitted IDs only (1 feature + 5 tasks + 17 subtasks),
  parent-first dependency order (feature; then U1..U5 each followed by its subtasks). Verified
  via shipment get. Left queued for Ship.

## Step 5.6 Stash archival — DONE
- Stash D23DFA0B archived (status archived; removed from active stash.jsonl -> archive/stash.jsonl).
  Forward reference: promoted to feature 082-F / shipment 082-S.

## Ship execution order
- U1 (082.001-T, no deps) -> U2 (082.002-T) -> {U3 (082.003-T), U4 (082.004-T) parallel-eligible}
  -> U5 (082.005-T, last; depends on U3 and U4).
