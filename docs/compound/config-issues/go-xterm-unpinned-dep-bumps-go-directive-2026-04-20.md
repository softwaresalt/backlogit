---
title: "Unpinned golang.org/x/term Transitively Bumps go Directive via x/sys"
description: "Adding golang.org/x/term without pinning x/sys can silently bump the go directive in go.mod to a newer version, breaking CI matrix enforcement."
problem_type: config_issue
category: config_issue
component: config
root_cause: schema_mismatch
resolution_type: dependency_update
severity: high
message: "go get golang.org/x/term (unpinned) pulls x/sys@v0.43.0 which declares go 1.25.0, propagating the directive into the main module"
file_path: "go.mod"
resolved: true
tags: [go.mod, go-directive, dependency-pinning, golang.org/x/term, golang.org/x/sys, ci-failure, version-bump, transitive-dependency]
date: 2026-04-20
---

## Problem

Running `go get golang.org/x/term` without a version pin fetches the latest
release (v0.42.0 at time of incident). That version's transitive dependency
`golang.org/x/sys@v0.43.0` declares `go 1.25.0` in its own go.mod. Go's module
system propagates the highest go directive from any transitive dependency into
the main module's go.mod, bumping `go 1.24.0` to `go 1.25.0`. The CI pipeline
then fails because workflow version matrices and the `TestWorkflowGoVersionMatchesMod`
integration test enforce strict alignment between go.mod and workflow matrices.

## Symptoms

* `go.mod` go directive changes from `go 1.24.0` to `go 1.25.0` after `go get`.
* `TestWorkflowGoVersionMatchesMod` fails: `ci.yml matrix does not include 1.25`.
* CI workflow matrices (`["1.23","1.24"]`) no longer include the required version.
* No explicit error from `go get` or `go mod tidy` — the bump is silent.

## What Did Not Work

* **Pinning only x/term**: `go get golang.org/x/term@v0.38.0` without also
  pinning x/sys left x/sys at v0.43.0, which still declared `go 1.25.0`. The
  directive remained bumped.
* **Running `go mod tidy` after the bump**: `go mod tidy` never lowers a go
  directive. Once raised, it stays raised regardless of what versions are
  resolved. Manual intervention is required.
* **Updating only workflow files**: Changing CI matrices to include `"1.25"`
  masks the problem instead of fixing it and creates a real toolchain drift.

## Solution

Pin both the direct dependency and the problematic indirect dependency
explicitly, then manually reset the go directive.

### Before

```text
// go.mod (after unpinned go get)
go 1.25.0

require (
    golang.org/x/term v0.42.0
)

require (
    golang.org/x/sys v0.43.0 // indirect
)
```

### After

```bash
# Step 1: pin direct dep to a version compatible with go 1.24
go get golang.org/x/term@v0.38.0

# Step 2: pin the indirect dep that carried the directive bump
go get golang.org/x/sys@v0.39.0

# Step 3: manually edit go.mod — change go 1.25.0 back to go 1.24.0
# (go mod tidy will NOT do this for you)

# Step 4: verify everything resolves cleanly
go mod tidy
go test ./...
```

```text
// go.mod (after fix)
go 1.24.0

require (
    golang.org/x/term v0.38.0
)

require (
    golang.org/x/sys v0.39.0 // indirect
)
```

## Why This Works

Go's module system propagates the highest `go` directive seen across the full
transitive closure of dependencies. `x/sys@v0.43.0` declares `go 1.25.0`,
which causes it to propagate. Pinning x/sys to `v0.39.0` (which declares
`go 1.18`) removes that propagation source. With no transitive dep declaring
1.25.0, the module is free to stay at 1.24.0 — but only after a manual reset,
since `go mod tidy` strictly never lowers the directive.

The key insight: **always pin both the direct dep AND its x/sys sibling**.
All `golang.org/x/...` packages are tightly coupled to `x/sys`. Any new major
version of x/sys tends to raise the go directive.

## Prevention

* When adding any `golang.org/x/...` package, pin both the package and x/sys
  in the same command:

  ```bash
  go get golang.org/x/term@v0.38.0 golang.org/x/sys@v0.39.0
  ```

* After any `go get`, diff go.mod before committing:

  ```bash
  git diff go.mod | Select-String "^[+-]go\s"
  ```

  Any change to the `go X.Y.Z` line is a red flag requiring investigation.

* The integration test `TestWorkflowGoVersionMatchesMod` in
  `tests/integration/ci_compliance_test.go` catches this automatically — run it
  locally after any dependency update that touches `golang.org/x/...`.

* Do not run `go get` without a version pin in a repository that enforces a
  strict `go` directive. Treat unpinned `go get` the same as `npm install
  latest` — it may silently upgrade your toolchain requirement.

## Related Solutions

* `docs/compound/github-actions/F013-workflow-sha-pinning.md` — related CI
  workflow and Go version matrix alignment patterns (high relevance).
* `docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md` — another silent CI gap that bypassed the PR cycle.
