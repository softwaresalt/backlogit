---
title: "212-S Wave 12 U1k harness evidence"
---

## Boundary and result

* Task: `211.022-T` only; feature `211-F`, shipment `212-S`
* Compilation: PASS
* Red Phase: CONFIRMED
* M22 remains 22 tasks; S23 remains 23 members. No exemptions or sibling reds.
* Source delta: `internal/core/gate/probe_test.go` only; no production edits
* Existing branch:
  `feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator`
* Worktree: `C:\Source\GitHub\backlogit`; no new branch/worktree or parent edits

## Observed RED

The final compile-only gate exited 0 across all 46 packages. The focused
race-enabled command exited 1 with both required named assertion failures:

```text
--- FAIL: TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout
    U1k temp ceiling: nonrepo t.TempDir inherited outer checkout HEAD
    Should be false
--- FAIL: TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity
    U1k version cache: unexpected subprocess count
    expected: 1
    actual  : 2
```

Git returned `true, nil` for the nonrepo instead of `false, nil`. The
committed repository under TEMP resolved HEAD from its subdirectory; an
explicit caller ceiling prevented that discovery. Ambient `GIT_*` overrides
are removed with restoration registered, and TEMP/TMP/TMPDIR are restored.
A fresh subtest avoids `testing.T.TempDir`'s already-cached parent base.

The trusted local `.cmd`/POSIX shell fixture returned `1.6.0` twice but ran
twice. Changing its size and output produced `1.7.0` on call three, with
counter 3 instead of 2. Changing the explicit environment then repeating it
produced counters 4 and 5 instead of 3 and 3. No required test skips or runs
in parallel. Both final REDs are behavioral assertions, not setup/build errors.

## Commands and static gates

All Go tooling used the pinned Go 1.24 workspace-contained wrapper.

```powershell
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -run=^$ -count=1 ./...
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -v -run '^TestU1k_' ./internal/core/gate
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go vet ./...
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 golangci-lint run --timeout=5m
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -v -run '^TestU175_001_BaselineEol$' ./tests
```

Vet and lint: PASS (exit 0, no findings). Tracked-first-party format: PASS
(`git ls-files -- "*.go"`, 1,400 files, batches of 100 passed to `gofmt -l`
inside the wrapper; no ignored modulecache traversal). EOL: PASS
(`checked=1746 excluded=0 index-not-lf=0 work-not-lf=0 w/mixed=0`).
No unfiltered test suite ran.

## Exact fail-closed scoped handoff command

Run from the existing worktree. This command was observed exiting 1 on the
REDs above. It requires native success AND both named top-level PASS records,
so an empty selector, skipped test or sibling PASS cannot satisfy it.

```powershell
pwsh -NoProfile -Command '$output = @(& pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -v -run "^TestU1k_" ./internal/core/gate 2>&1); $code = $LASTEXITCODE; $output; if ($code -ne 0) { exit $code }; foreach ($name in @("TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout", "TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity")) { if (-not ($output -match ("^--- PASS: " + [regex]::Escape($name) + " "))) { throw "Missing named PASS: $name" } }'
```

## Execution metadata

Configured MCP `update_item` applied `harness-ready`, preserving existing labels
and implementation posture, and added this evidence path plus the exact scoped
command to implementation notes only. Status remains `queued`; acceptance,
scope and planning fields are unchanged. The tool also appended its label-update
audit event (`seq=4661`) to `.backlogit/hooks_queue.jsonl`.
Lifecycle remains caller-owned: no claim, start, completion, archive, push, PR
or merge.
