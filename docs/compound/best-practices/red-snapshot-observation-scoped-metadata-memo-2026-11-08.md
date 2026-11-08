---
title: "Red snapshot collection with observation-scoped metadata memos"
description: "Remove redundant ancestor metadata probes without sharing filesystem trust between observations."
problem_type: "collector_performance"
category: "best-practices"
component: "red snapshot collector"
root_cause: "Repeated PowerShell filesystem metadata probes across shared ancestor segments."
resolution_type: "code_fix"
severity: "high"
message: "Live Capture timeout; repeated Join-Path, Test-Path and Get-Item ancestor probes."
file_path: "scripts/wave-scheduler-sim.ps1"
date: "2026-11-08"
task_id: "211.016-T"
commit: "faed3b2c9c05d993f67a22c7b4635b0bf1d354e0"
citations:
  - "docs/memory/2026-11-08/212-S-option5-served-root-and-collector.md"
  - "docs/memory/2026-11-08/212-S-U2j-C1/manifest.md"
tags:
  - "powershell"
  - "red-snapshot"
  - "collector-performance"
  - "observation-scoped-memo"
  - "filesystem-metadata"
  - "directory-summary"
  - "fail-closed"
  - "quiescence"
---

## Problem

Live red-snapshot collection exceeded a 295-second timeout while processing
approximately 5,802 files and 77,000 ancestor segments. Repeated PowerShell
`Join-Path`, `Test-Path` and `Get-Item` probes revisited shared directories.
The completed follow-up measured 5,804 paths per observation; these are
different capture windows, not interchangeable census values.

Native Git ignored-policy directory summaries also needed a distinct semantic
path. Sending a directory record ending in `/` through the strict FILE validator
caused an unsafe-alias refusal. That input-kind mismatch did not justify
weakening file-path validation.

## Root Cause

Ancestor validation repeated filesystem cmdlets across paths sharing the same
directories. Validation must remain fresh across independent observations, but
verified existing ancestors can be reused within one observation.
The fix targets this redundant metadata work, not file-byte hashing.

## Resolution

`Get-RedSnapshotSafePath` uses .NET path combination and filesystem metadata
attributes with `DirectoryInfo` and `FileInfo` instead of repeated cmdlet probes.
Memoize only verified existing ancestor directories. Do not cache missing
directories or infer safety from a cached leaf identity.

Create a fresh ancestor memo for Observation 1 and another for Observation 2.
Create another independent memo for every policy fingerprint. Never share these
memos across observations, policy passes or later Capture calls.
Reuse checked leaf attributes without bypassing raw-byte file hashing.

Keep FILE and DIR_SUMMARY semantics strictly separate:

* FILE retains strict separator and alias validation, including rejection of
  trailing `/` and backslash forms.
* DIR_SUMMARY strips exactly one native trailing `/`, validates an existing
  non-reparse directory and excludes that directory record from policy-file
  hashing. It does not discard ordinary policy-file records.

Preserve reparse refusal, 8.3 and device-alias refusal, containment checks,
native Git NUL parsing, raw-byte hashes, baseline bindings, policy checks,
two-observation equality and quiescence requirements.

## Verification and limits

The live public Capture passed in **165.322 seconds**, not a few seconds.
Observation 1 hashing took 76.972 seconds; Observation 2 hashing took
7.494 seconds. Both observations independently verified 345 ancestor
directories. Earlier timeouts are not completed comparable benchmarks.
Do not infer a speedup ratio or speculate about the cause of the timing
difference from these observations.

Prior scoped verification passed 59/59 named cases, 34/34 real-Git cases and
257/257 simulation assertions. These results belong to the collector window,
not a later scheduler convergence gate.

Subsequently, Task015 used the caller-supported Compare operation and accepted
equal pre/post witnesses with SHA-256
`6c239ac28b1fe7092ecfae1e7b519f9360d1db8528ff7ee3e2414d4ffefbcd09`.
No leaf writes occurred at that comparison boundary.
This is subsequent caller evidence, not a rerun performed for this learning.

The current Wave 3 race gate **FAILED** after 11 minutes 41 seconds because
Git fixture staging reported `Filename too long`. That fixture failure is
not solved by this collector learning. Neither the prior green census nor
the equal Compare witnesses establish Wave 3/4 convergence.

Filesystem checks fail closed on observed unsafe metadata; they are not an
atomic proof against concurrent writers. Attribute checks, memo reuse and
hashing leave check/use intervals. Closed writers are required throughout the
capture and comparison boundary. Do not treat a memo or equal observations
as permission to run writers concurrently.

## Prevention

* Scope trust to one observation or policy pass, never a process-wide cache.
* Preserve strict FILE negatives when supporting native directory summaries.
* Validate using the supported public Capture and Compare entrypoints.
* Retain failed observations, gate verdicts and raw witnesses with their
  original scope; do not relabel fixture-stage failures as collector failures.
* Report measured timings and census values, not speculative benchmark causes.
* Require closed writers and retain containment, alias and raw-hash checks.
