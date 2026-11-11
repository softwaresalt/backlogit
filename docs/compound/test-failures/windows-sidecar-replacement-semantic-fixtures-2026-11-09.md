---
chunk_strategy: h1-h2-h3
title: "Keep Windows sidecar replacement tests semantic, not throughput-bound"
description: "Separate a real atomic replacement witness from deterministic retry-contract assertions"
doc_type: learning
schema_version: "1.0"
source: docs/compound/test-failures/windows-sidecar-replacement-semantic-fixtures-2026-11-09.md
problem_type: "flaky-test"
category: "test-failures"
component: "internal/cli"
root_cause: "The fixture required 256 atomic publications inside a fixed deadline while continuously opening readers"
resolution_type: "code_fix"
severity: "medium"
message: "U6_REGULAR_REPLACEMENT_DEADLINE"
file_path: "internal/cli/mcp_session_shim_test.go"
citations:
  - "docs/memory/2026-11-09/212-S-u6-regular-sidecar-replacement-blocked.md"
  - ".backlogit/archive/211.006-T.md"
tags:
  - windows
  - atomic-replacement
  - regression-tests
  - contention
---

## Problem

A sidecar-reader regression test sometimes completed 255 of 256 required
publications before its 15-second deadline. It failed after a successful
reader fix despite no observed reader validation error in that run.

## Root Cause

The fixture added a writer-throughput requirement that the task did not
authorize. Sustained readers can contend with Windows replacement. This
fixture-design diagnosis does not establish a host-process cause or explain
other shipment timeouts.

## Resolution

Require one real atomic identity replacement, complete records before and
after publication, and successful concurrent reads. Yield between readers
so the writer has an opportunity to publish. Retain the original deadline.
Supplement the behavioral witness with source-contract assertions for the
otherwise timing-dependent four-attempt, regular-only retry and immediate
unsafe-type refusal. Reconfirm RED against the pre-fix implementation.

The corrected exact harness, combined race regressions and three fresh
cold-start repetitions passed. These scoped results do not imply that the
unfiltered shipment suite passed.

## Prevention

* Derive fixture completion from the behavior contract, not an arbitrary count
* Keep genuine replacement identity and complete-record assertions
* Distinguish reader assertion RED from writer-contention or deadline failures
* Record source-contract RED separately from behavioral RED
* Never weaken unsafe-file refusal or infer root cause from a timeout snapshot
