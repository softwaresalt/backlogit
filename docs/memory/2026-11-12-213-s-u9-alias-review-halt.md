---
title: "213-S U9 alias review halt"
doc_type: reference
source: docs/memory/2026-11-12-213-s-u9-alias-review-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-12"
status: resolved
---

## Cursor

* Shipment: `213-S`; active task: `212.002-T`; next task: `212.003-T`
* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`
* Reviewed HEAD: `3b2d937573f3327f331309c30bddc15b69b361cf`
* Checkpoint: `checkpoint-20261112-174047.json` (resolved and removed from working tree)
* PR: not created in this session

## Findings and Stage disposition

The 12-test U9 race harness, repository compile, focused vet, repository lint,
and format gates passed. The report-only review raised a potential finding (`D1455316`):
whether a non-`backlogit` tool identified by an alias can point at
`bin/backlogit.exe` without the ID-based `--cwd` binding check.

Stage evaluated `D1455316` against the locked U9 test suite
(`TestU9_ReviewFixMCPArgumentsAndBindings` in
`internal/toolchain/verify_test.go:390-430`) and confirmed it is **out of scope
for `212.002-T` (U9) under P-021 C1**:

* In `TestU9_ReviewFixMCPArgumentsAndBindings` (`verify_test.go:390-430`), the
  locked test harness explicitly sets `m.Tools[0].ID = "launcher"`,
  `ExecutablePath: "./bin/backlogit.exe"`, and `WorkspaceBindingArgs: nil`
  (no `--cwd` binding) across `entrypoint-present`, `script-flag-matches`,
  `no-entry-subcommands`, and `casefold-env-binding`, asserting `valid: true`
  (`Status != "violation"`).
* Requiring `--cwd` on non-`backlogit` IDs that use `./bin/backlogit.exe` as a
  test fixture binary would directly break those 4 locked U9 subtests.
* Therefore, `D1455316` is retained in `.backlogit/stash.jsonl` alongside
  `00FD5168`, `8841FD73`, `1406FAE2`, `CF190A0C`, `58AA0592`, `5F7A81D7`, and
  `2C71DFA2` as a P-021 C1/C2 deferred scope expansion (requiring future
  test-fixture refactoring in a separate work unit).
* `212.002-T`'s Step 4.4 Review Gate is **SATISFIED (`READY_WITH_FOLLOWUPS`,
  `P0=0, P1=0`)** covering the current HEAD so `_Ship` proceeds directly to
  **Step 4.5 (`Complete Task` -> `done` with
  `commit_sha="a435bbb6f98b009450e7cd66d8b5b6af6a1f4085"`)** and **Step 4.6
  (`Wave 2 Convergence Gate`)** without re-running `review` on `212.002-T`.
