---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for feature 166-F (shipment 206-S, Phase 1): add remediation and retry_after_ms to CLI --json gate-error payload, flip S4 expected-divergence scenario from report-only to fail-closed, and add cross-surface field-parity regression tests.'
doc_type: plan
docline:
  feature_id: 166-F
  shipment_id: 206-S
  source_decision: docs/decisions/2026-10-30-gate-payload-parity-remediation-deliberation.md
schema_version: "1.0"
source: docs/exec-plans/2026-10-30-gate-payload-parity-remediation-plan.md
title: 'Implementation Plan: Cross-surface gate-payload parity remediation (166-F)'
---

## Objective

Remediate the tracked cross-surface gate-error payload drift owned by feature `166-F` (superseding archived `156.007-T`):

1. Add `remediation` and `retry_after_ms` to the CLI `--json` gate-error payload (`gateJSONPayload` and `renderGateErrorJSON` in `internal/cli/gate_exit.go`) so `backlogit move --json` and `backlogit update --json` match the MCP `gateClassResult` contract (`internal/mcp/gate_errors.go`).
2. Flip the S4 parity comparator (`internal/faultline/parity/comparator.go`) and recurring-failure scenario (`KnownGatePayloadDriftScenario` in `internal/faultline/parity/scenarios.go`) from `report_only` to fail-closed (`StatusFail`).
3. Add real cross-surface CLI/MCP field-parity regression tests and transition `TestU3SeedCorpus_TrackedDefectResolution_Valid` so `166-F`'s completion and archival at shipment closure keeps the test suite green.

Source decision: `docs/decisions/2026-10-30-gate-payload-parity-remediation-deliberation.md`.

## Activation Contract & Routing Context

* `DARK_MODE_ACTIVE`: `true`
* `activation_utc`: `2026-10-30T05:55:05Z`
* `operator_request_verbatim`: `'Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode.'`
* `bounded_ordered_capability_scope`: `["Phase1 existing feature 166-F", "Phase2 active stash DE3E7B67", "Phase3 active stash A441A74C"]`
* `cursor`: `last_completed=none, next=Phase1/166-F`
* `merge_approval_pre_authorized`: `true` (within this exact scope after current-HEAD local readiness + CI + review gates; merge commits only per P-009)
* `admin_fallback_pre_authorized`: `false`
* `routing_note`: `ROUTING_DEGRADED: context_tier` (Stage resolved `gemini-4-argon/google/high/long_context`, dispatched at `default` context tier; propagated to skills). `TOOL_DEGRADED: engram` (workspace-status timed out twice during startup assessment; using allowed direct codebase `grep`/`view` fallback).

## Problem Frame

When a gated task or subtask transition fails with a typed `*corerrors.GateError` (`Class` in `{"setup", "config", "timeout", "in_progress"}`):

* The MCP surface (`gateClassResult` in `internal/mcp/gate_errors.go:77-105`) emits:
  * `remediation`: `"Install or repair the autoharness binary (min 1.4.7), or disable the gate for this workspace."` for `"setup"`, and `"Fix the gate configuration or base/HEAD ref, then retry the completion."` for `"config"`.
  * `retry_after_ms`: `ge.RetryAfterMs` for `"timeout"` and `"in_progress"` when `ge.RetryAfterMs > 0`.
