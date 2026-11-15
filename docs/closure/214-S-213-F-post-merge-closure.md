---
title: "214-S / 213-F post-merge closure"
doc_type: closure
source: docs/closure/214-S-213-F-post-merge-closure.md
ingested_at: "2026-11-15T00:50:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
merge_commit: f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3
close_path: cascade
close_evidence: docs/closure/evidence/214-S-213-F-close-evidence.json
closure_status: READY
compaction_status: done
docline:
  shipment_id: 214-S
  feature_id: 213-F
---

## Release and governed scope

PR #649 (`https://github.com/softwaresalt/backlogit/pull/649`, `headRefOid: 3c2c1c19d3c4f747c352d7f56ee9b6d58c4d1bf8`)
received explicit P-014 operator merge approval (`PR 649: Merge approved`) and
merged via merge commit `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3` at
`2026-11-15T00:24:59Z`. Before closure, all §1.9 pre-merge checks were
re-verified: current `headRefOid` match, `READY_WITH_FOLLOWUPS` (`P0=0, P1=0`),
8/8 `SUCCESS` CI checks, P-018 `autoharness gate copilot-review 649` `SATISFIED`
(`exit_code: 0`), and P-009 merge-commit-only repository settings.

Local `main` was synchronized via the 8-step fast-forward-only protocol
(`HEAD == origin/main == f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`) with
unrelated local state (`.autoharness/config.yaml` and
`checkpoint-20261114-220306.json`) preserved unchanged before creating
`post-merge/214-s-closure`. Lifecycle topology gate returned
`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [213-F, 213.004-T, 213.001-T, 213.002-T, 213.003-T]`
plus shipment control `214-S`. Pre-close, safe-close, and post-close
reconciliation passed under one held `.backlogit/queue/214-S.md` agent-lock,
which was released and verified absent at post-mode completion. `214-S` is
archived with `archived_status: shipped`; `213-F` and `213.001-T`..`213.004-T`
are archived with `archived_status: done` and merge commit
`f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`. Zero non-member changes and zero
tracked archive deletions occurred (P-007 PASS).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/214-S-pre-20261115T003022Z.md` | `PROCEED` |
| `.backlogit/reconcile/214-S-safe-close-20261115T004213Z.md` | `CLOSED` |
| `.backlogit/reconcile/214-S-post-20261115T004213Z.md` | `PROCEED` |

Canonical governed evidence: [214-S close evidence](evidence/214-S-213-F-close-evidence.json)
(`run_id: 366fea9139b84a71a635bbb24996872e`, `classifier_verdict: CASCADE`,
`engine_verdict: VERIFIED`, `selected_close_path: cascade`,
`phase: post_close`, `postcondition_verdict: pass`, `failures: []`,
invoked with `C:\Tools\backlogit.EXE` `1.12.0` / `718e710`).

## Invariants and validator evidence

* Level 1 workspace-local pinning remains bounded by design:
  `containment_level: "level-1-workspace-local-pinned"` and
  `os_container_sandbox_deferred: true` remain explicit in
  `.backlogit/toolchain.yaml` and `ToolchainVerificationReport`.
* `VerifyLauncherScripts` and `AuditRepositoryToolchain` inspect `.mcp.json`,
  `start.ps1`, and `start.sh` read-only and reject ambient `PATH` lookups or
  unpinned binaries.
* `WriteToolchainManifest` and `RenderPinnedMCPConfig` reject path traversal,
  symlink/reparse escapes, duplicate keys, and `unsupported`/`non-conformant`
  classifications with wrapped `ErrValidation`.
* Operator-owned local runtime files (`./bin/*.exe`, `.mcp.json`) remain under
  operator custody outside repository commits.

See [post-merge runtime verification](214-S-post-merge-runtime-verification.md)
for full probe output (`verdict: PASS`):

* Post-merge `go test -timeout=30m -count=1 -run '^(TestU11_|TestU11a_)' ./internal/cli ./internal/toolchain` — `PASS` (`4.853s` / `0.421s`).
* Post-merge `go test -timeout=30m -count=1 -run '^TestU12_' ./tests/integration` — `PASS` (`3.714s`).
* Isolated `--cwd <tempDir>` CLI probe (`go run ./cmd/backlogit --cwd <tempDir> mcp status --toolchain --strict --format json`) — exit `1`, JSON output (`valid: false`, `violations: 12`, `os_container_sandbox_deferred: true`), wrapped `ErrValidation`.
* Merged-tip `Line Endings` workflow (`43658054879` on `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`) — `success`.

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library release; `.backlogit/toolchain.yaml` is additive; no database migration required |
| Pre-deploy audit | `VerifyLauncherScripts`, `AuditRepositoryToolchain`, `WriteToolchainManifest`, `RenderPinnedMCPConfig`, and CLI flags verified pre- and post-merge |
| Post-deploy checks | Post-merge U11a/U11/U12 suites, isolated `--cwd` `--toolchain --strict` probe, and `main` `Line Endings` CI run passed |
| Risky action: governed cascade close | `ActionRisk: destructive`; explicit operator authorization (`PR 649: Merge approved`); `ActionResult: applied` (`exit_code: 0`) |
| Risky action: isolated CLI probe | `ActionRisk: low` (read-only against repo; temp dir under `bin/tmp/` removed); `ActionResult: applied` (`PASS`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (post-merge verification and handoff); operator (rollback authorization); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed inside window) plus 7-day multi-session observation window (`2026-11-15` through `2026-11-22`).
* **Healthy signals**:
  * `backlogit mcp status --toolchain --format json` reports expected `containment_class` per tool (`violations: 0` when operator-provisioned `.mcp.json` and `./bin/*` pins align).
  * `backlogit mcp status --format json` reports healthy daemon state without cross-workspace mismatch false positives.
  * `backlogit doctor` reports `0` new errors and CI `CLI Reference Drift`, `Docline frontmatter gate`, and `P-008` checks stay green.
* **Failure signals / rollback trigger**:
  * Unexpected `violations > 0` on a properly provisioned workspace, `ErrDaemonWorkspaceMismatch` false positives, or CI drift/regression on `main`.
* **Rollback procedure**:
  * Stop affected workspace MCP daemons (`backlogit mcp stop`), revert merge commit `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3` via a reviewed PR (`git revert -m 1 f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`), rebuild `./bin/backlogit.exe`, and run `backlogit sync`.

## Source artifact provenance

Feature `213-F` declares `source_stash_id: none` and
`source_deliberation_id: none`. `.backlogit/archive/082-DL.md` is retained as
a shared reference (`retained_shared_reference`) and was neither mutated nor
re-archived. Linked-stash preflight against `.backlogit/stash.jsonl` and
`backlogit_get_links` confirmed zero linked stash entries for `214-S`, `213-F`,
and `213.001-T`..`213.004-T`.

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: apply`;
  [214-S compound refresh](214-S-compound-refresh.md)): 3 compound entries
  reviewed and kept; `docs/ARCHITECTURE.md` and
  `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`
  updated via knowledge graduation.
* Invoked `compact-context` (`target: all`;
  [214-S compaction report](2026-11-15-214-s-compaction-report.md)): archived
  superseded `2026-11-13-214-s-u11-harness-halt.md` to
  `docs/archive/memory/214-S/2026-11-13-214-s-u11-harness-halt.md`, created
  [214-S compacted memory](../memory/compacted/2026-11-15-214-s-213-f-compacted.md),
  and preserved cited/latest `214-S` handoff records. `compaction_status: done`.
* Twelve out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries remain
  active in `.backlogit/stash.jsonl` awaiting Stage triage (`9CFC88E5`,
  `0CD5F543`, `E231EA13`, `D8EE8389`, `040826D4`, `3466D3CB`, `08A5E6F7`,
  `C5189EAF`, `65129EBA`, `FBF280DB`, `44EE1E84`, `375A4079`). No new
  follow-up stash entries were required during post-merge closure.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |
