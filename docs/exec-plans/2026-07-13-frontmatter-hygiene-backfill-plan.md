---
chunk_strategy: h1-h2-h3
description: Lean plan to backfill two soft-convention frontmatter gaps — the missing name key on the in-repo spike SKILL.md and the missing chunk_strategy/schema_version keys on four 091-S docline docs — as a single frontmatter-hygiene shipment.
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

Both frontmatter gaps are closed with surgical, additive edits, and existing
validators stay green.

## Scope notes / validators

* **B42F5EF3** is verified against `TestPluginBundleStructurallyValid` (the
  plugin bundle structure test). Note `.github/` is **excluded** from the
  docline scope, so this is governed by the plugin-bundle test, not by
  `backlogit docs lint`.
* **3F3FB119** keys are soft-convention: `backlogit docs lint` currently
  reports the four docs as valid (0 violations) without them, so this is a
  consistency/ingestion-quality backfill, not a lint-failure fix. Note
  `docs/memory/` is docline-**excluded**, so the memory doc's keys are pure
  convention (not lint-enforced) — still added for cross-doc consistency.
  The three in-scope docs must continue to pass `backlogit docs lint` after
  the edit.

## Approach

### Task 1 — B42F5EF3 (add `name:` to spike SKILL.md)

1. Add `name: spike` as the first frontmatter key in
   `.github/skills/spike/SKILL.md`, above the existing `description:`, matching
   the plugin copy exactly.
2. Verify with `go test ./... -run TestPluginBundleStructurallyValid` (and any
   plugin-drift/structure test in the suite).

### Task 2 — 3F3FB119 (add docline keys to four docs)

1. For each of the four docs, add `chunk_strategy: h1-h2-h3` and
   `schema_version: "1.0"` to the YAML frontmatter, preserving existing keys
   and ordering conventions. Keep nested `docline:` blocks at their existing
   indentation; the two new keys are top-level scalars.
2. Verify with `backlogit docs lint` — the three in-scope docs must stay valid
   (0 violations); the memory doc is out of scope but edited for consistency.

## Non-goals

* No broad repo-wide doc-metadata backfill pass (the stash note *suggests*
  folding into one, but this shipment stays scoped to the four named docs).
* No changes to the upstream `.tmpl` sources (out-of-tree; see stash
  `7F0A6E89`, deferred).
* No Go code changes (kept isolated from the timestamp-normalization shipment).

## Acceptance criteria

* `.github/skills/spike/SKILL.md` frontmatter begins with `name: spike` and
  `TestPluginBundleStructurallyValid` passes.
* All four named docline docs carry `chunk_strategy: h1-h2-h3` and
  `schema_version: "1.0"`; `backlogit docs lint` reports the in-scope docs
  valid.

## Estimated effort

Two trivial, surgical edits sets. Well within the 2-hour rule; kept as two
tasks so the skill-doc concern and the docline-doc concern stay isolated.

## Plan review

**Provenance: inline single-agent self-assessment by the Stage agent — NOT a
formal multi-persona `plan-review` skill run.** Labeled honestly: no
independent reviewer personas were spawned.

* **Scope / width isolation:** PASS. Docs-only; no Go/schema/template code.
  Two separate tasks keep the skill-doc vs docline-doc concerns isolated.
* **2-hour rule:** PASS. Both tasks are surgical additive frontmatter edits.
* **Validator coverage:** PASS. Each task names its verifier
  (`TestPluginBundleStructurallyValid`; `backlogit docs lint`).
* **Residual risk:** LOW. Additive keys only; existing lint already green.

Self-assessment disposition: **proceed to harvest.** No `plan-harden` needed
(no schema/distribution/multi-template blast radius).
