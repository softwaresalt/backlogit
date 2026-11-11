---
title: "Stage Custody Handoff — 212-S / 211-F / 089-DL PR #645 Fix-CI Cycles 1-3 (Ordinal 19 & 20 PASS, Copilot P-018 SATISFIED & Stash 1CED4AD6, Ordinal 21 Git-Disappearance Halt [failed_count: 14 -> 15]) and Bounded Local Verification Authorization for 74c09bae (M22, S23)"
doc_type: reference
source: "docs/memory/2026-11-10/stage-212-s-089-dl-ordinal21-bounded-verification-authority.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: "212-S"
feature_id: "211-F"
deliberation_id: "089-DL"
phase: "stage-custody-handoff-m22-ordinal21-bounded-verification-authority"
---

## Summary of Bounded Same-Manifest Fix-CI Adjudication (`M22`, `S23`, `NOFEATURECOMMIT`)

Under the operator's explicit `"Proceed."` directive (`2026-11-09T23:24:15Z`), after the Orchestrator restored `git` (`git version 2.55.0.windows.5` via `C:\Program Files\Git\mingw64\libexec\git-core\git.exe` / `C:\Source\GitHub\backlogit\bin\git.exe`), Stage adjudicated the Step 5 fix-CI progress on [PR #645](https://github.com/softwaresalt/backlogit/pull/645) reported in `docs/memory/2026-11-10/212-S-pr645-git-tool-unavailable-checkpoint.md` (`checkpoint-20261110-232635.json`, SHA-256 `114b60be2c988d8272d3fad8228b2c34cb2a13b98c564753784defe3efa7f5e7`) for Feature `211-F` / Shipment `212-S` (`089-DL` / `D18`):

1. **Record Ordinal 19 PASS, Ordinal 20 PASS, Copilot P-021 C2 Stash `1CED4AD6`, and Ordinal 21 Git-Disappearance Halt (`failed_count: 14 -> 15`)**:
   * **CI Cycle 1 (`62064e89`) & Ordinal 19 PASS**:
     Portable `U4`/`U5`/`U6` relative-root and socket-length fixtures in `internal/cli/mcp_ipc_test.go` and `internal/cli/mcp_daemon_multisession_test.go` (`62064e89cf3b00d1daa6f5a1b0bffe040806e6bf`, `docs/memory/2026-11-10/212-S-pr645-ci-cycle1-portable-fixtures.md`); **Ordinal 19** (`bin/_rrc1645a19/pr645-ci1-ordinal19/result.json`, SHA-256 `79825a5724e5cf9202650e0f91a0870420776349cc8e3deb811a0a01d1ad0222`) passed all `46/46` packages (`3,922` top-level passes, `0` failures).
   * **CI Cycle 2 (`c574669b`), Copilot P-018 `SATISFIED` (`1CED4AD6`), & Ordinal 20 PASS**:
     Inherited Linux pipe/socket input cancellation in `internal/cli/mcp_daemon.go` and `internal/cli/mcp_ipc_unix.go` (with `internal/cli/mcp_ipc_windows.go` and `internal/cli/mcp_session_input_unix_test.go`, `c574669b0a016040a5ffb0582e2482a6b505289d`, `docs/memory/2026-11-10/212-S-pr645-ci-cycle2-shim-stdin-memory.md`); **Ordinal 20** (`bin/_rrc2645a20/pr645-ci2-ordinal20/result.json`, SHA-256 `5be6ffa55ab7d9c39c9c3261332ccf074b56fae4d27fe19bff8bcdd2405af6e0`) passed all `46/46` packages (`3,922` top-level passes, `0` failures, `native_exit: 0`, `evidence_valid: true`). Copilot review `5849123381` (U1k Git-ceiling observation in `internal/core/gate/baseref.go`, comment `4530351422`) was captured under P-021 C2 as stash **`1CED4AD6`**, replied in `4530424016`, and resolved on thread `PRRT_kwDORzozKM615Onq`; Copilot review `5850382383` on `c574669b` is clean (`P-018: SATISFIED`, `0` open threads).
   * **CI Cycle 3 (`74c09baeea828231ccbae811100bc4259f4f2335`) & Ordinal 21 Accounting (`failed_count: 14 -> 15`)**:
     Test-only private stdout duplication (`unix.FcntlInt(os.Stdout.Fd(), unix.F_DUPFD_CLOEXEC, 0)`) in `internal/cli/mcp_session_input_unix_test.go` (`//go:build !windows`, `7` lines changed) so `RunMCPSessionShim` does not close Go coverage's `os.Stdout` during `go test -coverprofile` on Linux CI (verified `6` cases × `10` repetitions = `60/60` PASS under WSL coverage mode). During **Ordinal 21** (`bin/_rrc3645a21/pr645-ci3-ordinal21/result.json`, SHA-256 `b3b9f756c80d56a8e1305e0825347649e3d6fcd2e28c5b08a181c45effa1070f`, consumption receipt `bin/tmp/212-ordinal21-c3645a21-consumption.json`), `44/46` packages passed (`3,903` top-level passes) and `tests` + `tests/integration` failed solely because `C:\Program Files\Git\cmd\git.exe` disappeared from `PATH` mid-run (`failed_count: 14 -> 15` retained with zero counter reset).

