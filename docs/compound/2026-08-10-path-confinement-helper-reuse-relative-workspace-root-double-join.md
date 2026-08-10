---
chunk_strategy: h1-h2-h3
description: "When reusing a path-confinement helper originally written for one calling convention (a path relative to workspace root), passing it an already-prefixed absolute-looking-but-actually-relative path silently double-joins the workspace root onto itself, corrupting the path and causing every relative-cwd invocation to fail containment. Discovered during 122-S runtime verification: internal/core/checkpoint_target.go built checkpointsDirAbs via filepath.Join(WorkspaceStorageRoot(ws.RootPath), checkpointsSubdir) -- a variable named *Abs that stays relative whenever ws.RootPath itself is relative (the normal case for a CLI invoked with a relative --cwd) -- then passed it into the existing confineToStorageRoot helper (doctor_target.go), which re-joins any non-absolute input onto ws.RootPath. Unit tests never caught this because every test workspace used t.TempDir(), which always returns an absolute path."
doc_type: learning
docline:
    date: 2026-08-10T00:00:00Z
    severity: high
    tags:
        - path-confinement
        - workspace-root
        - relative-path
        - runtime-verification
        - checkpoint-disposition
        - go
schema_version: "1.0"
source: docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md
title: "Reusing a path-confinement helper across calling conventions silently double-joins a relative workspace root"
---

# Path-Confinement Helper Reuse: Relative Workspace Root Double-Join

## Context

Surfaced during **122-S** (checkpoint administrative disposition, PR #342)
post-merge runtime verification. `internal/core/checkpoint_target.go`'s
`ResolveDispositionTarget` was written as "a thin adapter over the existing
`confineToStorageRoot` containment primitive" (`doctor_target.go`) to avoid
re-implementing path-escape defenses — a reasonable, DRY design choice.

`confineToStorageRoot`'s original calling convention (from its one existing
caller, `doctor.go`'s single-file validation path) is: pass a path that is
**either absolute, or relative to `ws.RootPath`**. When given a non-absolute
input, it joins that input onto `ws.RootPath` itself:

```go
target := filePath
if !filepath.IsAbs(target) {
    target = filepath.Join(ws.RootPath, filePath)
}
```

`ResolveDispositionTarget` built its candidate path like this:

```go
checkpointsDirAbs := filepath.Join(WorkspaceStorageRoot(ws.RootPath), checkpointsSubdir)
absCandidate := filepath.Join(checkpointsDirAbs, filename)
absTarget, inScope, err := confineToStorageRoot(ws, absCandidate)
```

`WorkspaceStorageRoot(ws.RootPath)` returns `filepath.Join(ws.RootPath,
".backlogit")` — it **already includes `ws.RootPath` as a prefix**. The local
variable name `checkpointsDirAbs` claims this is an absolute path, but
`filepath.Join` never makes anything absolute; if `ws.RootPath` itself is
relative (the normal case for a CLI invoked with `--cwd .` or any relative
directory), `checkpointsDirAbs` stays relative too. Passed into
`confineToStorageRoot`, the `!filepath.IsAbs(target)` branch fires and
re-joins `ws.RootPath` onto a value that already starts with `ws.RootPath`,
producing a corrupted, nonexistent path like
`<root>/<root>/.backlogit/checkpoints/<file>`. Containment against the real
`.backlogit` directory then always fails with "target escapes checkpoints
directory" — **every** `AbandonCheckpoint`/`QuarantineCheckpoint` call (and
by extension every CLI `checkpoint abandon`/`checkpoint quarantine`
invocation and their MCP tool equivalents) with a relative workspace root was
broken.

## Why Unit Tests Missed It

Every test workspace in `internal/core/checkpoint_target_test.go` and
`checkpoint_disposition_test.go` was built via `t.TempDir()`, which always
returns an **absolute** path on every OS. `ws.RootPath` was therefore always
absolute in every test, `checkpointsDirAbs` was therefore always genuinely
absolute despite the misleading construction, and the buggy re-join branch
inside `confineToStorageRoot` was never exercised. 100% unit/integration test
pass rate coexisted with a completely broken feature for the CLI's normal,
common invocation pattern (a relative `--cwd`). Only a runtime-verification
pass that deliberately exercised the compiled binary with a relative `--cwd`
value caught it.

## Rule

1. **Never trust a variable name to guarantee a path property.** A name like
   `xDirAbs` is a claim, not a proof; if it is built with `filepath.Join`
   alone and the join's first argument might itself be relative, the result
   inherits that relativity. Normalize explicitly with `filepath.Abs(...)`
   whenever a downstream consumer's correctness depends on absoluteness.
2. **Before reusing an existing path-confinement/containment helper for a new
   caller, read its exact calling convention** (what shape of input does it
   expect: absolute-or-relative-to-root? already-absolute-only?
   already-prefixed?) rather than assuming "it does the same kind of check I
   need" is sufficient. A helper that re-joins non-absolute inputs onto a
   root is unsafe to call with a value that already contains that root as a
   prefix.
3. **Unit test workspace roots that are always absolute (`t.TempDir()`)
   cannot exercise relative-workspace-root code paths.** When a function's
   contract explicitly needs to work with a relative `ws.RootPath` (as CLI
   `--cwd` flags commonly are), add at least one test that constructs the
   workspace with a genuinely relative root (e.g. `os.Chdir` into a parent
   temp dir, then pass a relative subdirectory name) — mirroring
   `TestResolveDispositionTarget_WorksWithRelativeWorkspaceRoot`, added as
   the regression test for this bug.
4. **Runtime verification of a compiled binary against real CLI flags is not
   redundant with `go test ./...`.** This bug had zero unit-test signal and
   100% green CI, yet broke the feature's primary intended usage. Building
   the actual binary and invoking it with the actual flag shapes real
   operators will use (including relative paths) is a distinct verification
   layer that unit tests running via `go test` structurally cannot replace.

## Applicability

Any Go codebase with a path-confinement or containment helper originally
written for one caller's calling convention (absolute-or-root-relative input)
that gets reused by a second caller which independently builds a path that
already includes the workspace root as a prefix. The generalized reflex:
whenever composing a path from `ws.RootPath` and then handing it to a
function that itself also knows about `ws.RootPath`, verify explicitly
whether that function expects an already-rooted path or a bare
root-relative path — and add `filepath.Abs` if there is any doubt, plus a
test using a relative workspace root to prove it.
