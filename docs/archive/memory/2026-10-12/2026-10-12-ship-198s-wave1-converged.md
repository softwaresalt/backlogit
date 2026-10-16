# 198-S (CT) wave 1 converged: session memory

Recorded: 2026-10-12 UTC (wave 1 convergence gate ran 23:28 to 23:59Z). Branch: `feat/ct-restore-test-suite-health-and-runtime-diagnostics`. Session: resumed Ship run under dark mode (operator AFK), same scope and contract as the original dispatch.

## Served-root attestation at wave admission

`SERVED_ROOTS_OK` (re-attested 2026-10-12 UTC): `backlogit_get_metadata_catalog` `workspace.root_path` = `C:\Source\GitHub\backlogit`, `workspace.storage_root` = `C:\Source\GitHub\backlogit\.backlogit`; `pragma_database_list` main = `C:\Source\GitHub\backlogit\.backlogit\backlogit.db` (one row, direct child of the served storage root).

## Declared P-012 scoped reads (A1.5 / Step 4.0)

No configured backlog tool returns a member's full event stream, so start-record and claim-state checks read `.backlogit/logs/<id>.jsonl` directly. Declared for this session, for the raw reads of `.backlogit/logs/198.001-T.jsonl` through `198.005-T.jsonl` (read-only, `Select-String`; no log written by the read). Permanent under A1.5; not limited to one task.

## Plan ruling and intake (resume steps 1 to 5)

* Final `## Plan Review`: `decision: ADVISORY`, `operator_authorization: approved` (Orchestrator dark-mode proxy, 2026-10-12T22:15:18Z). A1.5 Intake accepts ADVISORY with the marker. Residual risk recorded in the plan (six cycle-4 P2 text fixes not re-reviewed). Mitigation: the adversarial review re-checks A1.5 dispatch constraints and the T4 and T5 bullets, and wording defects go to stash, not plan edits.
* Ruling line: exactly one column-0 `a1_4_operator_ruling: T1=A; T3=A: ` line (plan line 730).
* Exempt contract blocks for 198.004-T and 198.005-T: byte-identical to A1.3 after line-ending normalization. Labels `harness-exempt` present.
* Green-regression blocks: one each on 198.001-T and 198.003-T, commands verbatim per A1.5.
* `backlogit docs lint` on the three Stage docs: valid, 0 findings. `scripts/md-lint.ps1`: 0 issues.
* Commits: docs `fa085cf1` (three Stage docs by explicit path); backlog state `f99400d2`; checkpoint `80114f4d`.

## A1.7 item 4(c) and 4(d), P-010 readings

A1.7 item 4 asked the operator to confirm two Ship role-boundary readings: (c) a spike deliverable produced through `build-feature`, and (d) an R6 general-purpose executor writing only ignored scratch files. The resume dispatch directs T4 and T5 executors and the R6 executor, but does not use the words "confirms (c)/(d)". Ship proceeded on that direction, reads it as the confirmation A1.7 asks for, and discloses it here and in the PR so the operator can revoke it. The executors did no source writes outside the task deliverable (T4) or ignored scratch (R6).

## Wave 1 execution

| Task | Lifecycle | Commits | Result |
|---|---|---|---|
| 198.003-T (T3) | mutation-kill, zero delta | delivering `9762e75f`; completion `18b5d827`; link `8c23935f` | done; regression 200 of 200 PASS (independently re-run) |
| 198.001-T (T1) | mutation-kill, zero delta | delivering `9762e75f`; completion `8837dd26`; link `454ddced` | done; regression -count=30 twice, ok (103 s and 177 s) |
| 198.002-T (T2) | genuine red to green | `42e4fd8d`; completion `8e9c5b09` | done; TestUT2_ green in Ship's own run |
| 198.004-T (T4) | exempt docs-only spike | `a5a07af9`; completion `bb7db9e3` | done; exempt gate EXEMPT_VERIFY_OK:198.004-T (independent re-run) |

Mutation-kill evidence (R6): `docs/memory/2026-10-12-198s-ct-mutation-kill-evidence.md` (commit `b9e1fb4b`). T1 10 of 10, T3 3 of 3, `R3_FINDINGS: 0`. Mutant logs and the executor's evidence are in ignored `logs/_ct-mutation/`.

## Deviations and decisions recorded during wave 1

1. **Fourth slog.SetDefault site (P-021 C2).** T2 names three capture sites. The build loop found a fourth, `silenceBenchLogs` (`internal/core/shipment_gate_bench_test.go:61`, added by 7505b740, benchmark-only). Out of scope under C1. Captured as deferred stash entry `8640432F` with `DISCOVERY-STATUS: AMBIGUOUS D8EF5443`. T2 covers the three named sites. Plain `go test` never runs the benchmark.
2. **AC2 of 198.002-T is vacuous as written.** `go test -count=1 ./internal/core -run 'BaseRef|ShippedEventDurability'` matches no test name and exits 0 with `no tests to run`. P-002.6 fails closed on that, so the literal AC2 is recorded as not met. The intent (the tests defined in the two named files) was verified with an exact-name selector of all eight tests: 8 PASS, exit 0 (`logs/ship-198s/t2-ac2-file-tests.log`). The wording defect is routed to post-merge closure follow-ups; the plan is not edited (P-010).
3. **Cached packages in the wave-1 suite.** Twenty-eight packages reported `(cached)` (unchanged binaries). Cache hits can hide load-sensitive flakes, so the T5 run and the final gate use `-count=1`.
4. **gofmt on this Windows tree.** `gofmt -l .` walked a Copilot session cache (25,663 files) and listed 516 tracked files in the working copy. These are CRLF false positives. All committed blobs are gofmt-clean (0 of 516 fail on their HEAD content).

## Spike result (198.004-T)

Run: `go test -json -count=1 -timeout 40m ./internal/core`, host alone, 2026-10-12 22:57 to 23:10Z, exit 0. Package 788.962 s; 1445 passed, 0 failed, 11 skipped. Deliverable `docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md`. `SPIKE_FOLLOWUP_COUNT: 9` (FU-1 to FU-9). Stash capture deferred to post-merge closure per A1.3. The top-ranked item is an AST walk of the git-ignored `logs/` tree (`TestU172_ItemLogBarrierSetterUnreachableFromProduction`, 170.57 s), which fits none of the four classes; recorded as `recovery-enabled-fixture` and flagged in the document.

## Wave 1 convergence (Step 4.6)

* Repo-wide compile (`go test -run=^$ -count=1 ./...`): exit 0.
* `go vet ./...`: exit 0. `golangci-lint run` (v1.64.8, matches CI pin): exit 0.
* Unfiltered `go test -timeout=30m ./...`: exit 0, 31 min 40 s wall, `internal/core` 941.205 s (52% of the 30-minute pin), no FAIL, no panic.
* Open red deliverables: none. Newly closed: none.
* R2 and R4 recorded as task comments on 198.001-T and 198.003-T.

## Next

Wave 2: 198.005-T (T5, verification-only). T5 runs once with `-count=1 -timeout 40m` on a quiet host, after this memory note is committed. A non-green T5 result halts through P-005 with 198-S left active (A1.3 Erratum, A1.5).
