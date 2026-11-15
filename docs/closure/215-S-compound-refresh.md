---
title: "215-S compound refresh and knowledge evaluation"
doc_type: closure
source: docs/closure/215-S-compound-refresh.md
ingested_at: "2026-11-15T08:30:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 215-S
  feature_id: 214-F
---

## Invocation and evidence

Invoked `compound-refresh` (`scope: recent`, `mode: propose`) for shipment
`215-S` (`214-F`) and evaluated durable documentation under Ship Step 6 items 3
and 4. Evidence sources: merged PR `#651` (`cc65c531a5739b8a215d3aa521317a3bb51010df`),
`internal/docline/policy.go`, `.autoharness/config.yaml`,
`.autoharness/harness-manifest.yaml`, `docs/docline-frontmatter-authoring-guide.md`,
and `docs/closure/evidence/215-S-214-F-close-evidence.json`.

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Docline frontmatter contract](../compound/2026-06-26-docline-frontmatter-contract.md) | `keep` | Body-preserving, idempotent migration and authoring-profile lint remain authoritative and verified by `TestU1_ApplyMigrationCanonicalAndLegacyIdempotent` |
| [Duplicate YAML keys survive safe_load validation](../compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md) | `keep` | Verified accurate; `u2_canonical_plan_config_test.go` and `u12_harness_manifest_namespace_test.go` enforce `yaml.Node` duplicate-key checks and LF-normalized manifest checksums |
| [Markdownlint library entrypoint false green](../compound/workflow-issues/markdownlint-library-entrypoint-false-green-2026-11-13.md) | `keep` | Verified accurate; `scripts/md-lint.ps1` caught 10 pre-claim `MD001` errors in `.backlogit/archive/214.003-T.md`..`214.012-T.md` before PR creation |

Reviewed: 3 entries (`keep: 3`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

`docs/ARCHITECTURE.md` delegates docline path-taxonomy details to
`docs/docline-frontmatter-authoring-guide.md` (updated by `214.011-T`), so no
extra `docs/ARCHITECTURE.md` edit was required. Three out-of-scope follow-up
items (`A991B4AC`, `6EB3B02E`, `7A712FBC`) remain active in `.backlogit/stash.jsonl`
for Stage triage.
