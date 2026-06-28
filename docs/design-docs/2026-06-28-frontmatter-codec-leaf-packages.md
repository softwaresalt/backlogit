---
chunk_strategy: h1-h2-h3
description: 'Design rationale for the internal/mdfront and internal/atomicfile leaf packages (shipped 068-S) — why the shared body-preserving frontmatter codec and atomic-write helper were extracted beneath internal/docline and internal/core to break duplication and the import cycle, and the type-alias API-preservation seam'
doc_type: design
docline:
    date: 2026-06-28T00:00:00Z
    status: accepted
    tags:
        - architecture
        - leaf-package
        - import-cycle
        - type-alias
        - frontmatter-codec
        - atomic-write
        - dependency-direction
ingested_at: "2026-06-28T19:58:00Z"
schema_version: "1.0"
source: docs/design-docs/2026-06-28-frontmatter-codec-leaf-packages.md
title: 'Frontmatter codec and atomic-write leaf packages (mdfront, atomicfile)'
---

# Frontmatter Codec & Atomic-Write Leaf Packages

- **Status**: Accepted (shipped 068-S, PR #148, merge `7450271a`)
- **Supersedes**: the 062-F import-cycle workaround that duplicated the codec + atomic-write helper across `internal/docline` and `internal/core`
- **Deliberation**: `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md`
- **Plan**: `docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md`

## Problem

The body-preserving Markdown frontmatter codec and the hardened atomic-write
helper existed as **two private copies** — one in `internal/docline`, one in
`internal/core`. The duplication was a deliberate workaround introduced by 062-F:
having either package import the other would have created an **import cycle**
(`docline` needs the codec; `core` needs the codec and also participates in the
docline-adjacent archive/doctor flow). Two copies meant two test surfaces and a
standing risk of divergence in a byte-sensitive code path.

## Decision

Extract the shared logic **downward** into two new stdlib-only leaf packages and
have both former owners depend on them:

| Package | Responsibility | Imports |
|---|---|---|
| `internal/mdfront` | Body-preserving frontmatter codec: the `Markdown` type with `Decode`/`Encode`, fence handling, YAML round-trip | `bytes`, `fmt`, `gopkg.in/yaml.v3` only |
| `internal/atomicfile` | Hardened `WriteFileAtomic` (temp file + rename, clamped mode) | stdlib only (`fmt`, `io`, `os`, `path/filepath`, `runtime`) |

Both packages import **zero internal packages** — they are leaves. Any number of
higher-level packages can depend on them without forming a cycle, and the cycle
cannot reappear as long as the leaves stay import-free.

### API-preservation seam (type alias)

`internal/docline` keeps its public API byte-for-byte by re-exporting the moved
type with a **true type alias** rather than a re-declaration:

```go
type Markdown = mdfront.Markdown   // alias: same type, inherited methods
```

Because the alias makes `docline.Markdown` and `mdfront.Markdown` the *same*
type, the `(*Markdown).Encode()` method is **inherited** from `mdfront` and
remains callable through `docline` with no wrapper. `Decode` (a package-level
function, not a method) is re-exported as a thin forwarder; `WriteFileAtomic`
forwards to `atomicfile`. `internal/core/doctor.go`'s archived_from repair was
migrated onto the same leaf packages. Every existing caller compiles unchanged.

## Dependency-direction impact

The leaf packages sit at the bottom of the dependency graph, beneath both
`docline` and `core`:

```text
docline → mdfront, atomicfile
core    → mdfront, atomicfile (doctor archived_from repair)
mdfront → (stdlib + yaml only)
atomicfile → (stdlib only)
```

This is recorded as a cross-cutting rule in `docs/ARCHITECTURE.md` (Dependency
Direction): `mdfront` and `atomicfile` are import-free leaves, so the
`docline <-> core` cycle that motivated the original duplication is structurally
prevented.

## Alternatives considered

- **Keep the duplication** (status quo): rejected — two byte-sensitive copies
  drift and double the maintenance/test cost.
- **Make `core` import `docline`** (or vice versa): rejected — reintroduces the
  import cycle 062-F worked around.
- **Named defined type** in `docline` (`type Markdown mdfront.Markdown`):
  rejected — a distinct type does not inherit the source type's methods, so every
  method would have to be re-declared (Go forbids declaring a method on a type
  defined in another package), reintroducing the very duplication being removed.

## Consequences

- **Positive**: single source of truth for the codec and atomic write; smaller,
  faster, focused leaf test suites; the import cycle is structurally impossible;
  `docline`'s public API and `gen-docs` output are unchanged (byte-identical).
- **Neutral/cost**: two new packages to keep import-free; an architecture note +
  (ideally) a future lint rule to assert the leaf property.
- **Verification**: behavior preservation proven by differential golden
  byte-equality tests over the codec and `rewriteArchivedFromField`, a green CLI
  Reference Drift check, a body-preserving docs-migrate dry-run, and live ship
  dogfooding. See `docs/closure/2026-06-28-068-S-codec-extraction-runtime-verification.md`.

## Related

- Learning: `docs/compound/2026-06-28-codec-extraction-leaf-packages.md`
- Contract: `docs/compound/2026-06-26-docline-frontmatter-contract.md`
- Closure: `docs/closure/2026-06-28-068-S-codec-extraction-closure.md`
