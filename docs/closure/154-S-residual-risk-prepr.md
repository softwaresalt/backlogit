# 154-S residual-risk record (pre-PR)

## Deferred finding

- Deferred scope entry: `6E37FD63`.
- Finding: the frozen regression command
  `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/... ./internal/db/...`
  was run once during W4 and failed because `internal/core` timed out after 10 minutes in the
  pre-existing `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt`
  (`internal/core/shipment_test.go`). The other listed packages passed.
- Disposition: no fix was applied and the command was not rerun under the task's
  no-known-flake-rerun requirement. The exact cause is not isolated. Under P-021 C1, changing
  the pre-existing test or another surface is outside the authorized 173.003-T test-file
  deliverable. Shipment `154-S` halts before W4 convergence; task `173.003-T` remains active.
- Source refs at capture: task `173.003-T`, feature `173-F`, shipment `154-S`,
  PR `N/A` (pre-PR), review thread `N/A` (threadless).

## Discovery record

The deferred entry was captured before any closure action. Active and archived stash sources
and current task/run residual-risk records were searched. Multiple candidates could not be
positively confirmed to describe this same timeout on this same surface; the new entry
therefore records `DISCOVERY-STATUS: AMBIGUOUS`. Candidate IDs copied from the entry:

`7AA35A39`, `A592FC1C`, `5A1C4D3F`, `6434A4D7`, `AF1E5075`, `24D693E1`,
`FE440C62`, `513E62AB`, `BDA56ED8`, `C29EBEE5`.

The new entry is the authoritative capture. Do not edit it, reuse a candidate, or create a
second entry for this same expansion. This record, the task comment, and the run checkpoint
all cite `6E37FD63`. No PR review thread exists, so no reply or resolution action applies.

## Existing known-risk check

- `D116AF58` concerns per-member production `shipment ship` validation performance and is not
  positively the same failure as this Go test timeout.
- `67F17B6B` concerns the harness/backlog lock-sidecar naming collision and was not observed.
- Neither entry was changed.

This is a pre-PR residual-risk record, not a declaration that the shipment is release-ready.
When PR readiness or post-merge closure is eventually prepared, carry forward `6E37FD63` and
the discovery candidates unchanged.
