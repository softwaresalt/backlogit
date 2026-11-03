---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for doctor --target nil-HeaderDef hardening: ValidateDoctorTargetResolved must stop returning kind=pass when ws.HeaderDef is nil (validation silently skipped) and instead fail closed with kind=io (exit 3) carrying a distinct "header definition not loaded" diagnostic, keeping the 071-S exit-code contract unchanged and both CLI and MCP surfaces consistent via the single shared validation function.'
doc_type: plan
ingested_at: "2026-07-01T10:30:00Z"
schema_version: "1.0"
source: docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md
title: 'doctor --target: fail closed on nil header-def (stop silent pass)'
---

## Source

- Stash: `C16DBBEB` (kind=task, priority=medium/P2) — "[071-S PR#156 Copilot follow-up K]".
- Origin: PR #156 review thread `PRRT_kwDORzozKM6Ngr6a` (merged as `531bd51`), accepted by
  the operator as a post-merge follow-up on 2026-07-01.
- Prior art / contract source: shipment `071-S` established the versioned doctor `--target`
  exit-code contract (0 pass / 1 validation / 2 timeout / 3 scope|io / 4 busy). See
  `internal/cli/doctor.go` (`doctorTargetExitCode`, header comment lines 54–66) and
  `internal/core/doctor_target.go`.
- Compound learning (canonical prior, high-confidence match):
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md` — governing rule:
  "when the cheap path is also the unsafe path, the zero value must fall back to the safe path";
  a nil field must mean "unknown/unseeded → take the safe branch", never "authoritative pass".
  Reinforced by `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`
  ("a validator missing the input it needs to judge is an inconclusive/system fault, not a pass")
  and `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`
  ("handle the absence case as its own explicit first branch; add a dedicated test"). This plan is
  a textbook application: nil `ws.HeaderDef` fails closed to `io`/exit 3, guarded by a test invariant.
- No separate deliberation artifact was created: the kind/exit-code decision resolves cleanly
  against the shipped 071-S contract and Ship's own thread acknowledgment (see Decisions). The
  A-vs-B trade-off is captured in the Decisions and Rationale section below per the lean-plan
  directive for a single-file defensive-hardening change.

## Problem Frame

`internal/core/doctor_target.go:ValidateDoctorTargetResolved` (around line 180–193) builds a
`DoctorTargetResult` stamped `kind=pass` after a successful frontmatter decode, then only runs
header-def required-field validation **inside** an `if ws.HeaderDef != nil` guard:

```go
res := newDoctorTargetResult(filePath, DoctorTargetPass) // OK == true
res.ArtifactID = artifact.ID
res.ArtifactType = artifact.ArtifactType

