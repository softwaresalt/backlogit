---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Linter rules need bounded work, bounded output and transitive relations"
description: "Copilot raised a fresh resource-exhaustion or soundness finding every round on the 142-S soundness linter; a one-batch adversarial audit by defect class avoids the extra round trips."
doc_type: learning
source: docs/compound/2026-10-16-linter-bounded-work-and-transitive-relations.md
docline:
    date: 2026-10-16T00:00:00Z
    severity: medium
    tags:
        - review
        - linters
        - performance
        - 142-S
---

# Linter rules: bounded work, bounded output, transitive relations

## Problem

PR #515 (142-S, `internal/soundness`) took six Copilot fix cycles. Almost every finding was the same defect family in a different rule.

## Learnings

* **Bound output as well as work.** Any rule that emits one finding per pair or per repeated entry needs aggregation (per subject or unit) and a hard cap with one "omitted" finding. Examples: decision-vs-unit contradictions (1024x1024), cycle findings per back edge, repeated dependency entries.
* **Deduplicate before accumulating.** Deduplicate repeated entries from one unit before appending to an owner map, or one hostile input turns a scan quadratic (100,000 identical paths).
* **Relations are transitive.** A "related" check over direct edges only wrongly blocks dependency-ordered chains (A<-B<-C). Use reachability, computed on demand and cycle-safe, rather than a materialized per-unit set, which is O(n^2) memory.
* **Cache repeated I/O per run.** Cache reference resolution (successes and failures) in the per-run context, never globally, so a later run re-resolves.
* **Reject raw `..` before cleaning.** A path cleaner hides `link/../x`, so validate segments on the raw input before any resolver runs. Every path-taking field needs the same check, not only the artifact-path field.
* **Review tactic.** After the second round of same-family findings, run one adversarial multi-persona audit of the whole package by defect class and fix the batch, instead of fixing only the cited line.

## Evidence

PR #515 review rounds 1-6; the final audit and fixes are in 0021ac2e, 3c28cc81 and bf389af4.