2. **Bounded Local Verification Authorization for `74c09bae` (Instead of an Ordinal 22 Windows Full-Suite Rerun)**:
   * Because the sole Go file changed between `c574669b` (where Ordinal 20 passed all `46/46` packages on Windows, `5be6ffa55ab7d9c39c9c3261332ccf074b56fae4d27fe19bff8bcdd2405af6e0`) and `74c09baeea828231ccbae811100bc4259f4f2335` is `internal/cli/mcp_session_input_unix_test.go` (`//go:build !windows`, which is excluded from Windows builds so the compiled Windows Go code of all `46` packages at `74c09bae` is 100% identical to `c574669b`), Ship is authorized to satisfy local verification after restoring `git` (`git version 2.55.0.windows.5` on `PATH`) by running:
     1. `go test -timeout=10m -count=1 ./tests ./tests/integration` (the two packages that failed when `git` disappeared during Ordinal 21)
     2. `GOOS=linux go test -c -o bin/tmp/_cli_linux.test ./internal/cli` (confirming Linux test compilation of `mcp_session_input_unix_test.go`, removing the temp binary afterward)
     3. `go build ./cmd/backlogit`, `go vet ./...`, `golangci-lint run --timeout=10m`, `backlogit_docs_lint`, and `TestU175_001_BaselineEol`
     paired with Ordinal 20's `46/46` Windows PASS receipt (`5be6ffa55ab7d9c39c9c3261332ccf074b56fae4d27fe19bff8bcdd2405af6e0`) and WSL Linux coverage-mode `TestU6_InheritedStdinShutdown` (`6x10` PASS), without launching an Ordinal 22 Windows full-suite rerun.

## SHA-256 Digests of Stage-Updated Files (Excluding Self-Digest)

| File Path | SHA-256 Digest |
|---|---|
| `.backlogit/archive/089-DL.md` | `4709ba9c3abfc45f6915f43472f0d67193c5d22d6896cb088088031a5c87483f` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `ff14db58f102b5adc57bbf361fb828f6084ab42f89c932d644a06bfc12e034a7` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `da627d057739c6da6695797bc2fe86e0c74e82fa2a0643abd95499273ba34eb6` |
| `.backlogit/queue/211-F.md` | `a5b59db03fd163cec42a7ea9cbc8e6838b4ade7940ff7955868b5432ae61de7f` |
| `.backlogit/queue/212-S.md` | `87fdc1d7090930b45d57e170f057dc8024a6f0ec4ed9b3bfc6a69a12bb2f393f` |
| `.backlogit/archive/211.004-T.md` | `83d4eacf67d9d059c44a5422410782c46b4b4bfc8de0bdc042da4f998bf1f42b` |
| `.backlogit/archive/211.005-T.md` | `78ccaa2233a951174a50fd3e25e8db046a4c7ad32a8707a0cbe0afe6a15e1722` |
| `.backlogit/archive/211.006-T.md` | `4ba52a27530c0fc1166cf87f8970c7c6a4bb7ec61d845a6e97fc63ac88c99d7e` |

## Custody and Boundary Attestations (`NOFEATURECOMMIT`)

* All `22` task members (`211.001-T`..`211.022-T`) remain `done` and archived in `.backlogit/archive/` (`1` `WORK_STARTED` record each, `open_red_deliverables = ∅`, `closed_exempt_set = ["211.007-T"]`).
* Zero Git commits, staging (`git add`), or pushes were performed by Stage (`NOFEATURECOMMIT`).
* Zero Go source/test files or checkpoint files were modified by Stage.
* Active checkpoints `.backlogit/checkpoints/checkpoint-20261110-232635.json` (`114b60be2c988d8272d3fad8228b2c34cb2a13b98c564753784defe3efa7f5e7`) and `.backlogit/checkpoints/checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`) remain untouched on disk.