if ws.HeaderDef != nil {
    if vErr := ValidateArtifactFields(artifact, ws.HeaderDef); vErr != nil {
        res.Kind = DoctorTargetValidation
        // ...
    }
}
return res
```

When `ws.HeaderDef == nil`, the required-field validation is **silently skipped** and the
function returns `kind=pass` / `OK=true`. A *skipped* validation is therefore indistinguishable
from a real *pass* — a fail-open defect. Both the CLI gate (`internal/cli/doctor.go` →
`doctorTargetExitCode` → exit 0) and the MCP tool (`internal/mcp/tools.go:handleDoctor` →
`core.DoctorTarget` → `ValidateDoctorTargetResolved`) inherit this behavior because both surfaces
route through the same shared function.

Reachability: in a normally-initialized workspace `ws.HeaderDef` is always populated —
`config.WriteDefaults` (`internal/config/defaults.go:427`, invoked from
`internal/cli/root.go:281` at init) writes `header-def.yaml`, which is loaded via
`config.LoadHeaderDef`. So this is **defensive hardening of an edge path**, non-regression,
P2 — not a live-user-facing bug. The fix must nonetheless be test-first: the test forces
`ws.HeaderDef = nil` explicitly to exercise the defensive branch.

## Requirements Trace

| Requirement (from stash C16DBBEB) | Implementation action |
|---|---|
| When `ws.HeaderDef` is nil, stop returning `kind=pass` for a skipped validation | Add an explicit nil-HeaderDef branch in `ValidateDoctorTargetResolved` that sets a non-pass kind + `OK=false` before returning |
| Surface a distinct "header definition not loaded" diagnostic | Set `res.Message = "header definition not loaded; cannot perform required-field validation"` on that branch |
| Choose a target kind/exit-code that is internally consistent with the 071-S contract | Map to `DoctorTargetIO` (exit 3), NOT `DoctorTargetValidation` (exit 1) — see Decisions |
| Keep CLI + MCP surfaces consistent | Fix lives in the single shared `ValidateDoctorTargetResolved`; both surfaces inherit it structurally — no per-surface duplication needed |
| Defensive path must still be test-first | Add a failing unit test that constructs a workspace, sets `ws.HeaderDef = nil`, validates an otherwise-valid artifact file, and asserts `kind != pass` (specifically `kind=io`, `OK=false`) |

## Implementation Units

### T1 — Fail closed on nil header-def in ValidateDoctorTargetResolved (test-first)

- **Execution posture:** test-first (TDD).
- **Files (2):**
  - `internal/core/doctor_target_test.go` (add failing test first)
  - `internal/core/doctor_target.go` (implement after red; includes a one-line doc-comment update
    on the `DoctorTargetIO` constant — same file, no new function)
- **Functions touched (1):** `ValidateDoctorTargetResolved`.
- **Test entry point:** exercise the public wrapper `DoctorTarget(ws, path)` (NOT
  `ValidateDoctorTargetResolved(ws, filePath, absTarget)` directly — the latter requires a
  pre-confined `absTarget`). `DoctorTarget` calls `PrepareDoctorTarget` + the shared
  `ValidateDoctorTargetResolved`, so a `DoctorTarget`-level test also exercises the exact MCP path
  and the lock lifecycle, matching every existing test in `doctor_target_test.go`. The red state
  is panic-safe: pre-fix, nil never reaches `ValidateArtifactFields` (the old `!= nil` guard blocks
  it), so the assertion fails on `kind=pass` vs `kind=io` without any nil-deref.
- **Test scenarios (2):**
  1. `TestDoctorTarget_NilHeaderDefFailsClosed` — build a valid artifact file inside the storage
     root, open a workspace via the existing `config.WriteDefaults` scaffold, then explicitly set
     `ws.HeaderDef = nil` (exported field on `*Workspace`, directly settable from `package core`),
     call `DoctorTarget(ws, path)`, and assert `res.Kind == DoctorTargetIO`, `res.OK == false`, and
     `res.Message` contains "header definition not loaded". This is the durable test invariant the
     compound learning prescribes (nil ⇒ `io` ⇒ exit 3).
  2. Regression guard (authored, not deferred): a new sibling assertion that with
     `config.WriteDefaults` applied (HeaderDef loaded), a valid artifact still returns `kind=pass`
     / `OK=true` — an explicit loaded-vs-nil pair so the classification precedence is deterministic
     and does not rely solely on the pre-existing `TestDoctorTarget_ValidTaskPasses`.
- **Doc-comment update (same file, in scope):** extend the `DoctorTargetIO` constant comment
  (`internal/core/doctor_target.go` ~line 27) so the versioned kind taxonomy stays
  self-describing: `io` means a system/config fault that prevents completing validation —
  unreadable/undecodable target, lock-sidecar IO failure, OR an absent header-def schema. No
  exit-code table change; this only documents the already-broadened `io` semantics.
- **Change detail:** replace the `if ws.HeaderDef != nil { ... }` block so the nil case is
  handled explicitly:

  ```go
  if ws.HeaderDef == nil {
      res.Kind = DoctorTargetIO
      res.OK = false
      res.Message = "header definition not loaded; cannot perform required-field validation"
      return res
  }
  if vErr := ValidateArtifactFields(artifact, ws.HeaderDef); vErr != nil {
      res.Kind = DoctorTargetValidation
      res.OK = false
      res.Message = vErr.Error()
      res.FieldErrors = parseMissingFields(vErr.Error())
  }
  return res
  ```

- **Acceptance criteria:**
  - AC1: A new unit test (via `DoctorTarget(ws, path)` with `ws.HeaderDef` nil-ed) asserting
    `kind=io`, `OK=false`, and a "header definition not loaded" message fails before the impl
    change and passes after.
  - AC2: The existing valid-pass and missing-required-field-validation tests remain green
    (no regression to `kind=pass` for loaded-HeaderDef, no regression to `kind=validation`), plus
    the new authored loaded-HeaderDef regression assertion (scenario 2) is green.
  - AC3: `doctorTargetExitCode` is unchanged; nil-HeaderDef maps to exit 3 through the existing
    `DoctorTargetScope, DoctorTargetIO` case — no new kind, no schema-version bump.
  - AC4: The `DoctorTargetIO` constant doc comment is updated to describe the broadened
    (unreadable/undecodable target | lock-sidecar IO | absent header-def schema) semantics.
  - AC5: The full quality-gate set passes (Ship executes): `go test ./...`, `go vet ./...`,
    `golangci-lint run`, `gofmt -l .` — whole-repo test, not narrowed, so CLI/MCP surfaces that
    inherit this function are covered.

## Dependency Graph

Single task (T1). No intra-plan dependencies. No cycles.

## Decisions and Rationale

**Decision: on `ws.HeaderDef == nil`, return `kind=DoctorTargetIO` (exit 3) with a distinct
"header definition not loaded" message. (Option B.)**

Options considered:

- **Option A — `kind=validation` (exit 1).** Treat nil header-def as a validation failure.
  *Rejected.* Exit 1 in the 071-S contract means "the file parsed but failed header-def
  **required-field** validation" — i.e., a user-correctable defect **in the target artifact**.
  When `HeaderDef` is nil, the target artifact may be perfectly well-formed; we simply cannot
  load the schema to check it. Reporting exit 1 would falsely blame the artifact for missing
  fields and mislead the operator/gate into "fix your file" when the real fault is a missing
  workspace schema. Semantically wrong.

- **Option B — distinct "header definition not loaded" diagnostic mapped to `kind=io`
  (exit 3).** *Chosen.* A nil workspace `HeaderDef` is a **system/config precondition fault**,
  not user input error. The 071-S contract already routes system faults that prevent completing
  the check (unreadable/undecodable target, non-contention lock IO failures) to `io`/exit 3.
  Ship's own acknowledgment on review thread `PRRT_kwDORzozKM6Ngr6a` observed that a missing/nil
  header-def is arguably a system/config fault rather than user input error — Option B reconciles
  the fix with both the shipped contract and that acknowledgment. The distinct `Message`
  ("header definition not loaded…") lets operators distinguish this from an ordinary file-read IO
  fault even though both share exit 3.

- **Rejected sub-option — introduce a new kind (e.g. `config`/`internal`) + new exit code.**
  That would expand the versioned 0–4 exit-code contract, a breaking change requiring a
  `DoctorTargetResult` schema-version bump — disproportionate for defensive hardening of an
  edge path that is unreachable in a normal workspace. Reusing the existing `io`/exit 3 bucket
  keeps the cross-repo gate contract stable.

**Decision: fix the single shared `ValidateDoctorTargetResolved` rather than each surface.**
Both the CLI (`runDoctorTargetMode` → `ValidateDoctorTargetResolved`) and MCP
(`handleDoctor` → `core.DoctorTarget` → `ValidateDoctorTargetResolved`) route through this one
function. Fixing it there makes CLI and MCP consistent structurally; per-surface duplicate
behavior tests are unnecessary and would push the task past the 2-file / 2-hour boundary.

**Decision: no separate deliberation artifact.** The choice is a two-option classification with
a clear contract-grounded answer; per the lean-plan directive this rationale is folded here
instead of into a `docs/decisions/` file.

## Risks and Caveats

- **Behavioral change on a defensive path.** Nil-HeaderDef moves from exit 0 → exit 3. Because
  `HeaderDef` is always loaded in a normally-initialized workspace (`config.WriteDefaults` at
  init), no real workspace reaches this branch; blast radius is effectively zero. Mitigation:
  documented reachability analysis above; the change only fires when the schema is genuinely
  absent, in which case a hard "cannot validate" signal is strictly better than a false pass.
- **Not a contract break.** The versioned exit-code table (0–4) and `DoctorTargetResult` schema
  are unchanged. Only an undocumented fail-open defect is closed. No downstream consumer relies
  on nil-HeaderDef→pass (that reliance would itself be a bug).
- **Message stability.** The new message string is diagnostic, not part of the versioned schema;
  downstream parsers key on `kind`, not `message`, so message wording is safe to introduce.
- **Result-shape carry-over is intentional.** The nil-HeaderDef `io` return keeps
  `res.ArtifactID` / `res.ArtifactType` (set before the branch) whereas read/decode `io` returns
  leave them empty. This is deliberate: the artifact decoded cleanly — only the workspace schema
  is absent — so echoing the resolved IDs is informative. Both fields are `omitempty` and
  downstream keys on `kind`, so the minor shape divergence is harmless.
- **Accepted advisories (deferred, out of scope for this task).** The plan-review parity persona
  suggested (P3) an additive structured discriminator (e.g. `io_reason: "header_definition_not_loaded"`)
  so an agent could branch without string-matching `message`, and (P3) enriching the MCP
  `backlogit_doctor` `target` param description to enumerate the `io` outcome. Both are deliberately
  deferred: `message` is the sole discriminator for `kind=io` outcomes on both surfaces (symmetric,
  no parity break), and adding a `DoctorTargetResult` field or touching `internal/mcp/tools.go`
  would expand this from a 2-file zero-blast-radius fix into a schema/surface change (YAGNI for an
  edge unreachable in a normal workspace). Recorded here rather than actioned.

## Plan Hardening Signals

- Public API, schema, or contract change: **absent.** No new kind, no exit-code table change,
  no `DoctorTargetResult` schema-version bump. Reuses existing `io`/exit 3. Closes a fail-open
  defect on an undocumented path.
- Security, auth, permission, or compliance-sensitive behavior: **absent.**
- Migration, backfill, destructive/irreversible action: **absent.** No data or config mutation;
  pure classification logic in one function.
- External integration, operator checkpoint, or external dependency: **absent.** The
  `doctor --target` gate is consumed cross-repo by autoharness, but the changed branch is
  unreachable in a normally-initialized workspace, so real gate behavior is unchanged.
- High runtime, rollout, or rollback risk: **absent.** Single-function, reversible edit; blast
  radius effectively zero.

**Requires plan hardening: no**

## Runtime Verification and Closure

- **Runtime surface changed:** CLI (`backlogit doctor --target`) exit code and JSON `kind`/`ok`
  fields, and the MCP `backlogit_doctor` target payload — both only for the (normally
  unreachable) nil-HeaderDef case. No change to the pass / validation / scope / busy / timeout
  paths.
- **Verification before absorbed (full quality-gate set, run in order, none skipped):**
  `go test ./...` (whole-repo — new nil guard + loaded regression green, existing target tests
  green, CLI/MCP inheritors covered), then `go vet ./...`, then `golangci-lint run`, then
  `gofmt -l .`. A narrowed `go test ./internal/core/...` is insufficient because the CLI and MCP
  surfaces inherit this function. Manual runtime spot-check is not required because the branch is
  unreachable without deliberately nil-ing `HeaderDef`, which the unit test does directly.
- **Operational closure:** no monitoring or rollback trigger needed for a zero-blast-radius
  defensive edge fix. Closure = merged PR with the new test as the durable regression guard.

## Constitution Check

- 2-hour rule: 1 task, 2 files, 1 function (+1 doc-comment), 2 test scenarios. PASS.
- Width isolation: Go-code-only (impl + test + colocated doc comment). PASS.
- TDD-first: failing nil-HeaderDef test precedes impl. PASS.
- Quality gates (Principle I): full set enumerated in Runtime Verification (`go test ./...`,
  `go vet ./...`, `golangci-lint run`, `gofmt -l .`). PASS.
- Contract stability: no versioned exit-code/schema change (reuses `io`/exit 3). PASS.
- Observability (Principle V): structured `DoctorTargetResult` unchanged; distinct diagnostic
  `Message` added. N/A beyond that.
- Safety modes (Principle VIII): no destructive/irreversible action. N/A.
- P-009 (merge commit, no self-merge): honored by Ship at landing.

## Plan Review

**Gate decision: PASS** (P2 resolved by plan revision; remaining findings are P3 advisory —
some folded into the plan, the scope-expanding ones explicitly deferred).

Reviewed by six personas via the `plan-review` skill: Constitution Reviewer, Go Reviewer, Scope
Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer.
No P0 or P1 findings from any persona. Plan hardening was correctly declared `no` (all five
hardening signals absent with justification) and this was accepted — no `## Plan Hardening`
section required.

