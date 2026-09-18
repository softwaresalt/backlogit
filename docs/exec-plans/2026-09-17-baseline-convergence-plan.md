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
| Durable, non-re-drifting line endings on Windows (CI guard) | U14 (`.github/workflows/ci.yml`) |

## Implementation Units

Each unit obeys the 2-hour rule (< 3 files, < 5 functions, < 4 test scenarios),
width isolation (single domain), and an atomic verifiable milestone. **U1 is the
declared exception** to the file-count bound (see U1) — a sanctioned, mechanical,
content-identical exception, not a compliant unit.

**Ordering barrier (resolves plan-review P1):** U1's renormalize commit is a
STRICT PREDECESSOR of every code-touching unit (U2, U3, U4–U11, U13, U14). These
units do NOT proceed in parallel with U1; they begin only after U1's renormalize commit is
merged (or rebase onto post-U1 state). This preserves U1's "line-ending-only
churn" verification invariant: a genuine content edit landing before U1 would
make U1's content-identical gate impossible to satisfy.

### U1 — `.gitattributes` line-ending hardening + renormalize (config) — STRICT PREDECESSOR

* **Operating mode:** careful / freeze-scope (high blast radius). **Mandatory,
  explicitly-recorded operator-only approval of the renormalization — recorded
  BEFORE any `git add --renormalize` staging or any working-tree refresh, not
  merely before the commit** (resolves plan-review P2 / Principle VII) — this is
  a required gate, not a recommendation. **Clean-tree precondition (blocking):**
  before staging anything, assert the working tree and index carry NO unrelated
  changes (`git status --porcelain` empty except the intended `.gitattributes`
  edit); FAIL and halt if any unrelated index/worktree change exists, so
  renormalization cannot sweep in unrelated edits. Ship (or any agent) may
  PREPARE and PRESENT the proposed `.gitattributes` rules, the enumerated
  affected-path set (previewable via `git ls-files --eol` and an index-only
  staged diff), and its verification evidence, but Ship CANNOT authorize the
  renormalization; only the operator can approve it, and that approval must be
  recorded before any staging or working-tree refresh mutates state.
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
  redundant/contradictory `text=auto eol=lf` combined form. Then, ONLY AFTER the
  recorded operator approval and with the clean-tree precondition satisfied, run
  `git add --renormalize .` to update the INDEX (index-only, non-destructive,
  reversible via `git reset`). Because `git add --renormalize .` rewrites only
  the index and does NOT refresh already-checked-out working-tree files,
  explicitly REFRESH the affected paths afterward so their on-disk `.go`/`.json`
  bytes become LF BEFORE any `w/lf` assertion is made. Derive the affected-path
  set as the UNION of (a) the paths the renormalize actually staged
  (`git diff --cached --name-only`) AND (b) every tracked path governed by a new
  `eol=lf` attribute whose working-tree EOL is still `w/crlf`/`w/mixed`
  (enumerated via `git ls-files --eol` restricted to the `eol=lf`-governed set).
  The staged-diff names ALONE are insufficient: if the index is already LF (e.g.
  a prior renormalize) while the working tree is CRLF, `git add --renormalize`
  stages nothing and the staged-diff list is empty even though CRLF working-tree
  files still need refresh. Re-checkout exactly that union set
  (`git checkout -- <affected paths>`). Do NOT use a
  forced whole-tree refresh (`git checkout-index -f -a`) or whole-tree
  removal/re-checkout — the refresh is scoped strictly to the enumerated
  `eol=lf`-governed affected-path set. Keep the concrete implementation deferred to Ship.
* **Files:** `.gitattributes` (+ mechanical renormalization of many tracked
  files — DECLARED file-count exception; content-identical).
