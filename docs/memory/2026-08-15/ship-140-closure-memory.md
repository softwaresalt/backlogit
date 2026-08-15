---
title: Ship 140 closure memory
type: session-memory
date: 2026-08-15
---

# Ship 140 Closure Memory

## Outcome

Feature `140-F` shipped through PR #362 with merge commit `fa54a35ba2c7f2147e8cb0694b9b12be7238070c`.

## Changes

* Added compensating rollback behavior that leaves the raw event log untouched when event capture or diffing fails
* Replaced task and event-log stale-sidecar reclamation with stable OS advisory handles
* Serialized adoption destination ID allocation with the parent artifact lock
* Held old and new event-log locks in stable order through adoption migration
* Added scanner-limit rollback and advisory-lock regression coverage

## Verification

* Focused rollback and adoption tests passed
* Full `internal/core` and `internal/events` Go test suites passed locally
* Full repository Go tests and `golangci-lint` had passed before final rollback review remediation; PR CI passed on the final head
* Copilot review passed on exact head `671fe67793b867804d671d5b0f74cc80ceb8ec4a`
* All PR checks passed: tests, Markdown lint, frontmatter gate, CLI reference drift, and code-change detection
* Merge-commit ancestry was confirmed in `origin/main`

## Backlog Traceability

* `140-F`, `140.001-T`, and `140.002-T` are `done`
* Merge commit was associated with all three backlog items
* Feature closure comment recorded PR, merge SHA, review, CI, and remediation evidence

## Subsequent Ship Progress

* Feature `141-F` shipped through PR #363 with merge commit `22827b1eedeed7e6cbc31ff870f33950dbefb1ee`
* The feature `141-F` and task `141.001-T` backlog items are `done` with merge-commit traceability
* Feature `142-F` remains queued for the next Ship execution unit
* Feature `138-F` remains blocked because its tasks require external-repository writes; stash entries `7F0A6E89` and `6FA0829B` remain active
* No formal release was created or published
