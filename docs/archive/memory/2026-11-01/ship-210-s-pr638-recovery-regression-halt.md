---
title: "210-S PR 638 recovery regression halt"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-01/ship-210-s-pr638-recovery-regression-halt.md
description: "Resume state after an unsuccessful attempt to remediate scoped crash recovery."
chunk_strategy: h1-h2-h3
---

# PR 638 recovery regression halt

Recorded 2026-11-01T17:25:29Z. Shipment `210-S`, feature `209-F`, PR #638,
branch `feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`.
Local HEAD is `8ff9f2f3b1a38cb85a2709da2492fedf95908204`; no commit, push,
review-thread reply or resolution, or merge occurred in this session.

The explicitly selected Ship checkpoint `checkpoint-20261101-170817.json` was
read and its state restored; the backlog index was synchronized and the engram
binding was available. The checkpoint remains active and unresolved because
the resumed work remains blocked. No additional checkpoint was created.

The delegated remediation added a draft created-feature locking change and
regression tests. With Go 1.26.5 and system TEMP outside the repository:

* `go test -count=1 ./internal/core -run '^TestRound3NewFeatureRelatedToAdoptedTask$'`
  passed.
* `TestRound3RecoveryPreservesIndependentSize` failed for both scoped-image
  variants, direct and cross-reference owner: recovery returned no error after
  an independent size update. The P1 data-loss risk is unresolved. The draft is
  not ready for commit or release.

The full quality gates, fresh report-only local review, runtime verification,
PR readiness and current-HEAD CI/Copilot gates remain outstanding. The
worktree, including the existing dirty `.backlogit/stash.jsonl`, must be
preserved. That file was not edited in this session; its SHA-256 after work was
`1FDB182374F80B01992BDEF1BB336BFC6E63F60DB68024945955936B6E15B189`.
The deferred capture `7E14915F` remains for Stage; do not triage or alter it.

Next: resume the existing checkpoint with explicit operator confirmation,
examine the uncommitted draft and implement a safe fail-closed P1 recovery
preflight without expanding the authorized scope. Prove the scoped and
unscoped regressions, then complete fresh verification and review before any
commit, push or fixing reply. Explicit operator approval is required for merge.
