---
chunk_strategy: h1-h2-h3
description: Closure record for install documentation cleanup from stash AD5C3E0C and 53FCC92A.
doc_type: closure
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-10-install-docs-npm-cleanup-closure.md
title: Install Docs NPM Cleanup Closure
---

## Scope

This release unit shipped backlog feature `091-F` with tasks `091.001-T` and
`091.002-T`, harvested from stash `AD5C3E0C` and `53FCC92A`.

The change kept the scope to install documentation:

* README Quick Start now leads with the Windows PowerShell `irm` installer and
  Linux/macOS `curl` installer
* README and the plugin guide no longer present npm global install as a user
  install path
* The plugin guide still describes the current `npx` wrapper bootstrap factually
  and points the direct GitHub Releases bootstrap migration to stash `60B8564F`
* Installation Method 1 now matches the Windows-first Quick Start priority

## Validation

Local validation before PR:

* `backlogit docs lint`
* Markdown link and code-fence check for README, plugin guide, and installation
  guide
* Critical self-review assertions for installer ordering, npm install removal,
  wrapper accuracy, and Method 1 ordering

PR validation:

* PR `207`: <https://github.com/softwaresalt/backlogit/pull/207>
* CI passed: Detect code changes, test, Docline frontmatter gate, and CLI
  Reference Drift
* Copilot review covered final head `2d5cc046517e82cff86a0bdd5ffe472be1aa66c2`
* All Copilot review threads were replied to and resolved before merge

## Merge

PR `207` merged normally with a merge commit:

* Merge commit: `f965f60b71a5bbf6e6ca8440affbb2dd9e125215`
* Admin fallback: not used
* Dark-mode marker: `DARK_MODE_MERGE_AUTHORIZED`

## Backlog state

Backlog items were completed and archived after the merge:

| Item | Final state |
|---|---|
| `091-F` | archived |
| `091.001-T` | archived |
| `091.002-T` | archived |

## Operational notes

No runtime surface changed. Monitoring is not applicable beyond normal docs CI
and operator review of the published Markdown.

## Follow-up

The plugin bootstrap migration away from the current `npx` wrapper remains out
of scope and is tracked separately by stash `60B8564F`.
