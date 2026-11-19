---
title: "Harness Tuning Report — 2026-10-24"
description: "Auto-Tune 1.6.0 drift pass: schema-contract upgrades, skill frontmatter conformity, model-routing and context_tier propagation, targeted-check fixes, and learning-driven closure guidance"
---

# Harness Tuning Report — 2026-10-24

* **autoharness**: 1.6.0 (`C:\Python\Python315\Lib\site-packages\autoharness\data`)
* **Branch**: `chore/autoharness-tune-2026-10-24` (never commit or push tune output directly to `main`)
* **Mode**: interactive-equivalent (autopilot); surgical updates preserving workspace-specific contracts and Go test suite invariants
* **Backups**: `.autoharness/backups/2026-10-24/` (44 artifacts backed up prior to modification)

## Drift Summary

| Category | Detected | Applied | Deferred / Open | Notes |
|---|---|---|---|---|
| Breaking (P0) | 0 | 0 | 0 | No missing scripts, instructions, or broken references |
| Degrading (P1) | 12 | 10 | 2 | Applied: 3 schema-contract upgrades (`manifest`, `config`, `profile`), 14 skill `name:` frontmatter additions (`193-F`), 21-agent model-routing + `context_tier` propagation (`194-F`/`208-F`), 3 targeted-check fixes (`_orchestrator`, `workflow-policies.md`, `_ship`), 1 portability fix (`_orchestrator`), 1 learning-driven closure frontmatter contract. Deferred: `.backlogit/` legacy root (TUNE-011), `start.sh` `ambiguous` (TUNE-012) |
| Growth (P2) | 2 | 1 | 1 | Applied: 1.6.0 threadless Copilot review-body findings (`github-pr-automation`, `P-018`), UTC timestamp discipline (`output-timestamps`), P-012 surface-choice & `dispose_shipment` (`backlog-integration`). Deferred: 3 workspace-authored custom agents (`mcp-protocol-reviewer`, `sqlite-reviewer`, `go-mcp-expert`, TUNE-013) |
| Cosmetic (P3) | 1 | 1 | 0 | Applied: escaped pipe characters in `workflow-policies.md` Amendment Log 1.19.0 row |

## Composition and Contracts

* **Installed primary stack pack**: `mcp-server` (current: `mcp-server`)
* **Installed stack packs**: `["mcp-server", "cli-tool"]` (current: `["mcp-server", "cli-tool"]`)
* **Installed layers**: `["foundation", "instructions", "workflow", "review", "runtime", "backlog", "knowledge", "overlays"]` (recommended: unchanged)
* **Preset**: `full`
* **Capability packs**: `backlogit`, `strict-safety`, `agent-engram`, `adversarial-review`, `release-observability`, `continuous-learning`, `graphtor-docs` (synced into `workspace-profile.yaml` `harness_recommendations.capability_packs`)
* **Schema contracts (post-tune)**:
  * `harness-manifest`: `current` (`1.0.0` → `1.1.0`)
  * `harness-config`: `current` (`1.1.0` → `1.3.0`)
  * `workspace-profile`: `current` (`1.0.0` → `1.1.0`, retired `agent_intercom` block removed)
* **New artifacts (`new_artifacts[]`)**: 0
* **Agent-identity migrations**: 0 (`_orchestrator.agent.md`, `_stage.agent.md`, `_ship.agent.md` already canonical)

## Checksum Scan

* **Checked artifacts**: 64 manifest-tracked entries (`0` missing).
* **LF-normalized SHA-256**: All 32 modified manifest-tracked artifacts had their LF-normalized SHA-256 checksums refreshed in `.autoharness/harness-manifest.yaml`, and `profile_hash` (`a46ff663...`) and `config_hash` (`f37e5b41...`) were updated to the LF-normalized SHA-256 digests of `.autoharness/workspace-profile.yaml` and `.autoharness/config.yaml`, preserving all `drift_allowed: true` flags and `drift_reason` citations required by `harness_manifest_196_drift_records_test.go`, `harness_manifest_197_drift_records_test.go`, and `harness_manifest_budget_reconciliation_test.go`.
* Note: In the JSON output (`logs/verify-workspace-post.json`), `checksum_scan` reports `user-modified` on Windows `core.autocrlf=true` checkouts because the verifier's `checksum_scan` array hashes raw working-tree bytes while the manifest stores LF-normalized digests.

