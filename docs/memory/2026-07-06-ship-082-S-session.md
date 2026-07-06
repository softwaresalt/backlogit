# Ship session — 082-S Pre-Task-Completion Gate Broker

- Date: 2026-07-06
- Agent: Ship
- Mode: operator-ATTENDING. HALT at P-014 merge gate (no auto-merge). Full pipeline
  incl. mandatory adversarial review (pre-push) + patient Copilot resolution.

## Tooling status (Step 0)
- Registry present; MCP tools not exposed as callable functions this session ->
  CLI-backed mode via `backlogit` (v1.3.0 on PATH; local build v1.2.0). Intentional
  fallback, not degraded.
- autoharness 1.4.7 confirmed: `gate check --json`, `--force`, `--no-count`, `--task`,
  `--base/--head`. Exit codes 0/1/2 documented.
- go1.26.1, golangci-lint v1.64.8, gh authed (softwaresalt).
- INTERCOM_DEGRADED (no MCP/CLI intercom surface) -> report inline to attending operator.
- `backlogit hooks` unavailable in v1.3.0 CLI (hook poll skipped). Checkpoints listed =
  stale April artifacts (quarantined) -> fresh start.
- INDEX_SYNC_OK (744 artifacts).

## Intake / branch (Step 0.5, P-011)
- Shipment 082-S claimed -> active. 23 items (082-F + 5 tasks + 17 subtasks).
- Branch: feat/pre-task-completion-gate-broker off local main d4db750 (Stage harvest
  commit; 1 ahead of origin). Operator WIP left uncommitted (acknowledged exception to
  clean-tree gate): .github/agents/*.agent.md (2 mod + 3 untracked .ship/.stage/_orchestrator),
  .gitignore, start.ps1, .backlogit/hooks_queue.jsonl. PATH-SCOPED git add ONLY.
- P-001 PASS (only 082-F active). Baseline `go test -run=^$ ./...` compiles green.

## Repo grounding (verified line numbers)
- UpdateArtifact @ internal/core/artifacts.go:464; FirePre pre-hook :487; persistArtifact :557.
- lockTaskFile @ task_lock.go:67 (non-blocking TryLock + O_EXCL sidecar; taskStaleLockTTL=60s :16;
  ErrTaskBusy :32). NOT currently held by UpdateArtifact.
- ShipShipment @ shipment_lifecycle.go:136; completeReleaseScope :234; NormalizeShipmentItems.
- Config: ws.Hooks *config.HooksConfig (hooks.yaml); LifecycleHooksConfig @ schema.go:126;
  DefaultHooksConfig @ defaults.go:475; LoadHooks+validate @ loader.go:91-116.
- errors leaf: internal/errors/errors.go (ConfigError/ValidationError patterns; sentinels).
- exit codes: internal/cli/exit_error.go (ExitError/ExitCodeFor; doctor uses 0-4).
- status model: internal/models/artifact.go:14-25 (queued|active|blocked|review|done|...).
- evidence: appendItemEvent/appendItemEventWithCommit @ shipment.go:279-315 (void/best-effort;
  events.NewEventWriter + bldb.IndexEvent). Need error-returning appendItemEventErr.
- .autoharness/config.yaml present but NO lifecycle_hooks/gate section -> real autoharness
  gate check fail-opens (exit 0) on this repo. Good for "no gates -> allowed" runtime check.

## Execution order
U1 082.001-T (config/resolver/probe) -> U2 082.002-T (core broker) ->
{U3 082.003-T CLI/MCP, U4 082.004-T evidence/shipment/doctor} -> U5 082.005-T docs.

## Status: U1, U2, U3 COMPLETE — U4 next

### U1 082.001-T — DONE (committed 06ef77b, marked done+archived)
- errors leaf gate types; gate pkg (types/decision/runner/baseref/probe); config schema block.

### U2 082.002-T — DONE (committed 06ef77b, marked done+archived)
- broker.go/broker_test.go; lockTaskFileWithHeartbeat (task_lock.go) + gate_lock_test.go;
  gate_evidence.go (appendItemEventErr + event consts + gateReportHash); gate_transition.go
  (UpdateArtifactWithGate/TransitionOptions/GateOutcome/ForceSource, full decision handling);
  artifacts.go refactor (UpdateArtifact delegates to gated path; body=updateArtifactUngated);
  workspace.go wiring (GateBroker+gateConfig). gate_transition_test.go (12 tests).
- CRITICAL fail-open-under-auto fix in broker.go: auto-discovery base-ref failure fails OPEN
  under enabled:auto (preserves "no gates configured -> allowed" invariant); explicit base
  override or enabled:true always fails closed (config error). +3 broker subtests.

### U3 082.003-T — DONE (committed f-branch, marked done+archived)
- ST3.1 gate_exit.go: ExitGateBlocked=6/Config=7/Retryable=8; gateExitError mapper; JSON
  payload renderers (renderGatePassJSON/renderGateBlockedJSON); gateHumanMessage.
- ST3.2 move.go + update.go: --gate-base/--force-gates/--force-reason/--json; route through
  UpdateArtifactWithGate; moveGateError maps typed errors -> exit code + payload.
- ST3.3 mcp/gate_errors.go: gateErrorResult(err,requestedStatus) -> structured MCP results
  for all 7 non-pass classes (gate_blocked/requeued/escalated/setup/config/timeout/in_progress)
  with retryable flags + retry_after_ms + allowed_next_actions; gatePassResult envelopes the
  artifact with a gate{} object (hash/head_sha) on passing gated completion. Wired
  handleMoveItem + handleUpdateItem (zero opts, NO MCP force field).
- ST3.4 operator-only force: --force-gates requires --force-reason (both CLI cmds);
  ForceSourceCLI marker; runGatedCompletion rejects Force when ForceSource!=CLI.
- Tests: cli/gate_exit_test.go (exit mapping, payloads, force guardrail),
  mcp/gate_errors_test.go (all classes, retryable, pass envelope). Full suite green;
  go vet clean; golangci-lint clean on touched pkgs; my files gofmt-clean (repo-wide
  CRLF noise ignored — only move.go/update.go/tools.go have real content diffs).

## Backlog state note
- U1/U2/U3 tasks+subtasks marked done via C:\Tools\backlogit.exe (v1.3.0), which
  AUTO-RELOCATES done items to .backlogit/archive/ (queue deletions + archive additions
  currently UNTRACKED/uncommitted). Convention (per 081-S history): separate
  `chore(backlog): mark ... done` commit; archive at post-merge closure. Plan: one
  path-scoped backlog-state commit before PR (exclude hooks_queue.jsonl + operator WIP).

## Next: U4 082.004-T (ST4.1 evidence tests, ST4.2 shipment two-level gating, ST4.3
## doctor --check-gate-evidence advisory), then U5 082.005-T docs, then quality gates,
## standard review, MANDATORY adversarial review (pre-push), pr-lifecycle (Copilot),
## runtime-verification, operational-closure, HALT at P-014.


## U4 (082.004-T) COMPLETE — commit 4bb… feat(core,cli) (082-F U4)
- ST4.1 evidence: injectable gateEvidenceAppend seam on Workspace + appendGateEvent
  dispatch; 3 appenders routed through it. gate_evidence_test.go: 8 tests (passed/
  blocked/requeued/error/forced, no-frontmatter-mutation, evidence_required rollback,
  append-under-lock). GREEN.
- ST4.2 shipment two-level gating: internal/core/shipment_gate.go gateShipmentCompletion
  wired into ShipShipment before completeReleaseScope. validateMemberGateEvidence refuses
  ship if any member is non-terminal, missing passed/forced evidence, or has stale
  head_sha; then full-shipment gate check with NoCount. Fail-open under auto/disabled.
  shipment_gate_test.go: 5 tests. GREEN.
- ST4.3 doctor advisory: --check-gate-evidence flag (default off). doctor.go adds
  FindingMissingGateEvidence + CheckGateEvidence opt; iterates terminal task/subtask,
  scans item logs via latestGatePassEvidence; advisory only (never changes exit code).
  doctor_gate_evidence_test.go: 4 tests. GREEN.
- Quality: build/vet clean; full suite 0 failures (23 pkgs); golangci-lint clean on
  touched pkgs (fixed 1 errcheck: blocked-evidence append now WarnContext-logged);
  gofmt clean on core/cli/mcp. U4 subtasks+task done+synced.

## U5 (082.005-T) COMPLETE — commit 985d994 docs(docs) (082-F U5)
- ST5.1: regenerated docs/cli-reference (move/update gate flags + exit-code long desc,
  doctor --check-gate-evidence). Documented exit codes 6/7/8 + MCP gate_blocked +
  gate_setup/gate_config/gate_timeout/gate_in_progress classes. Command map is a
  runtime export (not committed) — cli-reference regen covers it. make docs-lint: 0 violations.
- ST5.2: NEW docs/pre-task-completion-gate.md (guide): three-valued enabled semantics,
  two-level gating model, autoharness lifecycle-hooks composition, hooks.yaml config
  reference (lifecycle.pre_task_completion_gate), gate evidence, security posture, and
  4 operator runbooks (gate failure, missing/incompatible binary, force override,
  kill-switch). Updated configuration.md hooks.yaml note; index entries in README +
  ARCHITECTURE. U5 subtasks+task done+synced.

## ALL 5 UNITS COMPLETE. Next: path-scoped chore(backlog) commit (.backlogit/queue +
## archive, EXCLUDE hooks_queue.jsonl + operator WIP), then standard review, MANDATORY
## adversarial review (pre-push), pr-lifecycle (Copilot patient resolve), runtime-
## verification (real autoharness 1.4.7), operational-closure, HALT at P-014 merge gate.
