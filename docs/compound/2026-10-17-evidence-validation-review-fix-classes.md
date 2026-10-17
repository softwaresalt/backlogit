---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Evidence validation review-fix classes from S10"
description: "Four recurring Copilot fix-cycle classes on the S10 evidence DAG (144-S): fold mismatch with encoding/json, check-then-act caps, retry paths skipping durability checks, and seals not binding the node definition."
doc_type: learning
source: docs/compound/2026-10-17-evidence-validation-review-fix-classes.md
docline:
    date: 2026-10-17T00:00:00Z
    severity: medium
    tags:
        - review
        - evidencedag
        - 144-S
---

# Evidence validation review-fix classes

## Problem

The S10 evidence DAG (`internal/evidencedag`, 144-S) needed many review rounds. The findings fell into four classes that apply to any Go code that validates, seals or persists evidence.

## Learnings

* **Match the decoder's key folding.** `encoding/json` matches keys by Unicode simple folding (for example `U+017F` folds to `s`), so an ASCII-only duplicate-key check misses variants the decoder accepts. Apply the same fold-aware duplicate check on every decode path, in-memory ones included, not only the file loader.
* **Caps need an atomic reserve.** A check-then-act limit (for example on abandoned executor goroutines) can be overshot by concurrent callers. Reserve the slot with an atomic compare-and-swap or under a lock before starting the work.
* **Retries re-run the safety checks.** An idempotent retry that finds the prior effect already recorded must still verify validity (clock and expiry) and durability (re-store the anchor, require success). Treating "already done" as success skips both.
* **Seal the definition, not just the output.** A sealed or digest-bound value must cover the node definition it answers (identity, task, commit, validator, applicability), otherwise a valid artifact can be replayed against a different node.
* **Isolate caller state.** Hand executors deep copies and return copies of cached results, so abandoned or hostile callbacks cannot mutate later checks.
* **Audit by class.** Before pushing a fix, grep for the sibling decode, retry and cache paths that share the flaw.
