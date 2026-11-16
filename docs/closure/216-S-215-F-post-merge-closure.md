---
title: "216-S / 215-F post-merge closure"
doc_type: closure
source: docs/closure/216-S-215-F-post-merge-closure.md
ingested_at: "2026-11-16T17:40:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 216-S
  feature_id: 215-F
  merge_commit: 04944c1ed866666ec6b22f948ba2822ebbc1612c
  close_path: cascade
  close_evidence: docs/closure/evidence/216-S-215-F-close-evidence.json
---

## Release and governed scope

PR `#654` (`https://github.com/softwaresalt/backlogit/pull/654`,
`headRefOid: 850cf2ab688480f5b1924ceb0d2f0d78652af03a`) passed current-HEAD local
multi-persona, adversarial, correctness, security, and concurrency reviews
(`READY_WITH_FOLLOWUPS`, `P0=0, P1=0` in-scope), 8/8 CI checks, P-018
`autoharness gate copilot-review 654` (`SATISFIED`, `exit_code: 0` after
addressing both Copilot round-1 threads in `850cf2ab`), and P-009
merge-commit-only settings, and merged under `DARK_MODE_ACTIVE`
pre-authorization (`merge_approval_pre_authorized: true`) via normal merge
commit `04944c1ed866666ec6b22f948ba2822ebbc1612c` at `2026-11-16T17:13:38Z`.

Local `main` was synchronized via fast-forward-only sync
(`HEAD == origin/main == 04944c1ed866666ec6b22f948ba2822ebbc1612c`) before
creating `post-merge/216-s-closure`. Lifecycle topology gate returned
`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [215-F, 215.001-T..215.008-T]` plus shipment
control `216-S`. Pre-close, safe-close, and post-close reconciliation passed
under one held `.backlogit/queue/216-S.md` agent-lock, which was released at
post-mode completion. `216-S` is archived with `archived_status: shipped`;
`215-F` and `215.001-T`..`215.008-T` are archived with `archived_status: done`
and merge commit `04944c1ed866666ec6b22f948ba2822ebbc1612c`. Zero non-member
changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/216-S-pre-20261116T171800Z.md` | `PROCEED` |
| `.backlogit/reconcile/216-S-safe-close-20261116T173500Z.md` | `CLOSED` |
| `.backlogit/reconcile/216-S-post-20261116T173500Z.md` | `PROCEED` |

Canonical governed evidence: [216-S close evidence](evidence/216-S-215-F-close-evidence.json)
(`classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`,
`selected_close_path: cascade`, `phase: post_close`,
`postcondition_verdict: pass`, `failures: []`, invoked with
`C:\Tools\backlogit.EXE` `1.12.0` / `718e710`).

## Invariants and validator evidence

* `atomicfile.ReplaceTempFileInRoot` verifies target directory and leaf handles on Windows and POSIX before `MoveFileExW` / `unix.Renameat`.
* `internal/toolchain` recognizes aliased `backlogit` executables (`bin/backlogit.exe` and `bin/backlogit`) and requires `--cwd` workspace binding.
* `mcp.Server.Shutdown` and `CloseCoordinator` stop session + coordinator admission before waiting on handler drain while `CloseAllSessions` retains session-only semantics; `RunMCPDaemon` wires shutdown to `s.Shutdown()`.
* `withTempGitCeiling` preserves Git discovery for `TEMP`-rooted repositories while keeping ancestor isolation for non-repo temp dirs.
* `CreateArtifact`, `AdoptItem`, and `ReconstituteShipments` serialize on `id_allocator.lock` using pinned `.locks` `*os.Root` handles, scan local branches and pending reconstitute journals, and unreserve rolled-back IDs without acquiring `B`/`C` inside `id_allocator.lock`.
* `backlogit init` / `install` installs the runtime executable into the active workspace storage root (honoring validated `BACKLOGIT_WORKSPACE_DIR` overrides) and writes ignore rules including `/.backlog/.locks/` and `/.backlogit/.locks/`.

See [post-merge runtime verification](216-S-post-merge-runtime-verification.md)
for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library release; no database migration required |
| Pre-deploy audit | All 50/50 Go packages, `go vet`, `golangci-lint`, `gofmt`, `md-lint`, `docs lint`, and `wave-scheduler-sim` passed |
| Post-deploy checks | Post-merge `TestU1_`..`TestU6_`, `TestWorkspaceLiteralGuard`, `go build ./cmd/backlogit`, and isolated CLI `install`/`add` probes passed on `main` |
| Risky action: governed cascade close | `ActionRisk: destructive`; authorized via `DARK_MODE_ACTIVE` pre-authorized release closure; `ActionResult: applied` (`exit_code: 0`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification); operator (rollback authorization); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-16` through `2026-11-23`).
* **Healthy signals**: All 50 Go packages, `golangci-lint`, and `go build ./cmd/backlogit` pass on `main`; `backlogit install` wires runtime + `.locks/` ignore cleanly; zero ID collisions or lock-order violations in multi-session operations.
* **Failure signals / rollback trigger**: Any regression in Windows/POSIX atomic file replacement, MCP daemon shutdown, TEMP-rooted Git gate discovery, ID allocation/rollback, or `init`/`install` ignore wiring on `main`.
* **Rollback procedure**: Preserve archived backlog/event history and submit an operator-approved scoped mitigation PR (or coordinated `git revert -m 1 04944c1ed866666ec6b22f948ba2822ebbc1612c` excluding `.backlogit/archive/` history).

## Source artifact provenance

Feature `215-F` declares `source_stash_id: none` and
`source_deliberation_id: none`. Decision document
`docs/decisions/2026-11-15-cap2-sandbox-multisession-runtime-hardening-deliberation.md`
(`090-DL`..`095-DL`) and archived deliberation files `.backlogit/archive/090-DL.md`..`095-DL.md`
are retained untouched as provenance only.

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`;
  [216-S compound refresh](216-S-compound-refresh.md)): 4 compound entries
  reviewed and kept (`keep: 4`).
* Invoked `compact-context` (`target: all`;
  [216-S compaction report](2026-11-16-216-s-compaction-report.md)): created
  [216-S compacted memory](../memory/compacted/2026-11-16-216-s-215-f-compacted.md)
  and finalized `compaction_status: done`.
* Four out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` entries (`39F868B1`,
  `E0A6FDAF`, `7C26BE01`, `DE5D1EA5`) and one closure follow-up (`18579EF1`)
  remain active in `.backlogit/stash.jsonl` awaiting Stage triage.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |
