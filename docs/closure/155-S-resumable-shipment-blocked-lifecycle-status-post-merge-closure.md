---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
conditions:
  - id: observation-window-outcome
    description: "The 24-hour manual observation window outcome is recorded."
    satisfied: true
    evidence: "DEGRADED-RECOVERED accepted by operator 2026-09-29; retry .backlogit/reconcile/155-S-post-20260926T203148Z.md exit 0; follow-ups 67F17B6B, D116AF58; no rollback"
  - id: closure-pr-merged
    description: "The post-merge closure PR is merged."
    satisfied: true
    evidence: "PR #451 merged at c57185c10bbd7c256bf49ecd2069fa0b357badd5"
description: "Post-merge operational closure for shipment 155-S and feature 174-F."
doc_type: closure
docline:
  date: 2026-09-26T20:31:10Z
  status: accepted
  tags:
    - operational-closure
    - post-merge
    - 155-S
    - 174-F
schema_version: "1.0"
source: docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md
title: "155-S post-merge operational closure"
---

## Summary

Shipment `155-S` (feature `174-F`, resumable governed `blocked` shipment
lifecycle status) closed through the governed flat shipment operation after
implementation PR #450 merged to `main`. The merge commit is
`2c8759c3f7583d678ef674b3c6566541b4945375`.

Only the 45 explicit manifest members were processed: `174-F` and 44 tasks.
The shipment control record is archived as shipped, every explicit member is
archived with valid provenance, no members were returned, and the unlisted
deliberations remain untouched. This report does not claim or bootstrap
`154-S`.

**Closure status:** `READY_WITH_CONDITIONS`. The technical close, validation,
and compaction gates are recorded here. The 24-hour manual observation window
completed with a `DEGRADED-RECOVERED` outcome, explicitly accepted by the
operator on 2026-09-29. Recovery succeeded through the authorized retry, and
the remaining follow-ups are tracked as `67F17B6B` and `D116AF58`; no rollback
trigger fired and no rollback was needed. The closure PR, #451, has merged.

## Governed shipment closure

The closure branch was created from synchronized `main` in the existing
worktree. The local binary was rebuilt from merged `main` before the governed
close.

* Pre-close reconciliation passed for 45 unique members, all already
  terminal; report:
  `.backlogit/reconcile/155-S-pre-20260926T194250Z.md`.
* The first ship invocation was terminated at five minutes during
  per-member evidence validation. Orchestrator lock and event evidence showed
  sequential progress and no artifact writes. It was not a reported lock
  conflict or a hung transaction. The five-minute timeout was too short for
  this validation path.
* The authorized second invocation ran for 33m50.868s, within the 45-minute
  gate/build timeout budget. It returned native exit code `0`,
  `shipment_status: shipped`, `returned_ids: []`, and the merge SHA.
* The result archived 42 records in this transaction: tasks
  `174.039-T`–`174.068-T` and `174.073-T`–`174.082-T`, feature `174-F`, and
  shipment `155-S`. Tasks `174.069-T`–`174.072-T` were already formally
  archived with `archived_status: done`.
* The post-close reconciliation found all 45 explicit members exactly once
  under `.backlogit/archive/`, with `status: archived`, valid
  `archived_from` provenance, and `archived_status: done`. All task
  `parent_id` values remain `174-F`; the feature remains the root. There are
  no missing members, status mismatches, duplicate queue/archive records, or
  returned members.
* The shipment control record is archived with `archived_status: shipped`,
  `commit: 2c8759c3f7583d678ef674b3c6566541b4945375`, and its original 45-item
  manifest.
* **Content anomaly:** the archived shipment body's historical bootstrap
  section still says not to claim or bootstrap `155-S`. The governed archive
  preserved that body verbatim; `status: archived` and
  `archived_status: shipped` are the terminal evidence. Ship did not hand-edit
  the backlog artifact. This is recorded for awareness; no archive edit was
  made, and the governed terminal state is authoritative.
* P-007 archive integrity passed: no tracked archive deletions were present.
  The only non-manifest backlog change visible in the queue/archive status is
  the pre-existing operator edit to `.backlogit/archive/stash.jsonl`; it was
  not staged.
* `.backlogit/ops` has no pending shipment-operation journal.

