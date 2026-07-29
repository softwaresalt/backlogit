---
chunk_strategy: h1-h2-h3
description: 'Injectable test seam patterns for durable write paths: package-level function variables for write and handler injection, path-selective fsync mocks to avoid breaking creation assertions, and t.Parallel prohibition for package-global seams.'
doc_type: learning
docline:
    date: 2026-07-29T00:00:00Z
    severity: medium
    tags:
        - testing
        - test-seam
        - durable-writes
        - fsync
        - mock
        - injectable
        - tdd
        - package-global
ingested_at: "2026-07-29T01:30:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-29-durable-writes-test-seam-patterns.md
title: 'durable_writes: injectable test seam patterns for write-path and handler injection'
---

## Problem

The durable write paths in `internal/core` and `internal/mcp` call unexported
functions and package-level singletons. The `mkdirDirSyncFn` seam exists for the
mkdir path but does NOT fire on the `relocate=false` path through
`WriteArtifactFileWithOptions`. The MCP `handleAppendComment` calls
`core.AppendComment` directly. Neither path is injectable without a seam.

## The persistArtifactWriteFn seam (core/shipment.go)

`persistArtifactWriteFn` is a package-level `var` in `internal/core/shipment.go`
that wraps the sole call to `WriteArtifactFileWithOptions` in `persistArtifact`.
Default value: the real implementation. In tests, override it to inject a
controlled error (e.g. `blerrors.ErrWriteIndeterminate`) without touching the
`mkdirDirSyncFn` or OS primitives.

```go
// In production code (shipment.go):
var persistArtifactWriteFn = WriteArtifactFileWithOptions
// Actual signature: func(artifact *models.Artifact, filePath string, durable bool) error

// In tests:
originalFn := persistArtifactWriteFn
persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
    return blerrors.ErrWriteIndeterminate
}
t.Cleanup(func() { persistArtifactWriteFn = originalFn })
```

This seam pattern is correct for `relocate=false` callers (`AddDependency`,
`RemoveDependency`) because the `mkdirDirSyncFn` seam only fires for the
`mkdirAllDurable` path, which is not invoked when the target directory already
exists (the common case for dependency updates on existing artifacts).

## The appendCommentFn seam (mcp/tools.go)

`appendCommentFn` is a package-level `var` in `internal/mcp/tools.go` that wraps
the call to `core.AppendComment` inside `handleAppendComment`. Default value: the
real implementation. In tests, override it to inject specific durability errors.

```go
// In production code (tools.go):
var appendCommentFn = core.AppendComment
// Actual signature: func(ctx context.Context, ws *core.Workspace, ew *events.EventWriter,
//   itemID, actor, comment, commitSHA string) error

// In tests:
originalFn := appendCommentFn
appendCommentFn = func(ctx context.Context, ws *core.Workspace, ew *events.EventWriter,
    itemID, actor, comment, commitSHA string) error {
    return blerrors.ErrWriteIndeterminate
}
t.Cleanup(func() { appendCommentFn = originalFn })
```

The seam is necessary because `core.AppendComment` routes through
`appendDurable`/`mkdirAllDurable` in `internal/events`, which are unexported and
not directly injectable from the `mcp` package.

## Path-selective fsync mocks (required for mkdirDirSyncFn)

When testing `mkdirAllDurable` retry behavior, a naïve "fail all paths" mock
breaks the test because the pre-creation ancestor re-confirm (added in 130-F)
calls `mkdirDirSyncFn` for `filepath.Dir(cur)` — the parent of the first existing
ancestor — before any dirs are created. If that call fails, `mkdirAllDurable`
returns `ErrWriteNotApplied` immediately and the target directory is never
created, causing the test assertion "dir exists" to fail.

**Pattern:** mock `mkdirDirSyncFn` with a path-selective function that only fails
for the specific path the test is verifying, leaving all other paths succeed:

```go
originalFn := mkdirDirSyncFn
mkdirDirSyncFn = func(p string) error {
    if p == targetParentPath { // only fail the specific path under test
        return errors.New("injected fsync failure")
    }
    return nil
}
t.Cleanup(func() { mkdirDirSyncFn = originalFn })
```

Similarly, `TestAppendEvent_DirFsyncFailureIsIndeterminate` must fail only
`p == logsDir` (the post-write dir) and NOT `p == filepath.Dir(logsDir)` (the
pre-write re-confirm path added by U3/Finding-1). Failing both makes the
test indeterminate about which call produced the error.

## t.Parallel prohibition for package-global seams

All three seams (`mkdirDirSyncFn`, `persistArtifactWriteFn`, `appendCommentFn`)
are package-level globals on the production write path. Tests that override them
MUST NOT call `t.Parallel()`. Parallel test execution with shared package globals
causes races: one test's `t.Cleanup` restore races with another test's override,
producing flaky failures that are hard to reproduce.

This constraint applies to ALL tests in the same package that touch the same seam,
not just the test that overrides it — the race can occur even if only one test
modifies the seam if the test runner happens to schedule another test in the same
package concurrently.

## Citations

- `internal/core/shipment.go`: `persistArtifactWriteFn` seam (feature 130-F, PR #315)
- `internal/mcp/tools.go`: `appendCommentFn` seam (feature 130-F, PR #315)
- `internal/core/durable_fs.go`: `mkdirDirSyncFn` seam (feature 123-F, PR #308)
- `internal/core/dependencies_indeterminate_test.go`: path-selective mock usage
- `internal/mcp/append_comment_durable_test.go`: `appendCommentFn` override pattern
- `internal/events/stream_durable_test.go`: post-write selective mock pattern
