---
chunk_strategy: h1-h2
description: "Stage cycle-23 bounded prompt/policy remediation: the harness-exempt consumer contract split into static intake and a claim-time gate, and made executable with a must-fail-before-deliverable probe on all ten exempt tasks"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-23 harness-exempt execution contract memory
---

## Session frame

* Agent: Stage (prompt/policy and planning artifacts only; no Go source, no Go test, no production
  Go, no build loop, no push, no merge, no shipment claim, no subagent delegation)
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; HEAD at session start
  `e8b974e819bfb3f536570dc8914bbf363f697998`
* Tooling: worktree-bound `go run ./cmd/backlogit --cwd .` CLI, `markdownlint-cli2@0.23.1` via
  `scripts/md-lint.ps1`, read-only `go test ./tests/integration/`, `go doc` and `go test -run`
  probes for command validation (MCP not used)

## Baseline observed before any edit

| Measure | Value |
|---|---|
| Queued tasks under `147-F` | 42 |
| Carrying `harness-ready` | 0 |
| Carrying `harness-exempt` | 10 |
| Exempt tasks carrying an executable verification command | 0 |
| Exempt tasks carrying machine-readable class/reason/owner metadata | 0 (prose only) |

Two independent P1 defects were present in the cycle-22 contract.

**Deadlock.** P-002.1 required the `covered-by` owner's red evidence at Ship Step 2a, which runs
*before* harness generation. The evidence cannot exist there, so `147.036-T` / U13 halts with
`EXEMPT_OWNER_NOT_RED` on every run and shipment `130-S` deadlocks on its own gate.

**False green.** Exempt dispatch could pass with no work at all. Verified directly at baseline:

```text
> go test -count=1 -run '^TestU1dGuard_' ./internal/events
ok   github.com/softwaresalt/backlogit/internal/events  1.735s [no tests to run]
exit=0
```

The `declaration-only` "compile check" likewise passes before the declaration exists, and
`147.019-T`, `147.026-T`, `147.041-T` carried no executable command at all.

## What this session did

### Authoritative policy — `.github/policies/workflow-policies.md`

* **P-002** `Applies To` corrected to name all three enforcing agents with their gates: `ship`
  (queue consumer and claim-time gate), `harness-architect` skill (producer / no-scaffold
  partition), `build-feature` skill (dispatch consumer and pre-work precondition probe). `Gate
  Point` names static intake, claim time, and the pre-work probe, reconciling the stale
  "task claiming (Step 3)" wording with Ship's actual step numbering.
* **P-002.1** split into **static intake** and **claim time**. Static intake validates fields,
  class, reason, closed-contract membership, owner existence / dependency edge / non-exempt type,
  and the declared commands. Claim time owns the owner's `harness-ready` label, `Compilation: PASS`
  / `Red Phase: CONFIRMED`, and landed harness commit. A "why the split is load-bearing" paragraph
  records the deadlock so the condition is not moved back.
* **P-002.1** now defines the canonical body-metadata contract block: five keys in fixed order
  (`harness_exemption_class`, `harness_exemption_reason`, `harness_owner`,
  `exempt_verification_command`, `exempt_precondition`) inside
  `<!-- BEGIN:harness-exemption-contract -->` delimiters, plus `harness_owner_command` appended for
  `covered-by`. Body metadata rather than frontmatter because `.backlogit/header-def.yaml` declares
  a closed per-type field set that would reject or drop the keys. The class token is bare
  `covered-by`; the owner lives in `harness_owner`.
* **P-002.2** gains the owning gate per code and five new codes:
  `EXEMPT_CONTRACT_INCOMPLETE`, `EXEMPT_COMMAND_MISSING`, `EXEMPT_FALSE_GREEN`,
  `EXEMPT_EVIDENCE_MISMATCH`, `EXEMPT_DELTA_EXCEEDS_CLASS`. `EXEMPT_OWNER_NOT_RED` is explicitly
  marked claim-time-only, with a note that raising it at intake is a false halt.
* **P-002.3 (new)** — Exempt Execution Contract. Mandatory pre-work probe run exactly once; a
  pre-work success is `EXEMPT_FALSE_GREEN`. A false-green signal table makes `[no tests to run]`,
  `testing: warning: no tests to run`, `no test files`, and zero named `--- PASS:` lines failures
  regardless of exit status. Per-class command authoring rules require `declaration-only` to probe
  the declared symbol and `docs-only` to probe required content before linting. Post-deliverable
  the command must pass **and** match declared evidence, else `EXEMPT_EVIDENCE_MISMATCH`. Loop
  command versus gate command is defined for `covered-by`.
* **P-002.4 (new)** — Objective Behavior Classification. `covered-by` is the only exempt class that
  may modify production behavior. A per-class changed-file delta surface is checked at the
  completion gate against `git diff --name-only`, failing closed on any unclassifiable delta.
  Behavior-preserving wrapper re-expression is admitted under `declaration-only` only when the
  command pins the pre-existing contract.
