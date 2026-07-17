---
chunk_strategy: h1-h2-h3
description: Two recurring JSON Schema authoring traps in layered allOf extension schemas — a property `default` does not enforce presence, and redeclaring a property `type` in the extension silently narrows the base contract's type union (e.g. object|null down to object), rejecting base-valid documents.
doc_type: learning
docline:
  date: 2026-07-16T00:00:00Z
  severity: medium
  tags:
    - json-schema
    - docline
    - schema-design
    - allOf
    - review
schema_version: "1.0"
source: docs/compound/2026-07-16-jsonschema-default-nonenforcing-and-base-union-narrowing.md
title: JSON Schema layered-extension traps — default is annotation-only and re-typing narrows the base union
---

## Problem

When authoring a layered `allOf` extension schema (a derived schema that composes
a base contract via `allOf: [{$ref: base}]` and adds owner-scoped constraints),
two subtle correctness bugs slipped past initial authoring and were caught in
review of `schemas/docline/ext/backlogit-v1.schema.json` (shipment 097-S):

1. **`default` does not enforce presence.** The owner profile object declared
   `schema_version` with `default: "1.0"` and no `required`. `default` is a
   non-enforcing annotation — validators do not populate it. A document with
   `docline.backlogit: {"size": 4212}` validated and stayed unversioned, silently
   violating the documented "every owner profile carries its own schema_version"
   rule.

2. **Re-declaring `type` narrows the base union.** The extension declared
   `properties.docline.type: "object"`. The base contract permits
   `docline: object | null` (and defaults to null). Because `allOf` intersects
   constraints, the extension's `type: object` narrowed the union and rejected an
   otherwise base-valid document whose `docline` is `null` — even when no
   `docline.backlogit` subtree exists.

## Root cause

An extension composed with `allOf` **adds** constraints; every keyword it
declares is intersected with the base. Authors instinctively re-state the
container's `type: object` for readability and reach for `default` to express an
expected value. Both instincts are wrong in a layered contract:

* `default` expresses intent, not a rule. Only `required` (or `minProperties`,
  `enum`, etc.) constrains.
* Re-typing a container that the base defines as a union quietly subtracts the
  other union members from the composed contract.

## Solution

* To require a field **only when its parent object is present**, put `required`
  on the parent object, not on the grandparent. Here: `docline.backlogit` is
  optional, but when present it must carry `schema_version` — so
  `required: ["schema_version"]` lives on the `backlogit` object.
* To constrain nested keys **without** narrowing a base union, omit `type` on the
  container. JSON Schema `properties` is ignored for non-object instances, so
  `properties.backlogit` still applies when `docline` is an object and
  `docline: null` still passes. Removing `type: object` restored base-null
  compatibility.
* Pin both invariants with a structural contract test that reads the schema JSON
  and asserts (a) the container declares no narrowing `type`, and (b) the owner
  profile lists `schema_version` in `required`. This prevents silent regression.

## Evidence

* `schemas/docline/ext/backlogit-v1.schema.json` — final schema: no `type` on the
  `docline` container; `required: ["schema_version"]` on `backlogit`.
* `internal/docline/schema_contract_test.go` —
  `TestExtSchemaPreservesBaseNullAndRequiresProfileVersion` pins both.
* PR #245 (shipment 097-S), fix commit `8127fa5`; both findings raised by Copilot
  review and confirmed valid.

## When this applies

Any layered/derived JSON Schema that composes a base via `allOf` and adds
owner- or profile-scoped keys: do not re-type base-defined containers, and use
`required` (not `default`) to enforce presence.
