---
schema_version: "1.0"
doc_type: memory
title: Stage memory for the 2A355F83 harvest attempt (HALT on RB12 chore-root collision)
description: Operator authorization receipt for the attempt-4 ADVISORY, owner checkpoint recovery, the RB12 chore-root child-ID collision that halted harvest, the retired partial root, the prepared finding dispositions, and the decision the Orchestrator needs before harvest can resume.
chunk_strategy: h1-h2-h3
---

# Stage memory: 2A355F83 harvest attempt 1 (HALT, RB12)

## Status

**Harvest halted on RB12.** The attempt-4 ADVISORY is now operator-authorized. The plan records
`operator_authorization: approved`, and the reviewed body SHA-256 prefix `ADBF9F245D84` is
unchanged.

The chore allocator assigned `001-C`, because chores are numbered separately from other types.
Its first task child, `001.001-T`, already exists in the archive under `001-F`, so a task create
failed at the pre-write uniqueness chokepoint and nothing was written. The plan's RB12 says that
any collision halts harvest.

Current state:

* No task, dependency edge, link, or shipment exists.
* No Harvest Record or `BOOTSTRAP_EXCEPTION_RECORDED` comment exists.
* No bootstrap shipment `B` exists.
* The empty partial root `001-C` is archived from `queued` and carries a `HARVEST_HALT_RB12`
  comment. The archive is reversible.

## Operator authorization receipt

Recorded verbatim in the plan, under `### Operator authorization (attempt 4 ADVISORY)`.

* Exact preceding request (Orchestrator to operator): "The remaining approval is specific:
  approve the ADVISORY result and resume checkpoint-20261002-031116.json for harvest. Stage can
  then create the seven-task bootstrap shipment, with the findings carried into implementation
  rather than silently ignored."
* Operator reply at `2026-10-02T03:20:52.330Z`: the full continuation directive. Stage received
  these relayed verbatim excerpts: "If you were planning, stop planning and start implementing"
  and "Keep working autonomously until the task is truly finished, then call task_complete."
* The Orchestrator publicly interprets the reply as contextual approval of the ADVISORY and as
  direction to harvest. The literal word "approved" does not appear in it.
* The approval is not a blanket approval, a gate waiver, dark mode, a merge or admin approval, an
  E3 grant, a claim authorization, or a Condition B attestation. No fifth review ran.

## Recovery

* Stage enumerated all 30 `stage` checkpoint summaries without a status filter. The result was
  `needs_quarantine` 0 and `quarantined` 0, with no empty `agent` or `status` field. The only
  active `stage` checkpoint was `checkpoint-20261002-031116.json`, which the operator named.
* Owner validation: `agent: stage`, `valid`, `conforming`. Engram was reachable: one bounded CLI
  `query-memory --limit 3` succeeded and returned no results.
* Restore, then prune, then resume. The cursor, the checkpoint pointer, and all four review
  verdicts were kept:
  * attempt 1: FAIL with 9 P1;
  * attempt 2: FAIL with 6 P1;
  * attempt 3: FAIL with 4 P1;
  * attempt 4: ADVISORY with 0 P0, 0 P1, and 10 deduplicated P2.
* No other checkpoint was touched.

## What happened

1. Step 0.1 index sync succeeded (1858 indexed).
2. Stage refreshed the metadata catalog, the type list, and the chore and task WIT metadata. A
   chore allows `task` children, and its ID format is `{NNN}{suffix}` with prefix `C`.
3. RB12 pre-check error: Stage checked `195` (the next global number) for collisions, not the
   chore allocator's actual candidate `001`. This was a check-before-create ordering defect.
4. `backlogit_create_item` (chore) returned `001-C`.
5. A task create with parent `001-C` failed: `create artifact "001.001-T": artifact ID already
   exists on the canonical filesystem`. The `.backlogit/archive/` directory holds `001.001-T` to
   `001.011-T` and their subtasks under the done feature `001-F`.
6. Stage added a comment on `001-C` and archived it (`archived_status: queued`). An aggregate hash
   over the 84 protected files (`154-S` plus all `182`–`194` items) matched before and after:
   prefix `448CCB0946F87E31`.

