---
title: "Unregister completed request cancellation bridges"
description: "Avoid retaining completed request contexts under a long-lived MCP session"
doc_type: reference
source: docs/compound/concurrency-issues/unregister-completed-request-session-cancellation-2026-11-09.md
problem_type: memory_retention
category: concurrency-issues
component: MCP session dispatcher
root_cause: Completed child contexts remained registered with the live session
resolution_type: code_fix
severity: high
message: Completed requests retained until session disconnect
file_path: internal/mcp/session_mux.go
citations:
  - .backlogit/archive/211.005-T.md
  - docs/memory/2026-11-08/212-S-M19-waves8-10-option5.md
tags:
  - context
  - cancellation
  - mcp
---

## Problem

Removing normal-completion cancellation preserved the existing request-ID
reuse witness but retained completed child contexts until session exit.
Restoring immediate deferred cancellation reintroduced the observed
`context canceled` failure in that witness.

## Root Cause

A context created with WithCancel remains registered under its cancellable
parent until cancellation. A long-lived session can therefore retain
one completed context per successful request.

## Resolution

U5 commit c773fd68241268a778284d264a6a179d8482eda4 constructs the request
context with WithCancel(WithoutCancel(sessionContext)). An AfterFunc bridge
forwards live session cancellation and is unregistered when the request
worker exits. Explicit in-flight cancellation remains separate. An
already-canceled session is checked before handler admission.

## Prevention

Distinguish parent-registration cleanup from observable handler-context
cancellation. Verify explicit cancellation, disconnect, duplicate IDs and
reuse together. U3/U3a and U5 race selectors passed without changing tests.
