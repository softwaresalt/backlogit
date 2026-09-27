---
title: "Harness Tuning Report — 2026-09-27"
description: "Auto-Tune 1.5.0 drift pass: lock hardening, model-routing propagation, closure-sequence fix, and deferred manual-review items"
---

# Harness Tuning Report — 2026-09-27

* **autoharness**: 1.5.0 (`C:\Python\Python314\Lib\site-packages\autoharness\data`)
* **Branch**: `chore/autoharness-tune-2026-09-27` (never commit or push tune output to `main`)
* **Mode**: interactive-equivalent (autopilot); surgical merges, no wholesale adoption of staged renders
* **Backups**: `.autoharness/backups/2026-09-27/`

## Drift Summary

| Category | Count | Notes |
|---|---|---|
| Breaking (P0) | 0 | No missing scripts, instructions, or broken references |
| Degrading (P1) | 4 | Lock scripts, model routing, closure sequence, missing registry rows |
| Growth (P2) | 4 | Constitution/spike/security-reviewer merges applied; runtime-verification/closure merge deferred |
| Cosmetic (P3) | 1 | `compact-context` generic model mention |

## Composition and Contracts

* Schema contracts (config, profile, manifest): `current`.
* Preset `full`; capability packs unchanged (backlogit, strict-safety, agent-engram, adversarial-review, release-observability).
* `new_artifacts[]`: 0. Agent-identity migrations: 0.

## Checksum Scan

* Post-tune: 79 `user-modified`, 3 `unchanged`, 0 missing.
* All 79 `user-modified` entries are CRLF false positives. The verifier hashes raw working-tree bytes, while the manifest stores LF-normalized SHA-256; re-hashing every flagged file LF-normalized matched its manifest checksum (0 real drift).

## Learning Signals

* Compound, continuous-learning, and closure mining: `learning_signals{}` empty. No learning-driven proposals and no policy-gap candidates.

## Applied Proposals

### P1 — Degrading

1. **TUNE-001 Lock hardening.** Refreshed `scripts/acquire_lock.{ps1,sh}` and `scripts/release_lock.{ps1,sh}` to the upstream capability-token plus workspace-containment model. Updated `concurrency.instructions.md`, `file-lock/SKILL.md` (H1 preserved), and the lock command lines in `_stage`/`_ship`. Exit codes verified: acquire 0, re-acquire while held 1, release without token 1, release with token 0, path escape 1 (both shells).
2. **TUNE-002 Model-routing propagation.** Applied `config.model_routing` to the frontmatter of 18 tiered agents: tier1 `gpt-6-luna`/openai/xhigh, tier2 `claude-sonnet-5`/anthropic/high, tier3 `claude-opus-5.5`/anthropic/high. Also set the anchor reviewer to `gpt-6-sol` (adversarial-review agent and instruction), set the escalation route to `claude-opus-5.5`/anthropic/xhigh (`_stage`, `_ship`, `escalation-protocol.instructions.md`), and rewrote the `_orchestrator` routing table and example. `model_tier` was preserved everywhere.
3. **TUNE-003 Ship closure sequence.** Restored the verifier-required `ship_release_closure_sequence` phrase in `_ship.agent.md`; the check now passes.
4. **TUNE-004 Registry rows.** Added 6 missing Extended Operations rows to `backlog-integration.instructions.md`, sorted to match the registry.

### P2 — Growth (applied)

1. **TUNE-005 Constitution.** Replaced "Branch per release unit" with the single-active-branch/worktree rule (P-016 aligned), added P-020 post-merge compaction to the closure item, and added the P-020 gate to enforcement row V.
2. **TUNE-006 Spike skill.** Added the graphtor-docs retrieval preference (two locations), switched to `docline.promoted_to`/`docline.plan_artifact`, quoted `source`, and added the source-substitution and description-required guidance.
3. **TUNE-007 Security reviewer.** Replaced the out-of-scope exclusion bullet with the upstream `## Scope Disposition` section.
4. **TUNE-008 Harness doctor.** Resolves the harness version live instead of relying on a hardcoded value.

## Deferred / Manual Review

| ID | Priority | Item | Reason |
|---|---|---|---|
| TUNE-009 | P2 | Validator-evidence merge into `runtime-verification` and `operational-closure` skills | Staged renders demote H1 and drop the workspace Step 5 Source Artifact Cleanup section; needs a careful hand merge |
| TUNE-010 | P2 | `shipment-reconcile`, `workflow-policies.md`, `_ship`/`_stage`/`_orchestrator` residual template deltas | Heavily customized workspace content; staged renders are stale relative to installed |
| TUNE-011 | P2 | `backlogit.instructions.md` Checkpoint Payload Contract | Staged render contains stale content (e.g. "no shipment blocked status") contradicting the governed blocked lifecycle |
| TUNE-012 | P2 | `technology.instructions.md`, security renders, `backlog-registry.yaml` | Review against staged output manually |
| TUNE-013 | P3 | `sqlite-reviewer` (`model: Claude Haiku 4.5`) and `go-mcp-expert` (`model: GPT-5.4`) | Custom agents use the `model:` key outside the tier-routing contract; operator decision |
| TUNE-014 | P3 | `compact-context/SKILL.md` generic "GPT-5.4-mini" mention | Cosmetic |
| TUNE-015 | P1 review | `start.sh` startup-script contract `ambiguous` | CRLF-induced; fails closed to operator review and is never auto-applied |

## False Positives (unchanged)

* `pipeline_topology_gate_ship_agent_wiring`: `backlogit_claim_shipment` appears in the `_ship` frontmatter `tools:` list, which trips the ordering constraint. All six `TOPOLOGY_GATE` markers are present and ordered correctly.
* Two portability warnings for `~/.autoharness` in `_orchestrator.agent.md`. This is the documented home-resolution fallback from the upstream template.
* 79 CRLF checksum mismatches (see Checksum Scan).

## Upstream Template Defects Observed

* Staged renders drop `model_tier` from agent frontmatter.
* Staged renders demote document H1 to H2, which would fail the P-008 MD041 gate.
* AGENTS.md and constitution renders leave `{{QUALITY_GATE_n}}` placeholders unresolved.
* The verifier checksum scan hashes raw bytes instead of the LF-normalized form the manifest uses.

## Advisory — Escalation Route

`model_routing.escalation` (`claude-opus-5.5`/anthropic/xhigh) differs from the Stage role route (`claude-opus-5.5`/anthropic/high) only in `reasoning_effort`. It is therefore not same-route degraded, but the config comment asks for a distinct vendor/family. Consider a cross-family escalation route (for example an OpenAI frontier model) so Stage escalations get a genuinely independent perspective. The config was not changed.

## Verification

* `autoharness verify-workspace --json`: 0 strict-schema blockers, 0 unresolved placeholders, 0 new artifacts. The only failing targeted check is the known topology false positive. The only migration proposal is the `start.sh` manual-review gate.
* `markdownlint-cli2@0.23.1`: 0 issues across the 29 changed markdown files.
* PowerShell AST parse of the lock scripts: 0 errors. `bash -n` on the lock scripts: clean.
* YAML parse of the manifest and all changed frontmatter: clean.
* Adversarial verify-harness pass: not run in this session. Recommend a `scope: new-only` pass during PR review.

## Recommendation

Review the diff on `chore/autoharness-tune-2026-09-27` and open a pull request against `main`. Tune again after the next major release, or monthly. Handle TUNE-009 through TUNE-012 as a dedicated hand-merge pass.