## Decision needed (Orchestrator to operator)

RB12 records the chore root as a reviewed choice and says that a collision halts harvest. Any
re-rooting is therefore an operator decision. Stage cannot treat it as a silent plan edit.

* **Option A (Stage recommends this):** authorize a `feature` root for the same seven tasks.
  * Feature numbering is free at `195`. Stage checked queue, archive, and logs for `195` and
    `196`, and found no files.
  * The children would be `195.001-T` and later IDs.
  * This is the documented fix in
    `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`.
  * The task bodies, DAG, waves, shipment, and exception binding do not change. Only the root
    type and the chore-root rationale in RB12 change.
  * Stage records this as a harvest-time deviation, with the operator's words verbatim.
* **Option B:** keep the chore root and fix the allocator first. The allocator would have to skip
  hierarchical IDs that already exist on the filesystem. That is a Go change outside this plan,
  so it needs separate staging.
* **Option C:** rescope or abandon.

When harvest resumes, Stage must check the actual allocator candidate for the root and its
children in queue, archive, and logs before creating anything.

## Prepared harvest shape (unchanged plan)

| Order | Unit | Classification | Depends on | Wave |
|---|---|---|---|---|
| 1 | UCS1 contract harness (RED) | red deliverable | none | 1 |
| 2 | UCS3 simulation harness (RED) | red deliverable | none | 1 |
| 3 | UCS2a Ship text | covered-by UCS1 | UCS1 | 2 |
| 4 | UCS2b policy text | covered-by UCS1 | UCS1 | 2 |
| 5 | UCS4 simulation model | covered-by UCS3 | UCS3 | 2 |
| 6 | UCS5a positive proof | verification-only | UCS2a, UCS2b, UCS4 | 3 |
| 7 | UCS5b fail-closed proof | verification-only | UCS2a, UCS2b, UCS4 | 3 |

* The shipment is queued and lists its members explicitly and flat: the root first, then the
  seven tasks in the order above.
* It has one `blocks` edge, `B` onto `154-S`, for P. It does not block on its own future C.
* Add a `related_to` link from the root to `182.001-T`. That task is the producer-side option A
  lifecycle net, not duplicate work.
* Bind the Harvest Record (delimiters at column 0) and the `BOOTSTRAP_EXCEPTION_RECORDED` comment
  to the actual returned `B`.

## Prepared finding dispositions (apply at resumed harvest; none is fixed yet)

The in-scope items below are contract completions on the named task's own surface (P-021 C1). The
task must address each one before it closes. Assigning an item to a task does not make it
resolved.

| Finding | Disposition | Owner |
|---|---|---|
| P2-1 UCS1 trailing-space literal | Must be fixed in UCS1. Assert the trimmed token ``no member of `ready_k` is `active` ``, never the CommonMark-padded literal, at every backtick-terminated literal. A copied trailing-space literal that cannot pass is a defect, not success. | UCS1 |
| P2-2 leftover `ready_k` prose | UCS2b rewrites the whole P-002.6 `Wave k` bullet, including the "queued members whose every dependency" prose. UCS1 `Policy` asserts that the old prose is absent. Both are same-contract. | UCS1, UCS2b |
| P2-3 dispatch shell forms | The dispatch block's `git show $m:<path>` and unquoted `origin/main^{commit}` fail in PowerShell. At dispatch, the Orchestrator must run the VMR-body forms `"${m}:<path>"` and `'origin/main^{commit}'`. It must not run the dispatch-block text literally. | Orchestrator |
| P2-4 grant PR breadth | Residual risk. When the Orchestrator reads an E3 grant, it should require that the pull request that introduced the grant changes only the grant path and is not the staging pull request. The operator decides this together with E3. | Orchestrator, operator |
| P2-5 VMR step 5 operability | Before dispatch, the Orchestrator dry-runs VMR on the merged plan path. That run also confirms that merge commits pass (P3). | Orchestrator |
| P2-6 permanent P-012 declaration | Same surface as UCS2a. UCS2a adds one sentence requiring a session-record P-012 declaration for member-log reads until a configured tool returns the event stream. | UCS2a |
| P2-7 Principle VII backup | Same surface as UCS5b. Back up the fixture file inside the ignored fixture before the in-place edit, write UTF-8 without a BOM, and record the backup in the evidence. | UCS5b |
| P2-8 RB11 follow-up captor | At the resumed harvest, Stage captures two stash entries after a duplicate scan (none found today): the CI `ci.yml` paths-filter gap, and a history-read tool. The new stash lines are staged on their own. | Stage |
| P2-9 carried-item dispositions | This table and the carried list below record each disposition. Accepted-risk items need the operator's disposition before `B` closes. | Orchestrator, operator |
| P2-10 UCS2a size | No change. Watch the duration telemetry. | UCS2a |

