---
title: "215-S post-merge runtime verification"
doc_type: closure
source: docs/closure/215-S-post-merge-runtime-verification.md
ingested_at: "2026-11-15T08:30:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-15"
  mode: post-merge
  shipment_id: 215-S
  feature_id: 214-F
  merge_commit: cc65c531a5739b8a215d3aa521317a3bb51010df
  verification_verdict: PASS
---

## Scope and validator contract

Verified merged `main` (`cc65c531a5739b8a215d3aa521317a3bb51010df`, PR `#651`)
on closure branch `post-merge/215-s-closure` against `.autoharness/workspace-profile.yaml`
(`runtime_validation.validator_manifest.surfaces: []`,
`validation_expectations.required: false`, `releasability.required: true`).

## Post-merge runtime and Go verification

| Probe / Suite | Outcome | Observed detail |
| --- | --- | --- |
| `go test -v -count=1 -run '^TestU1_' ./internal/docline` | `PASS` | `1.964s`; all 5 U1 tests passed (`Classify`, `ValidateClassifyPath`, `Scope`, `PlanMigration`, `ApplyMigration` idempotency) |
| `go test -v -count=1 -run '^(TestU2_WorkspaceConfigCanonicalPlansDir\|TestU12_HarnessManifestNamespaceReconciliation\|TestU180006_ManifestChecksumsCurrent)$' ./tests/integration` | `PASS` | `1.248s`; canonical config, manifest checksums, duplicate-key rejection passed |
| `go build ./cmd/backlogit` | `PASS` | `exit 0` |
| `.\backlogit.exe docs classify docs/plans/x.md` | `PASS` | `{"doc_type": "plan"}` |
| `.\backlogit.exe docs classify docs/exec-plans/x.md` | `PASS` | `{"doc_type": "plan"}` |
| `.\backlogit.exe docs lint --format json` | `PASS` | `{"valid": true, "violation_count": 0, "findings": []}` |

## Verdict

`PASS`: canonical `docs/plans/` and legacy `docs/exec-plans/` classification,
workspace configuration, manifest checksums, and docline linting are verified on
synchronized `main` with zero blocked prerequisites.
