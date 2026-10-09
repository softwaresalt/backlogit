---
title: "Stage PR 487/489 findings: provenance and continuity corrections"
description: "Complete stash provenance and CT pre-claim semantics are verified; prior validation completed and the selected Stage checkpoint is resolved."
doc_type: reference
source: docs/memory/2026-10-08/stage-copilot-findings-memory.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
ingested_at: "2026-10-09T04:48:31Z"
---

# Stage PR 487/489 findings: provenance and continuity corrections

## Scope and status

The current operator-authorized follow-up is findings A-D (six review
findings grouped by correction) on PR #489, branch
`chore/stage-copilot-findings-2026-10-08`, starting HEAD `f2900db9`.
It extends the earlier artifact-repair scope below only to the specified
provenance fields, CT wording, completion memory, and checkpoint closure.
No Go, test, config, lock, PR-body, branch, or push changes are authorized.
The earlier scope and interrupted baseline below are historical.

Operator-authorized scope is findings 1-3 on merged PR 487. Finding 4,
PR 488, existing memory, PR bodies, application code, tests, configuration,
branches, worktrees, and pushes remain outside this correction.
This exact new memory path is the exception explicitly requested at finish.

The interrupted baseline was HEAD `4f577b5a` on main, with no corrective
edit or commit made. The resumed corrections below are now applied.
The operator-owned configuration remains untouched.
Its observed SHA-256 is
`92091d6a5ea44eac34d8f0fd3a43c7cb2dd407213f65a65d9af784b9ac6ac371`.

## Verified findings and intended corrections

1. Rehydration derives harvested links from artifact
   `custom_fields.source_stash_id`, not labels or archive text
   (`internal/db/rehydration.go`, `stashRecordFromArtifact`).
   The 34 corrective archive records have no `harvested_artifact_id`.
   `CorrectStashProvenance` rejects that missing historical baseline before
   validating the canonical artifact. Do not fabricate historical harvests
   or call that API as though its preconditions held.
   Restore the harvest metadata on existing, documented task artifacts.
   There are 29 harvested entries and five non-harvest dispositions:
   `8CCB29CF` folded into active `DE3E7B67`; `8B52A5F1` and `37699341`
   duplicate/superseded; `18E587A0` and `92F79833` already fixed.
   Preserve those five honestly without invented links.
2. `198.005-T` requires full-suite verification after CX merges, while
   `198-S` has no prerequisite. Add native `198-S blocks-on 197-S`;
   preserve membership and both existing prerequisites of `152-S`.
   Update CT decision D6, the plan dependency graph, and its residual
   advisory. Queue position alone does not enforce that contract.
3. The original local full-suite capture is not tracked. The CT decision was
   the only document found referencing it. Create a sanitized durable evidence
   document and replace both decision references. The log records
   `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`,
   `gate_evidence_formal_test.go:476`, a gate-counter lock error for
   `001.001-T`, core duration `1656.232s`, and
   `EXIT=1 ELAPSED=00:28:41.1696794`. The decision records the command
   `go test ./... -count=1 -timeout 30m`. Do not rerun tests in Stage.

## Lock blocker

### Recovery decision

The operator explicitly selected and authorized resumption of
`checkpoint-20261009-044824.json` on 2026-10-08. The operator confirmed a
single-agent, single-branch, single-worktree session: no further locks are
required. The prior lock-recovery cursor below is superseded. No force
release, deletion, or new acquisition is authorized or attempted; all 31
locks from this session remain advisory and in place.

The full native checkpoint enumeration returned 46 valid summaries, no
quarantine anomalies, and exactly one active Stage candidate (the selected
checkpoint). Native retrieval confirmed its ownership and conformance.
Engram is reachable and bound to main in this workspace. Restore uses this
bounded memory state, not a replay of prior traces; the unresolved pointer
and the correction cursor are retained.

Fresh config parsing and the authoritative installed harness validation
returned no `strict_schema_blockers`; the configuration hash is unchanged.
The config's Stage route remains unsupported by this runtime; the operator
declared the gpt-6.1-sol routing degradation. No configuration edit is needed.

