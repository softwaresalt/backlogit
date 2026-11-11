---
title: 212-S U1c C1 additional review regression RED
date: 2026-11-06
task_id: 211.009-T
shipment_id: 212-S
phase: review-regression-red
---

## Scope and baseline

Production baseline SHA: `2358176202b24fcd58127fe981439579d2dd3255`.
P021 C1 concerns `archive.go:897-900`: failed canonicalization retains
`ErrValidation` but drops the underlying filesystem cause.
Only the existing `TestInGitWorktreeBounded` receives a named subtest;
no new top-level functions, wrappers, or production edits.
The four-function overall budget is unchanged.

This is an additional C1 regression before the fix, not a waiver or Stage
plan amendment. The original RED history remains in
`212-S-wave1-U1c-harness-manifest.md`, unchanged. Original guards and
manifests were not modified or forced back into old Phase Red.
Engram fallback remains unchanged: 0 symbols, 3 timeouts; known-file,
line-level confirmation used the exact-source exemption.

## Native RED proof

Observed at 2026-11-06 06:37 UTC with cached Go 1.24.0:

```powershell
. ./bin/tmp/212-go-env.ps1
go test -timeout=30m -count=1 -race -json -run 'TestInGitWorktreeBounded/canonicalization_errors_keep_underlying_chain' ./internal/core
```

Native exit **1**; additional in-session JSON guard exit **0**.
Exactly one run and fail occurred for each name:

* `TestInGitWorktreeBounded`
* `TestInGitWorktreeBounded/canonicalization_errors_keep_underlying_chain`

Exactly one package failed: `github.com/softwaresalt/backlogit/internal/core`.
The guard required those run/fail names and the assertion output
`Should be true` with
`canonicalization error must retain underlying fs.ErrNotExist`.
It rejected skips, unexpected failing tests, panic, build failure, race
reports, or failure messages from either preceding assertion.
The non-nil error and `errors.Is(err, ErrValidation)` assertions passed.
Only `errors.Is(err, fs.ErrNotExist)` failed at test line 117.
Both nonexistent paths derive from `t.TempDir`; no Git fixture is needed
for this witness, and it executes before any Git availability skip.

A subsequent bounded race/-v run of the original three canonical tests
also exited **1**, solely for the new C1 assertion. Its selector's optional
empty branch selected all subtests, not just the intended frozen cases.
All original strategy-detection cases, ancestor checks, isolated fake-Git
skip, direct/intermediate boundaries, and unnamed worktree assertions
passed. This is not represented as a GREEN command or an original RED
guard result.

## Quality and preservation proof

Every shell invocation dot-sourced `bin/tmp/212-go-env.ps1`; no host/default
Go retry, lint v2, or dependency download was used.

* Whole-repository compile before and after: exit **0**,
  `go test -run=^$ -count=1 ./...`
* Whole-repository `go vet ./...`: exit **0**
* Pinned `bin/golangci-lint.exe` v1.64.8: exit **0**, `Issues=[]`,
  `run --timeout=5m --out-format=json ./internal/core/... ./internal/policyengine/... ./tests/integration/...`
* Existing eight-file bounded `gofmt -l` gate: exit **0**, no findings
* Removing only the new subtest and `io/fs` import reproduced HEAD's
  preexisting test content verbatim, including frozen hermetic negatives
* Top-level function declarations matched HEAD exactly; no whole-file
  checksum was computed

Stop before production implementation. Only the test and this proof belong
in the regression commit; unrelated dirty backlog and memory state remains
untouched. No claims, moves, push, PR, nested agents, or planning edits.
