---
chunk_strategy: h1-h2-h3
title: A nested os.Root Name is not an absolute validation path
description: Windows daemon validation queried process-CWD runtime paths after a successful handle-relative file open
doc_type: learning
schema_version: "1.0"
source: docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md
problem_type: runtime_error
category: runtime-errors
component: mcp-daemon-runtime-path-validation
root_cause: Nested Root.Name was treated as an absolute path for Windows GetFileAttributes
resolution_type: code_fix
severity: high
message: inspect runtime\mcp-daemon.lock reports that the system cannot find the path
file_path: internal/cli/mcp_daemon.go
citations:
  - docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md
  - docs/compound/2026-10-27-pin-the-handle-not-the-path.md
tags:
  - windows
  - go-1.24
  - os-root
  - reparse
  - 212-S
---

## Problem

The supported Go 1.24 full suite failed existing daemon-lock tests.
`TestU3_StopMCPDaemon_NotRunning` reported a missing
`runtime\mcp-daemon.lock` path. Holder-process tests subsequently
reported EOF while waiting for readiness.

## Root cause

`storageHandle.OpenRoot("runtime")` creates a nested pinned root.
Go 1.24 stores the supplied relative name; `Root.Name()` is `"runtime"`,
not the canonical absolute runtime directory.

The daemon successfully creates and stats the lock through that handle.
It then constructs a relative path from `runtimeRoot.Name()` and passes
it to `rejectLink`. Windows reparse validation calls
`GetFileAttributes` with that path, resolving it against process CWD.
The API inspects a different location and returns `ERROR_PATH_NOT_FOUND`.
The same construction occurs in sidecar validation.

Holder acquisition exits before writing its readiness line. The parent's
stdout reader therefore reports EOF; that symptom alone hides the
original path-validation failure.

## Resolution boundary

Stage authorized the bounded in-feature prerequisite as U1b, `211.008-T`,
through `085-DL` and plan D10/M1-M3. Ship validated the formal M7-to-M8
amendment before continuing; it did not silently broaden U1 or admit U5.

Harness commit `b86dadec9e22294def4d922cf6e873bc9b097616` added a sequential
default-entrypoint regression and non-skippable malicious lock and sidecar
junction fixtures. It compiled and assertion-failed before production edits.
Repair commit `834972ab9108321c79c155022c5daaa9a9857d7f` threads the verified
absolute runtime directory through leaf validation while retaining pinned
relative filesystem operations, fail-closed checks, and opened-object identity.
The legacy wrapper rejects relative root names instead of guessing a base.

The exact guarded task then passed with four security refusals and zero
skips. Supported Go 1.24 race regressions covering the task and existing
daemon suites passed with native exit 0 in 60.137 seconds.
Whole-repository compile, vet, pinned lint, and source-format checks passed.
The unfiltered run subsequently confirmed CLI package PASS in 324.828
seconds. It exited 1 overall on independent Git-discovery, lifecycle-fixture,
and Windows replacement observations outside this repair. Core exhausted
the unchanged 30-minute package budget. This proves the original daemon
failure is repaired, not that the release or full repository suite is ready.

Changing CWD, creating an unrelated CWD lock, ignoring attribute errors,
or skipping existing tests would mask the defect rather than repair it.
Existing deferred entry `C2CFE06E` identifies this exact contract surface.

## Prevention

* Never assume `Root.Name()` is canonical or absolute
* Distinguish handle-relative operations from path-based OS APIs
* Keep validation bound to the same object used for mutation
* Preserve helper stderr when diagnosing readiness EOF
* Exercise nested roots whose names differ from process CWD
