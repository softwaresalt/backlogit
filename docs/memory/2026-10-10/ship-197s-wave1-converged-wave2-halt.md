---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S wave 1 converged; wave 2 admission halted on U6 target gap
description: Ship resumed 197-S after Stage's P-002 contract amendment, verified the Step 3 schedule on the real shipment, scaffolded and confirmed the seven wave 1 red deliverables, completed the two wave 1 exempt tasks, and converged wave 1. Wave 2 admission halted because 197.016-T (U15) requires an explicit numeric U6 target that is not recorded. No PR was opened and no merge occurred.
timestamp: "2026-10-10T05:29:00Z"
---

# Ship 197-S wave 1 converged; wave 2 admission halted

## Session status

**PARTIAL: HALTED at wave 2 admission (Step 2 item 6 gap report, 197.016-T).**
Scope `[197-S]` only. Branch `feat/cx-ship-closure-protocol-and-gate-correctness`,
HEAD `c41f504e` (pushed at `de75c673`; later commits are local until the push at the end of
this invocation). No PR exists. `origin/main` is untouched. `stash@{0}` and
`.autoharness/config.yaml` were not touched.

## Verification performed by Ship (not taken from the Stage handoff)

| Step | Result |
|---|---|
| Served-root attestation (`get_metadata_catalog` + `pragma_database_list`) | `workspace.root_path` = `C:\Source\GitHub\backlogit`; storage `.backlogit`; one `main` row, `.backlogit/backlogit.db`. |
| Post-claim topology, run twice | exit 0, `topology gate pass`; 197-S is the sole active shipment. |
| `WORK_STARTED` before wave 1 | none for any member. One record appended per task at its claim. |
| Real Step 3 validation (`CONTRACT_CHECK_OK`, run by Ship) | M = 19 task IDs; `197-F` excluded as a feature. Waves W1 = 001-008, 017; W2 = 009, 011-016; W3 = 010, 018; W4 = 019. Acyclic. Seven red mappings pass the five-key rule, the strictly-later-wave rule, and close-wave equality. Ten exempt contracts match the closed set on 197-F, and each covered-by owner is a direct dependency that is not exempt. Selectors are `-count=1`, anchored to `^TestU`, with explicit packages. No green-regression blocks. |
| P-002.5 screen on the exempt commands | 197.008-T matched the token `--apply` inside a case-insensitive `-notmatch` regex literal. Ship classified it as a documented false positive (operation-level screen: read-only). The command was run as a read-only probe. Operator may veto (see Decisions). |

## Commits on the shipment branch

| Commit | Content |
|---|---|
| `de75c673` | 197-S claim records and Stage's P-002 contract amendment (queue files, plan Erratum E1, Stage amendment note). |
| `370a7cf3` | Wave 1 red harnesses (harness-architect, 15 files, test files only, no production `.go`). |
| `cb736fa6` | Hook events for the wave 1 `harness-ready` labels (committed so the baseline is clean). |
| `ec5fdf0c`, `47ed8970`, `db493ca2`, `8e32e874`, `931740df`, `280f40d7`, `b7c2381f`, `c5a4df13` | Per-task red completion commits, each committed with explicit paths and the archive move. |
| `7505b740` | 197.007-T U6 benchmark file (`internal/core/shipment_gate_bench_test.go`, 100 insertions). |
| `4665c2d4`, `b4415beb`, `c41f504e` | 197.007-T and 197.008-T completion, and 197.008-T's docs registration. |

## Red deliverables (wave 1) confirmed by Ship

Each red selector was run independently by Ship, not only by the harness subagent. Each
compiles, fails on named assertions, and has an empty zero-delta set against its per-task baseline.
At the convergence gate (`c41f504e`) all seven still fail with exit 1 and are not vacuous.

| Task | Selector | Red evidence | Green-maker (close wave) |
|---|---|---|---|
| 197.001-T | `^TestUCXS1_` | 3 subtests fail | 197.009-T (2) |
| 197.002-T | `^TestUSR3_` | `ProceedAndShipStep6` fails | 197.009-T (2) |
| 197.003-T | `^TestUCXS2_` | 2 subtests fail | 197.010-T (3) |
| 197.004-T | `^TestU(CXS3_\|SR1_)` | `TestUCXS3_` and `TestUSR1_/ProcedureLiterals` fail | 197.011-T (2) |
| 197.005-T | `^TestUCXS4_` | all six file rows fail | 197.012-T, 197.013-T, 197.014-T (2) |
| 197.006-T | `^TestUCXS6_NormalizeClosureGateKeysStayTopLevel$` (`./internal/docline`) | `GateKeysStayTopLevel` fails | 197.015-T (2) |
| 197.017-T | `^TestUCXS5_` (`./internal/cli`) | fails on missing progress lines | 197.018-T (3) |

## Exempt tasks (wave 1)

* **197.007-T (U6, verification-only).** Pre-work probe exit 1, no marker. Completion gate exit 0
  with `EXEMPT_VERIFY_OK:197.007-T`. P-002.4 path pass is exactly the benchmark file, and the
  content pass is 100 insertions with 0 deletions. gofmt is clean on LF-normalized content.
  golangci-lint on `internal/core` is clean. Verdict **CONFIRMED** (see the blocker below).
