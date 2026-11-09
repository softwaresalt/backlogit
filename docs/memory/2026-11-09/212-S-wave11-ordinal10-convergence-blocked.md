---
title: "212-S Wave 11 Ordinal 10 convergence blocked"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave11-ordinal10-consumed-failed"
---

## Completed amendment and recovery

The operator explicitly selected and confirmed recovery of
`checkpoint-20261109-204413.json`. Full enumeration found no malformed or
quarantined records. The selected checkpoint was valid, conforming, and
Ship-owned. Bounded Engram CLI retrieval and prior memory supplied the
recovery summary without pruning a cursor, unresolved pointer, verdict, or
historical counter. Only that selected checkpoint was resolved.

The seven Stage amendment files matched all operator-supplied SHA-256
digests and were committed by exact path in
`826e98f086052e78951c6f47cc171b3dfd07f32e`.
The delegated, test-first four-file Windows tuning landed in
`9b07a7f5de55879054f423db541cc418c0084376`.
Both tasks remain done, and both are associated with that commit through
MCP commit tracking. No task was reclaimed or restarted, and no new
WORK_STARTED record was appended.

The tuning provides six retries, seven total attempts, and delays
1/2/4/8/16/32 ms, totaling 63 ms. Read-only and directory destinations
fast-fail without retry or sleep on access denied. Core probes remain
handle-relative to the bound directory and do not follow reparse points.
Inconclusive attribute probes retain bounded retry behavior.

Both authorized scoped race commands passed after named assertion RED.
Repository compile-only, vet, pinned lint, and format gates passed.
The delegated delta review found no substantiated defects but had incomplete
leaf inspection coverage. Ship directly inspected the full four-file diff
to cover the missing language, concurrency, and security checks, finding
zero P0/P1/P2/P3 issues. This was not branch-wide persona readiness.
See [prelaunch evidence](212-S-ordinal10-tuning-ready.md).

Commit `b6fdacfd2a017d49c058de6f0d5fcc30e388d1a1` records traceability,
selected checkpoint resolution, and prelaunch memory. All five specified
traceability/checkpoint files were already UTF-8 LF without BOM; hooks and
memories required no rewrite. The worktree was completely clean and
`TestU175_001_BaselineEol` passed before authorization.

The protected `checkpoint-20261106-210317.json` remains active, untouched,
and unstaged. Its SHA-256 was verified before and after the suite:
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
Live routing configuration, MCP served-root/database attestation, index
sync, and lifecycle topology checks passed. Hook polling returned no
events. Agent-intercom is not installed; visibility used local output.

## Single authorized execution

Receipt: `212-S-wave11-unfiltered-convergence-ordinal-10`.
Issuer: coordinator under explicit operator authorization.
Ordinal: 10. Historical failed count before execution: 8.
Budget: 2,700 seconds. Package timeout: 1,800 seconds.
The 64-byte external token and authorization/counter receipts were created
in ignored `bin/tmp/`, bound to the exact clean HEAD above.

Exactly one unfiltered suite child was launched through the workspace
environment wrapper:

```text
go test -count=1 -p=1 -timeout=30m -json ./...
```

Independent Go discovery returned all 46 first-party packages.
Discovery and the driver's EOL preflight passed. The driver remained
unchanged, with SHA-256
`9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e`.
HEAD and the clean worktree remained unchanged throughout the run.
No manual stop or second launch occurred.

## Observed result

| Evidence | Result |
|---|---|
| Authorization and post-run HEAD | `b6fdacfd2a017d49c058de6f0d5fcc30e388d1a1` |
| Suite start | `2026-11-09T21:56:58Z` |
| Suite finish | `2026-11-09T22:41:58Z` |
| Elapsed | 2,700.038 seconds |
| Suite native exit | 124 |
| Evidence valid | false |
| Named failure events | 3, comprising two U6 subtests and their parent |
| Top-level passes recorded | 2,849 |
| Package terminals | 34 pass, 2 fail, 10 missing |
| Critical receipt errors | 5 missing required receipts |
| Test cycle consumed | true |
| Recorder evidence guard | FAIL |
| Idle-sleep hold and clear | both successful, same thread |
| Maximum poll gap | 0.062245 seconds |
| Poll gaps over five seconds | 0 |
| Supervisor or cleanup errors | none |

