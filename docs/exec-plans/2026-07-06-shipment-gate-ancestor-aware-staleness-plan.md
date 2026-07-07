---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash 885A7F65: make the shipment member-evidence staleness check in internal/core/shipment_gate.go ancestor-aware. Replaces strict head_sha equality with git merge-base --is-ancestor lineage inclusion so post-merge multi-commit shipment closure stops falsely rejecting valid evidence, while still rejecting genuinely divergent heads and failing closed on git errors. One feature, one test-first task, two ordered subtasks (git-lineage helper + SHA-shape guard; then wiring + reworked staleness tests). Empty-head bypass (B85DAEE8) and malformed-JSONL (F3844849) are out of scope.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-06-shipment-gate-ancestor-aware-staleness-plan.md
title: 'Ancestor-aware shipment member-evidence staleness check'
---

# Ancestor-aware shipment member-evidence staleness check

**Source deliberation:** `docs/decisions/2026-07-06-shipment-gate-ancestor-aware-staleness-deliberation.md`
**Stash:** `885A7F65` (medium/bug)
**Prior art:** `docs/closure/2026-07-06-083-S-post-merge-closure-BLOCKED.md`,
`docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md`,
`docs/compound/2026-07-06-external-process-timeout-before-probe.md`,
`docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`

## Problem Frame

`internal/core/shipment_gate.go` → `validateMemberGateEvidence` (lines 152-156) gates the
`shipment ship` path. For each terminal gated member it compares the member's latest
gate-evidence `head_sha` against the current shipment head (`ws.headSHA(ctx)` = `git rev-parse HEAD`):

```go
if shipmentHead != "" {
    if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {
        return shipmentMemberEvidenceError(id, "gate evidence is stale (recorded at a prior head)")
    }
}
```

`head_sha` is recorded at `internal/core/gate_transition.go:408`
(`delta["head_sha"] = outcome.HeadSHA`) as `git rev-parse HEAD` at the moment the member's
pre-completion gate passed — the **feature-branch build commit**. Once a shipment's feature PR
merges, the shipment head is a **merge commit** whose SHA differs from every member's build
commit by construction, while each member's build commit is a git **ancestor** of that merge
commit. Strict equality therefore rejects all valid evidence as "stale" (false staleness). This
blocked `083-S` (nine members, all recorded heads proven ancestors of merge `ac41bb1`).

