---
type: circuit-breaker
timestamp: "2026-10-03T00:25:33Z"
agent: "Ship"
skill: "direct"
breaker_type: universal
operation: "proof-driver checker SelfTest for active shipment JSON"
attempts: 3
identity: "Checker self-test failed: active-shipment-json"
---

# Circuit Breaker - Proof Driver Active Shipment JSON

## Failure Chain

### Attempt 1

- Exit/timeout: exit code 1
- Operation evidence: `pwsh -NoProfile -File logs/2a355f83-proof/proof-driver.ps1 -Mode SelfTest`; cwd `C:\Source\GitHub\backlogit`; phase: scratch proof-driver preflight; stable target: `active-shipment-json`
- Normalized message: `Checker self-test failed: active-shipment-json`
- Diagnostic artifact: shell 1063; driver `logs/2a355f83-proof/proof-driver.ps1`

### Attempt 2

- Exit/timeout: exit code 1
- Operation evidence: same SelfTest command and cwd; phase: scratch proof-driver preflight; stable target: `active-shipment-json`
- Normalized message: `Checker self-test failed: active-shipment-json`
- Diagnostic artifact: shell 1067; driver `logs/2a355f83-proof/proof-driver.ps1`

### Attempt 3

- Exit/timeout: exit code 1
- Operation evidence: same SelfTest command and cwd; phase: scratch proof-driver preflight; stable target: `active-shipment-json`
- Normalized message: `Checker self-test failed: active-shipment-json`
- Diagnostic artifact: shell 1069; driver `logs/2a355f83-proof/proof-driver.ps1`

## Context

- Files involved: ignored operational driver `logs/2a355f83-proof/proof-driver.ps1`; scratch capture `logs/2a355f83-proof/dryrun-20261003T002107Z/workspace/logs/2a355f83-proof/dryrun-20261003T002125Z/claim-marks-members-active-shipments.txt`
- Related dry-run evidence: full dry run shell 1059 failed at Step 6 with `PROOF_PRECHECK_FAILED — active shipment IDs:  (expected exactly 001-S)`. The captured `shipment list --status active --format json` output is valid and contains `id: 001-S`; the driver extracted no IDs.
- Post-trip invocation: after the third matching failure, a diagnostic SelfTest invocation (shell 1071) errored on a null-valued expression and a further SelfTest invocation (shell 1073) again failed `active-shipment-json`. These calls occurred because the breaker was not recognized immediately; no further proof-driver SelfTest or dry run is authorized.
- Provisional-to-concrete identity link: none; all three counted attempts returned the same named checker failure.
- Logging controls: no raw output was copied into this checkpoint; bounded evidence is retained under ignored `logs/` paths.
- Branch: `feat/claimed-versus-started-bootstrap-repair-for-ship-wave-admission-2a355f83`; `git status --short` was empty.
- Shipment status: no counted proof attempt began. `195.006-T` remains 0/5 under the reset budget; `195.007-T` remains 0/5. Earlier scratch runs are retained.
- Resolution: circuit breaker triggered after three consecutive same-operation failures. Halted; awaiting operator guidance.
- Suggested next steps: inspect the captured active-shipment JSON and repair the collection-shape checker only after operator direction, then rerun parser, SelfTest, and the required full scratch dry run before any counted proof attempt.
