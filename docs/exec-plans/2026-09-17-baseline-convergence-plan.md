---
chunk_strategy: h1-h2-h3
description: 'Restore repository-wide test/lint/format green by fixing the CRLF golden-fixture failure and repository line-ending, errcheck, and staticcheck baseline debt, unblocking shipment 149-S.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
title: 'Repository Baseline Convergence Release Unit'
---

# Repository Baseline Convergence Release Unit

Source deliberation: `docs/decisions/2026-09-17-baseline-convergence-deliberation.md`
Grounding evidence: `docs/memory/2026-09-17/149-s-wave-1-convergence-hard-stop.md`

## Problem Frame

Shipment `149-S` is blocked at wave-convergence because inherited repository
baseline debt fails the mandatory gates. Three failure classes, two root causes:

1. **Line-ending root cause** (`* text=auto` + Windows checkout): `.gitattributes`
   classifies every text file as `text=auto`, so a Windows checkout converts the
   LF-stored blobs to CRLF. This produces (a) the
   `internal/faultline.TestU4aBehaviorCanonicalByteStable` failure — the golden
   `internal/faultline/testdata/parity_v1.golden.json` is read with CRLF while
   `EvidenceArtifact.Canonical()` emits LF (`bytes.Equal` fails at
   `evidence_conformance_test.go:94`), and (b) the broad `gofmt -l .` drift across
   `.go` files.
2. **Genuine code-quality debt** (independent of line endings): 50 `errcheck`
   (unchecked error returns) + 6 `staticcheck` findings reported by
   `golangci-lint run` across `internal/cli`, `internal/db`, `internal/telemetry`,
   `internal/stash`, `internal/events`, `tests/integration`, and possibly others.

Completion restores `go test ./...`, `golangci-lint run`, and `gofmt -l .` to
green, which is the precondition for `149-S` to resume.

## Requirements Trace

| Requirement (source) | Implementation unit(s) |
|---|---|
| `TestU4aBehaviorCanonicalByteStable` passes (92F79833) | U1 (`.gitattributes`), U3 (golden fixture) |
| `gofmt -l .` clean (4DB1DFF1) | U1 (renormalize), U2 (residual gofmt), U4–U11, U13 (per-unit gofmt) |
| 50 errcheck findings resolved (4DB1DFF1) | U4–U10, U13 (closed residual set) |
| 6 staticcheck findings resolved (4DB1DFF1) | U11 |
| `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l .` all green | U12 |
| Durable, non-re-drifting line endings on Windows (CI guard) | U1 |

## Implementation Units

Each unit obeys the 2-hour rule (< 3 files, < 5 functions, < 4 test scenarios),
width isolation (single domain), and an atomic verifiable milestone. **U1 is the
declared exception** to the file-count bound (see U1) — a sanctioned, mechanical,
content-identical exception, not a compliant unit.

**Ordering barrier (resolves plan-review P1):** U1's renormalize commit is a
STRICT PREDECESSOR of every code-touching unit (U2, U3, U4–U11). U4–U11 do NOT
proceed in parallel with U1; they begin only after U1's renormalize commit is
merged (or rebase onto post-U1 state). This preserves U1's "line-ending-only
churn" verification invariant: a genuine content edit landing before U1 would
make U1's content-identical gate impossible to satisfy.

### U1 — `.gitattributes` line-ending hardening + renormalize (config) — STRICT PREDECESSOR

* **Operating mode:** careful / freeze-scope (high blast radius). **Mandatory,
  explicitly-recorded operator-only approval of the renormalization diff,
  captured immediately before the whole-tree renormalization commit lands**
  (resolves plan-review P2 / Principle VII) — this is a required gate, not a
  recommendation. Ship (or any agent) may PREPARE and PRESENT the
  renormalization diff and its verification evidence, but Ship CANNOT authorize
  the renormalization; only the operator can approve it, and that approval must
  be recorded before the commit lands.
