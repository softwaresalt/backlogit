---
title: "215-S / 214-F post-merge closure"
doc_type: closure
source: docs/closure/215-S-214-F-post-merge-closure.md
ingested_at: "2026-11-15T08:30:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
merge_commit: cc65c531a5739b8a215d3aa521317a3bb51010df
close_path: cascade
close_evidence: docs/closure/evidence/215-S-214-F-close-evidence.json
closure_status: READY
compaction_status: done
docline:
  shipment_id: 215-S
  feature_id: 214-F
---

## Release and governed scope

PR `#651` (`https://github.com/softwaresalt/backlogit/pull/651`,
`headRefOid: 34af8a0cb7bbde8b9832d3231017ab11688da393`) passed current-HEAD local
multi-persona, adversarial, and rubber-duck reviews (`READY_WITH_FOLLOWUPS`,
`P0=0, P1=0` in-scope), 8/8 CI checks, P-018 `autoharness gate copilot-review 651`
(`SATISFIED`, `exit_code: 0`), and P-009 merge-commit-only settings, and merged
under `DARK_MODE_ACTIVE` pre-authorization (`merge_approval_pre_authorized: true`)
via normal merge commit `cc65c531a5739b8a215d3aa521317a3bb51010df` at
`2026-11-15T07:54:36Z`.

Local `main` was synchronized via fast-forward-only sync
(`HEAD == origin/main == cc65c531a5739b8a215d3aa521317a3bb51010df`) before
creating `post-merge/215-s-closure`. Lifecycle topology gate returned
`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [214-F, 214.001-T..214.012-T]` plus shipment
control `215-S`. Pre-close, safe-close, and post-close reconciliation passed
under one held `.backlogit/queue/215-S.md` agent-lock, which was released at
post-mode completion. `215-S` is archived with `archived_status: shipped`;
`214-F` and `214.001-T`..`214.012-T` are archived with `archived_status: done`
and merge commit `cc65c531a5739b8a215d3aa521317a3bb51010df`. Zero non-member
changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/215-S-pre-20261115T080020Z.md` | `PROCEED` |
| `.backlogit/reconcile/215-S-safe-close-20261115T082850Z.md` | `CLOSED` |
| `.backlogit/reconcile/215-S-post-20261115T082850Z.md` | `PROCEED` |

Canonical governed evidence: [215-S close evidence](evidence/215-S-214-F-close-evidence.json)
(`classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`,
`selected_close_path: cascade`, `phase: post_close`,
`postcondition_verdict: pass`, `failures: []`, invoked with
`C:\Tools\backlogit.EXE` `1.12.0` / `718e710`).

## Invariants and validator evidence

* `internal/docline/policy.go` classifies both `docs/plans/` and legacy
  `docs/exec-plans/` as `plan` while keeping `docs/plans-extra/` as `guide`
  and preserving `ValidateClassifyPath` containment.
* `.autoharness/config.yaml` sets `docs.subdirectories.plans: plans`, and
  `.autoharness/harness-manifest.yaml` aligns `DOCS_PLANS_DIR`, `config_hash`,
  and all 6 modified manifest artifact checksums.

See [post-merge runtime verification](215-S-post-merge-runtime-verification.md)
for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library and harness-guidance release; no database migration required |
| Pre-deploy audit | Unfiltered `go test -timeout=30m ./...` (48/48 packages), `go vet`, `golangci-lint`, `gofmt`, `md-lint`, and `docs lint` passed |
| Post-deploy checks | Post-merge `TestU1_*`, `TestU2_*`, `TestU12_*`, `TestU180006_*`, `go build ./cmd/backlogit`, and CLI `docs classify`/`docs lint` probes passed |
| Risky action: governed cascade close | `ActionRisk: destructive`; authorized via `DARK_MODE_ACTIVE` pre-authorized release closure; `ActionResult: applied` (`exit_code: 0`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification); operator (rollback authorization); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-15` through `2026-11-22`).
* **Healthy signals**: `backlogit docs lint` returns `0` violations; `backlogit docs classify docs/plans/x.md` and `docs/exec-plans/x.md` return `plan`; `TestU180006_ManifestChecksumsCurrent` and `make md-lint` stay green on `main`.
* **Failure signals / rollback trigger**: Any misclassification of `docs/plans/**` or `docs/exec-plans/**`, containment regression, or manifest drift on `main`.
* **Rollback procedure**: Retain canonical `docs/plans/` classification while any files remain in `docs/plans/`; do not revert `internal/docline/policy.go` alone. Coordinated full rollback requires explicit operator approval and `git revert -m 1 cc65c531a5739b8a215d3aa521317a3bb51010df`.

## Source artifact provenance

Feature `214-F` declares `source_stash_id: none` and
`source_deliberation_id: none`. Decision document
`docs/decisions/2026-11-04-docs-plan-namespace-compatibility-deliberation.md` is
retained untouched as provenance only.

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`;
  [215-S compound refresh](215-S-compound-refresh.md)): 3 compound entries
  reviewed and kept (`keep: 3`).
* Invoked `compact-context` (`target: all`;
  [215-S compaction report](2026-11-15-215-s-compaction-report.md)): created
  [215-S compacted memory](../memory/compacted/2026-11-15-215-s-214-f-compacted.md)
  and preserved recent `215-S` handoff/closure records. `compaction_status: done`.
* Three out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries remain
  active in `.backlogit/stash.jsonl` awaiting Stage triage (`A991B4AC`,
  `6EB3B02E`, `7A712FBC`). No new follow-up items arose during post-merge
  closure.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |
