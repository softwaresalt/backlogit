---
description: "Ship post-merge closure for shipment 097-S — docline.backlogit owner-scoped extension namespace (Model A): feature 110-F and tasks 110.001-T/110.002-T/110.003-T. PR #245 merged b9bae62; shipment shipped/archived with linked deliberation 052-DL; 2 Copilot review findings fixed; compound learning captured."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-16T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-16-097-S-ship-closure.md
title: "097-S docline.backlogit owner-scoped extension (Model A) — Ship closure"
---

## Outcome

`ship next` executed queued shipment **097-S** (docline.backlogit owner-scoped
extension namespace, Model A) end-to-end and closed it. Merged via **PR #245**,
merge commit **`b9bae62`** (true merge commit — parents `c1df5a4` base +
`8127fa5` PR head, P-009 satisfied). Shipment `097-S` shipped/archived; feature
`110-F` and tasks `110.001-T`/`110.002-T`/`110.003-T` archived, along with the
linked deliberation `052-DL`.

## Model A (what shipped)

Backlogit extends the docline base contract as an **owner profile** nested under
the open `docline` map at `docline.backlogit.*`. The docline base top level
stays `additionalProperties: false` (Model B's top-level-extension direction —
tasks 107.009-T/107.011-T — was explicitly reversed). This lets a consumer such
as engram be both docline- and backlogit-schema aware without the base contract
enveloping backlogit-owned fields.

**Scope boundary (deliberate):** deliverables cover docline DOCUMENTS only.
`docline.backlogit.*` durability on `.backlogit` ARTIFACT frontmatter is NOT
resolved here (the generic artifact codec carries only `custom_fields`, so a
top-level `docline` map is dropped on artifacts). That bridge is deferred to the
109.x spike. This PR does not touch the artifact codec.

## Members completed

| ID | Type | Work |
|---|---|---|
| `110.003-T` | task | Schema-contract test (RED) + layered ext schema `schemas/docline/ext/backlogit-v1.schema.json` (GREEN): `allOf` on base v1, constrains only `docline.backlogit`. |
| `110.001-T` | task | Model A decision doc `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md`. |
| `110.002-T` | task | "Owner-scoped extension profiles" section + See-also links in `docs/docline-frontmatter-authoring-guide.md`. |
| `110-F` | feature | Covering feature — done/archived. |
| `052-DL` | deliberation | Linked deliberation — archived on ship. |
| `097-S` | shipment | Shipped (`shipment ship 097-S`), queue→archive. |

## Files changed (PR #245, merged b9bae62)

* `schemas/docline/ext/backlogit-v1.schema.json` — **NEW** layered owner-profile
  ext schema.
* `internal/docline/schema_contract_test.go` — **NEW** 4 subtests pinning the
  Model A contract (base stays closed, ext composes base v1, ext constrains only
  `docline.backlogit`, base-null preserved + profile version required).
* `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md`
  — **NEW** decision doc.
* `docs/docline-frontmatter-authoring-guide.md` — owner-profile convention
  section.

## Review findings addressed

Two Copilot review findings on the ext schema, both fixed in `8127fa5`,
replied, and resolved via GraphQL:

1. **Base union narrowing** — the ext declared `docline` as `type: object`,
   narrowing the base's `object | null` and rejecting base-valid `docline: null`
   documents. Removed the `type` constraint; `properties` still constrains
   `docline.backlogit` whenever `docline` is an object.
2. **Non-enforcing default** — `schema_version` used `default: "1.0"` (an
   annotation, not a constraint), permitting an unversioned profile. Added
   `required: ["schema_version"]` to the backlogit profile.

Both fixes are pinned with a new contract-test regression guard
(`TestExtSchemaPreservesBaseNullAndRequiresProfileVersion`). Copilot re-reviewed
the fix HEAD (`8127fa5`) and raised no new threads.

## Gate evidence

* CI on merged HEAD: 4/4 green (`test`, CLI Reference Drift, Detect code
  changes, Docline frontmatter gate).
* §1.9 pre-merge readiness: review fresh on HEAD, 0 unresolved Copilot threads,
  no branch-protection block.
* Local gates that PASSED on merged `main`: `go build`, `go test
  ./internal/docline/...`, `go vet`, `golangci-lint run ./internal/docline/...`,
  `gofmt -l` on the changed files (empty output), and ext-schema JSON validity.
* Format-check caveat (non-passing, non-blocking): repo-wide `gofmt -l .` reports
  ~28 pre-existing unrelated files as unformatted. This is a local
  toolchain-version artifact (local go1.26.5 vs the repo pin 1.24.0), NOT a
  regression from this shipment and NOT CI-enforced — CI has no standalone
  `gofmt` gate; golangci-lint (with its own pinned formatter) owns formatting.
  The changed files in this shipment are `gofmt`-clean.
* P-009: true merge commit `b9bae62` (two parents). P-014: operator approved
  merge explicitly.

## Post-merge closure

* `shipment ship 097-S` → `shipped`; archived `097-S`, `110-F`,
  `110.001/002/003-T`, and linked `052-DL`.
* **Release-SHA traceability:** the initial `shipment ship 097-S` was run without
  `--sha`, so the merge SHA was not attached. Backfilled the parent merge commit
  `b9bae626c1cb6c8243511f669b6b0b6e06b3f0fd` onto every archived scope item
  (`097-S`, `110-F`, `110.001-T`, `110.002-T`, `110.003-T`, `052-DL`) via
  `backlogit update <id> --commit b9bae62`, writing the durable frontmatter
  `commit` field on each. Re-running `shipment ship --sha` was not possible
  because the shipment was already `shipped` (the ship path requires
  `status: active`). Future ships should pass
  `shipment ship <id> --sha <merge-sha> --message ... --author ...` in one step.
* Shipment-reconcile GI/GR (pre + post): all manifest items confirmed archived;
  `097-S.md` moved queue→archive.
* Post-merge backlog state committed (`cd82c54`) and shipped via closure branch
  `chore/097-S-post-merge-closure` (direct push to `main` blocked by branch
  protection).

## Follow-ups

* `docline.backlogit` durability on `.backlogit` artifact frontmatter remains
  open — gated on the **109.x** ownership spike (artifact-codec bridge).
