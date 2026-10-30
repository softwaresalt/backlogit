---
chunk_strategy: h1-h2-h3
description: 'Deliberation for feature 166-F (Phase 1): add remediation and retry_after_ms to CLI --json gate-error payload to match MCP, add real cross-surface field-parity regression tests, and flip the S4 expected-divergence scenario from report-only to fail-closed.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/exec-plans/2026-10-30-gate-payload-parity-remediation-plan.md
    - .backlogit/queue/166-F.md
    - .backlogit/queue/206-S.md
  promoted_to: plan
  tags:
    - parity
    - cli
    - mcp
    - faultline
    - gate-error
    - 166-F
    - 206-S
  topic: 'Remediate CLI --json gate-error payload parity drift (remediation and retry_after_ms) and flip S4 scenario to fail-closed'
schema_version: "1.0"
source: docs/decisions/2026-10-30-gate-payload-parity-remediation-deliberation.md
title: 'Deliberation: Cross-surface gate-payload parity remediation (166-F)'
---

## Problem Frame

* Existing queued high-priority feature `166-F` (*Cross-surface gate-payload parity tracked defect (report-only owner)*, superseding archived `156.007-T`) is the standalone durable owner for the known S4 cross-surface drift between the CLI `--json` gate-error payload (`gateJSONPayload` / `renderGateErrorJSON` in `internal/cli/gate_exit.go`) and the MCP gate-error payload (`gateClassResult` in `internal/mcp/gate_errors.go`).
* On MCP, `gateClassResult(ge *corerrors.GateError)` (`internal/mcp/gate_errors.go:77-105`) emits:
  * `remediation`: `"Install or repair the autoharness binary (min 1.4.7), or disable the gate for this workspace."` when `ge.Class == "setup"`, and `"Fix the gate configuration or base/HEAD ref, then retry the completion."` when `ge.Class == "config"`.
  * `retry_after_ms`: `ge.RetryAfterMs` when `(ge.Class == "timeout" || ge.Class == "in_progress") && ge.RetryAfterMs > 0`.
* On the CLI under `--json` (`backlogit move --json` and `backlogit update --json` via `moveGateError`), `renderGateErrorJSON(id string, ge *corerrors.GateError)` (`internal/cli/gate_exit.go:166-178`) marshals `gateJSONPayload{ID: id, Outcome: "error", Error: ge.Error(), Retryable: ge.Retryable()}`, omitting both `remediation` and `retry_after_ms`.
* Because of this omission, the S4 parity comparator (`internal/faultline/parity/comparator.go`: `knownDrifts`) and recurring-failure corpus (`internal/faultline/parity/scenarios.go`: `KnownGatePayloadDriftScenario`) exempt `{remediation}` on `dimRemediation` and `{retry_after_ms}` on `dimRetryability` as `StatusReportOnly` pinned to `TrackedDefectGatePayload = "166-F"`.
* Operator Phase 1 directive (`2026-10-30T05:55:05Z`, Dark Factory mode): *"Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode."* Scope for Phase 1 is strictly bounded to `166-F`: add `remediation` and `retry_after_ms` to the CLI `--json` gate-error payload to match MCP, add real cross-surface field-parity regression tests, and flip the S4 expected-divergence scenario from `report-only` to `fail-closed` together with its scenario-owner lifecycle transition.

## Research Findings

