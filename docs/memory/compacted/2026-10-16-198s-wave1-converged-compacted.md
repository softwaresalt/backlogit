# 198-S wave 1 converged — compacted

Source: `docs/archive/memory/2026-10-12/2026-10-12-ship-198s-wave1-converged.md` (original, moved not deleted). Compacted by compact-context (P-020) during 158-S post-merge closure; 198-S is archived (done).

* Wave 1 of 198-S (CT) converged on `feat/ct-restore-test-suite-health-and-runtime-diagnostics`: tasks 198.003-T, 198.001-T (mutation-kill, zero delta), 198.002-T (red to green), 198.004-T (exempt docs-only spike) done; mutation-kill 10/10 and 3/3.
* Served roots attested at wave admission; raw item-log reads declared as scoped P-012 reads (permanent under A1.5).
* Plan review ADVISORY with operator authorization recorded; A1.7 items 4(c)/(d) P-010 readings disclosed for operator revocation.
* Deviations: fourth `slog.SetDefault` site captured as deferred stash `8640432F`; AC2 of 198.002-T vacuous as written (verified with an exact-name selector, 8 PASS); `-count=1` used because cached packages hide flakes; `gofmt -l .` false positives from a Copilot cache and CRLF working copies.
* Spike 198.004-T: `internal/core` 788.962 s, 1445 passed, 0 failed, 11 skipped; 9 follow-ups (FU-1..FU-9).
* Convergence gate: compile, vet, lint, unfiltered full suite (31m40s wall, core 941 s) all green; no open red deliverables.
* Next at the time: wave 2, 198.005-T (T5 verification-only, `-count=1 -timeout 40m` on a quiet host).
