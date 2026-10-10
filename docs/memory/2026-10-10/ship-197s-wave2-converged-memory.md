---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S wave 2 converged; wave 3 next (DARK_MODE, P-017)
description: Ship resumed 197-S after the operator's descope ruling, recomputed the amended Step 3 schedule (M = 18, four waves), and completed wave 2 (seven tasks) with per-task exempt gates, review gates, in-scope fixes, out-of-scope P-021 C2 captures, and the Step 4.6 wave convergence gate. No PR has been opened and no merge has occurred.
timestamp: "2026-10-10T08:45:00Z"
---

# Ship 197-S wave 2 converged

## Session status

**PARTIAL (resume point: wave 3, task 197.010-T).** Scope `[197-S]` only. Branch
`feat/cx-ship-closure-protocol-and-gate-correctness`, HEAD `4c07de3a` (pushed up to
`51e9f2fe`; later commits are local until the push at the end of this invocation).
`origin/main` is untouched. No PR exists.

## Verification performed by Ship

| Step | Result |
|---|---|
| Served roots (`backlogit_get_metadata_catalog` plus `pragma_database_list`) | Workspace `C:\Source\GitHub\backlogit`, storage `.backlogit`, one `main` row, `.backlogit/backlogit.db`. |
| Post-claim topology | `topology gate pass`; 197-S is the sole active shipment. |
| Index sync | `backlogit_sync_index` ran (1965 rows) at intake and before wave 2. |
| Checkpoints (read-only enumeration) | Three active Ship checkpoints for 197-S, plus one active for 153-S. No quarantine or validation error. Per operator direction nothing was restored, resumed, or resolved. |
| Step 3, real schedule (`logs/ship-197s-step3-check.ps1`, read-only, not committed) | `CONTRACT_CHECK_OK`. S = 19, M = 18, excluded `197-F`. W1 = 001-008, 017; W2 = 009, 011-015, 018; W3 = 010; W4 = 019. Seven red contracts valid (close wave equals latest green-maker wave; every green-maker in a later wave). Ten exempt contracts valid. `needs_harness = [197.019-T]`. No green-regression blocks. |
| Step 4.0 snapshot (SQL over frozen M) | 9 done, 9 active (claim-assigned, marker `197-S`, no `WORK_STARTED` at wave 2 admission), 0 blocked, 0 unsupported. `197.016-T` archived and outside M. |

## Commits on the shipment branch (this invocation)

| Commit | Content |
|---|---|
| `8d19d4e6`, `51e9f2fe` | Stage U15 descope: archive, queue, stash, hooks, plan Erratum E2, and resume notes. |
| `40af5ee3` / `cdcc4f22` / `4518e3e7` / `5025f90c` / `6cedc5de` | 197.009-T (covered-by): `_ship.agent.md` Step 6 item 1, after review-fix cycles 1 to 3. |
| `67e4e45a` / `ec60af65` / `db37247e` / `983fbaf0` | 197.011-T (covered-by): `_orchestrator.agent.md`, after review-fix cycles 1 to 2. |
| `8d8d2b84` / `32b6a157` | 197.012-T (covered-by): PowerShell lock sidecars. |
| `1107b823` / `f5eb0d82` | 197.013-T (covered-by): bash lock sidecars. |
| `aec27214` / `f6ff4d50` / `793dcb6d` | 197.014-T (covered-by): concurrency and file-lock documentation. |
| `adcbcde4` / `ea9d2f3e` / `a48157ad` | 197.015-T (covered-by): docline closure gate keys. |
| `f6a8b18f` / `482a8866` / `4c07de3a` | 197.018-T (covered-by): CLI progress on stderr. |

## Gate results (wave 2 convergence, Step 4.6)

* `go test -run=^$ -count=1 ./...` exit 0. `go vet ./...` exit 0. `golangci-lint run` exit 0.
* `gofmt -l .` lists 25,658 files because of CRLF checkout artefacts. The four Go files touched in wave 2 are clean on LF-normalised committed blobs.
* Newly closed red deliverables are GREEN: `TestUCXS1_` (4 PASS), `TestUSR3_` (4), `TestU(CXS3_|SR1_)` (6), `TestUCXS4_` (7), `TestUCXS6_NormalizeClosureGateKeysStayTopLevel` (4), `TestUCXS5_` (1). Zero FAIL in each.
* `TestUCXS2_` (197.003-T, green-maker 197.010-T, close wave 3) is still RED with 3 FAILs, as required.
* **FULL_SUITE_DEFERRED: wave 2.** Open red: `197.003-T` selector `^TestUCXS2_` (tests/integration), green-maker `197.010-T` scheduled wave 3. Reason: compile, vet, lint, format, and every declared scoped command have run and passed; the only failing selector in the repository is the declared open red.