**Fix (technical):** replace the strict `h != shipmentHead` inequality with an ancestor-aware
check: accept when the member head is an ancestor of (or equal to) the shipment head — i.e.,
`git merge-base --is-ancestor <memberHead> <shipmentHead>` returns exit 0 — and reject only
genuinely divergent (non-ancestor) heads. Errors fail closed. To keep the ancestor-aware
semantics sound (attempt-1 Security P1), the shipment head is resolved **once** in
`gateShipmentCompletion` and the aggregate full-diff check (#2) and the member-lineage check (#1)
are bound to that same observed head via a HEAD-drift guard.

## Requirements Trace

| # | Requirement (from deliberation success criteria) | Implementation action |
|---|---|---|
| R1 | Ancestor/equal member head is accepted (false staleness gone) | New `--is-ancestor` branch returns nil for exit 0; RED→GREEN test with a real ancestor commit |
| R2 | Genuinely divergent (non-ancestor) head still rejected | Exit 1 → `shipmentMemberEvidenceError(... "divergent")`; divergent-branch test |
| R3 | Exact equality still passes | Equality fast-path `h == shipmentHead` → accept (no subprocess); equality test |
| R4 | Git-lineage error never silently passes | Exit ≠0/≠1 or exec failure → block, wrapped with `%w` + `slog.WarnContext`; non-existent-ref test asserts a block |
| R5 | argv-array + MinimalEnv + **mandatory bounded ctx** exec discipline | Helper mirrors `headSHA` for argv/env and `gate/runner.go` for timeout ordering; derives its OWN `context.WithTimeout` (the ship path is unbounded) |
| R6 | Untrusted on-disk `head_sha` cannot inject git args | SHA-shape guard `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` before exec; malformed test asserts a block |
| R7 | Empty-member-head bypass (B85DAEE8) unchanged; check #2 (aggregate diff) unchanged | Preserve `h != ""` clause verbatim; do not touch `gateShipmentCompletion` check #2 semantics |
| R8 | Timeout-killed git is never misread as "divergent" (exit 1) | Check `ctx.Err()`/`DeadlineExceeded` BEFORE the `*exec.ExitError` trichotomy (mirrors `gate/runner.go:83-94`, not `baseref.go`) |
| R9 | Checks #1 and #2 observe the SAME shipment head (attempt-1/2 Security P1) | Resolve `shipmentHead` once (bounded) in `gateShipmentCompletion` before `Evaluate`; thread it into `validateMemberGateEvidence`; re-resolve as the LAST read before the success path and fail closed on drift — brackets the whole evaluation |

## Implementation Units

### Unit 1 — Git ancestor-lineage helper + SHA-shape guard (test-first)

**Execution posture:** test-first.

* **What changes.** Add two unexported helpers to `internal/core/shipment_gate.go`:
  * `func (ws *Workspace) isAncestor(ctx context.Context, ancestor, descendant string) (bool, error)` —
    runs `git merge-base --is-ancestor ancestor descendant` via
    `exec.CommandContext(runCtx, "git", "merge-base", "--is-ancestor", ancestor, descendant)` with
    `cmd.Dir = ws.RootPath`, `cmd.Env = gate.MinimalEnv()`, and `cmd.Stderr = &stderr` (a
    `bytes.Buffer`, so shallow-boundary / "not a valid object" diagnostics reach the error).
    * **Mandatory self-bounded context (R5, attempt-1 Go P1).** The ship path is unbounded
      (`ShipShipment` → `gateShipmentCompletion` → here pass the caller `ctx` straight through, and
      `headSHA` is likewise unbounded), yet this spawns one subprocess per non-equal member (up to
      N) while the workspace lock is held. So `isAncestor` derives its OWN deadline —
      `runCtx, cancel := context.WithTimeout(ctx, d); defer cancel()` — where `d` is
      `ws.GateBroker.TimeoutSeconds` seconds when `> 0`, else a small package default
      (`ancestryCheckTimeout`, e.g. 5s). This mirrors `gate/broker.go:74-79`. Do NOT rely on the
      caller imposing a deadline.
    * **Result interpretation — timeout BEFORE ExitError (R8, attempt-1 Go P2).** Mirror
      `gate/runner.go:83-94`, NOT `baseref.go` (which is safe only because it runs under the
      broker's already-bounded `runCtx`):
      1. `runErr := cmd.Run()`; `if runErr == nil` → exit 0 → `(true, nil)` — ancestor or equal.
      2. **`if stderrors.Is(runCtx.Err(), context.DeadlineExceeded)` → `(false, fmt.Errorf("...: %w", ...))`**
         BEFORE reading any exit code (a killed git reports a platform-dependent code, e.g. -1 on
         Windows, that must never be read as exit-1 "divergent").
      3. `var ee *exec.ExitError; if stderrors.As(runErr, &ee)` → `ExitCode() == 1` → `(false, nil)`
         (definitively not an ancestor); any other code (e.g. 128/129) → `(false, error)` with the
         captured stderr, wrapped `%w`.
      4. non-`ExitError` (git binary missing, ctx cancelled) → `(false, fmt.Errorf("...: %w", runErr))`.
  * `func isGitObjectName(s string) bool` — reports whether `s` matches
    `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` (a compiled package-level `regexp`, matching exactly the
    producible SHA-1/SHA-256 `git rev-parse` output; attempt-1 Go P3). Refuses handing a
    malformed/tampered recorded head to git (argument-injection defense: a leading-`-` value can
    never reach git).
* **Files affected (1):** `internal/core/shipment_gate.go` (add helpers + `ancestryCheckTimeout`
  const; add imports `bytes`, `os/exec`, `regexp`, `time`; `stderrors` alias, `context`, `fmt`,
  and `gate` are already imported).
* **Tests (new file `internal/core/shipment_gate_ancestry_test.go`):** a real-git-repo fixture
  helper `initGitRepoWithCommits(t, dir)` that runs `git -c init.defaultBranch=main init` **in the
  same dir as `ws.RootPath`** (so `isAncestor`'s `cmd.Dir` resolves), sets BOTH committer and author
  identity (`GIT_AUTHOR_NAME/EMAIL` + `GIT_COMMITTER_NAME/EMAIL`, or `git config user.*`, to avoid
  "empty ident" on a clean CI), and creates a small ancestor/divergent history returning real SHAs.
  Skip with `t.Skip` if `git` is not on PATH. Scenarios (≤4 for this subtask):
  1. `isAncestor(base, head)` where `base` is an ancestor of `head` → `(true, nil)`.
  2. `isAncestor(divergent, head)` where `divergent` is a REAL sibling-branch commit → `(false, nil)`
     (exit 1). (A valid-shape but absent SHA yields exit 128 → error, which is a different branch.)
  3. `isAncestor("<valid-shape but absent SHA>", head)` → non-nil error (fail-closed).
  4. `isGitObjectName` table: accept a 40-hex and a 64-hex; reject "", "oldsha0000", "not-a-sha",
     a leading-dash `-foo`, a 7-hex abbreviation, and a 65-char string.
* **Milestone:** `go test ./internal/core/ -run 'IsAncestor|GitObjectName'` green; `go vet` clean.
* **Depends on:** none.

### Unit 2 — Wire ancestor-aware logic into `validateMemberGateEvidence` + rework staleness tests

**Execution posture:** test-first (rework the existing red-able test, then green the wiring).

* **What changes.** Replace the strict-equality branch (lines 152-156) with:

  ```go
  if shipmentHead != "" {
      if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {
          // h != "" preserves the empty-member-head bypass (B85DAEE8, out of scope).
          // h == shipmentHead is the equality fast-path: an equal head never enters this block.
          if !isGitObjectName(h) {
              slog.WarnContext(ctx, "member evidence head_sha is malformed",
                  "member", id, "member_head", h, "shipment_head", shipmentHead)
              return shipmentMemberEvidenceError(id,
                  "gate evidence head_sha is malformed (not a git object name)")
          }
          included, aerr := ws.isAncestor(ctx, h, shipmentHead)
          if aerr != nil {
              // A security guard must never silently pass on an unverifiable lineage.
              slog.WarnContext(ctx, "member evidence lineage check failed",
                  "member", id, "member_head", h, "shipment_head", shipmentHead, "error", aerr)
              return shipmentMemberEvidenceError(id,
                  fmt.Sprintf("cannot verify gate evidence lineage: %v", aerr))
          }
          if !included {
              return shipmentMemberEvidenceError(id,
                  "gate evidence is stale (recorded at a divergent head)")
          }
      }
  }
  ```

  Note: the outer `shipmentMemberEvidenceError` already wraps `*GateBlockedError` with `%w`
  (unchanged); the inner git error is preserved for operators via the `slog.WarnContext` line
  (Constitution P3/V) plus the helper's own `%w` chaining (Constitution/Go P3). Also update the
  `validateMemberGateEvidence` doc comment (lines 123-127) to describe ancestor-inclusion instead
  of exact-match. Add `log/slog` to imports (already imported in `shipment_gate.go`).
* **Files affected (2):** `internal/core/shipment_gate.go` (the branch + comment),
  `internal/core/shipment_gate_test.go` (rework `TestValidateMemberGateEvidence_StaleRefused`).
* **Tests:** rework `TestValidateMemberGateEvidence_StaleRefused` to use the real-repo fixture and
  assert the SPECIFIC branch messages (not just `Contains("stale")`, since the malformed and
  error branches do not contain "stale" — attempt-1 Go P2):
  1. **RED→GREEN (R1):** member `head_sha` = an ancestor commit `A`, `shipmentHead` = descendant
     `B` → today rejected ("stale"), after fix **accepted** (nil).
  2. **Divergent (R2):** member `head_sha` = REAL sibling-branch commit `D` (not ancestor of `B`) →
     rejected; error contains "divergent"; `errors.As` → `*GateBlockedError`.
  3. **Equality (R3):** member `head_sha` == `shipmentHead` == `B` → accepted (fast-path, no repo
     access required).
  4. **Empty-head unchanged (R7):** member `head_sha` == "" → accepted (bypass preserved).
* **Milestone:** `go test ./internal/core/ -run 'ShipmentGate|MemberGateEvidence'` green;
  full `go test ./... && go vet ./... && gofmt -l .` clean.
* **Depends on:** Unit 1 (uses `isAncestor` + `isGitObjectName`).

### Unit 3 — Bracket the whole gate evaluation with a stable-head assertion (test-first)

**Execution posture:** test-first (pure comparison helper first, then wire into `gateShipmentCompletion`).

* **Why (attempt-1 Security P1 + attempt-2 Security P1 refinement).** Ancestor-aware only preserves
  the non-weakening argument if the member-lineage check (#1) and the aggregate full-diff check (#2)
  observe the SAME shipment head. Today `gateShipmentCompletion` runs `Evaluate` at
  `shipment_gate.go:42` (autoharness independently resolves the pinned symbolic `HEAD` for the
  aggregate diff) and SEPARATELY calls `validateMemberGateEvidence(..., ws.headSHA(ctx))` at line 60.
  Attempt-2 review showed that a drift check placed only *between* Evaluate and the member scan
  leaves a **residual TOCTOU window**: HEAD could still advance after that check but during/around
  the member scan, so the shipment is completed against a head neither check fully validated, and
  ancestor-aware would admit a member whose old head is now an ancestor of the advanced HEAD.
  Because `ev.HeadRef` is **intentionally pinned to `"HEAD"`** (`gate/types.go:31-34`, a
  non-weakening guard against an empty-diff ref) we cannot pass a resolved SHA into `Evaluate`;
  instead we **bracket the entire evaluation** with a single stable-head assertion whose drift check
  is the LAST read before the success path returns.
* **What changes** (in `internal/core/shipment_gate.go`, `gateShipmentCompletion`):
  1. Add `func (ws *Workspace) headSHABounded(ctx context.Context) (string, error)` — wraps
     `ws.headSHA` under a derived `context.WithTimeout` (same source/fallback as `isAncestor`:
     `ws.GateBroker.TimeoutSeconds`, else `ancestryCheckTimeout` ~5s), so a hung `git rev-parse`
     cannot stall completion on the unbounded ship path (attempt-2 Go P2). **It distinguishes a
     bounded-context failure from a legacy resolution failure** (attempt-3 Security P1): after
     calling `h := ws.headSHA(bctx)`, if `h == ""` and `bctx.Err() != nil` (deadline/cancel), it
     returns `("", bctx.Err())` — a bounded-read failure that MUST fail closed; otherwise it returns
     `(h, nil)` where `h` may be a real SHA or a **legacy** `""` from a non-context resolution error
     (e.g. non-repo test harness). This keeps `headSHA`'s "" behavior for the pre-existing
     (FLAGGED) empty-head case while ensuring the NEW timeout path my helper introduces never
     collapses into a silent skip.

     ```go
     func (ws *Workspace) headSHABounded(ctx context.Context) (string, error) {
         d := ancestryCheckTimeout
         if ws.GateBroker != nil && ws.GateBroker.TimeoutSeconds > 0 {
             d = time.Duration(ws.GateBroker.TimeoutSeconds) * time.Second
         }
         bctx, cancel := context.WithTimeout(ctx, d)
         defer cancel()
         h := ws.headSHA(bctx)
         if h == "" && bctx.Err() != nil {
             return "", bctx.Err() // timeout/cancel: bounded-read failure → caller fails closed
         }
         return h, nil // real SHA, or legacy "" (non-context resolution failure → legacy skip)
     }
     ```
  2. Resolve `shipmentHead, headErr := ws.headSHABounded(ctx)` **once**, *before* `Evaluate`
     (line 42).
  3. Run `Evaluate` (unchanged; aggregate check #2 executes against pinned `HEAD`).
  4. **After `!ev.Enforced` returns (line 53-57), fail closed on a bounded-read failure**: if
     `headErr != nil`, return a typed `*GateBlockedError` (`headResolveError(shipmentID, headErr)`)
     — a timeout/cancel resolving HEAD must block, never silently skip staleness (attempt-3
     Security P1). A legacy `""` (`headErr == nil`) preserves the pre-existing skip (FLAGGED).
  5. Pass the single pre-resolved `shipmentHead` to `validateMemberGateEvidence` at line 60 (no
     second in-scan resolution).
  6. **After** the check #2 decision confirms `DecisionProceed` (after the block/error branches at
     lines 73-108, immediately before the passing-evidence append at line 110 — the LAST read
     before `return nil`), when `ev.Enforced && shipmentHead != ""`, re-resolve
     `postHead, postErr := ws.headSHABounded(ctx)`; if `postErr != nil` return
     `headResolveError(...)` (fail closed on a post-read timeout/cancel too); else return
     `headDriftError(shipmentID, shipmentHead, postHead)` if `postHead != shipmentHead`. Placing
     this here brackets Evaluate (#2) AND the member scan (#1): any HEAD advance across the whole
     evaluation window fails closed, closing the attempt-2 residual window.
* **Scope note — legacy empty-`shipmentHead` skip is deliberately NOT changed here.** When
  `headSHABounded` returns a **legacy** `""` (`headErr == nil` — rev-parse failed for a non-context
  reason, e.g. the no-repo test harness) the member staleness block is skipped and the drift guard
  does not engage — **identical to today's behavior** (the current code already reads `headSHA` once
  and skips on `""`; this revision reuses that single value). The NEW timeout/cancel path
  (`headErr != nil`) fails closed and does NOT widen the legacy fail-open. Making "enforced + legacy
  empty → fail closed" would break many pre-existing no-repo shipment tests
  (`TestShipmentGate_AllMembersHaveEvidence_Ships` and peers) that rely on the skip, is orthogonal
  to this equality→ancestor change, and ripples widely — see **Discovered adjacent issue (FLAGGED)**
  below. This is the operator-honored scope line (fix non-empty `head_sha` staleness only; FLAG
  adjacent fail-opens).
* **Files affected (2):** `internal/core/shipment_gate.go` (`gateShipmentCompletion` +
  `headDriftError` + `headResolveError` + `headSHABounded` helpers), `internal/core/shipment_gate_test.go`
  (drift + no-drift + bounded-read-failure unit tests on the pure helpers; a no-repo-safe wiring
  test).
* **Tests:** table-test the pure helpers (no git needed for the comparison helpers):
  1. `headDriftError`: `pre == post` (incl. both `""`) → nil; `pre != post` both non-empty →
     `*GateBlockedError` naming both SHAs.
  2. `headResolveError`: non-nil ctx error → `*GateBlockedError` (fail closed).
  3. `headSHABounded` distinction (small real-repo or injected-fake): a `context.DeadlineExceeded`
     derived ctx → `("", ctxErr)`; a non-repo dir → `("", nil)` (legacy skip preserved).
  Plus a no-repo wiring assertion that with a legacy `""` the member scan skips and the drift guard
  is inert (existing `TestShipmentGate_*` no-repo tests keep passing unchanged — regression proof).
* **Milestone:** `go test ./internal/core/ -run 'HeadDrift|HeadResolve|HeadSHABounded|ShipmentGate'`
  green; `go vet` clean.
* **Depends on:** Unit 2 (shares the `shipmentHead` plumbing into `validateMemberGateEvidence`).

## Dependency Graph

```
Unit 1 (helper + guard)  →  Unit 2 (wire + rework tests)  →  Unit 3 (head-drift binding)
```

Acyclic, strictly ordered. Unit 2 blocks-on Unit 1; Unit 3 blocks-on Unit 2 (it reuses the
`shipmentHead` value that Unit 2 threads into `validateMemberGateEvidence`). All three are
single-domain (Go code + its Go tests).

## Decisions and Rationale

* **Direct-exec `*Workspace` helper (deliberation Option A), not an injected `GitRunner`.**
  Mirrors `headSHA`/`commits.go` (core already execs git directly); honors the operator's
  argv+MinimalEnv instruction; minimal blast radius; real-repo tests exercise the actual
  `--is-ancestor` exit semantics (the security-load-bearing part), which a mock would not.
* **Equality fast-path before any git call.** Preserves single-commit behavior, avoids a
  subprocess in the common case, and keeps the exact-equal case correct without depending on a
  git repo being present.
* **SHA-shape guard on the recorded head.** `head_sha` comes from on-disk evidence JSONL
  (tamperable — the closure doc calls hand-editing it a tampering vector). Validating it as a
  git object name (`^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` — exactly the SHA-1/SHA-256 shapes
  `git rev-parse` produces) before exec is the "data must not choose the args" defense; a
  non-conforming value fails closed.
* **Head-binding drift guard, not a resolved-SHA parameter (attempt-1 Security P1).** Checks #1 and
  #2 must observe the same head, but `ev.HeadRef` is intentionally pinned to `"HEAD"`
  (`gate/types.go`) so we cannot pass a resolved SHA into `Evaluate` without reopening the
  empty-diff-ref weakness. Resolve once + re-resolve + refuse-on-drift preserves both invariants and
  keeps no-repo tests green (`"" == ""`).
* **Error → fail closed.** A security guard must never let an unverifiable lineage pass. Exit
  128 / exec failure / shallow-missing-objects / timeout all block, wrapped `%w` + `slog.WarnContext`.
* **Empty-member-head clause preserved verbatim.** That is B85DAEE8's scope. Ancestor-correctness
  does not require touching it (an evidence event with no head cannot be lineage-checked at all).
  The separate empty-*shipment*-head fail-open is FLAGGED as a new follow-up, not fixed here.

## Risks and Caveats

* **Shallow clone.** If the member's ancestor object is beyond the shallow boundary,
  `--is-ancestor` errors (exit 128) → block (fail-closed). Correct behavior; operational
  remediation is `git fetch --unshallow`, not a code change. Called out in hardening.
* **Detached HEAD / non-repo.** `rev-parse HEAD` still yields a SHA in detached HEAD (fine); in
  a non-repo `shipmentHead == ""` so the entire staleness block is skipped exactly as today.
* **Test harness has no git repo today.** Mitigated by the new real-repo fixture; the existing
  fake-SHA test is reworked, not left to break.
* **Windows CI.** `git` is on PATH in CI; `exec.CommandContext(ctx, "git", ...)` + `MinimalEnv`
  (which forwards ambient `PATH`/`PATHEXT`) resolves it. Fixture uses argv, no shell.
* **Regression surface.** The other members-scan behaviors (terminal-status check, composed
  passing/forced predicate, F5 DecisionError fidelity) are untouched; only the staleness branch
  and its doc comment change.

## Constitution Check

Mapping this change against every principle in `.github/instructions/constitution.instructions.md`:

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Go 1.24; no `unsafe`; argv-array exec + `MinimalEnv`; `golangci-lint`/`go vet`/`gofmt` in the quality gate. **PASS** |
| II. Test-First (NON-NEGOTIABLE) | All three units are test-first: real-git-repo RED (false-staleness) → GREEN; divergent-rejected, equality, malformed, error, and pure drift-helper tests precede wiring. **PASS** |
| III. Workspace Isolation & Security | `cmd.Dir = ws.RootPath`; no path outside the workspace; `head_sha` validated (no traversal/injection); no secrets logged (`slog` logs SHAs + member id only). **PASS** |
| IV. CLI Workspace Containment | Planning only; no out-of-workspace writes; commits are path-scoped to plan/decision/memory + backlog state. **PASS** |
| V. Structured Observability | `slog.WarnContext` on every fail-closed staleness branch (malformed-SHA AND lineage-error) surfaces the guard's decision; git error wrapped `%w`. **PASS** |
| VI. Single Responsibility | No new module dependencies; reuses `os/exec`, `regexp`, `time`, existing `gate.MinimalEnv`. Each unit single-domain. **PASS** |
| VII. Destructive Command Approval | Read-only path; `--is-ancestor` mutates nothing; no destructive command introduced. **PASS** |
| VIII. Explicit Safety Modes | Security-sensitive gate-semantics change → plan-harden invoked; `## Plan Hardening` section present with invariants + ProposedAction/ActionRisk/controls. **PASS** |
| IX. Git-Friendly Persistence | No new persisted state; the check remains a pure read. **PASS** |
| X. Agent Context Efficiency | N/A to runtime code; planning used query-driven backlog lookup. **PASS** |
| XI. Merge Commit Preservation | Out of scope (Ship owns merges); unaffected. **N/A** |
| Overlay — backlogit | Planning used backlogit query/queue lookup (not manual scan); harvest records explicit ST1→ST2→ST3 dependency edges + task traceability to this plan; index synced. **PASS** |
| Overlay — agent-intercom | Broadcasts surfaced inline (no MCP broadcast tool in this CLI-mode session); remote-visibility degradation noted, no approval step silently bypassed. **PASS (degraded, disclosed)** |
| Overlay — agent-engram | Not enabled in this workspace registry. **N/A** |
| Quality Gates | Runtime verification requires `go test ./... && go vet ./... && golangci-lint run && gofmt -l .` all green (083-S parity). **PASS** |
| Task Granularity (2-Hour Rule) | 1 feature + 1 task + 3 subtasks; each < 3 files, < 5 funcs, ≤ 4 test scenarios, single-domain. **PASS** |

No principle is violated. The change strengthens Principle III/V (adds a validated-input security
guard + observability on a previously silent branch).

## Plan Hardening Signals (REQUIRED)

* **public API, schema, or contract change** — **Absent.** No exported signature, no CLI/MCP
  surface, no schema change. Internal `validateMemberGateEvidence` behavior only.
* **security, auth, permission, or compliance-sensitive behavior** — **PRESENT.** The staleness
  check is a security guard proving gated members' work is included in the shipment. Changing
  its semantics is security-sensitive, and a new git exec consumes untrusted on-disk input.
* **migration, backfill, destructive/irreversible action** — **Absent.** No data migration; no
  writes to evidence logs; the check is read-only and returns before any state mutation.
* **external integration / operator checkpoint / external dependency** — **PRESENT (mild).**
  Introduces a new `git merge-base --is-ancestor` subprocess on the lock-holding ship path
  (external `git` process; must be bounded and fail-closed).
* **high runtime, rollout, or rollback risk** — **Absent (low).** Rollback = revert one branch;
  no persistent state changes. Runtime risk limited to one bounded subprocess per non-equal
  member.

**Requires plan hardening: yes** (security-sensitive gate-semantics change + new exec on a
lock-holding critical path).

## Runtime Verification and Closure

* **Runtime surface changed:** the `backlogit shipment ship` gate path (member-evidence
  validation). No CLI flags, MCP tools, or config change.
* **Runtime verification (what must be proven before absorption):**
  * Unit tests demonstrate RED (false staleness rejection under equality) → GREEN (accepted
    under ancestor-aware) with real commits, plus divergent-rejected, equality-accepted,
    empty-head-unchanged, malformed-blocked, and error-blocked.
  * `go test ./... && go vet ./... && golangci-lint run && gofmt -l .` all green (matches the
    083-S quality-gate bar).
  * Post-merge integration proof (Ship, after this ships): re-run
    `backlogit shipment ship 083-S --sha ac41bb1…` on a `post-merge/083-S` branch and confirm
    the member scan now passes (the acceptance signal recorded in the closure doc).
* **Operational closure:** revert trigger = any real weakening surfaced by adversarial review
  or a shipment closing that should have been refused; rollback = revert the single feature
  branch (no state cleanup); owner = Ship at closure time; validation window = first successful
  `083-S` post-merge closure.

<!-- plan-harden appends the authoritative "## Plan Hardening" section below. -->

## Plan Hardening

**Hardening required:** yes. Triggered by two signals — (a) a **security-sensitive
gate-semantics change** (the member-evidence staleness check is a guard that gated members'
work is actually included in the shipment), and (b) a **new external `git` subprocess on the
lock-holding `shipment ship` critical path** consuming **untrusted on-disk input** (`head_sha`
read from evidence JSONL).

### Learnings and instruction files consulted

* `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` — "data must not
  choose the code/args"; argv-array-only exec + `MinimalEnv` + input validation must hold
  together.
* `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — every external-process
  call on a lock-holding path must be context-bounded; an unbounded child under the lock is a DoS.
* `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` — the shipment
  member scan contract; the one-way `core → gate` boundary; the injected-runner principle scoped
  to the autoharness seam (not core's own git helpers).
* `docs/closure/2026-07-06-083-S-post-merge-closure-BLOCKED.md` — the reproduction, the nine
  ancestor SHAs, and the "no supported bypass / do not force" boundary.
* `.github/instructions/strict-safety.instructions.md`, `.github/instructions/go.instructions.md`.

### Protected invariants (must survive the change)

1. **Non-bypass:** a member that did not pass its gate (missing/`ran=false`/non-terminal) is
   still refused — only the staleness *branch* changes; the terminal-status check and the
   composed passing/forced predicate are untouched.
2. **No state mutation on refusal:** `validateMemberGateEvidence` remains a pure read that
   returns before any evidence append or shipment-state change (verified no-op preserved).
3. **Empty-head bypass unchanged:** the `h != ""` clause stays verbatim (B85DAEE8 scope).
4. **Aggregate full-diff check #2 unchanged:** `gateShipmentCompletion`'s shipment-level
   `gate check` still runs and still catches post-gate file modifications.
5. **F5 error-class fidelity unchanged:** the DecisionError exit-7/8 handling below the member
   scan is not touched.
6. **Fail-closed:** any lineage that cannot be *proven* included is refused (never silent pass).
7. **Same-head binding + fail-closed head resolution (attempt-1/2/3 Security P1):** the
   member-lineage check (#1) and the aggregate full-diff check (#2) must observe the same resolved
   shipment head; the drift guard is the LAST read before the success path, bracketing the whole
   evaluation, so any HEAD advance across the window fails closed under enforcement. A bounded head
   read that times out or is cancelled fails closed (never a silent skip). Only a legacy non-context
   empty head preserves the pre-existing (FLAGGED) skip, so no-repo tests do not regress.

### Risky actions (ProposedAction / ActionRisk / ActionResult)

* **ProposedAction A1 — introduce `git merge-base --is-ancestor` subprocess on the ship path.**
  * **ActionRisk:** Medium. External process on a lock-holding critical path; consumes untrusted
    `head_sha`. Risks: unbounded hang (DoS), argument injection, false pass on error.
  * **Controls (all required):**
    * argv-array exec `exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", h, shipmentHead)`
      — literal `"git"`, no shell, no string interpolation of `h`.
    * `cmd.Env = gate.MinimalEnv()` — allowlisted env.
    * **mandatory self-bounded `ctx`** — the ship path is **unbounded**
      (`ShipShipment`/`gateShipmentCompletion` pass the caller `ctx` straight through and `headSHA`
      is unbounded too), so `isAncestor` derives its OWN deadline with `context.WithTimeout` (source:
      `ws.GateBroker.TimeoutSeconds`, else a small package default ~5s) — mirrors `gate/broker.go`.
      A hung `git` can never stall under the workspace lock. Do **not** depend on the caller
      imposing a deadline.
    * **timeout checked BEFORE the exit code** — after `cmd.Run()`, test
      `stderrors.Is(runCtx.Err(), context.DeadlineExceeded)` first (mirrors `gate/runner.go:83-94`);
      a deadline-killed git reports a platform-dependent code (e.g. -1 on Windows) that must never
      be read as exit-1 "divergent". Only then read `*exec.ExitError`.
    * **SHA-shape guard** `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` on `h` **before** exec; a
      non-conforming value is refused (block), never handed to git — closes the argument-injection
      vector (a leading-`-`
      value can never reach git) and rejects tampered/garbage heads.
    * **exit-code trichotomy:** 0 → included; 1 → divergent → block; other/exec-error → block.
  * **Approval needed:** no operator checkpoint at execution time (deterministic, read-only);
    the gate-semantics decision is approved via this plan's review.
  * **ActionResult (expected):** for well-formed inputs, a boolean lineage decision within the
    ship deadline; for malformed/error inputs, a typed `*GateBlockedError` refusal.

* **ProposedAction A2 — relax the staleness acceptance predicate (equality → ancestor).**
  * **ActionRisk:** Medium (security semantics). Risk: over-accepting evidence that should be
    refused.
  * **Control:** the acceptance set widens only from `{h == head}` to `{h is-ancestor-of head}`,
    which is exactly "the gated commit is contained in the shipment history." Divergent heads
    remain refused; the aggregate full-diff check #2 remains the guard for post-gate
    modifications. See the deliberation's security reasoning (the crux) — reviewers must confirm
    no weakening. **If a reviewer finds a real weakening, this is gate-blocking.**
  * **ActionResult (expected):** false-staleness rejections eliminated; divergent + error +
    malformed still refused; equality still accepted.

* **ProposedAction A3 — bracket the whole gate evaluation with a stable-head assertion (attempt-1/2
  Security P1).**
  * **ActionRisk:** Medium (security semantics). Risk: check #1 (member lineage) and check #2
    (aggregate full-diff) observing DIFFERENT resolutions of the pinned symbolic `HEAD`, or HEAD
    advancing anywhere across the evaluation window, letting ancestor-aware admit a head-advance
    that strict equality would have caught.
  * **Controls (all required):**
    * Resolve `shipmentHead, headErr := ws.headSHABounded(ctx)` **once**, before `Evaluate`; thread
      that single value into `validateMemberGateEvidence` (no re-resolution inside the member scan).
    * Bound BOTH head reads via `headSHABounded` (`context.WithTimeout`, same source/fallback as
      `isAncestor`) — the ship path is unbounded, so an unbounded `git rev-parse` must not stall
      completion (attempt-2 Go P2).
    * **A bounded-read failure fails closed, never skips (attempt-3 Security P1).** `headSHABounded`
      returns `("", ctxErr)` on deadline/cancel; under enforcement the caller returns
      `headResolveError` (typed `*GateBlockedError`) for either the pre- or post-read. Only a
      **legacy** non-context `""` (rev-parse failed for a non-timeout reason, e.g. non-repo) keeps
      the pre-existing skip. The new timeout path must not widen the legacy fail-open.
    * Make the drift re-resolution the **LAST read before the success path** returns — after the
      block/error decision branches, immediately before the passing-evidence append — so it brackets
      BOTH Evaluate (#2) and the member scan (#1). Refuse (`headDriftError` → `*GateBlockedError`) if
      `ev.Enforced && shipmentHead != "" && postHead != shipmentHead`. Closes the attempt-2 residual
      TOCTOU window.
    * `ev.HeadRef` stays pinned to `"HEAD"` (do NOT pass a resolved SHA into `Evaluate` — that would
      reopen the empty-diff-ref weakness that pinning was added to prevent, `gate/types.go:31-34`).
    * The legacy empty-`shipmentHead` skip is NOT changed (see FLAGGED item); the drift guard is
      inert when `shipmentHead == ""` with `headErr == nil`, so no existing no-repo test regresses.
  * **ActionResult (expected):** checks #1 and #2 provably observe the same tree; any HEAD advance
    across the evaluation window, and any bounded-read timeout/cancel, fails closed instead of
    silently admitting new work or skipping the guard.

### Discovered adjacent issue (FLAGGED — not fixed here)

While hardening the head-binding argument, plan-review surfaced a **pre-existing fail-open**: in
`gateShipmentCompletion`, when the gate is **enforced** but HEAD resolves to `""` for a
**non-context reason** (git rev-parse fails because the tree is not a repo — e.g. the no-repo test
harness), the member-lineage staleness check is skipped entirely rather than failing closed. This
predates this change: the current code already resolves the head exactly once
(`ws.headSHA(ctx)` at `shipment_gate.go:60`) and skips staleness on `""`. This revision **reuses
that single resolution** (moved earlier and bounded), so the **legacy** empty-head fail-open
semantics are unchanged. Note: the NEW bounded-read timeout/cancel path introduced by
`headSHABounded` does **not** widen this fail-open — it fails closed (attempt-3 Security P1); only
the pre-existing non-context empty case remains deferred. Closing the legacy fail-open (enforced +
non-context empty → refuse) would break many existing no-repo shipment tests that deliberately rely
on the skip (`TestShipmentGate_AllMembersHaveEvidence_Ships` and peers), is orthogonal to the
equality→ancestor change, and would silently expand scope. Per the operator's "in order" instruction
and the no-silent-scope-expansion rule, this is **FLAGGED as a new follow-up stash `1AEA2B0E`**
(low/bug, sibling to B85DAEE8's empty-*member*-head case), NOT fixed in this item.

### Added verification detail

* Unit tests must cover the full matrix on a **real git repo fixture**: ancestor-accepted (the
  RED→GREEN case), divergent-rejected, equality-accepted, empty-head-unchanged, malformed-blocked,
  non-existent-ref-error-blocked.
* Add a targeted assertion that an equal head does **not** spawn a subprocess is optional; the
  behavioral guarantee (equal → accept even with no reachable ancestor object) is covered by the
  equality test.
* Quality bar (083-S parity): `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
  all green before Ship pushes.
* Adversarial/security review at Ship time must re-run the security reasoning and specifically
  probe: (i) can a divergent-but-crafted head be accepted? (ii) does any git error path silently
  pass? (iii) is the child process bounded under the lock? (iv) can HEAD drift between checks #1
  and #2 admit work strict equality would have caught (the drift guard must refuse it)?

### Rollback and closure

* **Rollback trigger:** adversarial/security review finds a real weakening, OR a post-fix
  shipment closes that should have been refused.
* **Rollback procedure:** revert the single feature branch. No persistent state to unwind (the
  check writes nothing on the read path).
* **Owner:** Ship, at `083-S` closure time.
* **Validation window:** first successful `083-S` post-merge closure (`shipment ship 083-S --sha ac41bb1…`)
  is the live acceptance signal; watch that no genuinely divergent evidence is admitted.

### Unresolved operator decisions

None blocking. Deferred-by-design: B85DAEE8 (empty-member-head) and F3844849 (malformed JSONL)
remain separate stash items and are explicitly untouched here. A new follow-up stash `1AEA2B0E`
(low/bug, empty-*shipment*-head enforced fail-open) is FLAGGED above for a future session.

## Plan Review

<!-- plan-review-attempt: 1 -->

### Attempt 1 — verdict: FAIL (2 × P1)

Five reviewers ran cross-model in parallel (Security Lens, Go, Constitution, Scope Boundary,
Architecture Strategist). Scope Auditor and Architecture Strategist returned PASS; the gate failed
on two P1 findings. Resolutions (folded into the units/hardening above) recorded here for the
attempt-2 re-review:

| # | Reviewer | Sev | Finding | Resolution in this revision |
|---|---|---|---|---|
| F1 | Security Lens | P1 | Non-weakening argument has a gap: check #1 (member lineage) and check #2 (aggregate full-diff) are not bound to the SAME resolved HEAD (`Evaluate` resolves pinned `HEAD` independently of `ws.headSHA(ctx)`); `headSHA` returns `""` on error → member-lineage silently skipped (fail-open under enforcement). Ancestor-aware can admit a head-advance strict equality would have caught. | **Unit 3 added**: resolve `shipmentHead` once before `Evaluate`; HEAD-drift guard re-resolves after and fails closed on drift under enforcement; single value threaded into `validateMemberGateEvidence`. `HeadRef` stays pinned. R9 + invariant #7 + ProposedAction A3 + updated crux. Empty-`shipmentHead` skip left unchanged and separately FLAGGED. |
| F2 | Go | P1 | `isAncestor` runs on the UNBOUNDED ship path (confirmed `shipment_lifecycle.go:171` passes `ctx` straight through; `headSHA` also unbounded) — a hung git stalls under the workspace lock. Timeout must be MANDATORY, and `ctx.Err()`/`DeadlineExceeded` must be checked BEFORE the exit-code trichotomy (mirror `runner.go:83-94`, not `baseref.go`). | **Unit 1 revised**: `isAncestor` derives its own `context.WithTimeout` (from `GateBroker.TimeoutSeconds`, else ~5s default), mandatory; timeout checked before `*exec.ExitError`; stderr captured; errors wrapped `%w`. R5/R8 added, ProposedAction A1 controls updated. |
| F3 | Go | P2 | Test rework mandatory (fake `oldsha0000` now hits the malformed branch, not divergent); divergent test MUST use a real sibling-branch commit (exit 1); fixture must set committer+author identity + `init.defaultBranch=main` + git-init the SAME dir as `Workspace.RootPath`; assert specific branch messages not just `Contains("stale")`. | **Unit 1/Unit 2 tests revised** accordingly: real-repo fixture `initGitRepoWithCommits` in `ws.RootPath`, both identities set, `-c init.defaultBranch=main`, real ancestor + real sibling-divergent commits, per-branch message assertions. |
| F4 | Go | P3 | Capture git stderr, wrap with `%w`; consider guarding `descendant`/`shipmentHead` too; tighten regex to producible SHA shapes. | Adopted: `cmd.Stderr` buffer + `%w`; regex tightened to `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`. `shipmentHead` originates from `git rev-parse` (trusted) so it is not shape-guarded, documented as trusted-provenance. |
| F5 | Constitution | P2 | Missing a dedicated "Constitution Check" section (Governance mandates it). | **Constitution Check section added**, mapping all XI principles + quality gates + 2-hour rule. |
| F6 | Constitution | P3 | Use `%w` not `%v` for git error chaining; add `slog.WarnContext` on fail-closed git-error/malformed branches. | Adopted (both branches in attempt 2): `%w` in the helper; `slog.WarnContext` on the lineage-error AND malformed-SHA fail-closed branches (Principle V). |
| — | Scope Boundary Auditor | PASS | SHA-shape guard + fail-closed confirmed INTRINSIC (not creep); empty-head bypass + F3844849 provably untouched; decomposition right-sized. | No change required. The new Unit 3 stays in-scope (it hardens the SAME staleness semantic) and the empty-`shipmentHead` fail-open is explicitly FLAGGED, not fixed. |
| — | Architecture Strategist | PASS | Option A consistent with `headSHA`/`commits.go` precedent + one-way core→gate boundary. | No change required. |

<!-- plan-review-attempt: 2 -->

### Attempt 2 — verdict: FAIL (1 × P1)

Re-ran Security Lens, Go, and Constitution cross-model. **Go = ADVISORY** (attempt-1 P1 confirmed
resolved for `isAncestor`); **Constitution = ADVISORY** (Constitution Check section confirmed
complete across I–XI + gates + granularity; two P3s). **Security Lens = FAIL** on a refined P1:
the attempt-1-designed drift guard (re-resolve only *between* Evaluate and the member scan) left a
**residual TOCTOU window** — HEAD could still advance around/after the member scan and the shipment
would complete against a head neither check fully validated. Resolutions folded into the units above:

| # | Reviewer | Sev | Finding | Resolution in this revision (for attempt 3) |
|---|---|---|---|---|
| G1 | Security Lens | P1 | Drift check placed only immediately after `Evaluate` leaves a residual TOCTOU window: a HEAD advance after the post-Evaluate re-read but during/around the member scan is uncaught, so completion proceeds against a head neither check validated — ancestor-aware then admits a member whose old head is an ancestor of the advanced HEAD where strict equality (fresh HEAD read at member-scan time) would have rejected. | **Unit 3 re-placed**: the drift re-resolution is now the **LAST read before the success path** (after the block/error branches at `shipment_gate.go:73-108`, before the passing-evidence append at 110), so it **brackets the entire evaluation** (Evaluate #2 + member scan #1). Any HEAD advance across the whole window fails closed. R9/A3/invariant #7 updated. |
| G2 | Security Lens | (P1 sub) | Moving the head read earlier appeared to create a NEW enforced fail-open on empty pre-resolution. | Rebutted + documented: the current code already resolves the head exactly once (`shipment_gate.go:60` arg) and skips on `""`; this revision reuses that single value, so empty-head semantics are **unchanged — not a new fail-open**. The enforced-empty fail-open is the **separately FLAGGED** pre-existing issue (would break no-repo tests; operator-scoped out). Held the scope line. |
| G3 | Go | P2 | `headSHA` used for the drift reads has no internal timeout and the ship ctx is unbounded — a hung `git rev-parse` could stall completion. | **`headSHABounded` helper added** to Unit 3: wraps `headSHA` under `context.WithTimeout` (same source/fallback as `isAncestor`); both drift reads use it. Returns `""` on error (behavior-preserving). |
| G4 | Constitution | P3 | Constitution Check omits dedicated capability-overlay rows (backlogit/agent-intercom/agent-engram). | Adopted: three overlay rows added (backlogit PASS, agent-intercom PASS-degraded, agent-engram N/A). |
| G5 | Constitution | P3 | `slog.WarnContext` was added only to the lineage-error branch, not the malformed-SHA branch (partial F6 adoption). | Adopted: `slog.WarnContext` added to the malformed-SHA fail-closed branch too (Unit 2 snippet). |
| — | Go | ADVISORY (was P1) | `isAncestor` mandatory-timeout + timeout-before-ExitError design confirmed **RESOLVED**. | No further change (G3 is the only residual, now fixed). |
| — | Constitution | ADVISORY | Constitution Check section confirmed complete; Test-First confirmed genuine across all 3 units. | No blocking change. |

Attempt-2 marker recorded. Attempt-3 re-review targets the Security Lens P1 (drift-guard
placement) as the gate-blocking item; Go/Constitution were already ADVISORY.

<!-- plan-review-attempt: 3 -->

### Attempt 3 — verdict: FAIL (1 × P1) — CYCLE CAP REACHED → operator intervention required

Re-ran Security Lens + Go cross-model. **Go = PASS** (attempt-2 G3 residual confirmed resolved;
`headSHABounded` + last-read drift placement sound; no new nil/cancel/context-leak; no-repo behavior
inert). **Security Lens = FAIL** but explicitly **confirms the attempt-2 P1 (TOCTOU drift window) is
RESOLVED** — the last-read bracketing closes it and is not a weakening versus the actual pre-fix
baseline (strict equality also never re-read after the member scan). It surfaces a **new, narrower
P1 (confidence 0.84)**:

| # | Reviewer | Sev | Finding | Resolution applied (ready for attempt 4, pending operator authorization) |
|---|---|---|---|---|
| H1 | Security Lens | P1 | `headSHABounded` returning `""` on its **own** timeout/cancel collapses a NEW synthetic condition into the legacy empty-head skip. Today `headSHA` returns `""` only on resolution error; an unbounded rev-parse would *hang*. My bounded helper converts that hang into `""` → skip, so a pre-read timeout makes BOTH the member-lineage check and the drift guard inert under enforcement — a **new fail-open** introduced by this change (distinct from the pre-existing, FLAGGED, deferred non-context empty case). | **Unit 3 revised**: `headSHABounded` now returns `(string, error)` — `("", ctxErr)` on deadline/cancel, `(sha, nil)` on success, `("", nil)` on a **legacy** non-context resolution failure. Under enforcement, a non-nil `headErr` on the pre- OR post-read returns `headResolveError` (typed `*GateBlockedError`, **fail closed**). Only the legacy non-context `""` preserves the pre-existing skip (no-repo tests stay green). The new timeout path does NOT widen the legacy fail-open. R9/A3/invariant #7/FLAGGED updated; `headResolveError` helper + its test added. |
| — | Go | PASS | `headSHABounded` (bounded), last-read drift placement, malformed-branch `slog`, and `isAncestor` timeout design all confirmed correct. | No change required. |

**Cycle-tracking status (P-005):** This is the 3rd plan-review attempt and the 2nd (final) permitted
re-entry cycle. Per the NON-NEGOTIABLE cycle cap ("Maximum 2 re-entry cycles; after attempt count
reaches 3, halt and require operator intervention"), Stage **HALTS here and does not auto-run a 4th
review**. The remaining H1 fix is fully specified and applied to this plan; the reviews are
converging (Go PASS; Security confirmed every prior P1 resolved; H1 is a single, well-scoped,
high-confidence hardening with a precise fix). Operator decision required before proceeding — see the
session report.

<!-- plan-review-attempt: 4 -->

### Attempt 4 — verdict: PASS (operator-authorized confirmation cycle)

The operator authorized ONE confirmation re-review (override of the cycle cap) scoped to confirming
the H1 fix and re-confirming Security Lens + Go. Both reviewers returned clean, cross-model:

| Reviewer | Verdict | Confirmation |
|---|---|---|
| Security Lens | **PASS** | **H1 resolved.** The `headSHABounded(ctx) (string, error)` split cleanly captures bounded timeout/parent-cancel (`h == "" && bctx.Err() != nil`) as a NEW fail-closed condition while preserving legacy non-context `""` as the pre-existing (deferred) skip only. No realistic path where a true timeout/cancel becomes `headErr == nil`; a legitimate no-repo stays `("", nil)` with a live parent ctx so no-repo tests do not regress. Both pre-read and post-read enforced failure paths are fail-closed; no enforced success path returns `nil` after swallowing a bounded-read error. FLAGGED legacy empty-shipment-head deferral is unchanged (not widened). End-to-end non-weakening argument holds: ancestor/equal accepted, divergent rejected, both checks bracketed to one stable head, drift + git timeout/cancel fail closed. **No new P0/P1.** |
| Go | **PASS** | No findings. Attempt-3 PASS holds; the `(string, error)` helper, wiring, `headResolveError`, and last-read drift placement are correct Go with no context leak / double-cancel / unused-variable issue and no no-repo regression. |

**GATE OUTCOME: PASS.** Security-lens non-weakening reasoning is explicitly resolved and confirmed;
all git-exec/timeout/drift error paths fail closed; Constitution Check complete. The plan is approved
for harvest. Proceeding to Step 5 (decomposition) → Step 5.5 (queued shipment) → Step 5.6 (archive).
