---
description: "Ship session memory for shipment 093-S — frontmatter hygiene backfill: build, TDD parity test, review, PR #237, merge 647263c, and post-merge closure with 3 stash follow-ups. Last queued shipment."
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-14T00:00:00Z
  ms.topic: memory
source: docs/memory/2026-07-14/093-S-frontmatter-hygiene-ship-memory.md
title: "093-S frontmatter hygiene backfill — Ship session memory"
---

## Outcome

`ship next` executed queued shipment `093-S` end-to-end and closed it. Merged via
**PR #237**, merge commit **`647263c`** (true merge commit, parents `832e682` +
`ef03a3c`). Shipment `093-S` shipped/archived; feature `104-F` and tasks
`104.001-T`/`104.002-T`/`104.003-T` archived recording `647263c`. **Last queued
shipment — queue empty after this closure.**

## Task IDs completed

* `104.001-T` (task) — added `name: spike` to `.github/skills/spike/SKILL.md`
  matching the plugin twin; NEW targeted TDD parity test.
* `104.002-T` + `104.003-T` (tasks) — backfilled `chunk_strategy: h1-h2-h3` +
  `schema_version: "1.0"` on the four 091-S docline docs (2 per task).
* `104-F` (covering feature) — done/archived.
* `093-S` (shipment) — shipped (`shipment ship 093-S --sha 647263c`).

## Feature branch / PR

* Branch: `feat/093-S-frontmatter-hygiene`; **PR #237**; merged `647263c`.
* Closure branch: `post-merge/093-S`; closure PR opened per §1.10.

## Key decisions & learnings

* **Real TDD red phase for a `.github` mirror.** `TestPluginBundleStructurallyValid`
  only walks `plugin/skills` (`manifest.Skills`), so it CANNOT red-phase a
  `.github/skills` edit. Wrote a dedicated
  `TestGitHubSpikeSkillFrontmatterMatchesPluginCopy` reading BOTH copies and
  asserting FULL frontmatter map parity (`require.Equal`), not just `name`
  presence — confirmed RED (`does not contain "name"`) before, GREEN after.
  Captured as compound learning
  `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md`.
* **Copilot review-fix cycle 1.** Auto-review flagged that the first test compared
  only the `name` value, not full parity. Rewrote to full-map comparison + single
  `parseFrontmatterDoc` helper; committed `ef03a3c`; replied + resolved the
  thread; post-push review on `ef03a3c` raised zero threads → converged.
* **Fresh-binary discipline (092-S lesson).** Built `bin/backlogit-closure.exe`
  from merged HEAD `647263c` and verified embedded `vcs.revision` before any
  lifecycle op. Confirmed the merged 092-S UTC writer: all 5 archived members
  carry `updated_at` in canonical UTC `Z` (093-S is the first shipment shipped
  with that writer — verified end-to-end).
* **compact-context = no-op (honest).** Memory 8 files / 28.2 KB, oldest 4 days;
  recent closures 1 day old; no plan with appended plan-review verbosity. Nothing
  meets compaction thresholds — did NOT fabricate compaction work.
* **Governance follow-ups deferred to stash**, not silently dropped: plan-review
  persona gate not runnable in this env; plans lack a labeled Constitution Check;
  soft docline keys have no regression guard.

## Stash follow-ups created

* `8CD8F46A` — task/medium — plan-review persona gate: enable or formally waive.
* `CA877CD1` — task/low — add labeled "Constitution Check" section to impl-plan.
* `A4BE2FAD` — task/low — regression guard for soft docline keys
  (`chunk_strategy`/`schema_version`).

## Retained scratch artifact

`docs/decisions/2026-07-13-scratch-spike.md` — untracked, retained; NOT committed
or deleted (Principle VII; deletion needs operator approval).

## Follow-ups / open items

* Pre-existing doctor orphan `016.001-R` (old review artifact, unrelated to
  093-S) remains — out of scope for this closure; needs a separate deliberate
  remediation decision (destructive `--fix-orphans` not run).
* Closure PR HALTED at §1.10 / P-014 "closure PR ready, awaiting operator merge
  approval" — NOT self-merged.
