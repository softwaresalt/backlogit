---
title: "Deliberation: 2026-10-08 Stage fold-ins and dispositions"
doc_type: "decision"
source: "docs/decisions/2026-10-08-stage-foldins-and-dispositions.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Stage decision for the operator-approved fold-ins of stash entries into existing shipments (152-S, 189-S, 183-S), the fit refusals for entries that do not belong to their named targets, the DE3E7B67 scope correction, the already-fixed parity-golden duplicates, and the deferral of the 156-S disposition to 185-S."
topic: "Fold-ins of stash entries into queued shipments and backlog hygiene dispositions"
depth: "standard"
decision_status: "decided (operator APPROVED Decision 1 and Decision 3c via the Orchestrator on 2026-10-08)"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-10-08-stage-foldins-plan.md"
  - "docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md"
  - "docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md"
---

# Deliberation: 2026-10-08 Stage fold-ins and dispositions

## Framing

The Orchestrator named fold-in targets for nine stash entries and asked Stage
to read each target before folding. An entry that does not fit stays in the
stash unchanged. No new shipment is created for a fold-in.

## Fit Evaluation

| Stash | Named target | Verdict | Reason |
|---|---|---|---|
| 97AB4978 | 152-S / 171-F | FITS | Its source refs name 171-F, 152-S, and PR #428. The legacy-import upgrade it hardens is 171.002-T's scope, which is not yet on `main`. |
| 7AA35A39 | 187-S / 189-S | DOES NOT FIT | It is a Ship/build-feature red-deliverable baseline workflow contract, not the readiness API (187-F) or the claim guard (189-F). |
| 9CA03F5D | 187-S / 189-S | DOES NOT FIT | Same surface as 7AA35A39. |
| E52607F5 | 187-S / 189-S | FITS 189-S | Same function (`ClaimShipment`) and the same claim crash-recovery path that 189-F hardens. |
| 95379DBA | 183-S | FITS | It is new CI evidence (run 37728825570) of the same numeric-adjacency predecessor defect that 183-F hands off upstream. |
| F88FE051 | 183-S | DOES NOT FIT | 183-F is documentation-only. The CI pin bump at `.github/workflows/ci.yml:278` needs an upstream release that does not exist yet. 183.004-T cites it as related. |
| 6EB55AE6 | 141-S..145-S | DOES NOT FIT | The FL002 analyzer lives in `internal/faultline/analyzer/errwrap`, owned by archived 158-F. No queued task in 159-F to 163-F touches analyzers (162.002-T is detector integration only). |
| E6EE8944 | 141-S..145-S | DOES NOT FIT | Same reason (FL003, `internal/faultline/analyzer/failopen`). |
| 8CCB29CF | DE3E7B67 | FITS (stash merge) | It corrects item (3) of the DE3E7B67 feature request. |

## Decisions

### F1: 97AB4978 into 171-F / 152-S

Add two tasks under 171-F, appended to 152-S: a RED test for present-but-null
`status`, `created_at`, and `updated_at` in the legacy-import upgrade path
(including its dry-run), then the presence-aware fix. Both follow 171.002-T,
which lands that path.

### F2: E52607F5 into 189-F / 189-S

Options:

* A: constrain the claim's bounded parent cascade so it never changes a
  manifest member whose preimage is not `queued`. Rejected: it changes
  claim status semantics (a parent with an active child would stay `done` or
  `review`) and is a wider contract change.
* B (chosen): keep the cascade. Claim crash recovery
  (`memberRecoveryCandidates`, `internal/core/shipment_recovery.go`) also
  accepts an unmarked `active` state for a member whose preimage is not
  `queued`, and rollback restores that member's exact preimage. The 173-F
  producer rules are unchanged: such a member is still neither marked nor
  unmarked.

Stage appended an amendment to
`docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`
recording option B. Two tasks under 189-F: a child-before-parent RED
regression test, then the recovery change.

### F3: 95379DBA into 183-F / 183-S

Add 183.004-T, a docs task that folds the CI evidence (PR #485, CI run
37728825570 job 113153060905, 153-S naming 152-S) into the upstream hand-off
document. 183.002-T (pre-delivery content review) now blocks on 183.004-T so
the review sees the new evidence.

### F4: 8CCB29CF into DE3E7B67

Merge the correction into the DE3E7B67 text with `stash edit`: item (3) adopt
rename propagation is limited to the shipment-manifest and
archived-stash-provenance gaps, because dependency, typed-link,
cross-artifact frontmatter, and ancillary indexed-reference rewrites already
exist. Then archive 8CCB29CF with the merge recorded here.

### F5: Parity-golden duplicates (18E587A0, 92F79833)

Commit `24a97b0d` ("fix(config): pin parity golden to LF") added
`internal/faultline/testdata/parity_v1.golden.json eol=lf` to
`.gitattributes`. `git ls-files --eol` reports `i/lf w/lf attr/text eol=lf`
and the commit is an ancestor of `main`. Both entries are fixed. 92F79833 is
a duplicate of 18E587A0 (surviving entry 18E587A0, the earlier capture).
Archive both with this evidence.

### F6: 156-S disposition deferred to 185-S

156-S is queued, empty, superseded, and carries a DO-NOT-CLAIM banner. No
governed queued-to-abandoned path exists until 185-S ("Governed
queued-shipment disposition") ships. Stage does not force a status change or
delete it. Stage appends a comment on 156-S and adds an informational link
from 185-S to 156-S.

## Entries Left Unchanged

7AA35A39, 9CA03F5D, F88FE051, 6EB55AE6, and E6EE8944 stay in the stash
unchanged for a future Stage session. 885263D2, DB48A817, and FF1F3AC7 look
fixed in code but stay active until the Orchestrator reports the full
`go test ./...` result.

## P-021 C5/C6 Triage Records

| Entry | Duplicate scan | Late-identifier reconciliation |
|---|---|---|
| 97AB4978 | clean scan | not triggered (PR #428 and thread PRRT_kwDORzozKM6f0sDb recorded) |
| E52607F5 | clean scan | not triggered (PR #466, thread PRRT_kwDORzozKM6ncEMm recorded) |
| 95379DBA | clean scan; DISCOVERY-STATUS AMBIGUOUS candidates F88FE051 (CI pin) and F05661B1 (closure-aware lifecycle gate) are different expansions | triggered (task N/A, review-thread N/A): the 153-S closure for PR #485 cites the entry; no review thread and no task-level identifier found. N/A stands as a truthful terminal record. |
| 8CCB29CF | merged into DE3E7B67 (not a duplicate; a scope correction). Surviving entry DE3E7B67; archived 8CCB29CF | not triggered (PR #467, threads PRRT_kwDORzozKM6nn9kQ and PRRT_kwDORzozKM6nn9m4 recorded) |
| 92F79833 | DUPLICATE of 18E587A0; both archived as fixed by 24a97b0d | not applicable |
| 6EB55AE6, E6EE8944 | clean scan | late identifier available: 140-S PR #436 (left unchanged in the stash; recorded here only) |
