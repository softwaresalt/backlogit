---
title: "Parity test realism and governed-operation expansion follow-ups"
description: "Low-priority plan for 118-S parity realism and F6 governed:true scope extension"
source: ".backlogit/archive/058-DL.md"
doc_type: plan
schema_version: "1.0"
---

## Source

* Deliberation: `.backlogit/archive/058-DL.md` (EA3BC800)
* Related stash: `4CF89803` (governance extension)
* Deferred external-only stash: `7F0A6E89`, `6FA0829B`

## Problem Frame

Two low-priority enhancements are ready for staging:

* parity test should validate actual Cobra dep-list output path rather than
  relying on constructed expectation strings
* governed-operation policy should expand beyond commit association while
  preserving CLI/MCP parity guarantees

Both are bounded and can ship independently.

## Requirements Trace

| Requirement | Implementation action |
|---|---|
| Parity test executes real CLI path | Unit 1 updates dep parity test harness to run Cobra command path |
| Governed scope can expand safely | Unit 2 defines next governed operations and parity checks |
| Keep external-repo template work out of backlogit scope | External stash items remain deferred in stash with blocked context |

## Implementation Units

### Unit 1 - Dep-list parity realism

* Changes
  * Update parity test to invoke actual Cobra `dep list` behavior
  * Assert output contract from real command path
* Files
  * `internal/cli/dep_type_parity_test.go`
* Tests
  * Existing parity suite with real execution path
* Posture
  * Test-first

### Unit 2 - Governed:true expansion plan and implementation

* Changes
  * Select additional candidate operations for governed policy
  * Apply `governed: true` markers and extend parity assertions
* Files
  * `.autoharness/backlog-registry.yaml`
  * related CLI/MCP parity tests under `internal/cli/*parity*_test.go`
* Tests
  * Guard tests that fail when governed CLI/MCP drift appears
* Posture
  * Characterization-first

## Dependency Graph

* Unit 2 depends on Unit 1 only for parity-pattern reuse (soft dependency)

No hard execution block between units.

## Decisions and Rationale

* Keep two separate features to avoid coupling low-risk parity and governance work
* Preserve deferral of external template propagation to autoharness workspace
* Reuse existing parity governance patterns from `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`

## Risks and Caveats

* Parity test invoking real CLI may increase test runtime
  * Mitigation: keep fixture small and deterministic
* Governed scope expansion may unintentionally over-constrain operations
  * Mitigation: phase markers and validate behavior with explicit tests

## Plan Hardening Signals

* public API, schema, or contract change: **yes** (registry governance contract)
* security, auth, permission, or compliance-sensitive behavior: **no**
* migration, backfill, destructive data/config action, or irreversible step: **no**
* external integration, operator checkpoint, or external dependency: **no**
* high runtime, rollout, or rollback risk: **no**

Requires plan hardening: **no**

## Runtime Verification and Closure

* Runtime surfaces changed
  * CLI/MCP parity test and governed metadata behavior
* Verification
  * Run parity tests and confirm real Cobra path is exercised
  * Validate governed operation map in metadata output
* Closure artifact expectations
  * Verification note listing updated governed operations and parity test evidence
  * Owner: Ship execution agent for low-priority enhancement wave

## Plan Review

decision: PASS

* bounded low-priority scope validated
* no blocking dependencies
* external-repo-only follow-ups explicitly excluded from this plan
* advisory
  * keep governed expansion incremental to simplify rollback if parity regresses