* **Changes:** First enumerate tracked binaries (`git ls-files` filtered for
  `*.exe`, `*.db`, images, and any intentionally-CRLF fixture) — add `-text`
  rules ONLY for classes actually present (no speculative image rules). Keep
  `* text=auto` for auto-classified files; add explicit `*.go text eol=lf` and
  `*.json text eol=lf`. For the golden fixture, PREFER the LF-preserving
  attribute `internal/faultline/testdata/parity_v1.golden.json text eol=lf`
  (which pins LF on every checkout) rather than `-text`. If `-text` is chosen
  instead to make the golden checkout-filter-immune, the fixture MUST FIRST be
  normalized to LF in the working tree and index (rewrite CRLF→LF, then
  `git add`) BEFORE the `-text` rule is applied — otherwise `-text` would freeze
  whatever (possibly CRLF) bytes are currently on disk. Place the golden rule
  AFTER the `*.json` rule so the most-specific/last match wins. Do NOT use the
  redundant/contradictory `text=auto eol=lf` combined form. Then run
  `git add --renormalize .` to update the INDEX. Because
  `git add --renormalize .` rewrites only the index and does NOT refresh
  already-checked-out working-tree files, explicitly REFRESH the working tree
  afterward (e.g., `git checkout-index -f -a`, or remove and re-checkout the
  affected paths) so the on-disk `.go`/`.json` bytes actually become LF BEFORE
  any `w/lf` assertion is made. Keep the concrete implementation deferred to
  Ship.
* **Files:** `.gitattributes` (+ mechanical renormalization of many tracked
  files — DECLARED file-count exception; content-identical).
* **Verify:** `git ls-files --eol` shows `w/lf` for tracked `.go`/`.json`
  (asserted ONLY after the working-tree refresh above) and `-text` (or `w/lf`
  under the `eol=lf` variant) for the golden fixture, and `-text` for declared
  binaries. `git diff --cached --stat` shows the staged line-ending churn. To
  prove the churn is content-identical, run the content-identity comparison
  EXCLUDING the intentional config change (`.gitattributes`), e.g.
  `git diff --cached -w --stat -- . ':(exclude).gitattributes'` — `.gitattributes`
  genuinely changes content and would otherwise make a global `-w` comparison
  non-empty by design. For tracked binaries, do NOT treat `-`/`-` in
  `git diff --cached --numstat` as proof of byte-identity (`-`/`-` only signals
  binary CLASSIFICATION, not unchanged bytes): instead require each declared
  binary to be ABSENT from the staged diff entirely (renormalization must not
  touch it), or compare its pre/post blob hashes (`git rev-parse :<path>` before
  vs after) and confirm they are identical.
* **Durable CI guard (resolves plan-review P2):** add a persistent CI step
  (`git add --renormalize . && git diff --cached --exit-code`, or a
  `git ls-files --eol` assertion) so line endings cannot silently re-drift after
  this ships.
* **Posture:** migration-first.

### U2 — Residual `gofmt` remediation (code-format)

* **Changes:** After U1 renormalization, run `gofmt -w` on any files still
  reported by `gofmt -l .` for genuine formatting (not line endings).
* **Files:** only the residual files `gofmt -l .` reports (expected small).
* **Verify:** `gofmt -l .` returns empty.
* **Posture:** characterization-first (`gofmt -l .` is the characterization).

### U3 — Golden-fixture LF normalization + U4a green (test) — fixes 92F79833

* **Changes:** FREEZE the committed `internal/faultline/testdata/parity_v1.golden.json`
  as byte-exact LF. Do NOT rely on `-update` regeneration to "verify" (it is
  self-cancelling: equal ⇒ nothing to change; unequal ⇒ that IS the regression
  signal and must not be overwritten). Forbid `-update` regeneration in CI.
* **REQUIRED non-vacuity assertion (resolves plan-review P1):** add a mandatory
  assertion in `evidence_conformance_test.go` that the golden is non-empty and
  that `Canonical()` output is compared byte-for-byte against the on-disk LF
  bytes (never a regenerated-and-trusted value). Cite
  `docs/compound/test-failures/go-analysistest-absolute-path-and-non-vacuity-2026-09-11.md`
  (same package `internal/faultline`, shipment 140-S) which makes non-vacuity
  mandatory, not optional.
* **Files:** `internal/faultline/testdata/parity_v1.golden.json` +
  `evidence_conformance_test.go` (non-vacuity assertion).
