# 154-S Runtime Verification

## Context

- **Shipment / feature:** `154-S` / `173-F` — Shipment-claim scheduler-baseline marker
- **Merged PR:** #466
- **Merge commit:** `6d233d21162a072ddbdfecb52ec62a8fb8a63793`
- **Branch:** `post-merge/154-s-closure`
- **Profile contract:** `.autoharness/workspace-profile.yaml` declares no
  validator-manifest surfaces; `validation_expectations.required` is `false`,
  with minimum verdict `PASS`.

## Surface and adapter

The shipped code affects the backlogit CLI and MCP item/shipment read surfaces.
With no configured validator entry and runtime validation not globally
required, the runtime skill's manual adapter was used for bounded local smoke
checks. No external autoharness scheduler environment was available.

| Probe | Result |
|---|---|
| `go build ./cmd/backlogit` | PASS |
| `.\backlogit.exe get 173.006-T --format json` | PASS; returned the unique archived `done` task record with merge commit traceability. The response did not contain `custom_fields.scheduler_baseline_claim`. |
| MCP `backlogit_get_item(173.006-T)` | PASS; returned the archived task record. No marker field was present in the response. |
| `.\backlogit.exe shipment list --status active --format json` | PASS; returned an empty JSON array after 154-S closure. |
| MCP `backlogit_list_shipments(status="active")` | PASS; returned an empty array. |

The member reads establish local CLI/MCP availability and the closed shipment
state. They do **not** establish that an external scheduler consumed a
154-S marker. The explicit bootstrap began with active member preimages, so
the marker's queued-preimage write condition is not proven by these archived
member reads.

## Invariants and manual checkpoint

Before any successor shipment behind 154-S is routed, the external scheduler
owner must attest that its current consumer is using the marker contract. The
relevant consumer rule is three-part: the item is `active`, its
`scheduler_baseline_claim` equals the currently active shipment ID, and the
item is in that shipment's manifest. A missing/ambiguous read, more than one
active shipment, CLI/MCP disagreement, doctor journal finding, or manifest
drift is indeterminate: fail closed and defer to the backlogit claim gate.

This is an open manual checkpoint tracked by existing stash `AF1E5075`;
no attestation was present in the available workspace evidence. Do not route
successors until it is recorded on the shipped 154-S artifact. No additional
stash entry for this attestation was created.

## Validator evidence

- **Verdict:** `PASS_WITH_FOLLOW_UP` for local CLI/MCP startup and read probes.
- **External scheduler consumption:** not verified; remains a routing
  prerequisite, not a claim of runtime success.
- **Manual evidence:** operator attestation required before successor
  routing; no manual verification was represented as complete.
- **Risky action:** governed post-merge shipment close used merge SHA
  `6d233d21162a072ddbdfecb52ec62a8fb8a63793`; archived state and shipped-event
  evidence were checked separately.

## Releasability handoff

- **Monitoring:** no deployment or active runtime environment was available
  for external scheduler observation. The external scheduler owner should
  validate the active-shipment/manifest/marker reads before the first
  successor route.
- **Rollback / mitigation:** on any indeterminate marker read, stop successor
  routing and defer to the claim gate. Do not mutate archived marker data or
  synthesize lifecycle events. Any code rollback requires a separately
  reviewed and approved PR.
- **Owner:** repository operator and the owner of the external autoharness
  scheduler.
- **Validation window:** until the attestation is recorded and the first
  successor shipment has been observed using the predicate above.
- **Outcome:** `READY_WITH_CONDITIONS` for post-merge closure; successor
  routing remains gated by the outstanding attestation.
