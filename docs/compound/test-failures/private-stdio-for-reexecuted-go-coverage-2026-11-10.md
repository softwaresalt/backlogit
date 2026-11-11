---
title: "Give re-executed Go test fixtures private owned stdio"
description: "Avoid closing coverage runtime stdout when a subprocess fixture exercises an owned-stream API."
doc_type: reference
source: "docs/compound/test-failures/private-stdio-for-reexecuted-go-coverage-2026-11-10.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
problem_type: test-infrastructure
category: test-failures
component: CLI subprocess fixtures
root_cause: "The fixture gave an owned-stream closer the Go test runtime's shared stdout descriptor."
resolution_type: code_fix
severity: high
message: "error generating coverage report: write /dev/stdout: file already closed"
file_path: internal/cli/mcp_session_input_unix_test.go
citations:
  - "https://github.com/softwaresalt/backlogit/pull/645"
  - "docs/memory/2026-11-10/212-S-pr645-ci-cycle2-shim-stdin-memory.md"
tags:
  - go
  - coverage
  - subprocess
  - stdio
  - ownership
---

## Problem

The six inherited-stdin child cases passed ordinary local tests but
failed hosted coverage execution with exit 2 after the shim returned:
`error generating coverage report: write /dev/stdout: file already closed`.
The original production shutdown assertion was no longer failing.

## Root cause

The fixture re-executed its Go test binary and passed global `os.Stdout`
to an API that correctly closes its owned streams. The Go test runtime
still needed that global descriptor when emitting coverage afterward.
Normal execution did not expose the later coverage write.

## Resolution

Give the API a private stdout duplicate created with `F_DUPFD_CLOEXEC`,
wrapped as its own file. Let the API close that owned duplicate normally,
and retain cleanup for setup failures. Keep the test runtime's global
stdout open and leave the original descriptor flags unchanged.

Do not disable coverage, ignore the child exit status, replace owned
closure with a no-op, or relax the shutdown assertions.

## Verification

Coverage-only execution reproduced all six failures before the fix.
Afterward, all six pipe/socketpair cases passed ten repetitions in
normal and atomic-coverage modes. The correction is recorded in
`74c09baeea828231ccbae811100bc4259f4f2335`.

Local WSL race compilation lacked a C compiler. That was a declared
environment limitation, not a race PASS or authority to install tools.
The unchanged race-enabled hosted check remains the final proof.

## Prevention

* Separate API-owned handles from descriptors shared with test frameworks
* Reproduce subprocess failures with the instrumentation used by CI
* Keep genuine EOF, cancellation, ownership and exit-status assertions
* Distinguish an unavailable local prerequisite from successful validation

See [PR 645](https://github.com/softwaresalt/backlogit/pull/645) and
[the detailed CI evidence](../../memory/2026-11-10/212-S-pr645-ci-cycle2-shim-stdin-memory.md).