* **Verify:** `git ls-files --eol` shows `w/lf` for tracked `.go`/`.json`
  (asserted ONLY after the working-tree refresh above) and `-text` (or `w/lf`
  under the `eol=lf` variant) for the golden fixture, and `-text` for declared
  binaries. `git diff --cached --stat` shows the staged line-ending churn. To
  prove the churn is EOL-only (CRLF->LF) and content-identical in every non-EOL
  byte, do NOT rely on `git diff -w` (which ignores ALL intra-line whitespace,
  not just line-ending bytes, and would mask a genuine non-EOL whitespace edit).
  Instead, for each staged text path — EXCLUDING the intentional `.gitattributes`
  change and all declared binaries — compare the base and index blob content
  after canonicalizing ONLY CRLF->LF:
  - base bytes `B = git cat-file blob HEAD:<path>`; index bytes
    `I = git cat-file blob :<path>`;
  - canonicalize each by replacing every CRLF (`\r\n`) with LF (`\n`) and leaving
    every other byte (including any bare CR) untouched;
  - require `SHA-256(canon(B)) == SHA-256(canon(I))` for every such path.
  Equal hashes prove that ONLY line-ending bytes changed; any non-EOL byte
  difference (added/removed/edited content, or a bare-CR change) makes the hashes
  differ and fails the check. `.gitattributes` is excluded because it genuinely
  changes content by design. For tracked binaries, do NOT treat `-`/`-` in
  `git diff --cached --numstat` as proof of byte-identity (`-`/`-` only signals
  binary CLASSIFICATION, not unchanged bytes): instead require each declared
  binary to be ABSENT from the staged diff entirely (renormalization must not
  touch it), or compare its pre/post blob hashes (`git rev-parse :<path>` before
  vs after) and confirm they are identical.
* **Durable CI guard — split into U14 (`175.014-T`):** the persistent CI
  line-ending guard is owned by U14, NOT U1, because installing it is a genuine
  content change to `.github/workflows/ci.yml` and would otherwise violate U1's
  content-identical (EOL-only) invariant. U14 depends on U1 and inspects
  working-tree EOL via `git ls-files --eol`, failing on `w/crlf`/`w/mixed` (see
  U14). U1 itself introduces NO workflow content change; its EOL-only
  content-identity proof therefore does not (and must not) account for a
  workflow edit.
* **Posture:** migration-first.

### U2 — Residual `gofmt` remediation (code-format) — runs LAST before U12

* **Changes:** After U1 renormalization AND after every code-owning unit
  (U3, U4–U11, U13) has completed, run `gofmt -w` on any residual files still
  reported by `gofmt -l .` for genuine formatting (not line endings) that are
  owned by NO other unit.
* **Files:** only the residual files `gofmt -l .` reports that no other unit
  owns (expected small).
* **Verify:** `gofmt -l <U2-owned files>` returns empty for the files this unit
  touched. Under the exactly-one-owner redesign U2 runs LAST among the fix units
  (after U3, U4–U11, U13; before only U12) and owns ONLY residual files not owned
  by any errcheck/staticcheck/test-fixture unit — per-unit gofmt for owned files
  folds into each owning unit — so U2 does NOT assert repository-wide `gofmt -l .`;
  that repository-wide assertion belongs to U12.
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
  `evidence_conformance_test.go` (non-vacuity assertion). These two files are
  pre-assigned to U3 and excluded from U11's overlap inventory; U3 owns EVERY
  finding in them (test correctness, gofmt, any errcheck/staticcheck) so exactly
  one unit owns each and no finding is orphaned.
* **Verify:** `go test -run '^TestU4aBehaviorCanonicalByteStable$' ./internal/faultline`
  passes on a Windows checkout.
* **Posture:** test-first (the failing test already exists; make it green).

### U4–U10, U13 — errcheck remediation, per-package with a closed residual set (code)

**Pre-step (enforce granularity up front — resolves plan-review P2):** before
committing the U4–U10 boundaries, enumerate per-package errcheck counts
(`golangci-lint run <pkg>`). If a package exceeds the 2-hour file bound
(≥ 3 files / ≥ 5 functions), pre-declare a split into bounded sub-units
(U4a/U4b…) rather than discovering the overflow mid-execution.

