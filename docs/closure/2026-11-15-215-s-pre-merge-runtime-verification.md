---
title: "215-S pre-merge runtime verification"
doc_type: closure
source: docs/closure/2026-11-15-215-s-pre-merge-runtime-verification.md
ingested_at: "2026-11-15T07:10:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-15"
  mode: pre-merge
  shipment_id: 215-S
  feature_id: 214-F
  evidence_head: b85300cd
  verification_verdict: PASS
---

## Scope and validator contract

Shipment `215-S` (`214-F`: Canonical plan namespace compatibility maintenance)
updates `internal/docline/policy.go` to classify `docs/plans/` as `plan` while
preserving `docs/exec-plans/` and path containment, sets
`docs.subdirectories.plans: plans` in `.autoharness/config.yaml`, updates active
authoring/skill guidance (`U3`–`U11`), and reconciles
`.autoharness/harness-manifest.yaml` (`U12`).

Loaded `.autoharness/workspace-profile.yaml` (`runtime_validation` contract):

```yaml
validator_manifest:
  surfaces: []
validation_expectations:
  required: false
  surfaces_expected: []
  minimum_verdict: PASS
  preserve_invariants: []
  release_blockers: []
releasability:
  required: true
  status_when_satisfied: READY
  required_evidence:
    - kind: monitoring-plan
      required: false
    - kind: rollback-procedure
      required: false
    - kind: owner
      required: false
```

`surface=auto` infers the `cli` surface (`backlogit docs classify`,
`backlogit docs scope`, `backlogit docs lint`, `backlogit docs migrate`).
Because `validator_manifest.surfaces` is empty and
`validation_expectations.required` is `false`, verification uses the CLI
command adapter plus MCP handler tests and Go test suites.

## Environment prechecks

* Built `backlogit.exe` via `go build ./cmd/backlogit` (`exit 0`, `GOTOOLCHAIN=go1.27.0`).
* Confirmed `go vet ./cmd/... ./internal/... ./tests/... ./scripts/...`, `golangci-lint run --concurrency=1 --timeout=15m`, `gofmt -l cmd internal tests scripts`, `scripts/md-lint.ps1` (`4175` files, `0` issues), and `backlogit docs lint --format json` (`valid: true`, `violation_count: 0`) all pass.

## Structured validator evidence

### CLI runtime probes (`.\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `.\backlogit.exe docs classify docs/plans/x.md` | `doc_type: "plan"` | `0` | `{"doc_type": "plan"}` |
| `.\backlogit.exe docs classify docs/exec-plans/x.md` | `doc_type: "plan"` | `0` | `{"doc_type": "plan"}` |
| `.\backlogit.exe docs classify docs/plans-extra/x.md` | `doc_type: "guide"` | `0` | `{"doc_type": "guide"}` |
| `.\backlogit.exe docs classify ../x.md` | Non-zero containment error | `1` | `Error: classify path must not contain dot-segment components: "../x.md"` |
| `.\backlogit.exe docs scope` | Both prefixes mapped to `plan` | `0` | `path_map` includes `"docs/plans/": "plan"` and `"docs/exec-plans/": "plan"` |
| `.\backlogit.exe docs lint --format json` | `valid: true`, `violation_count: 0` | `0` | `{"valid": true, "violation_count": 0, "findings": []}` |
| `.\backlogit.exe docs migrate --path docs/plans` | Dry-run metadata report, zero writes | `0` | `dry_run: true`, `body_bytes_changed: false` |
| `.\backlogit.exe docs migrate --path docs/exec-plans` | Empty dry-run metadata report | `0` | `dry_run: true`, `changes: []`, `findings: []` |

### Automated Go and MCP verification

* `go test -v -count=1 -run '^TestU1_' ./internal/docline` — `PASS` (`TestU1_ClassifyPlanNamespaces`, `TestU1_ValidateClassifyPathContainment`, `TestU1_ScopeIncludesBothPlanPrefixes`, `TestU1_PlanMigrationCanonicalAndLegacyFixtures`, `TestU1_ApplyMigrationCanonicalAndLegacyIdempotent`).
* `go test -v -count=1 -run '^(TestU2_WorkspaceConfigCanonicalPlansDir|TestU12_HarnessManifestNamespaceReconciliation|TestU180006_ManifestChecksumsCurrent)$' ./tests/integration` — `PASS`.
* `go test -v -count=1 -run 'Test.*Docs(Classify|Scope|Migrate)' ./internal/mcp` — `PASS` (9/9 MCP handler tests covering `backlogit_docs_classify`, `backlogit_docs_scope`, `backlogit_docs_migrate`).
* `go test -timeout=30m ./internal/docline ./tests/integration` — `PASS` at `b85300cd` (1.174s / 293.477s); unfiltered `go test -timeout=30m ./...` passed across all 48 packages at Wave 1 and Wave 2 convergence (`internal/docline/policy.go` unchanged after `7fc6b68d`).

## Invariants and manual checkpoints

* Canonical `docs/plans/` and legacy `docs/exec-plans/` both classify as `plan` without weakening `ValidateClassifyPath`, `docs/plans-extra/` remains `guide`, and `PlanMigration`/`ApplyMigration` preserve body bytes (`body_bytes_changed: false`).
* Existing `docs/plans/` documents remain protected because canonical recognition is retained alongside legacy recognition.

## Verdict and releasability handoff

`PASS`: all CLI probes, MCP handler tests, docline migration fixtures, and unfiltered repository Go test suites passed with zero blocked prerequisites.
