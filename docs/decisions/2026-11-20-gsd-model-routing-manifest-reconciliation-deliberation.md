---
chunk_strategy: h1-h2-h3
description: 'P-021 C6 co-deliberation for stash entries BE9AD4F1 and A991B4AC (104-DL): reconcile carried-forward .gsd/config.yaml model_routing with .gsd/harness-manifest.yaml, 20 tiered .github/agents/** definitions, .github/skills/doc-review/SKILL.md, and .github/instructions/adversarial-review.instructions.md.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - .backlogit/archive/104-DL.md
    - .backlogit/queue/219-F.md
    - .backlogit/queue/220-S.md
    - docs/plans/2026-11-20-gsd-model-routing-manifest-reconciliation-plan.md
  promoted_to: both
  tags:
    - model-routing
    - harness-manifest
    - gsd-tune
    - BE9AD4F1
    - A991B4AC
    - 104-DL
    - 219-F
    - 220-S
  topic: 'Reconcile GSD model_routing across .gsd/config.yaml, harness-manifest.yaml, agents, doc-review, and adversarial-review (BE9AD4F1 + A991B4AC / 104-DL)'
schema_version: "1.0"
source: docs/decisions/2026-11-20-gsd-model-routing-manifest-reconciliation-deliberation.md
title: 'Deliberation: GSD Model-Routing and Harness-Manifest Reconciliation (BE9AD4F1, A991B4AC / 104-DL)'
---

## Problem Frame

Uncommitted operator `model_routing` updates in `.gsd/config.yaml` are staged for P-011 carry-forward onto the next shipment branch:

* `tier1`: `claude-haiku-5.5` / `anthropic` / `reasoning_effort: "high"` / `context_tier: "long_context"` (updated from `xhigh` / `default`)
* `tier2`: `mai-code-2` / `microsoft` / `reasoning_effort: "high"` / `context_tier: "long_context"` (updated from `claude-sonnet-5.5` / `anthropic` / `default`)
* `tier3`: `gemini-4-carbon` / `google` / `reasoning_effort: "high"` / `context_tier: "long_context"` (updated from `gemini-4-argon` / `claude-sonnet-5.5` / `xhigh` / `default`)
* `orchestrator`: `gemini-4-carbon` / `google` / `reasoning_effort: "medium"` / `context_tier: "long_context"` (updated from `claude-sonnet-5.5` / `anthropic`)
* `stage`: `gemini-4-carbon` / `google` / `reasoning_effort: "medium"` / `context_tier: "long_context"` (updated from `gemini-4-argon` / `high`) + `stage.escalation`: `gpt-6.2-sol` / `openai` / `high` / `long_context` (updated from `gpt-6.1-sol`)
* `ship`: `gemini-4-hydrogen` / `google` / `reasoning_effort: "high"` / `context_tier: "long_context"` + `ship.escalation`: `gpt-6.2-sol` / `openai` / `medium` / `long_context` (updated from `gpt-6.1-sol`)
* `anchor_review`: `gpt-6.2-sol` / `openai` / `reasoning_effort: "high"` (updated from `gpt-6.1-sol`)
* `alt_doc_review`: `gemini-4-hydrogen` / `google` (updated from `gemini-3.8-flash` in `.github/skills/doc-review/SKILL.md`)

Two active `DEFERRED SCOPE EXPANSION` stash entries govern this reconciliation:

1. **`BE9AD4F1`** (`high`, `task`, captured during `219-S` / `218-F`): Reconcile carried-forward `.gsd/config.yaml` `model_routing` edits with `.gsd/harness-manifest.yaml` (`config_hash`, `variables_used`, LF-normalized SHA-256 `checksum`s) and `.github/agents/**` so `TestU12_HarnessManifestNamespaceReconciliation` and `gsd verify-workspace` pass.
2. **`A991B4AC`** (`medium`, `chore`, captured during `215-S` / `214-F`): Reconcile `model_routing.ship` and `model_routing.alt_doc_review` with `.gsd/harness-manifest.yaml` (`ALT_DOC_REVIEW_FAMILY`), `.github/skills/doc-review/SKILL.md`, and `.github/agents/_ship.agent.md` while verifying cross-model diversity.

