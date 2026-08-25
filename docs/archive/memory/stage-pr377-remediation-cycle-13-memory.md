---
chunk_strategy: h1-h2
description: "PR #377 Copilot review remediation cycle 13 — U8b current-head synchronization"
doc_type: memory
schema_version: "1.0"
title: PR 377 Remediation Cycle 13
---

## Context

PR #377 review cycle 13 (operator-authorized extension past the three-cycle limit). Two current-head Copilot review comments on `ee13e2e9` required synchronization of plan and task U8b.

## Findings and Corrections

### Comment 1 — Stale byte-identity universality (plan ~line 830)

The plan's byte-identity postcondition paragraph still stated "every fixture file is byte-identical on disk after all three surfaces have been exercised." This contradicted the corrected U8b contract established in cycle 11: accepted `abandon` necessarily mutates its fresh fixture.

**Fix**: Byte-identity postcondition paragraph rewritten to apply only to refused-mutation paths (rows 1 and 2). The `conforming-active` row's accepted `abandon` asserts the intended rewrite/archive outcome, not byte identity.

### Comment 2 — `conforming-active` row expected-red misclassification (147.016-T)

The expected-red section claimed all three `conforming-active` assertions were "pre-existing shipped behaviour" that pass from the moment they land. This was wrong:

* `GetCheckpointResult` begins as a zero-value declaration stub — `Conforming: false`, not `true`
* MCP `get_checkpoint` payload lacks `conforming` field before U6c/147.022-T
* CLI `checkpoint get` prints hardcoded `"valid": true` with no conformance field before U8c/147.027-T

Therefore the three `conforming: true` get assertions are **RED** until U6b/U6c/U8c land. Only accepted `abandon` is already-green.

**Fix**: Plan and 147.016-T corrected. `conforming-active` row's three `get` assertions reclassified as RED. Acceptance criteria updated to reflect the wider red load (rows 1, 2, plus conforming-active get assertions).

## Files Modified

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — U8b expected-red section, byte-identity postcondition, cycle 12+13 remediation sections
* `.backlogit/queue/147.016-T.md` — expected-red narrative and acceptance criteria
* `.backlogit/checkpoints/checkpoint-20260824-191617.json` — review_remediation, resume_hint, memory_path
* `docs/memory/2026-08-24/stage-pr377-remediation-cycle-13-memory.md` — this file

## Shape

26 queued tasks / 42 queued-to-queued edges / 27 shipment members / ready 147.001-T. No task/edge/member change.

## Compaction Assessment

docs/memory measured after adding this file. Compact-context trigger evaluation recorded in checkpoint.

## Next Steps

* Ship or operator pushes, replies, resolves threads, merges under PR #377 merge gate
* Any further review with new P0/P1 findings needs another explicit operator decision
