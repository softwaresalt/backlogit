---
title: "213-S U9 POSIX red evidence halt"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-posix-red-evidence-halt.md
schema_version: "1.0"
date: "2026-11-11"
status: resolved
chunk_strategy: h1-h2-h3
---

## State

Shipment `213-S` remains active on its feature branch at
`177cf3b6524428a69417995679fa8a190cae237f`. Task `212.002-T` was moved
from queued to active. Its existing `WORK_STARTED: 213-S` record was
verified exactly once in the current shipment-claim epoch; no duplicate was
appended. `212.003-T` remains claim-assigned, pending `212.002-T`.

The existing uncommitted `internal/toolchain/verify.go` changes were
preserved. The task status transition also changed
`.backlogit/queue/212.002-T.md` and `.backlogit/hooks_queue.jsonl`.
No implementation edit, staging, commit, task completion, or PR was made.

## Halt

The build-feature delegate observed all 12 U9 tests pass on Windows before
the requested cross-platform remediation. POSIX assertion-red evidence for
that remediation was not established; WSL has no Go executable on its PATH.
The delegate halted rather than change production code without the required
test-first evidence. Stage or the operator must resolve the evidence or
contract gap without weakening the test-first gate.

The WSL Go availability probe omitted the requested PowerShell environment
block; its `go version` command failed because Go was not found.

## Checks and resumption

Index sync, root attestation, scheduler replay, compile preflight and lifecycle
topology check passed. A prior intake reconciliation report exists. Wave 2
has not converged; later gates and merge were not attempted. The next session
must use the checkpoint recovery protocol and re-evaluate the active,
already-started U9 cursor under the applicable scheduler policy; it must not
append a second start record or assume ordinary wave admission permits resume.

## Resolution and recovery

* Resolved checkpoint `checkpoint-20261112-045733.json` was cleaned up so the
  next `_Ship` session enters via `ZERO-CANDIDATE NORMAL STARTUP` (without
  calling `backlogit_get_checkpoint`).
* Stage returned `212.002-T` to `status: "queued"` with `harness-ready`
  retained in `labels` for clean Step 4.0 Wave 2 admission into Step 4.1b,
  reusing the single existing `WORK_STARTED: 213-S` record in the `shipment
  claimed` epoch (`count == 1`) without appending a duplicate.
* Stage added the cross-compiled WSL Linux test binary verification recipe
  (`$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go test -c ./internal/toolchain -o C:\Source\GitHub\backlogit\bin\tmp\u9-linux.test`
  followed by `wsl.exe --cd /mnt/c/Source/GitHub/backlogit/internal/toolchain -e /mnt/c/Source/GitHub/backlogit/bin/tmp/u9-linux.test '-test.v' '-test.run' '^TestU9_'`)
  to `.backlogit/queue/212.002-T.md` (`implementation-notes`), providing
  executable POSIX red-to-green evidence for
  `TestU9_ReviewFixMCPArgumentsAndBindings/casefold-env-binding` while leaving
  `M internal/toolchain/verify.go` unstaged so `build-feature` can observe the
  POSIX red failure before applying the cross-platform normalization fix.
