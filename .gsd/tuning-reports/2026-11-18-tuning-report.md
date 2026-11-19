---
title: "Harness tuning report"
doc_type: tuning-report
schema_version: "1.0"
created_at: "2026-11-18T23:52:47Z"
updated_at: "2026-11-18T23:52:47Z"
status: pass-with-warnings
source: tune-harness
gsd_version: "1.6.0"
branch: "chore/gsd-harness-migration"
---

# Harness Tuning Report - 2026-11-18

## Outcome

The GSD 1.6.0 migration state is usable and deterministic. This tune pass cleared
all strict schema blockers, unresolved findings, stale manifest checksums, and
escalation-route drift that could make future renders disagree with
`.gsd/config.yaml`.

The workspace remains on the non-default branch
`chore/gsd-harness-migration`. No commit or push was performed. Backups for
modified managed artifacts are under `.gsd/backups/2026-11-18/`.

## Drift Summary

* Breaking changes found and resolved: 3 groups
* Degrading changes found and resolved: 5
* Growth opportunities: 0
* Cosmetic adjustments: 0
* New artifact candidates: 0
* Learning-driven proposals: 0

## Composition

| Setting | Installed | Current recommendation |
|---|---|---|
| Primary stack pack | `mcp-server` | `mcp-server` |
| Stack packs | `mcp-server`, `cli-tool` | `mcp-server`, `cli-tool` |
| Preset | `full` | `full` |
| Capability packs | `backlogit`, `agent-engram`, `kbdb` | unchanged |
| Backlog root | `.backlogit` | unchanged legacy-compatible root |
| Config schema | `1.3.0` | current |
| Manifest schema | `1.1.0` | current |
| Profile schema | `1.1.0` | current |

## Checksum Scan

* Manifest-tracked artifacts checked: 97
* Missing artifacts: 0
* User-modified artifacts: 0
* Unchanged artifacts: 97
* Ignored artifacts: 0
* Startup contract: `start.sh` is `current`

## Applied Changes

### P0 - Breaking

1. **TUNE-001 - Escalation-chain directive parity**
   * Replaced hand-derived escalation route prose with the deterministic
     `gsd routing resolve` contract in Stage, Ship, P-013.6, and the canonical
     escalation instruction.
   * Added ordered-chain advancement, `--unavailable`, fail-closed error tokens,
     and `ESCALATION_DEGRADED: chain-exhausted`.
   * Status: applied.

2. **TUNE-002 - Manifest template provenance**
   * Reclassified workspace-owned deploy scripts and trusted-source skills and
     prompts so the manifest no longer references nonexistent or unsafe paths
     inside the 1.6.0 bundle.
   * Status: applied.

3. **TUNE-003 - Backlog header checksum recovery**
   * Refreshed the stale `.backlogit/header-def.yaml` checksum to the verified
     committed working-tree digest.
   * Status: applied.

### P1 - Degrading

1. **TUNE-004 - Live route alignment**
   * Aligned Ship frontmatter to `gemini-4-hydrogen` / `google` / `high`.
   * Updated manifest tier, escalation, alternate-doc-review, KBDB binary, and
     capability-pack render seeds from `.gsd/config.yaml`.
   * Removed baked route tuples from Stage and Ship escalation bodies so runtime
     resolution is authoritative.
   * Status: applied.

2. **TUNE-005 - Resolver and policy coherence**
   * Clarified nested-route versus legacy-flat block selection.
   * Documented that schema `1.3.0` is chain-free and requires schema `1.4.0`
     before ordered chains or unavailable lists are valid.
   * Status: applied.

3. **TUNE-006 - Adjacent render drift**
   * Updated `doc-review` from `gemini-3.8-flash` to `gemini-4-hydrogen`.
   * Repaired the Ship Circuit Breakers table by moving the P-021 annotation
     after the table.
   * Status: applied.