* **197.008-T (U7, docs-only).** Pre-work probe exit 1 (`140-S closure registration missing`).
  Completion gate exit 0 with `EXEMPT_VERIFY_OK:197.008-T`. Path pass is exactly
  `docs/closure/140-S-158-F-post-merge-closure.md`, 66 insertions, 0 deletions.
  `backlogit docs lint --path docs/closure` gives `valid: true` and zero violations.
  markdownlint shows 0 issues. AC1's gate-run output cannot be observed while 197-S is active
  (`PRECLAIM_ACTIVE_SHIPMENT_PRESENT`). Per the task's evidence note it is recorded at closure.

## Wave 1 convergence gate (Step 4.6)

* Every ready member is `done` (001-008 and 017). Nothing is active residual in the wave.
* Repo-wide `go test -run=^$ -count=1 ./...` exit 0 at `c41f504e`.
* `go vet ./...` exit 0. `golangci-lint run` exit 0.
* `gofmt`: the repo-wide `gofmt -l .` output is dominated by CRLF checkout false positives.
  The shipment-touched Go files were re-checked on LF-normalized content: zero real issues.
* Every open red selector still fails with exit 1 (not vacuous).
* `newly_closed_k`: none. No green-maker has landed yet.
* `open_red_deliverables`: `197.001-T`, `197.002-T`, `197.003-T`, `197.004-T`,
  `197.005-T`, `197.006-T`, `197.017-T`.
* **FULL_SUITE_DEFERRED: wave 1.** The open red set is non-empty. Open selectors are listed above,
  with green-makers 197.009-T and 197.011-T through 197.015-T at wave 2, and 197.010-T and
  197.018-T at wave 3. Stated reason: compile, vet, lint, format, and each scoped command have run
  and passed, every open selector is confirmed RED, and the only failures are the declared open reds.

## Blocker: wave 2 admission (Step 2 item 6, gap report)

**197.016-T (U15) cannot be scaffolded yet.** Amendment A1 requires its RED harness to be derived
from the U6 verdict, ns/op, profile, and target, and says that missing evidence is a wave-2 halt.
The U6 notes (comments on 197.007-T, actor `ship`) record the verdict (CONFIRMED), ns/op, and the
top-five profile. They do **not** record an explicit numeric target. A1 defines the target as the
share of time U6 attributes to per-member lookup and log reads. The recorded attribution is
`events.ReadAllEvents` 80.2% and `core.loadArtifact` 13.6%, with `findArtifact`/WalkDir 0% on the
timed path. Ship has not converted that attribution into a target, because that would be a guess.

Two facts Stage should weigh when it records the target or amends A1:

1. The benchmark is noisy on this host (39 to 153 ms per op). Its own notes attribute the spread to
   Defender scanning. Timing may only corroborate a RED, so the RED must rest on a work-count or
   `_test.go`-seam condition.
2. The U6 measurement contradicts the plan's WalkDir premise. `findArtifact` is off the timed path,
   and a WalkDir-only fix would not move this benchmark. U15's RED shape should follow the measured
   log-read cost, not the premise.

Resume requirement: Stage records the numeric U6 target (or amends A1) before wave 2 admission. Then
the orchestrator re-invokes Ship. Ship re-runs admission and scaffolds 197.016-T's RED at wave 2
start, with the characterization guard required by A1.

No other wave 2 member was admitted. P-002.6 forbids a partial wave, so 197.009-T, 197.011-T
through 197.015-T, and 197.016-T remain `active` and claim-assigned. Their owners are red and
their exemption contracts are valid.

## Decisions (Ship, this invocation)

1. Committed Stage's contract amendment with explicit paths (no `git add -A`).
2. Scaffolded wave 1 harnesses through harness-architect (subagent) and verified each red
   selector independently.
3. Ran the wave 1 red completions and exempt gates directly per the build-feature Step 0.5 and
   Step 0 procedures. For 197.007-T and 197.008-T, the build work itself was delegated to a
   build-feature subagent with the exempt constraints.
4. **P-002.5 classification of 197.008-T's screen match.** The token `--apply` sits only inside a
   case-insensitive regex that the command searches for. The command reads one file and runs
   read-only `docs lint`. It performs no destructive operation. Ship ran it as a read-only probe.
   This is a judgement call, recorded for operator veto.
5. Evidence for every exempt completion was written as backlog comments, not body edits, so that
   the P-002.4 diff range stayed on the permitted surface.
6. Raw reads of `.backlogit/logs/<id>.jsonl` (gitignored, read-only, no-follow path checks) were
   used only to verify the start-record count. P-012 scoped declaration for this session:
   the backlog MCP tool does not return the full event stream for an item.
7. Left `checkpoint-20261010-024921.json` (197-S, 02:49) in place. It is superseded by the
   checkpoint written at the end of this invocation, whose `resume_hint` names it. Ship did not
   restore, resume, or resolve any checkpoint.

## Follow-ups

* No new P-021 deferred-scope entries were created in this invocation.
* The unrelated `.backlogit/hooks_queue.jsonl` writes are committed per task (see commit table).

## Unattended-run risks

* Every backlog `done` transition moves the task file from `.backlogit/queue/` to
  `.backlogit/archive/`. The archive file must be committed in the same commit, or the next
  zero-delta check fails. Ship does this for every completion.
* Wave 2 cannot be admitted until Stage records the U6 target. That is the single gating item.
