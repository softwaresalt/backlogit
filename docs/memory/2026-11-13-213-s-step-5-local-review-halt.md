---
title: "213-S Step 5 local-review halt"
doc_type: reference
source: docs/memory/2026-11-13-213-s-step-5-local-review-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-13"
status: resolved
---

## Preserved execution cursor

* Shipment: `213-S`; feature: `212-F`
* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`
* Reviewed HEAD: `ad2a3ddafbfedb766def4858f35c3d9626c1a6d0`
* Completed tasks: `212.001-T`, `212.002-T`, `212.003-T`
* Open RED obligations: none
* Feature PR: not created; no merge or post-merge closure performed

The explicitly selected Ship checkpoint `checkpoint-20261113-024204.json`
was restored with bounded summarization, resolved through MCP after successful
resumption, and removed under operator authorization. Served workspace/storage
roots and the index database binding matched the handoff.

Stage's four disposition files were committed as `ad2a3dda`. No source or test
edits were made during this invocation.

## Current readiness gate

Standard local review returned `BLOCKED`, with P0=0, P1=1, P2=7, P3=5.
The supplemental three-reviewer adversarial review also returned combined
`BLOCKED`. Its security assessment confirmed the static library defect but
assessed current exposure as P2 rather than demonstrated P1 exploitation.
This disagreement does not clear the standard P1.

The blocker is the root-equivalence shortcut in
`internal/toolchain/verify.go:557-558`: an absolute `R/link/..` can be approved
after lexical cleaning to `R`, before traversal/reparse rejection. On POSIX,
if `R/link` points to `O/subdir`, filesystem resolution reaches `O`.
This is static review evidence, not a newly executed runtime reproduction.

The finding concerns the existing U9 cross-workspace containment-validation
surface. It has not been closed, waived, or classified as a captured scope
expansion. The operator's zero-source/test-edit restriction remains in force.
Stage's specific null-scalar and transport-matrix dispositions do not decide
this new finding. Related entries `58AA0592` and `5F7A81D7` are not confirmed
duplicates.

Additional unique adversarial observations about renamed interpreters and
relative-root spelling remain open for Stage/operator classification. No
permission to expand the current work unit or silently close findings is
implied.

## Quality evidence

At the reviewed HEAD:

* `go vet ./...`: PASS
* `golangci-lint run --concurrency=1 --timeout=15m`: PASS
* `go build ./cmd/backlogit`: PASS
* `go test -timeout=30m ./...`: PASS, unfiltered run exited 0
* Temporary-build cleanup: complete; no matching directories remained
* `gofmt -l .`: PASS, no output
* Memory frontmatter lint: PASS, zero violations
* Cached `markdownlint-cli2` over both memory files and Stage's plan/task: PASS

The suite uses the operator's environment, including
`GOFLAGS=-count=1 -p=1 -parallel=2`, with GOMAXPROCS, GOGC, and GOMEMLIMIT unset.
Workspace daemon PID `58268` was identity-verified and stopped under explicit
operator authorization. No engram calls occurred during the suite.
The daemon was not restarted. No build/test process remains running.

## Follow-up provenance

All 19 existing captures remain unchanged:

`00FD5168`, `8841FD73`, `1406FAE2`, `CF190A0C`, `58AA0592`,
`5F7A81D7`, `2C71DFA2`, `D1455316`, `3083FC1A`, `01D3F26B`,
`CBD2D21C`, `B7E3A649`, `8AD5625B`, `5688AB99`, `600835CA`,
`0AF43C97`, `F12B587F`, `C446F206`, `5B8497D4`.

Preserve existing `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` qualifications.
`C446F206` retains `DISCOVERY-STATUS: AMBIGUOUS 036B0CB9`;
`5B8497D4` retains
`DISCOVERY-STATUS: AMBIGUOUS 48F1E692 6BF65C9D 5F7A81D7`.

## Resume requirements

`DARK_MODE_HALTED`: no PR creation or merge while local P1 remains unresolved.
Return to Stage/operator for evidence-based severity/scope adjudication and
permitted remediation. Do not reset exhausted review-cycle counters.
Any future accepted change requires fresh quality and current-HEAD review gates.

The feature merge preauthorization is conditional on passing readiness, CI,
P-018, P-009, and P-016. It is not a waiver. The closure PR requires separate
operator approval. Shipment `213-S` remains active for P-001; do not begin
`214-S` or `215-S`.

## Stage and Operator Adjudication (READY_WITH_FOLLOWUPS, P0=0, P1=0)

* **Finding 1 — `bindingWithin` (`internal/toolchain/verify.go:557-558` `R/link/..`, captured in `6FADB54A`, `DISCOVERY-STATUS: AMBIGUOUS 58AA0592 5F7A81D7`)**:
  * **P2 Severity Adjudication (Not P1)**: Stage and the Operator concur with
    the three-reviewer `Adversarial Review` security assessment that the
    severity is **P2 (non-blocking)** because `internal/toolchain` is a
    standalone Level 1 static validator library in `213-S` with zero live
    runtime execution callers, and reaching `R/link/..` requires a local
    workspace writer who already controls `.backlogit/toolchain.yaml`,
    `.mcp.json`, and a symlink inside the workspace root.
  * **P-021 C1 Out-of-Scope Classification (Worked Discrimination Case `b`)**:
    In U9 (`212.002-T`, locked by `TestU9_ReviewFixMCPArgumentsAndBindings` at
    `verify_test.go:47,57,406-418`), the authorized `bindingWithin` contract
    validates exact workspace-root equality (`--cwd <root>` or `--cwd .`) and
    relative subpaths (`ENGRAM_DATA_DIR=.` vs `../outside`), while
    symlink/reparse escape testing in U9 is locked to `ExecutablePath`
    (`TestU9_VerifyWorkspaceToolchainRejectsSymlinkAndReparseEscapes` at
    `verify_test.go:262-300`), and expanded reparse-point / symlink binding
    permutations were deferred in `5F7A81D7`. Rejecting uncleaned `..` segments
    inside an absolute `path` prior to `samePath(path, root)` in
    `bindingWithin` handles a different syntactic form
    (`<abs_root>/<symlink>/..`) in the same function/file under **P-021 C1
    worked discrimination case `(b)`**
    (`.github/policies/workflow-policies.md:1957`).
* **Finding 2 — Renamed Interpreter Detection Beyond `isRunner` Basename Matching (`internal/toolchain/verify.go:331-348`, captured in `EC589C00`, `DISCOVERY-STATUS: AMBIGUOUS D1455316 9FC3A6A6`)**:
  * Classified out of scope under P-021 C1 (`P2`/`P3` advisory): `082-DL` and
    U9 (`212.002-T`, locked by
    `TestU9_ReviewFixMCPArgumentsAndBindings/runner-without-entrypoint-*` at
    `verify_test.go:542-569`) explicitly scope Level 1 runner detection to
    static executable basename matching (`isRunner`), while content-based /
    binary-header / magic-byte inspection of renamed interpreter binaries
    belongs to Level 2 binary provenance (`082-DL`, related to archived stash
    `9FC3A6A6` and active stash `D1455316`).
* **Finding 3 — Relative `workspace_root` Spelling (Positive Reuse of `8AD5625B`)**:
  * Positively confirmed to match existing active stash entry `8AD5625B` on the
    same contract surface (`212.002-T` / `212-F` / `213-S`); reused per
    `_ship.agent.md` Step 4.4a without creating a duplicate entry.
* **Complete 21-Item Follow-Up List**:
  `00FD5168`, `8841FD73`, `1406FAE2`, `CF190A0C`, `58AA0592`,
  `5F7A81D7`, `2C71DFA2`, `D1455316`, `3083FC1A`, `01D3F26B`,
  `CBD2D21C`, `B7E3A649`, `8AD5625B`, `5688AB99`, `600835CA`,
  `0AF43C97`, `F12B587F`, `C446F206`, `5B8497D4`, `6FADB54A`,
  `EC589C00`.
* **Merged Local Review Readiness Outcome**:
  The merged local review readiness outcome for `213-S` is
  **`READY_WITH_FOLLOWUPS` (`P0=0, P1=0, P2=8, P3=5`)** with **zero `.go` or
  `schemas/` edits**, authorizing `_Ship` to commit these Stage-adjudicated
  artifacts and proceed directly to PR creation, CI, P-018 Copilot review,
  merge, and Step 6 post-merge closure without re-running local code review
  unless `.go` or `schemas/` files change.
