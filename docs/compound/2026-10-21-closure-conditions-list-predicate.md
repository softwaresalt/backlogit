---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "READY_WITH_CONDITIONS closures need a conditions list or the pre_claim gate blocks successors"
description: "The pipeline-topology pre_claim gate rejects a predecessor closure unless closure_status is READY, or READY_WITH_CONDITIONS with a frontmatter conditions list in which every entry has satisfied: true and evidence. Prose-only conditions block every dependent shipment with PREDECESSOR_CLOSURE_INCOMPLETE."
doc_type: learning
source: docs/compound/2026-10-21-closure-conditions-list-predicate.md
docline:
    date: 2026-10-21T00:00:00Z
    severity: high
    tags:
        - pipeline-topology
        - pre-claim-gate
        - closure
        - dark-mode
        - workflow
---

# READY_WITH_CONDITIONS closures need a conditions list

## Problem

During the 188-S to 185-S dark run, `autoharness gate pipeline-topology --phase pre_claim`
for 193-S returned `PREDECESSOR_CLOSURE_INCOMPLETE: predecessor 186-S is terminal but
missing required closure evidence`, although 186-S was archived as shipped and had a
closure file.

## Root Cause

`docs/closure/186-S-186-F-post-merge-closure.md` was `READY_WITH_CONDITIONS` and described
its three open conditions in prose only. The acceptance predicate
(`topology._closure_artifact_complete`) requires `compaction_status` of done or degraded
and `closure_status` READY, or READY_WITH_CONDITIONS with a non-empty `conditions:` list
whose entries are mappings with `satisfied: true` (literal boolean) and non-empty `evidence`.
The conditions were genuinely open, so the list could not honestly be written.

## Resolution

Do not flip conditions to satisfied without evidence. For 186-S the open checks were run:
live `_Ship` halt-row sessions in isolated fixtures, plus the Orchestrator dry routing
pass. The closure was then amended with an evidence-backed `conditions:` list (PR 558) and
the gate passed. A condition that needs operator approval (fixture deletion) was recorded
as a retained-and-deferred disposition and the choice was disclosed in the PR.

## Prevention

* When closing a shipment, either complete every condition or use `closure_status: READY`.
* If conditions remain, write them as a frontmatter `conditions:` list from the start, and
  expect dependents to stay blocked until each is satisfied with evidence.
* Run `autoharness gate pipeline-topology --phase pre_claim` for the next shipment right
  after the closure PR merges.
* `autoharness gate closure-evidence` additionally needs `close_path` and `close_evidence`,
  which the installed harness does not produce (stash 3FF72EB8).
