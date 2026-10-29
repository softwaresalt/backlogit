---
chunk_strategy: h1-h2-h3
description: "Stage dark-mode (P-017, scope 199-S) decision on 199-S post-merge closure: ratifies Option A (READY_WITH_CONDITIONS with documented Stage exception for the un-adopted 192-F close-evidence record), records PREVENTION-2 disposition, captures P-021 follow-up 73B7C429, and repairs docline soft-key/frontmatter compliance on the 199-S closure docs."
doc_type: memory
docline:
  date: 2026-10-15T19:28:00Z
  status: final
  tags:
    - stage
    - 199-S
    - post-merge-closure
    - dark-mode
schema_version: "1.0"
source: docs/closure/199-S-199-F-post-merge-closure.md
title: "Stage 199-S post-merge closure exception decision (Option A)"
---

# Stage 199-S post-merge closure exception decision (2026-10-15, UTC)

Context: P-017 dark mode, scope `199-S` only, branch `post-merge/199-f-cy-a-lifecycle-hardening`. Carry-forward operator files `.autoharness/config.yaml` and `.backlogit/stash.jsonl` and untracked `.backlogit/checkpoints/*.json` left unstaged and uncommitted.

## 1. Protocol and gate analysis

* **Root cause of the 192-F `close-evidence` mismatch**: The editable Python `autoharness` CLI (`C:\Source\GitHub\autoharness\src\autoharness\cli.py:_evaluate_closure_evidence`) runs five checks in order — `workspace_containment`, `input`, `filename`, `frontmatter_predicate` (`topology._closure_artifact_complete`), and `discoverability` — and then runs `_check_close_path_requirement`, which checks `close_path: cascade | safe_close` and validates `docs/closure/evidence/{S}-{F}-close-evidence.json`. However, the live installed `backlogit` harness (`.github/agents/_ship.agent.md`, `.github/skills/shipment-reconcile/SKILL.md`, `.github/skills/operational-closure/SKILL.md`) intentionally preserves native flat-manifest `backlogit_ship_shipment` (`.autoharness/harness-manifest.yaml` `drift_allowed: true` from `155-S` / `197-S`) and has not adopted the `.autoharness/staging/` 192-F `autoharness shipment cascade-close --classify-only` workflow (`docs/closure/evidence/` is empty across the repository, and `197-S` and `198-S` closed under the identical posture).
* **Post-hoc constraint (no fabrication)**: Once `backlogit_ship_shipment` archived `199-S` (`status: archived`, `archived_status: shipped`), `autoharness shipment cascade-close --classify-only` refuses post-hoc because `199-S` is no longer open in `.backlogit/queue/`. Neither `close_path` nor `docs/closure/evidence/199-S-199-F-close-evidence.json` may be fabricated, guessed, or backdated.
* **Consumer acceptance predicate (`topology._closure_artifact_complete`)**: The authoritative consumer predicate used by `autoharness gate pipeline-topology` (including successor `pre_claim` and CI `ambient`) and by `autoharness gate closure-evidence` check 4 (`frontmatter_predicate`) requires `compaction_status` in `{done, degraded}` and `closure_status: READY` or `closure_status: READY_WITH_CONDITIONS` with a non-empty `conditions:` list in which every entry is a mapping with `satisfied: true` and non-empty `evidence`. As documented in the staged `operational-closure` skill Step 3a, `pipeline-topology` never reads `close_path`, `close_evidence`, or `docs/closure/evidence/*.json`.

## 2. Decision (Option A ratified)

