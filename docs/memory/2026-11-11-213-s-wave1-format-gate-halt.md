---
title: "213-S wave 1 formatting gate halt"
doc_type: reference
source: docs/memory/2026-11-11-213-s-wave1-format-gate-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 213-S
feature_id: 212-F
status: resolved
---

## 213-S wave 1 formatting gate halt

* `DARK_MODE_HALTED`: the required `gofmt -l .` exits 2 on pre-existing
  invalid Go compiler testdata in ignored `bin/_authoring_gomodcache/`.
  The output also lists ignored cache and temporary files.
* Shipment 213-S and its three tasks remain active. U8 is claim-assigned,
  with no `WORK_STARTED: 213-S` record when wave admission was checked.
* U8's compilable, assertion-red source-shape harness and harness-ready
  metadata were committed at `95f30016`. No production implementation,
  task completion, PR, or merge occurred.
* Intake reconciliation passed and was committed at `0e6a4e5a`.
  The scheduler simulation passed 279/279 assertions.
* The active branch is
  `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`.
* Resolution (`2026-11-11`): Stage extended the already-decided `079-DL`
  (`DF533572`) Governed Source-Tree Formatting Gate Option A contract
  (`docs/decisions/2026-11-01-repository-format-gate-session-cache-containment-deliberation.md`)
  to Shipments `213-S`, `214-S`, and `215-S` (and Tasks `212.001-T`,
  `212.002-T`, `213.001-T`, `213.002-T`, `214.001-T`, `214.002-T`,
  `214.012-T` + `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md`).
  At `build-feature` post-loop, Ship Step 4.3, Ship Step 4.6, and Ship Step 5,
  the formatting gate is satisfied via `gofmt -l cmd internal tests` (exit `0`,
  `0` lines) + verifying all modified/new `.go` files in the branch diff
  (`git diff --name-only main...HEAD -- '*.go'`) are `gofmt`-clean with LF-only
  bytes (`0` `\r` bytes) +
  `go test -count=1 -timeout=2m -run '^TestU175_001_BaselineEol$' ./tests`
  (exit `0`) + `git diff --check` (exit `0`), recording `DF533572` (`079-DL`)
  in the PR's `Follow-ups` (`READY_WITH_FOLLOWUPS`) without creating a
  duplicate P-021 C2 stash entry.