* **Verify:** `go test -run '^TestU4aBehaviorCanonicalByteStable$' ./internal/faultline`
  passes on a Windows checkout.
* **Posture:** test-first (the failing test already exists; make it green).

### U4–U10, U13 — errcheck remediation, per-package with a closed residual set (code)

**Pre-step (enforce granularity up front — resolves plan-review P2):** before
committing the U4–U10 boundaries, enumerate per-package errcheck counts
(`golangci-lint run <pkg>`). If a package exceeds the 2-hour file bound
(≥ 3 files / ≥ 5 functions), pre-declare a split into bounded sub-units
(U4a/U4b…) rather than discovering the overflow mid-execution.

Per-category remediation strategy (resolves plan-review P2 — blanket `%w` is not
idiomatic for the dominant errcheck categories):

* deferred `Close()` on a WRITABLE file → named return + assign inside the
  deferred closure: `defer func() { err = errors.Join(err, f.Close()) }()`
  (a terse `defer errors.Join(err, f.Close())` would discard the close error;
  `errors.Join` with nil operands returns nil, so the pattern is otherwise sound).
* `Write`/short-write paths → the existing checked helper pattern
  (`docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md`).
* genuinely discardable returns (`Fprintf` to a buffer) → explicit `_ = …` with a
  justified inline reason; in test code prefer `t.Setenv` over discarding
  `os.Setenv`'s error where a `*testing.T` is in scope.
* propagation with `fmt.Errorf("…: %w", err)`; preserve any existing sentinel
  classification with the two-`%w` form when a shared error policy classifies via
  `errors.Is` (`docs/compound/2026-09-04-two-percent-w-discriminator-for-shared-error-policy.md`).
* **Shared-helper coupling guard (resolves plan-review P2):** do NOT introduce a
  new cross-package shared helper opportunistically inside these
  graph-independent units. Introduce a shared checked helper ONLY when ≥ 3
  genuinely identical call sites exist AND hoist it into an explicit predecessor
  sub-unit that U4–U10 depend on; otherwise a single inline check suffices.

* **U4** `internal/cli`
* **U5** `internal/db`
* **U6** `internal/telemetry`
* **U7** `internal/stash`
* **U8** `internal/events`
* **U9** `tests/integration` — **test-layer policy (resolves plan-review P3):**
  prefer `require.NoError`/explicit assertions over `%w` propagation; justified
  `//nolint:errcheck` allowed for genuinely non-load-bearing test cleanup.
* **U10** `internal/core` — the largest residual error-returning surface outside
  U4–U9, assigned its OWN bounded unit (NOT its sub-packages `internal/core/gate`
  / `internal/core/templates`, which belong to U13).
* **U13** residual CLOSED package set — every module package not owned by U3–U11
  is explicitly enumerated and owned by U13 (full list in the `175.013-T` task
  body): `internal/canonical`, `internal/mcp`, `internal/hooks`, `internal/parser`,
  `internal/mdfront`, `internal/docline`, `internal/errors`, `internal/models`,
  `internal/jsonutil`, `internal/fsutil`, `internal/atomicfile`, `internal/release`,
  `internal/version`, `internal/gateevidence`, `internal/gateproof`,
  `internal/cli/format`, `internal/core/gate`, `internal/core/templates`,
  `internal/faultline/{mutation,parity,compatcorpus,analyzer/*}`, `cmd/backlogit`,
  `cmd/faultline-analyze`, `cmd/gen-docs`, `scripts`, `tests`, `tests/contract`.
  Explicitly EXCLUDED: the U4–U10 packages, `internal/faultline` top-level (U3),
  and `internal/config` (evidenced clean in the 149-S wave-1 hard-stop memory).
  **Not an open-ended catch-all and NOT a Ship-created planning unit:** Stage owns
  the COMPLETE package→unit assignment here. If the aggregate residual surface
  exceeds the 2-hour/<3-file bound at execution, Ship performs a MECHANICAL
  execution subdivision (per-package subtasks under U10/U13, e.g. `175.013.a-T`) —
  execution decomposition of an already-owned unit; Ship creates NO new planning
  units. Packages with zero findings close as no-ops. Exact per-file counts are
  execution-verified by Ship (Stage role forbids running linters); the package
  OWNERSHIP decision — the planning decision — is closed here by Stage.

