---
title: "220-S post-merge runtime verification"
description: "Post-merge runtime verification evidence on synchronized main for Shipment 220-S (Feature 219-F)."
doc_type: closure
source: docs/closure/220-S-post-merge-runtime-verification.md
ingested_at: "2026-11-21T08:42:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-21"
  mode: post-merge
  shipment_id: 220-S
  feature_id: 219-F
  merge_commit: 98aa1bf4033c5662005b4b9d236dfb5bc1090ade
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge runtime verification for Shipment `220-S` (`219-F`, merged at `98aa1bf4033c5662005b4b9d236dfb5bc1090ade`, PR `#668`) executed on synchronized `main` against `.gsd/config.yaml`, `.gsd/harness-manifest.yaml`, and all 22 reconciled `.github/**` artifacts per `.gsd/workspace-profile.yaml` (`validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true`).

## Environment prechecks

* Synchronized local `main == origin/main == 98aa1bf4033c5662005b4b9d236dfb5bc1090ade` (`POST_MERGE_SYNC_OK`).
* Verified `gsd routing resolve` across all 5 site/kind pairs (`stage` route, `ship` route, `orchestrator` route, `stage` escalation, `ship` escalation).
* Verified `gsd verify-workspace --workspace . --json` checksum scan and schema validation.
* Executed the integration contract suites (`TestU12_GSDModelRoutingManifestReconciliation`, `TestU12_HarnessManifestNamespaceReconciliation`, `TestUSR6_HarnessManifestDriftRecords`, `TestUCXS17_RefreshedDriftRecords`, `TestU19R3_ManifestBudgetReconciliation`, `TestWorkspaceAgentsGrantFullBacklogitToolSurface`).

## Structured validator evidence

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `gsd routing resolve --workspace . --site stage --kind route --json` | `gemini-4-carbon` / `google` / `medium` / `long_context` | `0` | `errors: []`, `degraded: []` |
| `gsd routing resolve --workspace . --site ship --kind route --json` | `gemini-4-hydrogen` / `google` / `high` / `long_context` | `0` | `errors: []`, `degraded: []` |
| `gsd routing resolve --workspace . --site orchestrator --kind route --json` | `gemini-4-carbon` / `google` / `medium` / `long_context` | `0` | `errors: []`, `degraded: []` |
| `gsd routing resolve --workspace . --site stage --kind escalation --json` | `gpt-6.2-sol` / `openai` / `high` / `long_context` | `0` | `errors: []`, `degraded: []` |
| `gsd routing resolve --workspace . --site ship --kind escalation --json` | `gpt-6.2-sol` / `openai` / `medium` / `long_context` | `0` | `errors: []`, `degraded: []` |
| `gsd verify-workspace --workspace . --json` | `0` checksum mismatches, `strict_schema_blockers: []` | `1` (pre-existing `backlog_registry_cli_surface` `dispose_shipment` only) | `checksum_scan: 97 total, 0 mismatches`, `blockers: []`, `strict_schema_blockers: []` |
| `go test -timeout=30m -count=1 -v -run '^(TestU12_GSDModelRoutingManifestReconciliation\|TestU12_HarnessManifestNamespaceReconciliation\|TestUSR6_HarnessManifestDriftRecords\|TestUCXS17_RefreshedDriftRecords\|TestU19R3_ManifestBudgetReconciliation\|TestWorkspaceAgentsGrantFullBacklogitToolSurface)$' ./tests/integration` | All 6 contract suites pass | `0` | `PASS` (`7.214s`) |

## Verdict

`PASS`: all post-merge routing, manifest-checksum, and integration contract checks passed on synchronized `main` (`98aa1bf4033c5662005b4b9d236dfb5bc1090ade`).