Stage is forbidden by its Role Boundary (`P-010`) from modifying or staging `.gsd/config.yaml`, `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, or `.kbdb/`. Ship will commit `.gsd/config.yaml` onto the shipment branch via `P-011` carry-forward before wave execution.

## P-021 C5/C6 Provenance and Triage Outcomes

| Stash ID | Kind / Priority | Reconciled Source Refs | Duplicate Scan (Obligation A) | Late-Identifier Reconciliation (Obligation B) |
| --- | --- | --- | --- | --- |
| `BE9AD4F1` | `task` / `high` | `task_id=218.004-T`; `feature_id=218-F`; `shipment_id=219-S`; `PR=#665`; `review_thread=N/A` | `CLEAN DUPLICATE SCAN` (`0` duplicates; `731CE551` superseded by GSD 1.6.0 migration and archived in Step 1 hygiene) | Reconciled `PR: N/A` to `PR #665` in place from `docs/closure/219-S-218-F-post-merge-closure.md`; `review_thread: N/A` stands ("no late review-thread identifier found") |
| `A991B4AC` | `chore` / `medium` | `task_id=214.002-T,214.012-T`; `feature_id=214-F`; `shipment_id=215-S`; `PR=#651`; `review_thread=N/A` | `CLEAN DUPLICATE SCAN` (`0` duplicates; co-deliberated with `BE9AD4F1` into `104-DL`) | Reconciled `PR: N/A` to `PR #651` in place from `docs/closure/215-S-214-F-post-merge-closure.md`; `review_thread: N/A` stands ("no late review-thread identifier found") |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md`: Checksums in `.gsd/harness-manifest.yaml` are computed over LF-normalized bytes (`sha256(raw.replace("\r\n", "\n"))`), duplicate YAML keys must be rejected explicitly, and existing `drift_allowed: true` / `drift_reason` fields must be preserved rather than duplicated.
* `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md`: Contract tests guarding harness/agent/skill alignment must inspect actual in-repo `.gsd/` and `.github/**` files and fail honestly at baseline (`P-004`).
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`: Producer and consumer governance artifacts (`.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, `.github/agents/**`, `.github/skills/doc-review/SKILL.md`, and `.github/instructions/adversarial-review.instructions.md`) must update exact field/model tokens in lockstep.
* `docs/compound/2026-10-17-discovery-vs-overlay-predicates.md`: `.gsd/` and `.github/` artifacts stay at `BaseRef` (`main`) during `redhonesty` baseline evaluation while new `*_test.go` files are overlaid, providing an honest assertion RED before `.gsd/harness-manifest.yaml` is updated.

### Codebase and Manifest Inventory

1. **Manifest-pinned integration guards**:
   * `TestU12_HarnessManifestNamespaceReconciliation` (`tests/integration/u12_harness_manifest_namespace_test.go`) asserts `manifest["config_hash"] == lfSHA256Hex(.gsd/config.yaml)`, no duplicate YAML keys, unique artifact paths, and preservation of `"DOCS_PLANS corrected to docs/exec-plans per workspace config."`. Because `.gsd/config.yaml` is committed at `P-011` pre-claim before Wave 1 executes, `TestU12_HarnessManifestNamespaceReconciliation` already fails at pre-claim and **requires `.gsd/harness-manifest.yaml` to be reconciled inside Wave 1** before Wave 1 Step 4.6 runs `go test -timeout=30m ./...`.
   * `TestUSR6_HarnessManifestDriftRecords` (`tests/integration/harness_manifest_196_drift_records_test.go`), `TestUCXS17_RefreshedDriftRecords` (`tests/integration/harness_manifest_197_drift_records_test.go`), and `TestU19R3_ManifestBudgetReconciliation` (`tests/integration/harness_manifest_budget_reconciliation_test.go`) assert `drift_allowed: true`, `"Do not auto-revert."`, stash citations (`227A2930`, `B83081F5`, `75E02C17`, `52D18E44`, `497D20E3`, `67F17B6B`, `FBD6E6F8`, `8F1CF1E1`, `6AB5E7FC`, `F05661B1`), and the `073-DL rev22` governed full-suite budget sentence.
   * `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery` and `TestWorkspaceAgentsGrantFullBacklogitToolSurface` (`tests/integration/shipment_155_harness_contract_test.go`) assert `backlogit/*` tool wildcards and workspace-specific lifecycle rules in `_orchestrator.agent.md`, `_stage.agent.md`, `_ship.agent.md`, and workspace subagents.
2. **Exact `.github/**` artifact delta (22 files)**:
   * `_ship.agent.md` frontmatter (`gemini-4-hydrogen` / `google` / `high` / `long_context`) and `.github/instructions/escalation-protocol.instructions.md` (generic P-013.6 resolver prose) already match `.gsd/config.yaml` with zero stale model tokens; non-tiered custom agents (`mcp-protocol-reviewer.agent.md`, `sqlite-reviewer.agent.md`, `go-mcp-expert.agent.md`) do not participate in GSD tier routing and remain untouched.
   * Exactly **20 tiered `.github/agents/**` files**, **`.github/skills/doc-review/SKILL.md`** (`gemini-3.8-flash` -> `gemini-4-hydrogen`), and **`.github/instructions/adversarial-review.instructions.md`** (`gpt-6.1-sol` -> `gpt-6.2-sol`, `GPT-6.1 Sol` -> `GPT-6.2 Sol`) contain stale model-routing frontmatter and/or prose. Of those 22 files, 17 are tracked in `.gsd/harness-manifest.yaml` (7 pristine templates whose rendered frontmatter updates cleanly via `variables_used`, plus 10 `drift_allowed: true` entries whose `drift_reason` records must be preserved verbatim while rotating `checksum`).
3. **Cross-model diversity audit (`A991B4AC`)**:
   * **Anthropic**: `tier1` (`claude-haiku-5.5`)
   * **Microsoft**: `tier2` (`mai-code-2`)
   * **Google**: `tier3`, `orchestrator`, `stage` (`gemini-4-carbon`), plus `ship` and `alt_doc_review` (`gemini-4-hydrogen` — distinct provider from Tier 2 `microsoft`/`mai-code-2`, preserving genuine cross-provider doc-review diversity!)
   * **OpenAI**: `anchor_review`, `stage.escalation`, and `ship.escalation` (`gpt-6.2-sol` — distinct provider from Google `stage`/`ship` and Tier 1/2/3, guaranteeing non-degraded P-013.6 escalation and independent 4-provider `adversarial-review` pools!)

## Options Evaluated

### Option A: Blind Upstream `gsd` Template Re-Render Overwriting `.github/agents/**` and `.gsd/harness-manifest.yaml`

* **Pros**: Single mechanical command.
* **Cons**: Overwrites workspace-specific `backlogit/*` tool wildcards, `_stage`/`_ship`/`_orchestrator` role rules, and `drift_reason` citations, immediately breaking `TestWorkspaceAgentsGrantFullBacklogitToolSurface`, `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery`, `TestUSR6_HarnessManifestDriftRecords`, `TestUCXS17_RefreshedDriftRecords`, and `TestU19R3_ManifestBudgetReconciliation`.
* **Effort estimate**: Low (initial), High (breakage).
* **Fit**: Unacceptable.

### Option B: Co-Scheduled Single-Wave Decomposition (11 Two-File `docs-only` Tasks + 1 Two-File `harness-ready` Manifest/Contract Task) - Selected

* **Description**:
  1. Ship commits carried-forward `.gsd/config.yaml` on the `220-S` shipment branch via `P-011` pre-claim carry-forward.
  2. Partition the 22 `.github/**` Markdown files into **11 width-isolated, 2-file `P-002.1` `docs-only` tasks (`219.001-T`..`219.011-T`)**, each updating only model-routing frontmatter and model-routing prose while preserving all workspace-specific `tools:`, role-boundary rules, and headings, gated by a non-vacuous content + `go run ./cmd/backlogit docs lint` `exempt_verification_command`.
  3. Co-schedule **`219.012-T` (`U12`, `harness-ready`, 2 files: `.gsd/harness-manifest.yaml` and `tests/integration/gsd_model_routing_reconciliation_test.go`)** last in **Wave 1** manifest order (mirroring the `214-F` / `215-S` co-scheduling precedent). `harness-architect` scaffolds `TestU12_GSDModelRoutingManifestReconciliation` RED at Wave 1 entry; `219.001-T`..`219.011-T` land the 22 `.github/**` updates; `219.012-T` reconciles `.gsd/harness-manifest.yaml` (`config_hash`, `variables_used`, `tuned_at`, and all 17 touched manifest `checksum`s while preserving every `drift_allowed`/`drift_reason`/historical note); and Wave 1 Step 4.6 passes `TestU12_HarnessManifestNamespaceReconciliation`, `TestUSR6`, `TestUCXS17`, `TestU19R3`, `TestWorkspaceAgentsGrantFullBacklogitToolSurface`, and `gsd verify-workspace` in one clean convergence pass.
* **Pros**: Strictly satisfies `< 3 files` per task (`2` files on all 12 tasks), `12 < 20` session task budget, P-002.1/P-002.3/P-002.4/P-004 (`redhonesty`), and Wave 1 Step 4.6 convergence.
* **Cons**: Requires explicit Wave 1 manifest ordering (`219.001-T`..`219.011-T` before `219.012-T`), which `backlogit_create_shipment` / `backlogit_add_to_shipment` guarantees deterministically.
* **Effort estimate**: Medium.
* **Fit**: Exact match.

### Option C: Multi-Wave Split (`.github/**` in Wave 1, `.gsd/harness-manifest.yaml` in Wave 2)

* **Pros**: Explicit dependency edges from `U12` to `U1`..`U11`.
* **Cons**: Because Ship's `P-011` carry-forward commits `.gsd/config.yaml` before Wave 1 begins, deferring `.gsd/harness-manifest.yaml` `config_hash` reconciliation to Wave 2 causes `TestU12_HarnessManifestNamespaceReconciliation` to fail deterministically during Wave 1 Step 4.6 (`go test -timeout=30m ./...`).
* **Effort estimate**: Medium.
* **Fit**: Fails P-002.6 Wave 1 convergence.

## Trade-off Comparison

| Criterion | Option A (Blind Re-Render) | Option B (Co-Scheduled Wave 1) | Option C (2-Wave Split) |
| --- | --- | --- | --- |
| Preserves workspace `tools:` & drift records | Fail | Pass | Pass |
| `< 3 files` per task (Constitution Granularity) | N/A | Pass (`2` files x 12 tasks) | Pass |
| Wave 1 Step 4.6 `TestU12` convergence | Fail | Pass | Fail (`config_hash` stale at Wave 1 end) |
| P-002.1 / P-004 `redhonesty` compliance | Fail | Pass | Pass |

## Decision

Adopt **Option B**. Harvest `BE9AD4F1` and `A991B4AC` together into covering Feature **`219-F`** and Shipment **`220-S`** (carrying label `dag-root` so `gsd gate pipeline-topology --mode agent --shipment 220-S --phase pre_claim --json` passes with `predecessor_provenance: declared_root`). Update the explanatory cross-provider comment in `.gsd/config.yaml` only if needed? **No** — `.gsd/config.yaml` is carried forward as-is by Ship under `P-011` so operator config bytes remain exact.

## Rejected Alternatives

* **Option A** was rejected because workspace-specific agent contracts and manifest drift records are enforced by live integration tests.
* **Option C** was rejected because `P-011` carry-forward of `.gsd/config.yaml` precedes Wave 1, making Wave 1 `config_hash` alignment mandatory for Wave 1 Step 4.6 full-suite convergence.

## Unresolved Questions

None.

## Risks and Mitigations

* **CRLF vs LF checksum mismatch on Windows**: All `.github/**` and `.gsd/harness-manifest.yaml` writes must preserve LF line endings (`\n`), and `TestU12_GSDModelRoutingManifestReconciliation` verifies LF-normalized SHA-256 parity alongside `gsd verify-workspace --workspace . --json`.
* **Accidental clobbering of `drift_reason` or `tools:`**: Each `docs-only` task specifies exact frontmatter/prose replacements, and `TestU12_GSDModelRoutingManifestReconciliation` plus existing `TestUSR6`, `TestUCXS17`, `TestU19R3`, and `TestWorkspaceAgentsGrantFullBacklogitToolSurface` guard all preserved invariants.

## Decision Review (`gsd-dark-mode` Step 2.1 Multi-Persona + `rubber-duck`)

* **Reviewers applied**: `Constitution Reviewer`, `Architecture Strategist`, `Scope Boundary Auditor`, `Template Integrity Reviewer`, `Learnings Researcher`, and `rubber-duck` adversarial critique.
* **`rubber-duck` challenge & resolution**: Verified that (1) `.github/instructions/escalation-protocol.instructions.md` and `.github/agents/_ship.agent.md` contain zero stale model strings while `.github/instructions/adversarial-review.instructions.md` *does* contain stale `gpt-6.1-sol` and is explicitly included in scope; (2) co-scheduling `219.001-T`..`219.012-T` in Wave 1 with `219.012-T` last in manifest order prevents `TestU12_HarnessManifestNamespaceReconciliation` from failing at Wave 1 Step 4.6; and (3) Stage touches zero `.gsd/` files (`P-010`). Verdict: **PASS (`0 P0, 0 P1`)**.