* The CLI `--json` surface (`gateJSONPayload` and `renderGateErrorJSON` in `internal/cli/gate_exit.go:79-95, 166-178`, invoked by `moveGateError` in `internal/cli/move.go:139-144` for both `move --json` and `update --json`) omits `remediation` and `retry_after_ms`.
* The S4 cross-surface parity comparator (`internal/faultline/parity/comparator.go:138-154`) and corpus (`internal/faultline/parity/scenarios.go:409-483`) currently allowlist this exact `{remediation, retry_after_ms}` drift as `StatusReportOnly` pinned to `TrackedDefectGatePayload = "166-F"`, and `internal/faultline/parity/scenarios_test.go:110-119` (`TestU3SeedCorpus_TrackedDefectResolution_Valid`) asserts `166-F` is non-terminal in the live `.backlogit/` workspace.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Add `Remediation string `json:"remediation,omitempty"`` and `RetryAfterMs int `json:"retry_after_ms,omitempty"`` to `gateJSONPayload` and populate them in `renderGateErrorJSON` (`internal/cli/gate_exit.go`) matching MCP `gateClassResult`, while preserving existing exit codes, messages, and `omitempty` behavior; expose `RenderGateErrorJSON` and `GateExitError` for cross-package parity testing | `166-F` & `2026-10-30-gate-payload-parity-remediation-deliberation.md` (Option A, item 1) | U1 (`166.001-T`) |
| R2 | Flip the S4 expected-divergence comparator rule (`knownDrifts` in `internal/faultline/parity/comparator.go`) and corpus scenario (`KnownGatePayloadDriftScenario` in `internal/faultline/parity/scenarios.go`) from `StatusReportOnly` to fail-closed (`StatusFail`), include MCP's `gate_*` category tokens in `exitCodeClasses`, and expose `GateErrorResult` in `internal/mcp/gate_errors.go` | `166-F` & `2026-10-30-gate-payload-parity-remediation-deliberation.md` (Option A, item 2) | U2 (`166.002-T`) |
| R3 | Add real cross-surface CLI/MCP field-parity regression tests driving `cli.RenderGateErrorJSON` and `mcp.GateErrorResult` through `parity.CompareResults` across all `GateError` classes, update comparator/corpus tests for fail-closed behavior, and make `TestU3SeedCorpus_TrackedDefectResolution_Valid` lifecycle-safe when `166-F` is completed and archived | `166-F` & `2026-10-30-gate-payload-parity-remediation-deliberation.md` (Option A, item 3) | U3 (`166.003-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Each unit modifies at most 3 files, at most 4 functions, at most 3 test scenarios, and stays within a single skill domain (`code` or `tests`).
* **Preserve existing codes, types, messages, and optional-field (`omitempty`) behavior**:
  * Do not change CLI exit codes (`ExitGateBlocked = 6`, `ExitGateConfig = 7`, `ExitGateRetryable = 8`, `ExitShipmentGovernance = 9`).
  * `Remediation` and `RetryAfterMs` MUST carry `omitempty` on `gateJSONPayload` so `renderGatePassJSON`, `renderGateBlockedJSON`, and classes where those fields do not apply omit the keys completely.
  * `RetryAfterMs` MUST only be set when `(ge.Class == "timeout" || ge.Class == "in_progress") && ge.RetryAfterMs > 0`.
* **Genuine tests-first RED -> GREEN**: Every `code` unit specifies the exact failing test assertions that must be observed RED before the production edit turns them GREEN. No `harness-exempt` tasks exist in this plan (`closed_exempt_set: []`).

## Implementation Units

### U1: Add `remediation` and `retry_after_ms` to CLI `--json` gate-error payload (`code`) — `166.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect`)**:
    1. In `internal/cli/gate_exit_test.go`, add `TestU1_RenderGateErrorJSON_RemediationAndRetryAfterMs` testing `renderGateErrorJSON` (and exported `RenderGateErrorJSON` / `GateExitError` once declared, or raw JSON map assertions against `renderGateErrorJSON`) across:
       * `Class: "setup"` -> raw JSON map has `"remediation": "Install or repair the autoharness binary (min 1.4.7), or disable the gate for this workspace."`, omits `"retryable"`, omits `"retry_after_ms"`.
       * `Class: "config"` -> raw JSON map has `"remediation": "Fix the gate configuration or base/HEAD ref, then retry the completion."`, omits `"retryable"`, omits `"retry_after_ms"`.
       * `Class: "timeout", RetryAfterMs: 1500` -> raw JSON map has `"retryable": true`, `"retry_after_ms": float64(1500)`, omits `"remediation"`.
       * `Class: "in_progress", RetryAfterMs: 2000` -> raw JSON map has `"retryable": true`, `"retry_after_ms": float64(2000)`, omits `"remediation"`.
       * `Class: "timeout", RetryAfterMs: 0` (and `-50`) -> raw JSON map has `"retryable": true`, omits `"retry_after_ms"`, omits `"remediation"`.
       * `renderGatePassJSON` and `renderGateBlockedJSON` -> raw JSON maps omit both `"remediation"` and `"retry_after_ms"`.
    2. In `internal/cli/move_gate_error_test.go`, extend `TestMoveGateError_JSONEmitsGateErrorPayload` (or add `TestU1_MoveGateError_JSONParityFields`) covering `setup`, `config`, `timeout` (`RetryAfterMs: 1500`), and `in_progress` (`RetryAfterMs: 2000`) through `moveGateError(cmd, id, ge, true)` and asserting the decoded JSON stdout carries the matching `remediation` and `retry_after_ms` fields and exit codes (`7` for `setup`/`config`, `8` for `timeout`/`in_progress`).
    3. Observe RED (`go test -count=1 -run 'TestU1_|TestMoveGateError_JSONEmitsGateErrorPayload' ./internal/cli` fails because `gateJSONPayload` and `renderGateErrorJSON` omit `remediation` and `retry_after_ms`).
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/cli/gate_exit.go`, add `Remediation string `json:"remediation,omitempty"`` and `RetryAfterMs int `json:"retry_after_ms,omitempty"`` to `gateJSONPayload`.
    2. In `internal/cli/gate_exit.go`, update `renderGateErrorJSON(id string, ge *corerrors.GateError) (string, error)` to populate `p.Remediation` for `"setup"` and `"config"` and `p.RetryAfterMs = ge.RetryAfterMs` for `"timeout"` and `"in_progress"` when `ge.RetryAfterMs > 0`.
    3. In `internal/cli/gate_exit.go`, export `RenderGateErrorJSON(id string, ge *corerrors.GateError) (string, error)` and `GateExitError(err error) *ExitError` (with `renderGateErrorJSON` and `gateExitError` delegating to them) so cross-surface tests in `internal/faultline/parity` can invoke the production CLI gate-error renderer and exit-code mapper directly.
* **Affected files**:
  * `internal/cli/gate_exit.go`
  * `internal/cli/gate_exit_test.go`
  * `internal/cli/move_gate_error_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing on missing `remediation` and `retry_after_ms` keys before `internal/cli/gate_exit.go` is updated.
  * GREEN phase passes `go test -count=1 ./internal/cli` with all existing and new gate exit tests green.
* **Execution posture**: test-first.

### U2: Flip S4 gate-payload drift from report-only to fail-closed and expose MCP gate-error renderer (`code`) — `166.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`166.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect`)**:
    1. In `internal/faultline/parity/scenarios.go`, update `KnownGatePayloadDriftScenario()` so `Validate` asserts fail-closed behavior when CLI `--json` omits `remediation` and `retry_after_ms` that MCP emits:
       * `requireDimensionStatus(t, report, dimRemediation, StatusFail)` and `requireDimensionStatus(t, report, dimRetryability, StatusFail)`.
       * Neither `dimRemediation` nor `dimRetryability` flags `ExpectedDivergence` (`d.ExpectedDivergence == false`) or sets `TrackedDefect` (`d.TrackedDefect == ""`), and no dimension in `report.Dimensions` has `StatusReportOnly`.
       * The union of `DivergentFields` across `StatusFail` dimensions equals `{"remediation", "retry_after_ms"}` via `sameFieldSet`.
       * `requireEvidenceStatus(t, report, faultline.StatusFail)`.
    2. Observe RED (`go test -count=1 -run 'TestU3SeedCorpus_KnownGatePayloadDrift|TestU3SeedCorpus_CorpusRegistryComplete' ./internal/faultline/parity` fails because `knownDrifts` in `comparator.go` still classifies the drift as `StatusReportOnly`).
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/faultline/parity/comparator.go`, empty `knownDrifts` (`var knownDrifts = map[string]driftDecl{}`) so divergences on `remediation` (`dimRemediation`) and `retry_after_ms` (`dimRetryability`) fail closed (`StatusFail`). Keep `const TrackedDefectGatePayload = "166-F"` exported for symbol compatibility with updated GoDoc noting `166-F` remediated the drift and `KnownGatePayloadDriftScenario` now fails closed.
    2. In `internal/faultline/parity/comparator.go`, expand `exitCodeClasses` to include MCP's concrete gate error-type tokens alongside the coarse simulation tokens:
       * Exit `6`: add `"gate_blocked": {}, "gate_requeued": {}, "gate_escalated": {}`
       * Exit `7`: add `"gate_config": {}, "gate_setup": {}`
       * Exit `8`: add `"gate_timeout": {}, "gate_in_progress": {}`
    3. In `internal/mcp/gate_errors.go`, export `GateErrorResult(err error, requestedStatus string) (*mcplib.CallToolResult, bool)` (with `gateErrorResult` delegating to `GateErrorResult`) so cross-surface parity tests in `internal/faultline/parity` can call the real MCP gate-error renderer directly.
* **Affected files**:
  * `internal/faultline/parity/comparator.go`
  * `internal/faultline/parity/scenarios.go`
  * `internal/mcp/gate_errors.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `internal/faultline/parity` before `knownDrifts` in `comparator.go` is emptied.
  * After `comparator.go` and `gate_errors.go` are updated, `KnownGatePayloadDriftScenario().Validate` enforces `StatusFail` on `dimRemediation` and `dimRetryability` and `faultline.StatusFail` on `report.Evidence`.