The machine-readable post-close report is
`.backlogit/reconcile/155-S-post-20260926T203148Z.md`. The second attempt's
stdout, stderr, native exit code, and timing are captured in
`logs/diagnostics/155-s-shipment-ship-attempt2.txt` and its adjacent metadata
file.

## PR, quality, and review gates

### Implementation PR #450

PR #450 merged to `main` at `2c8759c3f7583d678ef674b3c6566541b4945375`.
All reported CI checks passed for reviewed head
`bd6c00ee6a9fe01a1434b47149c0d6db896cfc65`:

| Check | Result |
|---|---|
| `test` | PASS |
| Windows handle/lock tests | PASS |
| Docline frontmatter gate | PASS |
| Markdown lint (P-008) | PASS |
| CLI Reference Drift | PASS |
| Detect code changes | PASS |
| `pipeline-topology (ambient)` | PASS |

P-018 was satisfied before merge without admin fallback. The Copilot review
completed for head `bd6c00ee6a9fe01a1434b47149c0d6db896cfc65`; GraphQL
verification found both Copilot-authored review threads resolved and no
additional thread pages. The normal merge-commit path was used.

### Final-gate evidence

* **Step 1 — governed full suite:** the single operator-authorized run at
  `0b11459f` (`go test -timeout=30m ./...`) exited 1 after 2029 seconds. All
  code packages passed; the sole failure was this review record missing
  `chunk_strategy: h1-h2-h3` and `schema_version: "1.0"`. That documentation
  defect was fixed in `24e75400507977dd5d0825252719f74e8773addb` and passed
  the targeted docline test and integration suite. The full suite was not
  rerun locally. PR #450's `test` CI check passed.
* **Step 2 — final review:** READY, with zero unresolved P0/P1 and no
  unresolved in-scope P2. The detailed record is
  `docs/closure/2026-09-25-155-S-final-review.md`.
* The Wave 20 standard review used eight personas. Out-of-scope findings were
  preserved under `AC6B669D`, `A4B62D86`, and `8AF55264`.
* Four adversarial slots found `ADV-W20-01` as a MEDIUM-confidence plurality
  finding; it was fixed in `d9a01e6f`. All four post-remediation reviewers
  returned READY with no residual findings.
* The lock-order test stabilization progressed through `85570a3b`,
  `f3deced0`, and `bd6c00ee`. The independent gpt-6-sol review found the final
  version READY with zero findings; the targeted lock-order run passed 10/10.
  `c7d25cfb` was markdown-only. Build, vet, pinned lint, and LF-normalized
  gofmt evidence for the final implementation head is recorded in the review
  artifact.
* Process deviations remain documented: `98367132` included a direct
  174.081-T production edit accepted by the Orchestrator; harness corrections
  `f7db30a3` and `b78ce24b` matched plan evidence; the adversarial-review
  agent refusal was replaced by direct four-slot dispatch.

## Runtime verification

The CLI runtime verification is `PASS WITH FOLLOW-UP`; see
`docs/closure/2026-09-26-155-S-runtime-verification.md`. The governed close
and the post-close read verified the shipped control record and all 45
archives.

## Knowledge maintenance and P-020

**Compound refresh assessment:** reviewed and retained as `keep` the existing
learnings on atomic multi-item claim/stale blocked clearing
(`docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`),
ancestor-aware shipment-gate staleness
(`docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md`), and
the shipped-status prevention envelope
(`docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md`). The
155-S implementation and verified close do not supersede or contradict them;
no compound file required an update.

`compact-context` was invoked with `target: all` on 2026-09-26, bounded to
tracked artifacts and the `155-S` release scope:

* The completed 155-S execution plan contained appended review history. It was
  consolidated into
  `docs/exec-plans/2026-09-26-resumable-shipment-blocked-lifecycle-decided-plan.md`;
  the full source and audit history were moved, not deleted, to
  `docs/archive/plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md`.
  The former execution-plan path remains as a compatibility pointer for
  existing backlog and checkpoint references.
* No memory or checkpoint file was modified, moved, or archived. The relevant
  memory set includes operator-dirty and untracked session records, so the
  latest-checkpoint preservation rule cannot be established without crossing
  the explicit preservation boundary.
* No closure record was compacted. The 155-S review and runtime records are
  recent, and existing topology-gate closure registrations were preserved.