ProposedAction: restore only canonical artifact harvest metadata, add one
shipment blocks edge, and repair the CT planning/evidence references.
Targets: the 29 tasks listed below, 198-S, the two CT planning documents,
one sanitized evidence document, and this memory note.
Change kind: non-destructive local backlog/planning edits.
ActionRisk: moderate (provenance and execution-order contracts).
Approval: explicitly authorized by the operator in this resumption.
Rollback: revert only this correction's allowlisted local commit.
ActionResult: applied; native provenance, DAG, and doctor proofs passed.

P-010 boundary: Stage cannot execute markdownlint or any other linter.
The Orchestrator completed markdownlint on the prior 34 files with zero
issues, as confirmed by the operator for this run. That old validation wait
is complete. This run's changed Markdown requires a separate Orchestrator
lint pass; Stage does not execute or delegate it.

### Interrupted batch (historical)

The acquisition batch captured tokens in a process-local dictionary but
exited before emitting that dictionary. It stopped when asked to lock the
not-yet-created evidence document. No release tokens are available.
Do not recover tokens from lock contents or force-release without explicit
operator authorization.

Exactly 31 existing-file locks were acquired. Task locks correspond to:

```text
197.001-T 197.009-T 197.019-T 197.012-T 197.016-T
197.003-T 197.004-T 197.011-T 197.010-T 197.015-T
198.001-T 198.002-T 198.003-T
199.002-T 199.008-T 199.006-T
200.003-T 200.005-T 200.007-T 200.009-T 200.011-T 200.012-T
201.003-T 201.006-T 201.008-T 201.010-T
171.008-T 189.007-T 183.004-T
```

Each task lock is `.backlogit/queue/.<task-id>.md.lock`. The two other locks
are:

- `docs/decisions/.2026-10-08-ct-test-suite-health-deliberation.md.lock`
- `docs/exec-plans/.2026-10-08-ct-test-suite-health-plan.md.lock`

The evidence, correction-decision, and this memory file were not locked by
the failed batch. No lock was acquired on the shipment or configuration.

## Verification and next cursor

- [x] Step 0.0: native query and metadata operations reachable.
- [x] Step 0.1: native initial sync indexed 1,964 artifacts.
- [x] Step 0: fresh configuration validated against authoritative schema.
- [x] Recovery: full enumeration has no anomalies; the sole active Stage
  checkpoint was explicitly selected and authorized. Concrete hooks empty.
- [x] Targeted audit: all three findings verified against repository state.
- [x] Pre-change DAG: 1,388 blocks edges, zero cyclic nodes, zero active
  shipments, verified through native SQL.
- [x] Correction application: direct advisory-lock edits permitted by the
  operator's single-agent decision; no lock refusal encountered.
- [x] Fresh-sync provenance proof: 29/29 expected links and harvested states.
- [x] Post-change DAG: 1,389 blocks edges, zero cycles, zero active shipments.
- [x] Dependency-ready shipments: exactly 197-S; 198-S withheld.
- [x] Membership and queue positions unchanged for CT and 152-S.
- [x] Durable evidence and references repaired; docline authoring fields
  reviewed against the source contract (reference is a recognized doc_type).
- [x] Prior-batch markdownlint: 34 files, zero issues, run by Orchestrator.
- [x] Backlog doctor: exactly the 23 pre-existing orphans, no other findings.
- [x] Local correction commit: 33 allowlisted artifacts, config excluded.
- [x] Native readback validated the updated resolved checkpoint as conforming.
- [x] Session-end index refresh: 1,964 artifacts after correction commit and
  checkpoint resolution; repeat after continuity commit before handoff.
- [x] Selected checkpoint resolved after prior external validation was
  confirmed and all other review fixes completed.

