<!-- markdownlint-disable-file -->
# PR Review Handoff: 013-release-pipeline-fix

## PR Overview

**F013: Release Pipeline Fix** resolves four categories of CI/CD compliance failures in the backlogit GitHub Actions workflows, validated by a new integration test harness.

This PR corrects security and reliability issues in `.github/workflows/ci.yml` and `.github/workflows/release.yml` and introduces a comprehensive compliance test suite (`tests/integration/ci_compliance_test.go`) that characterizes and enforces all fixes.

* Branch: `013-release-pipeline-fix`
* Base Branch: `main`
* Head Commit: `a875a20` (fix(workflows): address PR review findings for F013)
* Total Files Changed: 3 in follow-up (5 overall against main)
* Total Review Rounds: 2 (original 10 findings → follow-up refresh)
* Review Artifacts:
  * `.backlogit/queue/F013.R001-branch-review.md` (original)
  * `.backlogit/queue/F013.R002-followup-review.md` (follow-up)
* Backlogit Work Item: F013
* Validation: `go test ./...` ✅ | `go vet ./...` ✅

---

## Changes Summary

### `.github/workflows/ci.yml`

| Change | Before | After |
|--------|--------|-------|
| Go version matrix | `["1.22", "1.23"]` | `["1.23", "1.24"]` |
| `actions/checkout` | `@v4` (tag) | `@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2` |
| `actions/setup-go` | `@v5` (tag) | `@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0` |
| `golangci-lint-action` | `@v6` / `version: latest` | `@2226d7cb...# v6.5.0` / `version: v1.64.8` |
| `persist-credentials` | absent | `false` on all checkouts |
| `concurrency:` block | absent | `group: ${{ github.workflow }}-${{ github.ref }}` / `cancel-in-progress: true` |

### `.github/workflows/release.yml`

| Change | Before | After |
|--------|--------|-------|
| Tag trigger | `"v[0-9]+.[0-9]+.[0-9]+*"` (regex) | `"v*.*.*"` (glob) |
| Go matrix | `["1.22", "1.23"]` | `["1.23", "1.24"]` |
| Build job Go version | `"1.22"` | `"1.24"` |
| All action refs | tag-pinned | 40-char SHA + version comment |
| `persist-credentials` | absent | `false` on all 3 checkouts |
| `permissions: contents: write` | workflow-level (over-privileged) | `release` job only (least-privilege) |
| `golangci-lint version:` | `latest` | `v1.64.8` |
| `concurrency:` block | absent | `group: ${{ github.workflow }}-${{ github.ref }}` / `cancel-in-progress: false` |

### `tests/integration/ci_compliance_test.go`

New 302-line compliance test harness with 5 test functions:

| Function | What it validates |
|----------|------------------|
| `TestReleaseTagTriggerUsesGlob` | Tag trigger uses glob syntax, not regex character classes |
| `TestWorkflowGoVersionMatchesMod` | CI/release matrices include the `go.mod`-declared version |
| `TestAllActionsUseSHAPins` | Every third-party action uses a 40-char SHA pin |
| `TestCheckoutStepsNoPersistCredentials` | All checkout steps set `persist-credentials: false` |
| `TestWorkflowsDeclareConcurrency` | Both workflows declare standard `concurrency:` blocks |
| `TestGolangciLintVersionPinned` | `golangci-lint-action` uses a pinned `v{major}.{minor}.{patch}` version |

---

## Review Decision Log

### Original 10 Findings — All Resolved ✅

| ID | Severity | Title | Resolution |
|----|----------|-------|------------|
| F-001 | P1 🔒 | Write permission at workflow scope | `contents: write` scoped to `release` job only |
| F-002 | P1 ⚠️ | SHA pin test missing `docker://` skip | `docker://` guard added to `TestAllActionsUseSHAPins` |
| F-003 | P2 | `scanner.Err()` unchecked | `require.NoError(t, scanner.Err(), ...)` added |
| F-004 | P2 | Missing `concurrency:` in `ci.yml` | Block added with `cancel-in-progress: true` |
| F-005 | P2 | Missing `concurrency:` in `release.yml` | Block added with `cancel-in-progress: false` |
| F-006 | P2 | `golangci-lint version:latest` in `ci.yml` | Pinned to `v1.64.8` |
| F-007 | P2 | `golangci-lint version:latest` in `release.yml` | Pinned to `v1.64.8` |
| F-008 | P2 | Tag glob broader than regex intent | Accepted by design — GitHub glob requirement |
| F-009 | P3 | `map[string]interface{}` | Replaced with `map[string]any` |
| F-010 | P3 | `assert.Equal(false, ...)` | Replaced with `assert.False(t, ...)` |

### Follow-up Review Findings — All Advisory / Deferred