* **Closure status**: Amend `docs/closure/199-S-199-F-post-merge-closure.md` from `closure_status: BLOCKED` to `closure_status: READY_WITH_CONDITIONS` (`compaction_status: degraded` kept honest and non-blocking per P-020).
* **Condition `close-evidence-record`**: Set `satisfied: true` (`actor: stage`, `attested_at: 2026-10-15T19:28:00Z`, `owner: Stage`) under the documented Stage exception, explicitly stating what is missing (`docs/closure/evidence/199-S-199-F-close-evidence.json` and `close_path`/`close_evidence`), why it cannot be produced post-hoc without prohibited fabrication, and the verified compensating evidence that exists:
  1. Merge commit `1f25d10daed74ce209a1f14ec7160041bb0a4b6e` on `origin/main` (PR #498, merge commit strategy P-009).
  2. Reviewed HEAD `e7961c8d7d480aef54ce6f62dcd3b045ad11f296` (local review `READY_WITH_FOLLOWUPS` P0=0 P1=0, adversarial 3-reviewer consensus `READY` P0=0 P1=0, `copilot-review` gate `SATISFIED`, 7 of 7 required CI checks green, unfiltered `go test -count=1 -timeout=30m ./...` exit 0 across 40 packages).
  3. Complete GI/GR reconcile reports `.backlogit/reconcile/199-S-pre-20261015T190500Z.md` (`PROCEED`), `.backlogit/reconcile/199-S-safe-close-20261015T192000Z.md` (`CLOSED`), and `.backlogit/reconcile/199-S-post-20261015T192500Z.md` (`PROCEED`), committed in `a690ad18` prior to the `23597705` provenance SHA repair (retaining the initial 39-character string as historical snapshots while all 11 archive records carry the corrected 40-character SHA).
  4. Verified archive records `.backlogit/archive/199-S.md` (`archived_status: shipped`), `.backlogit/archive/199-F.md`, and `.backlogit/archive/199.001-T.md` through `199.009-T.md` all carrying `commit: 1f25d10daed74ce209a1f14ec7160041bb0a4b6e`, item-log `shipped` event present, and `backlogit doctor --check-shipped-event-completeness` reporting 0 findings for `199-S`.
* **PREVENTION-2 (`23D47703`) decision**: Do not create a blocking condition backlog item or `blocks` edge on successor shipments (`200-S`, `201-S`, `189-S`). The 192-F pre-close JSON gap for `199-S` is a closed, one-time historical process exception on an already-archived shipment (impossible to produce later without prohibited fabrication), not an open runtime precondition on `200-S` or `201-S`. Wiring a `blocks` edge to `200-S` would mutate out-of-scope shipments and stall the successor chain behind an unsatisfiable condition. With `close-evidence-record` satisfied by the governed exception, the DAG queue view and `pipeline-topology` (`closure_complete('199-S') == True`) agree.

## 3. Follow-up capture and docline repairs

* **P-021 C2 capture**: Created stash `73B7C429` (`DEFERRED SCOPE EXPANSION`, `DISCOVERY-STATUS: AMBIGUOUS; candidate overlapping entries: 3FF72EB8`) to reconcile the live `.github/agents/_ship.agent.md` Step 6, `.github/skills/shipment-reconcile/SKILL.md`, and `.github/skills/operational-closure/SKILL.md` closure protocol with the 192-F `autoharness shipment cascade-close --classify-only` pre-close step and `autoharness gate closure-evidence`.
* **Docline frontmatter and soft-key repairs**: Added `chunk_strategy: h1-h2-h3` to `docs/closure/199-S-199-F-post-merge-closure.md` and `docs/memory/2026-10-15/ship-199-s-closure-memory.md`, and added `chunk_strategy: h1-h2-h3`, `description`, `shipment_id: 199-S`, and `source: docs/closure/199-S-199-F-post-merge-closure.md` to `docs/closure/2026-10-15-199-s-compound-refresh.md` so `backlogit_docs_lint` (`make docs-lint`) and `TestDoclineSoftKeys_LiveTrackedCorpus` pass in CI.
* **Adversarial persona review**: Constitution Reviewer (`READY_WITH_FOLLOWUPS`, 0 P0, 0 P1, 1 P2 on the reconcile-report 39-char SHA note fixed in place, 1 P3 on `ship-199-s-closure-memory.md` next-steps wording fixed in place), Scope Boundary Auditor (`PASS / READY`, 0 P0, 0 P1, 0 P2, 0 P3), and direct Correctness verification (`READY`, 0 P0, 0 P1, 0 P2, 0 P3).
