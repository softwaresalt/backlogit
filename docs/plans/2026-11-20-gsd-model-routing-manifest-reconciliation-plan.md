---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for Group 1A (BE9AD4F1 + A991B4AC / 104-DL -> 219-F / 220-S): reconcile carried-forward .gsd/config.yaml model_routing with .gsd/harness-manifest.yaml, 20 tiered .github/agents/** definitions, .github/skills/doc-review/SKILL.md, and .github/instructions/adversarial-review.instructions.md.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-20-gsd-model-routing-manifest-reconciliation-plan.md
title: 'Plan: GSD Model-Routing and Harness-Manifest Reconciliation (219-F / 220-S)'
---

## Problem Frame

Uncommitted operator `model_routing` edits in `.gsd/config.yaml` will be carried forward by Ship onto the `220-S` shipment branch at pre-claim under `P-011` (`Step 3` of `/gsd-dark-mode`). As established in [the Group 1A deliberation](../decisions/2026-11-20-gsd-model-routing-manifest-reconciliation-deliberation.md) (`104-DL`, covering stash entries `BE9AD4F1` and `A991B4AC`):

1. **`TestU12_HarnessManifestNamespaceReconciliation` (`tests/integration/u12_harness_manifest_namespace_test.go`)**: Immediately fails once `.gsd/config.yaml` is committed at pre-claim because `.gsd/harness-manifest.yaml` `config_hash` (`2b114a784fdf277e9552280767f798d706695eb8d5e191573486409011096157`) does not match the LF-normalized SHA-256 of `.gsd/config.yaml`.
2. **Stale `.github/**` Model-Routing Frontmatter & Prose (22 files)**:
   * `_orchestrator.agent.md` (`claude-sonnet-5.5`/`anthropic` -> `gemini-4-carbon`/`google` + `Model Routing` table/prose)
   * `_stage.agent.md` (`gemini-4-argon`/`high` -> `gemini-4-carbon`/`medium`)
   * `review/adversarial-review.agent.md` & `instructions/adversarial-review.instructions.md` (Tier 3 `claude-sonnet-5.5`/`anthropic`/`xhigh`/`default` -> `gemini-4-carbon`/`google`/`high`/`long_context`, Anchor Reviewer `gpt-6.1-sol` -> `gpt-6.2-sol`, Reviewer-B Tier 2 -> `mai-code-2`, Reviewer-C Tier 3 -> `gemini-4-carbon`)
   * `skills/doc-review/SKILL.md` (`gemini-3.8-flash` -> `gemini-4-hydrogen`)
   * Tier 2 subagents/reviewers (`subagents/go-engineer.agent.md`, `review/security-lens-reviewer.agent.md`, `review/security-reviewer.agent.md`: `claude-sonnet-5.5`/`anthropic`/`default` -> `mai-code-2`/`microsoft`/`long_context`)
   * Tier 3 security sentinels (`review/security-sentinel.agent.md`, `subagents/security-sentinel.agent.md`: `claude-sonnet-5.5`/`anthropic`/`xhigh`/`default` -> `gemini-4-carbon`/`google`/`high`/`long_context`)
   * Tier 1 subagents/reviewers (12 files: `research/learnings-researcher.agent.md`, `subagents/prompt-builder.agent.md`, `review/agent-native-parity-reviewer.agent.md`, `review/architecture-strategist.agent.md`, `review/concurrency-reviewer.agent.md`, `review/constitution-reviewer.agent.md`, `review/correctness-reviewer.agent.md`, `review/go-quality-reviewer.agent.md`, `review/maintainability-reviewer.agent.md`, `review/schema-cli-docs-coupling-reviewer.agent.md`, `review/scope-boundary-auditor.agent.md`, `review/template-integrity-reviewer.agent.md`: `reasoning_effort: "xhigh"` -> `"high"`, `context_tier: "default"` -> `"long_context"`)
3. **Manifest `variables_used`, Checksums, and Pinned Drift Records (`.gsd/harness-manifest.yaml`)**: `variables_used` (`MODEL_ROUTING_*`, `ANCHOR_REVIEW_*`, `ALT_DOC_REVIEW_*`, `ESCALATION_*`, `STAGE_ESCALATION_*`, `SHIP_ESCALATION_*`), `config_hash`, `tuned_at`, and LF-normalized SHA-256 `checksum`s for all 17 modified manifest-tracked `.github/**` artifacts must be updated while strictly preserving `drift_allowed: true` / `drift_reason` records (`"Do not auto-revert."`, stash citations, and historical notes pinned by `TestUSR6_HarnessManifestDriftRecords`, `TestUCXS17_RefreshedDriftRecords`, `TestU19R3_ManifestBudgetReconciliation`, and `TestU12_HarnessManifestNamespaceReconciliation`).

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Update `_orchestrator.agent.md` and `_stage.agent.md` model-routing frontmatter and `_orchestrator.agent.md` routing table/prose without modifying `tools:` or workflow rules | `BE9AD4F1`, `A991B4AC` / `104-DL` | Unit U1 (`219.001-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.001-T` + `TestU12_GSDModelRoutingManifestReconciliation` |
| R2: Update `review/adversarial-review.agent.md` and `instructions/adversarial-review.instructions.md` for Tier 3 `gemini-4-carbon`, Tier 2 `mai-code-2`, and Anchor Reviewer `gpt-6.2-sol` | `BE9AD4F1` / `104-DL` | Unit U2 (`219.002-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.002-T` + `TestU12_GSDModelRoutingManifestReconciliation` |
| R3: Update `skills/doc-review/SKILL.md` (`gemini-4-hydrogen`) and `subagents/go-engineer.agent.md` (Tier 2 `mai-code-2`/`microsoft`/`long_context`) | `A991B4AC`, `BE9AD4F1` / `104-DL` | Unit U3 (`219.003-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.003-T` + `TestU12_GSDModelRoutingManifestReconciliation` |
| R4: Update Tier 2 reviewers `security-lens-reviewer.agent.md` and `security-reviewer.agent.md` (`mai-code-2`/`microsoft`/`high`/`long_context`) | `BE9AD4F1` / `104-DL` | Unit U4 (`219.004-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.004-T` |
| R5: Update Tier 3 sentinels `review/security-sentinel.agent.md` and `subagents/security-sentinel.agent.md` (`gemini-4-carbon`/`google`/`high`/`long_context`) | `BE9AD4F1` / `104-DL` | Unit U5 (`219.005-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.005-T` |
| R6–R11: Update all 12 Tier 1 subagents and reviewers (`claude-haiku-5.5`/`anthropic`/`high`/`long_context`) in six 2-file units | `BE9AD4F1` / `104-DL` | Units U6–U11 (`219.006-T`..`219.011-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:219.006-T`..`219.011-T` |
| R12: Reconcile `.gsd/harness-manifest.yaml` (`config_hash`, `variables_used`, `tuned_at`, and 17 LF-normalized SHA-256 `checksum`s while preserving all `drift_allowed`/`drift_reason`/historical notes) and enforce full cross-surface parity in `tests/integration/gsd_model_routing_reconciliation_test.go` | `BE9AD4F1`, `A991B4AC` / `104-DL` | Unit U12 (`219.012-T`, Wave 1 — last in manifest order) | `TestU12_GSDModelRoutingManifestReconciliation`, `TestU12_HarnessManifestNamespaceReconciliation`, `TestUSR6_HarnessManifestDriftRecords`, `gsd verify-workspace --workspace . --json` |

## Closed P-002.1 Harness-Exempt Set

The closed `harness-exempt` (`docs-only`) set for Feature `219-F` / Shipment `220-S` is:

* `219.001-T` (`U1`), `219.002-T` (`U2`), `219.003-T` (`U3`), `219.004-T` (`U4`), `219.005-T` (`U5`), `219.006-T` (`U6`), `219.007-T` (`U7`), `219.008-T` (`U8`), `219.009-T` (`U9`), `219.010-T` (`U10`), `219.011-T` (`U11`).

`219.012-T` (`U12`) is `harness-ready` (`test-first`).

## Implementation Units

### Unit U1: Update `_orchestrator.agent.md` and `_stage.agent.md` model routing (`219.001-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `S` / `low`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `.github/agents/_orchestrator.agent.md`
  * `.github/agents/_stage.agent.md`
* **Changes needed**:
  1. `.github/agents/_orchestrator.agent.md`: Update frontmatter `model_provider: "google"`, `model_family: "gemini-4-carbon"` (keeping `reasoning_effort: "medium"`, `context_tier: "long_context"`, and `tools:` unchanged). In the `## Model Routing` section, update the role table, `Configuration example` YAML snippet (updating `tier2` to `mai-code-2` / `microsoft` / `high` / `context_tier: long_context`, `tier3` to `gemini-4-carbon` / `google` / `high` / `context_tier: long_context`, `orchestrator` and `stage` to `gemini-4-carbon` / `google` / `medium` / `long_context`, and `ship` to `gemini-4-hydrogen` / `google` / `high` / `long_context`), and the `Cross-provider routing` paragraph to reflect the 4-provider split (`google` for Orchestrator/Stage/Ship/Tier 3/alt_doc_review, `microsoft` for Tier 2/Auto-MergeInstall/Auto-Tune, `anthropic` for Tier 1, and `openai` for escalation/anchor_review — without copying the stale inline comment from `.gsd/config.yaml`).
  2. `.github/agents/_stage.agent.md`: Update frontmatter `reasoning_effort: "medium"` and `model_family: "gemini-4-carbon"` (keeping `model_provider: "google"`, `context_tier: "long_context"`, and `tools:` unchanged).
* **Verification (`P-002.3`)**: Assert `_orchestrator.agent.md` contains `model_family: "gemini-4-carbon"`, `model_provider: "google"`, and `mai-code-2` and contains none of `claude-sonnet-5.5`, `gemini-4-argon`, or `context_tier: default`; assert `_stage.agent.md` contains `model_family: "gemini-4-carbon"` and `reasoning_effort: "medium"` and does not contain `gemini-4-argon`; run `go run ./cmd/backlogit docs lint --format json`; emit `EXEMPT_VERIFY_OK:219.001-T`.

### Unit U2: Update `adversarial-review` agent and instructions (`219.002-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `S` / `low`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `.github/agents/review/adversarial-review.agent.md`
  * `.github/instructions/adversarial-review.instructions.md`
* **Changes needed**:
  1. `.github/agents/review/adversarial-review.agent.md`: Update frontmatter `reasoning_effort: "high"`, `model_provider: "google"`, `model_family: "gemini-4-carbon"`, `context_tier: "long_context"`, `anchor_review_family: "gpt-6.2-sol"` (preserving `tools:` `backlogit/*`). In prose and the Reviewer Route table, replace both `gpt-6.1-sol` and Title-Case `GPT-6.1 Sol` with `gpt-6.2-sol` / `GPT-6.2 Sol`, Reviewer-B Tier 2 `claude-sonnet-5.5` with `mai-code-2`, and Reviewer-C Tier 3 `claude-sonnet-5.5` with `gemini-4-carbon`.
  2. `.github/instructions/adversarial-review.instructions.md`: Update `gpt-6.1-sol` to `gpt-6.2-sol` and `GPT-6.1 Sol` to `GPT-6.2 Sol`.
* **Verification (`P-002.3`)**: Assert both files reference `gpt-6.2-sol` and `GPT-6.2 Sol` and match neither `(?i)gpt-6\.1` nor `claude-sonnet-5.5`, assert `adversarial-review.agent.md` contains `mai-code-2` and `gemini-4-carbon`, run `go run ./cmd/backlogit docs lint --format json`, and emit `EXEMPT_VERIFY_OK:219.002-T`.

### Unit U3: Update `doc-review` skill and `go-engineer` subagent (`219.003-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `XS` / `low`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `.github/skills/doc-review/SKILL.md`
  * `.github/agents/subagents/go-engineer.agent.md`
* **Changes needed**:
  1. `.github/skills/doc-review/SKILL.md`: Replace both occurrences of `gemini-3.8-flash` with `gemini-4-hydrogen` (`ALT_DOC_REVIEW_FAMILY`).
  2. `.github/agents/subagents/go-engineer.agent.md`: Update frontmatter `model_provider: "microsoft"`, `model_family: "mai-code-2"`, `context_tier: "long_context"` (preserving `reasoning_effort: "high"` and `tools:`).
* **Verification (`P-002.3`)**: Assert `doc-review/SKILL.md` contains `gemini-4-hydrogen` and not `gemini-3.8-flash`, assert `go-engineer.agent.md` contains `model_family: "mai-code-2"`, `model_provider: "microsoft"`, and `context_tier: "long_context"` and not `claude-sonnet-5.5`, run `go run ./cmd/backlogit docs lint --format json`, and emit `EXEMPT_VERIFY_OK:219.003-T`.

### Unit U4: Update Tier 2 security reviewers (`219.004-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `XS` / `trivial`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `.github/agents/review/security-lens-reviewer.agent.md`
  * `.github/agents/review/security-reviewer.agent.md`
* **Changes needed**: Update frontmatter on both files to `model_provider: "microsoft"`, `model_family: "mai-code-2"`, `reasoning_effort: "high"`, `context_tier: "long_context"` (matching `templates/agents/review/*.tmpl` `TIER_2_*` variables).
* **Verification (`P-002.3`)**: Assert both files contain `model_family: "mai-code-2"`, `model_provider: "microsoft"`, and `context_tier: "long_context"` and not `claude-sonnet-5.5`, run `go run ./cmd/backlogit docs lint --format json`, and emit `EXEMPT_VERIFY_OK:219.004-T`.

### Unit U5: Update Tier 3 security sentinels (`219.005-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `XS` / `trivial`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `.github/agents/review/security-sentinel.agent.md`
  * `.github/agents/subagents/security-sentinel.agent.md`
* **Changes needed**: Update frontmatter on both files to `reasoning_effort: "high"`, `model_provider: "google"`, `model_family: "gemini-4-carbon"`, `context_tier: "long_context"` (preserving `tools:` `backlogit/*`).
* **Verification (`P-002.3`)**: Assert both files contain `model_family: "gemini-4-carbon"`, `model_provider: "google"`, `reasoning_effort: "high"`, and `context_tier: "long_context"` and not `claude-sonnet-5.5`, run `go run ./cmd/backlogit docs lint --format json`, and emit `EXEMPT_VERIFY_OK:219.005-T`.

### Units U6–U11: Update Tier 1 research, subagent, and review personas (`219.006-T`..`219.011-T`, Wave 1)

Each unit is `Domain: docs` (`P-002.1` `docs-only`; 2 files, 0 Go files, `Size: XS`, `Complexity: trivial`, `Posture: docs-only`), updating frontmatter `reasoning_effort: "high"` (from `"xhigh"`) and `context_tier: "long_context"` (from `"default"`) while keeping `model_provider: "anthropic"`, `model_family: "claude-haiku-5.5"`, and `tools:` unchanged:

* **Unit U6 (`219.006-T`)**: `.github/agents/research/learnings-researcher.agent.md`, `.github/agents/subagents/prompt-builder.agent.md` (`EXEMPT_VERIFY_OK:219.006-T`)
* **Unit U7 (`219.007-T`)**: `.github/agents/review/agent-native-parity-reviewer.agent.md`, `.github/agents/review/architecture-strategist.agent.md` (`EXEMPT_VERIFY_OK:219.007-T`)
* **Unit U8 (`219.008-T`)**: `.github/agents/review/concurrency-reviewer.agent.md`, `.github/agents/review/constitution-reviewer.agent.md` (`EXEMPT_VERIFY_OK:219.008-T`)
* **Unit U9 (`219.009-T`)**: `.github/agents/review/correctness-reviewer.agent.md`, `.github/agents/review/go-quality-reviewer.agent.md` (`EXEMPT_VERIFY_OK:219.009-T`)
* **Unit U10 (`219.010-T`)**: `.github/agents/review/maintainability-reviewer.agent.md`, `.github/agents/review/schema-cli-docs-coupling-reviewer.agent.md` (`EXEMPT_VERIFY_OK:219.010-T`)
* **Unit U11 (`219.011-T`)**: `.github/agents/review/scope-boundary-auditor.agent.md`, `.github/agents/review/template-integrity-reviewer.agent.md` (`EXEMPT_VERIFY_OK:219.011-T`)

Each unit's `exempt_verification_command` asserts both target files contain `reasoning_effort: "high"` and `context_tier: "long_context"` and contain neither `reasoning_effort: "xhigh"` nor `context_tier: "default"`, runs `go run ./cmd/backlogit docs lint --format json`, and prints its task-unique `EXEMPT_VERIFY_OK:<task_id>` marker.

### Unit U12: Reconcile `.gsd/harness-manifest.yaml` and enforce model-routing contract parity (`219.012-T`, Wave 1 — last in manifest order)

* **Domain**: `config` / `tests` (`harness-ready`, `test-first`; 2 files, 1 test function, 3 test scenarios)
* **Size / Complexity**: `M` / `medium`
* **Execution posture**: `test-first` (`P-002.1` compile PASS + runtime assertion RED at Wave 1 entry before `.gsd/harness-manifest.yaml` is edited; executed 12th in Wave 1 manifest order after `219.001-T`..`219.011-T` land all 22 `.github/**` file edits)
* **Affected files (2)**:
  * `.gsd/harness-manifest.yaml`
  * `tests/integration/gsd_model_routing_reconciliation_test.go`
* **Changes needed**:
  1. In `.gsd/harness-manifest.yaml` (written with LF `\n` line endings and zero duplicate YAML keys):
     * Confirm as an implementer step-ordering precondition (never inside the durable Go test) that `219.001-T`..`219.011-T` are `done`, update `config_hash` to match the LF-normalized SHA-256 of `.gsd/config.yaml`, and update `tuned_at` to a valid RFC3339 UTC timestamp.
     * Update `variables_used`:
       - `MODEL_ROUTING_TIER1: "claude-haiku-5.5"`
       - `MODEL_ROUTING_TIER2: "mai-code-2"`
       - `MODEL_ROUTING_TIER3: "gemini-4-carbon"`
       - `ANCHOR_REVIEW_PROVIDER: "openai"`, `ANCHOR_REVIEW_FAMILY: "gpt-6.2-sol"`, `ANCHOR_REVIEW_REASONING_EFFORT: "high"`
       - `ALT_DOC_REVIEW_PROVIDER: "google"`, `ALT_DOC_REVIEW_FAMILY: "gemini-4-hydrogen"`
       - `ESCALATION_FAMILY: "gemini-4-carbon"`, `ESCALATION_PROVIDER: "google"`, `ESCALATION_REASONING_EFFORT: "high"`
       - `STAGE_ESCALATION_FAMILY: "gpt-6.2-sol"`, `STAGE_ESCALATION_PROVIDER: "openai"`, `STAGE_ESCALATION_REASONING_EFFORT: "high"`, `STAGE_ESCALATION_CONTEXT_TIER: "long_context"`
       - `SHIP_ESCALATION_FAMILY: "gpt-6.2-sol"`, `SHIP_ESCALATION_PROVIDER: "openai"`, `SHIP_ESCALATION_REASONING_EFFORT: "medium"`, `SHIP_ESCALATION_CONTEXT_TIER: "long_context"`
     * Rotate LF-normalized SHA-256 `checksum`s for every manifest-tracked artifact modified by `U1`–`U11` (all 17 manifest-tracked `.github/**` files: `_orchestrator.agent.md`, `_stage.agent.md`, `review/adversarial-review.agent.md`, `instructions/adversarial-review.instructions.md`, `skills/doc-review/SKILL.md`, `subagents/go-engineer.agent.md`, `review/security-lens-reviewer.agent.md`, `review/security-reviewer.agent.md`, `review/security-sentinel.agent.md`, `research/learnings-researcher.agent.md`, `subagents/prompt-builder.agent.md`, `review/agent-native-parity-reviewer.agent.md`, `review/concurrency-reviewer.agent.md`, `review/correctness-reviewer.agent.md`, `review/maintainability-reviewer.agent.md`, `review/schema-cli-docs-coupling-reviewer.agent.md`, `review/template-integrity-reviewer.agent.md`).
     * **Strictly preserve** every existing `drift_allowed: true`, `drift_reason` (including `"Do not auto-revert."`, `227A2930`, `B83081F5`, `75E02C17`..`F05661B1`, and `073-DL rev22`), and historical `note` (`"DOCS_PLANS corrected to docs/exec-plans per workspace config."`).
  2. In `tests/integration/gsd_model_routing_reconciliation_test.go` (`package integration_test`):
     * Keep all 3 scenarios as `t.Run` subtests inside the **single** top-level `TestU12_GSDModelRoutingManifestReconciliation` function (never split into separate top-level `Test*` functions so `redhonesty` against `origin/main` cannot hit `RejectAlreadyPasses`), reuse package-scoped `testRepoRoot`, `readGovernedSurface`, `lfSHA256Hex`, and `checkUniqueYAMLMappingKeys` helpers, decode `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, and agent frontmatter into typed local Go structs alongside recursive `yaml.Node` `MappingNode` duplicate-key checks (+ an in-test negative duplicate-key assertion), and never read `.backlogit/queue/` status inside the test so it remains green after `ShipShipment` archives `219.*-T`:
       1. `config_and_manifest_variables_reconciled`: Asserts recursive `yaml.Node` key uniqueness on `.gsd/config.yaml` and `.gsd/harness-manifest.yaml`, `manifest.ConfigHash != "2b114a784fdf277e9552280767f798d706695eb8d5e191573486409011096157"`, `manifest.ConfigHash == lfSHA256Hex(.gsd/config.yaml)`, and all `variables_used` model-routing/escalation/review scalars (`mai-code-2`, `gemini-4-carbon`, `gpt-6.2-sol`, `gemini-4-hydrogen`) match `.gsd/config.yaml`.
       2. `tiered_agents_doc_review_and_adversarial_review_aligned`: Asserts all 21 tiered `.github/agents/**` frontmatters, `_orchestrator.agent.md` routing prose/YAML snippet (no `context_tier: default`), `adversarial-review` prose/instructions (no case-insensitive `gpt-6.1`), and `doc-review/SKILL.md` match `.gsd/config.yaml` with zero stale `gemini-4-argon`, `claude-sonnet-5.5`, `gpt-6.1-sol`, or `gemini-3.8-flash` tokens in active frontmatter/`variables_used`/live routing sections (excluding `tuning_history` and preserved `drift_reason` fields), while confirming read-only leaf personas retain explicit `read, search` `tools:` and `_orchestrator`/`_stage` retain enumerated `kbdb/<verb>` entries with no `kbdb/*` wildcard.
       3. `manifest_checksums_and_drift_records_intact`: Asserts every entry in `.gsd/harness-manifest.yaml` `artifacts` has a valid 64-hex LF-normalized SHA-256 matching its on-disk file bytes and preserves `drift_allowed` / `drift_reason` invariants.
  * **Scoped verification command (`P-002.6` `harness_cmd`)**: `go test -timeout=30m -count=1 -v -run '^TestU12_GSDModelRoutingManifestReconciliation$' ./tests/integration`
  * **`green_regression_cmds` (`P-002.6` `green-regression-contract`)**: `go test -timeout=30m -count=1 -v -run '^(TestU12_HarnessManifestNamespaceReconciliation|TestUSR6_HarnessManifestDriftRecords|TestUCXS17_RefreshedDriftRecords|TestU19R3_ManifestBudgetReconciliation|TestWorkspaceAgentsGrantFullBacklogitToolSurface|TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery)$' ./tests/integration`

## Dependency Graph

All 12 tasks (`219.001-T`..`219.012-T`) have zero inter-task `blocks` edges so they co-schedule in **Wave 1 (`ready_1`)**, ordered in `220-S`'s manifest as `219.001-T`..`219.011-T` followed by `219.012-T`:

```text
Wave 1 (ready_1, manifest execution order):
  219.001-T (U1) -> 219.002-T (U2) -> ... -> 219.011-T (U11) -> 219.012-T (U12: .gsd/harness-manifest.yaml + contract test)
  --> Wave 1 Step 4.6 Convergence: TestU12 + TestUSR6 + TestUCXS17 + TestU19R3 + gsd verify-workspace PASS
```

## Decisions and Rationale

* **Single-wave co-scheduling (`U1`–`U12` in Wave 1)**: Because Ship commits carried-forward `.gsd/config.yaml` at `P-011` pre-claim before Wave 1 begins, `TestU12_HarnessManifestNamespaceReconciliation` requires `.gsd/harness-manifest.yaml` `config_hash` to be updated before Wave 1 Step 4.6 runs `go test -timeout=30m ./...`. Co-scheduling `U1`–`U11` (`docs-only`) ahead of `U12` (`harness-ready`) in Wave 1 manifest order follows the proven `214-F` (`215-S`) precedent.
* **Strict 2-file task width**: Splitting the 22 `.github/**` files into 11 two-file `docs-only` tasks plus 1 two-file `harness-ready` manifest/test task keeps every task strictly `< 3 files` while staying well within the 20-task session ceiling (`12 < 20`).

## Risks and Caveats

* **Windows CRLF and UTF-8 BOM hazard**: `.gsd/harness-manifest.yaml` and `.github/**` files must be written as BOM-free UTF-8 (`[System.Text.UTF8Encoding]::new($false)`, never Windows PowerShell `Set-Content -Encoding utf8`) with LF (`\n`) endings so `gsd gate line-endings --json` and LF-normalized SHA-256 checksums agree on disk and in Git.
* **Foreign worktree paths & baseline refresh**: Never stage, commit, or list `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, or `.kbdb/` in any task `## File Budget`; after committing `.gsd/config.yaml` via `P-011` carry-forward on the `220-S` branch, refresh `.gsd/worktree-baseline.json` on disk via `gsd gate pipeline-topology --mode agent --shipment 220-S --phase lifecycle --capture-baseline --json` per `/gsd-dark-mode` Step 3. If `.copilot/session-state/` Go caches exist on the host before `222-S` lands, `079-DL`'s bounded `gofmt -l cmd internal tests scripts` + diff check applies.

## Constitution Check

* **I. Safety-First Go**: `pass` — Integration contract test in `tests/integration/gsd_model_routing_reconciliation_test.go` uses standard Go 1.24 + `testify` + `gopkg.in/yaml.v3` with zero `unsafe`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — `U12` scaffolds a compiling, failing `TestU12_GSDModelRoutingManifestReconciliation` before `.gsd/harness-manifest.yaml` is modified, and `U1`–`U11` carry complete fail-before-deliverable `P-002.1`/`P-002.3` `docs-only` contracts.
* **III. Workspace Isolation and Security Boundaries**: `pass` — All reads/writes stay within workspace `.gsd/`, `.github/`, and `tests/integration/`, with zero secrets committed.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — Zero operations escape the repository working tree.
* **V. Structured Observability**: `pass` — Every task emits named `--- PASS:` or `EXEMPT_VERIFY_OK:<task_id>` output and `gsd verify-workspace --workspace . --json` evidence.
* **VI. Single Responsibility**: `pass` — Zero new dependencies introduced; every task is strictly 2 files in a single domain.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — All `exempt_verification_command` probes are strictly read-only (`Get-Content` + `go run ./cmd/backlogit docs lint`).
* **VIII. Explicit Safety Modes**: `pass` — Careful preservation of workspace-specific `drift_reason` and `tools:` contracts enforced by automated guards.
* **IX. Git-Friendly Persistence**: `pass` — LF-normalized Markdown and YAML edits with unique mapping keys.
* **X. Agent Context Efficiency**: `pass` — Targeted YAML/frontmatter edits rather than full template regeneration.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — Shipment `220-S` merges via `--merge` commit only (`P-009`).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Updates cross-agent model-routing governance across `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, and 22 `.github/**` agent/skill/instruction definitions.
* **security, auth, permission, or compliance-sensitive behavior**: `absent` — No auth/permission/MCP boundary changes, though agent `tools:` wildcards must be preserved unchanged.
* **migration, backfill, destructive data/config action, or irreversible step**: `present` — `P-011` carry-forward of `.gsd/config.yaml` and `.gsd/harness-manifest.yaml` checksum/drift-record rotation.
* **external integration, operator checkpoint, or external dependency**: `absent` — Uses already-configured model identifiers in `.gsd/config.yaml`.
* **high runtime, rollout, or rollback risk**: `present` — Incorrect manifest checksums or wave ordering would block `gsd verify-workspace` or `TestU12` at Wave 1 convergence.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime/harness surfaces**: `.gsd/config.yaml` (via `P-011` carry-forward), `.gsd/harness-manifest.yaml`, `.github/agents/**` (20 files), `.github/skills/doc-review/SKILL.md`, and `.github/instructions/adversarial-review.instructions.md`.
* **Required verification before PR readiness**:
  1. `gsd routing resolve --workspace . --site stage --kind route --json`, `gsd routing resolve --workspace . --site ship --kind route --json`, `gsd routing resolve --workspace . --site orchestrator --kind route --json`, `gsd routing resolve --workspace . --site stage --kind escalation --json`, and `gsd routing resolve --workspace . --site ship --kind escalation --json` all exit `0` with `errors: []`.
  2. `gsd verify-workspace --workspace . --json` reports `0` checksum mismatches, `0` blockers, and `strict_schema_blockers: []`.
  3. Unfiltered `go test -timeout=30m ./...` (including `TestU12_GSDModelRoutingManifestReconciliation`, `TestU12_HarnessManifestNamespaceReconciliation`, `TestUSR6_HarnessManifestDriftRecords`, `TestUCXS17_RefreshedDriftRecords`, `TestU19R3_ManifestBudgetReconciliation`, `TestWorkspaceAgentsGrantFullBacklogitToolSurface`, and `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery`), `scripts/md-lint.ps1`, and `go run ./cmd/backlogit docs lint` all pass with `0` violations.
* **Operational closure**: `docs/closure/220-S-219-F-post-merge-closure.md` recording healthy routing/manifest verification signals, `_Ship` owner, 7-day validation window, and scoped revert procedure preserving `.backlogit/archive/`.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by cross-agent routing contract updates, `P-011` `.gsd/config.yaml` carry-forward coupling, and `.gsd/harness-manifest.yaml` checksum/drift-record invariants.
* **Protected invariants**:
  * **I1 (`P-011` carry-forward, foreign-path exclusion, and baseline refresh)**: Ship commits carried-forward `.gsd/config.yaml` via literal pathspec on the `220-S` branch before Wave 1 claim, refreshes `.gsd/worktree-baseline.json` on disk via `--capture-baseline` per `/gsd-dark-mode` Step 3, and NEVER stages, commits, or lists `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, or `.kbdb/` in any task `## File Budget`.
  * **I2 (Preservation of workspace `tools:` wildcards & agent governance prose)**: Units `U1`–`U11` edit ONLY the declared frontmatter routing keys (`model_family`, `model_provider`, `reasoning_effort`, `context_tier`, `anchor_review_family`) and model-routing prose sections (`_orchestrator.agent.md` `Model Routing`, `adversarial-review` routing prose, `doc-review` `Alternate Model Support`), leaving `tools:` (`backlogit/*`), role boundaries, served-root attestation, and recovery protocols byte-identical.
  * **I3 (Preservation of `drift_allowed`, `drift_reason`, and historical notes in `.gsd/harness-manifest.yaml`)**: Unit `U12` updates `config_hash`, `tuned_at`, `variables_used` routing keys, and the 17 modified `.github/**` `checksum` values over LF-normalized bytes without adding duplicate YAML keys or altering any `drift_allowed: true`, `drift_reason` (`"Do not auto-revert."`, `227A2930`, `B83081F5`, `75E02C17`..`F05661B1`, `073-DL rev22`), or historical `note` (`"DOCS_PLANS corrected to docs/exec-plans per workspace config."`).
  * **I4 (Wave 1 co-scheduling order)**: All 12 tasks (`219.001-T`..`219.012-T`) execute in Wave 1 in exact manifest order (`219.001-T`..`219.011-T` before `219.012-T`) so `TestU12_HarnessManifestNamespaceReconciliation` and `TestU12_GSDModelRoutingManifestReconciliation` converge green at Wave 1 Step 4.6.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/prompt-builder.instructions.md`, `.github/instructions/markdown.instructions.md`, `.github/policies/workflow-policies.md`
  * `docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md`
  * `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md`
  * `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`
  * `docs/compound/2026-10-17-discovery-vs-overlay-predicates.md`
  * `docs/compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md`
* **Risky actions classification**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Carry forward dirty `.gsd/config.yaml` onto `220-S` shipment branch (`P-011`) | `.gsd/config.yaml` | `low` (explicit `P-011` carry-forward) | Commit via literal pathspec on shipment branch only; exclude `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, `.kbdb/` | `.gsd/config.yaml` committed cleanly ahead of Wave 1 |
| Update model-routing frontmatter and prose across 22 `.github/**` artifacts (`U1`–`U11`) | `.github/agents/**`, `.github/skills/doc-review/SKILL.md`, `.github/instructions/adversarial-review.instructions.md` | `moderate` (shared harness agent/skill definitions) | Edit only routing keys/sections; enforce read-only P-002.3 content + `docs lint` gates and `TestWorkspaceAgentsGrantFullBacklogitToolSurface` | All 22 `.github/**` artifacts match `.gsd/config.yaml` with zero contract regressions |
| Reconcile `.gsd/harness-manifest.yaml` `config_hash`, `variables_used`, and 17 LF checksums (`U12`) | `.gsd/harness-manifest.yaml`, `tests/integration/gsd_model_routing_reconciliation_test.go` | `moderate` (manifest integrity & drift-record preservation) | Reject duplicate YAML keys; compute LF SHA-256; verify `TestU12`, `TestUSR6`, `TestUCXS17`, `TestU19R3`, and `gsd verify-workspace` | `0` checksum mismatches, `0` schema blockers, all integration tests green |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode:` and `decision:` markers, cover all 7 required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) plus `rubber-duck` adversarial critique (`/gsd-dark-mode` Step 2.1), and achieve `decision: PASS` before harvest.
* **Unresolved operator decisions**: None.

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `TOOL_OK: reviewer-subagent-dispatch`
* `decision: FAIL`
* **Gate rationale (Attempt 1 — superseded by Attempt 2)**: Multi-persona + `rubber-duck` review (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`, and `rubber-duck`) identified two `P1` blockers (`U12` bundling pre-existing green suites into `harness_cmd` instead of `green_regression_cmds` under `P-002.6`/`P-004`, and missing `--site stage`/`--site ship` flags on `gsd routing resolve --kind escalation` plus `.gsd/worktree-baseline.json` `--capture-baseline` wording in `I1`) and three `P2` improvements (`_orchestrator.agent.md` `context_tier: default`, single-function `t.Run` `redhonesty` guard, and recursive `yaml.Node` duplicate-key check), all remediated in Attempt 2 above.

<!-- plan-review-attempt: 2 -->

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 2)**: Dispatched all 7 required `plan-review` personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, and `Security Lens Reviewer`) plus `rubber-duck` adversarial critique (`/gsd-dark-mode` Step 2.1). Seven subagents returned structured `PASS` verdicts (`0 P0, 0 P1, 0 P2`); `Agent-Native Parity Reviewer` returned an empty stream token on Attempt 2, so per `plan-review/SKILL.md` Terminal States a complete sequential rubric pass was executed across all 7 personas + `rubber-duck` and recorded under `single-agent-declared-degradation`. Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by `## Plan Hardening` (`I1`–`I4` and `ProposedAction` / `ActionRisk` table). `## Constitution Check` concludes with literal `Constitution Check: pass`. Merged findings: `0 P0, 0 P1, 0 P2, 0 P3` (all Attempt 2 P3 polish notes folded into `U12` above before harvest).
* **Per-persona findings summary**:
  * **Constitution Reviewer**: `PASS` (`0` findings). All Principles I–XI, `< 3 files` / `< 5 functions` / `< 4 scenarios` granularity, `12 < 20` task budget, and `P-002.6` `harness_cmd` vs `green_regression_cmds` rules satisfied.
  * **Go Reviewer**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 notes on `U12` queue-state decoupling and reusing package helpers incorporated).
  * **Scope Boundary Auditor**: `PASS` (`0` findings). Exact 22 `.github/**` files + `.gsd/harness-manifest.yaml` + integration test, with foreign paths excluded.
  * **Learnings Researcher**: `PASS` (`0 P0, 0 P1, 0 P2`, `confidence: high`). Recursive `yaml.Node` duplicate-key checks and BOM-free UTF-8 LF writes verified.
  * **Architecture Strategist**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 note on dynamic checksum parity and `U12` step-1 precondition incorporated).
  * **Agent-Native Parity Reviewer** (complete rubric pass): `PASS` (`0` findings). Verified `_orchestrator.agent.md` `context_tier: long_context` and 4-provider prose, `doc-review` `gemini-4-hydrogen`, `adversarial-review` `gpt-6.2-sol`, and 21 tiered agent frontmatters match `.gsd/config.yaml` and `.gsd/harness-manifest.yaml`.
  * **Security Lens Reviewer**: `PASS` (`0` findings). Least-privilege `tools:` invariants, `P-002.5` read-only probes, and `P-011` foreign-path exclusions verified.
  * **`rubber-duck`**: `PASS` (`0 P0, 0 P1, 0 P2`; P3 `U12` queue-archive note incorporated).
