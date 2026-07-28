---
title: "durable_writes second-layer hardening: caller reconciliation and retry idempotency"
description: "Decision to complete ErrWriteIndeterminate caller reconciliation and durable-mkdir/append retry idempotency across five sites before durable_writes is promoted toward default/GA."
source: docs/decisions/2026-07-28-durable-writes-second-layer-hardening-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "durable_writes second-layer hardening (stash 50471E28)"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-07-28-durable-writes-second-layer-hardening-plan.md"
  - "docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md"
  - "docs/exec-plans/2026-07-27-durable-writes-fsync-protocol-plan.md"
tags:
  - durable-writes
  - fsync
  - err-write-indeterminate
  - retry-idempotency
  - error-contract
  - p2-followup
---

## Problem Frame

Feature 123-F (shipment 109-S, PR #308) landed the durable_writes fsync
primitives and the two-class outcome-based error contract
(`ErrWriteNotApplied` — definitely not applied, safe to retry;
`ErrWriteIndeterminate` — possibly applied, never blindly retry or roll back).
Copilot review cycle-4 dispositioned five caller/retry-completeness gaps to
stash `50471E28` rather than opening a fourth fix cycle past the review §1.8
limit. Those gaps are the second layer: the primitives are correct, but not
every caller reconciles the indeterminate class, and two durable-mkdir/append
retry paths early-return past a previously failed parent fsync.

The work must be completed before durable_writes is promoted toward
default/GA, because at GA the triple gate (durable ON + fsync failure + retry)
that keeps these gaps inert would no longer be the default posture.

Who cares: maintainers hardening durable_writes toward default; agents that
retry `append_comment` and must not duplicate audit events.

Success criteria: each of the five sites either reconciles the indeterminate
class per the commit-then-surface contract, re-attempts the previously failed
parent flush on retry, or maps both durability classes to explicit
machine-readable outcomes — each proven by a regression test that injects the
fsync/write failure through an existing seam.

## Research Findings

Grounded in the current codebase (index synced, 962 artifacts) and the
compound learning `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`:

- The two-class contract lives in `internal/errors/durability_errors.go`
  (`ErrWriteNotApplied`, `ErrWriteIndeterminate`, `IsWriteNotApplied`,
  `IsWriteIndeterminate`). Both wrap the root cause with `%w`.
- The **commit-then-surface** pattern is the governing rule: on a post-mutation
  fsync failure, do NOT roll back — accumulate, finish the op so FS and DB stay
  in agreement, and return the built result wrapped with `ErrWriteIndeterminate`.
  `AdoptItem` (after-commit variant) and `persistArtifact` Site-1 (before-commit
  variant) are the reference implementations.
- All five sites have existing in-process fault-injection seams, so tests can
  simulate an fsync/write failure without a real power loss:
  - AC1 archive restore write → `replaceFileWriteFn` seam
    (`internal/core/archive.go`; precedent test
    `archive_durable_write_test.go`).
  - AC2 dependency callers → source-dir fsync routes through the
    `mkdirDirSyncFn` package seam; the cross-ref indeterminate caller pattern is
    already modelled in `artifact_references.go` (`applyCrossRefWriteFn`) with
    `artifact_references_indeterminate_test.go` as the template.
  - AC3 events append/mkdir → `EventWriter.fsyncDirImpl` / `fsyncFileImpl`
    seams (`stream_durable_test.go`).
  - AC4 core `mkdirAllDurable` → `mkdirDirSyncEnabled` / `mkdirDirSyncFn`
    package seams (`durable_fs_test.go`).
  - AC5 MCP `handleAppendComment` → behavioural test surface in
    `append_comment_test.go`.
- Confirmed current locations (line numbers drift, verified by grep):
  - AC1 `UnarchiveItem` non-git branch — `internal/core/archive.go`
    (~L768 restore content write returns early on error).
  - AC2 `AddDependency` / `RemoveDependency` → `persistArtifact`
    — `internal/core/dependencies.go` (L46, L100) and
    `internal/core/shipment.go` (`persistArtifact` L366).
  - AC3 `appendDurable` / `EventWriter.mkdirAllDurable`
    — `internal/events/stream.go` (~L133, ~L169).
  - AC4 `mkdirAllDurable` — `internal/core/durable_fs.go` (~L33).
  - AC5 `handleAppendComment` — `internal/mcp/tools.go` (~L943) mapping every
    append error to `InternalError`.

## Options Evaluated

### Option A: One combined "durable hardening" task across all five sites

Fix all five ACs in a single task/PR-sized unit.

- Pros: one review pass; shared contract context stays warm.
- Cons: violates the 2-Hour Rule and Width Isolation (5 files, 8+ functions,
  10+ test scenarios); a single failing site blocks the rest; hard to bisect.

### Option B: Five independent test-first tasks, one per site (one file each)

Decompose into five atomic tasks, each scoped to a single file plus its
colocated test, each written test-first against the existing seam.

- Pros: each task is <3 files, <5 functions, <4 test scenarios; single skill
  domain (Go code + colocated test); atomic verifiable milestone (failing test
  → passing test); the sites are largely independent so tasks parallelize and
  bisect cleanly.
- Cons: five review passes; minor contract-context repetition across tasks
  (mitigated by the plan citing the compound learning once).

### Option C: Combine the two mkdir-retry sites (AC3 + AC4) into one task

AC3 and AC4 are the same class of early-return-skips-failed-flush bug in two
different `mkdirAllDurable` implementations.

- Pros: conceptually symmetric fix.
- Cons: they live in different packages/files (`events/stream.go` vs
  `core/durable_fs.go`) with different seams; combining crosses a package
  boundary and couples two independent regressions. Width Isolation favors
  keeping them separate.

## Trade-off Comparison

| Criterion | Option A (combined) | Option B (5 tasks) | Option C (merge AC3+AC4) |
|---|---|---|---|
| 2-Hour Rule fit | Fails | Passes | Marginal |
| Width isolation | Fails | Passes | Fails (2 packages) |
| Bisectability | Poor | Strong | Medium |
| Review load | 1 pass | 5 passes | 4 passes |
| Blast-radius isolation | Low | High | Medium |

## Decision

Adopt **Option B**: one covering feature decomposed into five independent,
test-first tasks — one per acceptance criterion, each scoped to a single
production file plus its colocated `_test.go`. Each task writes the failing
regression test first (injecting the fsync/write failure through the named
existing seam), then applies the fix.

Governing rule for every task: the **commit-then-surface** contract from the
compound learning. Post-mutation fsync failures are surfaced as
`ErrWriteIndeterminate` and are never rolled back; pre-mutation failures remain
`ErrWriteNotApplied` and are safe to retry; retry paths must re-attempt a
previously failed parent flush rather than early-returning past it.

The five sites are largely independent (confirmed during research): no
execution-blocking dependencies are wired between them. AC3 and AC4 share a bug
class but not a file, so they stay separate with no dependency edge.

## Rejected Alternatives

- Option A rejected: violates task-granularity NON-NEGOTIABLE constraints and
  harms bisectability of five distinct regressions.
- Option C rejected: couples two independent regressions across a package
  boundary and breaks Width Isolation for a purely conceptual symmetry.

## Scope Boundaries (explicitly OUT of scope)

- Do NOT promote `durable_writes` to default/GA in this feature. This feature is
  the prerequisite hardening; the default flip is a separate future decision.
- Do NOT change the two-class error contract or the fsync primitives from 123-F.
- Do NOT touch the three non-stageable stash entries (6FA0829B, 7F0A6E89 —
  external autoharness repo writes forbidden by Constitution Principle IV;
  918BCDAF — GitHub branch-protection operator/admin action outside the repo
  tree). They remain active in the stash.

## Unresolved Questions

- AC2 has a documented "or document reliance on sync-rebuild" escape hatch. The
  plan should default to explicit reconciliation (do-not-roll-back on
  indeterminate) and only fall back to a documented sync-rebuild note if
  reconciliation proves disproportionate for the legacy dependency callers.
- Windows/POSIX seam: dirent durability is best-effort on Windows. Tests must
  swap the package-global seams (no `t.Parallel`) to exercise POSIX ordering
  in-process on a Windows host.

## Risks and Mitigations

- Risk: a fix rolls back an indeterminate write and diverges FS from DB.
  Mitigation: commit-then-surface contract is mandatory; regression tests assert
  no rollback and no duplicate event/comment on the indeterminate path.
- Risk: retry double-applies (duplicate audit event / duplicate comment).
  Mitigation: AC3/AC5 tests assert event/comment count stays exactly one across
  a retry, mirroring `TestSizeSeam_*` idempotency assertions.
- Risk: triple-gated paths are hard to reach in tests. Mitigation: every site
  has a named existing seam; the plan cites each seam per task.