**Compaction result:** one plan consolidated, zero memory files compacted,
zero closure files compacted, and all protected memory/checkpoint state
preserved. `compaction_status: degraded` records the intentionally deferred
memory compaction under the operator's tracked-only/preservation constraint;
the decided plan is complete and its verbose source remains traceable in the
archive.

## Releasability and operational monitoring

This is an on-demand CLI/MCP lifecycle capability; there is no continuously
running service, deployment, database migration, or feature flag. The
workspace profile declares no automated runtime validators. The monitoring
plan is therefore a manual observation checklist, not a claim of dashboard or
alert automation.

| Signal | Baseline and observation | Investigation trigger |
|---|---|---|
| Shipment lifecycle result | `155-S` closed with `returned_ids: []`; all 45 manifest members archived. Inspect shipment logs and `shipment-reconcile` evidence. | Any partial/indeterminate result, missing shipped event, returned ID, status mismatch, or non-member mutation. |
| Per-member close latency | Attempt 2 took 33m50.868s for 45 members; the first attempt's five-minute timeout interrupted read-only validation. | Any future governed close approaches or exceeds its approved 45-minute budget. |
| Archive integrity | All 45 archives have valid provenance; no tracked archive deletion was found. | Missing/duplicate archive entry, invalid `archived_from`, or an unexpected tracked deletion. |

* **Observation window:** 24 hours from implementation merge, through
  `2026-09-27T19:21:03Z`.
* **Observation-window outcome:** `DEGRADED-RECOVERED`. The window ran from
  `2026-09-26T19:21:03Z` through `2026-09-27T19:21:03Z`. Stash `67F17B6B`
  records that a harness-lock filename collision with backlogit's persistent
  lock sidecars blocked the 155-S post-merge closure at `2026-09-26T19:29:20Z`.
  Stash `D116AF58` records that shipment validation exceeded the five-minute
  command timeout and was killed at `2026-09-26T19:55:20Z`; the associated
  safe-close report is
  `.backlogit/reconcile/155-S-safe-close-20260926T194947Z.md`. Recovery was
  completed by the authorized retry recorded in
  `.backlogit/reconcile/155-S-post-20260926T203148Z.md`: exit `0`,
  `shipment_status: shipped`, and `returned_ids: []`. The shipped-event
  completeness check (`backlogit doctor --format json
  --check-shipped-event-completeness`) exited `0`; it reported only the
  pre-existing 23 orphans and no shipped-event advisory for `155-S` or
  `182-S`. There were no reverts or corrective production lifecycle commits
  after `2c8759c3f7583d678ef674b3c6566541b4945375`, and `182-S` later completed
  the governed lifecycle and archived cleanly. The operator explicitly
  accepted this window outcome as `DEGRADED-RECOVERED` on `2026-09-29`; this is
  an acceptance of a recovered degraded outcome, not a claim that the window
  was healthy. No rollback trigger fired and no rollback was needed. Follow-ups
  remain tracked as `67F17B6B` and `D116AF58`.
* **Owner:** backlogit maintainers and the release operator.
* **Post-merge checks:** during the window, review shipment-operation logs,
  run `backlogit doctor` or shipment reconciliation if any anomaly appears, and
  record healthy, degraded, or rolled-back outcome before closing the window.
* **Rollback trigger:** any mismatch between a governed result and the
  explicit manifest, an indeterminate terminal event, or a reproducible
  lifecycle regression.
* **Rollback procedure:** stop further affected lifecycle operations. For a
  code regression, revert merge commit
  `2c8759c3f7583d678ef674b3c6566541b4945375` through a reviewed corrective
  PR. Do not reverse shipment records by hand; any data correction must use
  the governed lifecycle or an explicitly approved recovery procedure.

### Risk and evidence record

* **ProposedAction:** close explicit shipment `155-S` with merge commit
  `2c8759c3f7583d678ef674b3c6566541b4945375`.
* **ActionRisk:** moderate; transaction affects the manifest and shipment
  control record only.
* **Approval:** operator-authorized normal close; no admin fallback.
* **ActionResult:** applied; native exit `0`, `returned_ids: []`, post-close
  reconciliation `CLOSED`.
