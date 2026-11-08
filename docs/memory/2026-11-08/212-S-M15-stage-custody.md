# 212-S M15 Stage custody receipt

Ship independently loaded the Stage handoff packet and validated all twelve declared file hashes. Packet SHA256: `481ae57770f1cdeb9e7ba072a1f8dd99e08e91334b349f34e2deba76dc9c210e`.

Source HEAD at handoff: `b40a2a9fb9decbc9b808aaeaf5443ee2277ef475`. Same primary branch/worktree. Stage reports custody released and idle; Ship takes serialized implementation custody.

Fresh bound MCP catalog and main-connection pragma confirmed supplied workspace/storage roots and the single storage-root direct-child `backlogit.db`. Fresh shipment record has S16 in the packet's exact order and status active, updated `2026-11-08T01:36:38.4473433Z`. Fresh exact-ID SQL resolves all fifteen task members: nine done, five active, one queued. Excluded member: `211-F` (feature).

Frozen M15:

`211.001-T,211.002-T,211.003-T,211.004-T,211.005-T,211.006-T,211.007-T,211.008-T,211.010-T,211.009-T,211.011-T,211.012-T,211.013-T,211.014-T,211.015-T`

Ready wave 3: `211.015-T` only. Its four blocks dependencies are independently confirmed and done. Expected subsequent waves: U3=4, U4=5, U5=6, U6=7, U7=8.

Canonical red mapping: task `211.015-T`; selector `go test -count=1 -run '^TestU2i_' ./internal/cli`; green-makers `211.003-T` through `211.007-T`; closing wave 8. It does not enter the completed open-red set until valid red-deliverable dispatch and completion.

Stage artifact bytes are committed unchanged through exact paths; `.backlogit/stash.jsonl` is intentionally not staged because its current differences are existing Ship deferred captures, not this Stage amendment. No stash edits or disposition. No shipment reclaim, previous-task replay or cursor restore.

Six historical full failures remain. Official checkpoint `checkpoint-20261106-210317.json` remains byte-preserved. Carry A47CD72B, 264ACCED, A467B030, F0973A97, 77493741, 7FBE7FFA; Stage provenance F2509AE3/96F1DFFD remains untouched.