**Exactly-one-owner ordering (resolves Copilot cycle-2 exactly-one-owner
finding):** U4–U10 and U13 each depend on U11 (`175.011-T`), which runs
IMMEDIATELY AFTER U1 as the staticcheck/errcheck overlap-inventory owner. U11
records the COMPLETE set of staticcheck-flagged files (minus the two U3-owned
files) and owns EVERY finding in them — staticcheck AND errcheck. Each errcheck
unit reads U11's recorded owned-file set and EXCLUDES it from its own scope, so
no file is ever owned by two units. Ownership is fixed by U11's up-front
inventory before any code edit; serialization order never confers ownership. This
REPLACES the earlier scheme where U11 depended on the errcheck units (which could
serialize two owners onto one file).

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
  `cmd/faultline-analyze`, `cmd/gen-docs`, `scripts`, `tests`, `tests/contract`,
  plus `internal/config` and `internal/faultline` top-level (both now INCLUDED
  with FILE-LEVEL exclusions for the U3-owned files only).
  Explicitly EXCLUDED: the U4–U10 packages. `internal/config` and
  `internal/faultline` top-level are NO LONGER package-excluded on 149-S
  evidence alone (Stage cannot attach package-wide clean-lint evidence — role
  boundary forbids running linters); instead they are owned by U13 with FILE-LEVEL
  exclusions for the two U3-owned files
  (`internal/faultline/testdata/parity_v1.golden.json` and
  `internal/faultline/evidence_conformance_test.go`).
  **Not an open-ended catch-all and NOT a Ship-created planning unit:** Stage owns
  the COMPLETE package→unit assignment here. If the aggregate residual surface
  exceeds the 2-hour/<3-file bound at execution, Ship performs a MECHANICAL
  execution subdivision (per-package subtasks under U10/U13, e.g. `175.013.001-ST`) —
  execution decomposition of an already-owned unit; Ship creates NO new planning
  units. Packages with zero findings close as no-ops. Exact per-file counts are
  execution-verified by Ship (Stage role forbids running linters); the package
  OWNERSHIP decision — the planning decision — is closed here by Stage.

* **Verify (each):** `golangci-lint run <pkg>` reports 0 errcheck; `gofmt -l` on
  touched files is empty (per-unit format cohesion); package tests still
  build/pass.
* **Test-first note (Principle II):** U4–U10, U11, and U13 are NORMAL
  harness-required units — **none** claims a harness exemption. The
  harness-architect authors a failing (RED) harness/test that compiles before the
  fix and fails on an assertion (P-002/P-004), and implementation drives it green.
  Where an errcheck fix introduces genuinely NEW reachable failure-path behavior,
  the RED test exercises the propagated error first. NO mechanical/generic/
  closure-time harness exemption, exempt contract, waiver, or Principle II
  deviation is declared for any errcheck unit; the sole harness-exempt unit in
  this plan is U12 (verification-only — see the Harness-Exempt Set section).
* **Posture:** characterization-first (the linter is the characterization).

### U11 — staticcheck remediation + overlap-inventory owner (code) — runs after U1

* **Changes:** FIRST run `golangci-lint run` and enumerate (a) the specific
  staticcheck check IDs and (b) the COMPLETE set of files staticcheck flags. U11
  EXCLUSIVELY OWNS every staticcheck-flagged file for ALL its findings —
  staticcheck AND any errcheck in the same file — EXCEPT the two U3-owned files
  (`parity_v1.golden.json`, `evidence_conformance_test.go`), which are
  pre-assigned to U3 and excluded from the inventory. Choose fix-vs-nolint per
  category — SA1019 (deprecated API) and SA4006 (unused write) are
  behavior-relevant and must not be uniformly suppressed; `//nolint:staticcheck`
  only with a justified inline reason. Errcheck findings inside a U11-owned file
  use the same per-category errcheck strategy as U4–U10.
* **Files:** the staticcheck-flagged files (expected ≤ 3), minus the two U3-owned
  files. U11 records this owned-file set as DETERMINISTIC HANDOFF EVIDENCE in its
  closure artifact so U4–U10/U13 can exclude it.
* **Dependencies (exactly-one-owner redesign — resolves Copilot cycle-2
  finding):** U11 depends on U1 (`175.001-T`) ONLY and runs IMMEDIATELY AFTER U1,
  BEFORE every errcheck unit. U4–U10 and U13 depend on U11 (the edge direction is
  REVERSED from the cycle-1 conservative scheme) and exclude U11's recorded
  owned-file set. Because overlap ownership is decided by U11's up-front inventory
  before any errcheck edit, no file is ever owned by two units and serialization
  never confers ownership. U12 remains the terminal sink.