* **Execution posture**: test-first.

### U3: Add real cross-surface CLI/MCP gate-error parity regression tests and lifecycle-safe TrackedDefect resolution tests (`tests`) — `166.003-T`

* **Domain**: `tests`
* **Depends on**: U2 (`166.002-T`)
* **Changes**:
  * In `internal/faultline/parity/comparator_test.go`:
    1. Update `TestU2ComparatorExpectedGateDrift` to assert that `gateDriftResults(t)` (where CLI omits `remediation` and `retry_after_ms` emitted by MCP) fails closed: `findDimension(t, report, "remediation")` and `findDimension(t, report, "retryability")` both have `Status == parity.StatusFail`, `ExpectedDivergence == false`, `TrackedDefect == ""`, exact divergent field union `{"remediation", "retry_after_ms"}`, and `report.Evidence.Status == faultline.StatusFail`. Remove the unused unexported `const trackedGatePayload` so `golangci-lint` (`unused`) stays clean.
    2. Add `TestU2ComparatorRealGateErrorParityAllClasses` that invokes the real CLI renderer (`cli.RenderGateErrorJSON(ge.ItemID, ge)` and `cli.ExitCodeFor(cli.GateExitError(ge))`) and the real MCP renderer (`mcp.GateErrorResult(ge, "done")`) across all `*corerrors.GateError` classes (`setup`, `config`, `timeout` with `RetryAfterMs: 1500` and `0`, `in_progress` with `RetryAfterMs: 2000` and `0`), feeds their actual JSON bodies into `parity.CompareResults`, and asserts every applicable dimension (`exit_code`, `structured_error`, `retryability`, `remediation`, `force_lever`) passes (`parity.StatusPass`) and `report.Evidence.Status == faultline.StatusPass`. Also verify that injecting a mismatch on `remediation` or `retry_after_ms` into one surface's rendered body causes `parity.CompareResults` to fail closed (`StatusFail`).
  * In `internal/faultline/parity/scenarios_test.go`:
    1. Update `TestU3SeedCorpus_KnownGatePayloadDrift` to assert that `runScenario(t, parity.KnownGatePayloadDriftScenario())` produces `StatusFail` on `remediation` and `retryability`, zero `StatusReportOnly` dimensions, and `report.Evidence.Status == "fail"`.
    2. Update `TestU3SeedCorpus_TrackedDefectResolution_Valid` so it does not fail when `166-F` transitions to `done`/`archived` on shipment closure:
       * Create a hermetic `t.TempDir()` `.backlogit/queue/` and `.backlogit/archive/` fixture with a `queued` artifact (`166-F` or `001-F`) and a terminal (`done`) artifact (`002-F`), asserting `parity.ResolveTrackedDefect` returns `nil` for the non-terminal artifact and `parity.ErrTrackedDefectTerminal` for the terminal artifact.
       * Against the live workspace (`backlogitDirForResolution(t)`), iterate over `parity.SeedCorpus()` and assert that any dimension with `Status == parity.StatusReportOnly` resolves its `TrackedDefect` via `parity.ResolveTrackedDefect` (while confirming zero dimensions in `SeedCorpus()` remain `StatusReportOnly`).
  * In `internal/mcp/gate_errors_test.go`:
    1. Extend `TestGateErrorResult_ClassFamily` to assert exact `remediation` strings for `setup` and `config`, absence of `remediation` for `timeout` and `in_progress`, exact numeric `retry_after_ms` when `RetryAfterMs > 0`, and absence of `retry_after_ms` when `RetryAfterMs == 0` on `timeout` and `in_progress`.
