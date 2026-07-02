---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for hardening the create/update write paths against a nil-HeaderDef fail-open: internal/core/artifacts.go CreateArtifact (line 224) and UpdateArtifact (line 514) gate ApplyFieldDefaults/ValidateArtifactFields behind `if ws.HeaderDef != nil`, so an absent workspace schema silently skips required-field validation and the write succeeds. Fix: fail closed via a shared requireHeaderDef(ws) helper that returns a plain (non-ErrValidation) system/config-fault error, mapping to MCP `internal` and a non-zero CLI exit, keeping CLI + MCP consistent through the single shared core write functions. Third instance of the nil-precondition-fail-open shape traced to the exported-cache zero-value compound learning; sibling of the shipped 072-S doctor --target fix.'
doc_type: plan
ingested_at: "2026-07-02T11:20:00Z"
schema_version: "1.0"
source: docs/exec-plans/2026-07-02-artifacts-write-nil-headerdef-hardening-plan.md
title: 'create/update write paths: fail closed on nil header-def (stop silent skip)'
---

## Source

- Stash: `266816CE` (kind=task, priority=medium) — "[072-S follow-up] Harden create/update
  write paths against nil HeaderDef fail-open."
- Origin: surfaced during review of PR #158 (072-S, merged `d3f0fac`) as the direct sibling of
  the doctor `--target` nil-HeaderDef fix. Recorded as follow-up in
  `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-closure.md` ("Follow-up") and named
  explicitly as the third recurrence site in the compound learning below.
- Prior art (directly analogous, already shipped + reviewed):
  - `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md` — 072-S plan and
    its Option-A-vs-B decision (nil schema = system/config fault, not user-input validation error).
  - `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-closure.md` — 072-S operational closure.
