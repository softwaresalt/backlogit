---
title: "213-S U9 harness fixture conflict"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-fixture-conflict.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 213-S
feature_id: 212-F
status: resolved
---

## State

- Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`; starting HEAD `a7e764ae`.
- Task `212.002-T` remains active and already started; do not claim or start it again.
- The existing `u9Manifest` fixture declares `--cwd` while `u9PinnedWorkspace` omits it from MCP arguments. The existing conformant test expects that configuration to pass, contrary to the requested binding enforcement.
- Build-feature stopped rather than modifying the existing fixture to reach green. Three newly added U9 regression groups were observed red; their uncommitted changes remain in `internal/toolchain/verify_test.go`. No production fix or commit was made.
- Wave 2 review and convergence, Wave 3, PR and post-merge closure remain outstanding. Route the conflicting harness contract to its owner or Stage for a governed resolution before resuming.
- No new structured checkpoint was created. Initial checkpoint enumeration found no active Ship checkpoints.
