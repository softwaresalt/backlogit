---
chunk_strategy: h1-h2-h3
description: "Ship session memory — shipment 097-S (docline.backlogit owner-scoped extension namespace, Model A) shipped end-to-end: PR #245 merged b9bae62, 2 Copilot findings fixed, shipment archived, closure/compound artifacts produced."
doc_type: memory
schema_version: "1.0"
docline:
  ms.date: 2026-07-16T00:00:00Z
  ms.topic: reference
source: docs/memory/2026-07-16/097-S-ship-closure-memory.md
title: "097-S Ship closure — session memory"
---

## Task IDs completed

* Shipment **097-S** shipped/archived (Model A docline.backlogit owner profile).
* Members archived: `110-F`, `110.001-T`, `110.002-T`, `110.003-T`, linked
  deliberation `052-DL`.

## Key SHAs

* PR #245 merge commit: **`b9bae62`** (true merge commit; parents `c1df5a4` +
  `8127fa5`).
* Fix commit (review): `8127fa5`. Impl commits: `26b6145`, `7a647a1`, `7df0a70`.
* Post-merge backlog-state commit: `cd82c54` (on closure branch
  `chore/097-S-post-merge-closure`).

## Files modified/created

* NEW `schemas/docline/ext/backlogit-v1.schema.json`
* NEW `internal/docline/schema_contract_test.go` (4 subtests)
* NEW `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md`
* MOD `docs/docline-frontmatter-authoring-guide.md`
* NEW `docs/closure/2026-07-16-097-S-ship-closure.md`
* NEW `docs/compound/2026-07-16-jsonschema-default-nonenforcing-and-base-union-narrowing.md`
* Backlog state: `097-S` + `052-DL` queue→archive; member archive status stamps.

## Decisions & rationale

* **Model A** (backlogit as owner profile nested under the open `docline` map;
  base top level stays `additionalProperties:false`) — reverses Model B's
  top-level-extension direction (107.009-T/107.011-T). Enables engram-style dual
  docline+backlogit schema awareness without the base enveloping backlogit fields.
* **Scope boundary:** docline DOCUMENTS only. Artifact-frontmatter durability of
  `docline.backlogit.*` deferred to the 109.x spike (artifact codec carries only
  `custom_fields`; top-level `docline` dropped on artifacts).

## Review learnings (captured to compound)

* JSON Schema `default` is annotation-only — use `required` to enforce presence.
* Re-declaring `type` on a base-defined container in an `allOf` extension narrows
  the base type union (object|null → object), rejecting base-valid docs. Omit it.

## Process notes

* Direct push to `main` is blocked by branch protection — post-merge closure
  state must ship via a closure PR (`chore/097-S-post-merge-closure`).
* `start.ps1` (modified) and `docs/decisions/2026-07-13-scratch-spike.md`
  (untracked) are intentionally kept UNSTAGED across the whole session.
* Local Go toolchain is go1.26.5 vs repo pin 1.24.0 → repo-wide `gofmt -l .`
  flags ~28 pre-existing unrelated files; ignore (not CI-enforced; golangci-lint
  owns formatting).

## Next steps

* Closure PR (`chore/097-S-post-merge-closure`) awaiting Copilot review + §1.9
  gate + operator merge approval.
* Shipment queue is being drained — re-assess remaining queued shipments after
  closure PR merges.
* 109.x spike remains the home for the artifact-frontmatter `docline.backlogit`
  bridge decision.