* **Verify:** `golangci-lint run` reports 0 staticcheck; `gofmt -l` on touched
  files empty.
* **Posture:** characterization-first.

### U14 — Persistent CI line-ending guard (`.github/workflows/ci.yml`) (config/CI)

* **Split rationale (resolves Copilot cycle-1 ownership-honesty finding):** the
  persistent CI guard is its OWN unit, not part of U1, because installing it is a
  GENUINE content change to `.github/workflows/ci.yml` and would break U1's
  content-identical (EOL-only) invariant if folded in. U1 owns only
  `.gitattributes` + the EOL-only renormalization; U14 owns the workflow content
  change, and U1's content-identity proof does not account for any workflow edit.
* **Changes:** add a persistent CI step that inspects WORKING-TREE end-of-line
  state via `git ls-files --eol` and FAILS the job on any `w/crlf` or `w/mixed`
  for tracked `eol=lf` text paths (`.go`/`.json`; the golden and declared
  binaries are excluded per their attribute rules). This working-tree guard MUST
  run on a `windows-latest` runner AFTER `actions/checkout` — CRLF re-drift only
  manifests on a Windows checkout, so a Linux checkout would false-green — and
  MUST run on EVERY PR/push to protected branches: it is NOT gated behind a
  `paths:`/`paths-ignore:`/changed-files filter, so a docs-only or backlog-only
  change still exercises the checkout + guard and cannot silently re-drift line
  endings (if path-based job skipping exists elsewhere in the workflow, this
  guard is explicitly exempt / always-run). The index-diff /
  renormalize-exit-code check (`git add --renormalize . && git diff --cached
  --exit-code`) is retained as SEPARATE, complementary content-identity evidence
  that MAY run on a distinct `ubuntu-latest` job — NOT a substitute for the
  Windows working-tree `w/*` inspection, since an index-only check can pass while
  the checkout carries CRLF.
* **Workflow scope (concrete — resolves Copilot cycle-3 finding):** the CURRENT
  `.github/workflows/ci.yml` triggers on `pull_request` ONLY (no `push`) and skips
  the Windows checkout for docs/backlog-only changes via `needs.changes` step
  conditions. A guard-STEP-only edit therefore could NOT satisfy the
  always-run-on-push/PR requirement. U14's ownership expands — WITHIN the same one
  file — to THREE coordinated changes: (1) add a `push:` trigger for protected
  branches (`main`) alongside `pull_request:` (reconciling the file's "PR-only"
  header comment for this intentionally push+PR guard, since CRLF re-drift can land
  via a direct push); (2) add a DEDICATED always-run Windows guard job modeled on
  the existing always-run `md-lint`/`topology-check` jobs — no `needs: changes`, no
  `paths:`/`paths-ignore:` filter, no changed-files skip — leaving the scoped
  `test-windows` (167.013-T) job unchanged; and (3) the `git ls-files --eol` guard
  step itself. Single CI/config domain, one-file contract kept under the 2-hour
  rule.
* **Files:** `.github/workflows/ci.yml` only. Stage does NOT edit the workflow;
  this unit authorizes Ship to make the trigger, always-run-job, and guard-step
  changes at execution time.
* **Depends on:** U1 (`175.001-T`) — authored after renormalization lands.
* **Verify:** the guard fails a synthetic CRLF re-drift and passes on the
  normalized tree; existing CI jobs still pass.
* **Posture:** migration-first (guard installation).

### U12 — Repository convergence verification (verification) — unblocks 149-S

* **Changes:** none to production/config; commits ONLY the evidence artifact
  `docs/closure/175-baseline-convergence-convergence-evidence.md` (the
  verification-only class delta surface under `docs/closure/`). Confirm and capture
  evidence that `go test ./...`, **`go vet ./...`**, `golangci-lint run`, and
  `gofmt -l .` are ALL green (all four mandatory quality gates — resolves
  plan-review P2 / Principle I).
