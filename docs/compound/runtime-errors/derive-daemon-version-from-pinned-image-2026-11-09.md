---
chunk_strategy: h1-h2-h3
title: "Derive daemon compatibility from the pinned image"
description: "Library hosts and the pinned daemon executable can have different build versions"
doc_type: reference
schema_version: "1.0"
source: docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md
problem_type: compatibility_mismatch
category: runtime-errors
component: MCP prewarm and session shim
root_cause: Test-host version was used to verify an independently built pinned CLI
resolution_type: code_fix
severity: medium
message: Daemon version mismatch from a library host
file_path: internal/cli/mcp_daemon.go
citations:
  - .backlogit/archive/211.006-T.md
  - docs/memory/2026-11-08/212-S-M19-waves8-10-option5.md
tags:
  - daemon
  - version
  - subprocess
---

## Problem

U6's real subprocess fixtures used a repository-built pinned CLI while
calling prewarm from a Go test host. The host and CLI resolved different
build versions, causing attachment refusal.

## Root Cause

The current process is not necessarily the workspace-pinned daemon image.
Its version is not authoritative for an independently built image.

## Resolution

U6 commit e6c2c62d7f9b4999ab81af1004925af90a681ed6 verifies the contained
regular pinned binary, then asks that image for its version when it differs
from the host executable. Compatibility checks use that result, not a
version trusted from the daemon sidecar or global PATH.

## Prevention

Keep binary identity, workspace identity and protocol verification explicit.
Exercise prewarm from both library hosts and real CLI subprocesses. Missing
pinned binary refusal, four-caller convergence and peer survival passed.
Native POSIX signal isolation remains a runtime verification follow-up.