* **Affected files**:
  * `internal/faultline/parity/comparator_test.go`
  * `internal/faultline/parity/scenarios_test.go`
  * `internal/mcp/gate_errors_test.go`
* **Acceptance criteria**:
  * `go test -count=1 -race ./internal/cli/... ./internal/mcp/... ./internal/faultline/...` passes with 0 failures.
  * `TestU2ComparatorRealGateErrorParityAllClasses` proves end-to-end field parity between `cli.RenderGateErrorJSON` and `mcp.GateErrorResult` across all four `GateError` classes.
  * `TestU3SeedCorpus_TrackedDefectResolution_Valid` remains green both while `166-F` is `queued`/`active` and after `166-F` is `done`/`archived`.
* **Execution posture**: test-first.

## Dependency Graph

* **Wave 1**: `166.001-T` (U1 — CLI `gateJSONPayload` + `renderGateErrorJSON` parity fields and unit tests)
* **Wave 2**: `166.002-T` (U2 — blocked by `166.001-T`; flip S4 comparator `knownDrifts` and `KnownGatePayloadDriftScenario` from `report_only` to fail-closed + export `mcp.GateErrorResult`)
* **Wave 3**: `166.003-T` (U3 — blocked by `166.002-T`; real cross-surface CLI/MCP gate-error parity regression tests + lifecycle-safe `ResolveTrackedDefect` tests)