## Learning Signals

* **Compound library mining (`docs/compound/`)**: 3 recent October 2026 entries (`docs/compound/2026-10-21-closure-conditions-list-predicate.md` [high], `docs/compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md` [high], `docs/compound/2026-10-21-commit-archive-record-after-governed-ship.md` [medium]) revealed a recurring post-merge closure / `pre_claim` gate pattern where `READY_WITH_CONDITIONS` closures written with prose-only conditions or `pending` `compaction_status` caused `autoharness gate pipeline-topology --phase pre_claim` to block successor shipments with `PREDECESSOR_CLOSURE_INCOMPLETE`. Promoted to P1 proposal **TUNE-010** and applied to `.github/skills/operational-closure/SKILL.md`.
* **Continuous-learning observations (`.autoharness/continuous-learning/`)**: 1 observation (`observations/2026-09-29-harness-artifact-default-accept.md`), 0 instincts (promotion threshold is `5`).
* **Policy-gap candidates**: 0 (existing `P-015` and `P-020` already govern closure and compaction; the gap was skill-level frontmatter schema documentation in `operational-closure/SKILL.md`).

## Applied Proposals

### P1 — Degrading

1. **[TUNE-001] (`source: schema-contract`, `upgrade-manifest-contract-1.0.0-to-1.1.0`)** — Upgraded `.autoharness/harness-manifest.yaml` `schema_version` from `"1.0.0"` to `"1.1.0"`, updated `autoharness_version` (`"1.5.0"` → `"1.6.0"`), and updated `autoharness_home` to `C:\Python\Python315\Lib\site-packages\autoharness\data`.
2. **[TUNE-002] (`source: schema-contract`, `upgrade-config-contract-1.1.0-to-1.2.0` / `1.3.0`)** — Upgraded `.autoharness/config.yaml` `schema_version` from `"1.1.0"` to `"1.3.0"` (closed `model_routing` schema + retired `agent-intercom` enum removal).
3. **[TUNE-003] (`source: schema-contract`, `upgrade-profile-contract-1.0.0-to-1.1.0` & `remove-retired-capability-pack`)** — Upgraded `.autoharness/workspace-profile.yaml` `schema_version` from `"1.0.0"` to `"1.1.0"`, removed the retired `agent_intercom` block, and synced `continuous-learning` and `graphtor-docs` into `harness_recommendations.capability_packs`.
4. **[TUNE-004] (`source: schema-contract`, `frontmatter-conformity` `FM_MISSING_REQUIRED` `name`)** — Added top-level `name: <skill-slug>` frontmatter to all 8 flagged managed skills (`.github/skills/brainstorm/SKILL.md`, `.github/skills/compound-refresh/SKILL.md`, `.github/skills/file-lock/SKILL.md`, `.github/skills/operational-closure/SKILL.md`, `.github/skills/plan-harden/SKILL.md`, `.github/skills/runtime-verification/SKILL.md`, `.github/skills/safety-modes/SKILL.md`, `.github/skills/skill-search/SKILL.md`) and 6 flagged workspace-authored skills (`.github/skills/build-feature/SKILL.md`, `.github/skills/compact-context/SKILL.md`, `.github/skills/compound/SKILL.md`, `.github/skills/deliberate/SKILL.md`, `.github/skills/fix-ci/SKILL.md`, `.github/skills/impl-plan/SKILL.md`). `frontmatter_conformity` now passes with 0 errors and 0 skill warnings (non-blocking `FM_UNKNOWN_KEY` info lines on `model_tier` and legacy `model_routing` remain on the 21 agent definitions because P-013.4 in `workflow-policies.md` requires `model_tier` alongside `max_subagent_tier`).
5. **[TUNE-005] (`source: schema-contract`, `model-routing` `refresh-route-frontmatter` & `ROUTE_VARIABLE_STALE`)** — Propagated the operator's updated `.autoharness/config.yaml` model routes (`tier1`: `claude-haiku-5.5`/`anthropic`/`xhigh`/`default`, `tier2`: `claude-sonnet-5.5`/`anthropic`/`high`/`default`, `tier3`: `claude-sonnet-5.5`/`anthropic`/`xhigh`/`default`, `orchestrator`: `claude-sonnet-5.5`/`anthropic`/`medium`/`long_context`, `stage`: `gemini-4-argon`/`google`/`high`/`long_context`, `ship`: `claude-sonnet-5.5`/`anthropic`/`medium`/`long_context`, `ship.escalation`: `claude-sonnet-5.5`/`anthropic`/`high`/`long_context`, `anchor_review`: `gpt-6.1-sol`/`openai`/`high`) across 21 agent files (the 12 managed subagent sites compared by `subagent_route_drift`: `learnings-researcher`, `adversarial-review`, `agent-native-parity-reviewer`, `concurrency-reviewer`, `correctness-reviewer`, `maintainability-reviewer`, `schema-cli-docs-coupling-reviewer`, `security-lens-reviewer`, `security-reviewer`, `template-integrity-reviewer`, `go-engineer`, `prompt-builder`; the 3 pipeline agents `_orchestrator`, `_stage`, `_ship`; and the 6 additional tiered review/subagent files `architecture-strategist`, `constitution-reviewer`, `go-quality-reviewer`, `scope-boundary-auditor`, `review/security-sentinel`, `subagents/security-sentinel`), plus `adversarial-review.instructions.md`, `escalation-protocol.instructions.md`, and `harness-manifest.yaml` `variables_used`. `subagent_route_drift` now passes with 0 drifted subagents and 0 `ROUTE_VARIABLE_STALE` warnings.
6. **[TUNE-006] (`source: structural-drift`, `orchestrator_invocation_routing_directive`, 194-F `context_tier`, & portability fix)** — Updated `.github/agents/_orchestrator.agent.md` Step 1 item 2 (Stage invocation) and Step 2 item 4 (Ship invocation) with the 1.6.0 CLI-first `autoharness routing resolve` call, `context_tier` override, and `ROUTING_DEGRADED: context_tier` fallback within each site's bounded window, and replaced the literal `~/.autoharness/` fallback text in Session-Start Dynamic Reload with runtime path wording (`the default global autoharness installation directory`), eliminating both P1 portability findings (`hardcoded_user_home`, `hardcoded_ah_home`). `orchestrator_invocation_routing_directive` now passes.
7. **[TUNE-007] (`source: structural-drift`, `dark_factory_visibility_contract`)** — Added the 1.6.0 self-contained **Dark Factory Visibility Protocol** event table and **Degraded visibility** paragraph to `P-017` in `.github/policies/workflow-policies.md` (alongside the 1.6.0 `P-013.5`/`P-013.6` `context_tier` and `P-018` review-body findings updates, advancing header/log to `1.33.0`). `dark_factory_visibility_contract` now passes.
8. **[TUNE-008] (`source: structural-drift`, `ship_source_artifact_cleanup`)** — Updated `.github/agents/_ship.agent.md` Step 6 item 7 (`Source artifact boundary`) to explicitly name `backlogit_stash_archive` and `backlogit_archive_item` as the operations Ship must not invoke on non-member provenance references, preserving the 155-S flat-closure boundary while satisfying `ship_source_artifact_cleanup`.
9. **[TUNE-010] (`source: learning-driven`, post-merge closure frontmatter conditions contract)** — Updated `.github/skills/operational-closure/SKILL.md` Step 3 to document the machine-readable top-level YAML frontmatter requirements (`closure_status`, `compaction_status: done|degraded`, and non-empty `conditions:` list with `id`, `satisfied: true` boolean, and `evidence` whenever `closure_status: READY_WITH_CONDITIONS` is used) enforced by `autoharness gate pipeline-topology --phase pre_claim`.
   * Evidence: `docs/compound/2026-10-21-closure-conditions-list-predicate.md`, `docs/compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md`, `docs/compound/2026-10-21-commit-archive-record-after-governed-ship.md`.

