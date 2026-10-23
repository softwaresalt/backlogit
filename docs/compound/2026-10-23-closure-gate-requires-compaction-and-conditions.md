---
chunk_strategy: h1-h2-h3
doc_type: compound
docline:
  date: 2026-10-23T23:30:00Z
  status: final
  tags:
    - closure
    - pipeline-topology
    - gate
schema_version: "1.0"
source: docs/compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md
title: "Closure gate requires compaction_status done and a conditions block"
---

# Closure gate requires compaction_status done and a conditions block

## Problem

The successor `pipeline-topology` `pre_claim` gate reads the predecessor closure document. A closure with `compaction_status` other than `done`, or `closure_status: READY_WITH_CONDITIONS` without a machine-readable list of conditions, blocked the next shipment even though the work was merged.

## Rule

* Set `compaction_status: done` (P-020) after compact-context runs, even when there were no candidates.
* Prefer `closure_status: READY` plus stash follow-ups when no condition is genuinely unmet.
* If `READY_WITH_CONDITIONS` is real, add a top-level frontmatter `conditions:` list at closure time; each entry needs `id`, `satisfied`, and `evidence`.
* Never edit the gate inputs to pass; record real evidence only.

## Verification

After the closure PR merges, run `autoharness gate pipeline-topology --mode agent --shipment <next> --phase pre_claim --json` from synchronized `main` and require exit 0 before reporting done.
