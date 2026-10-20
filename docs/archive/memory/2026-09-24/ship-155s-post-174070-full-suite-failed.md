---
title: Ship 155-S Post-174.070 Full Suite Failed
date: 2026-09-24
status: blocked
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 46145ed5f3b68aea0c98d7b60610ece6df72174d
---

## Outcome

The single authorized post-correction execution of `go test ./...` exited `1`.
The command was not rerun. Final review, PR, CI, merge, shipment closure, and
post-merge work did not start.

## Captured evidence

The bounded capture is complete and untruncated:

* Output: `logs/diagnostics/155-s-go-test-post-174070-20260923.txt`
* Metadata:
  `logs/diagnostics/155-s-go-test-post-174070-20260923.metadata.json`
* Native exit: `1`
* Captured: 780 lines, 113,607 bytes
* Limit: 10,000 lines or 1 MiB

## Failure 1

Package `github.com/softwaresalt/backlogit/internal/core` timed out after ten
minutes while running:

```text
TestCreateArtifact_RejectsLevel2WithoutParent
```

The stack shows:

```text
025_hierarchy_harness_test.go:29
  -> setupTestWorkspace
  -> NewWorkspace
  -> recoverPendingShipmentOperations
  -> lockShipmentLifecycleGlobalRaw
  -> lockShipmentMembership
  -> resolveContainedArtifactPath
  -> WorkspaceStorageRoot
  -> ResolveStorageRoot
  -> filepath.EvalSymlinks
```

The package result was:

```text
FAIL github.com/softwaresalt/backlogit/internal/core 600.818s
```

## Failure 2

Integration test
`TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery` failed
at `tests/integration/shipment_155_harness_contract_test.go:43`.

The Ship tool declaration did not contain the expected governed operation:

```text
backlogit_block_shipment
```

The package result was:

```text
FAIL github.com/softwaresalt/backlogit/tests/integration 37.945s
```

## Current state

* Corrective tasks through `174.070-T` remain archived
* `155-S` remains active
* `154-S` remains queued and unmodified
* PR #449 remains untouched
* Current HEAD remains `46145ed5f3b68aea0c98d7b60610ece6df72174d`
* No standard or adversarial review, PR, CI, merge, or closure work ran

## Next action

Stage must disposition both failures as governed corrective work before a new
full-suite operation can be authorized. The failed full-suite operation must
not be rerun in its current state.
