---
schema_version: "1.0"
doc_type: memory
title: Stage 197-S red-deliverable contract amendment
description: Stage authored the seven canonical red-deliverable contracts and the P-002.1 exemption contracts for 197-S so Ship Step 3 admits the wave schedule. It formalizes the reviewed plan without changing scope. Two members (197.016-T, 197.019-T) are harness-required under the Orchestrator decision, the selector prefixes follow the P-002.6 requirement 4 TestU convention, and Erratum E1 records the postures in the plan. A final section records the operator-ruled descoping of U15 (197.016-T), the 18-task manifest, and Erratum E2.
timestamp: "2026-10-10T05:57:50.0000000Z"
---

# Stage 197-S red-deliverable contract amendment

## Status

**Contracts amended; the U15 and U17 postures are resolved (Orchestrator decision, P-017).**
Scope `[197-S]` only. **Superseded in part by the final section ("U15 descoped by operator
ruling"): U15 (197.016-T) is no longer in 197-S, so `S` is 197-F plus 18 tasks and `M` is 18
tasks.** The text below is the historical record of the earlier passes. This note formalizes the already-reviewed plan
`docs/exec-plans/2026-10-08-cx-s-ship-closure-gate-correctness-plan.md`. It does **not** change
scope, shipment membership `S` (197-F plus the 19 tasks), the task set `M` (the 19 tasks),
dependencies, waves, acceptance criteria, task titles, or statuses. The only label change is that
`harness-exempt` was added to ten tasks (P-002.1 item 1).
No git operation was run. Edits are working-tree only, on Ship's feature branch, for Ship to
commit. Later passes appended Amendment A1 to 197.016-T and Erratum E1 to the plan, renamed the
test selector prefixes (P-002.6 requirement 4), and appended Amendments A2 (197.019-T) and A3
(197.017-T); all are recorded below.

## The gap

Ship halted at Step 3 with `WAVE_RED_MAPPING_UNRESOLVED` because seven members whose deliverable
is a red harness carried no canonical `red-deliverable-contract` block. The same pass found no
`harness-exempt` metadata on any member, and no closed exempt set that P-002.1 requires. Ship
cannot author planning fields, so Stage supplied them.

## Validated red-to-green mapping

The wave partition was re-derived from `item_deps` (18 `blocks` edges, identical to the task
frontmatter): W1 = 001-008 and 017; W2 = 009, 011-016; W3 = 010, 018; W4 = 019.

| Red deliverable | Green-maker(s) | Green-maker wave | `green_maker_closes_wave` | Plan citation |
|---|---|---|---|---|
| 197.001-T (U1, `TestUCXS1_`) | 197.009-T (U8) | 2 | 2 | U1; U8 AC1; Dependency Graph `U8 <- U1, U1b` |
| 197.002-T (U1b, `TestUSR3_`) | 197.009-T (U8) | 2 | 2 | U1b AC1; U8 AC1 |
| 197.003-T (U2, `TestUCXS2_`) | 197.010-T (U9) | 3 | 3 | U2; U9 AC1; `U9 <- U2, U8` |
| 197.004-T (U3, `TestUCXS3_` + `TestUSR1_`) | 197.011-T (U10) | 2 | 2 | U3 AC1; U10 AC1; `U10 <- U3` |
| 197.005-T (U4, `TestUCXS4_`) | 197.012-T, 197.013-T, 197.014-T (U11-U13) | 2 | 2 | U4; U11-U13 AC1; `U11/U12/U13 <- U4` |
| 197.006-T (U5, `TestUCXS6_NormalizeClosureGateKeysStayTopLevel`) | 197.015-T (U14) | 2 | 2 | U5; U14 AC1; `U14 <- U5` |
| 197.017-T (U16a) | 197.018-T (U16b) | 3 | 3 | U16a; U16b AC1; `U16b <- U15, U16a` |

Ship's offered mapping was confirmed exactly. The plan and the dependency graph agree, and every
green-maker is in `M`, is not the task itself, and lands in a strictly later wave. The close
wave equals the real latest green-maker wave in all seven.

Three declarations go beyond the plan's literal text, and they are flagged for review:

* **Selector prefix rename (P-002.6 requirement 4).** The plan's test names `TestCXS1_` to
  `TestCXS4_` and `TestNormalize_ClosureGateKeysStayTopLevel` do not start with `TestU`.
  `.github/policies/workflow-policies.md` (P-002.6 requirement 4), `build-feature` Step 0.5, and
  `_ship.agent.md` require every task-scoped harness command to be anchored to
  `-run '^TestU<unit>_'`, and `Test-TaskScopedCommandShape` in `scripts/wave-scheduler-sim.ps1`
  enforces it with `-run\s+'?\^TestU`. Ship would halt `WAVE_RED_MAPPING_UNRESOLVED` at wave-1
  dispatch of all seven red deliverables, so the policy wins over the plan's names. The unit-token
  map is U1=CXS1, U2=CXS2, U3=CXS3, U4=CXS4, U16a=CXS5, U5=CXS6 (package `internal/docline`), and
  the prefixes are `TestUCXS1_` to `TestUCXS5_` and
  `TestUCXS6_NormalizeClosureGateKeysStayTopLevel`. `TestUSR1_`, `TestUSR3_`, and `TestUSR6_` are
  unchanged. Union selectors keep the anchor: `'^TestU(CXS3_|SR1_)'` and `'^TestU(CXS1_|SR3_)'`.
  The 197.* task bodies carry the new names, with a one-line "Name note" on the five tasks whose
  plan name changed; Erratum E1 documents the map, and no existing plan line was edited.
* **`TestUCXS5_` (197.017-T).** The plan leaves the U16a test name to the implementer, but
  `red_selector_command` must be exact. Stage declared the next collision-free unit-token prefix
  (`TestUCXS1_`-`TestUCXS4_` are used; `TestUCXS5_` is unused) and the package `./internal/cli`
  (where the `shipment ship` command and its tests live). It declares no test body or scope.
  Amendment A3 on 197.017-T states that a differently named test would match nothing under the
  declared selector (`WAVE_RED_DELIVERABLE_VACUOUS`) and supersedes "implementer names it".
* **197.009-T owner command.** U8 turns two red selectors green, so its `harness_owner_command`
  is the combined `^TestU(CXS1_|SR3_)`. The grammar allows one `harness_owner` ID, so
  `197.001-T` is named and `197.002-T` is recorded in the reason as co-owner.

## Exemption decisions (P-002.1 / P-002.4)

Only `harness-exempt` was added as a label, on ten tasks (P-002.1 item 1). No `harness-ready`
was added, and no class labels or `red-deliverable` labels. The closed exempt set, which
P-002.1 item 3 requires and the plan does not enumerate, is recorded on `197-F`
(`harness-exempt-set` block). It is exactly the ten tasks below.

| Task | Class | Owner | P-002.1 rationale |
|---|---|---|---|
| 197.007-T (U6) | `verification-only` | none | Adds only the new `*_test.go` benchmark for already-shipped behavior. It is green by design (a benchmark cannot be failing-first) and adds zero non-test Go. Gate: file must exist and a named benchmark result line must print. |
| 197.008-T (U7) | `docs-only` | none | One new markdown file; no `*.go`. Gate probes the required content (top-level `closure_status`/`compaction_status`, `gate_registration`, the migrate warning, the narrative path, `#436`, the merge commit), then runs `backlogit docs lint`. |
| 197.009-T (U8) | `covered-by` | 197.001-T (co-owner 197.002-T) | Edits only `_ship.agent.md`; owners are red deliverables, direct dependencies, not exempt. |
| 197.010-T (U9) | `covered-by` | 197.003-T | Edits only `_ship.agent.md`; `TestUCXS1_` is also required to stay green (U9 AC1). |
| 197.011-T (U10) | `covered-by` | 197.004-T | Edits only `_orchestrator.agent.md`. |
| 197.012-T (U11) | `covered-by` | 197.005-T | PowerShell lock scripts only. |
| 197.013-T (U12) | `covered-by` | 197.005-T | Bash lock scripts only. |
| 197.014-T (U13) | `covered-by` | 197.005-T | Two documentation files only. |
| 197.015-T (U14) | `covered-by` | 197.006-T | Two `internal/docline` production files; also requires the package suite green (U14 AC1). |
| 197.018-T (U16b) | `covered-by` | 197.017-T | Production code only; the owner is its red deliverable. |

Why the green-makers are `covered-by` and not left to the harness-architect: their failing
harness is the predecessor's red deliverable. P-002 requires every task to be `harness-ready` or
P-002.1-valid exempt, and `covered-by` is the only P-002.1 form that expresses a predecessor-owned
red. Scaffolding a second, substitute harness would duplicate the owner's. This follows the 196-S
precedent (196.002-T, 196.005-T, 196.009-T; 196 plan Amendment 2, "Why U2, U5, and U9 are
`covered-by`"). Each `covered-by` block carries the sixth key `harness_owner_command`, which is the
owner's exact selector and carries no marker (P-002.3).

Fan-out note (U11-U13): `TestUCXS4_` turns fully green only after all three land, and the plan
names no subtests. Each gate therefore pins its own deliverable by content probe (`.agent-lock`
present, old `$lockFile`/`LOCKFILE` assignment gone) and requires the owner run to be non-vacuous,
free of `SKIP`, and free of failing rows that name its own files. The last of the three to land is
checked against the whole owner at the wave 2 convergence gate (`newly_closed`). The three
gates filter `--- FAIL:` rows by script or document path, so they depend on the `TestUCXS4_`
subtests being named after the file rows. 197.005-T carries one line requiring that naming so its
owner pins it; the gates were not reworked, and the Step 4.6 `newly_closed` GREEN check is the
backstop. The 197.018-T gate has no path filter. It requires a non-vacuous named
`TestUCXS5_` PASS row.

### Not admitted: 197.016-T (U15) and 197.019-T (U17)

Neither task was given an exemption. Both stay outside the closed set, with no `harness-exempt`
label and no contract. Their prose "harness: exempt" and "verification-only" are inert. The
analysis below was the original finding. The Orchestrator decision section that follows resolves
the open U15 item.

* **197.016-T (U15), blocker for wave 2.** It changes `internal/core/shipment_gate.go`, which is
  behavior-changing production code. Under P-002.4, `docs-only` and `verification-only` forbid
  non-test `*.go`, so only `covered-by` could admit it. That needs a harness owner that is in `M`, a
  declared dependency, not exempt, and red at claim time (`Compilation: PASS`, `Red Phase:
  CONFIRMED`). Its only dependency is 197.007-T, which is itself exempt (`EXEMPT_OWNER_INVALID`) and
  whose benchmark is green by design. The plan's coverage claim ("the U6 benchmark and the existing
  shipment gate tests") names a benchmark and green tests, not a red owner. Its Constitution Check
  calls this a "justified deviation", which P-002.1 (cycle 29) withdrew as a Principle II carve-out.
  197.017-T is not a dependency of 197.016-T (the edge is 016 to 018), covers progress output rather
  than this refactor, and the graph may not be changed here. **Result:** at wave 2 Ship treats U15
  as "needs harness". The harness-architect must produce a genuine red harness (P-002.1 allows a
  source-shape harness), or Ship halts at Step 2 item 6 with a reported gap, and P-002.6 forbids a
  partial wave. **Stage decision needed before wave 2** (outside this invocation): a reviewed
  amendment making U15 harness-required with a real red harness, or adding a red-owner dependency
  through a manifest amendment.
* **197.019-T (U17), no Step 3 blocker.** `verification-only` forbids any repository-configuration
  file (P-002.4), and `.autoharness/harness-manifest.yaml` is one. The 196 plan Amendment 2, "Why
  U6 stays harness-required", reached the same conclusion for the same file. An exemption would
  halt at the completion gate with `EXEMPT_DELTA_EXCEEDS_CLASS`. It stays on the default
  harness-required path: the harness-architect scaffolds its harness at the start of wave 4. The
  RED shape follows the 196 drift-record test (see Amendment A2 below): it is red at wave-4 start
  because the recorded checksums and drift reasons are stale, and green after the refresh.

## Orchestrator decision (P-017, 2026-10-10): U15 and U17 are harness-required

The Orchestrator chose the conservative, stricter-than-plan resolution. Stage executed it. No
dependency edge, task, manifest change, or label was added, and `M` and the wave partition are
unchanged.

* **197.016-T (U15) is not exempt.** P-002.1 withdrew the plan's "justified deviation"
  (a prose-only exemption), and U6 (197.007-T) is exempt, so it cannot own U15's harness. U15 is
  an ordinary harness-required task. It carries neither `harness-exempt` nor `harness-ready`
  now. Ship's harness-architect scaffolds its RED harness at the start of wave 2 (Step 2), after
  U6 (wave 1) is `done` and its verdict, ns/op, profile, and target are recorded in the U6 task
  notes. This closes the open blocker above, because the "Stage decision needed" is now made.
* **Amendment A1** was appended to the body of 197.016-T, preserving every existing line
  including the AC. It withdraws the prose "Harness: exempt", states that U15 is
  harness-required, and gives the RED constraints: fail on main for the right reason, pass only
  after the refactor, a failing condition derived from U6's verdict and target, task-scoped, and
  the six P-002.6 requirements. The shape is the harness-architect's choice. The three shapes
  named there (a work-count characterization test, a U6-derived performance budget, and a
  `go/parser`/`go/ast` source-shape assertion) are examples, not a closed list.
  A characterization guard keeps every existing shipment gate test, verdict, error
  classification, and lock order unchanged. A REFUTED U6 verdict, a missed target, or no valid RED
  harness means HALT and return to Stage, with no exemption and no substitute harness. The
  deliverable, files, AC, and dependencies are unchanged. The wording was later tightened, see
  "Review-round fixes" below.
* **197.019-T (U17) stays harness-required.** It edits repository configuration
  (`.autoharness/harness-manifest.yaml`), which no exempt class admits. The harness-architect
  scaffolds its RED harness in wave 4. Amendment A2 (below) fixes the RED shape.
* Confirmed: neither 197.016-T nor 197.019-T carries an exemption label, a `harness-ready`
  label, or any exemption, red-deliverable, or green-regression contract block. The closed exempt
  set on 197-F still lists exactly the ten tasks above and names both as deliberate non-members.

## Review-round fixes (P-017, 2026-10-10)

Three independent adversarial reviewers (constitution, scope, correctness) returned
APPROVE_WITH_NOTES. Stage applied exactly these fixes. It changed no membership, dependency, title,
status, claim, deliverable file, or acceptance-criteria semantics.

* **Selector prefix (major).** Every `red_selector_command`, every covered-by
  `harness_owner_command`, and every other `-run` selector in the 197.* bodies is now anchored to
  `^TestU`, per the rename described above. Every owner command is byte-identical to its owner's
  red selector, except the documented 197.009-T union. The old names remain only in the one-line
  "Name note" on five tasks and in Erratum E1.
* **Amendment A2 (197.019-T, U17).** U17 stays harness-required. Its RED harness follows the 196
  drift-record shape in `tests/integration/harness_manifest_196_drift_records_test.go`: per entry
  it asserts exactly one entry, `drift_allowed: true`, a 64-hex checksum that differs from the
  pre-refresh value pinned at wave-4 start, and a `drift_reason` that keeps `Do not auto-revert.`
  and the existing citations and cites this release's stash IDs. It must not permanently assert
  that the checksum equals the current file hash, because that would redden every later shipment
  that edits those files. `TestUSR6_HarnessManifestDriftRecords` (AC1) is already green on main, so
  it is a regression guard and not the RED harness. The one-time currency check in AC2 uses the
  manifest's LF-normalized SHA-256 convention (raw `Get-FileHash` on this CRLF checkout does not
  match any manifest entry). No valid RED shape means halt and return to Stage. The earlier
  checksum-equals-hash wording is withdrawn.
* **Amendment A3 (197.017-T).** The declared selector is `-run '^TestUCXS5_'` in package
  `./internal/cli`; it supersedes "implementer names it" for the test name and package.
* **Amendment A1 (197.016-T), tightened.** A work-count RED may observe only through
  `_test.go`-only seams and must add no production seam and no production stub (P-002.1 cycle 31
  forbids a declaration ahead of its harness). A timing or performance budget may only corroborate a
  RED and is never the sole RED condition, because timing is nondeterministic. The three shapes
  are examples, not a closed list. A pointer line to Amendment A1 now follows the stale "Harness:
  exempt" paragraph; the original lines were not deleted.
* **197.008-T (U7) probe and evidence.** The exemption command's content probe now matches the
  gate-required comment even when it is line-wrapped across `# ` comment lines
  (`do not run[\s#]+\S?backlogit\s+docs\s+migrate\s+--apply`). Positive control: a copy of the real
  `docs/closure/136-S-154-F-post-merge-closure.md` with only the id tokens substituted
  (136-S to 140-S, 154-F to 158-F) plus one narrative-citation line (the 136-S file has no
  narrative closure by design) printed `EXEMPT_VERIFY_OK:197.008-T` with exit 0, and the pattern
  matched the wrapped comment in the frontmatter alone, which the old pattern did not. Negative
  control on the real tree, where the deliverable is absent: exit 1,
  `140-S closure registration missing`. An evidence note on 197.008-T records that AC1's gate-run
  output can be observed only after 197-S leaves `active` (the known
  `PRECLAIM_ACTIVE_SHIPMENT_PRESENT`), so that step is satisfied post-closure; no AC text changed.
* **197.007-T (U6).** One sentence states that its verdict, ns/op, top-five profile, and target are
  the input Amendment A1 of 197.016-T requires, and that missing evidence is a wave-2 halt for
  197.016-T. The command is unchanged.

## Erratum E1 (plan)

`## Erratum E1 (2026-10-10)` was appended at the end of the plan, after the final `## Plan Review`
section. It is append-only: no existing plan line changed, and the file diff is 156 added lines and
no changed text when end-of-line whitespace is ignored. The only byte-level difference on an
existing line is the end-of-line that the old final line gained. The plan keeps its CRLF line
endings. It records, without changing scope:

* the P-002.2 red-deliverable contracts and the validated red, green-makers, close-wave table;
* the P-002.1 exemption contracts and the closed exempt set recorded on 197-F;
* the selector prefix rename (P-002.6 requirement 4) with the unit-token map U1=CXS1, U2=CXS2,
  U3=CXS3, U4=CXS4, U16a=CXS5, U5=CXS6, and the `TestUCXS5_` selector declaration for 197.017-T
  with the package `./internal/cli` (Amendment A3);
* the U15 harness posture change, which supersedes the Constitution Check "justified deviation"
  line and the U15 unit line "Harness: exempt", with the tightened Amendment A1 wording;
* the U17 posture (Amendment A2): harness-required because it edits repository configuration, with
  a 196-shape drift-record RED harness scaffolded by the harness-architect at wave-4 start;
* the 197.008-T probe and AC-evidence note, the 197.007-T evidence sentence, and the 197.005-T
  subtest-naming line;
* an explicit statement that shipment membership `S` (197-F plus the 19 tasks), the task set `M`
  (the 19 tasks), dependencies, waves, titles, and acceptance criteria are unchanged.

Plan-review was not re-run, because this documents an already-reviewed plan and is not a scope
change. No P-021 deferred-scope entry was created.

## Validation

* `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue`: `WAVE_SIM_OK: 200/200 assertions PASS
  across 26 scenario(s)`. This replay is bound to the fixture shipment 130-S, so it proves the
  scheduler logic still holds. It does not read 197-S.
* Inline contract check (not committed), reusing the scheduler's own parser functions: `S=20 M=19`,
  excluded `197-F(feature)`, W1-W4 as above, all seven red contracts pass the five-key rule, the
  strictly-later-wave rule, and close-wave equality (rows in the mapping table), no
  `green-regression-contract` blocks (empty default), ten exempt members all in the closed set with
  five or six keys in order, owner-dependency and not-exempt checks, marker last and unique, no
  destructive pattern, and valid inner PowerShell syntax. Result: `CONTRACT_CHECK_OK`.
  Re-run after Amendment A1 and Erratum E1 over all 19 members: waves W1-W4 unchanged, 7 red
  deliverables, 10 exempt members (the closed set on 197-F), and 2 "needs harness" members.
  197.016-T (wave 2) and 197.019-T (wave 4) carry no exemption label, no `harness-ready` label,
  no exemption, red-deliverable, or green-regression block, and are not red deliverables. The
  partition total is 19. Result: `CONTRACT_CHECK_OK`.
* Inline semantic check (not committed), with `go` and `backlogit` shimmed (no real build or test
  ran): 76 scenarios. Every exempt command exits non-zero without its marker on the real
  pre-work tree, exits 0 with its marker on the post-deliverable fixture, and fails closed on a
  missing named PASS line, a `FAIL` or `SKIP` line, a vacuous run, a build failure, or a non-zero
  exit. Result: `SEMANTIC_CHECK_OK`, `failed=0`. Not re-run in the A1/E1 pass, because no
  exemption command or contract changed. The review-round fixes changed exemption commands only by
  test-name tokens (and the 197.008 probe pattern), so the shimmed check was not re-run. Every inner
  command was re-parsed as valid PowerShell, and the 197.008 command was exercised with the positive
  and negative controls above.
* Scheduler replay re-run after A1/E1: `WAVE_SIM_OK: 200/200 assertions PASS across 26
  scenario(s)`.
* Review-round re-validation (inline, not committed): the contract checker was extended with the
  `Test-TaskScopedCommandShape` regex (the sim's own function, extracted by AST) applied to every
  `red_selector_command` (7) and every covered-by `harness_owner_command` (8), with seven negative
  controls that the function rejects (old name, bare `./...`, missing `-count=1`, `-short`,
  unanchored union, `-tags`, `|| true`). Also checked: every owner command byte-identical to its
  owner's red selector (197.009-T equals the documented union `'^TestU(CXS1_|SR3_)'`, whose tokens
  equal the selectors of 197.001-T and 197.002-T), every `-run` selector in the exemption blocks
  anchored to `^TestU`, every named `--- PASS:` token matched by the command's own selector, no
  `TestCXS` or old docline name in any contract block or outside a "Name note" line in
  `.backlogit/queue/197*.md`, close-wave equality, the later-wave rule, and the exempt set on 197-F
  equal to the ten `harness-exempt` labels. Result: `S=20 M=19`,
  `W1=001-008,017 W2=009,011-016 W3=010,018 W4=019`, `CONTRACT_CHECK_OK`.
  `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` again printed `WAVE_SIM_OK: 200/200
  assertions PASS across 26 scenario(s)` (bound to the 130-S fixture).
* Lint after the review-round fixes: `npx markdownlint-cli2@0.23.1` reported 0 issues on the plan
  and this note, and `backlogit docs lint` reported `valid: true`, `violation_count: 0` on both
  paths.
* `backlogit_sync_index` after the review-round fixes: `indexed: 1964`.
* Lint after A1/E1: `npx markdownlint-cli2@0.23.1` (MD001/MD025/MD041) reported 0 issues on the
  plan and this note, and `backlogit docs lint` reported `valid: true` with 0 violations on both
  paths.
* `backlogit_sync_index`: `indexed: 1964`, run again after the A1 edit. The dependency edges are
  unchanged.
* Observation: while 197-S is active, `autoharness gate pipeline-topology --shipment 141-S
  --phase pre_claim` stops at `PRECLAIM_ACTIVE_SHIPMENT_PRESENT`, so U7 AC1 cannot show a
  predecessor verdict until 197-S is no longer active. That is outside the exemption command.

## Resume point

Ship may re-run Step 3. It should pass the red mapping, and wave 1 can be admitted: 197.007-T and
197.008-T are statically admitted exempt, and the seven red deliverables are scaffolded. Wave 2 is
no longer blocked on a Stage decision. At the start of wave 2 (Step 2), after 197.007-T is `done`
and its verdict, ns/op, profile, and target are in the U6 task notes, Ship's harness-architect
scaffolds the RED harness for 197.016-T under the constraints in Amendment A1. If that cannot be
done (REFUTED U6 verdict, missed target, or no valid RED harness), Ship halts and returns to Stage.
In wave 4 the harness-architect scaffolds the 196-shape drift-record RED harness for 197.019-T
(Amendment A2). No
P-021 deferred-scope entry was created.

## U6 target determination: REFUTED-like finding, STOPPED (2026-10-10T05:34:27Z)

Stage was asked to record a numeric U6-derived target for 197.016-T (Amendment A1). The code
shows no work-count reduction exists inside U15's declared scope, so Stage recorded no target and
did not append Amendment A1.1 or Erratum E2. Nothing else changed: 197.016-T, the plan, statuses,
dependencies, and membership are as before. Wave 2 admission stays halted. This is the A1 HALT
class ("no way to form a valid RED harness") and returns to the operator/Stage for a re-plan.

### Work counts from the code (task or subtask member, DB-hit case)

| Scope | `events.ReadAllEvents` | `loadArtifact` | `findArtifact` WalkDir |
|---|---|---|---|
| One `validateMemberGateEvidence` call (what U6 timed) | 1 (`shipment_gate.go:716`) | 1 (`shipment_gate.go:703`) | 0 |
| One `ShipShipment` | 2 (gate runs at `shipment_lifecycle.go:607` and `:627`) | 2 in the gate, plus others | 0 in the gate |

* `loadArtifact` reads the DB first (`shipment.go:1836`) and calls `findArtifact` (WalkDir) only on
  a DB miss (`shipment.go:1844`), then upserts. U6 measured `findArtifact` at 0% for this reason.
* `ReadAllEvents` (`internal/events/reader.go:26`) is one `os.Open` and one scan, with no lock.
* A single gate pass is already at the single-resolution, single-read floor. The redundant share
  of the benchmarked loop is 0%. The 93.8% (80.2% + 13.6%) that U6 attributes to log reads and
  lookups is the essential cost of reading each member once, so AC1's "share attributable to
  per-member lookup and log reads" cannot be met by removing redundant work in this function.
* The only duplicate read is the second gate pass (`shipment_lifecycle.go:627`). It is a
  deliberate re-validation after the artifact locks are taken (comment at `:624`-`:626`). Removing
  it changes verdict timing and lock order and edits `shipment_lifecycle.go`, which breaks U15's
  "do not change verdicts, error classification, or lock order" and its one-file scope.
* A timing-only target is not defensible: the host noise band is 39 to 153 ms/op (3.9x), and no
  work removal exists to justify a percentage.

### Where the per-member cost sits (code-derived, not measured)

U6's 39 to 153 ms over 45 members is about 0.9 to 3.4 ms per member, against the stash D116AF58
report of about 10 s per member with `.backlogit/.locks/itemlog` timestamps. The per-member
lock and walk cost is outside `validateMemberGateEvidence`, in `snapshotShipArtifacts`:
`findArtifact` WalkDir (`shipment_lifecycle.go:238`), `FindArtifactPath` walk (`:246`), and
`LockItemLogCrossProcess` (`:254`), plus `attachCommitToItems` `findArtifact` (`:877`). U6 did not
profile that path, so U6's CONFIRMED verdict is true for its narrow function and does not
establish an optimization for U15.

### Options for the next Stage or operator decision (not executed)

1. Re-plan U15 through Stage: a new profiling spike of the real `ShipShipment` path, then a U15
   that targets the per-member walks and itemlog locks. This changes U15's files
   (`shipment_lifecycle.go`) and needs plan review.
2. Descope U15 from 197-S and keep U16b (`197.018-T`, progress output) by removing its edge to
   197.016-T. This changes membership and dependencies, so it needs an operator decision.
3. A separate task for the 1 MiB scanner buffer in `internal/events/reader.go` (13.0% of the loop
   is `memclrNoHeapPointers`). It is a different file, so it is outside U15.

### Wave 2, 3, and 4 contract check (read-only)

* 197.016-T blocks wave 2 (P-002.6 allows no partial wave), so 197.009-T, 197.011-T to
  197.015-T stay unadmitted. By wave sequencing, wave 3 (`197.010-T`, `197.018-T`) and wave 4
  (`197.019-T`) wait too. `197.018-T` also depends on `197.016-T` directly.
* No other wave 2, 3, or 4 contract needs a wave 1 input that was not recorded. The harness
  owners are the wave 1 red tasks and Ship's Step 3 check passed. The 197.019-T Amendment A2
  harness reference (`tests/integration/harness_manifest_196_drift_records_test.go`) exists.
* The `197.018-T` progress callback is independent of U15's performance outcome.

## U15 descoped by operator ruling; manifest re-frozen at 18 tasks (2026-10-10T05:57:50Z)

Scope `[197-S]` only, P-017 dark factory mode. Stage executed the recommended option exactly as
described in the Orchestrator memory `docs/memory/2026-10-10/orchestrator-197s-wave2-halt-memory.md`
("Decisions needed", option 1) and in the "Options" list above (option 2 there).

### Operator ruling (verbatim)

> "Descope U15 as recommended."

Ruled 2026-10-09T22:47-07:00 (2026-10-10T05:47Z). This is the explicit operator authorization
for a Stage manifest amendment and an explicit re-freeze of `M`. Nothing was claimed, shipped, or
committed. The branch is `feat/cx-ship-closure-protocol-and-gate-correctness`, HEAD `45dea2f8`.

### What changed

| Artifact | Change |
|---|---|
| Stash `76553D8D` (new, kind `task`, priority `high`) | "Profile the real ShipShipment path and optimize per-member snapshot/lock cost (R10 remainder)". Self-contained: provenance (D116AF58, 197-S, descoped U15 by this ruling), U6 evidence (`ReadAllEvents` about 80.2%, `loadArtifact` about 13.6%, `findArtifact` WalkDir 0%, about 0.9 to 3.4 ms per member against about 10 s observed), the unprofiled suspects (`snapshotShipArtifacts`, `LockItemLogCrossProcess`, `attachCommitToItems`, the second gate pass at `:607` and `:627`, the 1 MiB scanner buffer at 13.0%), and the constraint "needs deliberation and a new profiling spike first; keep verdicts, error classification, and lock order". No other stash entry was touched; `D116AF58` stays `harvested`. |
| `197-S` manifest (`.backlogit/queue/197-S.md`) | `197.016-T` removed from `custom_fields.items`. The order of the rest, `queue_position: 100`, status `active`, priority, label `dag-root`, and the `related_to` 196-S link are unchanged. An amendment paragraph was appended to the body; the old "197-F plus exactly 19 atomic tasks" wording is superseded (now 197-F plus 18 tasks). `get_shipment` shows 19 items and 18 task members. |
| Dependency edge | `197.018-T` to `197.016-T` removed with `backlogit_remove_dependency`. `197.018-T` now depends only on `197.017-T`. Amendment A4 appended to the body of `197.018-T`: deliverable, files, and acceptance criteria unchanged. |
| `197.016-T` (U15) | Comment (actor `stage`) recording the ruling, the refuted premise, and the follow-up stash. Then archived with the governed `backlogit_archive_item`: status `archived`, `archived_status: active`, file moved to `.backlogit/archive/197.016-T.md`. No lifecycle hook is configured for `archive_item`, the governed op ran its own lock and shipment-membership checks, and no hook or config was bypassed or edited. The task is not claimed, not in any shipment, and not an active orphan under 197-F. Its own edge to `197.007-T` stays on the archived record. The linked stash `D116AF58` was already `harvested`, so the archive's best-effort stash archival archived nothing. |
| `197-F` | Not edited. One comment (actor `stage`) records the amendment and the follow-up stash ID so the follow-up is traceable from 197-F. Stash IDs cannot be a native semantic-link target, so no link was added; no dependency was added. |
| `197.017-T` (archived, done) | Single-key edit under this authorization: `green_maker_closes_wave: 3` became `2` in the red-deliverable contract block. The diff is one line, every other byte is preserved. |
| Plan | Erratum E2 appended at the end (158 added lines when end-of-line whitespace is ignored, no existing line changed). |

### Recomputed wave partition (18 tasks, acyclic)

* W1 = 197.001-T to 197.008-T and 197.017-T (done)
* W2 = 197.009-T, 197.011-T, 197.012-T, 197.013-T, 197.014-T, 197.015-T, 197.018-T
* W3 = 197.010-T (needs 197.003-T and 197.009-T)
* W4 = 197.019-T (depends on 197.009-T to 197.014-T; the real edge to 197.010-T keeps it in W4)

`197.018-T` moves up from W3 to W2. `197.019-T` never depended on `197.016-T` or `197.018-T`, so
its wave and Amendment A2 are unchanged.

### R10 coverage and the follow-up

R10 ("profiled, fixed, and reports progress") is now covered by profiled (U6, done) plus reports
progress (U16a, U16b). The "fixed" part moves to stash `76553D8D`. Amendment A1 on the archived
`197.016-T` is moot and no A1.1 was written.

### Contract re-validation

* Only one contract changed: `197.017-T` `green_maker_closes_wave` from 3 to 2, because its
  green-maker `197.018-T` now lands in wave 2 (strictly later than wave 1).
* The other six red contracts (`197.001-T` to `197.006-T`) are unchanged and valid: green-makers in
  `M`, strictly later waves, close waves 2, 2, 3, 2, 2, 2 equal to the actual latest green-maker
  waves.
* Ten exemption contracts re-checked (`197.007-T` to `197.015-T` and `197.018-T`): five or six keys
  in order, owners are declared dependencies, red deliverables, and not exempt, the sixth key
  matches the owner's selector (`197.009-T` is the documented union of co-owners `197.001-T` and
  `197.002-T`), the marker is unique and last, and no destructive pattern. The exempt set on 197-F
  is unchanged and equals the ten `harness-exempt` labels. `197.019-T` stays the only
  harness-required member that is neither a red deliverable nor exempt.
