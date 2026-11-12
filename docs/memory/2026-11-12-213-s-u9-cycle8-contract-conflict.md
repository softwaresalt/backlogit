---
title: "213-S U9 cycle 8 contract conflict"
doc_type: reference
source: docs/memory/2026-11-12-213-s-u9-cycle8-contract-conflict.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-12"
status: resolved
---

## State

* Shipment `213-S` is active; task `212.002-T` is review blocked and
  `212.003-T` is pending
* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`
* HEAD: `196f486e2a11b19135190011825495204403eefc`
* Selected checkpoint `checkpoint-20261112-162605.json` was restored and
  resolved; `checkpoint-20261112-165206.json` is the new active checkpoint
* Lifecycle topology passed

## Blocker

The authorized red setup was assertion red. The Go implementation changed, but
the 12-test harness remains red with 11 of 12 top-level tests passing.
`TestU9_ReviewFixMCPArgumentsAndBindings/backlogit-case-whitespace-remote/mcp`
expects a nil error and one violation for a padded inventory ID. Stage cycle 8
expressly requires `ErrValidation` for an untrimmed inventory ID. These
expectations cannot both hold. Request Stage contract clarification or
amendment rather than weakening the test.

## Stage Resolution

Stage clarified Cycle 8 item (a) in `.backlogit/queue/212.002-T.md` and
`docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md`: in
`validateMCPConfigPinning`, `inventory` construction checks `if id == ""`
(returning `blerrors.ErrValidation` only when `id == ""` or duplicate
`inventory[id]`, not when `tool.ID != strings.TrimSpace(tool.ID)`), while inside
`for _, name := range names`, `tool.ID != strings.TrimSpace(tool.ID)` emits a
`"violation"` finding (`finding(name, NonConformant, "violation",
"mcp_command_invalid", "Invalid or duplicate MCP server name.")`) with
`err == nil`, preserving `backlogit-case-whitespace-remote` (`require.NoError(t,
err)` and `findings[0].Status == "violation"`).

## Verification and handoff

Compile passed. Lint, format, and review were not performed. No commit or PR
was created; the task is not complete. Preserve the uncommitted `verify.go`,
`verify_test.go`, and active checkpoint file `checkpoint-20261112-165206.json`.
