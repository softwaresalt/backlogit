---
name: runtime-verification
description: "Validate affected runtime surfaces after build and CI using the lightest verification that still provides confidence"
argument-hint: "surface={cli|api|browser|background-job|auto} [target=...] [mode={manual|api|browser|auto}]"
---

# Runtime Verification

Use this skill after build and CI when green tests are necessary but not sufficient for confidence.

## Inputs

* `surface`: Required. One of `cli`, `api`, `browser`, `background-job`, or `auto`.
* `target`: Optional command, path, URL, or subsystem.
* `mode`: Optional, defaults to `auto`.
* `context`: Optional PR, task, feature, or branch context.

## Output

Write the verification report to `docs/closure/{YYYY-MM-DD}-{slug}-runtime-verification.md`.

## Required protocol

1. Determine the affected runtime surface.
2. Choose the lowest-cost verification that still gives confidence.
3. Record what was verified, how it was verified, expected behavior, observed behavior, and evidence.
4. Return one status: `PASS`, `PASS WITH FOLLOW-UP`, or `FAIL`.
5. Hand follow-up context to `operational-closure` when appropriate.

## Quality criteria

* Verification matches the surface actually changed.
* Evidence is specific enough to reproduce.
* Manual verification is used honestly when automation is not available.
