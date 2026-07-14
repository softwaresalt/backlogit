---
chunk_strategy: h1-h2-h3
description: Lean plan to backfill two soft-convention frontmatter gaps — the missing name key on the in-repo spike SKILL.md (with a targeted RED-phase test) and the missing chunk_strategy/schema_version keys on four 091-S docline docs — as a single frontmatter-hygiene shipment.
doc_type: plan
docline:
    stash_ref: B42F5EF3
    conclusion: proceed
    confidence: high
schema_version: "1.0"
source: docs/exec-plans/2026-07-13-frontmatter-hygiene-backfill-plan.md
title: Frontmatter hygiene — spike SKILL name key + 091-S docline metadata backfill
---

## Frontmatter hygiene — spike SKILL name key + 091-S docline metadata backfill

## Context

Two low-priority, soft-convention frontmatter gaps surfaced during 091-S
review. Both are documentation/frontmatter hygiene (same artifact class), so
they group into one shipment while remaining **separate tasks** (different
files and validators — artifact-class isolation within the shipment):

* **`B42F5EF3`** — `.github/skills/spike/SKILL.md` is missing the top-level
  `name:` frontmatter key that its `plugin/skills/spike/SKILL.md` counterpart
  carries. Verified: the `.github` copy has only `description:`; the plugin
  copy has both `name: spike` and `description:`. Pre-existing inconsistency
  (not introduced by 091-S).
* **`3F3FB119`** — four 091-S closure/compound/memory docline docs are missing
  the `chunk_strategy: h1-h2-h3` and `schema_version: "1.0"` keys used by the
  current docline convention:
  * `docs/compound/2026-07-13-copilot-review-loop-convergence.md`
  * `docs/closure/2026-07-13-091-S-spike-docline-closure.md`
  * `docs/closure/2026-07-13-091-S-compound-refresh.md`
  * `docs/memory/2026-07-13/091-S-spike-docline-ship-memory.md`

## Goal

Both frontmatter gaps are closed with surgical, additive edits; a targeted test
guards the `.github` skill metadata; and existing validators stay green.

## Scope notes / validators

* **`.github/` is docline-excluded**, so `backlogit docs lint` does not govern
  the spike SKILL edit. The `TestPluginBundleStructurallyValid` integration test
  (`tests/integration/plugin_manifest_test.go:124-154`) **only traverses
  `plugin/skills`** — it reads `manifest.Skills` (= `plugin/skills`) and asserts
  `SKILL.md` metadata for the *plugin* copies (lines 143-152). It never reads
  `.github/skills/spike/SKILL.md`, so it passes both before AND after the
  `.github` edit and **cannot provide a red phase** (PR #234 review, threads
  `3575108875` / `3575121058`). A dedicated test is required.
* **`3F3FB119`** keys are soft-convention: `backlogit docs lint` currently
  reports the four docs as valid (0 violations) without them, so this is a
  consistency/ingestion-quality backfill, not a lint-failure fix. Note
  `docs/memory/` is docline-**excluded**, so the memory doc's keys are pure
  convention (not lint-enforced) — still added for cross-doc consistency. The
  three in-scope docs must continue to pass `backlogit docs lint` after the edit.

## Approach

### Task 1 — `B42F5EF3` (add `name:` to spike SKILL.md, test-first)

1. **Write the failing test first (red phase).** Add a targeted test that reads
   `.github/skills/spike/SKILL.md`, parses its YAML frontmatter, and asserts the
   top-level `name:` key exists and equals `spike` — OR, preferably, asserts
   **key parity** between `.github/skills/spike/SKILL.md` and
   `plugin/skills/spike/SKILL.md` (identical top-level frontmatter keys). This
   test **fails** on the current `.github` copy (no `name:`). Keep
   `TestPluginBundleStructurallyValid` as a **secondary regression gate only**,
   and document explicitly that it does NOT cover the `.github` copy.
2. **Make the change.** Add `name: spike` as the first frontmatter key in
   `.github/skills/spike/SKILL.md`, above `description:`, matching the plugin
   copy exactly.
3. **Green.** The new parity/metadata test passes; `TestPluginBundleStructurallyValid`
   stays green.

### Task 2 — `3F3FB119` (add docline keys to four docs)

1. For each of the four docs, add `chunk_strategy: h1-h2-h3` and
   `schema_version: "1.0"` to the YAML frontmatter, preserving existing keys and
   ordering conventions. Keep nested `docline:` blocks at their existing
   indentation; the two new keys are top-level scalars.
2. Verify with `backlogit docs lint` — the three in-scope docs must stay valid
   (0 violations); the memory doc is out of scope but edited for consistency.

## Non-goals

* No broad repo-wide doc-metadata backfill pass (the stash note *suggests*
  folding into one, but this shipment stays scoped to the four named docs).
* No changes to the upstream `.tmpl` sources (out-of-tree; see stash
  `7F0A6E89`, deferred).
* No Go production-code changes beyond the new test (kept isolated from the
  timestamp-normalization shipment `092-S`).

## Acceptance criteria

* A dedicated test asserts the `name:` key (or `.github`↔`plugin` frontmatter
  key parity) for `spike/SKILL.md`, **fails** on the pre-change `.github` copy,
  and passes after `name: spike` is added; `TestPluginBundleStructurallyValid`
  stays green as a secondary regression gate.
* `.github/skills/spike/SKILL.md` frontmatter begins with `name: spike`.
* All four named docline docs carry `chunk_strategy: h1-h2-h3` and
  `schema_version: "1.0"`; `backlogit docs lint` reports the in-scope docs valid.

## Estimated effort

Two trivial, surgical edit sets (plus one small targeted test for Task 1). Well
within the 2-hour rule; kept as two tasks so the skill-doc concern and the
docline-doc concern stay isolated.

## Plan review

**Provenance (honest): inline single-agent self-assessment + incorporation of
external automated review (GitHub Copilot code review on PR #234).** This is
**NOT** a formal multi-persona `plan-review` skill run. The formal
`plan-review` skill (`.github/skills/plan-review/SKILL.md`; required gate in the
AGENTS.md Release Unit Pipeline) **spawns independent reviewer persona
subagents**; the Stage agent in this environment cannot dispatch persona
subagents, so the formal gate could not be executed. It is not simulated or
claimed.

External review finding incorporated into this revision:

* **[C — high] Test provides no red phase** (threads `3575108875`,
  `3575121058`): resolved by the targeted `.github`-copy parity/metadata test in
  Task 1, which fails pre-change; `TestPluginBundleStructurallyValid` demoted to
  a secondary regression gate with an explicit note that it does not cover the
  `.github` copy.
* **[E — nit] Grammar** (threads `3575108909`, `3575121086`): "edits sets" →
  "edit sets" (Estimated effort).

Self-assessment: scope/width isolation PASS (docs/test only, no Go/schema/template
family code); 2-hour rule PASS; test-first PASS for Task 1 (red phase now
demonstrable); validator coverage PASS (each task names its verifier). Residual:
none blocking.

**Recommended pre-build follow-up:** run the formal multi-persona `plan-review`
skill against this revised plan before Ship builds `093-S`, and append its real
result here.
