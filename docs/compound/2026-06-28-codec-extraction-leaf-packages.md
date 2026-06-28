---
chunk_strategy: h1-h2-h3
description: 'Durable refactor pattern from 068-S — break a cross-package duplication/import cycle by extracting the shared logic into stdlib-only leaf packages, preserve the original package API (including an inherited method) via a true Go type alias instead of re-declaration, and prove behavior preservation with differential golden byte-equality tests over real and generated output'
doc_type: learning
docline:
    date: 2026-06-28T00:00:00Z
    severity: medium
    tags:
        - refactor
        - leaf-package
        - import-cycle
        - type-alias
        - go
        - golden-test
        - byte-equality
        - behavior-preserving
        - codec
        - docline
ingested_at: "2026-06-28T19:54:00Z"
schema_version: "1.0"
source: docs/compound/2026-06-28-codec-extraction-leaf-packages.md
title: 'Behavior-preserving extraction: stdlib-only leaf packages + true type alias + differential golden byte-equality tests (068-S)'
---

# Behavior-Preserving Extraction via Leaf Packages, a True Type Alias, and Golden Byte-Equality Tests

Three durable techniques graduated from shipment 068-S (feature 068-F, PR #148,
merge `7450271a`), which removed the `internal/docline <-> internal/core`
body-preserving-codec + atomic-write duplication that the 062-F import-cycle
workaround had introduced.

## Context

Package A (`internal/docline`) and package B (`internal/core`) had each grown a
private copy of the same body-preserving frontmatter codec and atomic-write
helper, because making B import A (or vice versa) would have created an import
cycle. Duplication is the classic "cycle workaround" tax: two copies drift,
two test surfaces, double maintenance.

## Rule 1 — Break the cycle by extracting shared logic into a stdlib-only *leaf* package

Move the shared logic **down**, not sideways. Create a new package that:

- imports **only the standard library** (here `internal/mdfront` = `bytes` + `fmt`
  + `gopkg.in/yaml.v3`; `internal/atomicfile` = pure stdlib), and
- imports **no internal package at all**.

A package with zero internal imports is a *leaf*: any number of higher-level
packages can depend on it without ever forming a cycle. Both A and B now import
the leaf instead of carrying private copies. The cycle is structurally
impossible to re-introduce as long as the leaf stays import-free — which is worth
asserting in an architecture note and, ideally, a lint rule.

> Heuristic: when two peer packages duplicate logic to dodge a cycle, the fix is
> almost always a new leaf beneath both, not a new edge between them.

## Rule 2 — Preserve the original package's API (including an inherited method) with a *true type alias*, not a re-declaration

To keep every existing caller of `docline.Markdown` compiling unchanged, the
migrated package re-exports the moved type with a **true type alias**:

```go
type Markdown = mdfront.Markdown   // alias '=', NOT 'type Markdown mdfront.Markdown'
```

Why the alias (`=`) and not a named defined type:

- An **alias** makes `docline.Markdown` and `mdfront.Markdown` the *same* type, so
  methods defined on `mdfront.Markdown` (e.g. `(*Markdown).Encode()`) are
  **inherited** and callable through the alias. No forwarding wrapper is needed,
  and there is zero behavior or signature drift.
- A **named defined type** (`type Markdown mdfront.Markdown`) would be a *distinct*
  type that does **not** inherit the source type's methods, forcing you to
  re-declare every method — reintroducing exactly the duplication you set out to
  remove. (Go also forbids declaring a method on a type whose definition is in
  another package, so you cannot bolt the method back on.)
- Package-level **functions** (not methods) still need a thin forwarding
  re-export when you want to keep the old call site: `func Decode(...) { return mdfront.Decode(...) }`.
  Only do this for functions; let the alias carry the methods.

Net effect: callers see an identical API surface; the implementation lives in one
place.

## Rule 3 — Prove a "behavior-preserving" refactor with *differential golden byte-equality* tests

A refactor's entire value proposition is "nothing observable changed." Make that
claim **testable and mechanical**, not a code-review opinion:

- **Byte-equality on the codec**: assert the moved `Encode`/`Decode` round-trips
  real inputs to **byte-identical** output, and that the migrated repair path
  (`rewriteArchivedFromField`) changes only the intended frontmatter field while
  leaving body bytes untouched (`body_bytes_changed: false`).
- **Byte-equality on generated output**: regenerate `cmd/gen-docs` output and
  `git diff --exit-code` it — a green CLI Reference Drift check proves the public
  formatting path is unchanged.
- **Idempotency**: re-applying the docline migration to an already-compliant file
  yields an empty content diff.
- **Live dogfooding**: the first real operation through the refactored path (here
  the 068-S `shipment ship`, which re-stamped 6 archive records) is the strongest
  proof — production data flowed through the new packages and `doctor
  --check-archived-from` stayed at 0 self-referential with body bytes preserved.

These checks turn "trust me, it's the same" into a CI guardrail set
(`test`, `Docline frontmatter gate`, `CLI Reference Drift Check`) that will catch
any future regression of the extracted logic.

## Evidence

- Shipped code at merge `7450271a` (PR #148): `internal/mdfront/` (codec.go, doc.go),
  `internal/atomicfile/` (atomicfile.go, doc.go), `internal/docline/` alias +
  forwarders, `internal/core/doctor.go` migrated onto the leaf packages.
- Runtime verification (this closure): targeted suites green; `gen-docs` 0 drift;
  `docs migrate` 0 body-byte changes over 233 entries; `docs lint` clean; live ship
  re-stamped 6 records canonically with 0 doctor self-refs and preserved body bytes.
- Plan: `docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md`.
- Deliberation: `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md`.
- Closure: `docs/closure/2026-06-28-068-S-codec-extraction-closure.md`.
- Design rationale: `docs/design-docs/2026-06-28-frontmatter-codec-leaf-packages.md`.

## Applicability

Reuse this trio for any "two packages duplicated X to avoid a cycle" cleanup, and
more generally for any move-without-behavior-change refactor: extract to a leaf,
alias the moved type to keep inherited methods and the caller API stable, and gate
the "nothing changed" claim with differential byte-equality + idempotency +
live-dogfooding evidence.

## Related learnings

- `docs/compound/2026-06-26-docline-frontmatter-contract.md` — the body-preserving
  codec + idempotent migration + born-compliant generation + CI gate pattern. The
  codec *implementation* it cites now lives in `internal/mdfront` (relocated by
  068-S); the contract pattern is otherwise unchanged. See the 068-S compound-refresh
  for the keep/update classification.