The failing CLI test was
`TestU6_PrewarmAndLazyAutoStartColdRaceConvergence`, with these failing
subtests:

* `lazy_first_client_eof_preserves_daemon_and_later_attach`
* `four_mixed_cold_starts_recover_stale_sidecar_to_one_pid`

Both reported a validation error stating that `runtime/mcp-daemon.json`
changed while opening. CLI terminated failed after 337.291 seconds.
These are observed failures, not an established causal diagnosis or
authorization to modify the runtime.

Core emitted `panic: test timed out after 30m0s` and terminated failed
after 1,802.332 seconds. The timeout listed
`TestValidateShipmentReconcilePreconditions_ArchivedRejectedMemberFailsClosed`
as running for zero seconds. Its retained stack included a wait for a
task-file heartbeat cleanup. This does not establish that the current
test caused the package's accumulated runtime or a deadlock.

The overall supervisor then exhausted its authorized 45-minute budget
and terminated its owned job. The unfinished inventory also contains
`internal/redhonesty/TestU161_003_GateAcceptsGenuineReproducerEndToEnd`.
No culprit or root cause is established.

The ten packages without terminal results are:

* `internal/redhonesty`
* `internal/release`
* `internal/soundness`
* `internal/stash`
* `internal/telemetry`
* `internal/version`
* `scripts`
* `tests`
* `tests/contract`
* `tests/integration`

## Evidence and counters

Local ignored evidence directory:
`bin/_rr9501da8a/wave11-ordinal10/`.
It contains independent discovery and EOL receipts, driver snapshots,
bounded raw output, `result.json`, and `recorder-result.json`.
Raw suite capture retained 1,048,576 of 8,214,646 bytes. The recorder
processed the complete stream presented to it and reported retention
limits explicitly. No failure entries or package failure tails were omitted.

* `result.json` SHA-256:
  `4f0915989d1a4a56b549264917ad47d5f9954b4e89e9729d2c3450b6a4c77c70`
* `recorder-result.json` SHA-256:
  `848d37ae1a8e9b96eb2110df73ff53237f28e4876fda02c00b6c4de2c228b46a`

Coordinator receipts now say `consumed_failed`. The authorization receipt
retains its issued failed count of 8 and records `failed_count_after: 9`.
The counter receipt records 9 and preserves the prior authorized count of
8. Neither receipt authorizes another run. No counter was reset.
The historical U1j build 5/5 counter was not reopened or reset; only the
explicit surgical Stage tuning amendment was executed.

## Gate disposition and next step

The live exact-ID census remains M21: 21 done, zero queued, active,
claim-assigned, active residual, blocked, or unsupported task members.
S22 remains 22 explicit members. Open red is empty.
The existing shipment branch was retained, and successors were untouched.

Wave 11 Step 4.6 and Step 5 item 1 remain BLOCKED.
Full local build and current-HEAD branch-wide standard plus three-model
Anchor review were conditional on suite success and were not invoked.
Local branch-wide readiness is NOT ESTABLISHED.
No PR was created, pushed, or merged. CI, Copilot, runtime verification,
merge approval, and operational closure remain pending.

Stop here. Ordinal 10 is consumed, with nine cumulative full-suite
failures. Further execution requires a fresh operator/Stage failed-
convergence disposition. No Ordinal 11, timeout increase, counter reset,
additional implementation repair, or merge is authorized.
The failed observations were preserved rather than silently deferred or
treated as successful convergence.
Existing residuals `0D073C21`, `D2FCA0FB`, native POSIX validation,
incomplete graph coverage, and prior incomplete adversarial coverage
remain unchanged.

The final structured checkpoint is
`.backlogit/checkpoints/checkpoint-20261109-224251.json`.
It was created through MCP and preserves the failed execution, evidence
digests, inherited counter references, and a no-retry resume hint.
It is the single active checkpoint created by this session.
The older protected active checkpoint was not resolved.
Explicit operator selection and confirmation are required for recovery.
