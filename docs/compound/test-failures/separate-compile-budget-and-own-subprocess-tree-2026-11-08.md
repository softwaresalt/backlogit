---
title: Separate compilation from short assertion budgets and own subprocess trees
description: R5 cleanup verification required precompiled race binaries, bounded pipe draining, and owned descendant cleanup.
problem_type: test_infrastructure
category: test-failures
component: integration subprocess helpers
root_cause: Compile time shared a short assertion window; helper pipe draining and descendant cleanup lacked explicit bounds.
resolution_type: code_fix
severity: high
message: U2J_INHERITED_PIPE_DRAIN_BOUND
file_path: tests/integration/claim_start_wave_sim_contract_test.go
citations:
  - docs/memory/2026-11-08/212-S-U2j-C1/manifest.md
  - .backlogit/archive/089-DL.md
tags:
  - subprocess
  - ownership
  - bounded-verification
  - r5
---

## Problem

A 25-second `go test` invocation returned no test events. A separate race-binary compile later took 30.78 seconds. This establishes the need to separate budgets, not the exact cause of the earlier startup delay.

The existing helpers used `CommandContext` without a bounded inherited-pipe drain or complete owned-tree cleanup. A finite descendant-held-pipe fixture produced genuine assertion RED, independently of the historical Go watchdog timeout.

## Resolution

Compile and hash the race-enabled test binary before timing the assertion. Supervise compiled execution through the existing suspended-child Windows Job and same-thread idle hold/clear.

The helper now owns native process identity/tree, bounds draining, preserves `ErrWaitDelay` and native errors, and reports cleanup failures. A passed in 26.67 seconds; B passed 58/58 named tests and all 33 real-Git cases in 547.54 seconds.

## Prevention

- Do not call a package panic or pre-test timeout assertion RED.
- Do not infer the historical hang's exact cause from the repaired gap.
- Give raw Go backups `.raw` filenames: copied `.go` files can become unintended packages. Preserve bytes and record relocation.
- Keep historical pre-claim SHA separate from independently captured pre/post-leaf state.
- Preserve failed observations and distinguish expected RED from unexpected failures.
