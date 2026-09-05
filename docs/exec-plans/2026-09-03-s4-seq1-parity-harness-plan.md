---
chunk_strategy: h1-h2-h3
description: "Execution plan for S4: Sequence 1/7 cross-surface golden parity harness"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s4-seq1-parity-harness-plan.md
title: "S4 Execution Plan — Sequence 1/7 Cross-Surface Golden Parity Harness"
---

# S4 Execution Plan — Cross-Surface Golden Parity Harness (Seq 1/7)

**Covering feature**: Cross-surface golden parity harness
**Stash member**: 5A4DBE3C (critical)
**Tier**: feature (shipment sequence S4) — program foundation

## Problem Frame

Transport and contract divergence between CLI, MCP, and internal/event APIs
recurs (typed errors collapsing to exit 1, domainError mapping drift, retryability
disagreement, arrays lost through omitempty) and is caught late. Build a
generated, table-driven harness that runs one governed scenario through all three
surfaces and compares response shape, error category, exit code, retryability,
remediation guidance, serialization guarantees, and durable side effects.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | scenario declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | harness runs against isolated temp workspaces |
| IV. CLI Containment | n/a |
| V. Observability | comparison report is structured |
| VI. Single Responsibility | one comparator, table-driven scenarios |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed on any surface mismatch |
| IX. Git-Friendly | golden fixtures are text |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Parallel-safe three-surface scenario driver
* Scope: define the governed-scenario table schema and a driver that executes one scenario through the CLI, MCP, and internal/event surfaces against a per-scenario ISOLATED temp workspace, using PARALLEL-SAFE injected runners — NO process-global mutation:
  - CLI: construct a FRESH root command per scenario and use `cmd.SetArgs([...])` + `cmd.SetOut`/`SetErr(buf)` (the existing test seam); capture the returned `error` and derive the CLI exit code via `internal/cli.ExitCodeFor(err)` (the SAME mapping `cmd/backlogit/main.go` uses via `os.Exit(cli.ExitCodeFor(err))`) — do NOT read or mutate `os.Args`/`os.Stdout`. A subprocess runner is an optional alternative only when real-process fidelity beyond `ExitCodeFor` is required.
  - MCP: `internal/mcp.Server.InvokeTool(ctx, name, request)` in-process.
  - internal/event: the corresponding `internal/core` / `internal/events` call in-process.
  Every surface call takes a `context.Context`, but the cancellation contract is NARROWED to the parity DRIVER's own goroutines and teardown: the harness does NOT claim to cancel in-flight PRODUCTION command work, because the gated CLI paths call `context.Background()` directly (`internal/cli/move.go:55`, `internal/cli/update.go:92`) so a cancelled cmd-context cannot stop them — threading `cmd.Context()` through those production commands is a SEPARATE production-migration task explicitly OUT of this harness-only scope. Each scenario gives EACH surface its OWN root CLONED from the same seed fixture — a MUTATING scenario CANNOT share one physical workspace (sequential calls would observe the prior surface's mutation; concurrent calls would contend on the same Markdown/SQLite/event state, so a difference would reflect execution order/locks, not surface parity): CLI runs with `--cwd <cliRoot>`, MCP with `internal/mcp.NewServerForRoot(mcpRoot)`, internal with `core.NewWorkspace(internalRoot)` — the three roots are INDEPENDENT clones of the identical seed, and the comparator (U2) compares their POST-states. Process-global bans (grounded in `internal/cli`): the driver MUST NOT pass `--log-level` (its `PersistentPreRunE` calls `applyLogLevel` → `slog.SetDefault`, a global logger swap) and MUST NOT configure via environment (`BACKLOGIT_LOG_LEVEL`/`BACKLOGIT_LOG_FORMAT`; `t.Setenv` panics under `t.Parallel()`) — ALL config flows through `SetArgs`; NO `os.Args`/`os.Stdout`/`os.Chdir`/`os.Setenv` and NO `slog.SetDefault`. Set `--no-update-check` (and inject `Server.LatestVersionLookup`) to stay hermetic. Teardown ordering: cancel the context and JOIN only the goroutines the parity DRIVER launched (the `Event`/`Telemetry`/`HookEvent` writers are synchronous mutex-protected and expose no goroutines/`Close`), then call `Workspace.Close` on EACH per-surface workspace (including the MCP server's lazily-initialized workspace) — `Workspace.Close` drains webhook dispatches and closes SQLite — before the temp-dirs are removed (register `Workspace.Close` AFTER temp-dir removal so LIFO closes handles first — Windows handle + goroutine-leak safety).
* Acceptance: a single seed scenario runs through all three surfaces (each on its own cloned root) in a parallel-safe way and captures response shape, error category, exit code (via `ExitCodeFor`), retryability, remediation guidance, serialization, and durable post-state; a `go test -race` run over the parallel scenarios is clean with NO `os.Args`/`os.Stdout`/`os.Chdir`/`os.Setenv` use and NO `slog.SetDefault` mutation; `Workspace.Close` (drains webhooks + SQLite) runs before temp-dir removal.
* Files: `internal/faultline/parity` harness (+ driver + tests). Single responsibility (parallel-safe driver).

### U2 — Dimension-aware cross-surface comparator + divergence report
* Scope: compare the three captures DIMENSION-BY-DIMENSION using APPLICABILITY / STATUS markers, NOT literal cross-surface equality. Each dimension carries a per-surface applicability marker (`applicable` / `not_applicable`) and a status; the comparator compares a dimension across surfaces ONLY where it is applicable on each:
  - exit code → CLI-only CAPTURE, but SEMANTICALLY cross-checked: the versioned gate exit codes (`6=blocked, 7=config, 8=retryable, 9=governance`) must agree with the applicable-to-all category/retryability (exit 8 ⇒ retryable=true ⇒ MCP retryable=true; exit 6/7/9 ⇒ the matching MCP error category);
  - RAW `mcplib` tool-result envelope / `IsError` wrapper → MCP-only. But STRUCTURED error data is applicable to BOTH CLI `--json` (the `gateJSONPayload`: error, retryable, remediation, `retry_after_ms`, repeated_failure, old/new_status, gate_report_hash) AND MCP — field-level parity asserted across the two (a concrete known drift: MCP gate errors carry `remediation` + `retry_after_ms` the CLI `--json` gate-error payload omits);
  - durable side effects / post-state (Markdown + SQLite + event stream) → applicable to ALL MUTATING surfaces: capture each surface's ISOLATED post-state (on its own cloned root) and compare them, so a CLI or MCP path that returns the right response but fails to persist the expected Markdown/DB/event change is detected (durable side effects are a cross-surface contract per the Problem Frame, NOT internal-only); `not_applicable` only for genuinely non-persisting (read-only) scenarios;
  - response shape → applicable to all three, but the success-path shape differs BY DESIGN (CLI flat `gateJSONPayload` vs MCP nested `gate` envelope) → compared via a documented field-mapping/normalization, not literal equality;
  - error category / retryability / remediation → applicable to all three. Retryability extraction per surface with explicit `absent == false` normalization (CLI `retryable,omitempty` omits the key on non-retryable; MCP always emits `false`) — semantic parity asserted here while the omitempty-vs-always serialization drift is caught by the SEPARATE serialization dimension, not masked; include the quantitative `retry_after_ms` backoff hint as an applicable-to-all sub-field;
  - serialization → applicable to all three (omitempty vs always-emit, null-vs-empty-array): captured explicitly so a serialization drift is reported here rather than masked by the semantic dimensions;
  - force-lever / destructive-approval capability → applicable to all three: any bypass/force affordance MUST be symmetrically present-or-absent and identically gated across surfaces (a surface-specific escape hatch is a divergence).
  Explicit comparison semantics: a mismatch on an APPLICABLE dimension fails closed; a `not_applicable` dimension is recorded but NEVER compared as a false divergence (so a CLI-only exit code is not a false MCP/internal mismatch). The field-level parity is the DIMENSION the harness asserts; a field that currently DIVERGES across surfaces (e.g. CLI `--json` omitting `remediation`/`retry_after_ms`) is NOT left as a failing (red) test — it is captured as an EXPECTED-DIVERGENCE scenario that asserts the divergence exists AND names the tracked defect, so the assertion is GREEN (report-only) while the drift stays visible; a NEW, un-expected divergence still fails closed. This keeps the seed corpus green at closure without requiring a production remediation of the CLI payload (which is OUT of this harness-only scope) and without silencing genuine new drift. Emit a deterministic divergence report AND a parity `EvidenceArtifact` conforming to the U4a contract.
* Acceptance: identical behavior passes; an injected divergence in each APPLICABLE dimension is detected and reported; a gate scenario asserts field-level parity of STRUCTURED error data (remediation, `retry_after_ms`, repeated_failure, old/new_status) across CLI `--json` and MCP; exit 8 ⇔ retryable ⇔ MCP retryable and exit 6/7/9 ⇔ the matching MCP category; a force/bypass affordance present on one surface but not the other is flagged; the success-path flat-vs-nested shape difference does NOT false-diverge while genuine field drift is still caught; a CLI-only exit-code difference does NOT produce a false cross-surface divergence for MCP/internal; durable POST-STATE (Markdown/DB/events) is captured and compared for EVERY mutating surface (a surface that returns correctly but under-persists is flagged); the emitted evidence artifact validates against the U4a contract. Depends on U1 and U4a-BEHAVIOR (`156.006-T`) — U2 both emits and VALIDATES the artifact, so it needs the `Canonical`/`Validate`/`DecodeAndValidate` behavior, not merely the declarations.
* Files: harness comparator (+ tests). Single responsibility (dimension-aware comparator).

### U3 — Seed the recurring-failure corpus
* Scope: seed scenarios for the known recurring failures (typed error → exit 1 collapse, domainError mapping drift, retryability disagreement, omitempty array loss).
* Acceptance: each seeded scenario is present and GREEN against current code; a scenario that encodes a KNOWN cross-surface drift (e.g. the CLI `--json` gate payload omitting `remediation`/`retry_after_ms`) is green by asserting the EXPECTED divergence AND naming its tracked defect (report-only), NOT by being left red; any genuinely NEW/undocumented red fails closed and must be triaged to an existing tracked defect (never left as an undocumented failing test). Depends on U2.

### U4a — Standalone versioned fault-line evidence-artifact contract (program foundation)
* Owner / dependency direction (grounded): a NEW STANDALONE foundation package `internal/faultline` (the program namespace; e.g. `internal/faultline/evidence.go`) owns the versioned `EvidenceArtifact` schema. It is NOT owned by the S4 parity harness (a test-infra consumer) and NOT by any single detector. It is a LEAF whose ONLY intentional internal dependencies are `internal/canonical` (the shared canonical serializer, below) and — if strictly needed — `internal/errors`; it imports no detector/harness/`core`/`events` package. ALL of S4 (the harness emits against it), S5-S9 (producers), and S10/S11 (consumers) import IT — dependency flows foundation ← producers/consumers, never the reverse. It is DISTINCT from `internal/gateevidence` (shipment-gate evidence, coupled to `internal/events`/`internal/core`); the fault-line contract MUST NOT be coupled to shipment-gate internals.
* Scope: define `EvidenceArtifact{ SchemaVersion int; ProducingTask string; ProducingCommit string; NodeFamily string; Applicability []string; ValidatorIdentity string; VerifiedEvidence VerifiedEvidencePayload; Status string }` where `VerifiedEvidencePayload` is a CONCRETE typed struct (NOT `any`/`json.RawMessage`) with a FULLY SPECIFIED shape — `VerifiedEvidencePayload{ ScenarioID string `json:"scenario_id"`; Dimension string `json:"dimension"`; Surfaces []string `json:"surfaces"`; DivergentFields []string `json:"divergent_fields"`; ExpectedDivergence bool `json:"expected_divergence"`; TrackedDefect string `json:"tracked_defect"`; Detail string `json:"detail"` }` (all JSON tags snake_case; `Surfaces` and `DivergentFields` normalized to non-nil empty slices per the always-array contract; `TrackedDefect` empty-string-valid only when `ExpectedDivergence` is false) — that participates in the golden fixture and `Canonical()` ordering — so a payload-shape change IS a schema change; with:
  - `schema_version` = exported const `EvidenceSchemaVersion = 1`, plus a `knownVersions` set;
  - CANONICAL deterministic serialization — a `Canonical() ([]byte, error)` implemented as a THIN WRAPPER over the existing stdlib-only leaf `internal/canonical.Canonicalize` (do NOT invent a second serializer — `internal/canonical` is ALREADY the evidence-authenticity serializer used by `gateproof`/`core.gate_evidence`; a parallel one would drift from S10 authenticity hashing). Provide a `canonicalMap()` step (mirroring `gateproof.canonicalMap()`) that converts the `EvidenceArtifact` struct to a `map[string]any` with integers-only numbers (no float/timestamp; integer-exact per `gateevidence.asInt`) and non-nil collections BEFORE calling `Canonicalize`, so the wrapper is byte-stable;
  - UNKNOWN-VERSION rejection on decode — a two-phase decode peeks `schema_version` before full unmarshal; a version outside `knownVersions` returns a TYPED error; decode uses `DisallowUnknownFields` so an unstamped additive drift is REJECTED, not tolerated;
  - NON-NIL collections — slices/maps normalized to non-nil empty on marshal (always-array/object, per compound `2026-07-21-omitempty-defeats-arrays-always-json-contract.md`);
  - a documented COMPATIBILITY / SKEW policy — ANY field add/remove/rename increments `EvidenceSchemaVersion`; producers stamp the current version; forward-incompatible artifacts are rejected, not guessed; a NEW golden fixture is added per version (old goldens are never mutated). At N=1 there is NO predecessor: only version 1 is accepted and version 0/absent is rejected. The `{N, N-1}` acceptance window is introduced WITH version 2 and served by VERSION-SPECIFIC decode shapes (a single struct + `DisallowUnknownFields` cannot accept an N-1 that renamed/removed fields), each with its own golden;
  - a ROLLOUT / report-only posture — see U4b for the FOUR distinct consumer outcomes (absent / present-known-valid / present-unknown-or-malformed / present-known-but-validation-fails).
* Acceptance: the contract is defined and versioned; `Canonical()` (wrapping `internal/canonical`) is byte-stable, asserted against a CHECKED-IN raw-bytes GOLDEN fixture (so any field/shape/tag change to `EvidenceArtifact`/`VerifiedEvidencePayload` breaks the golden — avoiding the round-trip `DeepEqual` false-green of `2026-07-21-omitempty-defeats-arrays`); empty collections are asserted present-and-array on the raw JSON (`.([]any)`), NOT via struct↔canonical `DeepEqual`; an unknown or unstamped-additive version fails closed with a typed error; the S4 harness (U2) emits against it. Independent foundation unit. Single responsibility (the versioned contract package).
* Harness-architect decomposition (declaration-first per `.github/skills/harness-architect/SKILL.md`): because the new exported declarations (`EvidenceArtifact`, `Canonical`, `DecodeAndValidate`, `Validate`) carry behavior, this splits into TWO backlog tasks — `156.004-T` = the DECLARATIONS + a source-shape harness (types + API signatures + red shape harness), and `156.006-T` = the BEHAVIOR (canonicalMap/`Canonical` wrapper, two-phase decode, `DisallowUnknownFields`, golden fixture, conformance). U4b's `Validate`/`DecodeAndValidate` consumers depend on `156.006-T`.
* Files: `internal/faultline/evidence.go` (+ conformance test).

### U4b — Bind S5-S8 (and S9) producer obligations
* Scope: WITHOUT implementing any later shipment, encode the FUTURE producer AND consumer obligations symmetrically:
  - Producers: document that each of S5 (157-F), S6 (158-F), S7 (159-F), S8 (160-F) — and S9 (161-F) as its red-test-honesty evidence applies — MUST emit an `EvidenceArtifact` conforming to U4a; record BACKLOG dependency edges (each producer feature → U4a `156.004-T`).
  - Consumers (symmetric): S10 (162-F) and S11 (163-F) also depend on U4a (`156.004-T`) and use the SAME conformance helper on the decode/accept path; a consumer-side conformance test asserts the FOUR-outcome behavior. Consumers MUST additionally authenticate provenance (producer/task/commit) and enforce anti-replay BEFORE applicability filtering — schema `DecodeAndValidate` proves SHAPE only, not authenticity (authenticate-before-filter; the S9 plan:119 / S10 plan:116 retain this as a blocking requirement). This authenticity + anti-replay verification is recorded here as a binding S10/S11 consumer obligation, not implemented in this unit.
  - The shared conformance helper is PRODUCTION code in `internal/faultline` (NO `testing` import — S10/S11 import it): a `DecodeAndValidate([]byte) (EvidenceArtifact, error)` decode-path helper that enforces `DisallowUnknownFields` + a trailing-data (`io.EOF`) check + version/shape validation (a bare `Validate(EvidenceArtifact)` on an ALREADY-decoded struct cannot enforce unknown-field rejection), plus a `Validate(EvidenceArtifact) error` for post-construction checks.
  - Distinct consumer outcomes (report-only must not silence real drift): (1) producer ABSENT = tolerated silently (benign, expected); (2) PRESENT, known-version, `DecodeAndValidate` succeeds = validated (blocking once promoted); (3) PRESENT but unknown/malformed version = reported LOUDLY; (4) PRESENT, known-version, but `DecodeAndValidate`/`Validate` FAILS = reported LOUDLY (real drift, same as unknown). Outcomes (3)/(4) are surfaced in the divergence report even while non-blocking — with a documented promotion path from report-only to fail-closed once all producers conform.
  No S5-S11 production code is written here.
* Acceptance: each of S5-S8 (+S9) has a documented producer obligation + a conformance-test hook referencing U4a; S10/S11 have a documented CONSUMER obligation + dependency edge to `156.004-T` + the authenticity/anti-replay obligation; the production `Validate()`/`DecodeAndValidate()` helpers exist (no `testing` import); a consumer conformance test asserts ALL FOUR outcomes (absent / known-valid / unknown / known-but-`Validate`-fails) match the skew policy; obligations are REPORT-ONLY but a present-but-unknown-version OR known-but-validation-failing artifact is surfaced, not silenced. Depends on U4a. Single responsibility (producer/consumer obligation binding).

## Dependency Graph

U4a-declarations (`internal/faultline` evidence contract types + shape harness)
is the independent LEAF foundation; U4a-behavior implements it. U1 (parallel-safe
driver) is independent; U2 depends on U1 + U4a-BEHAVIOR (it EMITS AND VALIDATES the
parity evidence artifact against the contract, so it needs `Canonical`/`Validate`/
`DecodeAndValidate` behavior, not merely the declarations); U3 depends on U2; U4b
depends on U4a-behavior (it uses `Validate`/`DecodeAndValidate`). Order: U4a-declarations
and U1 in parallel → U4a-behavior → U2 → U3; U4b after U4a-behavior. Backlog mapping:
`156.001-T` (U1), `156.002-T` (U2, depends `156.001-T` + `156.006-T`, which
transitively depends on `156.004-T`), `156.003-T` (U3, depends
`156.002-T`), `156.004-T` (U4a declarations + shape harness), `156.006-T`
(U4a behavior, depends `156.004-T`), `156.005-T` (U4b, depends `156.006-T`).
Single skill domain (Go test infrastructure / schema) per unit.

## Runtime Verification and Closure

Harness is itself a verification surface; closure = the harness runs in CI and
the seed corpus is green. No production runtime change.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Define the program-wide versioned fault-line evidence contract | High — S5-S11 depend on this schema and skew can invalidate later gates | Own the contract in the STANDALONE leaf `internal/faultline` (distinct from `internal/gateevidence`); it imports only stdlib and is imported BY producers/consumers (foundation ← producers/consumers). Include an `EvidenceSchemaVersion` const, a byte-stable `Canonical()`, typed unknown-version rejection, non-nil collections, and a documented compatibility/skew policy; ship report-only until producers conform |
| Capture CLI, MCP, and internal/event dimensions in one harness | Medium — literal equality is impossible for CLI-only dimensions such as exit code | Dimension-aware captures with per-surface `applicable`/`not_applicable` markers: exit code is CLI-only, tool-result envelope MCP-only, durable side effects (Markdown/SQLite/events) applicable to ALL MUTATING surfaces (CLI + MCP + internal) — each captured on its OWN isolated cloned root and compared so an under-persisting CLI or MCP path is caught (NOT internal-only); compare a dimension only where applicable on each surface, never emitting a false divergence for a `not_applicable` dimension |
| Execute CLI/MCP/internal scenarios from tests | Medium — process globals such as `os.Args` and `os.Stdout` are not parallel-test-safe | Use the injected in-process runners (fresh cobra command + `SetArgs`/`SetOut` + `internal/cli.ExitCodeFor`; `internal/mcp.Server.InvokeTool`; direct `internal/core`/`events` call) — NO `os.Args`/`os.Stdout` mutation; pass `context.Context` for DRIVER-OWNED goroutine cancellation + teardown (NOT production command cancellation — gated CLI paths use `context.Background()`, so threading `cmd.Context()` is a separate out-of-scope production migration) + `t.Cleanup` for teardown; `go test -race`-clean under `t.Parallel()`. Subprocess only when real-process fidelity beyond `ExitCodeFor` is required |
| Bind S5-S8 as future evidence producers | High — consumers may assume producers exist before their plans actually emit artifacts | Document S5-S8 (+S9) producer obligations + a shared conformance-test helper; encode BACKLOG dependency edges (each producer feature → U4a `156.004-T`); keep report-only until each producer conforms |

Rollback: keep the contract in report-only/conformance-test use until all
producer obligations are implemented. Compatibility: versioned artifacts must be
readable by S10/S11 only when the schema version is known. Ownership: the shared
foundation package owns the schema; detector plans are producers, not owners.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — shared versioned evidence-artifact contract consumed by S5-S11.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: PRESENT-minor — broad harness-gate rollout and producer/consumer skew risk.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6` / `grok-4.6`)
* Architecture Strategist (`grok-4.6`)
* Learnings Researcher over `docs/compound/`
* Concurrency Reviewer (triggered by the parallel three-surface driver) — concurrency-PASS
* Schema-CLI-Docs Coupling Reviewer (triggered by the versioned evidence contract) — PASS after remediation
* Agent-Native Parity Reviewer (triggered by CLI/MCP structured-error parity) — PASS after remediation
* Security Lens Reviewer — not risk-triggered (test-infra + schema; no attacker-controlled data)

Review history (genuine multi-agent-dispatch, cross-model, 2 cycles):
* Attempt 1 (FAIL): the mandatory adversarial pass resolved the four original controlling P1s (hardening signal; standalone `internal/faultline` owner with correct dependency direction; dimension-aware applicability; parallel-safe seam), but three specialized lenses surfaced NEW blocking P1s — Schema-CLI-Docs (round-trip `DeepEqual` false-green; untyped `VerifiedEvidence`; no version-bump/`DisallowUnknownFields` policy; asymmetric producer-only binding), Agent-Native Parity (structured error data wrongly MCP-only when CLI `--json` also emits it — real `remediation`/`retry_after_ms` drift), and Concurrency (`--log-level`→`slog.SetDefault` + env globals; cleanup ordering).
* Attempt 2 (PASS/ADVISORY): all schema P1s resolved (`Canonical()` wraps `internal/canonical` via `canonicalMap()`; checked-in raw-bytes golden + present-and-array asserts; concrete `VerifiedEvidencePayload` + `DisallowUnknownFields`/`DecodeAndValidate`; version-bump + `knownVersions` + N=1/`{N,N-1}` skew); agent-native P1 resolved (structured data cross-surface with field-level parity; retryability `absent==false`; exit↔category/retryable; force-lever + flat/nested + serialization dimensions); concurrency P1s resolved (process-global bans, fresh per-scenario `NewServerForRoot`, goroutine-join + close-before-remove teardown, `-race`-clean). Ten-reviewer manifest; no corroborated new HIGH P1.

Residual (non-blocking, folded into unit wording): `canonicalMap` integer-only conversion; version-specific decode shapes for the `{N,N-1}` window; `NewServerForRoot` MCP binding; goroutine join before close; `DecodeAndValidate` decode-path; fourth consumer outcome (present + known + `Validate`-fail).

Readiness: READY. Same-contract program-foundation completion; no scope creep. Ship may claim 138-S.
