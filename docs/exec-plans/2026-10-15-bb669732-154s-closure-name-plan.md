---
chunk_strategy: h1-h2-h3
description: 'Plan to conform the 154-S closure record name to the pipeline-topology gate pattern so the 141-S pre_claim predecessor check can recognize it (stash BB669732).'
doc_type: plan
schema_version: "1.0"
source: docs/decisions/2026-10-15-bb669732-154s-closure-name-deliberation.md
title: 'Implementation Plan: Conform 154-S closure record name for the 141-S pre_claim gate'
---

# Implementation Plan: Conform 154-S closure record name for the 141-S pre_claim gate

## Objective

Make the `154-S` predecessor closure record recognizable to
`autoharness gate pipeline-topology --shipment 141-S --phase pre_claim`, and
confirm the gate result. Source decision:
`docs/decisions/2026-10-15-bb669732-154s-closure-name-deliberation.md`
(Option A).

Requires plan hardening: no. Rationale: docs-only rename with unchanged record
content, no code, schema, lifecycle, or security surface, and trivial rollback
(`git revert`).

## Constraints

* No dependency edges to or from `176-S`, `157-S`, `158-S`, or other
  shipments. The shipment manifest of those shipments is not touched
* Docs-only. No source, test, or config file changes
* The renamed record's front matter and body stay byte-identical except for the
  rename note in the body

## Implementation units

### U1: Rename the 154-S closure record (docs)

* `git mv docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md docs/closure/154-S-173-F-post-merge-closure.md`
* Append a short `Rename note` section recording the old name and stash
  `BB669732`. Do not edit front matter
* Acceptance criteria
  * The new file exists, the old path does not, and `git diff -M` shows a
    rename with only the appended note
  * Front matter keys `closure_status`, `compaction_status`, `conditions`,
    `shipment_id`, and `feature_id` are unchanged
* Verification: `git diff -M --stat` shows a rename and the front matter block
  diff is empty; the only added bytes are the appended section
* `backlogit docs lint` reports no new findings for the renamed record
* Posture: characterization-first

### U2: Repoint machine-read source references (docs)

* Update the `source:` front matter that names the old path in
  `docs/closure/2026-09-30-154-s-runtime-verification.md`,
  `docs/closure/2026-09-30-154-s-compound-refresh.md`, and
  `docs/memory/2026-09-30-ship-154s-post-merge-closure.md`
* Leave narrative citations in historical memory, decision, exec-plan, and the
  197-S closure documents unchanged
* Acceptance criteria
  * No `source:` front matter value points at the old filename
  * No other line in those three files changes
  * A repo-wide search for the old stem across all file types and hidden
    directories (`rg -uu -F 154-S-173-F-scheduler-baseline-marker`) returns
    only allowed narrative citations and the rename note
  * `backlogit docs lint` reports no new findings
* Depends on U1. Posture: characterization-first

### U3: Post-merge closure obligation, re-probe the successor gate (no backlog task)

* On the merged clean `main` tree, run
  `autoharness gate pipeline-topology --mode agent --shipment 141-S --phase pre_claim --json`
  read-only. Record the exit code, token, and blocking predecessor in a short
  closure note. Do not claim `141-S`
* Acceptance criteria
  * The result for `154-S` is recorded verbatim (recognized, or the exact
    remaining token)
  * Any new blocker (for example `197-S` or condition content) is captured as a
    stash entry and not fixed in this chore
  * Expected outcomes are exit 0 or a named next blocker. Either way the chore
    closes, and a new blocker only produces a stash entry
  * The initial token (`UNRECOGNIZED` versus `INCOMPLETE`) is confirmed or
    corrected in the note
* Not a task in shipment 202-S: it needs the merged rename on clean main, and
  Ship runs every manifest task before the PR lifecycle, so a manifest task
  would depend on its own merge. Ship performs it as a mandatory post-merge
  closure obligation for 202-S
* Depends on U1 and U2 being merged

## Dependency order

`U1 -> U2`, then the post-merge closure obligation U3

## Effort

2 tasks x under 2 hours each, one skill domain (docs).

## Rollback

`git revert` of the merge commit restores the old filename. No state outside
git is changed.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* decision: ADVISORY
* attempt: 1
* reviewers: Scope Boundary, Correctness, Schema-CLI-Docs Coupling lenses (one reviewer agent)
* findings: lint and repo-wide stem search ACs added, U1 hash step replaced by diff check, U3 reframed as Ship post-merge verification. Copilot review then showed a circular manifest dependency, so U3 left the task manifest and became a post-merge closure obligation. Findings applied.
* operator_authorization: approved (dark mode P-017, Stage judgment under pre-authorization)
