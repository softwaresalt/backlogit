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

## Status: IMPLEMENTATION STARTING (U1)