Steps 1, 1.5, 1.8, 2, 3, 4, 5, 5.5, and 5.6 are inherited approved state
from the completed staging, not gates bypassed or work to replay. There is
no new intake, deliberation outcome, implementation scope, harvest, shipment,
or stash archival in this correction session. Continuous-learning and
compound capture are not triggered: this is bounded artifact repair.

The prior validation blocker is cleared. The operator explicitly selected
`checkpoint-20261009-044824.json` for owner-Stage closure after this run's
corrections. Its lock-recovery and validation-pending hints are obsolete.
Do not acquire, release, delete, or force locks.

## Applied provenance map and proof

The first correction added only `custom_fields.source_stash_id`. PR #489
review correctly identified that this recreated links but overwrote the
archived stash priority/kind/text with incomplete artifact-derived values
during rehydration. This run backfills the full normal-harvest metadata:
`source_stash_id`, `source_stash_priority`, `source_stash_kind`,
`source_stash_text`, and `source_stash_path` (`stash.jsonl`).
`source_deliberation_id` is conditional in the normal harvest; none of the
29 archived entries has that field, so it remains absent rather than
invented from free-text references.

All values come verbatim from `.backlogit/archive/stash.jsonl`.
No artifact body, acceptance criterion, label, parent, archive record, or
index/cache was hand-edited. The update MCP and CLI do not expose arbitrary
custom fields; canonical Markdown plus native sync is the supported
existing mechanism. Contracts: `internal/core/stash.go:344-353`,
`internal/db/rehydration.go:621-653`, and `internal/db/stash.go:168-181`.

Every selected task appears in its stash archive's explicit task_ids.
For entries covering multiple tasks, the existing labels/body retain the
full traceability set; the scalar field selects one of those tasks as the
canonical indexed link. Conversely, nine canonical artifacts carry multiple
stash IDs in their bodies: 197.001-T, 197.003-T, 197.004-T, 197.009-T,
197.010-T, 197.011-T, 197.019-T, 200.011-T, and 200.012-T.
`stashRecordFromArtifact` reads only one string `source_stash_id`; it neither
expands a list nor derives additional rows from labels/body text.
Their complete many-to-many relationships cannot be represented by that
scalar rehydration contract. Each of the 29 distinct stash entries does
have its own distinct canonical artifact, so all 29 canonical links and
full stash records are representable without changing Go or fabricating
extra artifacts. No new historical harvest is fabricated to bypass
`CorrectStashProvenance`'s unmet prior-harvest precondition.

| Stash ID | Canonical artifact | Fresh-sync result |
|---|---|---|
| 75E02C17 | 197.001-T | PASS |
| 52D18E44 | 197.009-T | PASS |
| 497D20E3 | 197.019-T | PASS |
| 67F17B6B | 197.012-T | PASS |
| D116AF58 | 197.016-T | PASS |
| FBD6E6F8 | 197.003-T | PASS |
| 8F1CF1E1 | 197.004-T | PASS |
| 6AB5E7FC | 197.011-T | PASS |
| F05661B1 | 197.010-T | PASS |
| 1293086D | 197.015-T | PASS |
| 8F41D60D | 198.001-T | PASS |
| D8EF5443 | 198.002-T | PASS |
| 92AB6879 | 198.003-T | PASS |
| 7D8717B1 | 199.002-T | PASS |
| 8AF55264 | 199.008-T | PASS |
| FA6AE139 | 199.006-T | PASS |
| 5247D4BC | 200.003-T | PASS |
| E4908F30 | 200.005-T | PASS |
| BACD94FC | 200.007-T | PASS |
| DBF89C9C | 200.009-T | PASS |
| AB31C9C5 | 200.011-T | PASS |
| 010F437C | 200.012-T | PASS |
| A0C733C6 | 201.003-T | PASS |
| E45E6D65 | 201.006-T | PASS |
| 0FFBF819 | 201.008-T | PASS |
| 45BD3B36 | 201.010-T | PASS |
| 97AB4978 | 171.008-T | PASS |
| E52607F5 | 189.007-T | PASS |
| 95379DBA | 183.004-T | PASS |

