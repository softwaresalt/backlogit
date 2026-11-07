---
title: "212-S R3 actual stash-busy witness: focused RED/GREEN"
doc_type: "memory"
schema_version: "1.0"
recorded_at_utc: "2026-11-07T05:07:51.015817Z"
baseline_head: "25ae363ae11d5b9992031de8f8e219eb7878b1ce"
status: "Focused GREEN; awaiting parent final review 2/2, not convergence or branch readiness"
checkpoint_sha256: "235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70"
---

## Bounded delta

Executed harness-architect test-first and build-feature fix steps directly;
no nested agents or task/planning mutations. The only production change is
three lines in `executeReconstitution`, invoking the existing context observer
with `"contended"` after the true `errors.Is(err, os.ErrExist)` stash-busy
check. `ErrTaskBusy` is not the stash error category. No new seam, hook API,
global state, helper file, dependency, or lock semantics.

Only the existing `TestU1f_` function changes in the test file. Its launch
acknowledgment is replaced by a buffered receipt through that existing observer.
With the stash holder still live, a bounded select requires actual contention
before cancellation. A second bounded select requires cancellation return
before holder release. Failure and cleanup paths release the holder before any
remaining join; no sleep, stress loop, or timeout increase.

The canonical three subtests/four-run guard remain. Token-level call comparison
against original harness commit `644e5ca2bee78325e071c61a3d2cd844926f3e84`
preserves all 45 original assertion nodes, and comparison against the current
baseline preserves all 106 accepted assertion nodes; there are now 108.
This comparison includes testify calls and testing failure calls, ignoring
only whitespace/comments. Cancellation, digest, ordering, public API, dry-run,
and durability behavior are not reimplemented or weakened. The accepted
three-function scope remains unchanged.

## Retained evidence

All files here are fresh, exclusively created proof; no past capture overwritten.
Receipt timestamps are JSON strings ending in UTC `Z`. Native Go JSONL streams
are retained byte-for-byte, including their native local-offset timestamps.

* `red.jsonl` / `red-receipt.json`: native exit **1** on unchanged production
  SHA256 `f4c181ba645e9465c997a908d76dcc9890cf865dd5419bd3393d6166e154cbe3`.
  The named assertion is
  `U1f R3 actual stash-busy witness assertion: no contention observed while holder remained live`.
  Four run events; only canonical subtest 1 and its parent failed. Other two
  subtests passed. No skip, race, panic, build failure, or timeout. Cancellation
  returned and bounded cleanup completed.
* `green.jsonl` / `green-receipt.json`: native exit **0**, four run events,
  no failures/skips, actual busy receipt followed by cancellation return before
  holder release.
* `strict-green2.txt` / `strict-green2-receipt.json`: unchanged strict four-run
  GREEN checks passed, native Go exit **0**, launcher exit **0**.
  Guard source SHA256
  `8bff0a9e59d889f2148f2eb4e39e562a3d41da19fbc7b32e236c0efe33eef368`.
* `strict-green.txt` / `strict-green-receipt.json`: retained **failed launch**,
  exit **1**, caused by Python interpreting a Windows path backslash as
  backspace. This did not run Go and is not assertion RED or convergence.
  The corrected raw-string launch is separately retained; no source repair.
* `quality-receipts.json` / `quality-0.txt` through `quality-3.txt`:
  compile-only, scoped vet, pinned v1 lint (`Issues: []`), and formatting
  each exited **0**. The required baseline compile-only check also exited **0**.
  Neither compile check executed tests; no whole-core or whole-repository
  test-running suite was invoked.
* `preservation.json`: exact source identities, assertion preservation, and
  immutable checkpoint hash. An initial local comparison counted only the
  40 testify calls; including the five original testing failure calls yields
  the required 45. No assertion was changed to repair that classification.
* `go.diff`: exact two-file Go delta against the baseline, 35 insertions and
  12 deletions; production is only three inserted lines.
  Its literal diff-context prefixes intentionally contain space-before-tab;
  staged whitespace validation excludes only this retained diff artifact,
  not either Go source file or any other proof file.

Final source SHA256:

* `internal/core/shipment_reconstitute_exec.go`:
  `61d4d8fcfc8f9a24b593dabad992f07bacc158e1d6b56f7635216d818be26155`
  (Git blob `48d10b732aec25bf190c0f296a2ff5dc576a9a4e`).
* `internal/core/shipment_reconstitute_hardening_test.go`:
  `1672aa134a53340c6c59585d08306d49173c7676de55cfc58b3dfa1d4252dd36`
  (Git blob `c3210b2f02e6eaf2c46ae0c3b3971bce32dd3108`).

## Native commands and environment

Every Go invocation dot-sourced cached offline Go 1.24 setup
`bin/tmp/212-go-env.ps1`; fresh short ignored TEMP/TMP was `bin/_r3a91`.
No toolchain downloads, dependency installs, recorder changes, or timeout raise.

```powershell
. ./bin/tmp/212-go-env.ps1
$env:TEMP = Join-Path $PWD 'bin/_r3a91'
$env:TMP = $env:TEMP
go test -count=1 -timeout=30m -race -json -run '^TestU1f_' ./internal/core
go test -run=^$ -count=1 ./...
go vet ./internal/core
& ./bin/golangci-lint.exe run ./internal/core --timeout=5m --out-format=json
gofmt -l internal/core/shipment_reconstitute_exec.go internal/core/shipment_reconstitute_hardening_test.go
```

The strict guard hardcodes its earlier TEMP and derives root from script
location. To honor this follow-up's fresh TEMP requirement without writing
the guard, the launch adapted **only** those two environmental literals in
memory. The native Go command and every strict assertion are unchanged.
Exact native `pwsh -NoProfile -Command` arguments are in its receipt:

```powershell
$guard = Get-Content -Raw scripts/verify-u1f-harness.ps1
$guard = $guard.Replace('$root = Split-Path $PSScriptRoot -Parent', '$root = ''C:\Source\GitHub\backlogit''').Replace('bin/_h8a2f10c4/t', 'bin/_r3a91')
& ([scriptblock]::Create($guard)) -Phase Green
```

## Continuity and handoff

Current `checkpoint-20261106-210317.json` remains byte-identical to the
authorized hash above. Four convergence failures, six completed tasks, M12,
and unstarted U2-U7 remain retained. No task start/claim/reopen/archive,
metadata/checkpoint/planning mutation, token, full-suite retry, PR, push, or
merge. Unrelated dirty queue/archive/memory files are excluded from this commit.
No fifth convergence, historical-failure repair, or whole-branch READY claim.

Stop at focused GREEN. Next action belongs to the parent: final review 2/2
of this exact source delta and its retained native receipts.