### P2 — Growth (Applied)

1. **[TUNE-009] (`source: structural-drift`, 1.6.0 instruction updates)** —
   * `.github/instructions/github-pr-automation.instructions.md`: Added §1.2, §1.3, §1.6.1, §1.7, §1.8, §1.9.1, §1.9.2, §1.9.4, and §1.9.5 rules for threadless Copilot review-body findings (`Previously missed (N)`, `Suppressed comments (N)`, `Copilot-Review-Body-Disposition: <review databaseId>`, `UNDISPOSITIONED_BODY_FINDINGS`).
   * `.github/instructions/output-timestamps.instructions.md`: Added `## UTC for Recorded Dates and Times`.
   * `.github/instructions/backlog-integration.instructions.md`: Added the P-012 surface-choice rule and the `dispose_shipment` row in the Extended Operations table.

## Deferred / Manual Review Items

| ID | Priority | Item | Reason |
|---|---|---|---|
| TUNE-011 | P1 (detect-and-report) | Legacy backlog storage root `.backlogit/` (`.backlog/` absent) | Per `tune-harness` Step 1.5, Tune must never invoke migration or rename/write the backlog storage root. Operator may review `backlogit migrate --workspace-dir --dry-run` when ready to migrate from `.backlogit/` to `.backlog/`. |
| TUNE-012 | P1 (manual-review) | `start.sh` startup-script contract (`status: ambiguous`) | Deliberate workspace customization (`drift_allowed: true` in manifest: `set -u`-safe array expansion and removal of `graphtor-docs sync`). Per Step 4.1, `ambiguous` fails closed to operator review and is never auto-applied. |
| TUNE-013 | P2 (manual-review) | Workspace-authored custom agents (`mcp-protocol-reviewer.agent.md`, `sqlite-reviewer.agent.md`, `go-mcp-expert.agent.md`) `frontmatter-conformity` | Advisory `nonconformant-workspace` proposals (`manual_review: true`); left for operator decision on whether to convert custom `user-invocable` / `model:` frontmatter to tier-routed keys. |

