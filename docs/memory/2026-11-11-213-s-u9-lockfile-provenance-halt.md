---
title: "213-S U9 lockfile provenance halt"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-lockfile-provenance-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 213-S
feature_id: 212-F
status: resolved
---

## State

* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`; starting HEAD `8f7f58ae`.
* `212.002-T` remains active and already started; do not claim or start it again.
* Review-fix cycle 1 is incomplete. Uncommitted changes remain in `internal/toolchain/verify.go` and `internal/toolchain/verify_test.go`. Preserve them.
* The delegate reported the U9 race test failing on the unrelated-lockfile provenance regression. Compile, lint, and formatting passed; no fix commit was produced.
* The approved plan required a non-empty pinned version or lockfile provenance, and Stage has now specified the exact lockfile-to-tool binding contract in both `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` (Unit U9 item 2) and `.backlogit/queue/212.002-T.md` (`implementation-notes`).
* Wave 2 review and convergence, Wave 3, PR, merge, and closure remain outstanding. No structured checkpoint was created in this session.

## Stage Ruling (`212.002-T` / Unit U9 Lockfile-Provenance Contract)

* **Pinning presence rule**: A `workspace-local-pinned` `ToolEntry` must declare **at least one** of a non-empty `pinned_version` (`strings.TrimSpace(tool.PinnedVersion) != ""`) **or** a non-empty `lockfile_path` (`strings.TrimSpace(tool.LockfilePath) != ""`); if both are empty, `VerifyWorkspaceToolchain` rejects the entry (`missing version or lockfile pin`).
* **Lockfile content-binding rule**: Whenever `strings.TrimSpace(tool.LockfilePath) != ""` is specified (whether alone as lockfile-only provenance or alongside `pinned_version`), `verifyLocalTool` opens `lockfile_path` via the pinned workspace `*os.Root` handle (`openLocal`), reads its UTF-8 content, and verifies:
  1. The lockfile is non-empty (`len(bytes.TrimSpace(data)) > 0`),
  2. The lockfile content contains the tool's trimmed `ID` (`strings.Contains(string(data), strings.TrimSpace(tool.ID))`), and
  3. When `strings.TrimSpace(tool.PinnedVersion) != ""` is also set, the lockfile content also contains the trimmed `PinnedVersion` (`strings.Contains(string(data), strings.TrimSpace(tool.PinnedVersion))`).
* **Failure classification**: If any of (1)-(3) fails, `verifyLocalTool` returns a wrapped `blerrors.ErrValidation` (`invalid lockfile provenance`), causing `VerifyWorkspaceToolchain` to record a `violation` finding (`Status: "violation"`, `ContainmentClass: NonConformant`, `Code: "local_tool_invalid"`).
* **Handback to Ship**: Resume `213-S` at `212.002-T` review-fix cycle 1 using the preserved working-tree changes in `internal/toolchain/verify.go` and `internal/toolchain/verify_test.go`.