* **P-004** relationship extended: vacuous satisfaction of P-004 does not remove the observed
  failure; the exempt task still owes the P-002.3 probe.
* Amendment log entry `1.16.0`.

### Consumers

* `.github/agents/.ship.agent.md` — Step 2a retitled *Harness-Exempt Static Intake* and forbidden
  from evaluating owner red evidence; Step 2 partition notes that a `covered-by` owner belongs in
  the "needs harness" set; **new Step 4.1a** claim-time gate (contract re-read, owner red evidence,
  must-fail-before-deliverable probe); Step 3 states that queue admission is not build admission;
  Step 4.2 passes `harness_cmd` / `exempt_gate_cmd` / `exempt_class`; Step 4.3 adds the exempt
  completion gate with the delta-surface check.
* `.github/skills/harness-architect/SKILL.md` — Step 1a is static intake only, enumerates the
  contract block, must not raise `EXEMPT_OWNER_NOT_RED`, and must **scaffold** `covered-by` owners
  rather than excluding them; Step 1, Step 6, completion criteria, and guardrails updated to match.
* `.github/skills/build-feature/SKILL.md` — new **Step 0** pre-work precondition probe; `Inputs`
  gain `exempt_gate_cmd` and `exempt_class`; Step 2 parses exit-0 false greens as failed attempts;
  **Step 5 narrow exception** lets `verification-only` and `declaration-only` deliverables create
  the new test/evidence files they name while still forbidding editing, deleting, relaxing,
  skipping, build-tagging, or narrowing the selector of any pre-existing assertion; post-loop
  exempt completion gate added; constraints and quality criteria updated.
* `.autoharness/drift-ignore` — cycle-23 note enumerating the corrected contract per file so a
  future autoharness template adoption re-applies the 1.16.0 form.

### The ten exempt tasks

Every task gained a `harness-exemption-contract` block with identical key grammar. All ten commands
were extracted from the task files and executed at HEAD `e8b974e`; all ten exited **1**.

| Task | Unit | Class | Gate shape | Baseline |
|---|---|---|---|---|
| `147.032-T` | U1d | `declaration-only` | `go doc` symbol + tag probe, then 3 named `--- PASS: TestU1dGuard_` | exit 1 — `no symbol RemediationIntent` |
| `147.038-T` | U15 | `declaration-only` | `go doc` type + func probe, then 2 named `--- PASS: TestU15Guard_` | exit 1 — `no symbol CheckpointReadResult` |
| `147.036-T` | U13 | `covered-by` (`147.035-T`) | seam-file precondition-set probe, then 3 named `--- PASS: TestU12_` | exit 1 — seam file absent, selector `[no tests to run]` |
| `147.007-T` | U3b | `verification-only` | named test file exists, then 2 named `--- PASS: TestU3bGuard_` | exit 1 — file absent |
| `147.021-T` | U2f | `verification-only` | named test file exists, then 2 named `--- PASS: TestU2fGuard_` | exit 1 — file absent |
| `147.017-T` | U9 | `docs-only` | 4 required design-doc strings present, `disjoint by design` absent, then `docs lint` | exit 1 — claim still present, markers absent |
| `147.018-T` | U9b | `docs-only` | 6 required instruction-file markers present, then `docs lint` | exit 1 — markers absent |
| `147.019-T` | U10 | `verification-only` | >= 5 `evidence_row: unit=U10` + 2 scalars | exit 1 — closure artifact absent |
| `147.026-T` | U10b | `verification-only` | >= 5 `evidence_row: unit=U10b` + 3 scalars | exit 1 — closure artifact absent |
| `147.041-T` | U10c | `verification-only` | >= 3 `evidence_row: unit=U10c` + 2 scalars | exit 1 — closure artifact absent |

`147.036-T`'s `harness_owner_command` (the U12 selector, the loop command) also exited 1.

### Durable runtime evidence manifest

The three runtime-verification units share one tracked, human-readable artifact under the existing
`docs/closure/` convention: `docs/closure/2026-08-checkpoint-disposition-runtime-verification.md`
(already named by U10 and U10c). Records use a fixed line grammar at column 0:

```text
evidence_row: unit=<U10|U10b|U10c> row=<n> filename=<bare-filename> sha256=<64-hex> state=<state> destination=<bare-destination|none> outcome=<refused|accepted|unchanged>
evidence_scalar: unit=<unit> <key>=<value>
```

Probes require a minimum row count plus the unit's declared scalars, so an absent, empty, or
partially-populated artifact fails and only a complete run passes. The git-ignored scratch
directory remains a working area, not the record.

## Command grammar and why it is falsifiable in both directions

