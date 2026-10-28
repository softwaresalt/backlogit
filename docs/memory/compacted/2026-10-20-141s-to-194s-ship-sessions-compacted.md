---
title: "2026-10-16 to 2026-10-20 Ship Sessions Compacted"
source: docs/memory/compacted/2026-10-20-141s-to-194s-ship-sessions-compacted.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

Consolidates nine verbose Ship session checkpoints for shipments 141-S, 158-S,
159-S, 185-S, 188-S, 190-S, 191-S, 192-S and 194-S. All of those shipments are
merged and closed. Originals are archived under
`docs/archive/memory/2026-10-20/`.

## Decisions and outcomes

* Dark mode (P-017) ran each shipment through harness, build, review, PR,
  Copilot review loop, merge-commit merge and post-merge closure
* 141-S: wave 1 then wave 2, all harness-first; review was single-reviewer
  (degraded) and remediated to READY
* 188-S: PR 543 merged at `d9412f90`; Copilot review produced zero threads
* 190-S: a deferred scope expansion (generic `UpdateArtifact` setting
  `disposition_*` on a queued shipment) was captured as a stash at closure
* 185-S: three tasks in one wave carried no harness label; their RED check
  was the baseline `git grep` from each acceptance criterion

## Recurring operational learnings

* The pipeline-topology gate rejects some first-choice branch slugs with
  `BRANCH_MISMATCH`; rename to the shipment-aligned slug
* backlogit MCP calls time out under load while the operation still
  completes; verify state (and use the registry CLI fallback after
  Served-Root Attestation) before retrying
* `golangci-lint` is unusable locally because of a Go toolchain
  export-data mismatch; CI is authoritative for lint
* Some tests (for example the P-021 claim serialization tests and
  `internal/core`) fail or exceed the budget under local load and pass in
  isolation or in CI
