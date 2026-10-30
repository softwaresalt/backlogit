---
chunk_strategy: h1-h2-h3
description: 'Stage Phase 3 session memory: deliberate, plan, harden, review, harvest stash A441A74C into feature 207-F (207.001-T..207.003-T), create follow-up stash 21BA4886, and assemble shipment 208-S.'
doc_type: memory
docline:
  date: 2026-10-30T17:52:00Z
  feature_id: 207-F
  shipment_id: 208-S
  status: done
  tags:
    - stage
    - A441A74C
    - 207-F
    - 208-S
    - 21BA4886
    - complexity-provenance
    - model-routing
schema_version: "1.0"
source: docs/memory/2026-10-30/stage-208-s-complexity-provenance-memory.md
title: 'Stage memory: Phase 3 complexity attribution and provenance (stash A441A74C, feature 207-F, shipment 208-S)'
---

## Summary

* **Mode and route**: Stage Phase 3 of 3 (`A441A74C` — Complexity attribution and provenance for work-attribute model routing) under `DARK_MODE_ACTIVE` (`2026-10-30T05:55:05Z`); model route `gemini-4-argon` / `google` / `high` / `long_context`.
* **Branch**: `chore/stage-208-S` branched from `main` at `480b379eca3d52a95ab1506cad31179c07e85bfb`.
* **Startup recovery & hook consumption**: `backlogit_list_checkpoints(consumer_id: "stage")` returned zero active or quarantined checkpoints; `backlogit_poll_hook_events(consumer_id: "stage")` returned events up to `seq: 4304` (post-ship `207-S` events), which were acknowledged via `backlogit_ack_hook_events(consumer_id: "stage", seq: 4304)`.

## Pipeline Execution

### 1. Stash Triage and Deliberation (`A441A74C`)

* Classified stash `A441A74C` (`medium` / `feature`) as feature-shaped.
* Retrieved compound learnings (`confidence: high`) covering task-only typed metadata guards (`2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`), CLI reference drift regeneration (`cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`), docline frontmatter contract, and generic update provenance preservation.
* Authored `docs/decisions/2026-10-30-complexity-attribution-provenance-deliberation.md` (`decision_status: decided`, `promoted_to: plan`), resolving all 5 open questions from `docs/design-docs/2026-09-10-complexity-attribution-provenance.md`:
  1. **Audit-event shape**: Reuse `estimate_history` with `"dimension": "complexity"` in `delta` (appended fail-closed before durable frontmatter write).
  2. **Ruleset ownership & naming**: `complexity_ruleset_version` is an opaque non-empty string validated for presence when `complexity_source` is set and rejected if whitespace-only or containing control characters.
  3. **Clear and value-only update provenance-reset semantics**: Clearing `complexity` (`complexity == ""`) atomically clears `complexity`, `complexity_source`, and `complexity_ruleset_version` and rejects any accompanying provenance fields; updating or re-setting a non-empty `complexity` value with empty provenance (`source == ""` and `rulesetVersion == ""`) sets `custom_fields["complexity"] = value` AND deletes any pre-existing `custom_fields["complexity_source"]` and `custom_fields["complexity_ruleset_version"]` keys (and nulls both columns in SQLite) so a value-only update from `high` (with `human`/`ah-v1` provenance) to `trivial` (without provenance) immediately makes `IsTaskRouteEligible` return `false` (`provenance-incomplete`) rather than retaining stale provenance.
  4. **Route-eligibility predicate**: Expose `IsTaskRouteEligible(item *models.Artifact) bool` in `internal/db` and `internal/core` returning `true` iff `item` is a `task` and all 6 size + complexity provenance fields (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version`) are present and valid.
  5. **Scope & phasing**: Deliver Phase 1 + Phase 2 + CLI/MCP transport-aware parity in `208-S` across 3 atomic `code` tasks (`207.001-T`, `207.002-T`, `207.003-T`), and capture Phase 3/4 (Stage harvest complexity-homogeneity assessment and deferred feature/shipment `ComplexityComposition` rollup) in follow-up stash `21BA4886`.

### 2. Implementation Plan, Hardening, and Plan Review

* Authored `docs/exec-plans/2026-10-30-complexity-attribution-provenance-plan.md` with 3 sequential single-domain (`code`) units (`U1 -> U2 -> U3`), `Constitution Check: pass`, `Requires plan hardening: yes`, `## Plan Hardening` (Invariants I1–I5, learnings consulted, and `strict-safety` classifications), `## Runtime Verification and Closure`, and `## Plan Review` (`dispatch_mode: single-agent-declared-degradation`, `decision: PASS`, `operator_authorization: approved`, `P0=0, P1=0, P2=0, P3=3 applied`).
* Addressed Copilot review findings on PR `#630`:
  * **Thread `PRRT_kwDORzozKM6yFRMP` (line 77)**: Updated R1, U1 (`207.001-T`), U3 (`207.003-T`), and Invariants I1/I2 so a value-only complexity update (`complexity` non-empty, `source == ""` and `rulesetVersion == ""`) deletes any pre-existing `custom_fields["complexity_source"]` and `custom_fields["complexity_ruleset_version"]` keys (and nulls both columns in SQLite), with explicit unit tests in `internal/core/artifact_complexity_test.go` and `internal/mcp/complexity_metadata_test.go` verifying that updating a provenance-complete task (`complexity: "high", complexity_source: "human", complexity_ruleset_version: "ah-v1"`) with a value-only update (`complexity: "trivial"`) clears the provenance pair and makes `IsTaskRouteEligible` return `false`.
  * **Thread `PRRT_kwDORzozKM6yFRNF` (line 129)**: Updated U3 (`207.003-T`) so `--complexity-source` and `--complexity-ruleset-version` are added to `artifactUpdateFlagsChanged` and `conflictingSizeFlags` in `internal/cli/update.go`, but excluded from `conflictingComplexityFlags` (matching `size-source` and `size-ruleset-version` in `conflictingSizeFlags`), allowing `--complexity`, `--complexity-source`, and `--complexity-ruleset-version` to be combined without self-conflict.

### 3. Harvest, Follow-Up Stash, and Shipment Assembly (`207-F`, `208-S`, `21BA4886`)

* Harvested `A441A74C` into covering feature `207-F` (*Complexity attribution and provenance for work-attribute model routing*, `priority: high`, `status: queued`), automatically archiving `A441A74C` from `.backlogit/stash.jsonl` to `.backlogit/archive/stash.jsonl`.
* Created 3 atomic tasks under `207-F` (`size: M`, `size_source: agent`, `size_ruleset_version: 1.0`, `complexity: medium`, `priority: high`) with `blocks` dependencies `207.001-T -> 207.002-T -> 207.003-T`:
  * `207.001-T` (U1): *Implement header-def and core complexity provenance seam with fail-closed audit event*
  * `207.002-T` (U2, blocked by `207.001-T`): *Project complexity provenance to SQLite with bound-parameter filters and IsTaskRouteEligible predicate*
  * `207.003-T` (U3, blocked by `207.002-T`): *Wire CLI and MCP transport-aware complexity provenance parity and regenerate CLI reference docs*
* Created follow-up stash entry `21BA4886` (`kind: feature`, `priority: low`) for deferred Phase 3/4 items.
* Assembled shipment `208-S` (*Complexity attribution and provenance for model routing (207-F)*, `priority: high`, `status: queued`, `items: ["207-F", "207.001-T", "207.002-T", "207.003-T"]`, blocked by `207-S`).
