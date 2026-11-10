---
chunk_strategy: h1-h2-h3
title: "Preserve Windows retry error boundaries and handle-aware test observers"
description: "Avoid premature NTSTATUS normalization and observer-induced sharing failures."
doc_type: learning
schema_version: "1.0"
source: docs/compound/test-failures/windows-retry-error-boundaries-and-handle-aware-observers-2026-11-09.md
problem_type: test-failure
category: test-failures
component: shipment-operation-journal
root_cause: "Retry abstraction changed error identity; test readers conflicted with retained handles."
resolution_type: code_fix
severity: medium
message: "Access is denied; absent shipment journal no longer recognized"
file_path: internal/core/shipment_ops_windows.go
citations:
  - .backlogit/archive/211.021-T.md
  - docs/memory/2026-11-09/212-S-wave11-execution.md
  - docs/memory/2026-11-09/212-S-wave11-harness.md
tags:
  - windows
  - retry
  - ntstatus
  - file-sharing
  - test-observer
---

## Problem

Adding bounded retries to Windows journal operations exposed three distinct
failures. Missing journals stopped satisfying the existing not-found predicate.
A backoff observer failed to read a temp file. A directory-swap fixture failed
before its retry assertions.

## Root Cause

Normalizing NTSTATUS inside the retry helper changed the error beneath the
caller's wrapping boundary. The existing predicate extracts NTSTATUS and then
calls `os.IsNotExist`; it did not recognize the newly wrapped errno chain.

The observer used `os.ReadFile` while production retained a DELETE-capable
source handle. The observer's open did not share DELETE access, creating its
own sharing violation. Closing the production handle would conceal the fixture
defect and weaken object binding.

On this host, Windows rejected renaming the parent directory while its child
was pinned. That restriction is distinct from the ability to replace the
canonical directory after validating its directory handle but before opening
the child.

## Resolution

* Preserve raw errors through shared syscall retries. Normalize only at the
  caller's established boundary.
* Classify NTSTATUS before converting to errno: `STATUS_FILE_IS_A_DIRECTORY`
  can map to access denied but must not become retryable.
* Use a test-local read handle sharing READ, WRITE, and DELETE. Check read and
  close errors; retain all byte-preservation assertions.
* Replace the canonical directory before pinning the source child. Keep decoys
  present throughout all retry sleeps, and assert they remain untouched.
  Treat an in-flight parent-rename rejection as an observed filesystem
  constraint, not a portable requirement.

## Prevention

Run existing lifecycle regressions alongside new retry tests. Assert raw error
identity and not-found recognition through the actual open wrapper. Retain
source-handle and buffer identity checks, exact attempt counts and backoff
durations, and explicit immediate-failure controls for validation errors.

Test observers must respect the same sharing model as the operation they
observe. A genuine RED result does not by itself prove every later fixture
branch can execute; verify success-path fixture feasibility before attributing
its failure to production.