* **Releasability:** `READY_WITH_CONDITIONS` — the completed manual observation
  window had a `DEGRADED-RECOVERED` outcome accepted by the operator on
  `2026-09-29`; the authorized retry succeeded, no rollback trigger fired or
  rollback was needed, and remaining follow-ups are tracked as `67F17B6B` and
  `D116AF58`.

## Deferred follow-ups

These existing stash entries remain Stage-owned and were not edited,
reprioritized, triaged, or harvested by Ship. The performance and lock-script
findings are included as follow-ups; no scope was expanded.

| ID | Captured follow-up |
|---|---|
| `FA6AE139` | Classify possibly partial terminal-event append failures and preserve indeterminate state. |
| `9900D0DD` | Complete Wave 20 operator documentation, CLI/MCP error mapping, and transaction-contract follow-ups. |
| `09D06A75` | Deliberate broader target-scoped recovery, snapshot provenance, and normalize API follow-ups. |
| `8AF55264` | Consolidate blocked-envelope guards and address separate artifact-type, hook, and write-path seams. |
| `7D8717B1` | Harden lifecycle error classification, compensation double faults, and partial-write reporting. |
| `388C586D` | Replace rollback JSONL rewrites with append-only compensation evidence. |
| `3340E97C` | Plan the separate Go toolchain and golangci-lint v2 upgrade. |
| `AC6B669D` | Decide whether `blocked_at` must preserve fractional-second precision. |
| `A4B62D86` | Deliberate cold-start auto-recovery ordering for blocked-shipment normalization. |
| `0FFBF819` | Use a verified no-follow handle-relative read for authoritative snapshots. |
| `45BD3B36` | Reject duplicate JSON object members in blocked-shipment snapshots. |
| `9CA03F5D` | Revisit red-deliverable baselines around task-claim bookkeeping and dirty trees. |
| `2F7FCA8B` | Add docline soft keys to the existing scratch pickup note. |
| `4A0B7BCF` | Preserve the explicit Ship tool allowlist across generated-agent tuning. |
| `7AA35A39` | Revisit zero-delta red-deliverable gating and task-claim lifecycle bookkeeping. |
| `67F17B6B` | Separate harness advisory lock names from backlogit's persistent OS-lock sidecars. |
| `D116AF58` | Profile per-member ship evidence validation and add useful progress reporting; this close took 33m50.868s. |

`E1`–`E5` remain deferred under
`.backlogit/checkpoints/checkpoint-20260925-005049.json`. Stash `C85386E6`
was already resolved and archived; it is not an open follow-up.

## Source artifact boundary

Feature `174-F` records `source_stash_id: 808E4323`. The stash archive records
that it was archived on `2026-09-14T21:15:00.6390139Z` during harvest to
`174-F`; it was already archived before the `155-S` close and the close did
not mutate it. It is provenance, not a shipment member. No
`source_deliberation_id` is recorded on the feature or explicit manifest.
Descriptions reference unlisted deliberations `067-DL`–`073-DL`; they remain
queued and were not archived or otherwise mutated.

## Post-merge main synchronization

The Orchestrator completed the local main synchronization after
`MERGE_SUCCEEDED`:

1. The unrelated porcelain state was recorded.
2. The Orchestrator ran `git fetch origin main:main`.
3. `git checkout main` completed without stashing, resetting, cleaning, or
   discarding local state.
4. `git pull --ff-only origin main` completed.
5. Local `HEAD` equaled `origin/main` at
   `2c8759c3f7583d678ef674b3c6566541b4945375`.
6. Before/after porcelain captures are identical:
   `logs/diagnostics/155-s-postmerge-porcelain-before.txt` and
   `logs/diagnostics/155-s-postmerge-porcelain-after.txt`.

The closure-PR review identified that the initial fetch destination was the
local `main` ref rather than the remote-tracking ref. From the closure branch,
the required explicit fetch `git fetch origin main:refs/remotes/origin/main`
was run and both `main` and `origin/main` were verified at
`2c8759c3f7583d678ef674b3c6566541b4945375`. The before/after porcelain
captures `logs/diagnostics/155-s-closure-review-sync-before-e1f3d721.txt`
and `logs/diagnostics/155-s-closure-review-sync-after-e1f3d721.txt` compare
identically. This review-time verification did not switch branches or alter
operator worktree state.

The closure PR remains pending operator merge approval.
