---
chunk_strategy: h1-h2-h3
description: Workspace-specific installer path correction and bounded validation.
doc_type: guide
schema_version: "1.0"
source: docs/memory/2026-11-05/pr643-local-installer-bin-memory.md
title: PR643 local installer bin correction
docline:
  recorded_at: "2026-11-05T20:22:32Z"
---

## Correction

* User-confirmed scope: this repository uses `bin`; other workspaces retain `.bin`
* Changed only the binary-directory assignment in `tools/mcp/install.ps1`
* Preserved missing-binary guard, frozen lockfile, ignored scripts, copyfile
  backend, and native exit propagation
* Existing branch: `stage/212-S-multi-session-mcp-runtime`, PR643
* No task or shipment claimed; no checkpoint restored or lifecycle changed

## Validation

* Before edit: isolated real path setup and guard exited 1 for missing `.bin\bun.exe`
* After edit: PowerShell parse and isolated setup passed against existing
  `bin\bun.exe`; Bun version was `1.4.0`
* Bun, install, cache, TEMP, TMP, and TMPDIR paths remained inside workspace
  `bin`, with no reparse-point ancestors
* Existing Go 1.24.0: CLI build, `go vet ./...`, and `golangci-lint run` passed
* Formatting passed for all 1,386 tracked Go files in bounded Windows batches
* Root-wide formatting failed on intentionally invalid ignored toolchain
  fixtures; one unbatched tracked-file invocation exceeded Windows argument
  handling. Neither required source changes.
* Markdown gate: existing CLI 0.23.1 checked 7,751 files with zero issues.
  The offline npx wrapper lacked registry metadata; the installed CLI ran
  directly with identical corpus arguments, without dependency installation.
* Installer provisioning, services, package installation, and E2E were not run
* Full Go tests were not run: no Go changes; validation remained script-scoped

## Handoff

* Publish as a normal fix commit on the existing branch; no merge authorized
* Fresh Copilot review and current-head CI are required before merge readiness
* Shared templates, dependencies, lockfiles, and other workspace paths stay unchanged
* Owned-memory compaction was a no-op; active state and other owners were preserved
