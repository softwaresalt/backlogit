---
title: "Ship 210-S shape and behavior contract halt"
doc_type: memory
schema_version: "1.0"
shipment_id: 210-S
feature_id: 209-F
status: blocked
---

## Outcome

`DARK_MODE_HALTED`: the amended U1 contract still conflicts with the installed
`harness-architect` skill's declaration/behavior split requirement.
No implementation, task start, PR creation, or merge occurred.

## Evidence

* Initial HEAD: `ebdb10bde10609683a465f96e0f2f7216a8bd890`
* Branch: `feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`
* Initial working tree was clean
* The amended plan's U1 RED phase checks declarations using source-shape assertions
* Its U1 GREEN phase, lines 66-69, also implements YAML parsing, filesystem
  containment reads, validation, the dry-run planner, and post-hook buffering
* `.github/skills/harness-architect/SKILL.md`, lines 230-234, requires a
  declaration whose body reads, mutates, or writes to be split into declaration,
  behavioral harness, and implementation; it explicitly directs a halt when a
  task asks for both declaration and behavior
* `.github/policies/workflow-policies.md`, lines 133-143, permits named later
  behavioral ownership but also explicitly requires this split for
  behavior-carrying seams
* U2 names later dry-run/validation coverage, but its RED observation is specified
  against live execution returning `ErrNotImplemented`, after U1's planner and
  hook-buffer behavior have already landed

The amendment fixes the earlier production-stub-before-RED defect. This halt
concerns the remaining behavior-behind-shape-only-RED conflict. Ship cannot amend
the plan, tasks, deliberation, or governing skill to resolve it.

## Startup verification

* Backlog version, metadata, SQL, and index sync succeeded; 2,159 items indexed
* Metadata reported workspace `.`, storage `.backlogit`, and the matching database
* The main database pragma matched `.backlogit/backlogit.db`
* Full recovery enumeration returned 119 summaries, no active Ship checkpoints,
  and no observed quarantine or required-field anomalies
* Concrete hook events 4368-4371 described the Stage task amendments and were
  acknowledged through sequence 4371
* Shipment `210-S` was already active; no claim was attempted
* Post-claim topology verification passed: sole active shipment, correct branch,
  one implementation worktree, and predecessor readiness
* `GOTOOLCHAIN=go1.26.5; go test -run=^$ -count=1 ./...` passed
* Fresh config validated against the installed runtime's 1.3.0 schema; the
  plugin-bundled 1.1.0 schema was stale and not used as authoritative
* Fresh default Ship route was `gpt-6.1-terra/openai/medium/long_context`;
  this invocation explicitly selected `gpt-6.1-sol/openai/medium/long_context`
* Fresh escalation route was `gpt-6.1-sol/openai/medium/long_context`;
  no escalation circuit opened
* Engram readiness timed out twice; permitted file-read fallback was declared
* Graphtor was reachable with three sources and 197,739 chunks
* Intercom is not installed; remote visibility is degraded, not approval

## Preserved scope

No task-type wave set was frozen. No raw authoritative item log was read.
Condition-b evidence was not revalidated, and intake reconciliation and wave
admission were not completed. No harness or build skill was dispatched.
No source, plan, deliberation, or task planning field was changed.
No shipment disposition was invoked.

Completed tasks: none. Shipment and task lifecycle states remain unchanged.
The existing served-root handoff remains blocked, not done.
Implementation merge SHA: none. Closure merge SHA: none.
Follow-up stash IDs: none. P-020 closure compaction was not reached.

## Next step

Return the remaining contract conflict to Stage for amendment or authoritative
resolution. Preserve the existing branch and claimed shipment; do not reclaim.
If the active halt checkpoint remains, a later Ship session must follow explicit
owner-exclusive checkpoint selection and confirmed recovery before resuming.
The operator's dark-mode authorization covers both shipment PRs only after all
required gates pass; it does not waive this harness gate.