## Exempt tasks (wave 2): must-fail probes and completion gates

Every probe ran at its own baseline before its claim and exited 1 with no marker. Every completion gate exited 0 with its exact `EXEMPT_VERIFY_OK:<id>` marker. Path passes matched each exemption surface exactly.

## Review and P-021 handling (summary)

| Task | Review cycles | P0/P1 | In-scope fixes | OUT_OF_SCOPE captures (P-021 C2, capture-only) |
|---|---|---|---|---|
| 197.009-T | 3 | 0 | F1, F2, F5 (plus cycle-2 P2 and P1) | 63233C1C, 6FF6320C, 9DA5FBD6, 405C8574, 9F2AA687, 2C8615A5 |
| 197.011-T | 2 | 0 | F1 containment, F2, F4, F5 | 00A9D01C (Go catalog queue_path) |
| 197.012-T | 1 | 0 | none (P3 residual risk) | 4AAC11E4 (.gitignore agent-lock) |
| 197.013-T | 1 | 0 | none (P3 residual risk) | none new |
| 197.014-T | 1 | 0 | F1, F2 (doc) | F8090390, A8122B01 |
| 197.015-T | 1 | 0 | comment | 8F45E676 (AMBIGUOUS 1293086D), AFDC480A |
| 197.018-T | 1 | 0 | F1 progress invariant | 2C01BD52, 5E0CCF91 |

Discovery was performed before every capture. Where a candidate existed but identity could not be
positively confirmed, the capture cites it as `DISCOVERY-STATUS: AMBIGUOUS`.

## Deviations (recorded, P-010 / file-rule)

1. The 197.009-T and 197.011-T review-fix subagents, and the 197.015-T subagent, wrote scratch files
   under the gitignored `logs/` with PowerShell redirection or `Set-Content`. None is committed. The
   files were left in place because the no-deletion rule applies.
2. One Ship-authored commit message file for 197.011-T was written with `[IO.File]::WriteAllText`
   under `logs/`. It is gitignored and not committed. Later messages used the create tool.
3. An early 197.009-T build subagent made a diagnostic run over 14 tests beyond the wave scope.
   Those were read-only and their failures were in the tolerated set.

## Residual risks and follow-ups (not blocking wave 2)

* Lock-name cutover: clear stale legacy `.*.lock` files before upgrading the lock scripts (197.012-T, 197.013-T).
* `internal/cli` progress: stderr is written while the shipment membership lock is held (advisory, 197.018-T).
* `internal/core` full suite exceeds the default 10 minute timeout; the wave gate uses `-timeout=30m`.
  Related performance follow-up: stash `76553D8D`. The R10 "fixed" part is NOT delivered by 197-S.
* The orchestrator harness-manifest checksum is stale by design; 197.019-T's drift record covers it.
* `dry-run docs migrate` over `docs/closure` reports 175 change entries with no per-key detail. Key
  preservation is evidenced by `TestUCXS6`, not the dry run. Do not run `docs migrate --apply` without
  per-key confirmation (see stash `8F45E676` and `1293086D`).

## Next steps (resume point)

1. Wave 3: task `197.010-T` (covered-by owner `197.003-T`; edits `.github/agents/_ship.agent.md` Step 4.0
   item 1, Step 5 item 5a, Step 6.0 item 4). Its must-fail probe, claim, build, verify, review, and
   bookkeeping follow the same procedure.
2. Wave 4: `197.019-T` (U17). harness-architect scaffolds the 196-shape drift-record RED harness at the
   start of wave 4 (Amendment A2; it must not permanently assert checksum equality with the current hash),
   then build.
3. After wave 4, Step 4.6 full-suite gate runs `go test -timeout=30m ./...` with no open red.
4. Then the full multi-persona and adversarial review of the branch, quality gates, the local readiness
   record, the feature PR, the Copilot loop (`autoharness gate copilot-review ... --enforcement required`),
   CI, merge commit, post-merge main sync, `backlogit_ship_shipment` with the merge SHA, closure, and the
   closure PR.
