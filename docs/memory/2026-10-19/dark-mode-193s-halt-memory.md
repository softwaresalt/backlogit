# Dark mode halt: 193-S blocked by 186-S closure evidence (2026-10-19)

## Completed in this dark run

| Shipment | Feature PR merge | Closure PR merge |
|---|---|---|
| 188-S | d9412f90 (#543) | dee11dd5 (#544) |
| 189-S | b5660192 (#546) | cb346b01 (#547) |
| 190-S | 1d75be28 (#548) | 1a41011e (#549) |
| 191-S | c22fea94 (#550) | dc9440d6 (#551) |
| 192-S | 13dddf8d (#552) | 1c33818d (#553) |

The earlier 189-S claim stall cleared on its own; no process held the lock.

## Halt: DARK_MODE_HALTED before 193-S

* `autoharness gate pipeline-topology --phase pre_claim` for 193-S returns
  `PREDECESSOR_CLOSURE_INCOMPLETE: predecessor 186-S is terminal but missing required closure evidence`.
* Cause: `docs/closure/186-S-186-F-post-merge-closure.md` is `READY_WITH_CONDITIONS` with prose
  conditions and no frontmatter `conditions:` list. The predicate requires every condition
  to be `satisfied: true` with evidence. The conditions are genuinely open:
  1. live fixture-bound Ship halt-row sessions not run;
  2. the Orchestrator dry routing pass not run;
  3. fixture directories under `logs/af1e5075-fixture/` await operator approval for removal.
* The remaining scope (193-S, 194-S, 184-S, 185-S) is blocked by this edge or by 193-S/194-S.
  184-S needs 194-S; 185-S needs 184-S and 194-S; 194-S needs 193-S.
* Not done: marking those conditions satisfied without evidence would falsify a gate record, and
  item 3 is a deletion that needs operator approval.

## Operator options

1. Decide conditions 1 and 2 (the live Ship runs 188-S to 192-S exercised the proceed path),
   approve removing the fixtures, then amend the 186-S closure frontmatter with a `conditions:` list
   (`satisfied: true` plus evidence per entry).
2. Or accept the risk explicitly and run 193-S under an operator-recorded waiver.

Then re-run dark mode for 193-S -> 194-S -> 184-S -> 185-S.
