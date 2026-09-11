---
title: FL005 Review Remediation Memory
date: 2026-09-11
task: 158.007-T
status: active
---

# FL005 Review Remediation Memory

## Outcome

Remediated the in-scope FL005 review findings while keeping 158.007-T active.
The analyzer now accepts sinks in the claim block or descendant blocks, rejects
sibling and reversed matches, uses type scopes to honor shadowing, and ends a
claim after direct context-variable reassignment.

## Files Modified

* `internal/faultline/analyzer/locktimeout/analyzer.go`
* `internal/faultline/analyzer/locktimeout/locks.go`
* `internal/faultline/analyzer/locktimeout/testdata/src/fl005bad/bad.go`
* `internal/faultline/analyzer/locktimeout/testdata/src/fl005good/good.go`

## Decisions

* Kept analysis syntactic and intra-function using AST ancestry and
  `types.Scope`; no CFG or SSA was introduced
* Removed the generic `Lock` receiver match while retaining declared `sync`
  lock methods and generic `Acquire`
* Bound suppressions to the direct block statement and nearest call, with
  exact preceding-line support
* Added fixtures for descendant, sibling, reversed, reassigned, `WithCancel`,
  custom `Lock`, function literal, suppression, shadowing, and `ValueSpec`
  cases

## Validation

Focused FL005 tests, compile-only checks, both relevant builds, vet, and
FL005-targeted lint passed. Repository-wide tests retain an unrelated newline
byte-stability failure in `internal/faultline`; repository-wide lint and format
checks retain pre-existing findings outside the changed files.

## Failed Approaches

The first suppression fixture was reformatted onto separate lines by `gofmt`.
It was replaced with a formatted `for` clause containing two same-line lock
calls so nearest-call ownership remains directly exercised.

## Next Step

Keep 158.007-T active for the owning review workflow. Do not push this local
commit.
