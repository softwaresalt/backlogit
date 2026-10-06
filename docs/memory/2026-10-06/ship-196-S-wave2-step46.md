# Ship 196-S Wave-2 Step 4.6 Convergence Gate (P-002.6)

Recorded at HEAD `e64d07d9` (Phase 2 item 21, operator ruling A).

## P-005 closing-wave drift (A3.5)

The closing-wave drift event was emitted through `backlogit_log_telemetry` before this gate:

- Declared `green_maker_closes_wave` values are 001 = 2, 003 = 2, 008 = 3.
- Under ruling A the effective closing waves are 3, 3, and 4.

## Gate record

1. Converged:
   - Wave 2 is {196.001-T, 196.003-T}. Both are `done`, with done-gate records at
     `c7b7c57f` and `24469868`. Both records postdate the Option A window and descend from
     reopen commit `7bc74325`.
   - No member of `ready_2` is `active`, `blocked`, or in an unsupported status.
   - 196.002-T, 196.005-T, 196.006-T, and 196.009-T remain claim-assigned and unstarted.
2. Always-on gates:
   - `go test -run=^$ -count=1 ./...`: PASS.
   - `go vet ./...`: PASS.
   - golangci-lint v1.64.8 (`GOTOOLCHAIN=local`, `--concurrency=1`, full run): PASS.
   - gofmt on a true-LF export of HEAD (880 `.go` blobs): 0 paths, 0 CR files.
   - Declared scoped commands: both wave members are red deliverables, so each
     `red_selector_command` must be RED, and both were RED. Green-regression arrays are
     `[]`.
   - Open-red selectors were each observed assertion RED. None was vacuous, and none
     panicked or hit a guard.
     - `^TestUSR1_`: exit 1. ProcedureLiterals FAIL, CallSites FAIL,
       CrossReferenceInvariant PASS.
     - `^TestUSR3_`: exit 1. SkillLiterals, ProceedAndShipStep6, and
       SupersededAndPreserved all FAIL.
     - `^TestUSR8_`: exit 1. DefinitionLiterals FAIL, CallSites FAIL,
       PreservedInvariants PASS.
   - `newly_closed_2` is empty.
3. Full suite: `FULL_SUITE_DEFERRED: wave 2`.

   | Open selector | Owner | Green-maker (not done) | Declared / effective closing wave |
   |---|---|---|---|
   | `^TestUSR1_` | 196.001-T | 196.002-T (wave 3) | 2 / 3 |
   | `^TestUSR3_` | 196.003-T | 196.005-T (wave 3) | 2 / 3 |
   | `^TestUSR8_` | 196.008-T | 196.009-T (wave 4) | 3 / 4 |

   Reason for the deferral:
   - Compile, vet, lint, format, and every declared scoped command have run and passed.
   - Every still-open red-deliverable selector was re-confirmed RED.
   - The only failing selectors are the declared open-red deliverables, whose green-makers
     are scheduled at waves 3 and 4.
   - A full run here would be classified rather than verified.
4. Deferral budget: wave index 2 does not exceed any declared closing wave, so no
   `WAVE_OPEN_RED_UNCLOSED`.

Phase 2 is complete. The next step is Phase 3, wave 3: 196.002-T and 196.005-T.