| ID | Severity | Title | Decision |
|----|----------|-------|----------|
| R-001 | P2 | `fmt.Sprintf("%v")` for YAML `any` values | Deferred → stash item `2ED12470` |
| R-002 | P3 | Doc comments on test helpers | Deferred (cosmetic) |
| R-003 | P3 | `versionPattern` at package level | Deferred (cosmetic) |
| R-004 | P3 | `strings.Contains` precision | Deferred (cosmetic) |
| R-005 | P3 | `t.Parallel()` opportunity | Deferred (future pass) |
| CR-001 | — | Missing checkout in release job | ❌ False positive — publish-only job |

---

## PR Comments Ready for Submission

No blocking comments to post. All P0/P1 findings are resolved. The advisory P2 finding (R-001) is deferred into the stash for a future workstream.

**Optional informational note (R-001) — low urgency, defer to follow-up PR:**

### File: `tests/integration/ci_compliance_test.go`

#### Comment (Lines ~57, ~86, ~110)

* Category: Code Quality
* Severity: P2 (Advisory — no block)

`step.With["go-version"]` and `step.With["version"]` are read from `map[string]any` and converted with `fmt.Sprintf("%v", val)`. This works for the current YAML files (all values are quoted strings), but bypasses type safety. Consider explicit type assertions in a follow-up:

```go
if s, ok := step.With["go-version"].(string); ok {
    // use s directly
}
```

**Deferred stash item:** `2ED12470`

---

## Review Summary by Category

| Category | Original Findings | Resolved | Deferred |
|----------|-------------------|----------|----------|
| 🔒 Security (permissions) | 1 (F-001 P1) | ✅ 1 | 0 |
| 🔒 Security (SHA pinning) | 1 (F-002 P1) | ✅ 1 | 0 |
| ⚠️ Reliability (concurrency) | 2 (F-004, F-005 P2) | ✅ 2 | 0 |
| ⚠️ Reliability (scanner.Err) | 1 (F-003 P2) | ✅ 1 | 0 |
| 🔍 Determinism (lint version) | 2 (F-006, F-007 P2) | ✅ 2 | 0 |
| 🔍 Correctness (tag trigger) | 1 (F-008 P2) | ✅ 1 | 0 |
| 💡 Code quality (Go idioms) | 2 (F-009, F-010 P3) | ✅ 2 | 0 |
| 💡 Type safety (follow-up) | 1 (R-001 P2) | — | Stash item `2ED12470` |
| 💡 Cosmetic (follow-up) | 4 (R-002–R-005 P3) | — | Deferred |

---

## Review Artifacts

* Original review: `.backlogit/queue/F013.R001-branch-review.md`
* Follow-up review: `.backlogit/queue/F013.R002-followup-review.md`
* In-progress tracking: `.copilot-tracking/pr/review/013-release-pipeline-fix/in-progress-review.md`
* Compound learnings: `docs/compound/github-actions/F013-workflow-sha-pinning.md`
* Memory checkpoint: `docs/memory/2026-04-04/F013-complete-checkpoint.md`
* Compound artifacts committed: yes
* Memory checkpoints committed: yes
* Deferred stash items: `2ED12470`

---

## PR Readiness Gate

| Check | Status |
|-------|--------|
| P0 findings unresolved | ✅ None |
| P1 findings unresolved | ✅ None |
| `go test ./...` passing | ✅ Confirmed |
| `go vet ./...` passing | ✅ Confirmed |
| SHA pins on all actions | ✅ Verified |
| `persist-credentials: false` everywhere | ✅ Verified |
| `concurrency:` blocks present | ✅ Verified |
| `golangci-lint` version pinned | ✅ Verified |
| `contents: write` scoped to release job | ✅ Verified |
| Compound artifacts on branch | ✅ Committed |

**🟢 PR READY — No blocking issues. Safe to create PR against `main`.**

---

## Suggested PR Title

```
fix(ci): F013 — release pipeline security and reliability hardening
```

## Suggested PR Body

```markdown
## Summary

Corrects four categories of CI/CD compliance failures in GitHub Actions workflows and introduces a characterization test harness to enforce compliance going forward.

## Changes

### Security
- Move `contents: write` from workflow scope to `release` job only (least-privilege)
- Pin all 8 third-party action references to 40-char commit SHAs
- Add `persist-credentials: false` to all `actions/checkout` steps

### Reliability  
- Add `concurrency:` blocks to both workflows (CI: `cancel-in-progress: true`, Release: `cancel-in-progress: false`)
- Fix non-deterministic `golangci-lint version: latest` → `v1.64.8`
- Fix Go version matrix from `[1.22, 1.23]` → `[1.23, 1.24]`

### Correctness
- Replace regex tag filter `v[0-9]+.[0-9]+.[0-9]+*` with GitHub glob `v*.*.*`

### Tests
- Add `tests/integration/ci_compliance_test.go` with 6 test functions covering all fixes
- Promote `gopkg.in/yaml.v3` and `github.com/go-playground/validator/v10` to direct dependencies

## Validation

- `go test ./...` ✅
- `go vet ./...` ✅

## Linked Work Item

F013
```