- Compound learning (canonical prior, high-confidence match; explicitly names this stash):
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md` — governing rule:
  "when a correctness check is gated on a precondition, an absent precondition must route to the
  fail-closed path, never to skip-and-succeed." The doc's "Reinforcement — 072-S" section calls
  out `internal/core/artifacts.go:224` and `:514` as the same `if ws.HeaderDef != nil` fail-open
  shape stashed as `266816CE`. This plan is the third application of that rule.
- Local sibling precedent inside the same package (the decisive error-mapping anchor):
  `internal/core/artifact_size.go:validateSizeValue` (line 100) already fails closed on a nil
  HeaderDef with a **plain** error (`"cannot validate size: header-def not loaded"`) — which the
  MCP layer maps to `internal` — while reserving `blerrors.ErrValidation` wrapping for the
  user-correctable case (type has no size field). `internal/core/metadata_catalog.go:110` does the
  same (`"header-def metadata is required"`). This plan mirrors that established convention exactly.
- No separate deliberation artifact was created: the hard-fail-vs-warn choice and the
  error/exit/MCP mapping resolve cleanly against three convergent, already-settled precedents
  (072-S Option B, the local `validateSizeValue` sibling, and the compound learning). Per the
  lean-plan directive for a small, single-file defensive-hardening change, the rationale is folded
  into the Decisions and Rationale section below instead of a `docs/decisions/` file.

## Problem Frame

`internal/core/artifacts.go` gates header-def field handling behind `if ws.HeaderDef != nil` at
two write-path call sites:

- **`CreateArtifact` (line 224)** — gates **both** `ApplyFieldDefaults` **and**
  `ValidateArtifactFields`:

  ```go
  // Apply field defaults and validate against header-def if available.
  if ws.HeaderDef != nil {
      if err := ApplyFieldDefaults(artifact, ws.HeaderDef); err != nil {
          return nil, fmt.Errorf("apply field defaults: %w", err)
      }
      if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
          return nil, fmt.Errorf("validate artifact fields: %w", err)
      }
  }
  ```

- **`UpdateArtifact` (line 514)** — gates `ValidateArtifactFields`:

  ```go
  // Validate against header-def if available.
  if ws.HeaderDef != nil {
      if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
          return nil, fmt.Errorf("validate artifact fields: %w", err)
      }
  }
  ```

When `ws.HeaderDef == nil`, required-field validation is **silently skipped** and the write
proceeds — a fail-open defect. On the create path a nil schema additionally skips field-default
application, so the artifact is persisted neither defaulted nor validated. A *skipped* validation
is indistinguishable from a real *pass*: the write succeeds as if the artifact were well-formed.

Both surfaces inherit the defect because both route through these single shared core functions
(confirmed): the MCP tools `backlogit_create_item` / `backlogit_update_item`
(`internal/mcp/tools.go:700`, `:550`, `:750` → `core.CreateArtifact` / `core.UpdateArtifact`) and
the CLI commands `backlogit add` / `update` / `move`
(`internal/cli/add.go:116`, `update.go:139`, `move.go:38` → the same core functions).

**Reachability.** `core.NewWorkspace` (`internal/core/workspace.go:69-76`) loads `header-def.yaml`
via `config.LoadHeaderDef`; when the file is genuinely absent (`os.ErrNotExist`) it sets
`headerDef = nil` and **opens the workspace anyway**. In a normally-initialized workspace
`config.WriteDefaults` writes `header-def.yaml` at init, so `HeaderDef` is populated and this
branch is unreachable. A nil `HeaderDef` at write time therefore means an uninitialized or
schema-stripped workspace. This is **defensive hardening of an edge path** (non-regression,
medium priority), not a live user-facing bug — the identical reachability profile as 072-S. The
fix must still be test-first: the test forces `ws.HeaderDef = nil` explicitly to exercise the
defensive branch.

## Requirements Trace

| Requirement (from stash `266816CE`) | Implementation action |
|---|---|
| Stop silently skipping required-field validation on create/update when `ws.HeaderDef` is nil | Replace both `if ws.HeaderDef != nil { ... }` guards with an explicit fail-closed precondition check that returns an error before any persist |
| Decide hard-fail vs warn for write-time absent header-def | **Hard-fail (fail closed).** Silently skipping a validation on a mutation write is an unacceptable fail-open at a safety boundary; refuse the write (see Decisions) |
| Keep CLI + MCP surfaces consistent | Fix lives in the single shared `CreateArtifact` / `UpdateArtifact`; both surfaces inherit structurally — no per-surface duplication |
| Handle the create-path defaults nuance | Fail closed **before** `ApplyFieldDefaults` so a nil schema never yields a persisted-but-undefaulted-and-unvalidated artifact |
| Two call sites, one shape | One shared `requireHeaderDef(ws)` helper used at both sites (mirrors the 072-S single-shared-function argument) |
| Map the fault to the right error surface | Return an error wrapping `blerrors.ErrConfig` (NOT `ErrValidation`) → MCP `internal` (500), non-zero CLI exit — a system/config fault, not user-input validation (see Decisions) |
| Defensive path must still be test-first | Add failing tests that force `ws.HeaderDef = nil` and assert create/update fail closed with no persisted mutation |

## Implementation Units

### T1 — Fail closed on nil header-def in the create/update write paths (test-first)

- **Execution posture:** test-first (TDD).
- **Files (2):**
  - `internal/core/artifacts_headerdef_test.go` (new; add failing tests first). Package
    `core_test`, reusing the existing `setupTestWorkspace(t)` helper from
    `artifacts_expansion_test.go` (which runs `config.WriteDefaults` → HeaderDef loaded).
  - `internal/core/artifacts.go` (implement after red: add the shared `requireHeaderDef` helper
    and rewire both call sites). Single file, no new imports (`fmt` already imported).
- **Functions touched (3):** new `requireHeaderDef` helper; `CreateArtifact` (line 224 block);
  `UpdateArtifact` (line 514 block).
- **Shared helper (new, same file):**

  ```go
  // requireHeaderDef fails closed when the workspace header-def schema is not
  // loaded. The create/update write paths gate required-field validation and
  // default application on the header-def; an absent (nil) schema is a
  // system/config precondition fault, so the write must refuse rather than
  // silently skip validation and succeed — the same fail-open shape closed for
  // the doctor --target path in 072-S. It is wrapped in blerrors.ErrConfig (NOT
  // blerrors.ErrValidation): a missing workspace schema is not a user-correctable
  // field error. ErrConfig is absent from domainError's validation case, so the
  // MCP layer still surfaces it as `internal` (500), never `validation_failed`
  // (422) — while giving callers/tests a positive errors.Is seam instead of a
  // brittle message-substring match. blerrors is already imported in this file.
  func requireHeaderDef(ws *Workspace) error {
      if ws.HeaderDef == nil {
          return fmt.Errorf("header definition not loaded; cannot validate artifact fields: %w", blerrors.ErrConfig)
      }
      return nil
  }
  ```

- **Create-path change (line ~223-231):** hard-fail before defaults/validation.

  ```go
  // A nil header-def means the workspace schema is absent, so required-field
  // validation and default application cannot be performed. Fail closed rather
  // than silently skip them and persist an unvalidated artifact. This check MUST
  // precede ApplyFieldDefaults/ValidateArtifactFields: both call
  // headerDef.ResolveFieldSchema, which dereferences the (now nil) receiver with
  // no nil-guard, so removing the old `if != nil` guard without failing closed
  // first would nil-pointer panic. The ordering is a load-bearing invariant.
  if err := requireHeaderDef(ws); err != nil {
      return nil, err
  }
  if err := ApplyFieldDefaults(artifact, ws.HeaderDef); err != nil {
      return nil, fmt.Errorf("apply field defaults: %w", err)
  }
  if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
      return nil, fmt.Errorf("validate artifact fields: %w", err)
  }
  ```

- **Update-path change (line ~513-518):** hard-fail before validation.

  ```go
  // Fail closed when the workspace schema is absent (see requireHeaderDef).
  if err := requireHeaderDef(ws); err != nil {
      return nil, err
  }
  if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
      return nil, fmt.Errorf("validate artifact fields: %w", err)
  }
  ```

- **Test scenarios (3, one new test file):**
  1. `TestCreateArtifact_NilHeaderDefFailsClosed` — `ws := setupTestWorkspace(t)`; first, with
     the loaded HeaderDef, `core.CreateArtifact(ctx, ws, "Loaded feature", "feature")` succeeds
     (loaded-path regression guard, mirroring the 072-S loaded-vs-nil pair discipline). Then set
     `ws.HeaderDef = nil` and call `core.CreateArtifact(ctx, ws, "Nil feature", "feature")`;
     assert: returned artifact is nil; `err != nil`; `errors.Is(err, blerrors.ErrConfig)` is true
     (the durable machine-checkable seam — proves the system/config-fault classification) AND
     `!errors.Is(err, blerrors.ErrValidation)` (proves it maps to MCP `internal`, not
     `validation_failed`); and no new artifact file was persisted (the write failed closed before
     `persistArtifact`). Uses a top-level `feature` so no parent_id is needed. A message-substring
     check ("header definition not loaded") is at most a soft/advisory assertion — the sentinel is
     the contract, not the wording.
  2. `TestUpdateArtifact_NilHeaderDefFailsClosed` — create a feature with the loaded HeaderDef,
     capture its ID and on-disk title; set `ws.HeaderDef = nil`; call
     `core.UpdateArtifact(ctx, ws, id, map[string]any{"title": "Renamed"})`; assert the same
     fail-closed error shape (`errors.Is(err, blerrors.ErrConfig)`, `!errors.Is(err, ErrValidation)`)
     AND that the persisted title is unchanged (no mutation persisted — the guard precedes
     `persistArtifact`).
  3. Loaded-path regression (embedded in scenario 1's first create + a sibling assertion): with
     HeaderDef loaded, create and update still succeed with no error and the artifact persists —
     pins that the fix does not regress the normal (schema-present) path.
- **Acceptance criteria:**
  - AC1: New failing tests (`ws.HeaderDef` nil-ed) assert `CreateArtifact` and `UpdateArtifact`
    both return a non-nil error for which `errors.Is(err, blerrors.ErrConfig)` is true and
    `errors.Is(err, blerrors.ErrValidation)` is false; they fail before the impl change and pass
    after. The sentinel checks are the durable seam; any message-substring check is advisory only.
  - AC2: Both nil-HeaderDef tests assert **no artifact mutation is persisted** (create writes no
    file; update leaves the on-disk artifact unchanged) — the write fails closed before persist.
  - AC3: The loaded-HeaderDef regression assertions (create + update succeed with schema present)
    are green — no regression to the normal path.
  - AC4: The fail-closed error wraps `blerrors.ErrConfig` (not `ErrValidation`) at both sites. This
    guarantees the MCP `internal` (500) outcome **structurally**, not via a direct MCP round-trip
    test: `ErrConfig` is absent from `domainError`'s validation case (so the `update_item`/`move_item`
    path falls to its default → `internal`), and `create_item` hard-maps every core error to
    `InternalError` regardless. The core-level `errors.Is(err, blerrors.ErrConfig)` +
    `!errors.Is(err, ErrValidation)` assertions are the falsifiable proxy for that mapping; no MCP
    test is added (it would be scope creep — see Decision 3). Consistent with the local
    `validateSizeValue` / `NewMetadataCatalog` nil-HeaderDef → `internal` convention.
  - AC5: The full quality-gate set passes (Ship executes): `go test ./...`, `go vet ./...`,
    `golangci-lint run`, `gofmt -l .` — whole-repo (not narrowed), so the CLI/MCP inheritors of
    `CreateArtifact` / `UpdateArtifact` are covered.

## Dependency Graph

Single task (T1). No intra-plan dependencies. No cycles.

## Decisions and Rationale

**Decision 1 — hard-fail (fail closed), not warn.** A write that silently skips required-field
validation and succeeds is a fail-open at a safety boundary. The stash explicitly asks
"hard-fail or warn?"; a warn-and-proceed would persist an unvalidated (and, on create,
undefaulted) artifact — exactly the defect being closed. The governing compound rule ("an absent
precondition must route to the fail-closed path, never skip-and-succeed") and the 072-S precedent
both mandate fail-closed. **Chosen: refuse the write when `ws.HeaderDef == nil`.**

**Decision 2 — `blerrors.ErrConfig`-wrapped error → MCP `internal` / non-zero CLI exit.** This is
the create/update *mutation* path, not the versioned `doctor --target` exit-code contract, so the
surfaced error shape differs from 072-S (there is no 0–4 exit-code table here). The MCP
`domainError` classifier (`internal/mcp/errors.go:78`) maps errors wrapping
`blerrors.ErrValidation` to `validation_failed` (422) and all other errors (including
`ErrConfig`-wrapped) to `internal` (500) via its default case.
- **MCP mapping mechanism (precise).** The two mutation surfaces do not use the same classifier:
  `handleUpdateItem` / `handleMoveItem` (`internal/mcp/tools.go:752`, `:552`) route errors through
  `domainError` (so an `ErrConfig`-wrapped error → default case → `internal`), whereas
  `handleCreateItem` (`internal/mcp/tools.go:702`) **bypasses `domainError`** and hard-maps *every*
  core error via `InternalError(...)`. For this nil-HeaderDef fault the **outcome is identical
  (`internal`/500) on both**, so parity holds by outcome — but the mechanisms differ. (Aside, out
  of scope: this create-vs-update divergence means a genuine `ErrValidation` field error surfaces
  as `internal` on `create_item` but `validation_failed` on `update_item`; a pre-existing
  asymmetry this plan neither introduces nor fixes — noted for a possible future follow-up.)
- Options considered:
  - **Option A — wrap `blerrors.ErrValidation` → `validation_failed` (422).** *Rejected.* A nil
    workspace `HeaderDef` is not a user-correctable defect in the submitted artifact; the artifact
    may be perfectly well-formed. `validation_failed` would falsely tell the caller/agent "your
    fields are invalid" when the real fault is a missing workspace schema. Semantically wrong, and
    it is exactly the mis-blame 072-S rejected when it declined exit 1 for the doctor path.
  - **Option B — wrap `blerrors.ErrConfig` (system/config fault) → `internal` (500) / non-zero CLI
    exit.** *Chosen.* A nil `HeaderDef` is a system/config precondition fault. This reconciles
    three convergent precedents: (1) 072-S Option B classified nil HeaderDef as a system/config
    fault, not user input; (2) the local in-package sibling `validateSizeValue` fails closed on the
    identical nil-HeaderDef precondition with a non-`ErrValidation` error that maps to `internal`,
    reserving `ErrValidation` for the user-correctable branch; (3) `NewMetadataCatalog` likewise
    hard-errors on nil HeaderDef → `internal`. `ErrConfig` (`internal/errors/errors.go:7`,
    "configuration error") is the semantically exact sentinel and — being absent from
    `domainError`'s validation case — maps to the same `internal` outcome as a bare plain error,
    at **zero import cost** (`blerrors` is already imported in `artifacts.go`). Wrapping the
    sentinel (vs a bare `fmt.Errorf`) turns the core→mcp contract from an *implicit absence-of-wrap*
    into a *positive, greppable, `errors.Is`-testable* assertion, and lets the tests key on
    `errors.Is(err, ErrConfig)` instead of a brittle message substring. Adopted on the convergent
    recommendation of the Go, Architecture, and Constitution plan-review personas.
  - **Rejected sub-option — introduce a new MCP error type (e.g. reuse `workspace_not_initialized`,
    `internal/mcp/errors.go:30`, or add a `config`/`precondition_failed` type).** `workspace_not_initialized`
    is semantically close ("schema absent"), and an agent-native argument exists that a distinct,
    non-retryable type is a better agent signal than generic `internal` (which agents may auto-retry).
    *Deferred/rejected for this scope*: routing nil-HeaderDef to a distinct MCP type would require
    a new core sentinel plus changes to **both** the `domainError` mapping **and** the `create_item`
    hard-map (which bypasses `domainError`), expanding a 2-file zero-blast-radius fix into a
    cross-layer MCP contract change — and would diverge from the established local sibling
    convention (`validateSizeValue` / `NewMetadataCatalog` both → `internal`) unless those are
    changed too. Disproportionate for an edge unreachable in a normally-initialized workspace.
    Recorded as an accepted advisory in Risks (mirrors 072-S's deferral of the `io_reason`
    discriminator).

**Decision 3 — one shared helper, one task (not two).** Both call sites are the same
nil-precondition-fail-open shape and are fixed by one `requireHeaderDef(ws)` helper. Splitting
create and update into separate tasks would fragment a single cohesive change, duplicate the
helper decision, and add coordination overhead for no benefit. One task keeps the change atomic
and within the 2-hour rule (2 files, 1 new helper + 2 edited blocks, 3 test scenarios). Fixing the
shared core functions makes CLI and MCP consistent structurally — per-surface behavior tests are
unnecessary and would push the task past the 2-file / 2-hour boundary.

**Decision 4 — fail closed *before* `ApplyFieldDefaults` on the create path (load-bearing).** The
create site gates defaults **and** validation behind the same guard. Placing `requireHeaderDef`
first is not merely the safe ordering — it is **required to prevent a nil-pointer panic**: both
`ApplyFieldDefaults` and `ValidateArtifactFields` call `headerDef.ResolveFieldSchema`
(`internal/config/headerdef.go:66`), which dereferences its receiver (`h.Types[...]`) with no
nil-guard. Removing the old `if ws.HeaderDef != nil` guard without failing closed first would turn
a nil `HeaderDef` into a panic instead of a clean error. Failing closed first also means a nil
schema refuses the create before any field mutation or persist — no partial/undefaulted write
escapes. The helper doc-comment records this invariant so a future refactor does not reorder it.

**Decision 5 — no separate deliberation artifact.** The choice is a two-option classification with
a clear, convention-grounded answer already established three times in this codebase. Per the
lean-plan directive this rationale is folded here rather than into a `docs/decisions/` file.

## Risks and Caveats

- **Behavioral change on a defensive path.** Create/update with a nil `HeaderDef` moves from
  "succeed (silently unvalidated)" to "fail closed (`internal` / non-zero exit)". Because
  `HeaderDef` is always loaded in a normally-initialized workspace (`config.WriteDefaults` at
  init), no real workspace reaches this branch; blast radius is effectively zero. The change only
  fires when the schema is genuinely absent, where refusing the write is strictly better than a
  false success. Mitigation: reachability analysis above; the new tests are the durable regression
  guard.
- **Not a contract break.** There is no versioned schema or documented contract promising that
  create/update succeed against an absent header-def; the response schemas are unchanged. Only an
  undocumented fail-open defect is closed. Downstream keys on the returned artifact / MCP error
  type, and no consumer legitimately relies on nil-HeaderDef→silent-success (that reliance would
  itself be the bug).
- **Latent test/setup surface (watch item for Ship).** The whole-repo `go test ./...` gate (AC5)
  will surface any pre-existing test or code path that constructs a workspace **without**
  `header-def.yaml` and then creates/updates an artifact. None are known — the core/CLI/MCP test
  helpers use `config.WriteDefaults` or `testHeaderDef()`, so `HeaderDef` is non-nil throughout.
  If a new failure appears, treat it as a legitimately-uninitialized workspace to fix in setup
  (add the header-def), **not** as a regression to revert — the whole point is that such a
  workspace should refuse writes.
- **Message stability — the sentinel is the contract, not the message.** The diagnostic message
  ("header definition not loaded…") is NOT a stable contract and may be reworded. The stable,
  machine-checkable discriminator at the core layer is the wrapped `blerrors.ErrConfig` sentinel
  (`errors.Is`), and at the MCP layer it is the error `type` (`internal`). Tests key on the
  sentinel, not the wording. This resolves the otherwise-contradictory position of treating a
  mutable string as a discriminator.
- **CLI + MCP parity holds by outcome (mechanisms differ).** Both `backlogit_create_item` and
  `backlogit_update_item` fail closed on the same underlying fault and both surface `internal`, so
  no user/agent parity gap is introduced for the nil-HeaderDef case. The *mechanisms* differ:
  `update_item`/`move_item` map via `domainError` (→ default → `internal`), while `create_item`
  hard-maps every core error via `InternalError`. Both converge to `internal` here (the wrapped
  `ErrConfig` is not `ErrValidation`), so the outcome is symmetric. No new MCP tool-description
  change is made (see deferred advisories).
- **Other artifact-write seams are out of shape (scope boundary).** `CreateArtifact`/`UpdateArtifact`
  are the only write paths that conditionally invoke `ValidateArtifactFields` behind the
  nil-HeaderDef guard. `SetArtifactSize` (`internal/core/artifact_size.go`) performs a targeted
  size write and already fails closed on nil HeaderDef via `validateSizeValue`; the cross-artifact
  reference-rewrite / cascade-status seams persist mutations without running full field validation
  **by design** (they are not the fail-open shape this plan addresses). Reviewed and confirmed
  out-of-shape, so "both surfaces inherit the fix through the single shared core functions" means
  the create/update field-validation path specifically, not literally every artifact write.
- **Accepted advisories (deferred, recorded not actioned — mirrors 072-S).**
  1. *Distinct agent-facing MCP error type for schema-absent* (e.g. reuse `workspace_not_initialized`
     or add a `config`/`precondition_failed` type) instead of generic `internal`, for better agent
     retry/remediation semantics. Deferred: expands a 2-file fix into a cross-layer MCP-contract
     change touching both `domainError` and the `create_item` hard-map, and would diverge from the
     local sibling convention (`validateSizeValue` / `NewMetadataCatalog` → `internal`); the branch
     is unreachable in a normal workspace. The `ErrConfig` sentinel already gives library callers a
     precise programmatic signal.
  2. *MCP tool-description enrichment* documenting the new fail-closed mode on
     `backlogit_create_item`/`backlogit_update_item`. Deferred as YAGNI, tied to unreachability
     (same rationale the 072-S plan used for its MCP tool-description deferral).
  3. *Hoisting `requireHeaderDef` above the pre-hooks* (create ~top-of-function, update ~top) so a
     deterministically-failing call short-circuits before any hook side effect. AC2 (no persisted
     mutation) holds either way because `persistArtifact` runs later; keeping the check at the
     existing header-def guard sites preserves shared-helper symmetry and a minimal diff. Optional
     at implementation time; not required.

## Plan Hardening Signals

- Public API, schema, or contract change: **absent.** No new MCP error-type, no response-schema
  change. Reuses the existing sentinel `blerrors.ErrConfig` and the existing `internal` mapping
  (via `domainError`'s default case on update/move, and `create_item`'s `InternalError` hard-map).
  Closes a fail-open defect on an undocumented, normally-unreachable path.
- Security, auth, permission, or compliance-sensitive behavior: **absent.** Pure precondition
  gating on a config schema; no auth, secrets, or sensitive-data handling.
- Migration, backfill, destructive/irreversible action: **absent.** The change *prevents* a write
  (the safe direction); no data or config is mutated, and the code change is fully reversible via
  `git revert`.
- External integration, operator checkpoint, or external dependency: **absent.** The create/update
  core functions are consumed cross-repo by autoharness via MCP, but a nil `HeaderDef` is
  unreachable in a normally-initialized workspace, so real behavior is unchanged.
- High runtime, rollout, or rollback risk: **absent.** One shared helper, two adjacent call sites,
  single file; blast radius effectively zero; trivially reversible.

**Requires plan hardening: no**

## Runtime Verification and Closure

- **Runtime surface changed:** CLI (`backlogit add` / `update` / `move`) exit code and the MCP
  `backlogit_create_item` / `backlogit_update_item` error payloads — both only for the (normally
  unreachable) nil-`HeaderDef` case. No change to the schema-present create/update paths.
- **Verification before absorbed (full quality-gate set, run in order, none skipped):**
  `go test ./...` (whole-repo — new nil-fail-closed create/update tests + loaded regression green,
  existing create/update/migrate tests green, CLI/MCP inheritors covered), then `go vet ./...`,
  then `golangci-lint run`, then `gofmt -l .`. A narrowed `go test ./internal/core/...` is
  insufficient because the CLI and MCP surfaces inherit these functions. No manual runtime
  spot-check is required because the branch is unreachable without deliberately nil-ing
  `HeaderDef`, which the unit tests do directly.
- **Operational closure:** no monitoring or rollback trigger needed for a zero-blast-radius
  defensive edge fix. Closure = merged PR with the new tests as the durable regression guard.

## Constitution Check

- 2-hour rule: 1 task, 2 files, 3 test scenarios. Production functions touched: 3 (new
  `requireHeaderDef` helper + the 2 edited call-site blocks in `CreateArtifact`/`UpdateArtifact`);
  the 2 new test functions are excluded from the `< 5 functions` production-code heuristic. PASS.
- Width isolation (Principle II/III): Go-code-only (impl + colocated unit tests). PASS.
- TDD-first (Principle II): failing nil-HeaderDef create/update tests precede impl. PASS.
- Quality gates (Principle I): full set enumerated in Runtime Verification. PASS.
- Dependency discipline (Principle VI): no new dependencies or imports; `blerrors` and `fmt` are
  already imported in `artifacts.go`. PASS.
- Workspace isolation / CLI containment (Principles III, IV): change is confined to
  `internal/core`; CLI/MCP surfaces inherit behavior without per-surface logic. N/A / PASS.
- Contract stability: no versioned schema/error-type change (reuses `internal` mapping and the
  existing `ErrConfig` sentinel). PASS.
- Observability (Principle V): structured error surfaces unchanged; `ErrConfig`-wrapped diagnostic
  added. N/A beyond that.
- Git persistence (Principle IX): standard file writes via existing persistence paths; unchanged.
  N/A.
- Context efficiency (Principle X): single-file impl + one test file; no bloat. N/A.
- Safety modes (Principle VIII): change is fail-closed and non-destructive (refuses a write).
  N/A.
- P-009 (merge commit, no self-merge): honored by Ship at landing.

## Plan Review

**Gate decision: PASS** (initial merge landed at ADVISORY — P2 findings only, no P0/P1 — with the
load-bearing P2s resolved inline by plan revision and the scope-expanding ones explicitly
deferred; final gate = PASS, cleared for harvest).

Reviewed by five personas via the `plan-review` skill: Constitution Reviewer, Go Reviewer, Scope
Boundary Auditor, Architecture Strategist, Agent-Native Parity Reviewer. (Learnings Researcher was
folded inline: the canonical prior `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`
was already applied and cited — it explicitly names this stash `266816CE` as the third recurrence
site.) No P0 or P1 findings from any persona. Plan hardening was correctly declared `no` (all five
signals absent with justification) and accepted — no `## Plan Hardening` section required.

