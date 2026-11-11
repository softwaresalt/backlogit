---
chunk_strategy: h1-h2-h3
title: "Retain Go buffers through overlapped pipe completion"
description: "Keep pending Windows I/O buffers live until completion and cancellation drain finish"
doc_type: learning
schema_version: "1.0"
source: docs/compound/concurrency-issues/overlapped-pipe-buffer-lifetime-2026-11-08.md
problem_type: "asynchronous-buffer-lifetime"
category: "concurrency-issues"
component: "cli-mcp-ipc"
root_cause: "The OS retains a buffer after the initiating Go syscall returns"
resolution_type: "code_fix"
severity: "high"
message: "Pending overlapped I/O lacks a Go buffer liveness anchor"
file_path: "internal/cli/mcp_ipc_windows.go"
citations:
  - "docs/memory/2026-11-08/212-S-M19-wave7-u4-accepted.md"
  - ".backlogit/archive/211.004-T.md"
tags:
  - "windows"
  - "overlapped-io"
  - "runtime-keepalive"
---

## Problem

Review of the U4 named-pipe transport found that Read and Write passed a
buffer to a syscall-start closure, then waited separately for completion.
The race harness passed, but that did not establish garbage-collector
liveness while Windows retained the buffer. No corruption was reproduced.

## Root Cause

An OVERLAPPED operation can outlive the syscall that starts it.
The OS's reference does not itself keep the Go backing allocation reachable.
A closure used only to start I/O is not an explicit liveness anchor through
the subsequent completion or cancellation/drain wait.

## Resolution

Call `runtime.KeepAlive(data)` immediately after the operation's synchronous
completion/drain wrapper returns, before any result-handling return.
Apply it to both Read and Write. The completion wrapper must not return
while the OS can still access that buffer.

## Prevention

Review OS-owned asynchronous buffer lifetimes independently of data-race
checks. Keep completion and cancellation/drain paths in the same lifetime
analysis. Verify the liveness anchor follows every pending-I/O wait;
a call before the wait ends is insufficient.
