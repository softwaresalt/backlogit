---
chunk_strategy: h1-h2-h3
description: Compacted Ship session memory for the 091-S/092-S/093-S shipment batch — docline spike reconciliation, item-writer UTC normalization, and frontmatter hygiene backfill.
doc_type: memory
docline:
    ms.date: 2026-07-14T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-14-091-092-093-S-ship-batch-compacted.md
title: Compacted memory - 091/092/093-S ship batch
---

## Summary

Three consecutive Ship cycles, each shipped and closed via a dedicated closure PR
(direct pushes to `main` are ruleset-blocked):

* **091-S** — spike findings-artifact docline reconciliation. PR #231, merge
  `ec2b859`. Members `102-F`, `102.001-T` archived. Impl commit `fd0a30b` replaced
  the `spike/SKILL.md` Phase-5 example (both plugin + `.github` copies) with a
  docline-conformant block using 4-space indentation (repo gold standard).
* **092-S** — item-writer UTC timestamp normalization (11 TDD tasks). PR #235,
  merge `4a90bf4`. Feature `103-F`, tasks `103.001-T`…`103.011-T` archived. Every
  item-artifact writer now emits `created_at`/`updated_at` in canonical UTC (`Z`)
  via the exported `models.NowUTC()`; the read path stays offset-tolerant.
* **093-S** — frontmatter hygiene backfill (last queued shipment of that batch).
  PR #237, merge `647263c`. Feature `104-F`, tasks `104.001-T`/`002-T`/`003-T`
  archived. Added `name: spike` to `.github/skills/spike/SKILL.md` and backfilled
  `chunk_strategy`/`schema_version` on the four 091-S docline docs.

## Archived originals

* `docs/archive/memory/091-S-spike-docline-ship-memory.md`
* `docs/archive/memory/092-S-item-writer-utc-ship-memory.md`
* `docs/archive/memory/093-S-frontmatter-hygiene-ship-memory.md`

## Decisions and learnings

* **Shared exported helper in the lowest package (092-S).** `NowUTC()` lives in
  `internal/models` and is exported so `core/templates` and `cli` reuse it with no
  import cycle. Normalize on write; stay liberal on read (historical offset
  artifacts still load — no corpus migration).
* **Parallel-test-safe RED phase (092-S).** For `t.Parallel()` packages
  (`internal/cli`), a process-global `time.Local` override is a data race
  (`go test -race` trips it). Use a hermetic `TZ=America/Los_Angeles` subprocess
  re-exec instead. Assert the exact trailing `Z` (`HasSuffix(v,"Z")` AND not
  `[+-]\d{2}:\d{2}$`), which is stronger than asserting a zero offset.
* **Real TDD red phase for a `.github` mirror (093-S).**
  `TestPluginBundleStructurallyValid` only walks `plugin/skills`, so it cannot
  red-phase a `.github/skills` edit. A dedicated parity test reading BOTH copies
  and asserting full frontmatter-map equality is required (confirmed RED before,
  GREEN after).
* **Two-commit feature-branch discipline (091-S).** Separate the implementation
  diff (`docs(harness):`) from the `.backlogit/` claim/completion lifecycle
  mutations (`chore(harness):`).

## Failed approaches / gotchas (carried forward)

* **Stale workspace binary re-emits closed defects (092-S).** The post-merge
  `ship_shipment` ran a `backlogit.exe` built before the merge and re-stamped
  archive `updated_at` in local `-07:00` — the exact defect 092-S closes. Caught by
  the closure PR Copilot review. Reflex: **rebuild the tool from merged HEAD before
  any post-merge write**, then re-verify the write path. Compound learning:
  `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`.
* **`ship_shipment` overwrites members' `commit` with the merge SHA** (impl commit
  → merge SHA). Expected; both recorded in closure artifacts.
* `gh pr edit <n> --add-reviewer "copilot"` returns `'' not found`; not needed —
  the `main` ruleset auto-triggers `copilot_code_review` on every push.
* Backlogit **MCP tools resolve the installed-plugin workspace, not the repo
  root** — use the repo CLI (`.\backlogit.exe … --cwd .`) for repo backlog work.
* `docs/memory/` is excluded from the docline gate scope; `docs/closure/`
  (→ `closure`) and `docs/compound/` (→ `learning`) ARE gate-checked by
  `make docs-lint`. There is no `backlogit reconcile` subcommand — GI/GR reconcile
  is a manual verification gate (`ShipShipment` runs `VerifyPostShipConsistency`).

## Follow-ups (open at batch close)

* Stash `7F0A6E89` (low) — out-of-tree upstream `spike/SKILL.md.tmpl` in the
  external autoharness repo (Principle IV, carried 091-S→092-S).
* 093-S closure created three stash follow-ups: `8CD8F46A` (plan-review persona
  gate enable/waive), `CA877CD1` (labeled Constitution Check section in impl-plan),
  `A4BE2FAD` (regression guard for soft docline keys `chunk_strategy`/`schema_version`).
* Pre-existing doctor orphan `016.001-R` (unrelated old review artifact) — needs a
  separate deliberate remediation (destructive `--fix-orphans` not run).
* Retained scratch `docs/decisions/2026-07-13-scratch-spike.md` — untracked,
  awaiting an operator deletion decision (Principle VII).
