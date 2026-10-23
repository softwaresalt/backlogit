---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Closure gate requires a completed compaction status and a conditions block"
description: "The pipeline-topology pre_claim gate blocks successors when a predecessor closure has a pending compaction status or READY_WITH_CONDITIONS without a machine-readable conditions list; prefer READY plus stash follow-ups when nothing is genuinely unmet."
doc_type: learning
source: docs/compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md
docline:
    date: 2026-10-23T23:30:00Z
    severity: high
    tags:
        - pipeline-topology
        - pre-claim-gate
        - closure
---

# Closure gate requires a completed compaction status and a conditions block

## Problem

The successor `pipeline-topology` `pre_claim` gate reads the predecessor closure document. A closure with a `pending` or unset `compaction_status`, or `closure_status: READY_WITH_CONDITIONS` without a machine-readable list of conditions, blocked the next shipment even though the work was merged.

## Rule

* Finalize `compaction_status` after compact-context is invoked (P-020): `done` on success, or `degraded` when the invoked run failed (non-blocking per the operational-closure skill). Never leave it `pending`, and never record a failed run as `done`.
* Prefer `closure_status: READY` plus stash follow-ups when no condition is genuinely unmet.
* If `READY_WITH_CONDITIONS` is real, add a top-level frontmatter `conditions:` list at closure time; each entry needs `id`, `satisfied`, and `evidence`.
* Never edit the gate inputs to pass; record real evidence only.

## Verification

After the closure PR merges, run `autoharness gate pipeline-topology --mode agent --shipment <next> --phase pre_claim --json` from synchronized `main` and require exit 0 before reporting done.