No cycles exist in the DAG (`166.001-T -> 166.002-T -> 166.003-T`).

## Decisions and Rationale

* **Keep `remediation` synthesis at the CLI and MCP adapter boundaries**: Per S4 U2 (`docs/exec-plans/2026-09-03-s4-seq1-parity-harness-plan.md`), `internal/errors.GateError` intentionally carries class and retry metadata without human-facing remediation strings so `internal/errors` stays a minimal domain leaf. Populating `p.Remediation` and `p.RetryAfterMs` in `renderGateErrorJSON` (`internal/cli/gate_exit.go`) mirrors `gateClassResult` (`internal/mcp/gate_errors.go`) directly and satisfies the cross-surface parity contract without broadening into an error-framework redesign.
* **Include concrete MCP `gate_*` category tokens in `exitCodeClasses`**: Adding `"gate_blocked"`, `"gate_requeued"`, `"gate_escalated"` (exit 6), `"gate_config"`, `"gate_setup"` (exit 7), and `"gate_timeout"`, `"gate_in_progress"` (exit 8) to `exitCodeClasses` in `internal/faultline/parity/comparator.go` lets `parity.CompareResults` validate real `cli.RenderGateErrorJSON` and `mcp.GateErrorResult` outputs without breaking any existing simulation scenario.
* **Decouple `TestU3SeedCorpus_TrackedDefectResolution_Valid` from `166-F`'s live queue status**: Because `166-F` is the covering feature of shipment `206-S`, shipping `206-S` moves `166-F` to `done` and `archived`. Using a hermetic `t.TempDir()` `.backlogit` fixture to test `ResolveTrackedDefect`'s positive non-terminal path (and negative terminal path) prevents a post-merge failure on `main`.

## Risks and Caveats

* **Unused unexported constant in `comparator_test.go`**: Removing `StatusReportOnly` assertions from `TestU2ComparatorExpectedGateDrift` leaves `const trackedGatePayload = "166-F"` in `comparator_test.go` unused unless removed; U3 explicitly removes that unexported constant so `golangci-lint` passes.
* **Toolchain note on local `golangci-lint`**: Per `docs/closure/205-S-204-F-post-merge-closure.md`, the ambient local `golangci-lint` binary has a Go export-data toolchain mismatch; Ship should verify `gofmt -l`, `go vet ./...`, and targeted `go test -race` locally and rely on CI for `golangci-lint`.

## Constitution Check