After the edits, the native sync timed out. Registry fallback
`backlogit --cwd . --no-update-check sync` succeeded with 1,964 artifacts
using the existing local binary. `backlogit_query_sql` then LEFT JOINed all
29 expected pairs to stash_links and stash_entries: every indexed item
matched and every entry was harvested. A separate native query counted
zero links for 8CCB29CF, 8B52A5F1, 37699341, 18E587A0, and 92F79833.

TOOL_DEGRADED: backlogit_sync_index — registered CLI fallback used.
INDEX_SYNC_OK (CLI fallback).

## Applied CT sequencing and proof

Native `backlogit_add_dependency` added only
`item_id: 198-S`, `depends_on: 197-S`, `dep_type: blocks`.
After rehydration, native SQL verified:

- Blocks edges: 1,388 to 1,389; transitive-closure cycle count remains zero.
- Dependency-ready set for the newly assembled 197-S through 201-S
  cohort: `{197-S, 198-S}` to `{197-S}`.
- 198-S stays queued but is withheld from dependency-aware queue results
  while 197-S is unresolved. Queue resolution accepts six terminal statuses,
  including abandoned; it is not an exactly-shipped guarantee.
- 152-S retains blocks edges to 154-S, 197-S, and 198-S.
- 198-S retains all six members, root first, and queue_position 150.
- 152-S retains all nine members and queue_position 400.
- Neither shipment's membership or lifecycle status changed.

Dependency readiness is not a claim authorization or a topology-gate PASS.
No claim, lifecycle change, PR edit, or push occurred.
CT D6, the plan graph, and the residual advisory record the native queue
edge and this correction's operator authority. The required after-merge
guarantee is a separate Orchestrator/Ship pre-claim check: read 197-S
immediately before claiming 198-S and require status exactly `shipped`,
not abandoned or another terminal status. `ClaimShipment` only checks
queued status and the active slot; it does not enforce dependencies.
A Go claim-time guard is noted only as a future candidate in the decision;
no new implementation scope or stash capture was required.

## Durable evidence and verification limits

New evidence:
`docs/evidence/2026-10-08-ct-full-suite-failure.md`.
Both CT decision references now point there. The evidence retains the
failing test, command, assertion location, 1656.232s package duration,
EXIT=1, elapsed time, and a short sanitized excerpt. The temporary absolute
gateproof path is removed. No new test was run and no green closure is
claimed.

The docs/exec-plans and docs/decisions reference audit found no other
independently-ready CT claim. The whole-docs literal evidence audit found
only this note's former historical scratch-file mention, now replaced.
Doctor at 2026-10-09T05:01:09Z reported only 016.001-R and
106.012-T through 106.033-T as orphans: 23 pre-existing, out of scope.
No automatic repair was requested.

## PR #489 follow-up run and verification

Local correction commit:
`f255156e165e98e0cee1052b56f16fc6db2560e8`
(`fix(docs): correct PR 487 provenance, sequencing, and evidence`).
It contains exactly the 29 canonical task files, 198-S, the two CT planning
files, and the new evidence: 33 artifacts. The dirty operator-owned config
and every lock file were excluded. No active Git commit hook was installed.
The prior correction and continuity are already committed through starting
HEAD `f2900db9`. This run changes 29 canonical tasks, the two CT planning
documents, `stage-corrective-resume-completion-memory.md`, this note, and
the selected checkpoint: 33 Markdown files plus one JSON file.
Correction commit:
`cae15841ee7ae0660e14e45e09eb00c1c4fcca5c`
(`fix(docs): preserve full stash provenance and narrow CT claim semantics`).
It contains 32 Markdown files; checkpoint and this memory are committed
separately as continuity closure artifacts.
198-S has no prose claiming native claim enforcement, so its manifest is
unchanged; the edge itself is preserved.