* Two prose spots on the archived `197.017-T` were deliberately not edited, because the
  authorization covers only the single key: the `red_deliverable_reason` phrase "in wave 3" and
  the Harness Manifest line in its implementation notes ("green_maker_closes_wave: 3"). They are
  historical, are superseded by Erratum E2, and the scheduler parses only the contract keys.

### Validation

* Inline contract checker (not committed; reuses the scheduler's own `Get-DelimitedContractBlock`,
  `Read-RedDeliverableContract`, and `Test-TaskScopedCommandShape`): `S=19 M=18`, excluded `197-F`,
  acyclic, partition W1 to W4 as above, seven red contracts and ten exemption contracts valid,
  needs-harness set `197.019-T`, no green-regression blocks. Result: `CONTRACT_CHECK_OK`.
* `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue`: `WAVE_SIM_OK: 200/200 assertions PASS
  across 26 scenario(s)` (bound to the 130-S fixture; it proves scheduler logic, not 197-S).
* `backlogit_doctor` (including the partial-mutation advisory check): no finding names any 197.*
  artifact (no orphan, no duplicate ID). Its only findings are pre-existing orphans `016.001-R` and
  `106.012-T` to `106.033-T`, unrelated to this work.
* `item_deps` after the amendment shows 17 edges: 16 among the 18 members, matching the task
  frontmatter the checker read, plus `197.016-T`'s own dependency on `197.007-T` on the archived
  record.
* Lint: `npx markdownlint-cli2@0.23.1` reported 0 issues on the plan and this note, and
  `backlogit docs lint` reported `valid: true`, `violation_count: 0` on both paths.
* `backlogit_sync_index` was run after the manifest edit and again at the end of the session.

## Resume point (amended)

Ship may re-run Step 3 with `M` = the 18 tasks. Wave 2 (`197.009-T`, `197.011-T` to `197.015-T`,
`197.018-T`) is no longer blocked on U15. No git operation was run by Stage. The governed archive
of `197.016-T` staged its rename in the git index (the archive operation uses `git mv`); every other
change is in the working tree.
