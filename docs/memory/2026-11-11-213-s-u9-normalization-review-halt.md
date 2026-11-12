---
title: "213-S U9 normalization review halt"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-normalization-review-halt.md
schema_version: "1.0"
date: "2026-11-11"
status: resolved
chunk_strategy: h1-h2-h3
---

## State

Shipment `213-S` and task `212.002-T` are active on branch
`feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`.
HEAD is `12c37d9b1315c20f3cdf6005f9fbe9c2ac8df415`. Task `212.003-T`
remains claim-assigned, pending U9. Wave 2 has not converged; no PR or merge
was attempted.

## Evidence

The existing U9 harness was observed assertion-red before implementation.
The 12-test scoped race harness, compile-only check, lint, vet and format
checks passed after implementation. The scheduler simulation passed 279
assertions. The existing U9 start record was confirmed exactly once; no
duplicate was appended. Telemetry begin was disabled. Engram was unavailable.

The report-only review is **BLOCKED** at the recorded HEAD. A confirmed
same-contract P1 omission is that `validBacklogitTool` uses an exact ID
comparison instead of the requested trimmed, case-insensitive comparison.
The passing harness does not exercise that omission. Reviewers also reported
additional security and correctness concerns requiring validation and scope
classification; they have not been closed. Adversarial consensus was
unavailable because permitted reviewer routes could not be established.

## Next step

Do not complete U9 or advance the wave. The supplied execution boundary
prohibits further test edits; test-first remediation of the normalization
omission therefore needs an operator or Stage disposition. Preserve the
existing start record and commit. A later session must follow explicit
checkpoint selection and confirmation and re-evaluate the active,
already-started cursor under the scheduler policy; it must not append a
duplicate start or assume ordinary wave admission permits resumption.

## Resolution and recovery

* **Authoritative U9 Tool-ID & Containment Contract (Satisfied at Commit `12c37d9b` — Ready for Step 4.3/4.4 Checkpoint Resumption)**:
  * Tool and prerequisite IDs in `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`, and `validBacklogitTool` use **exact canonical ID matching** (`tool.ID == "backlogit"`, `seen[tool.ID]`, `inventory[tool.ID]`, `name == "backlogit"`), matching `schemas/workspace-toolchain.schema.json`'s canonical lowercase identifier pattern (`^[a-z0-9][a-z0-9._-]*$`).
  * Case-folded or whitespace-padded `tool.ID` / `p.ID` / server-name normalization is explicitly out of scope for U9 and tracked under P-021 C2 stash `CF190A0C` (alongside `00FD5168`, `8841FD73`, and `1406FAE2`).
  * Commit `12c37d9b1315c20f3cdf6005f9fbe9c2ac8df415` (`internal/toolchain/verify.go` and `internal/toolchain/verify_test.go`) satisfies the complete U9 contract (observed assertion-RED before `12c37d9b` and 12/12 top-level `TestU9_*` tests passing after `12c37d9b`). No further `harness-architect` or `build-feature` execution on `212.002-T` is needed; `_Ship` resumes `212.002-T` from its active checkpoint directly at **Step 4.3 Quality Gates -> Step 4.4 Review Gate -> Step 4.5 Complete Task (`done`) -> Step 4.6 Wave 2 Convergence Gate**.
