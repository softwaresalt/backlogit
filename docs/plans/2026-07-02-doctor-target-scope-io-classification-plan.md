---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for doctor --target scope-vs-io classification: PrepareDoctorTarget must stop classifying confineToStorageRoot resolution errors (err != nil) as kind=scope with a dropped error message and instead classify them as kind=io (exit 3) carrying the wrapped underlying error text, while leaving the genuine containment-violation branch (ok == false) as kind=scope so the 071-S symlink-escape security guarantee is preserved. Exit-code-neutral (scope and io both map to exit 3); no DoctorTargetResult schema-version bump; CLI and MCP surfaces stay consistent via the single shared PrepareDoctorTarget.'
doc_type: plan
ingested_at: "2026-07-02T13:10:00Z"
schema_version: "1.0"
source: docs/exec-plans/2026-07-02-doctor-target-scope-io-classification-plan.md
title: 'doctor --target: classify path-resolution faults as io (preserve diagnostic text), keep containment violations as scope'
---

## Source

- Stash: `6B2C2E53` (kind=task, priority=low/P3) — "[071-S PR#156 Copilot follow-up J]".
- Origin: PR #156 review thread `PRRT_kwDORzozKM6Ngr6D` (merged as `531bd51`), accepted by
  the operator as a post-merge follow-up on 2026-07-01. This closes the last outstanding
  071-S review follow-up (its sibling "K" shipped as 072-S).
- Prior art / contract source: shipment `071-S` established the versioned doctor `--target`
  exit-code contract (0 pass / 1 validation / 2 timeout / 3 scope|io / 4 busy). See
  `internal/cli/doctor.go` (`doctorTargetExitCode`) and `internal/core/doctor_target.go`.
- Sibling plan (same doctor_target arc, same review-thread cohort):
  `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md` (072-S). This
  plan mirrors its structure, its "single shared function → both surfaces" argument, and its
  lean-plan (no separate deliberation) posture.
- Compound learnings (searched `docs/compound/`, 47 files). No compound entry directly covers
  `confineToStorageRoot` / scope-vs-io classification. The governing prior is the same family
  the 072-S plan cites: `exported-cache-zero-value-bypass-2026-06-29.md` and
  `empty-string-vs-sentinel-in-classification-2026-05-09.md` — "handle each classification
  case as its own explicit branch; do not let one bucket silently absorb another". This plan
  applies that rule: a resolution (IO) fault must not be silently absorbed into the `scope`
  bucket.
- No separate deliberation artifact was created. Per the operator's lean-plan directive and
  the Stage lean-scaling guidance, the decision is a two-option classification with a clear
  contract-grounded answer resolved by direct code inspection; the A-vs-B trade-off and the
  scope-vs-io discriminator choice are captured in the Decisions and Rationale section below.

## Problem Frame

`internal/core/doctor_target.go:PrepareDoctorTarget` (line ~128–131) resolves and confines the
target path via `confineToStorageRoot`, then handles a non-nil resolution error like this:

```go
resolved, ok, err := confineToStorageRoot(ws, filePath)
if err != nil {
    return "", nil, newDoctorTargetResult(filePath, DoctorTargetScope) // (1) misclassified + text dropped
}
if !ok {
    res := newDoctorTargetResult(filePath, DoctorTargetScope)
    res.Message = fmt.Sprintf("path outside workspace storage root: %s", filePath) // (2) correct: real scope
    return "", nil, res
}
```

Branch (1) is the defect (the review-thread `//nolint:nilerr` observation): when
`confineToStorageRoot` returns a non-nil `err`, the outcome is classified `kind=scope` **and the
underlying error text is discarded** (no `Message` is set). But `err != nil` from
`confineToStorageRoot` is exclusively an **IO / path-resolution fault** — it is returned only
from the two `filepath.Abs(...)` calls (`"resolve storage root: %w"` at line ~219 and
`"resolve target path: %w"` at line ~229). It is never returned for a genuine containment
violation. Genuine containment violations return `(ok == false, err == nil)` via branch (2) and
the three in-function `return absTarget, false, nil` sites (storage-root-itself, lexical
non-prefix, and the `!pathContained` symlink-escape guard). So branch (1) mislabels a
system/config fault as a scope violation and drops the diagnostic that would tell an operator
what actually failed.

