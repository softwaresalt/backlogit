---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash 885A7F65 (medium/bug): make the shipment-level member-evidence staleness check in internal/core/shipment_gate.go ancestor-aware. Today validateMemberGateEvidence rejects any member whose recorded gate-evidence head_sha != the current shipment head (strict equality), which produces FALSE staleness for post-merge multi-commit shipments (every member built at a feature-branch commit that is an ANCESTOR of the merge commit, not equal to it). This blocked closing shipment 083-S. Confirms the fix is a coherent single covering feature, names it, records the seam decision (direct-exec Workspace helper mirroring headSHA vs. extending the injected gate.GitRunner), and captures the security reasoning that ancestor-inclusion is a non-weakening semantic. Scope is strictly the non-empty head_sha equality->ancestor change; the empty-head bypass (B85DAEE8) and malformed-JSONL (F3844849) are explicitly OUT of scope.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-06-shipment-gate-ancestor-aware-staleness-deliberation.md
title: 'Shipment member-evidence staleness: strict head equality -> ancestor-aware (non-weakening gate-semantics fix)'
stash_id: 885A7F65
decision_status: decided
tags:
  - gate-broker
  - shipment-gate
  - staleness
  - git-merge-base
  - ancestor
  - security
  - core
---

## Question

The shipment-ship gate refuses to close a multi-commit shipment after its feature PR
is merged. Concretely: `validateMemberGateEvidence` (`internal/core/shipment_gate.go`
~152-156) rejects a member task whose recorded gate-evidence `head_sha` is not
**exactly equal** to the current shipment head (`git rev-parse HEAD`). After a merge
commit exists, each member's recorded head is its **feature-branch build commit**,
which is an **ancestor** of the merge commit — never equal to it — so all valid
evidence is falsely rejected as "stale." This blocked closing shipment `083-S` (all
nine members' recorded heads are proven git ancestors of merge commit `ac41bb1`).

Is making the staleness check **ancestor-aware** (accept a recorded head that is an
ancestor of, or equal to, the shipment head; reject only genuinely divergent heads)
the right fix, and does it **weaken** the gate's security guarantee?

This is a gate-semantics change to the shipped 082-F broker, so it warrants
deliberation, planning, hardening, and a passing plan-review before implementation
(the stash entry itself flags "Needs deliberation").

## Problem Frame

* **Problem being solved.** Strict head-SHA equality is fundamentally incompatible
  with post-merge multi-commit shipment closure. It is a latent 082-F defect, first
  tripped by `083-S` (the first shipment built after per-member `head_sha` population
  became active AND closed post-merge). Every future post-merge shipment closure is
  blocked until this is fixed.
* **Who cares.** The Ship agent and any operator closing a merged shipment. Right now
  there is no supported bypass (no gate config to relax, no `--force` on
  `shipment ship`), so the only "workarounds" are audit-log tampering or an unplanned
  compiled-in semantics change — both explicitly out of bounds.
* **Constraints.**
  * Security-sensitive: the staleness check is a guard that gated members' work is
    actually part of what is shipping. The fix must not weaken it.
  * Keep the argv-array + `MinimalEnv` exec discipline used elsewhere in the gate
    broker for any git invocation (082-S RCE/DoS lessons).
  * The empty-`head_sha` bypass is a **separate** tracked stash (`B85DAEE8`, low/bug),
    to be handled in a later phase. Do not fold it in.
  * Repo has no `chore` work-item type; the covering release unit is a `feature`.
* **Success criteria.**
  1. A member whose recorded head is an ancestor of the shipment head is **accepted**
     (false staleness eliminated) — reproduced by a red test, then green.
  2. A genuinely **divergent** (non-ancestor) recorded head is still **rejected**.
  3. Exact-equality still passes.
  4. A git-lineage **error** (unknown ref, shallow clone, non-repo) must **never**
     silently pass a security guard — it fails closed (blocks).
  5. No weakening of the gate: the reasoning below holds under adversarial review.
* **Out of scope (explicit).**
  * `B85DAEE8` — the empty-`head_sha` short-circuit (`h != ""`). Left verbatim.
  * `F3844849` — malformed-JSONL-line handling. Untouched.
  * Any change to the shipment-level aggregate full-diff `gate check` (check #2).
  * Any change to how `head_sha` is recorded (`gate_transition.go:408`).

## Research Findings

Grounded in the actual shipped code and prior learnings (not guesses):

* **The exact buggy branch** (`internal/core/shipment_gate.go:152-156`):

  ```go
  if shipmentHead != "" {
      if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {
          return shipmentMemberEvidenceError(id, "gate evidence is stale (recorded at a prior head)")
      }
  }
  ```

  The `h != ""` sub-clause is the empty-head bypass (**B85DAEE8**). The `h != shipmentHead`
  sub-clause is the strict-equality defect this deliberation targets.
* **`head_sha` provenance.** Recorded at `gate_transition.go:408`
  (`delta["head_sha"] = outcome.HeadSHA`), where `outcome.HeadSHA` is `git rev-parse HEAD`
  at the moment the member's pre-completion gate passed — i.e., the feature-branch build
  commit. Pre-existing since before the 083 base commit; 082 members recorded an *empty*
  head, which is why they slipped past the check and 083-S is the first to trip it.
* **`shipmentHead` provenance.** `ws.headSHA(ctx)` (`gate_transition.go:462-472`) =
  `git rev-parse HEAD`, argv-array + `cmd.Dir = ws.RootPath` + `cmd.Env = gate.MinimalEnv()`,
  best-effort (empty string on any error).
* **Existing git-exec discipline to mirror.** `headSHA` (direct `exec.CommandContext`,
  `MinimalEnv`) and `commits.go:132` (`git -C ... log`) both reach for `exec` directly in
  `internal/core`. The **gate package** (`internal/core/gate/*`) instead uses an injected
  `GitRunner`/runner seam (`ExecGitRunner.Verify` = `git rev-parse --verify --quiet <ref>^{commit}`,
  distinguishing exit-1 "does not resolve" from real errors via `*exec.ExitError`). That
  `Verify` exit-code idiom is the exact pattern to copy for `--is-ancestor`.
* **Prior learnings consulted (`docs/compound/`):**
  * `2026-07-06-autoharness-gate-broker-integration-contract.md` — the shipment member
    scan requires terminal status + latest passing/forced evidence, "with an optional
    `head_sha` staleness check." Also states the design principle "inject the runner; do
    not reach for `exec` directly inside core logic" — but that principle is about the
    **autoharness gate-invocation seam** that crosses the one-way `core -> gate` boundary,
    not about core's own git helpers (`headSHA`, `commits.go`), which already exec directly.
  * `2026-07-06-external-process-timeout-before-probe.md` — every external-process call on
    a lock-holding critical path must be timeout-bounded (context deadline). The new git
    call runs under the shipment-ship path and must inherit a bounded `ctx`.
  * `2026-07-06-exec-binary-config-must-be-bare-path-validated.md` — "data must not choose
    the code": pair argv-array-only exec + `MinimalEnv` with input validation. `head_sha`
    is read from on-disk evidence JSONL (the closure doc itself notes hand-editing those
    values is a tampering vector), so a recorded head must be treated as **untrusted input**
    to the git call.
* **Test harness reality.** `newGateTestWorkspace` uses `t.TempDir()` with **no git repo**,
  so `headSHA` returns `""` and the existing `TestValidateMemberGateEvidence_StaleRefused`
  uses **fake** SHAs ("oldsha0000"/"newsha1111") with an explicitly-passed `shipmentHead`.
  An ancestor-aware check that consults real git therefore requires a **real git repo
  fixture** (git init + real commits) for the divergent/ancestor/error paths, and an
  **equality fast-path** so the exact-equal case needs no subprocess (and stays correct).

## Options Evaluated

### Option A: Direct-exec `*Workspace` git-lineage helper (mirror `headSHA`)

Add a small `*Workspace` helper in `shipment_gate.go` (co-located with its sole caller)
that runs `git merge-base --is-ancestor <memberHead> <shipmentHead>` via
`exec.CommandContext` with `cmd.Dir = ws.RootPath` and `cmd.Env = gate.MinimalEnv()`,
interpreting exit codes with `*exec.ExitError` (0 = ancestor/equal → included; 1 = not
ancestor → divergent; anything else → error → fail closed). Wire it into the staleness
branch behind an exact-equality fast-path and a SHA-shape guard. Tests use a real git
repo fixture.

* **Pros:** Matches the operator's explicit "argv-array + MinimalEnv discipline"
  instruction and the established local pattern (`headSHA`, `commits.go`); minimal blast
  radius (one file for logic, one seam); no interface change rippling across the gate
  package; real-repo tests exercise the *actual* `--is-ancestor` exit semantics (higher
  fidelity than a fake).
* **Cons:** Not injectable → tests need a real `git` binary and a repo fixture (git is
  already a hard runtime + CI dependency, so acceptable); a second core git helper alongside
  `headSHA`.
* **Effort:** Low.
* **Fit:** Strong — honors the narrow-scope, in-order instruction.

### Option B: Extend the injected `gate.GitRunner` seam with `IsAncestor`

Add `IsAncestor(ctx, ancestor, descendant string) (bool, error)` to the `gate.GitRunner`
interface, implement it on `ExecGitRunner`, update `fakeGitAllOK` and any other
implementers, and have `validateMemberGateEvidence` call `ws.GateBroker.Git.IsAncestor`.

* **Pros:** Fake-injectable → unit tests mock ancestry without a real repo; aligns with the
  "inject the runner" line in the integration-contract learning.
* **Cons:** Broader blast radius — interface change touches every implementer and the
  broker wiring; couples a *core* security check to the *gate*-package runner whose current
  charter is base-ref resolution, blurring the one-way boundary; the broker's `Git` may be
  `nil` in some paths (fail-open/unwired), needing extra guards; more surface for the same
  ~30-line behavior; a mocked ancestor result does **not** verify real git exit-code
  handling, which is the security-load-bearing part.
* **Effort:** Medium.
* **Fit:** Weaker against the "narrow scope, in order" instruction; over-generalizes.

### Option C: Hybrid — direct-exec helper behind a package-level func seam

Option A, but assign the exec to an unexported package `var` (e.g. `isAncestorFn`) so tests
can stub it without a real repo.

* **Pros:** Direct-exec discipline + stubbable in unit tests.
* **Cons:** A test-only indirection seam on a security path invites stubbing away the exact
  exit-code logic that must be tested for real; a real-repo fixture (Option A) gives the same
  coverage without a production seam that exists only for tests.
* **Effort:** Low-medium.
* **Fit:** Middling — adds a seam without buying coverage a real repo doesn't already give.

## Trade-off Comparison

| Criterion | Option A (direct-exec helper) | Option B (inject GitRunner) | Option C (func-var seam) |
|---|---|---|---|
| Blast radius | 1 file logic + 1 helper | interface + all implementers + wiring | 1 file + a test seam |
| Matches operator instruction | Yes (argv+MinimalEnv, in-order) | Partial | Yes |
| Exercises real `--is-ancestor` exit semantics | Yes (real-repo tests) | No (mocked) | Only if fixture, else no |
| One-way core→gate boundary | Preserved | Blurred | Preserved |
| Nil-broker / fail-open edge handling | N/A (core helper) | Extra guards needed | N/A |
| Security-path test fidelity | High | Lower (mock) | Lower if stubbed |
| Effort | Low | Medium | Low-Medium |

## Decision

**Adopt Option A.** Add a direct-exec `*Workspace` git-lineage helper that runs
`git merge-base --is-ancestor` (argv-array, `cmd.Dir`, `gate.MinimalEnv()`, bounded `ctx`),
mirroring `headSHA` and the `ExecGitRunner.Verify` exit-code idiom. Wire it into
`validateMemberGateEvidence` behind:

1. the preserved empty-head bypass (`h != ""`) — **unchanged** (B85DAEE8 territory);
2. an **exact-equality fast-path** (`h == shipmentHead` → accept, no subprocess) — preserves
   prior behavior for single-commit shipments and keeps the common case subprocess-free;
3. a **SHA-shape guard** (`^[0-9a-fA-F]{7,64}$`) — because the recorded head is untrusted
   on-disk input; a non-conforming value is refused (block) rather than handed to git
   (argument-injection defense; "data must not choose the args");
4. the ancestor check for a well-formed, non-equal head:
   * exit 0 → ancestor/equal → **accepted** (the gated code is contained in the shipment head);
   * exit 1 → not an ancestor → **rejected** as divergent (genuine staleness preserved);
   * any other exit / exec failure → **error → fail closed** (block; never silent pass).

Covering release unit: a single **feature** (repo has no `chore` type), one implementation
**task** (test-first), decomposed into two ordered **subtasks** (git helper + guard; then
wiring + reworked staleness tests). This is small but security-sensitive, so it still travels
through plan-harden and a passing plan-review before harvest.

### Security reasoning — why ancestor-aware does NOT weaken the guard (the crux)

The per-member evidence check answers exactly one question: *"did THIS member's gates pass
on code that is actually part of what we are shipping?"*

* `head_sha` is the commit at which the member's pre-completion gate passed.
* **Strict equality** is not a security property — it is an over-tight proxy. It admits only
  a head identical to the current shipment head, which by construction can never hold once a
  merge commit exists, so it converts *all* valid post-merge evidence into false rejections.
* **Ancestor-inclusion** (`merge-base --is-ancestor member_head shipment_head` = true) means
  the member's gated commit is **reachable from** the shipment head — the exact code state the
  member gated IS contained in the shipment's history. This is precisely the semantic the
  PR-review model uses (a review approves a commit that is an ancestor of the merge). It is
  the *correct* expression of "the gated work is included."
* It still **rejects genuinely divergent heads**: a member gated on a commit that is NOT in
  the shipment's history (abandoned branch, different lineage) is not an ancestor → rejected.
  Divergent means the gated code is not what is shipping → correct refusal.
* **Residual concern** ("later commits after the member's gate could modify the same files")
  is real but is **not** what the per-member check is for. It is covered by the **shipment-level
  aggregate `gate check` over the FULL shipment diff** (check #2 in `gateShipmentCompletion`),
  which already runs and is **unchanged** by this fix. Division of labor: check #1 proves
  "member passed + member's work is included"; check #2 proves "the full shipment diff passes
  now." Ancestor-aware corrects check #1 without touching check #2, so the aggregate guard over
  later modifications is fully preserved.
* **New fail-closed behaviors tighten, not loosen, the guard:** unknown ref / shallow clone /
  non-repo → error → block; malformed (non-SHA) recorded head → block. A security guard must
  fail closed.

Conclusion: ancestor-aware **tightens correctness** (removes false rejections) while **preserving
the security property** (divergent evidence rejected; full-diff aggregate enforced; errors fail
closed). If a reviewer finds a real weakening, treat it as gate-blocking.

## Rejected Alternatives

* **Option B (inject `gate.GitRunner`)** — rejected for over-generalizing a ~30-line security
  fix: broader interface blast radius, blurs the one-way core→gate boundary, needs nil-broker
  guards, and — most importantly — a mocked ancestry result does not verify the real
  `git merge-base --is-ancestor` exit-code handling, which is the load-bearing part of the guard.
* **Option C (func-var seam)** — rejected because a real-repo fixture gives the same (better)
  coverage without shipping a test-only indirection on a security path.
* **Do nothing / operator-force** — rejected: no supported bypass exists, and forcing would
  require audit-log tampering or an unreviewed compiled-in change.

## Unresolved Questions

* None blocking. The empty-head bypass (B85DAEE8) is intentionally deferred; the design keeps
  it verbatim, and ancestor-correctness does **not** require touching it (an evidence event with
  no recorded head cannot be lineage-checked at all — it is an orthogonal gap).

## Risks and Mitigations

* **Git edge cases** (shallow clone, unknown ref, detached HEAD, non-repo) — enumerated for
  plan-harden. Mitigation: exit-code trichotomy with error→fail-closed; detached HEAD is fine
  (`rev-parse HEAD` still yields a SHA); non-repo yields empty `shipmentHead` → the whole check
  is skipped exactly as today.
* **Argument injection via a tampered `head_sha`** — mitigated by the SHA-shape guard + argv-array
  exec (no shell) + `MinimalEnv`; git's own option-parsing failure is a fail-closed backstop.
* **Test-harness gap** (no git repo in `newGateTestWorkspace`) — mitigated by adding a real git
  repo fixture for the ancestry tests and reworking `TestValidateMemberGateEvidence_StaleRefused`
  from fake SHAs to real ancestor/divergent commits.
* **Scope creep into B85DAEE8** — mitigated by preserving the `h != ""` clause verbatim and
  asserting the empty-head path is unchanged.

## Promotion

Promoted to **plan** — handed to `impl-plan`. Source document for
`docs/exec-plans/2026-07-06-shipment-gate-ancestor-aware-staleness-plan.md`.