## Known Upstream Verifier Lag

* **`backlog_registry_cli_surface` (`dispose_shipment: must not be registered`)**: `autoharness` 1.6.0 (`backlog_surface_contract.py`) still lists `dispose_shipment` in `UNREGISTERED_OPERATIONS` ("deferred until backlogit 194-F ships"). In this `backlogit` repository, **194-F shipped on 2026-10-20** (commits `cff8d56d`, `3bcbed87`, `70174479`, `a86b8065`), and `internal/cli/registry_parity_test.go` enforces that `dispose_shipment` is registered in `.autoharness/backlog-registry.yaml`. Keeping `dispose_shipment` registered is required by the repository test suite; upstream `autoharness` should remove `dispose_shipment` from `UNREGISTERED_OPERATIONS`.

## Base-Branch Activation Note (`copilot-code-review.instructions.md`)

`.github/instructions/copilot-code-review.instructions.md` was verified healthy (`applyTo: '**'`, `excludeAgent: 'cloud-agent'`, FOCUS framing intact). Reminder: GitHub Copilot code review reads instruction files from the pull request's **base branch**, so any instruction changes take effect for Copilot code review only after the tuning pull request is merged to `main` (and the Settings → Copilot → Code review custom-instructions toggle is an advisory prerequisite).

## Verification Summary

