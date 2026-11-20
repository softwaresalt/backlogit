---
title: "219-S pre-PR execution and review readiness summary"
description: "Session summary for Shipment 219-S (Feature 218-F) covering Waves 1-7, review remediation, and P-021 C2 deferrals."
doc_type: memory
source: docs/memory/2026-11-19/219-s-session-summary.md
ingested_at: "2026-11-20T09:32:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 219-S
  feature_id: 218-F
  branch: feat/prompt-for-target-tool-directory-install-backlog
---

## Execution Summary

* `shipment_id`: `219-S`
* `covering_feature`: `218-F`
* `branch`: `feat/prompt-for-target-tool-directory-install-backlog`
* `base`: `main` (`6a5fefa63da298f73c4173669882fe10a510f352`)
* `tasks_completed`: `218.001-T` (U1), `218.003-T` (U3), `218.006-T` (U2a), `218.008-T` (U2b), `218.002-T` (U2), `218.007-T` (U4a), `218.004-T` (U4), `218.005-T` (U5)
* `tasks_blocked`: none

## Wave Commits and Remediation

* `3fbb2e20`: P-011 carry-forward `.gsd/config.yaml` and served-root handoff memory
* `0b07211d`: `218.001-T` (U1) pinned root creation, binary install, and gitignore escaping
* `baba670d`: `218.003-T` (U3) default target tools dir and 5-source preflight
* `58014162`: `218.006-T` (U2a) manifest relative `workspace_root` and `BACKLOGIT_WORKSPACE_DIR` override
* `48cca11f`: `218.008-T` (U2b) `VerifyProposedLauncherScripts` and pre-install tooling validation
* `3d7b0f22`: `218.002-T` (U2) `.mcp.json` canonicalization, `toolchain.yaml` and launcher reconciliation, rollback
* `2b230fee`: `218.007-T` (U4a) `writeWorkspaceBootstrapDefaults` handle confinement
* `44c57cbb`: `218.004-T` (U4) interactive/flag `--tools-dir` resolution in `init`/`install`
* `0d5d26b0`: `218.005-T` (U5) `docs/cli-reference/backlogit_init.md` regeneration and drift test
* `0be1cf82`: In-scope P1/P2 review remediation (preflight, nil guards, case-folded `.mcp.json` keys, `allowPreInstall` gate)
* `db3d60da`: F-03 in-memory `RenderDefaultBootstrapFiles`, F-04 LIFO cleanup before `rollbackAll`, F-01/F-09 hardening
* `d9135b35`: Copilot Round 1 remediation and Linux socket path length fix (`internal/cli`, `internal/toolchain`)
* `dd41d4ce`: Copilot Round 2 remediation — allow quoted spaces and brackets in `launcherProfile.path`
* `c37d4e44`: Copilot Round 3 remediation — allow literal `#` and `!` in quoted `launcherProfile.path`
* `db0015e4` + forward restoration commit: Prior local-only `.gsd/harness-manifest.yaml` `config_hash` experiment superseded per operator authorization (`2026-11-20T18:54:42Z`); `.gsd/config.yaml` and `.gsd/harness-manifest.yaml` restored forward to `main` (`6a5fefa63da298f73c4173669882fe10a510f352`) so PR `#665` carries zero `.gsd/` diff against `main`

## P-021 C2 Deferred Scope Expansions

* `EF372865`: `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` updates for `tools_dir` and `.bin` default
* `CC4F1741`: `validateMCPConfigPinning` `servers` block auditing in `internal/toolchain/verify.go`
* `6470AD6F`: Cross-package consolidation of `os.Root`, JSON/YAML scanning, and ancestor reparse policy
* `63E74B09`: Unified `BACKLOGIT_WORKSPACE_DIR=""` and `.backlog`/`.backlogit` storage defaults across `init`/`install`/`core`
* `BE9AD4F1`: Reconciling carried-forward `.gsd/config.yaml` with `.gsd/harness-manifest.yaml` (`config_hash` / `gsd tune` / `TestU12_HarnessManifestNamespaceReconciliation`), retained in stash for post-closure `Auto-Tune`

## Gate State

* Local full build (`go build ./cmd/backlogit`): PASS
* `go vet ./...`, `gofmt -l`, `backlogit_docs_lint`, `make md-lint`: PASS
* `TestU12_HarnessManifestNamespaceReconciliation`: PASS after restoring `.gsd/config.yaml` and `.gsd/harness-manifest.yaml` to `main`
* Targeted `-race` suites (`./internal/config`, `./internal/toolchain`, `./cmd/gen-docs`, `./internal/cli` U1-U6/init, `./tests/integration` `TestU4`): PASS
* Local review readiness (`218-F` scope): re-verified on current HEAD (`P0=0, P1=0`, 5 P-021 C2 follow-ups stashed)