Reachability: in a normally-initialized workspace `ws.RootPath` is always absolute (set by
`NewWorkspace`), so `WorkspaceStorageRoot(ws.RootPath)` and the joined target are absolute, and
`filepath.Abs` on an absolute path does not error. Branch (1) is therefore an **unreachable
defensive path** under normal inputs — non-regression, P3, exit-code-neutral. The fix is
correctness + diagnostic preservation, not a live-user-facing bug (matching the stash: "Exit
code is identical (scope→3, io→3). Pre-existing, non-regression, P3.").

Both surfaces route through this one function:
- CLI: `internal/cli/doctor.go:runDoctorTargetMode` → `core.PrepareDoctorTarget` (line 171).
- MCP: `internal/mcp/tools.go:handleDoctor` → `core.DoctorTarget` → `PrepareDoctorTarget`
  (line 1798).

So a single core-level fix covers both surfaces (mirroring the 072-S structural argument;
confirmed by inspection, not assumed).

## Requirements Trace

| Requirement (from stash 6B2C2E53) | Implementation action |
|---|---|
| Stop classifying `confineToStorageRoot` resolution errors as `kind=scope` | In `PrepareDoctorTarget` branch (1), set `kind=DoctorTargetIO` instead of `DoctorTargetScope` |
| Preserve / wrap the underlying resolution error so diagnostic text is not dropped | Set `res.Message = fmt.Sprintf("confine target to storage root: %v", err)` on that branch |
| Classify as `io` only when the failure is an IO / path-resolution error | Use the existing `(ok, err)` contract: non-nil `err` ⟺ resolution/IO fault → `io`; `ok == false` (nil err) ⟺ containment violation → stays `scope` (unchanged) |
| Do not weaken the 071-S symlink-escape security guarantee | Leave branch (2) and the three `ok == false` return sites in `confineToStorageRoot` untouched; genuine escapes still return `ok == false` → `kind=scope` |
| Keep the change exit-code-neutral and schema-stable | No new kind, no `doctorTargetExitCode` change (both scope and io already → exit 3), no `DoctorTargetResult` schema-version bump |
| Keep CLI + MCP surfaces consistent | Fix lives in the single shared `PrepareDoctorTarget`; both surfaces inherit it structurally |
| Defensive path must still be test-first | The buggy branch is unreachable via normal inputs, so introduce a narrow unexported boundary seam (`var confineFn = confineToStorageRoot`) that a test overrides to force the resolution error and assert `kind=io` + preserved message. This leaves the security-sensitive `confineToStorageRoot` internals byte-for-byte untouched |

## The Core Decision (scope-vs-io discriminator)

**How `PrepareDoctorTarget` distinguishes a genuine out-of-scope containment violation (stays
`kind=scope`) from an IO/path-resolution failure (becomes `kind=io`, preserve text):**

**Chosen mechanism — reuse the existing two-value `(ok, err)` contract of
`confineToStorageRoot`. No new sentinel error is introduced.**

`confineToStorageRoot` already partitions its outcomes cleanly:

| Return shape | Meaning | Current caller kind | Correct kind |
|---|---|---|---|
| `err != nil` | `filepath.Abs` failed resolving storage root or target — an IO/path-resolution fault | `scope` (bug: text dropped) | **`io`** (with wrapped text) |
| `ok == false, err == nil` | storage-root-itself, lexical non-prefix, or `!pathContained` symlink escape — a genuine containment violation | `scope` | `scope` (unchanged) |
| `ok == true, err == nil` | in scope | proceed to lock | proceed (unchanged) |

Because the partition already exists and is exhaustive, the fix is a one-branch reclassification
plus a message. **A sentinel error (`ErrNotContained` / `ErrOutOfScope`) is explicitly NOT
needed** and would be redundant complexity: the `ok == false` cases already carry the
containment-violation semantics and are already covered by green tests
(`TestDoctorTarget_OutsideStorageRootRejectedAsScope`,
`TestDoctorTarget_SymlinkEscapeRejectedAsScope`).

**Security-guarantee preservation (071-S symlink escape):** the `!pathContained` guard lives in
`confineToStorageRoot` and returns `(absTarget, false, nil)` — i.e. branch (2)/`ok == false`,
which this plan leaves entirely untouched. A real symlink escape therefore still yields
`kind=scope` and is never read through. The fix only narrows branch (1) — the `err != nil`
resolution-fault path — so it cannot weaken the escape guarantee. This is proven by the existing
`TestDoctorTarget_SymlinkEscapeRejectedAsScope` and
`TestDoctorTarget_OutsideStorageRootRejectedAsScope` remaining green (they exercise `ok == false`
paths, not the `err != nil` path).

**Exit-code neutrality:** `doctorTargetExitCode` maps `DoctorTargetScope, DoctorTargetIO` to the
same `case` → exit 3. Reclassifying branch (1) from scope to io changes neither the process exit
code nor the JSON `kind`→exit mapping table, and touches no `DoctorTargetResult` field, so no
schema-version bump is required. The observable delta is: (a) a correct `kind` and (b) a
non-empty, informative `Message` on a formerly text-dropping path.

## Implementation Units

### T1 — Classify path-resolution faults as io in PrepareDoctorTarget (test-first)

- **Execution posture:** test-first (TDD).
- **Files (2):**
  - `internal/core/doctor_target_test.go` (add the failing test first).
  - `internal/core/doctor_target.go` (implement after red: reclassify branch (1) + add the
    unexported resolution seam; same file, no new exported symbol).
- **Functions touched (1, same file):** `PrepareDoctorTarget` (branch (1) reclassification only).
  Plus one new unexported package-level boundary seam `var confineFn = confineToStorageRoot`. The
  security-sensitive `confineToStorageRoot` is left byte-for-byte unchanged (the seam wraps the
  call at the `PrepareDoctorTarget` boundary, not the function's internals).
- **Test entry point:** exercise the public wrapper `DoctorTarget(ws, path)` (which calls
  `PrepareDoctorTarget` and returns the short result when `short != nil`), matching every
  existing test in `doctor_target_test.go` and exercising the exact CLI + MCP path.
- **Change ordering (P2 fix — preserve a compiling assertion-red before impl, per Constitution
  Principle II):**
  1. **Land the behavior-neutral seam first (all existing tests stay green).** Add the unexported
     package-level `var confineFn = confineToStorageRoot` and change `PrepareDoctorTarget` to call
     `confineFn(ws, filePath)` instead of `confineToStorageRoot(ws, filePath)`. Production behavior
     is byte-for-byte identical (default value is the real function); the whole suite stays green.
     This is a pure refactor step.
  2. **Add the failing test (compiles; assertion-level red).** The test overrides `confineFn`;
     because the seam now exists the `core` package compiles, and the assertion goes red against the
     CURRENT `kind=scope` / empty-`Message` behavior — a genuine assertion red, NOT a compile error.
  3. **Apply the reclassification (green).** Change branch (1) in `PrepareDoctorTarget`.

  ```go
  // package-level, unexported boundary seam so the (normally unreachable) path-resolution
  // fault branch is deterministically testable WITHOUT modifying the security-sensitive
  // confineToStorageRoot. MUST remain a pure pass-through in production (default below);
  // only the single non-parallel test in doctor_target_test.go may override it (defer-restore).
  var confineFn = confineToStorageRoot
  ```

  Branch (1) reclassification in `PrepareDoctorTarget` (step 3), calling the seam:

  ```go
  resolved, ok, err := confineFn(ws, filePath)
  if err != nil {
      // A path-resolution fault is a system/config IO error, not a scope
      // violation. Classify it as io (exit 3, same as scope) and preserve the
      // wrapped underlying error text instead of dropping it.
      res := newDoctorTargetResult(filePath, DoctorTargetIO)
      res.Message = fmt.Sprintf("confine target to storage root: %v", err)
      return "", nil, res
  }
  if !ok {
      res := newDoctorTargetResult(filePath, DoctorTargetScope)
      res.Message = fmt.Sprintf("path outside workspace storage root: %s", filePath)
      return "", nil, res
  }
  ```

- **Test scenarios (2):**
  1. **Guard (b) — new, fails-first — IO reclassification with preserved text.**
     `TestPrepareDoctorTarget_ResolutionErrorIsIO` (non-parallel): temporarily override `confineFn`
     to return `("", false, errors.New("boom-resolve"))` with a `defer` restoring (and asserting
     the restore of) the original, then call `DoctorTarget(ws, path)` for any path, and assert
     `res.Kind == DoctorTargetIO`, `res.OK == false`, and `res.Message` contains the sentinel text
     `"boom-resolve"` (proving the underlying error is preserved, not dropped). **Assertion-red
     before the fix** — with the seam already landed in step 1 the package compiles and the test
     observes `kind=scope` with an empty `Message`; **green after** step 3. The test MUST NOT call
     `t.Parallel()` because it mutates a package-level seam.
  2. **Guard (a) — regression, authored/confirmed — containment violations stay scope.**
     Assert the security-preserving branch is unchanged. The existing
     `TestDoctorTarget_OutsideStorageRootRejectedAsScope` (parent-traversal, `docs/` path,
     absolute-outside path → `kind=scope`) and `TestDoctorTarget_SymlinkEscapeRejectedAsScope`
     (symlink escape → `kind=scope`) MUST remain green with the seam at its default value. Add
     one explicit assertion in T1's new test region that a lexical out-of-scope path still
     returns `kind=scope` with a non-empty `Message` containing `"path outside workspace storage
     root"`, so the scope branch's message contract is locked alongside the new io branch (an
     explicit scope-vs-io pair, so the classification precedence is deterministic and does not
     rely solely on pre-existing tests).
- **Acceptance criteria:**
  - AC1: A new unit test that forces a `confineToStorageRoot` resolution error (via the
    `resolveAbs` seam) and asserts `kind=io`, `OK=false`, and a `Message` containing the
    underlying error text fails before the impl change and passes after.
  - AC2: The existing scope tests (`TestDoctorTarget_OutsideStorageRootRejectedAsScope`,
    `TestDoctorTarget_SymlinkEscapeRejectedAsScope`) remain green — genuine containment
    violations (including symlink escapes) still return `kind=scope` (071-S security guarantee
    preserved), plus the new authored lexical-out-of-scope assertion (scenario 2) is green.
  - AC3: `doctorTargetExitCode` is unchanged; both scope and io continue to map to exit 3 — no
    new kind, no `DoctorTargetResult` schema-version bump, no exit-code table edit.
  - AC4: In production (seam at default `filepath.Abs`) behavior is byte-for-byte unchanged for
    every reachable path; only the unreachable resolution-fault branch is reclassified.
  - AC5: The full quality-gate set passes (Ship executes, run in order, none skipped):
    `go test -race ./...` (whole-repo — new io-reclassification test + existing scope/io/pass tests
    green, and the CLI/MCP surfaces that inherit `PrepareDoctorTarget` covered; `-race` matches the
    Makefile's canonical `make test` and durably guards the seam-mutating test), then
    `go vet ./...`, then `golangci-lint run`, then `gofmt -l .`.

## Dependency Graph

Single task (T1). No intra-plan dependencies. No cycles.

## Decisions and Rationale

**Decision: on `confineToStorageRoot` returning `err != nil`, return `kind=DoctorTargetIO` (exit
3) with the wrapped resolution error preserved in `Message`; leave the `ok == false` branch as
`kind=DoctorTargetScope`. (Option B.)**

Options considered:

- **Option A — leave the branch as `scope` and only add a `Message`.** *Rejected.* This preserves
  the misclassification the stash calls out: a `filepath.Abs`/resolution fault is an IO/system
  fault, not a boundary violation. Reporting `scope` mislabels a system fault as a security
  boundary rejection and misleads an operator/gate into "your path escaped the root" when the
  real fault is "the path could not be resolved". Semantically wrong even though exit-code-neutral.

- **Option B — reclassify to `io` and wrap the underlying error text. (Chosen.)** A resolution
  fault is a system/config precondition fault; the 071-S contract already routes such faults
  (unreadable/undecodable target, non-contention lock IO failures, absent header-def schema) to
  `io`/exit 3. This makes branch (1) internally consistent with every other IO fault in the
  function and preserves the diagnostic the operator needs. Exit-code-neutral (both → 3), so it
  does not alter the shipped 0–4 contract.

- **Rejected sub-option — introduce a sentinel error (`ErrOutOfScope` / `ErrNotContained`) in
  `confineToStorageRoot` to distinguish scope from io.** Unnecessary. The existing `(ok, err)`
  two-value contract already distinguishes them exhaustively (see The Core Decision table), and
  the `ok == false` scope cases are already test-covered. Adding a sentinel would be redundant
  API surface for a P3 diagnostic fix and would touch the containment logic that must stay
  untouched to preserve the security guarantee.

**Decision: introduce a narrow unexported boundary seam (`var confineFn = confineToStorageRoot`)
so the fix is genuinely TDD-verifiable while leaving the security-sensitive function untouched.**
The buggy branch is unreachable through normal inputs (`filepath.Abs` does not fail for the
always-absolute paths a real workspace produces), so without a seam the reclassification would
ship untested — violating the NON-NEGOTIABLE TDD-first principle. A package-level
function-variable seam is the idiomatic Go testability pattern (cf. `var timeNow = time.Now`),
is unexported (not public API), and defaults to the real `confineToStorageRoot` so production
behavior is byte-for-byte identical. Seaming at the `PrepareDoctorTarget` boundary (rather than
routing `confineToStorageRoot`'s internal `filepath.Abs` calls through a var) was chosen
deliberately: it keeps the security-sensitive `confineToStorageRoot` — including the 071-S
`!pathContained` symlink-escape guard — literally unmodified, and it seams exactly the function
whose behavior this task changes. The single test that mutates it is non-parallel and restores it
via `defer`. Rejected alternatives: (a) ship the reclassification with no deterministic test for
guard (b) (weakens durability, violates TDD-first); (b) an inner `var resolveAbs = filepath.Abs`
seam that recompiles the security function's call sites through a mutable var (larger footprint
into the containment logic for no additional coverage).

**Decision: fix the single shared `PrepareDoctorTarget` rather than each surface.** Both the CLI
(`runDoctorTargetMode` → `PrepareDoctorTarget`) and MCP (`handleDoctor` → `DoctorTarget` →
`PrepareDoctorTarget`) route through this one function. Fixing it there makes CLI and MCP
consistent structurally; per-surface duplicate behavior tests are unnecessary and would push the
task past the 2-file / 2-hour boundary.

**Decision: no separate deliberation artifact.** A two-option classification with a
contract-grounded answer, resolved by code inspection; per the lean-plan directive this rationale
is folded here instead of into a `docs/decisions/` file.

## Risks and Caveats

- **Behavioral change on an unreachable defensive path.** Branch (1) moves from `kind=scope` to
  `kind=io` (exit 3 → exit 3, unchanged) and gains a `Message`. Because `filepath.Abs` never
  fails for the absolute paths a normal workspace produces, no real workspace reaches this branch;
  blast radius is effectively zero. Mitigation: reachability analysis above; the change only fires
  when path resolution genuinely fails, in which case a correct `io` classification plus preserved
  diagnostic text is strictly better than a misleading `scope` label with no text.
- **Security guarantee is preserved, not weakened.** The symlink-escape / containment logic
  (`!pathContained` and the `ok == false` returns) is untouched. Real escapes still return
  `kind=scope`. Proven by the existing scope/symlink tests remaining green (AC2). This is the
  single most important invariant of this change and is explicitly asserted.
- **Not a contract break.** The versioned exit-code table (0–4) and `DoctorTargetResult` schema
  are unchanged. Only a misclassification + dropped-diagnostic defect on an unreachable path is
  closed. No downstream consumer relies on resolution-fault→scope (that reliance would itself be a
  bug; and the exit code is identical either way).
- **Message stability.** The new message string is diagnostic, not part of the versioned schema;
  downstream parsers key on `kind`, not `message`, so introducing the wording is safe.
- **Seam-mutation test hygiene.** The one test that overrides `confineFn` must be non-parallel and
  restore the original via `defer` (assert-restore encouraged). Called out in scenario 1 so a
  future editor does not add `t.Parallel()` and introduce a data race across tests. The seam is
  referenced ONLY inside `PrepareDoctorTarget`; no other `core` code should adopt `confineFn`.
  Verification runs under `-race` (AC5) to catch any accidental parallel mutation.

## Plan Hardening Signals

- Public API, schema, or contract change: **absent.** No new kind, no exit-code table change, no
  `DoctorTargetResult` schema-version bump (reuses `io`/exit 3). The `resolveAbs` seam is
  unexported. Closes a misclassification + dropped-diagnostic defect on an unreachable path.
- Security, auth, permission, or compliance-sensitive behavior: **security-adjacent but neutral.**
  The change is in the same function family as the 071-S scope-confinement guard, but it only
  narrows the `err != nil` (IO) branch of `PrepareDoctorTarget` and leaves the entire
  `confineToStorageRoot` function — including the containment/symlink-escape decision
  (`ok == false`, `!pathContained`) — byte-for-byte unmodified (the test seam wraps the call at the
  `PrepareDoctorTarget` boundary). The security guarantee is preserved and explicitly
  regression-tested (AC2), so no new security-sensitive behavior is introduced and no hardening
  ProposedAction/ActionRisk classification applies.
- Migration, backfill, destructive/irreversible action: **absent.** No data or config mutation;
  pure classification logic in one function plus a test seam.
- External integration, operator checkpoint, or external dependency: **absent.** The
  `doctor --target` gate is consumed cross-repo by autoharness, but the changed branch is
  unreachable in a normally-initialized workspace and the exit code is identical, so real gate
  behavior is unchanged.
- High runtime, rollout, or rollback risk: **absent.** Single-function, reversible edit; blast
  radius effectively zero.

**Requires plan hardening: no**

## Runtime Verification and Closure

- **Runtime surface changed:** CLI (`backlogit doctor --target`) JSON `kind`/`message` fields and
  the MCP `backlogit_doctor` target payload — both only for the (normally unreachable)
  path-resolution-fault case, and with an identical exit code (3). No change to the pass /
  validation / scope / busy / timeout / read-IO paths.
- **Verification before absorbed (full quality-gate set, run in order, none skipped):**
  `go test -race ./...` (whole-repo — new io-reclassification test green, existing scope/symlink/pass
  tests green, CLI/MCP inheritors covered), then `go vet ./...`, then `golangci-lint run`, then
  `gofmt -l .`. A narrowed `go test ./internal/core/...` is insufficient because the CLI and MCP
  surfaces inherit `PrepareDoctorTarget`. Manual runtime spot-check is not required because the
  branch is unreachable without the injected seam, which the unit test exercises directly.
- **Operational closure:** no monitoring or rollback trigger needed for a zero-blast-radius,
  exit-code-neutral defensive fix. Closure = merged PR with the new io-reclassification test as
  the durable regression guard and the existing scope/symlink tests confirming the security
  guarantee held.

## Constitution Check

- 2-hour rule: 1 task, 2 files, 1 function changed in one file (+1 unexported boundary seam var),
  2 test scenarios. PASS.
- Width isolation: Go-code-only (impl + test in the same package). PASS.
- TDD-first: behavior-neutral seam lands first (green), then a compiling assertion-red
  io-reclassification test (via the `confineFn` seam), then the branch reclassification (green).
  PASS.
- Quality gates (Principle I): full set enumerated in Runtime Verification (`go test ./...`,
  `go vet ./...`, `golangci-lint run`, `gofmt -l .`). PASS.
- Contract stability: no versioned exit-code/schema change (reuses `io`/exit 3). PASS.
- Security (Principle relating to scope confinement): symlink-escape/containment guarantee
  preserved and regression-tested; no weakening. PASS.
- Observability (Principle V): structured `DoctorTargetResult` unchanged; a previously-dropped
  diagnostic `Message` is now preserved. Net improvement.
- Safety modes (Principle VIII): no destructive/irreversible action. N/A.
- P-009 (merge commit, no self-merge): honored by Ship at landing.

## Plan Review

**Gate decision: PASS** (P2 resolved by plan revision; remaining findings are P3 advisory —
folded into the plan or explicitly acknowledged/deferred).

Reviewed by five personas via the `plan-review` skill against the real code
(`internal/core/doctor_target.go`, `internal/core/doctor_target_test.go`, `internal/cli/doctor.go`,
`internal/mcp/tools.go`): Go Reviewer, Constitution Reviewer, Scope Boundary Auditor, Security
Reviewer (security-lens, triggered by the scope-confinement adjacency), and Architecture
Strategist. No P0 or P1 findings from any persona. Plan hardening was correctly declared `no`
(all five hardening signals absent/neutral with justification) and this was accepted — no
`## Plan Hardening` section required.

**Security guarantee (071-S symlink escape) — CONFIRMED PRESERVED.** The Security Reviewer traced
every return path of `confineToStorageRoot` and verified that `err != nil` is returned *only* from
the two `filepath.Abs` calls (resolve storage root / resolve target path) — lexical operations that
never fail for containment reasons and never resolve symlinks. All genuine containment violations
(storage-root-itself, lexical non-prefix, and the `!pathContained` symlink-escape guard) return
`(absTarget, false, nil)` via the `ok == false` branch this plan leaves untouched; `EvalSymlinks`
errors are swallowed internally and never surface as `err`. Reclassifying the `err != nil` branch
to `io` therefore cannot reach, weaken, or bypass the symlink-escape defense. Both `scope` and `io`
are fail-closed terminal short-circuits (no read-through). Existing tests
`TestDoctorTarget_SymlinkEscapeRejectedAsScope` and `TestDoctorTarget_OutsideStorageRootRejectedAsScope`
exercise the `ok == false` path and remain a valid regression guard.

**Findings and disposition:**

- **P2 (Go Reviewer + Constitution Reviewer, deduplicated) — TDD red would be a COMPILE failure,
  not an assertion-level red.** As originally written, the test referenced the seam before it was
  introduced, so the pre-impl state would fail to compile — violating Constitution Principle II
  (NON-NEGOTIABLE: a compiling-but-failing harness must precede implementation). **RESOLVED by
  revision:** the Change Ordering now lands the behavior-neutral seam first (all tests green), then
  adds the compiling test that observes `kind=scope`/empty `Message` (genuine assertion red), then
  applies the reclassification (green).
- **P3 (Scope Boundary Auditor, echoed by Go/Constitution/Architecture) — reduce the seam's
  footprint into the security function.** **FOLDED:** switched from an inner
  `var resolveAbs = filepath.Abs` (which routed `confineToStorageRoot`'s internal calls through a
  var) to a boundary seam `var confineFn = confineToStorageRoot`, leaving the security-sensitive
  function byte-for-byte unmodified and seaming exactly the function this task changes.
- **P3 (Go Reviewer) — verification should run `-race` to match the Makefile's `make test` and
  guard the seam-mutating test.** **FOLDED:** AC5 and Runtime Verification now specify
  `go test -race ./...`.
- **P3 (Go/Constitution/Security/Architecture) — `var confineFn` is package-level mutable state
  (a `go.instructions.md` anti-pattern).** **ACKNOWLEDGED with rationale:** the function-variable
  seam is the accepted idiomatic Go testability pattern (cf. `var timeNow = time.Now`), is
  unexported, defaults to the real function (production byte-identical), is referenced only inside
  `PrepareDoctorTarget`, and is mutated by a single non-parallel `defer`-restored test verified
  under `-race`. A doc-comment stating it must remain a pure production pass-through is required in
  the change. Full dependency injection was judged premature abstraction for a normally-unreachable
  P3 defensive branch.
- **P3 (Architecture Strategist) — lock the `(ok, err)` invariant with a contract test.**
  **ACKNOWLEDGED:** the invariant "`err != nil` ⟺ IO fault; containment violations return
  `err == nil`" is already effectively pinned by the existing green tests — out-of-scope/symlink
  inputs to the real `confineToStorageRoot` yield `kind=scope` (the `err == nil`/`ok == false`
  path), never `io`. AC2 keeps those tests green, so a genuine violation surfacing as `io` would be
  caught. No additional test required for this P3 fix.
- **P3 (Scope Boundary Auditor) — scenario 2 adds an assertion on the untouched scope branch.**
  **ACCEPTED (minor, low-cost):** retained as an explicit scope-vs-io pairing so the classification
  precedence is deterministic and self-documenting; AC2's regression-check on the pre-existing
  tests remains the primary security guard.
- **P3 (Go/Constitution) — AC4 is an unreachability argument, not a directly-executable
  assertion.** **ACKNOWLEDGED:** AC4 is backed by the seam's default value and AC2's green suite;
  it reads as "existing `DoctorTarget` suite remains green with the seam at default," which is
  directly verifiable.
- **P3 (Architecture Strategist) — narrow the cohesion claim.** **ACKNOWLEDGED:** the "single
  shared function → both surfaces" claim is precisely true for the scope/io *confinement*
  classification centralized in `PrepareDoctorTarget`; `timeout`/`cancel` kinds are legitimately
  constructed in the CLL caller and are out of scope for this fix.
- **P3 (Security Reviewer) — `res.Message` may echo a filesystem path on the MCP surface.**
  **ACKNOWLEDGED (not a regression):** consistent with pre-existing path disclosure in the same
  file (scope branch echoes `filePath`; read-IO branch echoes an absolute path); no
  credentials/tokens flow through `filepath.Abs` errors; proportionate for a local single-tenant
  CLI/MCP dev tool. Revisit only if the MCP tool is exposed to untrusted remote callers.

Runtime verification and operational closure are present and adequate for a zero-blast-radius,
exit-code-neutral defensive fix. Plan is cleared for harvest.

<!-- plan-review-attempt: 1 -->