* **`autoharness verify-workspace --json`**:
  * `strict_schema_blockers`: `0`
  * `blockers`: `0`
  * `unresolved`: `0`
  * `new_artifacts`: `0`
  * `portability_findings`: `0`
  * `schema_contracts`: all 3 `current` (`manifest` 1.1.0, `config` 1.3.0, `profile` 1.1.0)
  * `targeted_checks`: **79 / 80 PASS** (all 4 previously failing harness checks — `frontmatter_conformity`, `ship_source_artifact_cleanup`, `dark_factory_visibility_contract`, `orchestrator_invocation_routing_directive` — now PASS; the only remaining failure is the upstream `backlog_registry_cli_surface` lag for `dispose_shipment`).
* **Go test suite**: `go test ./tests/integration/... ./tests/contract/... ./internal/cli/...` — **PASS** (`tests/integration` 39.4s, `tests/contract` 35.5s, `internal/cli` 99.4s, `internal/cli/format` 1.5s).
* **Markdownlint (`P-008`)**: `npx markdownlint-cli2@0.23.1` — **0 errors** across all tracked repository and harness Markdown files (`TestMarkdownlintGate` in `tests/integration` PASS).

## Adversarial Verification

### Reviewer Dispatch

* **Anchor Reviewer (`model_routing.anchor_review`, `overlay-coherence`)**: `gpt-6.1-sol` (`openai`, `high`) — `0` findings (`TOOL_OK: anchor-review-model`)
* **Reviewer A (`template-fidelity`)**: `claude-sonnet-5.5` (`anthropic`, `xhigh`) — `1` finding (MINOR)
* **Reviewer B (`overlay-coherence`)**: `gemini-3.8-flash` (`google`) — `0` findings
* **Reviewer C (`cross-reference`)**: `claude-haiku-5.5` (`anthropic`, `xhigh`) — `5` actionable findings (all auto-remediated)

### Consensus Findings (ordered by confidence × severity)

#### HIGH Confidence

| # | Severity | Domain | File | Issue | Status |
|---|---|---|---|---|---|
| 1 | MAJOR | `cross-reference` | `.autoharness/harness-manifest.yaml` | `profile_hash` and `config_hash` carried literal `"updated"` instead of SHA-256 digests | AUTO-FIXED |
| 2 | MAJOR | `cross-reference` | `.github/agents/_orchestrator.agent.md` | Line 696 contained `~/.autoharness/`, triggering 2 P1 portability findings (`hardcoded_user_home`, `hardcoded_ah_home`) | AUTO-FIXED |
| 3 | MAJOR | `cross-reference` | `.autoharness/tuning-reports/2026-10-24-tuning-report.md` | Targeted checks count cited `75 / 76` instead of `79 / 80` and omitted explicit per-skill/per-subagent lists | AUTO-FIXED |
| 4 | MINOR | `template-fidelity` | `.github/policies/workflow-policies.md` | Amendment Log 1.19.0 row had unescaped pipes inside `` `|ready_k| > 1` `` | AUTO-FIXED |
| 5 | MINOR | `cross-reference` | `.autoharness/tuning-reports/2026-10-24-tuning-report.md` | Trailing stray characters at EOF and Drift Summary table column clarity | AUTO-FIXED |

### Auto-Remediation Summary

* Findings auto-fixed: `5`
* Findings requiring manual review: `0` (beyond the 3 pre-declared deferred items TUNE-011..TUNE-013)
* False positives dismissed: `1` (MD025 `front_matter_title` is retargeted to `^\s*_title\s*[:=]` in `.markdownlint.json`, so frontmatter `title:` plus body `# H1` is the canonical P-008 convention in this repository)

### Verification Result: PASS

## Recommendation & Next Steps

1. Review `git diff` on branch `chore/autoharness-tune-2026-10-24` and open a pull request to `main` (never commit or push tune output directly to `main`).
2. Schedule the next harness tuning run after the next major feature release or monthly.