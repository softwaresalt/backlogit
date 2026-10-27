---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 160-S Ship session (2026-10-22)"
description: "Dense extract of eleven Ship notes for shipment 160-S (baseline convergence RS-W03, tasks 175.022-T to 175.030-T): intake, wave freeze, harness, per-task lint remediation, and decisions; originals archived."
source: docs/memory/compacted/2026-10-22-160s-ship-session-compacted.md
chunk_strategy: h1-h2-h3
---

# Compacted memory: 160-S Ship session

P-020 compaction (2026-10-27). Originals are preserved under `docs/archive/memory/2026-10-22/` (listed at the end). The shipment is closed; no active work item references these notes.

## Intake and served-root evidence

* Two earlier intake attempts halted: one on a covering-feature membership interpretation, one on `WORKTREE_DIRTY`. The operator then directed Ship to proceed with the unchanged task-only manifest, which supersedes the stale checkpoint resume hint that asked Stage for a manifest amendment.
* Membership is task-only for the multi-wave covering feature `175-F`, per decision `docs/decisions/2026-09-19-baseline-convergence-decomposition.md` H6 and shipments `157-S`, `158-S`, `159-S`, and `176-S`. Adding `175-F` would have archived the feature while queued tasks remained outside the shipment.
* Blocks chain: `160-S -> 159-S -> 158-S -> 157-S -> 176-S -> 154-S`. Condition (b) passed from archived Markdown plus the append-only log (`154-S` shipped; operator `CONDITION_B_ATTESTED` comment).
* Served roots matched the Orchestrator handoff: workspace `.`, storage `.backlogit`, one `main` database at `.backlogit/backlogit.db`. `pipeline-topology pre_claim` passed; branch `feat/160-s-baseline-convergence-rs-w03-remediation-wave-3` was created and the active checkpoint carried forward as commit `e1a200b7`.
* The MCP claim RPC timed out, the first post-claim check returned `CLAIM_NOT_OBSERVED`, and the CLI fallback hit a lifecycle gate. An exact item read showed `160-S` active, and bounded re-verification passed with no claim retry. This matches the learning in `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` and `docs/compound/2026-10-21-claim-stall-under-load-poll-state.md`.

## Wave freeze and harness

* Live status catalog: `queued`, `active`, `blocked`, `review`, `done`, `accepted`, `rejected`, `archived`, `shipped`, `abandoned`. Frozen immutable set `M` is nine tasks, `175.022-T` through `175.030-T`; all depend only on archived `175.011-T`, so the schedule is a single wave. No red-deliverable or green-regression contracts exist (empty mapping and `[]`). Scheduler simulation returned `WAVE_SIM_OK` (200/200 over 26 scenarios).
* All nine tasks were `active` with `scheduler_baseline_claim: 160-S`, classified `claim-assigned`, each with exactly one claim event and no `WORK_STARTED` before build. Scoped P-012 declarations named each raw log path before any read; every read used no-follow open, direct-child and containment checks, and opened-path verification.
* The harness architect scaffolded nine `lintfix_175_0NN_test.go` files, each named-assertion red (finding counts 3, 8, 15, 6, 13, 5, 2, 1, 1), with compile-only checks green. All nine received `harness-ready` labels (several label RPCs timed out; an exact-ID SQL read confirmed them).
* For `175.024-T` the task text names `NewWorkspace`, a wrapper around `newWorkspace`. Decision: clear the declared 15 findings in the declared files without widening scope, and record the rationale in the PR.

## Task outcomes

All nine commits carry the Copilot co-author trailer, were linked to their tasks, and passed standard local review with zero P0 to P3 findings. Each ran its named harness (one named PASS), `go test -run=^$ -count=1 ./...`, and `go vet ./...`, and showed zero diagnostics in owned files from golangci-lint v2.13.2.

