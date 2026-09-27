---
type: circuit-breaker
timestamp: 2026-09-23T19:14:00Z
agent: "Ship"
skill: "direct quality-gate verification"
breaker_type: universal
operation: "go test canonical byte-stability gate"
attempts: 4
identity: "canonical-byte-stability-crlf-materialization"
---

# Circuit Breaker - Canonical Byte-Stability Test

## Failure Chain

### Attempt 1

* Exit/timeout: exit 1
* Operation evidence: `go test ./... -timeout 15m`; repository root;
  final quality-gate phase; `TestU4aBehaviorCanonicalByteStable`;
  `internal/faultline/testdata/parity_v1.golden.json`
* Normalized message: Generated canonical bytes end with LF, but the Windows
  checkout fixture was read with CRLF
* Diagnostic artifact: delegated Go Engineer result in the active session

### Attempt 2

* Exit/timeout: exit 1
* Operation evidence: `go test ./... -count=1 -timeout 30m`; repository root;
  final quality-gate phase; `TestU4aBehaviorCanonicalByteStable`;
  `internal/faultline/testdata/parity_v1.golden.json`
* Normalized message: Same LF-versus-CRLF canonical byte mismatch
* Diagnostic artifact: active session tool result

### Attempt 3

* Exit/timeout: exit 1
* Operation evidence: targeted canonical test under `canonical-final`;
  final quality-gate diagnosis; `TestU4aBehaviorCanonicalByteStable`;
  `internal/faultline/testdata/parity_v1.golden.json`
* Normalized message: `checkout-index` materialization still contained one
  CRLF terminator
* Diagnostic artifact: byte count reported `crlf=1`, `lf=1`

### Attempt 4

* Exit/timeout: exit 1
* Operation evidence: targeted canonical test under `canonical-final2`;
  final quality-gate diagnosis; `TestU4aBehaviorCanonicalByteStable`;
  `internal/faultline/testdata/parity_v1.golden.json`
* Normalized message: `git archive` materialization also applied one CRLF
  terminator
* Diagnostic artifact: byte count reported `crlf=1`, `lf=1`

## Context

* Files involved:
  `internal/faultline/testdata/parity_v1.golden.json`,
  `internal/faultline/evidence_conformance_test.go`
* Provisional-to-concrete identity link: Every invocation produced the same
  byte-for-byte LF-versus-CRLF mismatch in the same test and fixture
* Blob evidence: `git cat-file blob` reports `crlf=0`, `lf=1` for both `HEAD`
  and `origin/main`; the remediation does not modify the fixture
* Task evidence: focused lock-order tests, full core tests, full core `-race`,
  compile, vet, CI-pinned lint, build, and `git diff --check` passed
* Logging controls: bounded summaries only; no raw environment or payload
  capture retained
* Resolution: Circuit breaker triggered. No further canonical test execution
  is permitted in this session
* Suggested next step: Resume in a fresh governed session and materialize
  tracked files directly from `git cat-file` blobs, or run the final suite in
  CI/Linux where checkout line endings match the committed blobs
