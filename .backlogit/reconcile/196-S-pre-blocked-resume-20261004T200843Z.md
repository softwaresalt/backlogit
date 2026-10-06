# Shipment Reconciliation — 196-S — Pre / Blocked Resume Audit

- **Timestamp:** 2026-10-04T20:08:43.613Z
- **Mode:** `pre`
- **Member expected status:** `queued`
- **Shipment status:** `blocked`
- **Shipment record classification:** `record-blocked-resumable`
- **Recommendation:** `PAUSED — governed unblock required`
- **Resume checkpoint:** `checkpoint-20261004-192622.json`

## Explicit manifest and current records

The shipment MCP record and its on-disk queue frontmatter agree on shipment ID, blocked
status, update timestamp, and the ordered manifest. The ordered manifest `S` contains ten
unique explicit IDs. Each ID resolves to exactly one queue record at `queued`; no archive
record exists for any member.

| ID | Type | Current status | Classification |
|---|---|---|---|
| `196-F` | feature | queued | matched |
| `196.001-T` | task | queued | matched |
| `196.003-T` | task | queued | matched |
| `196.004-T` | task | queued | matched |
| `196.007-T` | task | queued | matched |
| `196.008-T` | task | queued | matched |
| `196.002-T` | task | queued | matched |
| `196.005-T` | task | queued | matched |
| `196.009-T` | task | queued | matched |
| `196.006-T` | task | queued | matched |

`M` is the nine task IDs above; `196-F` is the sole excluded non-task manifest member.
No unlisted descendants, siblings, source artifacts, or linked deliberations were inspected
or added to scope.

## Block envelope

The shipment has a non-empty `blocked_reason`, RFC3339 `blocked_at`, `blocked_by: ship`,
an exact-manifest `member_status_snapshot` whose ten values are all `active`, and
`resume_checkpoint_ref: checkpoint-20261004-192622.json`. The governed block operation
record `shipment-operation-8e7ee989436fd25bb570d6267bcd60f6.json` is for `196-S`, targets
`blocked`, has phase `committed`, and records the exact ten-member preimage. Its correlated
intent/applied/committed lifecycle evidence was verified during the preceding blocked
audit. The block transaction returned the explicit queue members to `queued`, matching
their current state.

This is a resumable governed pause, not a reconciliation failure. Normal execution remains
paused until the explicit confirmed unblock operation completes.