* **Verify (each):** `golangci-lint run <pkg>` reports 0 errcheck; `gofmt -l` on
  touched files is empty (per-unit format cohesion); package tests still
  build/pass.
* **Test-first note (Principle II — resolves plan-review P2):** where an errcheck
  fix introduces genuinely NEW reachable failure-path behavior, add/confirm a
  failing test exercising the propagated error first; where the change is purely
  mechanical capture with no reachable behavior change, record a justified
  Principle II deviation in the closure artifact.
* **Posture:** characterization-first (the linter is the characterization).

### U11 — staticcheck remediation (code)

* **Changes:** FIRST enumerate the specific staticcheck check IDs
  (`golangci-lint run`), then choose fix-vs-nolint per category — SA1019
  (deprecated API) and SA4006 (unused write) are behavior-relevant and must not
  be uniformly suppressed. `//nolint:staticcheck` only with a justified inline
  reason.
* **Files:** bounded to the files staticcheck flags (expected ≤ 3).
* **Verify:** `golangci-lint run` reports 0 staticcheck; `gofmt -l` on touched
  files empty.
* **Posture:** characterization-first.

### U12 — Repository convergence verification (verification) — unblocks 149-S

* **Changes:** none (verification only). Confirm and capture evidence that
  `go test ./...`, **`go vet ./...`**, `golangci-lint run`, and `gofmt -l .` are
  ALL green (all four mandatory quality gates — resolves plan-review P2 /
  Principle I).
* **Verify:** all four commands exit clean; evidence + the full enumerated list
  of any `//nolint` suppressions and their justifications recorded in the closure
  artifact.
* **Posture:** runtime-verification.

## Dependency Graph

```
U1 (.gitattributes + renormalize)  [STRICT PREDECESSOR of all code units]
 ├─> U2  (residual gofmt)
 ├─> U3  (golden fixture / U4a)
 ├─> U4  (errcheck internal/cli)      ─┐
 ├─> U5  (errcheck internal/db)        │
 ├─> U6  (errcheck internal/telemetry) │ mutually independent
 ├─> U7  (errcheck internal/stash)     │ once U1 has landed
 ├─> U8  (errcheck internal/events)    │
 ├─> U9  (errcheck tests/integration)  │
 ├─> U10 (errcheck internal/core)      │
 ├─> U11 (staticcheck)                 │
 └─> U13 (errcheck residual closed set)─┘
U2, U3, U4..U11, U13 ──> U12 (convergence verify)  [terminal gate]
```

No cycles. U1 is a strict predecessor barrier for every code-touching unit so the
renormalization diff stays content-identical. U2, U3, U4–U11, U13 are mutually
independent AFTER U1 lands. U12 is the terminal sink node whose incoming edges
are every fix unit (U2, U3, U4–U11, and U13).

**File-partition invariant (resolves plan-review cycle-2 P2):** every touched
file has exactly ONE owning unit — the package/lint-category axes must not both
edit the same file concurrently. Enforce: (a) U2 residual `gofmt` is scoped
strictly to files NOT owned by any errcheck/staticcheck unit (per-file gofmt for
owned packages folds into U4–U11, U13); (b) if U11 staticcheck flags a file inside a
package already owned by an errcheck unit (U4–U10, U13), merge that staticcheck fix
into the owning package unit OR add an explicit intra-package ordering edge so
the two units never edit the same file concurrently. This preserves the
"mutually independent" claim at file granularity, not just package granularity.

## Decisions and Rationale

* **Config-first line-ending fix** resolves the shared root cause of both the
  golden-fixture failure and the gofmt drift once, durably, preventing re-drift
  on future Windows checkouts (Option A of the deliberation).
* **Feature (not chore) as covering root** — the shipment covering-item
  derivation (`internal/core/shipment_covering.go:isRootCoveringFeature`) and the
  security-relevant manifest-binding digest require `artifact_type == "feature"`
  with a dotless root ID.
