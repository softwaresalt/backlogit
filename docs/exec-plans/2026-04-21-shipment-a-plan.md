---
chunk_strategy: h1-h2-h3
description: Operator scope clarification applied post-harvest
doc_type: plan
docline:
    ms.date: 2026-04-21T00:00:00Z
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-21-shipment-a-plan.md
title: Shipment A Scope Refinement
---

## Scope Refinement — Install Story (2026-04-21)

Operator clarified the install experience for 039-F:

1. **Primary path**: Download standalone binary, place anywhere, add to PATH
2. **Convenience**: Single copyable one-liner (curl|sh, irm|iex) — like uv/rustup patterns
3. **NOT**: Complex install scripts with package manager logic

Tasks updated:
- 039.011-T: Renamed to "Create One-Liner Install Scripts" — minimal scripts, copyable commands
- 039.012-T: Renamed to "Rewrite Installation Documentation" — three methods ordered by simplicity

Original plan file (docs/exec-plans/2026-04-21-shipment-a-plan.md) was lost to ENOSPC.
Backlog items carry full context from the reviewed plan. See plan-review section in session checkpoint 008.