* **Prior learnings (`docs/compound/2026-09-08-parity-harness-design-patterns.md`)**:
  * Learning #1: `getBoolPresent` and `getIntPresent` in `internal/faultline/parity/comparator.go` distinguish key absence (`omitempty`) from JSON `null` or wrong-typed values. `gateJSONPayload` uses `omitempty` on optional fields so zero values are omitted rather than emitted as zero/empty.
  * Learning #3 & #8: `ResolveTrackedDefect(backlogitDir, id)` (`internal/faultline/parity/scenarios.go:104-116`) walks `.backlogit/` and rejects any artifact whose status is terminal (`done`, `archived`, `shipped`, `abandoned`). In `internal/faultline/parity/scenarios_test.go:110-119`, `TestU3SeedCorpus_TrackedDefectResolution_Valid` currently asserts `require.NoError(t, parity.ResolveTrackedDefect(dir, "166-F"))` against the live repository `.backlogit/` directory. When `166-F` completes and is archived by `backlogit_ship_shipment` at the end of shipment `206-S`, `166-F` becomes terminal (`archived`), which would break `TestU3SeedCorpus_TrackedDefectResolution_Valid` on `main` unless the test is transitioned alongside flipping `KnownGatePayloadDriftScenario` from `report_only` to `fail`.
* **Comparator exit-class equivalence (`internal/faultline/parity/comparator.go:160-166`)**:
  * `exitCodeClasses` currently lists coarse simulation tokens (`"blocked"`, `"config"`, `"setup"`, `"retryable"`, `"timeout"`, `"in_progress"`, `"governance"`). Real MCP `gateBlockedErrorType` and `gateClassErrorType` (`internal/mcp/gate_errors.go:107-134`) emit `"gate_blocked"`, `"gate_requeued"`, `"gate_escalated"`, `"gate_setup"`, `"gate_config"`, `"gate_timeout"`, and `"gate_in_progress"`. Including those concrete MCP error-type tokens in `exitCodeClasses` (buckets `6`, `7`, and `8`) allows real cross-surface parity tests to feed the actual outputs of `internal/cli` and `internal/mcp` directly into `parity.CompareResults` while preserving all existing simulation tests.

## Options Evaluated

### Option A: Targeted `gateJSONPayload` Field Parity + Real Renderer Cross-Surface Tests + Fail-Closed S4 Transition — Chosen

* **Description**:
  1. Add `Remediation string `json:"remediation,omitempty"`` and `RetryAfterMs int `json:"retry_after_ms,omitempty"`` to `gateJSONPayload` in `internal/cli/gate_exit.go`, and populate them in `renderGateErrorJSON` using the exact class switch in MCP's `gateClassResult` (`"setup"`, `"config"`, `"timeout"`, `"in_progress"`). Export thin wrappers `RenderGateErrorJSON` and `GateExitError` in `internal/cli/gate_exit.go` and `GateErrorResult` in `internal/mcp/gate_errors.go`.
  2. Empty `knownDrifts` in `internal/faultline/parity/comparator.go` so `remediation` and `retry_after_ms` divergences fail closed (`StatusFail`), flip `KnownGatePayloadDriftScenario` in `internal/faultline/parity/scenarios.go` and existing expectations in `TestU2ComparatorExpectedGateDrift` (`comparator_test.go`) and `TestU3SeedCorpus_KnownGatePayloadDrift` (`scenarios_test.go`) from `StatusReportOnly` to `StatusFail`, make `TestU3SeedCorpus_TrackedDefectResolution_Valid` (`scenarios_test.go`) lifecycle-safe via a hermetic `t.TempDir()` `.backlogit` fixture so archiving `166-F` on shipment closure keeps the test suite green, and export `GateErrorResult` in `internal/mcp/gate_errors.go`.
  3. Add a real cross-surface parity test (`TestU2ComparatorRealGateErrorParityAllClasses` in `internal/faultline/parity/comparator_test.go`) driving `cli.RenderGateErrorJSON` and `mcp.GateErrorResult` through `parity.CompareResults` across all `GateError` classes (`setup`, `config`, `timeout`, `in_progress` with `RetryAfterMs > 0` and `0`), extend `TestGateErrorResult_ClassFamily` in `internal/mcp/gate_errors_test.go`, and expand `exitCodeClasses` in `internal/faultline/parity/comparator.go` with MCP's concrete `gate_*` category tokens so the new cross-surface test turns from RED to GREEN.
