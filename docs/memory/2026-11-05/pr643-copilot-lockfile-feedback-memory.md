---
title: PR643 Copilot lockfile feedback
doc_type: memory
date: "2026-11-05"
---

## Scope and outcome

* The threadless finding in Copilot review `5788487556` is valid:
  `tools/mcp/*.lock` prevents ordinary staging of the required Bun lockfile.
* Added only `!tools/mcp/bun.lock`; preserved the existing lockfile byte-for-byte
  for tracking. No dependency upgrades, installer changes, task claims, or merge.
* Work remains on `stage/212-S-multi-session-mcp-runtime`.

## Verification

* Before the fix, ordinary dry-run staging failed because the lock was ignored.
* After the fix, the lock is not ignored and ordinary dry-run staging succeeds;
  `node_modules` and other MCP lockfiles remain ignored.
* JSONC parses; manifest name, exact dependency pins, override, and all 159
  package integrity hashes match. Public HTTPS mirror URLs contain no credentials
  or local paths. Initial screening assumptions about the mirror and drive-path
  syntax were corrected without changing package data.
* Bun frozen dry-run in a fresh workspace-contained fixture passed with scripts
  disabled, no installed packages, and unchanged source and fixture hashes.
* Lock SHA-256: `334cc369eb78e23bec9b49d8599459f146c4d194d08a940cf2c7409d44707e67`
* Go 1.24 full CLI build, `go vet ./...`, `golangci-lint run`, and diff whitespace
  checks passed. No Go sources changed; the full Go test suite was not rerun.

## Delivery and handoff

* Next: normal commit/push, then post the verified threadless disposition with
  `Copilot-Review-Body-Disposition: 5788487556` and the pushed fixing SHA.
* Request a fresh current-HEAD Copilot review; prior CI/review evidence is stale
  after the push. Do not declare merge readiness or merge.
* Preserve the existing namespace planning checkpoint and unrelated backlog.
