# Dark mode halt: 189-S claim stuck (2026-10-19)

## Completed

* 188-S shipped (PR 543, merge d9412f90); closure PR 544 merged (dee11dd5).
* Continuity carry-forward PRs 542 and 545 merged.

## Halt: DARK_MODE_HALTED at 189-S

* The claim of 189-S timed out over MCP and left a half-applied operation:
  `.backlogit/queue/189-S.md` shows `status: active` and a pending journal at
  `.backlogit/ops/shipment-operation-83467929c7a4135045b2d155c4bfcb7e.json`.
* Every backlogit CLI/MCP open fails with `recover shipment operations: lock
  shipment lifecycle recovery: ... gate in progress for item`; retries over
  about 2 minutes did not clear it.
* Candidate holders: hung `backlogit mcp` processes (PIDs 23528, 26660, 40780).
  Another session is also running `backlogit update NNN-T --size` calls.
* No process was killed and no lock or journal was hand-edited (destructive; needs operator approval).

## Resolution

At 18:55 the lock cleared on its own: no process held a handle on the lock
files (checked with a handle scan), pending-operation recovery completed, and
189-S is `active` with its members claimed. Cause: a claim that timed out under
load while concurrent sessions contended for the global shipment-lifecycle lock.
Dark mode resumed at 189-S. The steps below are historical.

## Next steps for the operator

1. Identify and stop the process holding the shipment-lifecycle lock.
2. Run any backlogit command so startup recovery completes the 189-S claim, or confirm 189-S is back to `queued`.
3. Re-run dark mode with scope 189-S, 190-S, 191-S, 192-S, 193-S, 194-S, 184-S, 185-S.
   Working branch `feat/claim-time-shipped-only-predecessor-guard` has no commits.