* **Harness-exempt (P-002.1 `verification-only`):** U12 carries the
  `harness-exempt` label and a canonical harness-exemption contract block — class
  `verification-only`, `harness_owner: none`, an exact `exempt_verification_command`
  that FAILS before the evidence artifact exists (required pre-work failure) and
  exits 0 with the unique `EXEMPT_VERIFY_OK:175.012-T` marker after the deliverable
  lands, `exempt_precondition: must-fail-before-deliverable`. It records evidence
  for already-delivered gate-green behavior, scaffolds no red harness, and adds no
  new red assertion. It is the SOLE member of the plan's closed harness-exempt set
  (below).
* **Verify:** all four commands exit clean; evidence + the full enumerated list
  of any `//nolint` suppressions and their justifications recorded in the evidence
  artifact with the exact gate-green marker strings the exempt command asserts.
* **Posture:** runtime-verification.

## Harness-Exempt Set (closed)

This release unit declares a CLOSED harness-exempt set with exactly ONE member
(satisfies P-002.1 required-metadata condition 3 — closed-exempt-set membership):

| Task | Class | `harness_owner` | Deliverable |
|---|---|---|---|
| `175.012-T` (U12) | `verification-only` | `none` | evidence artifact `docs/closure/175-baseline-convergence-convergence-evidence.md` |

No other unit in this plan is harness-exempt. U1–U11, U13, and U14 are all NORMAL
harness-required units gated by a real red harness authored by the
harness-architect (P-002/P-004). Any `harness-exempt` label on a unit outside this
set, or any member of this set failing its P-002.1 static-intake contract, is a
fail-closed halt under the P-002.2 taxonomy — never a generic, mechanical, or
retrospective waiver.

## Dependency Graph

```
U1 (.gitattributes + renormalize)  [STRICT PREDECESSOR of all code units]
 ├─> U3  (golden fixture / U4a — owns 2 named files; excluded from U11 inventory)
 ├─> U11 (staticcheck + overlap-inventory owner — runs IMMEDIATELY after U1)
 └─> U14 (persistent CI line-ending guard, .github/workflows/ci.yml)

U1, U11 ──> U4  (errcheck internal/cli)      ─┐ each errcheck unit depends on
U1, U11 ──> U5  (errcheck internal/db)        │ U1 (predecessor) AND U11, and
U1, U11 ──> U6  (errcheck internal/telemetry) │ EXCLUDES U11's recorded
U1, U11 ──> U7  (errcheck internal/stash)     │ staticcheck-owned file set, so
U1, U11 ──> U8  (errcheck internal/events)    │ exactly ONE unit owns each file
U1, U11 ──> U9  (errcheck tests/integration)  │ (ownership fixed by U11's
U1, U11 ──> U10 (errcheck internal/core)      │ up-front inventory, never by
U1, U11 ──> U13 (errcheck residual closed set)┘ serialization order)

U3, U4..U11, U13 ──> U2 (residual gofmt — runs LAST before U12; residual UNOWNED paths only)
U2, U3, U4..U11, U13, U14 ──> U12 (convergence verify)  [terminal sink]
```

No cycles (41 task-dependency edges total). U1 is a strict predecessor barrier for
every code-touching unit so the renormalization diff stays content-identical. U11
(staticcheck + overlap inventory) runs IMMEDIATELY after U1 and is a predecessor of
every errcheck unit (U4–U10, U13), which each also depend on U1; U11 owns every
staticcheck-flagged file (minus the two U3-owned files) for ALL findings and
records that owned-file set, so each errcheck unit excludes it and exactly one unit
owns each file. U3 and U14 depend on U1 only. U2 (residual gofmt) runs LAST among
the fix units — it depends on U3, U4–U11, and U13 and formats only residual paths
owned by no other unit. U12 is the terminal sink node whose incoming edges are every
fix unit (U2, U3, U4–U11, U13, and U14).

