# Compound: GitHub Actions Workflow SHA Pinning (F013)

**Date**: 2026-04-04
**Feature**: F013 — Release Pipeline Fix
**Branch**: 013-release-pipeline-fix
**Attempts**: 1 (first-pass success, all 4 tests green on initial implementation)

## Problem

GitHub Actions workflows in `.github/workflows/ci.yml` and `.github/workflows/release.yml` had three compliance defects:

1. Tag trigger used regex syntax `v[0-9]+.[0-9]+.[0-9]+*` instead of GitHub Actions glob `v*.*.*`
2. Go version matrix `["1.22", "1.23"]` did not match `go.mod` requirement of `go 1.24.0`
3. All third-party actions used version tags (`@v4`, `@v5`) instead of full 40-char SHAs

## Solution Pattern

### 1. Tag Trigger Fix
```yaml
# Before (broken — regex character class, GitHub doesn't support this)
- "v[0-9]+.[0-9]+.[0-9]+*"

# After (correct — GitHub Actions glob syntax)
- "v*.*.*"
```

### 2. Go Version Matrix
```yaml
# Before
go-version: ["1.22", "1.23"]

# After — drop oldest, include go.mod version
go-version: ["1.23", "1.24"]
```

Also update the build job `go-version` (not a matrix) from hardcoded `"1.22"` to `"1.24"`.

### 3. SHA Pinning Pattern
```yaml
# Before (forbidden by workflows.instructions.md)
uses: actions/checkout@v4

# After (required pattern)
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
with:
  persist-credentials: false
```

## SHA Lookup Method

Use the GitHub API to get the commit SHA behind a tag:

```powershell
$url = "https://api.github.com/repos/actions/checkout/git/refs/tags/v4.2.2"
$response = Invoke-RestMethod -Uri $url -Headers @{ "Accept" = "application/vnd.github.v3+json"; "User-Agent" = "script" }
$sha = $response.object.sha
# If response.object.type == "tag" (annotated tag), dereference:
if ($response.object.type -eq "tag") {
    $tagObj = Invoke-RestMethod -Uri $response.object.url -Headers $headers
    $sha = $tagObj.object.sha
}
```

## Resolved SHAs (as of 2026-04-04)

| Action | Tag | SHA |
|--------|-----|-----|
| actions/checkout | v4.2.2 | `11bd71901bbe5b1630ceea73d27597364c9af683` |
| actions/setup-go | v5.4.0 | `0aaccfd150d50ccaeb58ebd88d36e91967a5f35b` |
| golangci/golangci-lint-action | v6.5.0 | `2226d7cb06a077cd73e56eedd38eecad18e5d837` |
| taiki-e/install-action | v2.49.0 | `e03236526ace47fa2e04bebcfc6da471ebd4690c` |
| actions/upload-artifact | v4.6.2 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |
| actions/download-artifact | v4.2.1 | `95815c38cf2ff2164869cbab79da8d1f422bc89e` |
| softprops/action-gh-release | v2.2.1 | `c95fe1489396fe8a9eb87c0abf8aa5b2ef267fda` |

> ⚠️ SHAs are pinned to specific versions. Re-run the SHA lookup when upgrading action versions.

## Test Strategy: Characterization-First

For YAML workflow infrastructure (no Go production code), use the characterization-first posture:

1. Write integration tests in `tests/integration/` that parse and validate the YAML files
2. Tests fail against the broken state (RED)
3. Edit the YAML files to fix the violations
4. Tests pass (GREEN)
5. No Go stubs needed — the "implementation" is purely YAML edits

Key test helpers:
- `readCIWorkflow(t, path)` — parses YAML into `ciWorkflow` struct
- `findRepoRoot(t)` — walks up to find `go.mod` for portability

## YAML Parsing Gotcha

When `persist-credentials: false` is in a YAML `with:` block and parsed by `gopkg.in/yaml.v3`
into `map[string]interface{}`, the value is `bool(false)` (not string). Tests must use:

```go
assert.Equal(t, false, persistCreds)  // bool comparison, not string "false"
```

## Dependency Ordering

For multi-step research tasks that unblock implementation tasks, wire the dependency in
backlogit before starting the build. The ordering is:
1. Research task (SHA lookup) — mark done → unblocks dependent implementation tasks
2. Implementation tasks (apply SHAs to each workflow file) — run after research task

This prevents attempting to apply unknown SHAs and gives the operator visibility into the
research results before the implementation proceeds.