* **I. Safety-First Go**: `pass` — all changes are idiomatic Go 1.24 with explicit error handling, typed struct tags, and zero `unsafe` usage.
* **II. Test-First Development (NON-NEGOTIABLE, P-002)**: `pass` — U1 and U2 specify explicit RED tests before production edits, and U3 completes the cross-surface regression suite. Closed `harness-exempt` set: `[]` (none).
* **III. Workspace Isolation and Security Boundaries**: `pass` — tests use `t.TempDir()` and in-memory buffers without mutating global state or paths outside the workspace.
* **IV. CLI Workspace Containment**: `pass` — no files are read or written outside `C:\Source\GitHub\backlogit`.
* **V. Structured Observability**: `pass` — CLI `--json` gate-error payloads now carry the same structured `remediation` and `retry_after_ms` fields as MCP.
* **VI. Single Responsibility**: `pass` — changes are cleanly separated across `internal/cli`, `internal/mcp`, and `internal/faultline/parity`.
* **VII. Destructive Command Approval**: `N/A` — no destructive commands or data migrations are involved.
* **VIII. Explicit Safety Modes for Elevated Risk**: `pass` — flipping `KnownGatePayloadDriftScenario` from `report_only` to `StatusFail` enforces fail-closed parity verification on `remediation` and `retry_after_ms`.
* **IX. Git-Friendly Persistence**: `pass` — all planning and backlog artifacts are plain UTF-8 Markdown with LF line endings.
* **X. Agent Context Efficiency**: `pass` — `remediation` and `retry_after_ms` give CLI `--json` agent callers immediate, synchronous recovery guidance matching MCP.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — shipment `206-S` merges via a merge commit (`--merge`), never squash or rebase (P-009).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — adds optional `remediation` and `retry_after_ms` fields to the machine-readable CLI `--json` gate-error payload (`gateJSONPayload`) and flips the S4 parity comparator (`knownDrifts`) from `report_only` to fail-closed (`StatusFail`).
* **security, auth, permission, or compliance-sensitive behavior**: `absent` — no authentication, authorization, filesystem permission, or secret-handling surface is touched.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — additive JSON fields with `omitempty`; no database or file migration.
* **external integration, operator checkpoint, or external dependency**: `absent` — pure in-repo Go changes across `internal/cli`, `internal/mcp`, and `internal/faultline/parity`.
* **high runtime, rollout, or rollback risk**: `absent` — low blast radius (6 files) and backward-compatible `omitempty` tags on all pass and blocked payloads.

Requires plan hardening: yes

## Plan Hardening

### 1. Risk Triggers and Protected Invariants

* **Invariant I1 (Exact CLI/MCP Gate-Error Field Parity)**: For every `*corerrors.GateError` class (`setup`, `config`, `timeout`, `in_progress`), `cli.RenderGateErrorJSON` and `mcp.GateErrorResult` MUST emit identical `remediation` strings (`setup` and `config` only; omitted on `timeout` and `in_progress`) and identical `retry_after_ms` integers (`timeout` and `in_progress` when `RetryAfterMs > 0`; omitted when `RetryAfterMs <= 0` and on `setup`/`config`).
* **Invariant I2 (Zero Regression on Pass and Blocked Payloads)**: `renderGatePassJSON` and `renderGateBlockedJSON` MUST NOT set `Remediation` or `RetryAfterMs`, and `omitempty` MUST omit both keys so existing CLI `--json` pass and blocked payloads remain byte-for-byte unchanged.
* **Invariant I3 (Fail-Closed Parity Enforcement)**: `knownDrifts` in `internal/faultline/parity/comparator.go` MUST NOT exempt `remediation` or `retry_after_ms`. Any divergence on either field between CLI `--json` and MCP MUST produce `StatusFail` on the dimension and `faultline.StatusFail` on the `EvidenceArtifact`.
* **Invariant I4 (Lifecycle-Safe Tracked-Defect Resolution Across `206-S` Closure)**: `TestU3SeedCorpus_TrackedDefectResolution_Valid` MUST NOT require `166-F` to remain non-terminal in the live `.backlogit/` workspace after `166-F` is remediated and archived by `backlogit_ship_shipment`.

### 2. Reinforced Instructions and Learnings Consulted

* `docs/compound/2026-09-08-parity-harness-design-patterns.md` — JSON `omitempty` vs presence semantics (`getIntPresent`, `getBoolPresent`, `getStringPresent`) and `ResolveTrackedDefect` non-terminal status enforcement.
* `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` — avoid freezing pre-remediation lifecycle state (`166-F` as `queued`) in tests that must remain green after shipment closure archives `166-F`.

### 3. Risky Action Classification (`strict-safety`)