* **Per-package errcheck isolation** keeps each task within the 2-hour rule and
  isolates blast radius, per the operator directive against one oversized task.
* **Byte-pin the golden fixture** (assert exact LF bytes) to avoid a
  same-marshaler false-green (learnings).

## Risks and Caveats

* **Renormalization blast radius** — a mis-scoped `.gitattributes` rule could
  reclassify a binary (`.exe`, `.db`, images, intentionally-CRLF fixtures) and
  corrupt it. Mitigation: explicit `-text` binary rules; verify with
  `git diff --stat` (line-ending-only churn) and a binary spot-check before
  commit. (Hardened in the Plan Hardening section.)
* **errcheck count unknown at plan time** — Stage cannot run linters (role
  boundary), so exact per-file errcheck COUNTS are execution-verified by Ship.
  Stage nonetheless owns the COMPLETE, CLOSED package→unit assignment (U4–U10 for
  the named packages, U13 for the fully-enumerated residual set); no package
  enumeration and no planning-unit creation is deferred to Ship. Any intra-package
  overflow beyond the 2-hour/<3-file bound is a MECHANICAL execution subtask under
  the already-owned unit, never a new planning unit. A package with zero findings
  closes as a no-op.
* **Golden false-green** — never regenerate-and-trust; assert exact bytes.

## Constitution Check

Mapped against `.github/instructions/constitution.instructions.md`:

* **Safety-First Go** — pass. errcheck fixes use a per-category strategy
  (named-return + `errors.Join` for deferred writable-close, checked helper for
  short-writes, `fmt.Errorf("…: %w", err)` for propagation, justified `_ =` for
  genuinely discardable returns); no `unsafe` introduced; production code stays Go.
* **Test-First Development** — pass with a documented mechanical-capture note.
  U3 makes an existing failing test green and adds a required non-vacuity
  assertion. For errcheck fixes that introduce genuinely new reachable
  failure-path behavior, a failing test is added first (U4–U10 test-first note);
  purely mechanical captures with no reachable behavior change record a justified
  Principle II deviation in the closure artifact.
* **Workspace Isolation and Security Boundaries** — pass. All changes are within
  the workspace; no secrets committed; `.gitattributes` scoped to this repo.
* **CLI Workspace Containment** — pass. Nothing created/modified outside the
  working tree.
* **Destructive Command Approval** — pass with mandatory gate. `git add
  --renormalize .` is git-tracked and revertible; U1 declares careful/freeze-scope
  mode and requires MANDATORY, explicitly-recorded operator-only approval of the
  renormalization diff immediately before the commit lands. Ship may prepare and
  present the diff and its verification evidence but CANNOT authorize the
  renormalization; authorization is the operator's alone.
* **Task Granularity (2-Hour Rule)** — documented deviation for U1 only: the
  renormalization mechanically touches many files, exceeding the < 3-files
  heuristic. Sanctioned because the churn is mechanical and content-identical
  (verified via `git diff --cached -w --stat` empty). Rejected simpler
  alternative: splitting the renormalization into many per-directory sub-units —
  rejected because `git add --renormalize .` is an atomic whole-tree operation
  and slicing it would fragment one mechanical commit without reducing risk. All
  other units comply.
* **Quality Gates** — pass. U12 proves all four mandatory gates green:
  `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`.
* **Merge Commit History Preservation** — pass. The release unit ships via a
  merge commit (Ship-owned), not squash/rebase.

Constitution Check: documented-deviations

## Plan Hardening Signals

* public API / schema / contract change — **absent**.
* security / auth / permission / compliance behavior — **absent**.
* migration / backfill / destructive / irreversible step — **present**:
  `git add --renormalize .` (U1) rewrites the stored form of many tracked files;
  a mis-scoped `.gitattributes` rule could corrupt binaries. Broad blast radius.
* external integration / operator checkpoint / external dependency — **absent**.
* high runtime / rollout / rollback risk — **present**: this release unit is the
  gating precondition for a governed shipment (`149-S`); a botched
  renormalization would broadly churn the repository.

## Plan Hardening

Hardening required: **yes** — triggered by the migration/irreversible-step signal
(`git add --renormalize .`, U1) and the high-blast-radius/rollout signal (this
release unit gates governed shipment `149-S`).

