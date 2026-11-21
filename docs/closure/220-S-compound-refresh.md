---
title: "220-S compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for Shipment 220-S (Feature 219-F)."
doc_type: closure
source: docs/closure/220-S-compound-refresh.md
ingested_at: "2026-11-21T08:42:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 220-S
  feature_id: 219-F
---

## Invocation and evidence

Evaluated relevant `docs/compound/` learnings and durable documentation under Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`). Evidence sources: merged PR `#668` (`98aa1bf4033c5662005b4b9d236dfb5bc1090ade`), `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, 22 reconciled `.github/**` files, `tests/integration/gsd_model_routing_reconciliation_test.go`, [220-S close evidence](evidence/220-S-219-F-close-evidence.json), and [220-S post-merge runtime verification](220-S-post-merge-runtime-verification.md).

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Duplicate YAML keys survive safe_load validation](../compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md) | `keep` | Accurate and reinforced by `219.012-T`: `decodeUniqueYAML` / `checkUniqueYAMLMappingKeys` validates `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, and all 21 tiered agent frontmatter blocks with an explicit negative self-test |
| [Durable writes two-class contract](../compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md) | `keep` | Accurate; BOM-free UTF-8 and LF `\n` writes prevented Windows BOM/CRLF drift across all 22 `.github/**` files and `.gsd/harness-manifest.yaml` |
| [Machine-readable governance field contract](../compound/2026-07-23-machine-readable-governance-field-contract.md) | `keep` | Accurate and applied in `TestU12_GSDModelRoutingManifestReconciliation` locking typed struct parity across `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, and `.github/**` |
| [GitHub plugin skill parity test gap](../compound/2026-07-14-github-plugin-skill-parity-test-gap.md) | `keep` | Accurate; `TestU12_GSDModelRoutingManifestReconciliation` asserts directly against the live in-repo `.github/agents/**`, `.github/skills/doc-review/SKILL.md`, and `.github/instructions/adversarial-review.instructions.md` surfaces |
| [Carry-forward continuity files into staging](../compound/workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md) | `keep` | Accurate; `220-S` carried `.gsd/config.yaml` forward cleanly onto `feat/220-s` while excluding `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, and `.kbdb/` from all commits |
| [Gitattributes EOL renormalization worktree refresh](../compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md) | `keep` | Accurate; manifest checksum verification uses LF-normalized SHA-256 (`lfSHA256Hex`) across all 97 manifest entries |

Reviewed: 6 entries (`keep: 6`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

* `.github/agents/**`, `.github/skills/doc-review/SKILL.md`, `.github/instructions/adversarial-review.instructions.md`: model-routing frontmatter and routing tables/prose synchronized with `.gsd/config.yaml` and `.gsd/harness-manifest.yaml` in `220-S`.
* `docs/ARCHITECTURE.md` and `AGENTS.md`: no structural updates required (no new Go packages, MCP tools, or pipeline role boundaries introduced).
* Remaining polish items (`alt_doc_review` diversity note, historical `claude-opus-4.8` `drift_reason` wording, `doc-review` Step 2 dispatch, and `adversarial-review` Phase 1 live `anchor_review` resolution) are captured in `.backlogit/stash.jsonl` (`072973EA`, `09AFFA9D`, `4C4A0919`, `179CE9A6`) for Stage triage.