**File-partition invariant (resolves plan-review cycle-2 P2 + Copilot cycle-2
exactly-one-owner finding):** every touched file has exactly ONE owning unit —
the package/lint-category axes must not both edit the same file. Enforce via an
UP-FRONT ownership inventory, not serialization: (a) U11 runs immediately after
U1 and, as the overlap-inventory owner, records the COMPLETE set of
staticcheck-flagged files (minus the two U3-owned files) and owns EVERY finding in
them (staticcheck AND errcheck); (b) each errcheck unit (U4–U10, U13) depends on
U11 and EXCLUDES U11's recorded owned-file set from its own scope, so a
staticcheck-flagged file is owned by U11 alone and is never edited by an errcheck
unit; (c) U2 residual `gofmt` runs LAST (after U3, U4–U11, U13) and is scoped
strictly to files owned by NO other unit (per-file gofmt for owned packages folds
into their owning units). Ownership is fixed BEFORE any code edit; serialization
order NEVER confers ownership (the earlier "merge OR add an ordering edge" escape
is removed). This preserves exactly-one-owner at file granularity, not just
package granularity.

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
* **Test-First Development** — pass. U3 makes an existing failing test green and
  adds a required non-vacuity assertion. U4–U10, U11, and U13 are NORMAL
  harness-required units — none claims a harness exemption; the harness-architect
  authors a failing (RED) harness/test that compiles before the fix and fails on
  an assertion (P-002/P-004), and implementation drives it green. For errcheck
  fixes that introduce genuinely new reachable failure-path behavior the RED test
  exercises the propagated error. U12 is the SOLE harness-exempt unit
  (verification-only, canonical P-002.1 contract with the required label, contract
  block, and closed-exempt-set membership). No mechanical/generic/closure-time
  harness exemption or Principle II deviation is declared for any errcheck unit.
* **Workspace Isolation and Security Boundaries** — pass. All changes are within
  the workspace; no secrets committed; `.gitattributes` scoped to this repo.
* **CLI Workspace Containment** — pass. Nothing created/modified outside the
  working tree.
* **Destructive Command Approval** — pass with mandatory gate. `git add
  --renormalize .` is git-tracked and revertible; U1 declares careful/freeze-scope
  mode and requires MANDATORY, explicitly-recorded operator-only approval of the
  renormalization recorded BEFORE any `git add --renormalize` staging or
  working-tree refresh (not merely before the commit), gated by a clean-tree
  precondition. Ship may prepare and
  present the diff and its verification evidence but CANNOT authorize the
  renormalization; authorization is the operator's alone.
* **Task Granularity (2-Hour Rule)** — documented deviation for U1 only: the
  renormalization mechanically touches many files, exceeding the < 3-files
  heuristic. Sanctioned because the churn is mechanical and content-identical
  (verified via the EOL-only normalized-blob SHA-256 comparison in U1's Verify
  section — CRLF->LF canonicalization with all non-EOL bytes preserved,
  `.gitattributes` and declared binaries excluded). Rejected simpler
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
* external integration / operator checkpoint / external dependency — **present**:
  a MANDATORY, explicitly-recorded operator-only approval checkpoint gates U1. The
  operator must approve the planned `.gitattributes` rules and the enumerated
  renormalization path set, recorded BEFORE any `git add --renormalize` staging or
  any working-tree refresh (not merely before the commit), gated by a clean-tree
  precondition that FAILS on any unrelated index/worktree change. Ship may prepare
  and present the diff and evidence but cannot authorize it.
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
  (content-identical); the content-identity check is the EOL-only normalized-blob
  SHA-256 comparison from U1's Verify section (per staged text path: canonicalize
  ONLY CRLF->LF in the base blob `HEAD:<path>` and the index blob `:<path>`, then
  require `SHA-256(canon(base)) == SHA-256(canon(index))`), EXCLUDING the
  intentional `.gitattributes` change and all declared binaries. It does NOT rely
  on `git diff -w`, which would ignore all intra-line whitespace and mask a
  genuine non-EOL edit; `.gitattributes` is excluded because it genuinely changes
  content by design.
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
  the renormalization recorded BEFORE any `git add --renormalize` staging or
  working-tree refresh (not merely before the commit), gated by a clean-tree
  precondition that fails on any unrelated index/worktree change** (settled,
  blocking — see U1 operating mode and the Constitution Check Destructive Command
  Approval entry). Ship may prepare and present the diff and evidence but CANNOT
  authorize it. Not advisory.
* `ProposedAction:` bulk `gofmt -w` on residual files (U2). `ActionRisk:` LOW,
  mechanical, reversible.

**Added verification / rollback / monitoring:**

