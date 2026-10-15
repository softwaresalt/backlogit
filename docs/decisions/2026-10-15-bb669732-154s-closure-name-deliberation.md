---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash BB669732: the 141-S pre_claim pipeline-topology gate reports PREDECESSOR_CLOSURE_UNRECOGNIZED for predecessor 154-S because its closure record name does not match the expected pattern.'
doc_type: decision
schema_version: "1.0"
source: docs/closure/197-S-197-F-post-merge-closure.md
title: 'Deliberation: 154-S closure record name versus the pipeline-topology gate'
---

# Deliberation: 154-S closure record name versus the pipeline-topology gate

## Problem frame

* Stash `BB669732` (bug, high) reports that
  `autoharness gate pipeline-topology --shipment 141-S --phase pre_claim`
  exits 1 for predecessor `154-S`. The 197-S closure record (Successor gate
  section) captured the token `PREDECESSOR_CLOSURE_UNRECOGNIZED` on a clean
  committed tree.
* The 154-S closure record is
  `docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md`.
  The gate expects `{shipment_id}-{feature_id}-post-merge-closure.md`, so the
  expected name is `154-S-173-F-post-merge-closure.md`.
* The 141-S predecessor list is `140-S`, `154-S`, `197-S`. `140-S` and `197-S`
  already conform. The gate stops at the first blocking predecessor, so `197-S`
  recognition is unverified until `154-S` resolves.
* The operator prompt described the token as `PREDECESSOR_CLOSURE_INCOMPLETE`.
  The only recorded evidence (stash text and 197-S closure) says
  `UNRECOGNIZED`. A re-run from this dirty working tree returned
  `WORKTREE_DIRTY` (unrelated uncommitted operator and Ship files), which
  precedes the predecessor check, so the token was not re-observed in Stage.
  The plan therefore makes token confirmation a first acceptance criterion
  instead of assuming either token.
* The 154-S record carries `closure_status: READY_WITH_CONDITIONS` with an
  attested condition (b). A rename must not alter that front matter. If the gate
  returns `INCOMPLETE` after the rename, the cause is the condition or status
  content and is a separate finding.

## Options

### Option A: rename the 154-S closure record (chosen)

* `git mv` to `154-S-173-F-post-merge-closure.md`, content unchanged
* Update machine-read `source:` front matter that points at the old path
* Cost: one small docs chore. No gate or source change. Reversible
* Risk: dangling narrative citations in historical memory, decision, and plan
  documents. Mitigation: leave a rename note in the renamed record and keep
  historical narrative citations unchanged, since they describe past state

### Option B: amend the gate pattern upstream in autoharness

* Correct in principle, but the gate is outside this repository and Stage cannot
  change it. Shipment `183-S` already carries the upstream predecessor-defect
  hand-off. Not a fix that unblocks 141-S now

### Option C: add a second conforming copy

* Duplicates a closure record and its attested condition. Two sources of truth
  for one closure. Rejected

## Chosen direction

Option A, as a single chore with three tasks. It runs as its own queued
shipment with no dependency edges to or from `176-S`, `157-S`, `158-S`, or any
other shipment, so the declared ship order `176-S -> 157-S -> 158-S` is
unchanged. Operators choose when to claim it. It is a prerequisite only in the
sense that `141-S` cannot pass `pre_claim` until the rename lands. That edge is
intentionally not recorded, per the sequencing instruction.

## Open questions

* Other closure records with non-conforming names (`153-S`, `155-S`, `195-S`,
  `196-S`) are out of scope. They are not 141-S predecessors. Captured as a
  deferred scope expansion stash entry.
* The task 3 verification may reveal a next blocker (for example `197-S`
  recognition or condition content). Those findings are captured as stash
  entries, not fixed inside this chore.

## Notes

* The probe needs a clean tree on the branch the gate expects. Ship runs it
  after merge on clean `main`.