* **Pros**:
  * Strictly scoped to `166-F` with zero unrelated error-framework churn.
  * Preserves all existing CLI exit codes (`6`, `7`, `8`, `9`), JSON field names, and `omitempty` behavior on pass/blocked/error payloads.
  * Eliminates the post-ship `ResolveTrackedDefect(liveDir, "166-F")` trap when `166-F` transitions to `done`/`archived`.
* **Cons**:
  * Keeps `remediation` synthesis at the CLI and MCP adapter boundaries rather than refactoring `internal/errors.GateError` (which S4 U2 explicitly documented as intentional so `internal/errors` remains a minimal leaf).
* **Effort estimate**: Low (3 atomic tasks).

### Option B: Broad CLI/MCP Error Envelope Unification Across All Commands

* **Description**:
  * Redesign all CLI and MCP error responses (`domainError`, `gateJSONPayload`, `ExitError`) into a single shared envelope across all commands.
* **Pros**:
  * Uniform schema across gate and non-gate errors.
* **Cons**:
  * Violates the explicit Phase 1 scope boundary (*"Scope tightly to that exact contract: preserve existing codes/types/messages/optional-field behavior... Do NOT broaden into generic error-framework redesign or unrelated mappings"*).
  * Breaks existing callers and golden tests expecting the established `gateJSONPayload` contract.
* **Effort estimate**: High.

## Trade-off Comparison

| Criterion | Option A (Targeted `166-F` Remediation + S4 Fail-Closed Flip) | Option B (Broad Error Envelope Redesign) |
|---|---|---|
| Matches `166-F` contract | Exact match | Out of scope (P-010 / scope boundary violation) |
| Backwards compatibility | Preserves all existing codes, types, messages, and `omitempty` behavior | Breaks existing CLI `--json` and MCP callers |
| Lifecycle safety when `166-F` ships | Handles `ResolveTrackedDefect` transition cleanly | Unaddressed |
| Blast radius | 6 files across `internal/cli`, `internal/mcp`, and `internal/faultline/parity` | Repo-wide |

## Decision

Adopt **Option A** and promote directly to implementation plan `docs/exec-plans/2026-10-30-gate-payload-parity-remediation-plan.md` under existing feature `166-F` and shipment `206-S`.

## Rejected Alternatives

* **Option B (Broad Error Envelope Unification)**: Rejected because it violates the operator's explicit scope boundary for Phase 1 (`166-F`) and introduces unnecessary breaking changes across non-gate error paths.

## Unresolved Questions

* None. All affected call sites and lifecycle transitions are identified and bounded.

## Risks and Mitigations

* **Risk 1 (`ResolveTrackedDefect` live-workspace coupling)**: `TestU3SeedCorpus_TrackedDefectResolution_Valid` currently calls `ResolveTrackedDefect(dir, "166-F")` against the live `.backlogit` directory, which would fail with `ErrTrackedDefectTerminal` as soon as `166-F` is marked `done`/`archived` on shipment closure.
  * **Mitigation**: Transition `TestU3SeedCorpus_TrackedDefectResolution_Valid` in `internal/faultline/parity/scenarios_test.go` to validate `ResolveTrackedDefect` against a hermetic `t.TempDir()` `.backlogit` fixture (covering both non-terminal `queued` acceptance and terminal `done` rejection) and assert that any `StatusReportOnly` dimension in `SeedCorpus()` (now 0) resolves against the live workspace.
* **Risk 2 (`omitempty` regression on pass/blocked/config payloads)**: Adding `Remediation` and `RetryAfterMs` without `omitempty` or populating `RetryAfterMs` when `<= 0` would emit spurious keys.
  * **Mitigation**: Tag both fields `omitempty` on `gateJSONPayload` and guard `p.RetryAfterMs = ge.RetryAfterMs` with `if ge.RetryAfterMs > 0` under `case "timeout", "in_progress":`, matching `internal/mcp/gate_errors.go:101-103`.
