---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 194-S (Stage-only dispose adapters, registry parity, role contract, B5)."
doc_type: closure
docline:
  date: 2026-10-21T06:00:00Z
  status: final
  tags:
    - closure
    - 194-S
schema_version: "1.0"
shipment_id: 194-S
source: docs/closure/194-S-194-F-post-merge-closure.md
title: "194-S closure"
---

# 194-S closure

Context: feature 194-F (tasks 194.001-T to 194.008-T), PR #561, merge `27eb21e8b1479324e2517a32615782e2343872e6`. Adds the CLI `backlogit shipment dispose` and the MCP tool `backlogit_dispose_shipment` over the existing `core.DisposeQueuedShipment`, the `dispose_shipment` registry entry (no `cli_command`, `cli_fallback.automatic: false`, confirm required), the regenerated CLI reference, and the Stage-only exception, disposition-only route, and Orchestrator ban in the agent contracts. Both forms are named in every rule (G-B). No runtime surface beyond the CLI and MCP tool, which Ship is forbidden to run against any workspace, so runtime-verification is not applicable.

## Releasability

`closure_status: READY`. Monitoring: none beyond CI. Rollback: revert PR #561. Owner: operator. Validation window: next Stage session that exercises a disposition (operator-run or Stage-run, never Ship). No conditions.

## Evidence

* Local review before PR (reviewed code HEAD `5fcff70c`): a shell-run code-review persona ran the dispose and registry tests, `go vet`, and checked confirm semantics and error-mapping order, outcome READY; security and Go personas (file-read only) READY_WITH_FOLLOWUPS; the adversarial-review agent's sub-reviewers had no shell access, so its only finding was a LOW-confidence unconfirmed P1. No P0 or confirmed P1.
* Copilot independently raised the same Stage disposition-only route gap; fixed in `4469f92a` (within P-021 C1), replied on the thread, resolved, review re-requested; `autoharness gate copilot-review` SATISFIED before and immediately before merge.
* CI green on `4469f92a` (8 checks, merge state CLEAN); merge used the merge-commit strategy (P-009).
* `backlogit shipment ship 194-S --sha 27eb21e8...` returned `shipped` and archived `194.001-T` to `194.008-T`, `194-F`, and `194-S`. The archive records were committed after the governed ship rewrote them.
* Ship never ran `backlogit shipment dispose` or `backlogit_dispose_shipment` (G-B); tests are in-process on temp-dir fixtures.

## R13 release-specific items (194-F, source plan Per-Release Closure Rule)

* Binary version: `backlogit` built from the repository at the pre-merge tree with go1.27.0; the CLI reports commit and build date as `unknown` for this local build.
* Guard scope: the predecessor guard covers shipped provenance (P) only, not scheduler consumption (C).
* Unchanged shipments: `156-S` and every other existing shipment record are unchanged; the diff touched only the 194 records, the hooks queue, code, registry, docs, agent contracts, and memory.
* Manual policy: the manual (P)+(C) 074-DL policy stays in force until AF1E5075 lands and the `154-S` attestation exists; `READY` does not relax it.
* CI-aligned gates: CI jobs `test`, `Windows handle/lock tests`, `Markdown lint (P-008)`, `Docline frontmatter gate`, `CLI Reference Drift`, `Line endings guard`, and `pipeline-topology (ambient)` passed. Local golangci-lint could not run (installed binary built with an older Go than the toolchain) and `go test ./internal/core` exceeded 30 minutes locally under load; local `-race` was not run. CI was authoritative.
* Runtime fixture checks stay in SA-CV and SB-CV under G-A and G-B; none were run here.

## Compaction status (P-020)

`compaction_status: degraded`. No files were moved: retained docs and checkpoints still reference existing `docs/memory/` paths, so compaction was not allowed to move them.

## Follow-ups

* P3: refresh `.autoharness/harness-manifest.yaml` checksums for `_stage.agent.md` and `_orchestrator.agent.md`, and the upstream template sources.
* P3: `plugin/agents/ship.agent.md` and any plugin Stage copy are separate copies without the new wording; revisit with the B6 workflow-doc work.
* P3: `.autoharness/backlog-registry.yaml` is only one place; `.github/instructions/backlog-integration.instructions.md` extended-operations table omits `dispose_shipment` (B6 docs lag, as planned in I-6).
* P3: the dispose CLI test should use `t.Cleanup` for `ws.Close()`; `By` is an audit label, not an identity, and authorization is enforced procedurally by the Stage contract (P2 advisory).
* The Ship role-boundary tool list cannot omit the tool because `_ship.agent.md` grants `backlogit/*`; the ban is prose-enforced.

## Source artifact provenance (untouched)

Source stash `6434A4D7` and its deliberation are provenance only and were not mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. Observation: backlogit CLI claim, move, and ship calls took 3 to 15 minutes each under server load; they converged, so poll state rather than re-issuing.
