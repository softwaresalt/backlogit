---
chunk_strategy: h1-h2-h3
description: Stage session memory for harvesting stash E75605E5 into feature 102-F and queued shipment 091-S (spike findings-artifact docline reconciliation).
doc_type: memory
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: memory
schema_version: "1.0"
source: docs/memory/2026-07-13/spike-findings-docline-stage-memory.md
title: Spike findings-artifact docline reconciliation — Stage session memory
---

## Outcome

`stage next` triaged stash `E75605E5` ("reconcile plugin spike skill
findings-artifact example with docline frontmatter contract without broad plugin
content-sync") through the full Stage pipeline and produced **queued shipment
`091-S`**.

* Feature `102-F` — Reconcile spike skill findings-artifact example with docline
  frontmatter contract (covering feature).
* Task `102.001-T` — Reconcile spike findings-artifact frontmatter example to
  docline base schema (harvested from `E75605E5`).
* Shipment `091-S` (status `queued`) — covers `102-F` + `102.001-T`.

Stash `E75605E5` final state: **harvested** (archived with
`harvested_artifact_id: 102.001-T`).

## Workspace-resolution gotcha (important)

The `backlogit` **MCP tools resolve the installed-plugin workspace**
(`.copilot/installed-plugins/softwaresalt/backlogit/.backlogit/`), NOT the repo
root `.backlogit/`. `fetch_stash`/`stash_get` therefore could not see
`E75605E5` (which lives only in the repo workspace) and returned unrelated
plugin-workspace entries. **All backlog mutations were done via the repo CLI**
(`.\backlogit.exe ... --cwd .` default) from `C:\Source\GitHub\backlogit`, which
correctly resolves the repo `.backlogit/`. Use the CLI (not MCP) for repo
backlog work until the MCP workspace root is corrected.

## The reconciliation (scope for Ship)

Artifact(s) to touch — surgical, two identical example-block edits:

* `plugin/skills/spike/SKILL.md` — Phase 5 "Write Findings Artifact" YAML example
  (the stash's explicit target).
* `.github/skills/spike/SKILL.md` — the in-repo twin with the identical block.

Current example diverges from docline base frontmatter v1: uses `type: spike`
(not `doc_type`), missing `source`, and 8 non-contract keys at top level. The
reconciled block uses top-level `title` / `source` /
`doc_type: decision` / `description` with `type` / `date` / `time_box` /
`conclusion` / `confidence` / `linked_parent_work_item` / `promoted_to` / `tags`
nested under `docline:` — matching existing conformant artifacts
`docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md` and
`docs/decisions/2026-07-09-github-actions-cost-spike.md`. Output path stays
`docs/decisions/*-spike.md` (→ `doc_type: decision`, verified via
`backlogit docs classify`).

Full spec: `docs/exec-plans/2026-07-13-spike-findings-docline-reconciliation-plan.md`.

**Plan-review provenance correction (post-merge; flagged by PR #230 Copilot
review).** The earlier "plan-review (gate PASS)" phrasing here overstated the
evidence and is corrected: the plan's "Plan Review" section is an **inline
single-agent Stage self-assessment** recorded for traceability — it is **NOT**
the output of the formal multi-persona `plan-review` skill, and **no formal
plan-review gate evidence exists**. Proceeding to build was justified by the LOW
blast radius (two identical instructional skill-doc example blocks; no schema,
CLI-distribution, or multi-template-family surface), not by a satisfied formal
gate. Ship's own `review` gate (report-only, Template Integrity + Constitution
lenses) ran normally before PR #231 and returned zero P0/P1/P2 findings.

## Decisions

* **No deliberation artifact** — this is a small, well-scoped, single-domain
  (~30 min) chore; a lean impl-plan was sufficient per the deliberate/impl-plan
  guidance.
* **No plan-harden** — blast radius is LOW (two instructional skill docs; no
  schema/CLI/multi-template-family surface).
* **`.github` twin included** with the plugin copy: same block, same concern,
  one atomic milestone; NOT a broad plugin content-sync. Reversible to
  plugin-only if the operator prefers (recorded as P3 advisory in the plan).

## Follow-ups (stash)

* `7F0A6E89` (low) — Upstream `spike/SKILL.md.tmpl` drift: the generating
  template lives in the external autoharness repo; update it there (never
  out-of-tree per Principle IV) so regeneration does not overwrite the in-repo
  fix.

## Commits (local on `main`, ahead of origin by 2)

* `5e2b7f8` docs(exec-plans): add spike findings-artifact docline reconciliation plan (E75605E5)
* `6638dd7` chore(harness): harvest E75605E5 into 102-F/102.001-T and stage shipment 091-S

## Verification

* Plan doc passes `backlogit docs lint --profile authoring` (0 findings).
* `backlogit sync` → 813 artifacts indexed; `shipment get 091-S` shows
  `status: queued`, items `[102-F, 102.001-T]`, covering feature `102-F`.
* `backlogit doctor` — only pre-existing orphan `016.001-R` (unrelated); no new
  integrity issues introduced.