**Findings and disposition:**

- **P2 (Constitution) — verification narrowed `go test` to `./internal/core/...` and omitted the
  full quality-gate set.** RESOLVED: Runtime Verification now mandates whole-repo `go test ./...`
  + `go vet ./...` + `golangci-lint run` + `gofmt -l .` (AC5), covering the CLI/MCP inheritors.
- **P3 (Go) — test must not call `ValidateDoctorTargetResolved` directly (needs pre-confined
  `absTarget`).** RESOLVED: test entry point changed to the `DoctorTarget(ws, path)` wrapper,
  matching existing tests and exercising the MCP path + lock.
- **P3 (Constitution) — scenario 2 under-specified.** RESOLVED: scenario 2 now explicitly
  authors the loaded-HeaderDef regression assertion (AC2).
- **P3 (Learnings, high confidence) — cite the canonical prior.** RESOLVED: Source cites
  `exported-cache-zero-value-bypass-2026-06-29.md` and two reinforcing learnings; the plan applies
  their governing rule (nil ⇒ safe/fail-closed path) and the test-invariant discipline.
- **P3 (Architecture) — `DoctorTargetIO` doc comment under-describes broadened semantics.**
  RESOLVED: doc-comment update added to T1 (AC4).
- **P3 (Constitution/Go) — result-shape carry-over of ArtifactID/Type on the io branch.**
  RESOLVED: documented as intentional in Risks and Caveats (artifact decoded; only schema absent).
- **P3 (Parity) — additive structured `io_reason` discriminator; MCP tool-description enrichment.**
  DEFERRED (accepted advisory): both would expand a 2-file zero-blast-radius fix into a
  schema/surface change; `message` is the symmetric discriminator on both surfaces. Recorded in
  Risks and Caveats.
- **P3 (Architecture) — nil precondition checked after read/decode.** ACKNOWLEDGED: placement is
  deliberate (classify target-file faults before config faults; keep guard adjacent to its
  consumer `ValidateArtifactFields`). No change; the "classify target faults first" intent is now
  explicit in the plan.

Runtime verification and operational closure are present and adequate for a zero-blast-radius
defensive edge fix. Plan is cleared for harvest.

<!-- plan-review-attempt: 1 -->