4. **TUNE-007 - Manifest audit trail**
   * Added current route variables, refreshed affected artifact checksums, and
     recorded the tune in `tuning_history`.
   * Status: applied.

## Adversarial Verification

### Reviewer Dispatch

* Anchor Reviewer (`gpt-6.1-sol`, high): 0 findings
* Reviewer A - Template Fidelity (`gemini-4-argon`, high): 1 finding
* Reviewer B - Overlay Coherence (`claude-sonnet-5.5`, high): 10 findings
* Reviewer C - Cross-Reference Integrity (`claude-haiku-5.5`, xhigh): 16 findings

### Consensus Findings

#### High Confidence

| Finding | Resolution |
|---|---|
| Manifest route seeds disagreed with live config | Fixed and checksum-verified |
| Ship role frontmatter disagreed with `model_routing.ship` | Fixed and resolver-verified |
| Stage and Ship hard-coded route tuples were stale | Removed in favor of resolver output |
| Worked tier3 example contradicted the live tier3 route | Replaced with a route-neutral example |
| Ship Circuit Breakers table was split by an annotation | Reassembled into one table |
| `doc-review` still referenced the prior Gemini family | Updated and checksum-verified |
| Nested escalation fallback could be read as inheriting legacy flat fields | Clarified whole-block precedence |

#### Medium Confidence - Operator Review

| Priority | Finding | Recommended decision |
|---|---|---|
| P1 | GSD `strict-safety` / P-013.3 operator-confirmation prose may conflict with P-013.6 automatic reasoning escalation for Ship | Confirm whether reasoning escalation supersedes P-013.3 for route dispatch, or add an explicit confirmation gate |
| P1 | P-010 does not mirror the Stage-only queued-shipment disposition and Ship capture-only stash carve-out already present in agent role boundaries | Add policy clauses and amendment entries, then update affected cross-references |
| P2 | Stage lacks the explicit Stop Conditions table and operator prompt used by the escalation protocol | Add a Stage stop table tied to plan-review and equivalent gate failures |
| P2 | The workflow policy version header reaches `1.33.0`, while the visible Amendment Log stops earlier | Reconstruct missing amendment rows from governed tuning history |
| P2 | Escalation payloads reference `schemas/tool-telemetry-event.schema.json`, but consumers do not qualify the schema as GSD-home relative | Clarify the reference or vendor the schema with manifest provenance |
| P2 | Retired `agent-intercom` wording remains in some workflow surfaces as conditional legacy text | Review and remove stale woven sections without deleting operator-owned custom text |

### Verification Result

**PASS WITH WARNINGS**

* All HIGH-confidence findings were resolved.
* Remaining findings require policy or operator decisions and were not
  auto-remediated.
* No HIGH-confidence CRITICAL or MAJOR findings remain.

## Deterministic Verification

* `gsd verify-workspace`: 0 blockers, 0 strict schema blockers, 0 unresolved
* Checksum scan: 97 unchanged, 0 missing, 0 divergent
* Stage resolver: `gpt-6.1-sol` / `openai` / `high` / `long_context`
* Ship resolver: `gpt-6.1-sol` / `openai` / `medium` / `long_context`
* `git diff --check`: pass

The verifier process still exits non-zero because
`backlog_registry_cli_surface` reports `dispose_shipment` as unimplemented.
That is a stale compatibility assertion: the current workspace binary exposes
`backlogit shipment dispose`. The registration is preserved rather than removed.

The verifier also reports one non-blocking
`backlogit-engine-version-unverified` warning for the local
`1.12.1-0.20261118031214-5fe752c9022a` development build.

## Recommendation

Keep the tuned artifacts on
`chore/gsd-harness-migration`, review the focused harness diff, and use the next
policy-focused work unit for the medium-confidence P-013 / P-010 and amendment
history decisions. File the stale `dispose_shipment` registry assertion and
development-build compatibility warning with the GSD/verifier maintainers.