P3 items, from the plan's deduplicated summary of 25 raw findings:

* **UCS1:** clarify the `Preserved` counts for rows that do not exist yet, add a token check on
  Step 4.0 item 7, record NotContains non-vacuity in the red evidence, and resolve the "never
  treats an `active` member as satisfied" assertion against the surviving true sentence (this
  last item is shared with UCS2b).
* **UCS2a:** name the full token set `K`; clarify "claim" versus Step 4.1b and "claim or start"
  in item 6; state that the H1 crash note applies to claim-assigned members only; state that
  non-shipment-mode classification is skipped; and add the `WAVE_NO_PROGRESS` convergence-scope
  note with a separate claim-assigned census count (shared with UCS2b).
* **UCS3 and UCS4:** define `active_ids` as residuals only.
* **UCS5a and UCS5b:** note that the fixtures are created only for the proof, and quote `#` in
  the evidence frontmatter.
* **UCS5b:** write UTF-8 without a BOM.
* **Orchestrator:** name the grant-file writer, the merger, and how `<yyyy-MM-dd>` is resolved;
  pin `gh --hostname github.com`; check the URL before fetching; and confirm that merge commits
  pass VMR step 5.
* **Stage at harvest:** anchor the Harvest Record delimiters at column 0. The body already
  mentions the delimiter token inline twice, so checkers must anchor at the start of the line.
* **Accepted with no edit:**
  * qualifying the circuit-breaker row is outside the reviewed edit scope;
  * the chore-root choice is now superseded by this decision;
  * the `B=<B>` grant binding and the extra dispatch field are harmless.

The attempt-3 carried P2/P3 items were "not worse and not re-raised" in attempt 4, and the
attempt-4 revision folded in most of them. The remaining items map to tasks as follows:

* backup and BOM to UCS5b;
* `len(scenarios)` and `ctx.Err` to UCS3;
* `QuoteMeta`, the P-002.2 slice scope, and pinning only literals to UCS1;
* the served-root rule to UCS2a.

Items with no task surface need the operator's H14 accept-risk disposition before `B` closes:

* lenient revocation;
* the grant tied to the Harvest Record SHA;
* the R3 tool map;
* the quality-gate order;
* the closure-pending checkpoint.

## Unchanged gates

* **E1 unmet.** The served MCP is `2c8759c3`, which lacks `6d233d21`. A CLI version is never MCP
  proof.
* **E2 unmet.** The artifacts are not on canonical `main` (`046c0130`).
* **E3 not granted.**
* **P is satisfied.** `154-S` is archived with `archived_status: shipped`, merge `6d233d21`.
* **C is waived for `B` only.** No `B` exists yet. The narrow exception ('Yes, I authorize that
  narrowly scoped bootstrap exception', `2026-10-02T00:29:30.253Z`) covers the one actual `B`
  only. No `CONDITION_B_ATTESTED` was written.

## Protected state

These were not committed, stashed, or reverted:

* `.autoharness/config.yaml`;
* five `checkpoint-20260930-*.json` files;
* `.backlogit/memories.json` (this session's official memory key was added locally);
* `.backlogit/stash.jsonl` (the `84E54F92` line);
* `docs/memory/2026-09-30-orchestrator-154s-closure-session.md`.

The routing record `ROUTING_DEGRADED` stays as declared. Engram counter histories are retained,
and no daemon start or replay was run.
