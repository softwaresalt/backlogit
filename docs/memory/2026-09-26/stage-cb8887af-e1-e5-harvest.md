# Stage session: CB8887AF E1–E5 governed test-budget ensemble harvest

* **Date:** 2026-09-26 (UTC 2026-09-27)
* **Agent:** Stage (claude-opus-5.5 / anthropic / high, routed by the Orchestrator)
* **Session:** `stage-cb8887af-harvest-20260927`
* **Branch:** `stage/cb8887af-test-budget-ensembles`, cut from `main` at `c3f5e2ce`
  (the PR #449 merge commit)
* **Status:** harvest complete. Ship owns `177-S`…`181-S`. Stage made no claim and did no
  build.
* **Resumed from:** `.backlogit/checkpoints/checkpoint-20260925-005049.json`. It was
  owned by Stage, the Orchestrator verified it, and the operator selected and confirmed it
  ("Run the suggested order; commit the 3 edited files in the Stage branch created from
  harvesting CB8887AF").

## Preconditions re-verified

* PR #449 merged, and `main` equals `origin/main` at `c3f5e2ce`.
* After sync, the highest existing IDs were `176-S` (shipment) and `175-F` (feature),
  so the 19R.11 allocator precondition held.
* `155-S` has shipped and is archived. `154-S` is queued at position 100 and depends only on
  `155-S`.
* Stage checkpoints: exactly one was active (the resumed one). No quarantine or validation
  anomalies.
* Plan-review gate: the archived plan is at
  `docs/archive/plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md`, and the
  compatibility pointer is at `docs/exec-plans/…`. Its final `## Plan Review` section is Wave
  19R attempt 3, with `multi-agent-dispatch`, `ADVISORY`, and
  `operator_authorization: approved`. The gate is satisfied, and the plan has no E1–E5
  amendment after 19R.12.
* Docline gate: the installed `backlogit docs lint --path` binary (D5 precedent) found 0
  violations on both the archived plan and the pointer.

## Resume-time occurrence inventory (19R.4 / 19R.11)

I searched the tree with `git grep` for any `go test … ./...` without `-timeout=30m`,
excluding `docs/`, `.backlogit/`, `plugin/`, `internal/`, and `cmd/`. A separate pass over
`internal/` and `cmd/` found no hits.

* Every 19R.4 surface is still present at the planned lines.
* **Hits new since 19R.4, all dispositioned:**
  * `go.instructions.md:36`: the coverage variant. It is in E4 task `180.004-T`, a file E4
    already planned to edit.
  * `.autoharness/drift-ignore:332–368`: explanatory comments. Descriptive, so unchanged.
  * `harness-manifest.yaml:461`: the install note on the pre-push entry. Descriptive, so
    unchanged.
  * `tests/simulation/wave-scheduler-contract.json:968–969`: test fixture data. Unchanged.
  * `.copilot-tracking/archive/**`: historical archive. Unchanged.
* **Directories named in the resume hint:**
  * `.claude-plugin/marketplace.json`: 0 hits.
  * `.github/plugin/plugin.json`: 0 hits.
  * `.copilot/`: not tracked (local command history), so it is not a surface.
* **drift_allowed re-check (19R.10 rule):**
  * These consumers are managed and not drift-allowed: `github-pr-automation`, `go-engineer`,
    and `harness-architect` (E3), plus `constitution` and `go.instructions` (E4). The E3→E5
    and E4→E5 edges are therefore kept.
  * `AGENTS.md`, `copilot-instructions.md`, and both pre-push scripts are
    `drift_allowed: true`.
  * `build-feature`, `fix-ci`, `README.md`, and `copilot-review-instructions.md` are not
    managed entries.
* No open backlog item duplicates E1–E5 scope.

## Harvest (19R.11 manifest order)

| Ensemble | Shipment | Feature | Tasks | Intra-feature edges (blocks) |
|---|---|---|---|---|
| E1 CI/release | `177-S` | `176-F` | `176.001-T`…`176.003-T` (3) | 002→001; 003→001,002 |
| E2 local gates | `178-S` | `177-F` | `177.001-T`…`177.004-T` (4) | 002→001; 003→001; 004→002 |
| E5 render inputs | `179-S` | `178-F` | `178.001-T`…`178.004-T` (4) | 002→001; 003→001; 004→002,003 |
| E3 procedures (closes D6) | `180-S` | `179-F` | `179.001-T`…`179.005-T` (5) | 002→001; 003→001; 004→001,002; 005→003,004 |
| E4 guidance (closes D4) | `181-S` | `180-F` | `180.001-T`…`180.006-T` (6) | 002..005→001; 006→002,003,004,005 |

* Totals: 5 features, 22 tasks, and 5 shipments. Each manifest lists the covering feature
  first, then its tasks in dependency order, and each was verified by reading it back
  (`covering_feature` present, item counts 4/5/5/6/7).
* Shipment edges (`blocks`):
  * `177-S→155-S`, `178-S→155-S`, and `179-S→155-S`. `155-S` has shipped, so these three are
    provenance edges.
  * `180-S→179-S` and `181-S→179-S`.
* No edge touches `154-S`, and no new shipment has a `queue_position`.
* The ready queue shows `177-S`, `178-S`, and `179-S`. `180-S` and `181-S` wait for `179-S`.

## Harvest decisions and deviations (none change plan scope)

1. **Provenance.**
   * I did not use `backlogit_harvest_stash`. It maps one stash entry to one item and
     consumes the entry, but CB8887AF feeds five features.
   * Instead, every feature and task cites CB8887AF, 073-DL O12, and plan 19R.10/19R.11 in
     its body, and each feature carries the plan references.
   * CB8887AF was annotated in place with a DISPOSITION line (original text preserved), then
     archived without deletion (`stash archive`).
2. **Harness classification.**
   * Plan 19R.10 declares no closed exempt set for E1–E5, so no task carries
     `harness-exempt`, and Ship intake classifies each task fail-closed (P-002.1/P-002.4).
   * Each manifest-checksum task notes the rev22.4 D8 lesson: YAML is not admissible under
     `docs-only`.
   * Each such task also cites the 174.076-T precedent: its own `TestU<unit>_` harness per
     P-002.6 requirement 4.
   * Unit tokens are left to Ship intake and harness-architect, so no contract block was
     invented.
   * Green-maker tasks that edit docs may also be classified at intake.
3. **Serialization edges** (ordering only, no scope change):
   * Tasks that edit the same file are serialized: `176.003-T→176.002-T` (`ci.yml`),
     `179.004-T→179.002-T` (`build-feature/SKILL.md`), and `178.004-T→178.002-T` (the
     manifest).
   * Full ensemble-green checks sit only on tasks whose dependency closure covers every
     green maker: `178.004-T`, `179.005-T`, and `180.006-T`.
4. **Constitution bump (reclassified after PR #452 review).** `180.002-T` now requires a
   MINOR bump, 1.0.0→1.1.0, under the constitution's Amendments rule ("material expansions
   require MINOR"). The harvest first specified PATCH 1.0.0→1.0.1; that decision is
   superseded and must not be implemented. Bare `go test ./...` enforces Go's default
   10-minute per-package timeout, so an explicit `-timeout=30m` budget materially loosens
   the gate. The rationale must name that enforcement change. Only a MAJOR judgment sends
   the task back to Stage (commit 80f06b1e).
5. **E1 task 3** keeps the plan's bundling of the `ci.yml` step with the `ci_compliance`
   wiring assertion. If Ship's width or class pass rejects the mixed delta, the task goes back
   to Stage.
6. **Priorities.** Task priority stays at the default (medium), and shipment priority is
   unset. This avoids reordering against other unpositioned queued shipments. The plan
   specified no priority.
7. **Tool incident.** Two parallel `create_item` calls raced on the allocator. The second
   call hit a self-edge (`179.004-T→179.004-T`) and was compensated (`not-applied`). I
   verified the surviving `179.004-T` and the index edges, then retried `179.005-T` alone.
   After that, every creation ran sequentially. Doctor reports no duplicate IDs.
8. **The archived plan was not modified**, per the harvest skill guardrail and because the
   plan is archived. This memory file and the CB8887AF disposition are the harvest record.

## Validation

* `backlogit sync` indexed 1766 artifacts.
* `backlogit doctor` (read-only, no `fix_orphans`) found 23 orphaned-artifact findings, all
  pre-existing (`016.001-R` and `106.012-T`…`106.033-T`). None involves a new item, and there
  are no duplicate IDs.

## Stash

* `CB8887AF`: annotated, then archived (consumed).
* These carry over unchanged: `95DF7CE9` (its local render-input part now lives in E5),
  `5F1A1873`, `5A1C4D3F` (which owns `make.ps1`), `D8EF5443`, and `7CE12EE6`.

## Next steps

1. **Orchestrator:** commit its own model-routing and memory files separately, then run the
   Step 1.5 staging PR for `stage/cb8887af-test-budget-ensembles`.
2. **Ship:** after the staging PR merges, `177-S`, `178-S`, and `179-S` are eligible in any
   order. `180-S` and `181-S` become eligible after `179-S` ships.