**Findings and disposition:**

- **P2 (Go, Architecture, Constitution — convergent) — wrap `blerrors.ErrConfig` instead of a bare
  plain error, and make the durable test seam `errors.Is`, not a message substring.** RESOLVED:
  `requireHeaderDef` now returns `fmt.Errorf("… : %w", blerrors.ErrConfig)`; verified zero import
  cost (`blerrors` already imported at `artifacts.go:13`) and that `ErrConfig` is absent from
  `domainError`'s validation case, so the `internal`/500 outcome is unchanged. AC1/AC4 and the test
  scenarios now assert `errors.Is(err, blerrors.ErrConfig)` (positive) + `!errors.Is(err, ErrValidation)`
  (negative); the message substring is demoted to advisory. This also resolves the message-stability
  contradiction (the sentinel, not the wording, is the contract).
- **P2 (Agent-Native Parity) — plan's MCP mechanism claim ("both route through `domainError`") is
  inaccurate for the create surface.** RESOLVED: verified `handleCreateItem` (`tools.go:702`)
  bypasses `domainError` and hard-maps every core error via `InternalError`, while `handleUpdateItem`
  (`:752`) uses `domainError`. Decision 2 now states the precise per-surface mechanism and that the
  two converge to `internal` for this non-`ErrValidation` fault (parity by outcome). The pre-existing
  create-vs-update `ErrValidation` divergence is noted as out of scope.
