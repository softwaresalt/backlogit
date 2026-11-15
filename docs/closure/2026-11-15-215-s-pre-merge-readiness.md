---
title: "215-S pre-merge readiness"
doc_type: closure
source: docs/closure/2026-11-15-215-s-pre-merge-readiness.md
ingested_at: "2026-11-15T07:10:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-15"
  mode: pre-merge
  shipment_id: 215-S
  feature_id: 214-F
  evidence_head: b85300cd
  closure_status: READY
  compaction_status: pending
---

## Readiness summary

Shipment `215-S` (`214-F`: Canonical plan namespace compatibility maintenance)
completed all 12 tasks (`214.001-T`..`214.012-T`, Waves 1–2), passed Wave 2
convergence across all 48 Go packages, passed full local build
(`go build ./cmd/backlogit`), and passed pre-merge runtime verification
(`PASS`).

Pre-merge readiness is `READY` (subject to current-HEAD local review, CI checks,
and P-018 Copilot review before merge execution).

## Invariants and pre-deploy audits

* Canonical `docs/plans/` and legacy `docs/exec-plans/` both classify as `plan` in `internal/docline/policy.go`; `docs/plans-extra/` classifies as `guide`.
* `ValidateClassifyPath` containment and backslash normalization remain unchanged.
* `.autoharness/config.yaml` sets `docs.subdirectories.plans: plans`, and `.autoharness/harness-manifest.yaml` reconciles `DOCS_PLANS_DIR`, `config_hash`, and all 6 modified manifest artifact checksums with duplicate-key rejection verified.
* Legacy `docs/exec-plans/` inputs remain accepted by `plan-harden`, `plan-review`, and `compact-context`.

## Validator evidence

See [215-S pre-merge runtime verification](2026-11-15-215-s-pre-merge-runtime-verification.md):

* Verdict: `PASS` (`cli` command adapter + MCP handler tests + Go test suites).
* `.\backlogit.exe docs classify` confirmed `plan` for `docs/plans/x.md` and `docs/exec-plans/x.md`, `guide` for `docs/plans-extra/x.md`, and exit `1` dot-segment rejection for `../x.md`.
* `.\backlogit.exe docs scope` and `TestDocsScopeTool_ReturnsScope` confirmed `path_map` parity.
* `TestU1_*`, `TestU2_WorkspaceConfigCanonicalPlansDir`, `TestU12_HarnessManifestNamespaceReconciliation`, `TestU180006_ManifestChecksumsCurrent`, `scripts/md-lint.ps1` (`4175` files, `0` issues), `backlogit docs lint` (`0` violations), `golangci-lint run`, `go vet`, `gofmt`, and `go build ./cmd/backlogit` all passed.

## Risky action record

| ProposedAction | Risk | Approval and result |
| --- | --- | --- |
| Canonicalize `docs/plans/` in `internal/docline/policy.go` and `.autoharness/config.yaml` | High (shared contract/config) | Approved in plan review (`2026-11-05T07:13:15Z`) and `DARK_MODE_ACTIVE`; applied and verified |
| Merge PR for shipment `215-S` | Moderate (release handoff) | Pending CI, current-HEAD local review, P-018, and P-014 merge approval |

## Monitoring plan, owner, and validation window

* **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
* **Validation window**: Immediate post-merge verification on `main` plus 7-day observation window.
* **Healthy signals**:
  * `backlogit docs lint` returns `valid: true`, `violation_count: 0`.
  * `backlogit docs classify docs/plans/x.md` and `docs/exec-plans/x.md` both return `plan`.
  * `TestU180006_ManifestChecksumsCurrent` and `make md-lint` remain green on `main`.
* **Failure signals**:
  * Any `docs/plans/**` file misclassified as `guide` or failing `docs lint`
  * Manifest checksum drift or `make md-lint` failures on `main`

## Rollback trigger and procedure

* **Rollback trigger**: Any classification regression, path-containment failure, or manifest/CI drift on `main`.
* **Rollback procedure**: Retain canonical `docs/plans/` classification while any files or active references live under `docs/plans/`; do not blindly revert `internal/docline/policy.go` alone. Full namespace rollback requires explicit operator approval and coordinated revert of the `215-S` merge commit (`git revert -m 1 <merge_sha>`) alongside any files created under `docs/plans/`.

## Source artifact provenance

Feature `214-F` declares `source_stash_id: none` and `source_deliberation_id: none`. Source decision `docs/decisions/2026-11-04-docs-plan-namespace-compatibility-deliberation.md` is referenced as provenance only and remains untouched. Pre-claim branch carry-forward commit `9c54e947` also carried `.backlogit/checkpoints/checkpoint-20261115-010850.json` (`214-S` resolved closure checkpoint) and `model_routing` updates; per P-010/P-021 C1 that pre-claim state is preserved untouched and the routing alignment is deferred to `A991B4AC`.

## Residual risks and deferred follow-ups

Three out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries were captured during `215-S` (active in `.backlogit/stash.jsonl`, awaiting Stage triage):

* `A991B4AC`: Reconcile `model_routing.ship` and `model_routing.alt_doc_review.model_family` in `.autoharness/config.yaml` (carried via pre-claim commit `9c54e947`) with `.autoharness/harness-manifest.yaml` `variables_used.ALT_DOC_REVIEW_FAMILY`, `.github/skills/doc-review/SKILL.md`, and `.github/agents/_ship.agent.md`, and review cross-model diversity.
* `6EB3B02E`: Align pre-existing template parity gaps between `plugin/skills/{impl-plan,plan-harden,plan-review,spike}/SKILL.md` and `.github/skills/`, fix pre-existing `{{count}}`/`{{size_reduction}}` placeholders in `.github/skills/compact-context/SKILL.md` and doubled code fence in `.github/copilot-instructions.md`, and evaluate registering untracked skills in `harness-manifest.yaml`.
* `7A712FBC`: Print sorted `ScopeDescriptor.PathMap` entries in `backlogit docs scope` text output (`internal/cli/docs.go`), clarify metadata-only preview behavior in `docs/docline-frontmatter-authoring-guide.md`, and consider a cross-package parity test between `docline.Scope()` and `custodyHandoffDocDirs`.

## Structured releasability and compaction

| Evidence | Requirement | State |
| --- | --- | --- |
| Runtime verification | Optional (`required: false`) | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Post-merge closure duty | `pending` |
