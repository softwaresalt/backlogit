---
title: "Ship 210-S served-root handoff and pre-build halt"
doc_type: memory
schema_version: "1.0"
shipment_id: 210-S
feature_id: 209-F
status: blocked
---

## Served-root handoff outcome

* Shipment: `210-S`, already active; no claim was attempted
* Branch: `feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`
* Initial HEAD: `8f913f5548ba8c270f977d4b95a197d1bf3b2ead`
* Served workspace root: `.`
* Served storage root: `.backlogit`
* Served index: `.backlogit/backlogit.db`
* Explicit ordered members: `209-F`, `209.001-T`, `209.002-T`, `209.003-T`
* MCP catalog workspace and storage roots matched the supplied canonical roots
* MCP `pragma_database_list` returned exactly the expected main database
* Independent post-claim topology check passed, including sole active shipment,
  branch ownership, predecessor readiness, and single-worktree topology

## Startup evidence

* Backlog version and read-only SQL probes succeeded; index sync indexed 2,159 items
* Recovery scan returned 118 checkpoints, no active Ship candidates, and zero
  quarantine anomalies
* Hook poll returned no concrete events or derived signals
* Fresh routing resolution succeeded for
  `gpt-6.1-sol / openai / medium / long_context`
* Engram workspace readiness timed out twice; permitted direct-file retrieval
  fallback was declared, without changing daemon or index state
* Graphtor status succeeded: three sources, 197,739 chunks, schema version four
* Intercom is not installed; remote visibility is degraded, not approval
* Baseline `go test -run=^$ -count=1 ./...` passed using `GOTOOLCHAIN=go1.26.5`

## Condition b evidence

The shipment predecessor closure reaches `154-S` through
`209-S -> 208-S -> 207-S -> 206-S -> 205-S -> 154-S`.
The required leading-line format was published:
`CONDITION_B_ATTESTED: <evidence>`.

Scoped P-012 raw reads were declared for `.backlogit/archive/154-S.md` and
`.backlogit/logs/154-S.jsonl`, because configured tools do not expose the full
authoritative event stream. Both reads used the operator-supplied Windows
no-follow helper, opened-path verification, root containment checks, and
reparse-component rejection after MCP served-root attestation.

The Markdown records `status: archived` and `archived_status: shipped`.
The log records shipment status `shipped` at
`2026-09-30T08:44:25.1831744-07:00`, followed by an `operator` comment at
`2026-10-06T23:31:36.0953115-07:00` whose leading line conforms to the attestation
format. No later revocation was present in the parsed 30-event log.

## Pre-build contract defect

`DARK_MODE_HALTED`: P-002 plan defect detected before harness generation,
task start, or implementation.

The authoritative
[implementation plan](../../exec-plans/2026-10-30-declarative-shipment-reconstitution-plan.md),
U1 RED phase, line 61, instructs `harness-architect` to declare exported types and
stub `ReconstituteShipments` before observing the failing behavior harness.
The [mandatory policy](../../../.github/policies/workflow-policies.md), P-002.1
cycle-31, explicitly identifies production stubs ahead of their harness as a
plan defect to report. New declarations require a compiling, assertion-red
source-shape harness before production declarations land.

The operator required the committed plan followed verbatim. Ship therefore
cannot silently replace its RED-phase contract or amend the plan. Stage owns
the amendment. No policy waiver, harness exemption, or production stub was
created.

## Preserved execution state

* Completed tasks: none
* Pending tasks: `209.001-T`, `209.002-T`, `209.003-T`
* Shipment and members remain in their pre-existing active state
* No `WORK_STARTED` comment was appended
* No task log was read and no harness/build dispatch occurred
* No task-type wave set was frozen; the intake member list is not `M`
* No source code, plan, deliberation, or review artifact was changed
* No implementation PR, merge SHA, closure PR, or follow-up stash was created
* P-020 post-merge compaction is not reached; this is not release closure

The pre-existing modified files were `.backlogit/hooks_queue.jsonl`,
`.backlogit/queue/209-F.md`, `.backlogit/queue/209.001-T.md`,
`.backlogit/queue/209.002-T.md`, `.backlogit/queue/209.003-T.md`, and
`.backlogit/queue/210-S.md`. Preserve them; do not reset, stash, or discard.

## Next step

Stage must amend the U1 test-first contract to require source-shape RED before
new production declarations and preserve prior RED coverage for behavior.
After that amendment, resume the same shipment branch through the
operator-selected checkpoint recovery protocol. Do not reclaim `210-S`.
The existing dark-mode merge authorization covers both shipment PRs only after
all current-HEAD review, CI, Copilot, merge-strategy, and topology gates pass;
admin fallback remains unauthorized.
