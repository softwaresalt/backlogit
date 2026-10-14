---
title: FL005 Review Remediation Memory
date: 2026-09-11
task: 158.007-T
status: active
---

# FL005 Review Remediation Memory

## Outcome

Remediated the final in-scope FL005 P1 while keeping 158.007-T active. The
analyzer now treats every real selector method named `Lock` or `Acquire` as a
sink when its explicit parameters do not accept `context.Context`. Plain
functions, function fields, and context-taking methods remain excluded.

## Files Modified

* `internal/faultline/analyzer/locktimeout/analyzer.go`
* `internal/faultline/analyzer/locktimeout/locks.go`
* `internal/faultline/analyzer/locktimeout/testdata/src/fl005bad/bad.go`
* `internal/faultline/analyzer/locktimeout/testdata/src/fl005good/good.go`

## Decisions

* Kept analysis syntactic and intra-function using AST ancestry and
  `types.Scope`; no CFG or SSA was introduced
* Made `Lock` and `Acquire` receiver-agnostic while requiring the selector to
  resolve to a method with a receiver
* Kept `RLock` restricted to `sync.RWMutex`
* Bound suppressions to the direct block statement and nearest call, with
  exact preceding-line support
* Added fixtures for descendant, sibling, reversed, reassigned, `WithCancel`,
  custom zero-context `Lock`, context-taking methods, non-method callables,
  function literals, suppression, shadowing, and `ValueSpec` cases

## Validation

The focused FL005 tests, scoped vet and lint, changed-file format check, and
`faultline-analyze` build passed.

## Failed Approaches

The final P1 red run correctly exposed both prior gaps: custom `Lock()` was
missed and a function field named `Acquire` was diagnosed as though it were a
method.

## Next Step

Keep 158.007-T active for the owning review workflow. Do not push this local
commit.
