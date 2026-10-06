# Stage Memory: 196-S Amendment 2 (Harness Lifecycle Contract)

* **Date:** 2026-10-04
* **Agent:** Stage. The Orchestrator delegated this under P-017 dark mode
  (`scope=[196-S]`, route claude-opus-5.5 / anthropic / high).
* **Branch:** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`.
  The base was HEAD `82ee69e8`. This is the single active worktree (P-016).
* **Trigger:** Ship governed-BLOCKED 196-S at wave-1 admission with
  `P-002.6_TASK_SCOPED_COMMAND_CONFLICT`
  (`docs/memory/2026-10-04/ship-196-S-p0026-task-command-contract-halt.md`).
* **Plan:** `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, sections
  `## Amendment 2: Harness Lifecycle Contract` and the final `## Plan Review`.

## Decisions

1. **The test unit token is `SR<n>`, not the literal `U<n>`.** `^TestU4_` in `./internal/core`
   and `^TestU7_` in `./internal/mcp` already match functions that existed before this work
   (`checkpoint_disposition_test.go`), which breaks P-002.6 rules 4 and 5. The new prefixes
   are `^TestUSR<n>_`, following the 195-S `TestUCS<n>_` precedent. Subtest names are fixed.
2. **196.004-T and 196.007-T are `harness-exempt` with class `verification-only`.** They are
   characterization tests that are green on arrival. Each command checks that the file exists,
   then runs the anchored selector. It rejects a vacuous run and asserts the named top-level and
   subtest PASS lines with no FAIL or SKIP. The marker is the last statement. P-021 halt
   semantics are preserved.
3. **196.002-T, 196.005-T, and 196.009-T are `harness-exempt` with class `covered-by`.** Their
   owners are 196.001-T, 196.003-T, and 196.008-T, following the 195-S precedent. Without this,
   waves 2 and 3 would have halted the same way.
4. **196.006-T (U6) stays harness-required.** The manifest is config, and no exempt class
   admits it. A `TestUSR6_HarnessManifestDriftRecords` spec is scaffolded by harness-architect
   at wave 4, following the 174.076-T precedent. The checksum rule is corrected to
   LF-normalized content, which is the manifest's convention.
5. **The closed exempt set is {196.002-T, 196.004-T, 196.005-T, 196.007-T, 196.009-T}.** It
   is declared in the plan. 196-F does not enumerate the harness lifecycle.
6. **196.001-T, 196.003-T, and 196.008-T get no `harness-ready` label now.** Their green-maker
   data is unchanged (closes waves 2, 2, and 3). Only `red_selector_command` changed.

## Validation

* All five exempt commands were run before any work, and all exited 1:
  * 004 and 007: file missing.
  * 002, 005, and 009: vacuous run.
* The 004 command's positive and negative paths were proven in a throwaway module outside
  the repository.
* `backlogit sync` and `backlogit_sync_index` indexed 1886 artifacts with 0 parse failures.
* `backlogit doctor` reported only orphans that existed before this work (106.\*,
  016.001-R).
* markdownlint 0.23.1 found no issues.
* `go test -count=1 ./tests/integration/...` passed.

## Reviews

* **Correctness persona:** ADVISORY.
* **Scope Boundary persona:** ADVISORY.
* **Adversarial Review (multi-model):** ADVISORY.
* **Findings:** no P0 or P1. The P2 and P3 findings were fixed in the body, except for the
  two residuals below.
* **Final gate:** `decision: ADVISORY`, with `operator_authorization: approved` under
  dark-mode provenance.

## Residuals (for the Orchestrator)

* **P-002.4 completion gate.** The gate's `git diff --name-only` does not list an untracked
  new `*_test.go`, so Ship must stage the U4 and U7 files before running it. A policy or
  skill fix would be outside 196-S.
* **Task titles keep `U4:` and `U7:`.** The selector must be read from the task contract,
  not derived from the title.
* **Ship resume.** Ship must re-derive its Step 3 mapping and red-deliverable freeze from the
  amended contracts. Unblocking is Ship's or the Orchestrator's action. Stage did not touch
  the block envelope, statuses, checkpoints, or ops records.

## Next step

The Orchestrator routes 196-S back to Ship to unblock and re-admit wave 1.
