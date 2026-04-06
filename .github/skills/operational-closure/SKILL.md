---
name: operational-closure
description: "Produce release-readiness, monitoring, rollback, and follow-up artifacts after implementation and verification"
argument-hint: "mode={pre-merge|post-merge|post-deploy} context=... [verification_report=...]"
---

# Operational Closure

Turn implemented work into safely absorbed work. This skill records the practical closure details that matter after code review, CI, and runtime verification.

## Inputs

* `mode`: Required. One of `pre-merge`, `post-merge`, or `post-deploy`.
* `context`: Required. PR, task, feature, or release context.
* `verification_report`: Optional path to a runtime verification report.

## Output

Write the closure artifact to `docs/closure/{YYYY-MM-DD}-{slug}-closure.md`.

## Required protocol

1. Gather the change summary, CI status, unresolved review items, and runtime verification results.
2. Build a closure checklist that includes healthy signals, failure signals, monitoring plan, rollback trigger, validation window, and owner.
3. Return one readiness state: `READY`, `READY WITH CONDITIONS`, or `BLOCKED`.
4. Feed durable learnings back into `compound` or documentation updates when appropriate.

## Quality criteria

* Closure artifacts contain concrete signals.
* Rollback triggers are actionable.
* Ownership and validation windows are explicit.