Commands are recorded as `pwsh -NoProfile -Command '<statements>'` with **single** outer quotes, so
a PowerShell parent passes them verbatim without interpolating `$f` / `$o` / `$LASTEXITCODE`. The
grammar was proven in both directions before adoption:

* negative — `go doc ./internal/events RemediationIntent` exits 1 with `no symbol …`
* positive — the identical shape against an existing symbol and an existing selector
  (`CheckpointSummary` + `^TestCheckpointV1_`, >= 3 named PASS lines) exits 0
* the evidence-row and evidence-scalar regexes were exercised against an in-memory record set and
  matched 5 rows plus both scalars

No file was created to prove the passing direction, so no deliverable was fabricated.

## No fabricated REDs

No test was invented for any declaration-only or docs-only unit. Their observed failure is the
declared `exempt_verification_command` against the pre-work tree — a real probe of the declared
symbol or the required content, not a build error and not a manufactured assertion.

## Gate status — unchanged

Cycle 23 is **remediation evidence, not a gate**. No `## Plan Review` record was appended, no
`PASS` is claimed, and the current gate state remains `cycle: 21` `FAIL` with
`restage_recommendation: confirmatory-review-of-cycle-21-fixes`. The plan carries a
"PR #377 plan remediation, cycle 23" appendix under the cycle-21 record that says so explicitly.

## Topology (unchanged, re-verified independently)

Recomputed from the queue frontmatter rather than from the index, so the check does not depend on
`sync` having run:

* 42 queued tasks under `147-F`, 104 queued-to-queued executable edges, 43 shipment members
* Ready roots unchanged: `{147.001-T, 147.032-T}`
* Kahn topological sort: 42/42 ordered, **acyclic**
* Exempt set unchanged: exactly the ten enumerated members
* No dependency edge, task count, shipment membership, or unit definition changed

## Validation

| Gate | Result |
|---|---|
| Contract-block key presence and order, all 10 tasks | 10/10 present, order exact |
| All 10 `exempt_verification_command` executed at `e8b974e` | 10/10 exit 1 (declared baseline) |
| `147.036-T` `harness_owner_command` executed | exit 1 |
| Command grammar positive control (existing symbol + existing selector) | exit 0 |
| Evidence-row / evidence-scalar regex positive control (in-memory) | 5 rows + 2 scalars matched |
| `sync` | 1208 artifacts indexed, 0 parse failures |
| `doctor` | 23 issues, all pre-existing orphans (`106.0xx-T`, `016.001-R`) outside `147-F`; 0 new |
| `docs lint` | `valid: true, violation_count: 0` |
| `scripts/md-lint.ps1` (markdownlint-cli2 0.23.1, repo-wide) | 2285 files, 0 issues, exit 0 |
| `go test ./tests/integration/ -count=1` | `ok` — existing structural prompt/plugin/CI guards pass |
| Frontmatter delimiter + key syntax on 49 changed/related artifacts | 0 failures |
| Prompt-artifact structure (frontmatter keys, required sections) | policy, ship agent, both skills conform |
| Class vocabulary consistency across policy / ship / both skills | all four tokens present in all four files |
| Halt-code cross-references | all 10 codes referenced in policy and consumers |
| Cross-reference existence on paths added by this pass | 17 referenced; 11 exist; 6 absent **by design** (the not-yet-created deliverables the probes target) |
| Topology: tasks / edges / members / roots / exempt | 42 / 104 / 43 / `{147.001-T, 147.032-T}` / 10 |

## Open questions and next steps

1. **The independent confirmation review is still outstanding and now wider.** It must cover the
   cycle-21, cycle-22, and cycle-23 changes together. The gate stays `FAIL` until it runs.
2. Only after that review should the branch be pushed and PR #377 reconciled.
3. No push, merge approval, or shipment claim was performed, and no subagent was invoked.
4. **Blocker recorded, not fixed**: `147.036-T` and `147.041-T` carry no `acceptance-criteria`
   block at all, which is a P-003 gap (every task needs at least one acceptance criterion). Their
   deliverables are declared in body prose and, for `147.041-T`, in the cycle-23 evidence-manifest
   requirement, so neither is undefined — but closing the gap means authoring acceptance criteria,
   which is outside a contract-correction pass. Recorded in the plan's cycle-23 appendix.
5. **Deferred by scope (carried from cycle 22)**: the repository's prompt/template validation tests
   are Go, and this session excluded Go source, so no structural test asserts P-002.1/P-002.3/
   P-002.4 or the contract-block grammar. A follow-up should add a guard that every
   `harness-exempt` task carries the five keys in order with a non-empty command, and that the
   class vocabulary matches across policy, agent, and skills.
6. `plugin/agents/ship.agent.md` and `plugin/skills/` were again deliberately not modified — the
   frozen standalone bundle carries no P-002 vocabulary at all and is not a coupled surface.