| Task | Findings | Files | Commit |
|---|---|---|---|
| `175.022-T` | 3 errcheck | `internal/core/shipment_reconcile_snapshot_windows.go`, `internal/core/artifacts_expansion_test.go` | `96d6256d` |
| `175.023-T` | 8 errcheck | `internal/core/shipment_test.go`, `internal/core/archive_consistency_harness_042_test.go` | `c49a64f6` |
| `175.024-T` | 15 errcheck | `internal/core/workspace.go`, `internal/core/migrate_links_canonical_test.go` | `68450ca3` |
| `175.025-T` | 6 errcheck | `internal/db/connection.go`, `internal/db/connection_spike_test.go` | `d55c3b91` |
| `175.026-T` | 13 errcheck | `internal/db/dependencies.go`, `internal/db/connection_test.go` | `b73c751c` |
| `175.027-T` | 5 errcheck | `internal/db/telemetry_schema.go`, `internal/db/queries.go` | `06c97604` |
| `175.028-T` | 2 errcheck | `internal/events/hook_events.go`, `internal/events/reader.go` | `89b27d53` |
| `175.029-T` | 1 staticcheck (QF1003 tagged switch) | `internal/gateevidence/formal.go` | `ee381e55` |
| `175.030-T` | 1 errcheck (webhook `resp.Body.Close` logged) | `internal/hooks/webhook.go` | `4172a22e` |

## Decisions and failed approaches

* Early halt on `175.022-T` (before its commit): the declared task-lint verifier `scripts/verify-task-lint.ps1` exited 2 (`FixtureRoot is required (real lint is never run)`); it is fixture-only, takes no fixture from the task command, and its success marker differs from the task's declared marker. Ship did not invent a fixture or edit the contract. The Orchestrator later dispositioned it as a Stage-owned contract defect: record `TOOL_DEGRADED` in PR readiness and closure evidence, with CI as backstop. P-021 C2 capture: stash `C007D571` (`DISCOVERY-STATUS: AMBIGUOUS` against candidate `E32ADEC0`), carried in task comments and the PR and closure residual-risk records.
* Toolchain: the environment selects Go 1.27.0 while `go.mod` declares 1.24.0. golangci-lint v2.13.2 built with Go 1.26.5 panicked on the Go 1.27 requirement, and the repo-local v1.64.8 failed typechecking. The resumed session used `bin/lint127/golangci-lint.exe` v2.13.2 (Go 1.27.0) and installed nothing.
* Raw `gofmt -l .` traverses ignored `.copilot/session-state` cache files; use the LF-normalized check over tracked Go files instead.
* Valid primary P-002.6 wrappers were used where the implementation-notes duplicate wrapper had a stale predicate (for example `^TestU175_027_`); neither note was edited.
* A delegated run once edited `.github/agents/subagents/go-engineer.agent.md` outside scope; Ship restored it before review and it is not in the task commit. A transient task-status RPC timeout was resolved by an exact reread (no retry).
* Repository-wide lint stayed nonzero until wave convergence because sibling tasks owned remaining baseline findings. The unfiltered full suite was deferred to the Step 4.6 convergence gate, with isolated reruns for the known flake `TestU172_LockOrder`.
* Dark-mode authority (P-017) covered `160-S` only, merge-commit only, `merge_approval_pre_authorized: true`, admin fallback not authorized. `agent-intercom` is not installed, so visibility was local only. `161-S` stays ineligible until `160-S` closure completes.

## Archived originals

Under `docs/archive/memory/2026-10-22/`:

* `2026-10-22-ship-160s-intake-resumed.md`
* `2026-10-22-ship-160s-175022-complete.md`
* `2026-10-22-ship-160s-175023-complete.md`
* `2026-10-22-ship-160s-175024-complete.md`
* `2026-10-22-ship-160s-175025-complete.md`
* `2026-10-22-ship-160s-175026-complete.md`
* `2026-10-22-ship-160s-175027-complete.md`
* `2026-10-22-ship-160s-175028-complete.md`
* `2026-10-22-ship-160s-175029-complete.md`
* `2026-10-22-ship-160s-175030-complete.md`