**Learnings and instructions consulted:**
`docs/compound/runtime-errors/windows-mojibake-utf8-powershell-fix-2026-04-08.md`
(Windows write paths silently alter bytes; add a CI byte-form validation),
`docs/compound/2026-06-28-codec-extraction-leaf-packages.md` (prove "nothing
changed" with `git diff --exit-code` on generated/golden output),
`docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md`
(unchecked returns are correctness bugs; consolidate into a checked helper),
`.github/instructions/constitution.instructions.md`.

**Protected invariants:**

* No true binary is reclassified as text. `*.exe`, `*.db`, `backlogit.exe`,
  `*.golden` that are intentionally binary, and any image must carry an explicit
  `-text` rule and remain byte-identical after renormalization.
* `git diff` after U1 shows ONLY line-ending churn for `.go`/`.json`
  (content-identical); the content-identity check EXCLUDES the intentional
  `.gitattributes` change (e.g.
  `git diff -w --stat -- . ':(exclude).gitattributes'` is empty), since
  `.gitattributes` genuinely changes content by design.
* errcheck fixes preserve behavior on success paths and only add
  capture/propagation on failure paths — no package test regresses.
* The golden fixture is asserted against exact LF bytes, never a same-marshaler
  regeneration accepted as verified.

**Risky actions (ProposedAction / ActionRisk):**

* `ProposedAction:` rewrite `.gitattributes` and `git add --renormalize .`
  across the whole tree (U1). `ActionRisk:` HIGH blast radius (touches many
  tracked files), reversible via single-commit revert. `ActionResult` (expected):
  line-ending-only churn, all four baseline gates progress toward green, no
  binary corruption. **MANDATORY, explicitly-recorded operator-only approval of
  the renormalization diff immediately before the commit lands** (settled,
  blocking — see U1 operating mode and the Constitution Check Destructive Command
  Approval entry). Ship may prepare and present the diff and evidence but CANNOT
  authorize it. Not advisory.
* `ProposedAction:` bulk `gofmt -w` on residual files (U2). `ActionRisk:` LOW,
  mechanical, reversible.

**Added verification / rollback / monitoring:**

* U1 pre-commit gate: after the working-tree refresh, run `git ls-files --eol`,
  confirm `w/lf` for `.go`/`.json` and `-text` for declared binaries; verify each
  tracked binary is byte-identical by confirming it is ABSENT from the staged diff
  or by comparing its pre/post blob hash (`git rev-parse :<path>`). Do NOT rely on
  `git diff --numstat` `-`/`-` as an identity signal — it only marks binary
  CLASSIFICATION, not unchanged bytes.
* Rollback: each unit is a discrete commit; U1 renormalization reverts as a
  single commit without touching later fix commits.
* U12 is the monitoring/closure gate: all four mandatory gates (`go test ./...`,
  `go vet ./...`, `golangci-lint run`, `gofmt -l .`) green on a clean checkout,
  evidence captured in the closure artifact.

**Review-gate capability risk carried forward:** plan-review MUST emit literal
`dispatch_mode:` and `decision:` markers. No P-012 degraded-tool condition is
expected for this plan (no backlog-registry tool dependency inside review); if
sub-agent dispatch is unavailable, plan-review must declare
`single-agent-declared-degradation` rather than silently skip.

**Settled operator decision (not open):** MANDATORY, explicitly-recorded
operator-only approval of the U1 renormalization diff is a required, blocking gate
before the renormalization commit lands (reconciled with U1 and the Constitution
Check — no longer an optional/unresolved checkpoint). Ship may prepare and present
the diff and evidence; only the operator can authorize.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces:** none directly (repo hygiene + error handling).
  errcheck fixes change error-propagation behavior on failure paths — U4–U10 must
  keep package tests green so no behavior regresses.
* **Runtime verification to prove absorption:** U12 confirms all FOUR mandatory
  quality gates — `go test ./...`, `go vet ./...`, `golangci-lint run`, and
  `gofmt -l .` — are all green on a clean checkout.
* **Operational closure artifact:** a convergence closure note recording the
  four green gates and the `git ls-files --eol` proof, plus confirmation that
  `149-S` is now eligible once the baseline shipment ships. Rollback trigger: if
  renormalization corrupts any file, revert the U1 commit (single-commit
  rollback). Owner: Ship (execution); validation window: the next `149-S`
  wave-convergence run.

<!-- plan-review-attempt: 1 FAIL (2 P1); revised addressing all P1/P2 findings -->

<!-- plan-review-attempt: 2 PASS (cycle-2 P2s remediated in-plan; only P3 advisories remain) -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS
operator_authorization: approved

Reviewer sub-agent dispatch was available; the gate ran in full multi-agent
mode. Personas dispatched (always-on + triggered cross-model): Constitution
Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture
Strategist. Agent-Native Parity Reviewer and Security Lens Reviewer were NOT
triggered — the plan exposes no MCP tools / agent-facing actions and touches no
auth/authz, API surface, sensitive data store, external integration, or secrets.
Plan hardening was required (migration + high-blast-radius signals) and is
present (`## Plan Hardening` section). Constitution Check verdict is
`documented-deviations` (U1 Task-Granularity deviation, sanctioned and
governance-complete).

### Cycle 1 — decision: FAIL (2 P1)

* **P1 (Architecture Strategist)** — Hidden coupling: U1 renormalization declared
  "parallel" with U4–U11 code edits would defeat U1's line-ending-only-churn
  invariant. **Resolved:** U1 is now a STRICT PREDECESSOR barrier of all
  code-touching units; dependency graph redrawn.
* **P1 (Learnings Researcher)** — Missed the same-package non-vacuity learning
  (`go-analysistest-absolute-path-and-non-vacuity-2026-09-11`, 140-S,
  `internal/faultline`); non-vacuity assertion left optional. **Resolved:** U3
  now cites it and makes the non-vacuity assertion REQUIRED; `-update`
  regeneration forbidden in CI.
* Material P2s (all resolved in the revision): missing `go vet` gate (added to
  U12/trace/Quality-Gates); blanket `%w` (replaced with per-category errcheck
  strategy); golden byte-pin tautology (freeze + forbid `-update` + non-vacuity);
  `.gitattributes` rule correctness (`* text=auto` + explicit `eol=lf` +
  `-text` golden, most-specific-last); durable CI byte-form guard; enumerate-
  before-split + U10 enumerate-then-split; `git diff --cached`; U1 approval made
  mandatory; shared-helper coupling gated; U11 enumerate staticcheck IDs; U9
  test-layer policy.

### Cycle 2 — decision: PASS (no P0/P1; P2s remediated in-plan; P3 advisories remain)

Both cycle-1 P1s independently confirmed RESOLVED by Architecture Strategist and
Learnings Researcher. Go Reviewer: approve. Scope Boundary Auditor: substantially
clean, no scope creep introduced. Cycle-2 P2 findings — all remediated in-plan:

* **P2 (Constitution ×2)** — stale "recommended"/"unresolved" wording for the U1
  approval contradicted the mandatory gate. **Fixed:** Plan Hardening risky-action
  and operator-decision text now state MANDATORY/settled/blocking.
* **P2 (Architecture)** — package-axis vs lint-category-axis units could edit the
  same file concurrently. **Fixed:** added a file-partition invariant (every
  touched file has exactly one owning unit; U2 scoped to unowned files; staticcheck
  merges into owning package unit or gets an ordering edge).

Remaining P3 advisories (non-blocking, acknowledged): `t.Setenv` over discarded
`os.Setenv` and explicit deferred-close closure-assignment form (both folded into
U4–U10 strategy); Constitution governance-completeness rejected-alternative
sentence (added); durable CI guard is preventive-beyond-strict-unblock (declared
in Requirements Trace, kept); "Feature-not-chore" rationale bullet retained as
useful harvest context.

### Runtime verification / closure

U12 proves all four mandatory gates (`go test ./...`, `go vet ./...`,
`golangci-lint run`, `gofmt -l .`) green and records the closure artifact +
enumerated `//nolint` justifications. Rollback: single-commit revert of U1.

Gate result: **PASS** — proceed to harvest.
