---
title: "213-S U9 resumed review blocked"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-resumed-review-blocked.md
schema_version: "1.0"
date: "2026-11-11"
status: active
chunk_strategy: h1-h2-h3
---

## Cursor and gates

Shipment `213-S`, task `212.002-T`, remains active on
`feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`.
Reviewed HEAD: `0cc4bec856ef4e2a1043d2742cdc7b94a57b67ff`. Selected
checkpoint `checkpoint-20261112-074602.json` was restored with explicit
operator confirmation. The U9 twelve-test race harness, repo compile-only
check, package vet and lint, and repository format check passed.

## Review outcome

Local review is **BLOCKED**. Standard reviewers found launch-containment
gaps; adversarial report-only review returned five high-confidence consensus
findings. Relative commands and bindings can resolve against a host working
directory different from the verified root; absent backlogit binding can
select another workspace; and `bash -s` can ignore the declared entrypoint.
The review also identified versioned-interpreter and script-shebang
provenance gaps. These counterexamples are static; no launch reproduction
was run. The review requires runtime verification following remediation.

The cross-workspace and declared-entrypoint concerns implicate U9's existing
containment contract. They cannot be treated as completed or waived merely
because the twelve-test harness passes. Stage classified the remaining findings
under P-021 C1 and captured out-of-scope runtime host-launcher `cmd.Dir`
binding and Level 2 script shebang (`#!`) inspection in P-021 C2 stash entry
`58AA0592` alongside existing deferred entries `00FD5168`, `8841FD73`,
`1406FAE2`, and `CF190A0C`.

## Stage P-021 C1/C2 authorization

Stage updated `.backlogit/queue/212.002-T.md` and
`docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` (Unit U9) to
authorize resuming `212.002-T` from active checkpoint
`checkpoint-20261112-075917.json` (`phase: "step_4_4_review_blocked"`) and
running the two-step test-first Step 4.4 remediation:

* **Step 4.4 Red-Test Pass (`Go Engineer` test setup on `internal/toolchain/verify_test.go`)**:
  Append subtest cases inside `TestU9_ReviewFixMCPArgumentsAndBindings` in
  `internal/toolchain/verify_test.go` (keeping 12 top-level `TestU9_*`
  functions) covering `backlogit-case-whitespace-remote` (`id: " BackLogIt "`,
  `class: RemoteDeclared`), `backlogit-missing-cwd-binding` (`id: "backlogit"`,
  `class: WorkspaceLocalPinned`, `WorkspaceBindingArgs: nil`), `bash-s-flag`
  (`args: []string{"./scripts/server.js", "-s"}`), and
  `versioned-runner-without-entrypoint`
  (`ExecutablePath: "./bin/python3.12"`, `EntrypointPath: ""`), then run
  `go test -count=1 -run '^TestU9_ReviewFixMCPArgumentsAndBindings$' ./internal/toolchain`
  to observe assertion-RED before committing.
* **Step 4.4 Green-Code Pass (`build-feature` on `internal/toolchain/verify.go` only)**:
  Dispatch `build-feature` (`task_id=212.002-T`) against the assertion-RED
  working tree to modify only `internal/toolchain/verify.go` (without touching
  `internal/toolchain/verify_test.go`) for trimmed case-insensitive `backlogit`
  ID matching and required `--cwd` binding, `-s` and `--stdin` interpreter flag
  rejection, version-stripped `isRunner` basename matching, and explicit Level
  1 (`082-DL`) / U8 / U9 / U12 contract boundary doc comments, then confirm all
  12 top-level `TestU9_*` functions pass and commit both files.
