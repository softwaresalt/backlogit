---
title: "A structural bundle test that only walks the published tree cannot guard the in-repo .github mirror — add a targeted full-frontmatter parity test"
source: docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md
doc_type: learning
description: "This repo keeps skills in two places: the authoritative plugin/skills/*/SKILL.md (what TestPluginBundleStructurallyValid validates, resolved from manifest.Skills == 'plugin/skills') and an in-repo .github/skills/*/SKILL.md mirror that agents actually load. The bundle-validity test walks only the plugin tree, so it passes both before AND after any edit to a .github copy and provides no red phase for drift there. When .github/skills/spike/SKILL.md was missing the top-level name: key its plugin twin had, no existing test could fail. Rule: a structural test scoped to one copy of a mirrored asset gives false confidence about the other copy. Guard mirrored assets with a targeted test that reads BOTH paths and asserts full parity (every key AND value via a map equality), not just presence of one key — and keep the broad structural test as a secondary regression gate for its own scope only."
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-07-14T00:00:00Z
    severity: medium
    tags:
        - testing
        - tdd
        - red-phase
        - skills
        - plugin
        - dot-github
        - parity
        - mirrored-assets
        - frontmatter
        - ship
---

# A Bundle Test Scoped to the Published Tree Cannot Guard the .github Mirror

## Context

Surfaced during shipment 093-S / task 104.001-T (PR #237, merged `647263c`).
The task added the missing top-level `name:` key to
`.github/skills/spike/SKILL.md` so it matches its authoritative twin
`plugin/skills/spike/SKILL.md`. The plan (correctly) demanded a genuine TDD red
phase: a test that FAILS before the edit and PASSES after.

The obvious candidate — `TestPluginBundleStructurallyValid` — could not provide
that red phase.

## Problem

`TestPluginBundleStructurallyValid` resolves the skills directory from
`manifest.Skills`, which equals `"plugin/skills"`
(`tests/integration/plugin_manifest_test.go:105,129`). It therefore only ever
reads `plugin/skills/*/SKILL.md`. It never opens anything under
`.github/skills/`. Consequences:

1. **No red phase.** The test passes both before and after editing a `.github`
   copy, so it cannot drive TDD for `.github` drift and cannot regress-catch it
   later either.
2. **False confidence from a green suite.** A mirrored asset has two copies; a
   structural test scoped to one copy says nothing about the other. The suite is
   green while the two copies silently diverge.
3. **The loaded copy is the unguarded one.** Agents load skills from
   `.github/skills/`, so the copy with the weaker guarantee is the copy actually
   in the execution path.

## Solution

Add a targeted test that reads **both** paths and asserts parity:

* `TestGitHubSpikeSkillFrontmatterMatchesPluginCopy`
  (`tests/integration/github_skill_parity_test.go`) parses the leading YAML
  frontmatter of each `SKILL.md` into a generic `map[string]any` (helper
  `parseFrontmatterDoc`) and compares them.
* Two-layer assertion: (1) a sharp `require.Contains(githubDoc, "name")` that
  yields a clear failure message for the specific gap the task closes, then
  (2) `require.Equal(pluginDoc, githubDoc)` for **full** top-level parity — every
  key AND value. The full-map equality (not just single-key presence) is what
  earns the test its `MatchesPluginCopy` name and also catches future drift in
  shared scalars like `description`.
* Verified genuinely RED first: before the edit it failed with
  `[]string{"description"} does not contain "name"`; after adding `name: spike`
  it passed.
* Keep `TestPluginBundleStructurallyValid` as a **secondary** regression gate for
  the plugin copies only, and say so explicitly in the new test's doc comment so
  the next reader knows the coverage boundary.

## Applicability

Any repository that keeps a mirrored asset in two trees — a published/bundled
copy plus an in-repo working copy (`.github/` mirrors, vendored copies,
generated-vs-source pairs, `dist/` vs `src/`). The generalized reflex: **a test
that walks only one copy of a mirrored asset proves nothing about the other, so
name its scope honestly and add a parity test that reads every mirror and
asserts full-value equality** rather than presence of a single field. Presence
checks pass as soon as a key exists; value-level map equality is what actually
prevents silent divergence.
