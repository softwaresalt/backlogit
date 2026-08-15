---
title: "Optimize backlogit MCP startup by canonical artifact indexing"
description: "Implementation plan for engram workspace MCP startup timeout caused by O(link-sources × artifact-scan) migration behavior"
source: ".backlogit/archive/056-DL.md"
doc_type: plan
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Source

* Deliberation: `.backlogit/archive/056-DL.md` (FDA30F35)
* Stash evidence: engram startup ~91s vs autoharness ~20s with same root MCP config

## Problem Frame

`core.NewWorkspace` performs `MigrateDBOnlyLinks` before MCP initialize returns.
In large workspaces this migration repeatedly scans artifacts via `findArtifact`
per link source, which pushes startup beyond Copilot's 60-second handshake limit.

We need a linear migration path that preserves correctness, adds timing evidence,
and removes best-effort migration from the pre-handshake critical path.

## Requirements Trace

| Requirement | Implementation action |
|---|---|
| Build canonical artifact index once | Unit 1 builds a reusable ID->artifact/path index and rewires migration to use it |
| Add startup diagnostics and benchmarks | Unit 2 adds timing counters/logging and benchmark fixture coverage |
| Reduce pre-handshake startup latency | Unit 3 moves or defers migration from initialize critical path |
| Preserve no-destructive recovery posture | All units avoid DB reset/delete paths and retain typed errors |

## Implementation Units

### Unit 1 - Canonical index refactor

* Changes
  * Refactor migration to construct one canonical artifact index once
  * Replace per-source tree rescans with index lookups
* Files
  * `internal/core/workspace.go`
  * `internal/core/link_migration.go` (or equivalent migration module)
* Tests
  * Unit test proving index built once and reused across all link sources
  * Regression test with synthetic high link-source count
* Posture
  * Test-first

### Unit 2 - Startup diagnostics and benchmark harness

* Changes
  * Add structured timing logs around startup phases
  * Add benchmark/test fixture recording migration and total startup durations
* Files
  * `internal/core/workspace.go`
  * `internal/core/*_test.go` startup benchmark/regression tests
* Tests
  * Timing regression test that fails when startup exceeds agreed threshold on fixture
* Posture
  * Characterization-first then tighten thresholds

### Unit 3 - Critical-path refactor

* Changes
  * Move best-effort link migration out of synchronous initialize path, or gate it as explicit maintenance
  * Ensure initialize returns promptly while preserving eventual consistency expectations
* Files
  * `internal/server/mcp_server.go` or startup orchestration entrypoint
  * `internal/core/workspace.go`
* Tests
  * Integration test confirming MCP initialize completes under timeout budget on large fixture
* Posture
  * Migration-first

## Dependency Graph

* Unit 1 -> Unit 2
* Unit 1 -> Unit 3
* Unit 2 -> Unit 3 (shared timing instrumentation)

No blocking external dependency identified.

## Decisions and Rationale

* Choose canonical indexing over timeout increase because timeout increase masks cost growth
* Keep migration correctness in-core but shift expensive best-effort work off handshake path
* Use benchmarked evidence to prevent regressions after optimization

## Risks and Caveats

* Startup flow changes can impact initialization invariants
  * Mitigation: integration tests around initialize ordering and readiness
* Deferred migration could temporarily expose stale link data
  * Mitigation: explicit status markers and documented eventual-consistency boundary
* Timing assertions can be flaky across CI hosts
  * Mitigation: fixture-relative thresholds and percentile-based assertions

## Plan Hardening Signals

* public API, schema, or contract change: **yes** (startup handshake behavior timing)
* security, auth, permission, or compliance-sensitive behavior: **no**
* migration, backfill, destructive data/config action, or irreversible step: **yes** (migration path)
* external integration, operator checkpoint, or external dependency: **yes** (MCP startup contract)
* high runtime, rollout, or rollback risk: **yes** (core initialization path)

Requires plan hardening: **yes**

## Runtime Verification and Closure

* Runtime surfaces changed
  * MCP startup/initialize path
* Verification
  * Run `backlogit mcp` startup timing capture in engram-scale fixture
  * Confirm initialize returns before 60-second handshake window
  * Confirm post-start migration status converges without destructive repair
* Closure artifact expectations
  * Runtime verification note with before/after startup timings
  * Rollback trigger: revert if initialize latency regresses beyond threshold
  * Owner: Ship execution agent during implementation wave

## Plan Review

decision: PASS

* scope/risk/deliverables validated for Stage gate
* no blocking dependencies identified
* ready for harvest into feature `139-F` with three tasks
* advisory
  * keep threshold values environment-normalized
  * capture diagnostics in machine-readable form for future trend tracking
