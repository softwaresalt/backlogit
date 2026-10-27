# shipment-reconcile safe-close and post: 201-S

- mode: safe-close then post; merge_commit_sha: f3b43e52bff174f155e83c55394d4ef562ef6d07 (PR #591)
- manifest M (11): 201-F, 201.001-T .. 201.010-T
- governed closure: `backlogit_ship_shipment` timed out client-side and converged server-side (verified by polling, no retry)
- shipment record: archived, `archived_status: shipped`

| Member | Classification |
|---|---|
| 201-S (control record) | archived (queue absent, archive present, archived_status shipped) |
| 201-F | matched (archive record with valid provenance) |
| 201.001-T .. 201.010-T | matched (archive records with valid provenance) |

Allowlisted paths (exact):

* .backlogit/queue/201-S.md (removed), .backlogit/archive/201-S.md
* .backlogit/queue/201-F.md (removed), .backlogit/archive/201-F.md
* .backlogit/archive/201.001-T.md .. .backlogit/archive/201.010-T.md
* .backlogit/reconcile/201-S-pre-20261027T1620Z.md, .backlogit/reconcile/201-S-safe-close-post-20261027T1700Z.md

P-007: no archive file shows as a working-tree deletion. No non-member mutation detected.

Recommendation: CLOSED
