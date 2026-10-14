---
type: circuit-breaker
timestamp: 2026-09-24T02:19:30Z
agent: "Ship"
skill: "Go Engineer"
breaker_type: universal
operation: "go test -count=1 -run='^TestBlockRecovery_CanonicalizesAppliedTargetForCAS$' ./internal/core"
attempts: 3
identity: "windows-handle-relative-rename-SetFileInformationByHandle-ERROR_INVALID_PARAMETER"
---

# Circuit Breaker - Windows Handle-Relative Rename

## Failure Chain

### Attempt 1

* Exit/timeout: exit 1
* Operation evidence: focused `internal/core` test on Windows during
  `174.068-T`
* Stable target: `TestBlockRecovery_CanonicalizesAppliedTargetForCAS`
* Normalized message: `SetFileInformationByHandle(FileRenameInfo)` returned
  `The parameter is incorrect`
* Affected path: `internal/core/shipment_ops_windows.go`
* Diagnostic artifact: none; bounded delegate result retained in session

### Attempt 2

* Exit/timeout: exit 1
* Operation evidence: same focused selector after correcting the
  `FILE_RENAME_INFO` allocation size
* Stable target: `TestBlockRecovery_CanonicalizesAppliedTargetForCAS`
* Normalized message: `SetFileInformationByHandle(FileRenameInfo)` returned
  `The parameter is incorrect`
* Affected path: `internal/core/shipment_ops_windows.go`
* Diagnostic artifact: none; bounded delegate result retained in session

### Attempt 3

* Exit/timeout: exit 1
* Operation evidence: same focused selector after adding the documented
  traverse right to the root directory handle
* Stable target: `TestBlockRecovery_CanonicalizesAppliedTargetForCAS`
* Normalized message: `SetFileInformationByHandle(FileRenameInfo)` returned
  `The parameter is incorrect`
* Affected path: `internal/core/shipment_ops_windows.go`
* Diagnostic artifact: none; bounded delegate result retained in session

## Context

* Task: `174.068-T`
* Shipment: `155-S`
* Branch:
  `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`
* HEAD: `7146a2f8f1c5895a936d2774f97dd97741406c98`
* Files involved:
  `internal/core/shipment_ops_windows.go`,
  `internal/core/shipment_ops_security_windows_test.go`
* Earlier focused object-swap and security selectors passed, including a race
  run, before the ordinary lifecycle write exposed the rename failure
* The latest source state is not certified by the earlier quality-gate results
* Provisional-to-concrete identity link: all three attempts used the same
  selector, target path, Windows API, native exit result, and normalized error
* Logging controls: no raw payload captured; summary is bounded and excludes
  environment values and sensitive content
* Resolution: circuit breaker triggered; no fourth local test or equivalent
  operation is permitted
* Suggested next step: perform read-only diagnosis of the Windows
  `FILE_RENAME_INFO` contract, then obtain a new governed remediation and
  verification authorization
