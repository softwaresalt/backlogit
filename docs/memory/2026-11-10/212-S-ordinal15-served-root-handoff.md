---
title: "212-S Ordinal 15 served-root handoff and recovery"
doc_type: reference
source: docs/memory/2026-11-10/212-S-ordinal15-served-root-handoff.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
---

# Handoff and recovery

- Scope: `212-S` / `211-F`, existing shipment branch; no claim, restart, or task completion repeated.
- TOOL_OK: backlogit version, metadata catalog, SQL, checkpoints, and hook polling. INDEX_SYNC_OK: 2266 artifacts. Fresh routing resolution passed schema validation with no degradation.
- Served workspace: `.`; served storage: `.backlogit`; metadata catalog matched both supplied roots. MCP `pragma_database_list` returned exactly one main database, `.backlogit/backlogit.db`.
- Live shipment: active, 23 ordered manifest members matching the Orchestrator handoff, covering feature `211-F`, 22 terminal-success tasks, open RED empty. No refusal labels.
- Explicit recovery selection and confirmation: Orchestrator invocation under operator "Proceed." selected `checkpoint-20261110-070156.json`. Its pre-restore SHA-256 matched `f28f141f3507bbe8634dbb81a74bf7b9ba7bc0d7e84fa118639515420ecca1a8`; MCP confirmed valid, conforming, Ship-owned state.
- Bounded prune-on-restore: reachable, workspace-bound Engram state read; superseded traces are not replayed. Preserved active shipment cursor, selected checkpoint pointer, historical gate verdicts, Ordinal 15 authorization, and cumulative failed count 12. Resumed at Wave 12 convergence, then resolved only the selected checkpoint.
- Protected `checkpoint-20261106-210317.json` remains untouched, SHA-256 `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
- Stage custody note SHA-256 matched `e735fe7e783fd2d671a38a7df57a160cd4deb65fac35f25b8128b70bcf921f2f`; all six listed artifact digests verified. Ship accepts these Stage-authored changes without modifying planning content.
- Stage timing disposition: U2j four-selector expectation is less than 300 seconds with `-timeout=5m`; historical measured PASS runs were 233.702 and 294.496 seconds. No claim that the superseded 180-second target passed.
- Next: commit this custody/recovery state, require completely clean HEAD, issue fresh coordinator receipts for authorized-unconsumed Ordinal 15 (`failed_count=12`, 2700-second budget), and run the pinned recorder (SHA-256 `9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e`) over all 46 first-party packages.
- Full build, current-HEAD standard/adversarial review, PR readiness, required CI, Copilot, runtime validation, and operational closure remain pending. No merge authorization; no push or PR yet.
