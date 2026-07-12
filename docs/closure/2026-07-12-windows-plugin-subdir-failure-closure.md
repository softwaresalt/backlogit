---
chunk_strategy: h1-h2-h3
description: Closure record for documenting Windows failure of the Copilot plugin subdirectory install form.
doc_type: closure
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-12-windows-plugin-subdir-failure-closure.md
title: Windows plugin subdirectory install failure closure
---

## Summary

Feature `098-F` recorded operator field data that the subdirectory plugin install
form is not a Windows workaround:

```bash
copilot plugin install softwaresalt/backlogit:plugin
```

On Windows, that command failed with `Failed to install plugin: Error: The
directory name is invalid. (os error 267)`. The colon in the derived installed
plugin directory is invalid on Windows, so the plain owner/repo install command
is the required cross-platform path:

```bash
copilot plugin install softwaresalt/backlogit
```

## Shipped changes

* Kept README, plugin guide, and installation docs focused on the plain
  `copilot plugin install softwaresalt/backlogit` command
* Removed local-dev wording that distracted from the canonical owner/repo path
* Updated `docs/closure/2026-07-12-plugin-install-path-fix-closure.md` with the
  Windows `os error 267` field result
* Added integration drift guards that:
  * require active docs to contain the plain owner/repo install command
  * reject active-doc recommendations of the colon subdirectory form
  * normalize Markdown whitespace so wrapped commands are still checked
  * require the closure record to preserve the Windows failure evidence

## Verification

* TDD red: `go test ./tests/integration -run "TestActivePluginDocsKeepPlainOwnerRepoInstallCanonical|TestPluginClosureRecordsWindowsSubdirInstallFailure"` failed before the closure update because the closure record did not mention Windows, `os error 267`, or `The directory name is invalid`
* TDD green: the same targeted test passed after the update
* `go run ./cmd/backlogit docs lint`
* `go build ./cmd/backlogit`
* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `gofmt -l tests/integration/plugin_manifest_test.go`
* CI for PR #220 passed: Detect code changes, Docline frontmatter gate, CLI
  Reference Drift, and test

## Review and PR

* Local review readiness: no in-scope P0/P1 findings
* One scope-review response reported unrelated shipment-gate files that were not
  present in `git diff --name-only`; the finding was rejected as a false
  positive after diff verification
* Copilot review on PR #220 completed for head
  `7fc7fa5515928e8b38915fa7f3cdb9435f744cc6`
* Copilot requested a stronger active-doc drift guard for `docs/rationale.md`
  and wrapped commands; fixed in `7fc7fa5` and resolved
* PR #220 merged normally with merge commit
  `bd8331e4f4a386b25dd12d2485992ea6f979fe75`

## Operator note

Live `copilot plugin install` remains operator-manual because it writes outside
the repository workspace. The operator already verified the Windows failure for
the colon subdirectory command. Future validation should use the plain
owner/repo command.