ProposedAction: non-destructive provenance and planning-state repair only.
ActionRisk: moderate (indexed provenance and execution-order wording).
Approval: explicit operator authorization for PR #489 findings A-D.
Rollback: revert only this run's allowlisted local commits.
ActionResult: edits applied and data/graph proofs passed.

### Native verification evidence

* TOOL_OK: version/query/checkpoint reads; ALL_TOOLS_OK.
* Fresh config schema validation passed; SHA-256 remains
  `92091d6a5ea44eac34d8f0fd3a43c7cb2dd407213f65a65d9af784b9ac6ac371`.
* Full checkpoint enumeration: 46 valid summaries, no anomalies, exactly
  one active Stage checkpoint (the operator-selected file). Native get
  confirmed Stage ownership, valid/conforming true, no unknown fields.
  Engram was reachable but its indexed branch is main; cited current
  code was read directly rather than trusting a mismatched branch index.
  Recovery retained the bounded correction cursor and did not replay work.
* INDEX_SYNC_OK: native sync indexed 1,964 artifacts before and after
  provenance edits. No fallback was needed in this run.
* Native `backlogit_query_sql` read all 29 full stash records after sync.
  Each record was compared exactly with the archived JSONL: priority,
  kind, full text, and optional deliberation ID all match 29/29.
  Source path is `stash.jsonl`, state is harvested, and canonical links
  match 29/29. Priorities: seven high, 20 medium, two low. Kinds: 28 bug,
  one task. Full archived text preserved: 43,042 UTF-8 bytes.
* Parsed artifact comparison against `f2900db9` proved all 29 task bodies
  and all non-provenance frontmatter unchanged. No source deliberation ID
  was invented. The five non-harvest dispositions still have zero links.
* Native recursive SQL audit: 1,389 blocks edges, zero cyclic nodes,
  zero active shipments. CT still depends on CX; 152-S still depends on
  154-S, CX, and CT. No edge was added or removed in this follow-up.
* The dependency-ready set within the new 197-S through 201-S cohort is
  exactly `{197-S}`. This is not a whole-workspace readiness assertion:
  unfiltered queue semantics also return pre-existing 147-S, 176-S,
  177-S, 178-S, 179-S, 183-S, 186-S, and 187-S. None was changed or claimed.
  Queue readiness is not topology clearance or claim authorization.
* The SQL read-only gate rejected a leading `WITH`; the equivalent
  recursive CTE nested beneath `SELECT` succeeded. No index write was
  attempted through SQL.
* Doctor at `2026-10-09T05:23:44Z`: exactly 23 pre-existing orphans
  (016.001-R and 106.012-T through 106.033-T), no other findings.
  Automatic repair was not requested.
* Finding D: the earlier completion memory now records that PR #489 added
  `198-S blocks-on 197-S`, plus the separate exactly-shipped pre-claim duty.

### Continuity and external lint cursor

The operator confirmed prior markdownlint completed: 34 files, zero issues,
run by Orchestrator; prior HEAD CI was green. Stage ran no builds, tests,
or linters. These results do not claim validation of this run's new edits.
After all other fixes were completed and committed, the selected checkpoint
was updated with this completed validation evidence. Native
`backlogit_get_checkpoint` verified ownership and conformance before the
owner-Stage `backlogit_resolve_checkpoint` call succeeded at
`2026-10-09T05:27:43.9096049Z`. Native readback confirms status `resolved`,
valid/conforming true, no unknown fields, and the completed prior
markdownlint evidence intact. This resolved file, not an active snapshot,
is the continuity artifact being committed. No new recovery candidate was
created and no Ship-owned checkpoint was touched.

The Orchestrator must lint the 29 canonical task Markdown files in the map
above, plus these four files (33 total):

* `docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md`
* `docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md`
* `docs/memory/2026-10-08/stage-corrective-resume-completion-memory.md`
* `docs/memory/2026-10-08/stage-copilot-findings-memory.md`

No push, PR-body edit, new branch, application/config change, shipment
claim/closure, or lock operation occurred. The dirty config and all stale
lock files stay untouched and excluded from commits.
