---
title: Ship 155-S Wave 17 Correctives Complete
date: 2026-09-24
status: awaiting-full-suite-authorization
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 9868af4ac697c5dc87419fbb5ce5c5731702df8f
---

# Ship 155-S Wave 17 Correctives Complete

## Outcome

Corrective tasks `174.071-T` and `174.072-T` completed and archived through
their ordinary lifecycle. The previously selected Ship checkpoint was restored
and resolved only after branch, ancestry, shipment, and workspace state
reconciled.

No full-suite, final review, CI, PR, merge, or closure operation ran.

## Task 174.071-T

Commits:

* `cfe37339` — test-only no-recovery workspace seam and fixture isolation
* `8a347ba3` — task archival

Changed files:

* `internal/core/workspace_test_seam_test.go`
* `internal/core/workspace_no_recovery_regression_test.go`
* `internal/core/artifacts_expansion_test.go`

Evidence:

* Bounded RED reproduced recovery-enabled construction rejecting malformed
  recovery state
* Targeted hierarchy/expansion and no-recovery regression selectors passed
* Explicit `TestNewWorkspace_RecoversPendingReturnBlockedJournal` passed
* Internal core compile and vet passed
* Go 1.24 `golangci-lint` v1.64.8 new-change validation passed
* `go build ./cmd/backlogit` passed
* Scoped `gofmt` and `git diff --check` passed

Recovery and journal tests continue to construct workspaces explicitly through
`NewWorkspace`; production constructors, recovery, and locking were unchanged.

## Task 174.072-T

Commits:

* `94ddde9d` — explicit Ship governed tool allowlist and manifest parity
* `9868af4a` — task archival and upstream follow-up capture

Changed files:

* `.github/agents/_ship.agent.md`
* `.autoharness/harness-manifest.yaml`
* `tests/integration/shipment_155_harness_contract_test.go`

Evidence:

* Targeted integration contract failed RED on `backlogit/*`
* The same selector passed after restoring the explicit scalar allowlist
* The allowlist retains `memory`, all intended explicit backlogit tools, the
  block/unblock/normalize lifecycle trio, `engram/*`, and eight read-only
  graphtor verbs
* `backlogit/*` and `vscode/memory` are absent
* Current model routing remains `gpt-6-luna` / `openai` / `xhigh`
* LF-normalized Ship checksum is
  `d6b046f8d2299e8b146df95994fa7d5d6472c28eec56335a8e8988fbf242a3d9`
* Harness verification retained its existing baseline: zero strict schema
  blockers, zero blockers, two warnings, one migration proposal, and zero
  unresolved placeholders

The one allowed verifier invocation ran before final LF materialization, so its
persisted report contains an obsolete raw-byte Ship checksum observation. Two
independent post-normalization checks matched the manifest checksum above. The
verifier was not rerun.

Mandatory upstream tune-preservation follow-up stash: `EC43AB70`.

## Current state

* `155-S`: active
* `154-S`: queued and unmodified
* `174.071-T`: archived
* `174.072-T`: archived
* Implementation PR: none
* PR #449: untouched
* Routing: degraded from configured Ship `gpt-6-luna` to the current
  GPT-5.6 Sol runtime

## Next action

Await explicit authorization for exactly one execution of:

```text
go test ./...
```

The command must start with workspace-contained bounded diagnostics and native
exit metadata. Do not rerun it after failure without new authorization.
