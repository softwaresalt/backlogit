# Dark factory run: 188-S to 185-S (2026-10-19 to 2026-10-21)

## Outcome

DARK_MODE_COMPLETE for scope 188-S, 189-S, 190-S, 191-S, 192-S, 184-S, 193-S, 194-S, 185-S.
All shipped through PR, CI, Copilot review (threads replied and resolved), and merge commits.
Final main: b51c5a29.

## Shipments

| Shipment | Feature merge | Closure merge |
|---|---|---|
| 188-S | d9412f90 | dee11dd5 |
| 189-S | b5660192 | cb346b01 |
| 190-S | 1d75be28 | 1a41011e |
| 191-S | c22fea94 | dc9440d6 |
| 192-S | 13dddf8d | 1c33818d |
| 184-S | b826f7b5 | 3e79a869, correction 66361f43 |
| 193-S | d81585ef | 416dc1d2 |
| 194-S | 27eb21e8 | 6c1d1e37, archive fix 77034a1b |
| 185-S | be7280e2 | b51c5a29 |

## Decisions and failed approaches

* 189-S claim stall: the lock cleared by itself; no process was killed.
* 193-S was blocked because the 186-S closure had prose conditions only. Fixed by running live
  halt-row Ship sessions and the dry routing pass (PR 558), not by marking conditions satisfied.
* 184-S closure PR merged with a stale archive record, because it was staged before the governed
  ship rewrote it. Commit the archive record after `shipment ship`.

## Open items

* Compaction is `degraded` on the 188-S, 190-S, 191-S, 192-S, 193-S, 194-S and 185-S closures
  (184-S and 189-S record `done` after an evaluation with no lasting file moves): retained docs and
  checkpoints reference docs/memory paths.
  Repoint them, then compact (needs operator approval).
* Fixture folders under logs/af1e5075-fixture and logs/6434a4d7-fixture await removal approval
  (stashes CBB7997C, E22DDC43).
* closure-evidence close_path/close_evidence not produced by the installed harness (stash 3FF72EB8).
* Plugin agent copies and harness-manifest checksums lag the edited Ship and Stage agents.
* Stage-only dispose is enforced by agent contract only.
* CLI shipment calls took 3 to 15 minutes under load; poll state instead of re-issuing.
