---
title: Keep private Windows Git fixture roots short
description: Long private TEMP prefixes can create Git ref and object paths beyond Windows limits even when the workspace itself is contained
problem_type: test_failure
category: test-failures
component: windows-test-runner
root_cause: Diagnostic TEMP prefix length combined with long test and fixture branch names exceeded Git path limits
resolution_type: workaround
severity: medium
message: Filename too long while creating fixture refs or objects
file_path: internal/core/custody_exec_test.go
citations:
  - docs/memory/2026-11-06/212-S-M11-fullsuite-circuit-handoff.md
tags:
  - windows
  - git
  - temp
  - fixture-isolation
  - 212-S
---

## Problem

An unfiltered count-1 package-serial run used a long, unique private TEMP
directory inside the workspace. Existing custody and attestation fixtures
failed Git checkout or add with explicit `Filename too long` errors.
Containment alone did not guarantee usable path lengths.

## Verified correction

Changing only the private ignored TEMP prefix to a 41-character absolute
path allowed the selected custody and attestation tests to pass, without
changing source, Git security sanitization, host long-path configuration,
or test assertions. Use a short opaque run identifier and record its
purpose in evidence rather than encoding the whole purpose in its path.

## Limits

The same bounded comparison still failed one concurrent reconstitution
replacement with AccessDenied. Short TEMP is not a universal Windows
replacement remedy and does not establish that external interference
caused the remaining failure. Do not infer full-suite success from it.

## Prevention

* Account for test names, random suffixes, storage leaves, refs, and objects
* Keep private scratch contained, ignored, uniquely owned, and short
* Preserve native errors and named-test coverage in comparisons
* Do not solve runner-path mistakes by weakening assertions or production
  path/security contracts