* U1 pre-approval gate: with the clean-tree precondition satisfied and after the
  affected-path-only working-tree refresh, run `git ls-files --eol`,
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
operator-only approval of the U1 renormalization is a required, blocking gate
recorded BEFORE any `git add --renormalize` staging or working-tree refresh (not
merely before the commit), gated by a clean-tree precondition (reconciled with U1
and the Constitution Check — no longer an optional/unresolved checkpoint). Ship
may prepare and present the diff and evidence; only the operator can authorize.

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

### Copilot PR #448 review cycle 1 — Stage-owned corrections (post-harvest, P-021 C1)

These corrections complete the already-authorized staging contracts for shipment
`156-S`; they add no new scope beyond honoring the existing CI-guard/line-ending
intent and do not change the PASS verdict. Deltas:

* **U1 approval gate hardened** — operator-only approval now recorded BEFORE any
  `git add --renormalize` staging or working-tree refresh (not merely before the
  commit); added a clean-tree precondition (fail on any unrelated index/worktree
  change); working-tree refresh scoped to affected paths only (forbid forced
  whole-tree `git checkout-index -f -a` / removal). Plan Hardening operator-checkpoint
  signal flipped to **present**.
* **CI guard split into new unit U14 (`175.014-T`)** — owns the genuine
  `.github/workflows/ci.yml` content change so U1 stays content-identical (EOL-only);
  U14 depends on U1 and feeds the U12 terminal sink; guard inspects working-tree
  EOL via `git ls-files --eol` (fail on `w/crlf`/`w/mixed`), index-diff check kept
  as separate content-identity evidence.
* **U2 verify de-scoped** — repository-wide `gofmt -l .` moved to U12; U2 asserts
  only its owned files.
* **U3 acceptance criteria** — added self-contained task-local criteria (scope,
  targeted test, non-vacuity/canonical-byte assertion, gofmt on owned files, no
  silent `-update` fixture rewrite).
* **U11 staticcheck dependencies** — U11 now depends on all errcheck units
  (U4–U10, U13) so overlap cannot be discovered too late; U12 remains terminal.
* **U13 residual scope** — `internal/config` and `internal/faultline` top-level
  now INCLUDED with file-level exclusions for the two U3-owned files (replacing
  the prior evidence-only package exclusion Stage cannot substantiate).
* **Subtask ID examples corrected** — `175.013.a-T` → `175.013.001-ST`,
  `175.010.a-T` → `175.010.001-ST` (valid numeric `-ST` subtask IDs).

Backlog effect: 13 → 14 tasks (U1–U14); dependency edges 22 → 32; shipment
`156-S` manifest 14 → 15 items (`175-F` + 14 tasks). All mutations via governed
backlogit operations.

### Copilot PR #448 review cycle 2 — Stage-owned corrections (post-harvest, P-021 C1)

Same-contract-surface planning/handoff corrections; no new scope, PASS verdict
unchanged. Deltas:

* **Exactly-one-owner DAG redesign (findings 4 + 7)** — U11 no longer depends on
  the errcheck units. Instead U11 runs IMMEDIATELY after U1 as the
  staticcheck/errcheck overlap-inventory owner: it owns every staticcheck-flagged
  file (minus the two U3-owned files) for ALL findings (staticcheck + errcheck) and
  records that owned-file set. U4–U10 and U13 now depend on U11 and EXCLUDE its
  recorded files, and U2 (residual gofmt) now runs LAST (after U3, U4–U11, U13). The
  prior "merge into owning unit OR rely on serialization" wording — which permitted
  two owners on one file — is removed; ownership is fixed by the up-front inventory,
  never by serialization order. Dependency edges 32 → 41 (U11→U4–U10/U13 reversed to
  U4–U10/U13→U11: −8/+8; U2→U1 replaced by U2→{U3,U4–U11,U13}: −1/+10). DAG remains
  acyclic; U12 terminal; U14 terminal-fed.
* **U1 refresh-set derivation (finding 2)** — the working-tree refresh path set is
  now derived from the UNION of staged renormalized paths AND every
  `eol=lf`-governed tracked path whose working-tree EOL is `w/crlf`/`w/mixed`
  (`git ls-files --eol`), not the staged-diff names alone — because the index may
  already be LF while the working tree is CRLF (renormalize would stage nothing).
  Clean-tree precondition, operator-only pre-approval, scoped refresh, and no forced
  whole-tree checkout are retained.
