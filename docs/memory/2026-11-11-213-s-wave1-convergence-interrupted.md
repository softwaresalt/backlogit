---
title: "213-S Wave 1 convergence interrupted"
doc_type: reference
source: docs/memory/2026-11-11-213-s-wave1-convergence-interrupted.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
status: resolved
shipment_id: 213-S
feature_id: 212-F
---

## State

`DARK_MODE_HALTED`: Wave 1 convergence remains incomplete. The shipment is
active on its existing feature branch at
`aa51b4d171bd3fb757a16262295636ccacf23afa`. Task `212.001-T` was already
completed; Waves 2 and 3 have not been admitted in this session. No PR or
merge was created in this session.

The index sync and served workspace and storage root attestation passed.
Startup checkpoint enumeration found no active Ship checkpoints and no
quarantine anomalies. Hook polling returned no events.

With the operator-supplied Go environment, compilation, vet, lint,
`gofmt -l .`, and the non-vacuous U8 scoped command passed. The unfiltered
`go test -timeout=30m ./...` was interrupted after more than 40 minutes
without a final result. Individual packages, including `internal/cli`, had
passed, and further test subprocesses were observed running. An interrupted
run is not a suite pass or proof of a test failure.

## Resolution — Wave 1 Full Suite Completed and Passed (Exit Code 0)

Root-cause investigation determined that running `go test -timeout=30m ./...`
without `GOFLAGS=-count=1` passes `-test.testlogfile=.../testlog.txt` to
`core.test.exe` (which writes a 320 MB `testlog.txt`), causing `go.exe`
`computeTestInputsID` to spend 25+ minutes hashing 3.5 million lines after
`core.test.exe` exits, and system `core.fsmonitor=true` spawns detached
`git fsmonitor--daemon` processes during temp-repo tests.

With `$env:GOFLAGS = "-count=1"`, `$env:GIT_CONFIG_COUNT = "1"`,
`$env:GIT_CONFIG_KEY_0 = "core.fsmonitor"`, `$env:GIT_CONFIG_VALUE_0 = "false"`,
`$env:GOMODCACHE = "C:\Users\dewilliams\go\pkg\mod"`,
`$env:GOPATH = "C:\Source\GitHub\backlogit\bin\_authoring_gopath"`,
`$env:TEMP = "C:\Users\dewilliams\AppData\Local\Temp"`, and Go 1.24 + `bin/` on
`$env:PATH`, the complete Wave 1 convergence gate was re-executed at the Wave 1
`212.001-T` code state (`2cae0b26`) and **PASSED with exit code `0` across all
46 packages**:

* `ok github.com/softwaresalt/backlogit/cmd/backlogit`
* `ok github.com/softwaresalt/backlogit/cmd/faultline-analyze`
* `ok github.com/softwaresalt/backlogit/cmd/gen-docs`
* `ok github.com/softwaresalt/backlogit/internal/atomicfile`
* `ok github.com/softwaresalt/backlogit/internal/cli 239.562s`
* `ok github.com/softwaresalt/backlogit/internal/config`
* `ok github.com/softwaresalt/backlogit/internal/core 906.051s`
* `ok github.com/softwaresalt/backlogit/internal/db 52.049s`
* `ok github.com/softwaresalt/backlogit/internal/docline`
* `ok github.com/softwaresalt/backlogit/internal/errors`
* `ok github.com/softwaresalt/backlogit/internal/events`
* `ok github.com/softwaresalt/backlogit/internal/faultline`
* `ok github.com/softwaresalt/backlogit/internal/hooks`
* `ok github.com/softwaresalt/backlogit/internal/mcp 66.708s`
* `ok github.com/softwaresalt/backlogit/internal/models`
* `ok github.com/softwaresalt/backlogit/internal/policyengine 15.084s`
* `ok github.com/softwaresalt/backlogit/internal/soundness`
* `ok github.com/softwaresalt/backlogit/internal/telemetry`
* `ok github.com/softwaresalt/backlogit/internal/toolchain`
* `ok github.com/softwaresalt/backlogit/tests 471.934s`
* `ok github.com/softwaresalt/backlogit/tests/contract 25.151s`
* `ok github.com/softwaresalt/backlogit/tests/integration 247.362s`
* Exit code: `0` (`0` failures across all packages).

Wave 1 (`212.001-T` / U8) convergence is **COMPLETE AND PASSED**. Proceed
directly to Wave 2 (`212.002-T` / U9) admission (`k = 2`).