* `ProposedAction`: Add `remediation,omitempty` and `retry_after_ms,omitempty` to `gateJSONPayload` in `internal/cli/gate_exit.go` (`U1`)
  * `ActionRisk`: `contract` (extends CLI `--json` error output for `*corerrors.GateError` to match MCP `gateClassResult`)
  * `Approval`: Pre-authorized by operator Dark Factory Phase 1 activation (`2026-10-30T05:55:05Z`, `166-F`)
  * `ActionResult`: `planned`
* `ProposedAction`: Empty `knownDrifts` in `internal/faultline/parity/comparator.go` and flip `KnownGatePayloadDriftScenario` to `StatusFail` (`U2`)
  * `ActionRisk`: `contract` (tightens cross-surface parity comparator from `report_only` to fail-closed on `remediation` and `retry_after_ms`)
  * `Approval`: Pre-authorized by operator Dark Factory Phase 1 activation (`2026-10-30T05:55:05Z`, `166-F`)
  * `ActionResult`: `planned`

### 4. Review-Gate Capability and Degraded-Mode Handling

* `plan-review` must record literal `dispatch_mode:` (`multi-agent-dispatch` or `single-agent-declared-degradation`) and `decision:` (`PASS` or `ADVISORY` with `operator_authorization: approved`) in `## Plan Review` before harvest.
* Propagated session degradation notes: `ROUTING_DEGRADED: context_tier` (`default` vs configured `long_context`) and `TOOL_DEGRADED: engram` (direct codebase `grep`/`view` fallback).

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit move <id> --status <status> --json` and `backlogit update <id> --status <status> --json` on `*corerrors.GateError` refusals (`internal/cli/gate_exit.go`).
  * Exported test-seam helpers `cli.RenderGateErrorJSON`, `cli.GateExitError`, and `mcp.GateErrorResult`.
* **Runtime verification expectations**:
  * Run `go test -count=1 -race ./internal/cli/... ./internal/mcp/... ./internal/faultline/...` and `go vet ./internal/cli/... ./internal/mcp/... ./internal/faultline/...`.
  * Verify `TestU2ComparatorRealGateErrorParityAllClasses` and `TestU3SeedCorpus_TrackedDefectResolution_Valid` pass with `-race`.
* **Operational closure expectations**:
  * Produce `docs/closure/206-S-166-F-post-merge-closure.md` conforming to the `{shipment_id}-{feature_id}-post-merge-closure.md` filename contract, with top-level `closure_status` and `compaction_status`.
  * Invoke `compact-context` (`target: all`) per P-020 before finalizing `compaction_status` and advancing the Dark Factory cursor to Phase 2 (`DE3E7B67`).
  * **Rollback trigger and procedure**: If CLI `--json` gate-error output or faultline parity tests regress in CI, revert the `206-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* ROUTING_DEGRADED: context_tier
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer (complete sequential rubric pass across all 6 applicable personas).
* plan_hardening: `Requires plan hardening: yes` confirmed and satisfied by `## Plan Hardening` (Invariants I1–I4, learnings trace, and `strict-safety` `ProposedAction` / `ActionRisk` classifications).
* constitution_check: `Constitution Check: pass` confirmed; all 11 constitutional principles mapped with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 2 (applied in place).
  * P3-1 (`Agent-Native Parity Reviewer`, U2 `internal/faultline/parity/comparator.go`): Ensure `exitCodeClasses` in `comparator.go` includes MCP's concrete `gate_blocked`, `gate_requeued`, `gate_escalated`, `gate_config`, `gate_setup`, `gate_timeout`, and `gate_in_progress` tokens so `TestU2ComparatorRealGateErrorParityAllClasses` exercises `CompareResults` against unmodified `mcp.GateErrorResult` output. Applied in place in U2.
  * P3-2 (`Learnings Researcher`, U3 `internal/faultline/parity/scenarios_test.go`): Transition `TestU3SeedCorpus_TrackedDefectResolution_Valid` to use a hermetic `t.TempDir()` `.backlogit` fixture for non-terminal/terminal resolution assertions so archiving `166-F` during `206-S` closure does not break `main`. Applied in place in U3.
* runtime_verification_and_closure: Targeted `-race` verification across `./internal/cli/...`, `./internal/mcp/...`, and `./internal/faultline/...` and post-merge closure artifact `docs/closure/206-S-166-F-post-merge-closure.md` with P-020 `compact-context` invocation are explicitly specified.
