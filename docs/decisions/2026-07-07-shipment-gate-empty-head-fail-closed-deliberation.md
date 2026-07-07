---
chunk_strategy: h1-h2-h3
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-07-shipment-gate-empty-head-fail-closed-deliberation.md
title: "Shipment-gate empty-head fail-closed hardening"
description: "Fail closed under enforcement when a shipment or member head is empty in a real git worktree, while preserving the legacy skip for no-repo / non-enforcement / non-autoharness environments"
topic: "shipment-gate empty-head fail-open holes (B85DAEE8 + 1AEA2B0E)"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-07-07-shipment-gate-empty-head-fail-closed-plan.md"
stash_ids:
  - "B85DAEE8"
  - "1AEA2B0E"
tags:
  - "security"
  - "gate"
  - "fail-closed"
  - "empty-head"
  - "shipment"
  - "core"
  - "git"
---

## Problem Frame

Two fail-**open** holes remain in the shipment-completion gate's empty-head
handling in `internal/core/shipment_gate.go`. Both were explicitly identified and
**deferred** by 084-F as separate, separately-reasoned seams (see
`docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md`, lines 56–59:
"Do not touch the empty-member-head bypass, the empty-shipment-head fail-open, or
the malformed-JSONL path — those are separate, separately-reasoned seams").

1. **B85DAEE8** — empty member `head_sha`. In `validateMemberGateEvidence`
   (`shipment_gate.go:351-352`) the guard is
   `if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {`.
   When a member's recorded evidence head is **empty**, the `h != ""`
   short-circuit skips the entire lineage/staleness comparison, so a terminal
   gated member with **no recorded head** passes the staleness check
   unconditionally (**fail-open**).

2. **1AEA2B0E** — empty shipment head under enforcement. In
   `gateShipmentCompletion` the single pre-resolved `shipmentHead` (line 198)
   can be a **legacy non-context empty** (`headSHABounded` returns `("", nil)`
   when `git rev-parse HEAD` fails for a non-timeout reason, e.g. not a repo /
   unborn branch / transient failure). Under enforcement (`ev.Enforced == true`,
   `headErr == nil`) that empty `shipmentHead` is passed into
   `validateMemberGateEvidence`, which then skips the whole member-lineage scan
   via `if shipmentHead != ""` (line 351), and the head-drift stable-head
   assertion is likewise skipped via `if ev.Enforced && shipmentHead != ""`
   (line 293). Net: **fail-open** — the gate cannot prove lineage but ships anyway.

**Security goal:** fail **closed** under enforcement (cannot verify lineage →
refuse) **WITHOUT** breaking legitimate empty-head cases. This mirrors the
pattern 084 used for the bounded-timeout case (distinguish an
enforced-real-failure from a legacy-legitimate-empty), which the mandate requires
be applied to these two remaining seams.

### Who cares and why

The shipment-completion gate is the reconciliation guarantee that release
finalization cannot become a gate bypass (082-F ST4.2). A fail-open on empty
heads means a member whose gated commit cannot be proven to be contained in the
shipment history — or a shipment whose own HEAD cannot be resolved in a repo —
can still ship. Under strict enforcement that is exactly the property the gate
exists to deny.

### Constraints

- **Must not regress the no-repo test suite.**
  `TestShipmentGate_AllMembersHaveEvidence_Ships` runs `injectBroker(ws,
  gate.EnabledTrue, ...)` in a `newGateTestWorkspace(t)` **temp dir that is not a
  git repo**, and asserts the ship **succeeds**. Many peers rely on the same
  skip. A naive "reject all empty heads under enforcement" breaks all of them.
- **`ev.Enforced` is NOT a repo-presence signal.** The test broker uses
  `Git: fakeGitAllOK{}`, which fakes base-ref resolution, so `Enforced == true`
  even in a non-repo temp dir (`broker.go:91`). A **separate repo-presence
  probe** is therefore required as the discriminator; the enforcement flag alone
  cannot distinguish "no repo" from "real repo, empty head".
- **No per-ship force escape hatch** exists at the shipment-gate level
  (`gateShipmentCompletion` takes no `opts.Force`; `ShipShipment` has no gate
  override). The only escapes are `enabled:false` (disables the gate) and
  `enabled:auto` in a non-autoharness env (not enforced → skip). Fail-closed
  therefore only bites `enabled:true` strict mode, which is opt-in "refuse if
  unverifiable."
- **Reuse the 084 exec/timeout/fail-closed discipline**: argv-array
  `exec.CommandContext` (no shell), `cmd.Dir = ws.RootPath`,
  `cmd.Env = gate.MinimalEnv()`, a self-derived bounded deadline
  (`boundedHelperTimeout`, capped at `ancestryCheckTimeout`), and the Windows
  gotcha of checking `runCtx.Err()` before reading an ExitError code.

### Success criteria

- Under `enabled:true` + a real git worktree: an empty shipment head **or** an
  empty member head → **fail closed** (typed `*GateBlockedError`, no shipment
  state change).
- Under no-repo / `enabled:auto` non-enforcing / `enabled:false`: the existing
  skip is preserved; every no-repo test stays green.
- Red tests prove each hole fails-open **today** under enforcement-with-repo;
  green after the fix; and legitimate-empty tests (no-repo, non-enforcement)
  prove no regression.

### Explicitly out of scope

- **F3844849** (malformed-JSONL-line handling divergence between
  `parseItemLogFile` and `events.ReadAllEvents`) — a separate, later phase,
  left untouched to honor the operator's ordering (B85DAEE8 before F3844849).
- The non-empty ancestor-aware lineage path (084-F, already shipped) — unchanged.
- The bounded-read timeout/cancel path (084-F `headResolveError`) — already
  fails closed; unchanged.

## Research Findings

### Prior art (learnings retrieval — confidence HIGH)

- `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — the
  direct predecessor. Names these two seams as out-of-scope-for-084 and supplies
  the reusable fail-closed git exit-code discipline and the head-drift bracketing.
- `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` —
  the `Enforced` contract (probe + base-ref resolution).
- `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — every bounded
  git helper must impose its own hard timeout cap (`ancestryCheckTimeout`).
- `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — bound the
  first lock-holding external call.

### Codebase evidence (verified against current post-084 `main`)

- `gateShipmentCompletion` (`shipment_gate.go:186-314`): single bounded
  pre-resolution `shipmentHead, headErr := ws.headSHABounded(ctx)` (198);
  `!ev.Enforced` fail-open early return (215-219); `headErr != nil` fail-closed
  (223-225); member scan (229); drift guard `if ev.Enforced && shipmentHead != ""`
  (293).
- `validateMemberGateEvidence` (`shipment_gate.go:327-384`): the
  `if shipmentHead != ""` block (351) and the `h != ""` member-head
  short-circuit (352) — **the B85DAEE8 seam**.
- `headSHABounded` (`shipment_gate.go:123-133`): distinguishes bounded-read
  failure `("", ctxErr)` from legacy non-context empty `("", nil)`.
- `headSHA` (`gate_transition.go:463-472`): best-effort `git rev-parse HEAD`,
  empty on any error.
- Evidence authoring (`gate_transition.go:407-409`): `delta["head_sha"]` is
  written **only when non-empty**, so an **empty member head_sha ⟺ the member's
  gate was authored without a resolvable HEAD** (no-repo / resolution failure at
  task-completion time), whether Passed(ran) or Forced.
- Member-evidence predicate `(Forced) OR (Passed && ran)`
  (`TestLatestGatePassEvidence_ComposedPredicate`, F4/083.002-T): forced evidence
  qualifies regardless of `ran`. In a **real repo** a forced gate records a
  non-empty head, so forced members never hit the empty-head branch there; a
  forced member with an empty head only arises from a no-repo authoring context.
- Broker `Enforced` derivation (`broker.go:60-113`): `Enforced = true` requires
  probe pass + base-ref resolution; the test harness fakes both, so `Enforced`
  does not track real repo presence.
- Test harness: `newGateTestWorkspace` = temp dir, **no repo**;
  `initGitRepoWithCommits` = real repo with A→B lineage + divergent sibling.
- **R7** (`shipment_gate_test.go:184-186`, inside
  `TestValidateMemberGateEvidence_StaleRefused`, real repo, non-empty
  `shipmentHead`): currently asserts an empty member head is **accepted**
  (`require.NoError`, "B85DAEE8 bypass preserved"). A repo-wide grep confirms this
  is the **only** test asserting empty-member-head-accepted in a real repo; its
  expectation must **flip** to fail-closed as part of this change.

## Options Evaluated

### Option A: Repo-presence discriminator (fail-closed under enforcement + real worktree)

Add a bounded, fail-closed repo-presence probe (e.g.
`git rev-parse --is-inside-work-tree`) reusing the same exec/timeout/MinimalEnv
discipline as `isAncestor`/`headSHABounded`. Under enforcement:

- **Empty shipment head (1AEA2B0E):** after the existing `headErr != nil`
  fail-closed check, if `shipmentHead == ""`, probe repo presence. `inRepo` →
  fail closed; probe **error** → fail closed; `!inRepo` → preserve legacy skip
  (`shipmentHead` stays `""`, member scan + drift guard remain inert).
- **Empty member head (B85DAEE8):** the `if shipmentHead != ""` block in
  `validateMemberGateEvidence` now implies "real repo + resolved head" (an empty
  shipment head in a real repo already failed closed above; a `!inRepo` no-repo
  keeps `shipmentHead == ""`, so that block is not entered). Change the empty
  member head inside that block from **skip → fail closed**.

* **Pros:** single clean discriminator; localizes one invariant — "under
  enforcement with a resolved shipment head, every gated member must carry a
  verifiable head"; reuses the 084 bounded-helper + fail-closed exit-code
  discipline; **no signature churn** to `validateMemberGateEvidence`
  (`shipmentHead != ""` is already the load-bearing guard); reuses existing typed
  `*GateBlockedError` + evidence patterns.
* **Cons:** one extra git subprocess on the (rare) empty-shipment-head path;
  intentionally flips R7; needs a new no-commit-repo fixture for the
  empty-shipment-head-in-repo test.
* **Effort:** low–medium.
* **Fit:** matches every constraint and success criterion; mirrors the mandated
  084 pattern exactly.

### Option B: Enforcement-flag-only fail-closed (no repo probe)

Treat any empty head under `ev.Enforced` as fail-closed unconditionally.

* **Pros:** simplest; no new subprocess.
* **Cons:** **breaks** `TestShipmentGate_AllMembersHaveEvidence_Ships` and every
  no-repo test running `EnabledTrue` (the harness fakes `Enforced=true`); this is
  precisely the naive "reject all empty heads" the mandate warns against.
* **Fit:** fails the no-regression constraint. **Rejected.**

### Option C: Convert the test harness to real git repos, then reject empty unconditionally

Make `newGateTestWorkspace` a real `git init` repo so heads always resolve.

* **Pros:** removes the "legitimate empty in tests" case; more realistic tests.
* **Cons:** large test-harness churn (many suites use `newGateTestWorkspace`);
  a git subprocess per test (slower); and it does **not** address the legitimate
  **production** non-autoharness/no-repo case — production non-git users would
  fail closed under strict mode with no discriminator. Insufficient alone and
  higher-churn.
* **Fit:** rejected as the mechanism. The real-repo fixture technique is instead
  used **narrowly** for the new fail-closed tests. **Rejected (as primary).**

### Option D: Special-case forced/`ran=false` evidence to preserve the empty skip

Keep the empty-member-head skip specifically for `EventGateForced` evidence.

* **Analysis:** unnecessary. In a real repo a forced gate records a non-empty
  head, so forced members never reach the empty-head branch there. A forced
  member with an empty head only arises from a no-repo authoring context; when
  that member is shipped in a **real repo** it is a genuine cross-environment
  inconsistency worth refusing under strict mode. **Folded into A: no forced
  exception.**

## Trade-off Comparison

| Criterion | A: Repo-presence probe | B: Flag-only | C: Real-repo harness | D: Forced exception |
|---|---|---|---|---|
| No-repo tests stay green | ✅ preserved via probe | ❌ breaks | ⚠️ via rewrite | ✅ (but incomplete) |
| Closes both holes under enforcement | ✅ | ✅ | ✅ | ⚠️ leaves forced hole |
| Production non-git safety | ✅ correct skip | ❌ over-refuses | ❌ over-refuses | ⚠️ |
| Churn / blast radius | low–medium | low | high | medium |
| Mirrors 084 pattern | ✅ | ❌ | ➖ | ➖ |
| Extra subprocess cost | 1 on rare path | none | per-test | none |

## Decision

**Adopt Option A — repo-presence discriminator.** Fail closed under enforcement
in a real git worktree when either the shipment head or a member head is empty;
preserve the legacy skip for no-repo / non-enforcement / non-autoharness
environments. No forced-evidence exception (Option D folded in as "not needed").

### The boundary (the crux)

| Condition | shipment head | member head | Outcome |
|---|---|---|---|
| `enabled:false` OR `auto` not enforced | any | any | skip (early `return nil`) — **unchanged** |
| enforced + **no** git worktree | `""` | any | **legacy skip preserved** (no-repo / non-autoharness) |
| enforced + **real** worktree | `""` (rev-parse failed, non-context) | — | **FAIL CLOSED** (cannot resolve own HEAD in a repo) |
| enforced + **real** worktree | non-empty | `""` (no recorded head) | **FAIL CLOSED** (cannot prove member lineage) |
| enforced + real worktree | non-empty | non-empty, ancestor/equal | accept (084 behavior — unchanged) |
| enforced + real worktree | non-empty | non-empty, divergent/malformed/unverifiable | fail closed (084 behavior — unchanged) |
| bounded-read timeout/cancel (either head) | — | — | fail closed (084 behavior — unchanged) |

**Discriminator** = a bounded repo-presence probe (recommended
`git rev-parse --is-inside-work-tree`) run under `boundedHelperTimeout`, with the
084 exec trust boundary (argv-array, `MinimalEnv`, `cmd.Dir = ws.RootPath`) and
fail-closed handling: `runCtx.Err()` checked **before** any exit-code read
(Windows gotcha); a `"not a git repository"` result (exit 128 / `false`) →
`!inRepo` (skip); any timeout / cancel / exec-error / non-deterministic result →
**fail closed**.

**Escape valve** for legitimate strict-mode edge cases: lower to `enabled:auto`
or `enabled:false`, or make a commit / re-run the gate so a head is recorded —
consistent with strict mode's opt-in "refuse if unverifiable" contract. This is
recorded so the plan-review Security lens can confirm the fail-closed direction
does not strand a legitimate operator with no path forward.

## Rejected Alternatives

- **B (flag-only):** breaks the no-repo suite; the naive over-refusal the mandate
  forbids.
- **C (real-repo harness as the mechanism):** high churn; does not protect
  production non-git users. Used only as a narrow fixture technique for new tests.
- **D (forced exception):** unnecessary; forced-in-real-repo records a head, and
  forced-in-no-repo shipped-in-repo is a genuine inconsistency that should refuse.

## Unresolved Questions (hand to impl-plan → plan-harden → plan-review Security lens)

1. Exact probe: `git rev-parse --is-inside-work-tree` vs `--git-dir`; bare-repo
   handling (`--is-inside-work-tree` prints `false`/exit 0 in a bare repo → treat
   as not-a-worktree skip). plan-harden owns git-exec edge cases.
2. Whether the empty-shipment-head-in-repo refusal reuses `headResolveError` or a
   dedicated message. Recommendation: a **distinct** message
   ("cannot resolve shipment head in repository") so the strict-mode empty-repo
   refusal is not confused with a bounded-read timeout.
3. Confirm (done — grep confirms only R7) that no other test asserts
   empty-member-head-accepted in a real repo, so the R7 flip is the only existing
   assertion that changes.

## Risks and Mitigations

- **R1 — Test regression / expectation flip.** R7 flips accept→refuse;
  `TestShipmentGate_AllMembersHaveEvidence_Ships` must stay green.
  *Mitigation:* plan splits R7 into (real-repo empty-member → refuse) + adds
  (no-repo / empty-`shipmentHead` empty-member → skip); adds a no-commit-repo
  empty-shipment-head → refuse test; keeps the no-repo full-ship test unchanged.
- **R2 — Repo-probe portability (Windows / unborn branch / bare repo).**
  *Mitigation:* reuse 084 bounded exec discipline; explicit exit-code map;
  plan-harden covers git-exec edge cases + enforcement-mode detection.
- **R3 — Extra subprocess cost.** *Mitigation:* probe only on the rare
  empty-shipment-head path; equality/ancestor fast-paths untouched; bounded by
  `ancestryCheckTimeout`.
- **R4 — Cross-environment false refusal** (member gated in no-repo, shipped in a
  real repo). *Mitigation:* a genuine inconsistency; strict mode correctly
  refuses; documented escape valve (lower enforcement / record a head).
- **R5 — Lock-holding DoS via a hung probe.** *Mitigation:* self-derived bounded
  deadline capped at `ancestryCheckTimeout`, same as `isAncestor`/`headSHABounded`.