- **P2 (Agent-Native Parity) — consider a distinct agent-facing MCP error type
  (`workspace_not_initialized` / config / `precondition_failed`) instead of generic `internal` for
  better agent retry semantics.** DEFERRED (accepted advisory): verified `workspace_not_initialized`
  exists (`errors.go:30`) but is for "no workspace open at all" (via `requireWorkspace`), a distinct
  condition; routing nil-HeaderDef to a distinct type would require cross-layer changes to both
  `domainError` and the `create_item` hard-map and would diverge from the local sibling convention
  (`validateSizeValue` / `NewMetadataCatalog` → `internal`). Out of the 2-file scope; recorded in
  Risks (mirrors 072-S's deferral of the `io_reason` discriminator). The `ErrConfig` sentinel gives
  library callers a precise programmatic signal in the meantime.
- **P3 (Architecture, verified) — the create-path ordering is load-bearing, not merely "safe."**
  RESOLVED: confirmed `ResolveFieldSchema` (`headerdef.go:66`) dereferences its receiver with no
  nil-guard, so `requireHeaderDef` MUST precede `ApplyFieldDefaults`/`ValidateArtifactFields` to
  avoid a nil-pointer panic. Decision 4 and the create-path change comment now record this invariant.
- **P3 (Architecture) — other artifact-write seams bypass `CreateArtifact`/`UpdateArtifact`.**
  RESOLVED: added a scope-boundary note in Risks — `SetArtifactSize` already fails closed via
  `validateSizeValue`; the ref-rewrite / cascade seams don't run full field validation by design.
  The "single shared functions" claim is scoped to the create/update field-validation path.
- **P3 (Scope) — AC4 implied an MCP round-trip assertion the tests don't perform.** RESOLVED: AC4
  reworded to state the `internal` outcome is guaranteed structurally by the `ErrConfig`/non-`ErrValidation`
  core assertions plus the existing classifier/hard-map; no MCP test is added (would be scope creep).
- **P3 (Constitution) — Constitution Check omitted several principles; function-count basis
  ambiguous.** RESOLVED: added III/IV/VI/IX/X lines and stated the production-function counting basis.
- **P3 (Go) — consider hoisting `requireHeaderDef` above the pre-hooks.** DEFERRED (accepted
  advisory): AC2 holds either way (`persistArtifact` runs later); keeping the check at the existing
  guard sites preserves shared-helper symmetry and a minimal diff. Optional at implementation time;
  recorded in Risks.

Runtime verification and operational closure are present and adequate for a zero-blast-radius
defensive edge fix. Plan is cleared for harvest.

<!-- plan-review-attempt: 1 -->
