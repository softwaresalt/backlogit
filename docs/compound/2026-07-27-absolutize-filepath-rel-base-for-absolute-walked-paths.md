---
chunk_strategy: h1-h2-h3
description: 'filepath.Rel(base, target) errors on Windows when base is a relative "." but target is absolute; a directory walk that resolves its root to an absolute path (via SafeResolve) yet keeps the raw relative root as the Rel base fails only under the relative-root caller (MCP server default RootPath=".") and only on Windows. Fix by absolutizing the Rel base with filepath.Abs(root), mirroring the existing ValidateApplyPath idiom.'
doc_type: learning
docline:
    date: 2026-07-27T00:00:00Z
    severity: medium
    tags:
        - filepath-rel
        - filepath-abs
        - windows
        - cross-platform
        - path-handling
        - docline
        - core
        - walkdir
        - mcp
        - tdd
        - relative-root
ingested_at: "2026-07-27T22:00:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-27-absolutize-filepath-rel-base-for-absolute-walked-paths.md
title: 'Absolutize the filepath.Rel base when walked paths are absolute'
---

# Absolutize the filepath.Rel Base When Walked Paths Are Absolute

## Context

`docline.collectInScopeDocs` (feature 127-F, shipment 107-S, stash EF4C0EC6)
resolves its walk root through `core.SafeResolve`, which returns an **absolute**
base, then `filepath.WalkDir`s that absolute base. For each visited file it
computed a repo-relative key with `filepath.Rel(root, p)` — but it passed the
**raw** `root` argument, not the absolutized base, as the `Rel` first operand.

The docline frontmatter gate is reachable two ways: the CLI (`backlogit docs
lint`, which already absolutized its root) and the MCP tool `backlogit_docs_lint`.
The MCP server defaults to `RootPath == "."`, so the bug only manifested through
the MCP path, and only on Windows.

## Problem

`filepath.Rel(base, target)` requires that `base` and `target` are **both**
absolute or **both** relative. When they disagree it returns an error:

```text
docs lint: docline.collectInScopeDocs: walk: Rel: can't make
C:\Source\GitHub\backlogit relative to "."
```

With `root == "."` (relative) but `p` absolute (WalkDir walked the absolute
`SafeResolve` base), `filepath.Rel(".", absPath)` cannot compute a relative path.

The reason it is a **Windows-only** failure is subtle: on POSIX, Go's `Rel`
first `Clean`s both operands and, because there is a single root (`/`), it can
often still produce an answer or a benign result; on Windows, a relative `.` and
an absolute `C:\...` live in incompatible volume/root namespaces, so `Rel` fails
outright. The defect was invisible on the Linux CI runner (where the same gate
runs green) and surfaced only when an operator ran the MCP tool on Windows.

## Root Cause

A **mixed-absoluteness** bug: one half of a path pair was absolutized
(`SafeResolve(root)` → walked base) while the other half (the `Rel` base
operand) kept the caller's raw, possibly-relative value. The invariant
"`filepath.Rel` operands must share absoluteness" was silently violated for the
relative-root caller.

## Solution

Absolutize the `Rel` base to the same absolute root the walk uses, then compute
the relative key against it:

```go
absRoot, err := filepath.Abs(root)
if err != nil {
    return nil, fmt.Errorf("docline.collectInScopeDocs: resolve root: %w", err)
}
// ... inside the WalkDir callback:
rel, err := filepath.Rel(absRoot, p)
```

`filepath.Abs` is a no-op for already-absolute inputs (`Abs(abs) == abs`), so the
change is **byte-for-byte identical** for every absolute-root caller (the CLI)
and only fixes the relative-root caller (MCP `RootPath="."`). This mirrors the
package's existing `ValidateApplyPath` idiom, which already absolutizes before
relativizing — the fix restores local consistency with an established pattern
rather than inventing a new one.

## Verification

Test-first (P-002), reproduced live before fixing:

* **RED** — a parallel-safe test seeds `docs/decisions/seed.md` in a `t.TempDir`,
  derives a **relative** root via `filepath.Rel(cwd, dir)`, and calls
  `collectInScopeDocs(rel, "")`. It failed with the exact `Rel` error above
  (temp on the same volume as cwd, so a genuine failure, not a cross-volume
  skip).
* **GREEN** — after absolutizing the base the same test passes.
* **Runtime** — on the real Windows repo, both `backlogit docs lint` (CLI) and
  the MCP `backlogit_docs_lint` tool driven with `RootPath="."` returned
  `valid: true, 0 violations` with no `Rel` error.

## Pitfalls

* **Do not skip the RED phase with a cross-volume `t.Skipf`.** `filepath.Rel`
  also fails when `base` and `target` are on **different Windows volumes** — a
  legitimately unrelatable case. Guard only that (skip when the seed temp dir is
  on a different drive than cwd) so the test still exercises the real defect on
  same-volume runs   instead of masking it as an environmental skip.
* **Absoluteness must be checked at the operand pair, not per call site.** Grep
  the whole package for every `filepath.Rel(` to confirm there is a single base
  source of truth; a second unabsolutized call site would reintroduce the defect
  for a different caller.
* **Linux CI will not catch this.** A cross-platform path bug that only fails on
  Windows needs a Windows-representative test (relative-vs-absolute operands),
  because the Linux gate stays green.

## Applicability

Reuse whenever code **walks an absolutized root** but later calls
`filepath.Rel(root, p)` (or `filepath.Rel(p, root)`) with a value that a caller
may pass relative. The durable rule: **before `filepath.Rel`, ensure both
operands share absoluteness — absolutize the base with `filepath.Abs` to match
absolute walked paths (or make both relative).** Prefer matching whatever
absoluteness the surrounding walk/resolution already established, and look for a
sibling idiom in the same package (here `ValidateApplyPath`) to stay consistent.

## Evidence

* Fix: `internal/docline/service.go` `collectInScopeDocs` — `filepath.Abs(root)`
  block after `core.SafeResolve`, WalkDir callback uses `filepath.Rel(absRoot, p)`.
  Reference idiom: `ValidateApplyPath` in the same file.
* Test: `internal/docline/service_test.go`
  `TestCollectInScopeDocs_RelativeRootDoesNotErrorOnRel`.
* Plan: `docs/exec-plans/2026-07-26-docline-collectinscopedocs-relative-root-rel-fix-plan.md`.
* Tasks 127.001-T (RED) / 127.002-T (GREEN), feature 127-F, shipment 107-S,
  stash EF4C0EC6 — PR #303, merge commit 8a757d5e.
