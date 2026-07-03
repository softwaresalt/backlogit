---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to consolidate the duplicated shipment-items read-edge normalization into a single exported core.NormalizeShipmentItems, delete the internal/mcp normalizeShipmentItems copy, and relocate the never-null invariant into the core function contract while keeping both named guard tests green.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-03-consolidate-shipment-items-normalization-plan.md
title: 'Consolidate shipment-items normalization into a single core source of truth'
---

## Source

- Stash: `17D29DDC` (kind=task, priority=low / tech-debt) — "Consolidate duplicate shipment items normalization."
- Origin: surfaced during plan-review of `075-F` (shipment covering-feature display); deferred there to avoid scope creep.
- Related prior work: `docs/exec-plans/2026-07-02-shipment-covering-feature-display-plan.md` (075-S), which put the shipment read-edge normalizer in play.
- Prior learnings (compound, confidence: high): `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — "Treat SQLite JSON arrays as lossy on the way back out. Normalize `[]interface{}` into `[]string` every time you read shipment `CustomFields`." The read-edge normalizer is a load-bearing invariant; two hand-maintained copies are exactly the drift risk this consolidation removes.
- No deliberation artifact was created: this is a narrowly-scoped internal refactor (single source of truth + delete one duplicate) with a signature dictated by existing callers and a single binary design decision (where the never-null invariant lives), resolved by code inspection below. Rationale is folded into this plan per Stage guidance.

## Problem Frame

Two functions independently implement the same `custom_fields["items"]` read-edge normalization (`[]any` / `[]string` / nil / unknown → `[]string`), and a third mutator wraps the same logic:

1. `internal/core/shipment.go:525` — `func shipmentItems(artifact *models.Artifact) []string` — the canonical **pure reader**. Guards nil artifact and nil `CustomFields`, maps `[]string` (clone via `append([]string(nil), items...)`), `[]any` (filter to strings, order-preserving), unknown/default → `[]string{}`. Consumed across core: `shipment.go` (lines 187, 229, 363, 482), `shipment_covering.go:61`, `shipment_lifecycle.go` (lines 68, 161), plus core tests.
2. `internal/core/shipment.go:359` — `func normalizeShipmentArtifact(artifact *models.Artifact)` — a core **mutator** that writes `CustomFields["items"] = shipmentItems(artifact)` (and lazily inits a nil map). Consumed by `CreateShipment` (line 64) and `GetShipment` (line 93); this canonicalizes the GET/CREATE path. Not named in the stash; in scope only for the mechanical call-site rename.
3. `internal/mcp/tools.go:1543` — `func normalizeShipmentItems(shipment *models.Artifact)` — an MCP **mutator** that re-implements the same `[]any`/`[]string`/nil/default switch **inline** and writes it back into `CustomFields`. Sole caller: `handleListShipments` (line 1587). **This is the duplicate the stash targets for deletion.**

Data flow that makes the MCP copy matter: `core.NewShipmentView` / `NewShipmentViews` embed the shipment artifact unchanged and JSON-marshal `CustomFields["items"]` directly. A non-nil `[]string{}` marshals to `[]`; a `nil` `[]string` marshals to `null`. The MCP list handler queries the DB (`db.QueryItems`), whose JSON decode yields `[]any`, then calls `normalizeShipmentItems` to coerce items to a non-nil `[]string` so the response is never `null`. `TestListShipments_EmptyItems_NeverNull` guards that end-to-end behavior; `TestNormalizeShipmentItems_AllCases` unit-tests the MCP copy directly.

### Duplicate confirmation (true duplicate?)

The `[]any`→`[]string` **mapping logic** is a true duplicate: both are order-preserving, filter non-string elements from `[]any`, and fall back to `[]string{}` for unknown types — behaviorally byte-equivalent. However the two functions are **not signature-identical**: the MCP function is a mutator, core's `shipmentItems` is a pure reader. There is exactly **one behavioral divergence** at the empty-slice edge:

- Empty `[]string{}` input: the MCP mutator returns non-nil `[]string{}` (via `make([]string, len(items))`); core's `shipmentItems` returns **nil** (via `append([]string(nil), items...)` which yields a nil slice when there is nothing to append).

This edge is **not reachable in the MCP list path today** (DB decode always produces `[]any`, never a live `[]string`), so the current never-null test passes regardless. But it is a latent shape difference that the consolidation must reconcile rather than silently inherit. There is no ordering, dedup, or trim difference to reconcile — neither function sorts, dedups, or trims.

## Requirements Trace

| # | Source requirement | Implementation action |
|---|---|---|
| R1 | Single exported `core.NormalizeShipmentItems` consumed by both MCP and core | Rename/export `shipmentItems` → `NormalizeShipmentItems` (same signature); update all core call sites (incl. `normalizeShipmentArtifact`) and core tests. |
| R2 | Delete the `internal/mcp` duplicate | Delete `mcp.normalizeShipmentItems`; rewrite `handleListShipments` to call `core.NormalizeShipmentItems`. |
| R3 | Core is the single source of truth | Only one implementation of the mapping logic remains (in core). |
| R4 | Preserve the MCP "never null / always `[]`" invariant | Relocate the guarantee into the core function's return contract: `NormalizeShipmentItems` never returns nil (harden the `[]string` branch to `make`+`copy`). |
| R5 | Keep `TestNormalizeShipmentItems_AllCases` and `TestListShipments_EmptyItems_NeverNull` green | Move the all-cases unit test to `internal/core` (testing the exported reader, incl. an empty-`[]string`→non-nil case); keep the never-null integration test in `internal/mcp`. |
| R6 | Do not silently change behavior | Reconcile to the superset (pure read + always-non-nil); document the convergence here and in code comments. |

## Implementation Units

### Unit 1 — Consolidate shipment-items normalization into `core.NormalizeShipmentItems` (test-first)

Single atomic Go refactor. Execution posture: **test-first** (introduce the new exported symbol via a failing/compile-red core test, then implement, then migrate call sites, then delete the duplicate, then confirm green).

**Ordered steps:**

1. **(test-first, red)** Add a core-package unit test `TestNormalizeShipmentItems_AllCases` (e.g. new file `internal/core/shipment_normalize_test.go`) that calls the *exported* `core.NormalizeShipmentItems(artifact)` for each case: nil `CustomFields`; `[]string{"a","b"}`; `[]any{"x","y"}`; `[]any{"ok", 42}` (non-string filtered); unknown type `123` → `[]string{}`; **and a new case: empty `[]string{}` input → non-nil `[]string{}` (asserts the never-null contract, `assert.NotNil` + length 0)**. This is compile-red until step 2 exists (the exported symbol does not yet exist).
2. **(implement)** In `internal/core/shipment.go`, rename `shipmentItems` → `NormalizeShipmentItems` (exported), keeping signature `func NormalizeShipmentItems(artifact *models.Artifact) []string`. Harden the `[]string` branch so empty input returns a non-nil slice:
   ```go
   case []string:
       out := make([]string, len(items))
       copy(out, items)
       return out
   ```
   Update the doc comment to make the contract unambiguous and load-bearing: (a) it is a **pure read** that returns a **non-nil** `[]string` and **does not mutate** the artifact (the deleted MCP function of the same intent was a mutator — the verb must not mislead); (b) the non-nil guarantee is a **JSON wire-shape invariant** — a nil `[]string` marshals to `null`, a non-nil empty slice marshals to `[]` — enforced through `core.ShipmentView` (marshaled by both CLI and MCP); (c) cite the f015 read-edge learning (`docs/compound/go-patterns/f015-shipment-stash-patterns.md`) and reference the guard test `TestListShipments_EmptyItems_NeverNull` so the invariant is discoverable from the function and a future maintainer does not "simplify" the `[]string` branch back to the nil-able `append([]string(nil), …)` form.
3. **(migrate core call sites)** Update every core caller of the old name to `NormalizeShipmentItems`. Enumeration below is illustrative (line numbers approximate); **`go build ./...` is the source of truth** for a complete migration. Known sites: `shipment.go` (187, 229, 363 inside `normalizeShipmentArtifact`, 482), `shipment_covering.go:61` (and the doc-comment reference at ~line 42), `shipment_lifecycle.go` (68, 161). Core test references: `shipment_atomic_test.go` (61, 103 — two calls on that line, 169), `shipment_state_integrity_test.go` (~103), `shipment_test.go` (337, 386, 476, 490, 538, 553). Prefer a `gopls`/IDE rename to make this mechanical and exhaustive.
4. **(rewrite MCP call site)** In `internal/mcp/tools.go`, replace the `normalizeShipmentItems(shipment)` call in `handleListShipments` with a thin adapter that reuses the core contract:
   ```go
   for _, shipment := range shipments {
       // The nil-map guard exists solely to provide an assignment target;
       // the never-null VALUE guarantee comes entirely from
       // core.NormalizeShipmentItems (single source of truth).
       if shipment.CustomFields == nil {
           shipment.CustomFields = map[string]any{}
       }
       shipment.CustomFields["items"] = core.NormalizeShipmentItems(shipment)
   }
   ```
   (The nil-map guard is retained only so the assignment target exists; the never-null value guarantee now comes entirely from `core.NormalizeShipmentItems`.)
5. **(delete duplicate)** Delete `mcp.normalizeShipmentItems` (tools.go:1539–1569).
6. **(test consolidation)** Before deleting, `grep` for `buildTestShipmentArtifact` across `internal/mcp` to confirm it has no other users (cheap insurance for Decision 4). Then remove `TestNormalizeShipmentItems_AllCases` and the helper `buildTestShipmentArtifact` from `internal/mcp/shipment_response_test.go`, **and remove the now-unused `github.com/softwaresalt/backlogit/internal/models` import from that file** (the helper is its only consumer in the file — leaving the import triggers a hard Go compile error and fails `golangci-lint`/`go vet`). Leave `TestListShipments_EmptyItems_NeverNull` in place unchanged — it now transitively exercises the core function through the handler and remains the end-to-end never-null guard.
7. **(verify green — run gates in order, do not skip)** Run `gofmt -l .` (expect no files listed), `go build ./...`, `go vet ./...`, `golangci-lint run` (constitution Principle I: must pass with zero warnings), and the full `go test ./...`; explicitly confirm `TestListShipments_EmptyItems_NeverNull` (internal/mcp) and the relocated `TestNormalizeShipmentItems_AllCases` (internal/core) pass.

**Files affected:** `internal/core/shipment.go`, `internal/core/shipment_covering.go`, `internal/core/shipment_lifecycle.go`, `internal/mcp/tools.go`, `internal/core/shipment_atomic_test.go`, `internal/core/shipment_state_integrity_test.go`, `internal/core/shipment_test.go`, `internal/mcp/shipment_response_test.go` (includes removing the now-unused `internal/models` import), and one new `internal/core/shipment_normalize_test.go`.

**Tests that verify the change:** relocated `TestNormalizeShipmentItems_AllCases` (core, incl. the new empty-`[]string`→non-nil case) and `TestListShipments_EmptyItems_NeverNull` (mcp), plus the full existing shipment suite (`shipment_test.go`, `shipment_atomic_test.go`, `shipment_state_integrity_test.go`) which continues to green under the rename.

**2-Hour Rule note (file count):** the raw file count (9) exceeds the "fewer than 3 files" heuristic, but the change is a single mechanical identifier rename (`shipmentItems` → `NormalizeShipmentItems`) plus one one-line branch hardening, one call-site rewrite, and one test relocation. It is **not divisible into atomic milestones**: a partial rename leaves the build red, so splitting would violate the Atomic Milestone constraint more severely than the file-count heuristic. Net new logic is one function's contract; cognitive scope and human-equivalent effort are well under 2 hours. This is intentionally one atomic task. Width is a single domain (Go source). Prefer `gopls`/IDE rename to make the call-site migration mechanical and safe.

## Dependency Graph

Single unit; no inter-unit dependencies. Internal step order is strict and sequential (test-red → implement → migrate → rewrite → delete → consolidate tests → verify), but this is intra-task ordering, not a cross-task dependency edge.

## Decisions and Rationale

1. **Exact signature + location: `func NormalizeShipmentItems(artifact *models.Artifact) []string` in `internal/core/shipment.go`.** Chosen over a raw-value signature (`NormalizeShipmentItems(raw any) []string`) because all 8 existing core call sites already pass the artifact, and the function must guard nil artifact / nil `CustomFields` internally. Keeping the artifact parameter makes the change a pure rename at every core call site (minimal churn, minimal risk) and preserves the established f015 read-edge shape.
2. **The never-null invariant lives in the core function's return contract**, not in a thin MCP coercion. `NormalizeShipmentItems` never returns nil (empty → `[]string{}`). Rationale: this makes the guarantee self-evident and independent of how items were decoded (DB `[]any` vs an in-memory `[]string`), so the MCP handler is a trivial adapter and the guarantee cannot be lost by a future caller that constructs items differently. The alternative (leave the reader able to return nil for empty `[]string` and re-add a nil→empty coercion in the MCP handler) was rejected because it re-scatters the invariant into the exact call site we are trying to simplify, defeating the single-source-of-truth goal.
3. **Reconcile to the superset of guarantees, documented.** The consolidated function keeps the pure-read semantics of `shipmentItems` and adopts the always-non-nil guarantee of the deleted MCP mutator. This is the one behavioral edge (empty `[]string` → non-nil) being converged deliberately, not silently: it is asserted by the new core test case and stated in the function doc comment. Verified safe for every existing core caller (all either `range` over the result, `append` to it, pass it to `containsString`, or pass it to `uniqueNonEmptyStrings` — none depends on nil-vs-empty distinction; `normalizeShipmentArtifact` merely benefits from a robustly non-nil canonical value).
4. **Test consolidation.** `TestNormalizeShipmentItems_AllCases` moves to `internal/core` because it must test the single source of truth directly; the MCP-side helper `buildTestShipmentArtifact` is removed with it (no other users). `TestListShipments_EmptyItems_NeverNull` stays in `internal/mcp` as the end-to-end integration assertion that the list handler still emits `[]` (never `null`) — it now transitively covers the core contract through the real handler path.
5. **`normalizeShipmentArtifact` stays as-is (call-site rename only).** It is a legitimate separate core concern (canonicalizing items into `CustomFields` on the CREATE/GET write path) and is not the stash's target. Folding it into the consolidation beyond the rename would be scope creep; it simply calls the renamed function.

## Risks and Caveats

- **Rename miss / stale reference:** an un-updated call site breaks the build. Mitigation: use tool-assisted rename and rely on `go build ./...` to surface any missed reference; the greps in this plan enumerate every known site.
- **Silent never-null regression:** if step 2's `[]string` branch hardening were skipped, an in-memory empty `[]string` could marshal to `null`. Mitigation: the new core test case asserts non-nil for empty `[]string`, and `TestListShipments_EmptyItems_NeverNull` guards the integration path.
- **CLI list path parity (out of scope, tracked as stash `7ECBAC7E`):** `internal/cli/shipment.go:151` passes DB-queried shipments to `NewShipmentViews` without normalizing items, unlike the MCP list handler. This is **pre-existing** behavior; the plan-review Parity persona confirmed a real divergence — CLI `shipment list --format json` emits `custom_fields.items: null` for an empty shipment while MCP `backlogit_list_shipments` emits `[]`. Closing it would add behavior to a CLI path that never normalized (scope creep for this consolidation, which the Scope Boundary persona flagged), so it is **deferred to a concrete tracked follow-up, stash `7ECBAC7E`** (mirror the MCP adapter in the CLI handler + add a cross-surface guard test; or the larger "normalize inside the shared `NewShipmentViews` shaper" end-state). This plan does not change CLI behavior.
- **Persisted empty-items representation shifts `null → []` (benign, no backfill):** hardening the `[]string` branch changes what `normalizeShipmentArtifact` (via `CreateShipment`/`GetShipment`, `shipment.go:363`) writes for a *newly created* empty shipment — from `{"items":null}` to `{"items":[]}`. This is a strict consistency improvement, but it means the on-disk/DB corpus becomes mixed (pre-existing empty shipments retain `null`; new ones store `[]`). No migration or backfill is required: the MCP list/get paths normalize both on read, so the mixed corpus is invisible to MCP consumers; only the un-normalized CLI list path (tracked as `7ECBAC7E`) can surface the mixture. Flagged for honesty because the earlier "no representation change" framing was too strong.
- **Acronym/casing:** the exported name is `NormalizeShipmentItems` (no initialisms involved; standard Go PascalCase), consistent with Go naming conventions.

## Constitution Check

Mapped against `.github/instructions/constitution.instructions.md`:

| Principle | Status | Notes |
|---|---|---|
| I — Quality Gates (zero-warning lint) | Compliant | Step 7 runs `gofmt -l .`, `go build`, `go vet`, `golangci-lint run` (zero warnings), and `go test ./...` in order. |
| II — Test-First (NON-NEGOTIABLE) | Compliant | Step 1 introduces a compile-red core test asserting the exported symbol before implementation (incl. the new empty-`[]string`→non-nil case). |
| III/IV — Workspace Isolation & CLI Containment | Compliant | All 9 affected paths + 1 new file are inside the repo tree; no out-of-tree writes. |
| V — Structured Observability / commit discipline | Compliant | Behavior-preserving refactor; the enclosing Ship commit should use a `refactor:` (or `chore:`) conventional prefix and reference stash `17D29DDC` for traceability. Stage does not commit code; Ship owns the branch/commit. |
| VI — Single Responsibility / dependency discipline | Compliant | Zero new dependencies; the change *increases* single-source-of-truth by deleting a duplicate. Scope creep resisted (CLI parity deferred to `7ECBAC7E`; `normalizeShipmentArtifact` left as call-site rename only). |
| VII — Destructive Command Approval (NON-NEGOTIABLE) | N/A | "Deletes" are in-source function/test removals via editing, not destructive terminal/VCS commands; fully git-revertible. |
| VIII — Safety Modes | Compliant | Enumerated Files affected + explicit out-of-scope note; `Requires plan hardening: no` justified by the signal table. |
| IX — Git-Friendly Persistence | Compliant | Markdown + YAML frontmatter, human-readable. |
| Task Granularity (2-Hour Rule) | Compliant with documented deviation | 9 files exceeds the "fewer than 3 files" heuristic; justified in the Unit 1 note — a mechanical identifier rename is indivisible without leaving the build red (which would violate the Atomic Milestone constraint more severely). Documented deviation with rejected-alternative, per the governance conflict-resolution clause. |
| X — Context Efficiency | N/A | Internal refactor plan. |

No unjustified violations.

## Plan Hardening Signals

- public API, schema, or contract change: **absent** — `NormalizeShipmentItems` is exported within `internal/core` (not a public/module-external API; `internal/` is import-restricted to this module). No wire schema or MCP tool contract changes; the MCP JSON response shape is preserved. Caveat (see Risks): the *persisted* empty-items representation for newly-created shipments shifts `null → []` — a benign consistency improvement, not a schema change, requiring no migration.
- security, auth, permission, or compliance-sensitive behavior: **absent** — no auth, secrets, or permission surface touched.
- migration, backfill, destructive data/config action, or irreversible step: **absent** — pure in-code refactor. The `null → []` persisted-shape shift for new empty shipments needs **no backfill** (read paths normalize both), and is fully revertible via git.
- external integration, operator checkpoint, or external dependency: **absent** — no external systems, no new dependencies.
- high runtime, rollout, or rollback risk: **absent** — behavior-preserving refactor guarded by existing + one new test; rollback is a git revert.

Requires plan hardening: no

## Runtime Verification and Closure

- **Runtime surface changed?** The MCP `backlogit_list_shipments` tool response is the primary runtime surface in the blast radius, and its shape is intentionally **unchanged** (`custom_fields.items` remains a non-null JSON array). The CLI `shipment list` surface is not modified by this plan (its pre-existing normalization gap is tracked separately as `7ECBAC7E`). No API, UI, or background-job behavior changes.
- **Runtime verification that proves absorption:** all Step-7 gates green (`gofmt -l .`, `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`), with `TestListShipments_EmptyItems_NeverNull` (mcp) and the relocated `TestNormalizeShipmentItems_AllCases` (core) both passing. Optional manual spot-check: `backlogit shipment list --status queued` returns `custom_fields.items: []` for an empty shipment (never `null`).
- **Operational closure artifact:** none required beyond the passing test suite. This is an internal, behavior-preserving consolidation with no monitoring, rollout window, or rollback trigger to own; the guard tests are the durable closure.

## Plan Review

Multi-persona plan-review gate (Stage, Step 4). Personas: Go Reviewer, Scope Boundary Auditor, Constitution Reviewer, Architecture Strategist, Agent-Native Parity Reviewer. Learnings surfaced pre-planning (f015 read-edge, confidence: high).

### Attempt 1 — FAIL
<!-- plan-review-attempt: 1 -->

Gate: **FAIL** (one P1 + one build-breaking P2). Merged findings (conservative severity on persona disagreement):

- **P1 (Parity) — CLI/MCP list parity gap, confirmed real.** `internal/cli/shipment.go:151` emits `custom_fields.items: null` for an empty shipment while MCP emits `[]`. The original "future task could optionally" deferral was under-specified. Scope Boundary persona countered that fixing it here is scope creep (the CLI path never normalized). **Resolution:** deferred to a concrete tracked follow-up (stash `7ECBAC7E`) rather than pulled into scope — honoring scope discipline while making the debt auditable.
- **P2 (Go) — build-breaker.** Removing `buildTestShipmentArtifact` leaves the `internal/models` import unused in `internal/mcp/shipment_response_test.go` (hard compile error; fails `go vet`/`golangci-lint`). **Resolved:** step 6 now removes the import; Files-affected updated.
- **P2 (Parity) — persisted representation change mischaracterized.** The `[]string`-branch hardening shifts newly-created empty-shipment persistence `null → []` via `normalizeShipmentArtifact`; the plan claimed "no representation change." **Resolved:** added a Risks caveat and corrected the Plan Hardening Signals (benign, no backfill — read paths normalize both).
- **P2 (Constitution) — incomplete quality gates + missing Constitution Check.** Verification omitted `golangci-lint run` / `gofmt -l .`. **Resolved:** step 7 now runs the full gate set in order; a `## Constitution Check` section was added.
- **P2 (Architecture) — partial abstraction** (logic centralized, enforcement scattered). Dedupes with the P1; **resolved** via the tracked follow-up `7ECBAC7E` (which also captures the "normalize inside the shared shaper" end-state).
- **P3s (advisory, addressed):** doc-comment now ties the non-nil guarantee to the JSON wire shape + references the guard test and f015 and contrasts pure-reader vs mutator; MCP adapter carries a clarifying comment; call-site enumeration marked illustrative with `go build` as source of truth; grep-verify added before helper deletion; commit-discipline (`refactor:` + stash ref) noted in the Constitution Check.

### Attempt 2 — PASS (post-revision)
<!-- plan-review-attempt: 2 -->

All P1/P2 findings resolved in place: the sole scope-expanding finding (CLI parity) is deferred to tracked stash `7ECBAC7E` (not vaguely), the build-breaker and gate/accuracy gaps are fixed, and the Constitution Check is present.

Gate: **PASS.** Residual items are the P3 advisories (folded into the plan) plus the deferred, tracked parity follow-up (`7ECBAC7E`). Plan hardening was not required (`Requires plan hardening: no`, all signals absent) and that assessment stands after revision. Runtime surface (`backlogit_list_shipments` shape) is preserved; verification and closure are defined. Cleared for harvest.