* **U14 Windows guard (finding 3)** — the persistent `git ls-files --eol`
  working-tree guard MUST run on `windows-latest` AFTER checkout, and MUST run on
  every PR/push (no `paths:`/changed-files skip) so docs/backlog-only changes still
  exercise the checkout + guard. The index-identity/renormalize-exit-code check
  stays SEPARATE (may run on `ubuntu-latest`). The workflow file itself is NOT
  edited during staging.
* **Deliberation source-kind (finding 6)** — `4DB1DFF1` machine kind corrected from
  `tech-debt` to the actual `task` in the deliberation grouped-entries table.

Backlog effect: 14 tasks (U1–U14) unchanged; dependency edges 32 → 41; shipment
`156-S` manifest unchanged at 15 items (`175-F` + 14 tasks). All mutations via
governed backlogit operations.

### Copilot PR #448 review cycle 3 — Stage-owned corrections (post-harvest, P-021 C1)

Same-contract-surface planning/handoff corrections; no new scope, PASS verdict
unchanged. All three findings are on the harness-contract / CI-contract surface:

* **Invalid generic harness exemptions removed (finding 1)** — U4–U10, U11, and
  U13 previously declared a "PRE-DECLARED mechanical harness-exemption contract …
  recorded in the closure artifact at execution." That is NOT a valid P-002.1
  contract: `mechanical`/generic/closure-time is not a recognized exemption class
  (closed vocabulary: `docs-only` / `verification-only` / `covered-by`) and it
  carries none of the required canonical metadata (label, contract block, closed
  exempt-set membership). All such language is removed from the tasks and from the
  plan's Test-first note and Constitution Test-First entry; U4–U10, U11, and U13
  are now NORMAL harness-required units gated by a real red harness authored by the
  harness-architect (P-002/P-004). No `covered-by` was fabricated — no existing
  predecessor harness supports one, and normal harness-required is preferred over
  speculative coverage. No retrospective waiver added.
* **U12 canonical verification-only exemption (finding 2)** — U12 (`175.012-T`) is
  verification-only with no implementation deliverable. It now carries the exact
  canonical P-002.1 contract: the `harness-exempt` label, a
  `<!-- BEGIN/END:harness-exemption-contract -->` block with the five canonical keys
  (class `verification-only`, reason, `harness_owner: none`, an exact
  must-fail-before-deliverable `exempt_verification_command` asserting the evidence
  artifact `docs/closure/175-baseline-convergence-convergence-evidence.md` records
  all four gates green and printing `EXEMPT_VERIFY_OK:175.012-T`, and
  `exempt_precondition: must-fail-before-deliverable`), plus membership in the new
  closed harness-exempt set (U12 sole member; see the "Harness-Exempt Set (closed)"
  section). Ship can statically classify U12 without scaffolding an impossible
  implementation harness.
* **U14 workflow scope expanded (finding 3)** — the current
  `.github/workflows/ci.yml` triggers on `pull_request` only and skips the Windows
  checkout for docs/backlog-only changes, so a guard-step-only edit cannot make the
  guard always-run on the relevant push and PR events. U14's ownership expands —
  within the SAME one file — to three coordinated changes: add a `push:` trigger for
  protected branches (reconciling the file's "PR-only" header comment), add a
  dedicated always-run Windows guard job (no `needs: changes`, no changed-files skip;
  the scoped `test-windows` (167.013-T) job is left unchanged), and the
  `git ls-files --eol` guard step. Kept as one CI/config task under the 2-hour rule;
  task acceptance updated so the Windows guard runs on the relevant push AND PR
  events and cannot be skipped by changed-file conditions.

Backlog effect: 14 tasks (U1–U14) unchanged; dependency edges unchanged at 41;
shipment `156-S` manifest unchanged at 15 items (`175-F` + 14 tasks). U12 gains the
`harness-exempt` label (no dependency/membership/DAG change). All mutations via
governed backlogit operations / canonical artifact edits.
