---
chunk_strategy: h1-h2-h3
description: 'Pre-push multi-model adversarial review for shipment 084-S (ancestor-aware shipment-gate member-evidence staleness) feature PR #182. Three independent reviewers (gpt-5.4, claude-sonnet-4.6, claude-opus-4.8) reviewed the strict-equality to ancestor-aware staleness change in internal/core/shipment_gate.go. Zero gate-blocking P0/P1 findings; the non-weakening property (member gated commit reachable in shipment lineage; final-tree integrity delegated to the unchanged aggregate diff check #2) and fail-closed discipline (git/exec/timeout/cancel/malformed/head-drift all refuse) were confirmed by all three reviewers. Advisory P2/P3 findings recorded with dispositions; ADV-7 (guaranteed-true ev.Enforced invariant) remediated pre-push comment-only; ADV-2/ADV-3/ADV-5 stashed as follow-ups (not regressions from the strict-equality baseline). Post-remediation gates green. GATE PASS.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-084-S-feature-pr-adversarial-review.md
title: 084-S Ancestor-Aware Shipment-Gate Staleness — Feature PR Pre-Push Adversarial Review
---

# 084-S Feature PR — Adversarial Review (pre-push)

**Shipment:** 084-S — Ancestor-aware shipment-gate member-evidence staleness
**Branch:** `feat/084-ancestor-aware-staleness`
**Date:** 2026-07-06
**Gate:** Ship Step 4 (adversarial review, MANDATORY, executed BEFORE push/PR)
**Reviewed commits:** `a09f20d` (Unit 1), `c3d8db6` (Unit 2), `7f609e0` (Unit 3), `23c4805` (backlog progress), `7ede042` (review clarification)
**Diff under review:** `git diff 05816c4..HEAD -- internal/core/`

## Change summary

`internal/core/shipment_gate.go` `validateMemberGateEvidence` previously rejected a member
unless its recorded gate-evidence `head_sha` was **strictly equal** to the current shipment
head. Post-merge, a member's feature-branch build commit is an **ancestor** of the shipment's
merge commit (not equal), so strict equality falsely rejected all valid evidence and blocked
multi-commit shipment closure (blocked 083-S). The fix makes the staleness check
**ancestor-aware**: a non-empty member head that is an ancestor-of-or-equal-to the shipment
head (`git merge-base --is-ancestor`) is accepted; genuinely divergent (non-ancestor) heads
are still rejected; every git/exec/timeout/cancel/malformed/head-drift path fails closed.

New symbols: `isAncestor` (bounded git exec), `isGitObjectName` (SHA-shape guard),
`headSHABounded` (bounded HEAD read → `(string, error)`), `headResolveError`, `headDriftError`.
`gateShipmentCompletion` resolves the shipment head once (bounded) before `Evaluate`, threads
that single head into the member scan (#1) and the aggregate diff check (#2), and re-resolves
as the **last read before success** (head-drift bracket, closes the TOCTOU window).

## Review panel (multi-model, 3 independent reviewers)

| Lens | Model | Verdict | Gate-blocking? |
|---|---|---|---|
| Security / fail-closed | `gpt-5.4` (high) | No weakening vs strict-equality baseline; every failure path refuses; input validation sound; scope preserved | **No** |
| Go correctness | `claude-sonnet-4.6` (high) | No P0/P1; Go-clean; context/exec/error-wrapping/trichotomy all correct; targeted tests pass | **No** |
| Decisions / security reasoning | `claude-opus-4.8` (high) | Sound for "no ungated/malicious code ships"; reachability-vs-inclusion documentation precision noted | **No** |

**Panel consensus: ZERO gate-blocking (P0/P1) findings.** All advisory findings are P2/P3.

## Operator security-focus confirmation (the crux)

1. **Non-weakening — CONFIRMED.** Relaxing acceptance from `{h == head}` to `{h is-ancestor-of head}`
   does not admit any member whose gated commit is outside shipment history: `--is-ancestor`
   exit 0 proves the recorded member head is **reachable from** the shipment head, i.e. contained
   in shipped lineage. Divergent heads (exit 1) are still rejected. The residual risk that a
   member's gated content is later reverted/overwritten (reachability ≠ final-tree content
   survival) is covered by the **unchanged** shipment-level aggregate full-diff check #2
   (`Evaluate` over `base..HEAD`), which gates whatever is actually in the shipped tree. The
   security property the gate protects — *no ungated or malicious code reaches a shipped release*
   — is preserved. (Precision note captured below as ADV-1.)
2. **Fail-closed — CONFIRMED for all paths.** The `gpt-5.4` reviewer walked every failure mode of
   `isAncestor` and `headSHABounded`:
   - context deadline / parent cancel → refuse (the `runCtx.Err() != nil` check is ordered
     **before** the `*exec.ExitError` trichotomy, so a Windows context-killed process reporting
     exit code 1 can never be misread as the exit-1 "not-ancestor" `(false, nil)` result);
   - exit 128 / any non-{0,1} code / git-missing / exec-start failure → error → refuse;
   - malformed `head_sha` → blocked by `isGitObjectName` before any exec;
   - valid-shape but absent object → git exit 128 → refuse;
   - bounded HEAD read timeout/cancel (pre- and post-read) → `headResolveError` → refuse.
   The only non-fail-closed path is the **preserved legacy** non-context empty-shipment-head skip
   (returns `("", nil)`), which is out-of-scope (follow-up `1AEA2B0E`) and, per the `opus`
   reviewer, is compensated because an unresolvable HEAD under enforcement also breaks check #2's
   diff resolution (which fails closed).
3. **Input validation — CONFIRMED.** Untrusted on-disk `head_sha` is validated against
   `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` (package-level compiled regexp) **before** reaching git;
   the git call is argv-array only (no shell) with `cmd.Env = gate.MinimalEnv()`. A leading-dash
   or garbage value can never become a git option/ambiguous ref.
4. **Scope discipline — CONFIRMED.** The empty-member-head bypass (`h != ""`, `B85DAEE8`) is
   preserved verbatim; the legacy empty-shipment-head skip is not widened; the new bounded-read
   timeout/cancel path does fail closed under enforcement.

## Findings & dispositions

All findings are advisory (P2/P3). None block the pre-push gate.

| ID | Sev | Conf | Source | Finding | Disposition |
|---|---|---|---|---|---|
| ADV-1 | P2 | 0.68 | opus | Non-weakening argument should be stated as **reachability** ("member's gated commit is reachable in shipment lineage"), not "member's work is included" — content survival is delegated to check #2. | **Documented** in this report + non-weakening section above. Code is correct; this is documentation precision. Plan phrasing noted for the deliberation follow-up. |
| ADV-2 | P2 | 0.78 | gpt-5.4 / opus | **ABA / there-and-back HEAD race:** the drift bracket detects *net* HEAD drift between two `rev-parse` samples but not a transient `H0→H1→H0` excursion, and does not bind autoharness's *internal* HEAD resolution to backlogit's samples. | **Follow-up stashed.** Both reviewers state this is **NOT a regression** from the strict-equality baseline (the pre-fix code relied on the same ambient HEAD resolution). Requires local repo write access in a seconds-wide window (low exploitability). |
| ADV-3 | P2 | 0.60 | opus | Gate anchors to ambient `git rev-parse HEAD`, not the explicit `--sha` target commit passed to `ShipShipment`. | **Follow-up stashed.** Pre-existing design property (the strict-equality baseline also compared against ambient HEAD); not introduced by this change. Recommend threading the target SHA in a future hardening. |
| ADV-4 | P3 | 0.55 | opus | Deferred enforced empty-shipment-head fail-open (`1AEA2B0E`) lacks a documented compensating control. | **Documented.** Compensating control recorded: an enforced unresolvable HEAD also breaks check #2 (fail closed), so the deferred skip is not an independently exploitable bypass. Out-of-scope stash `1AEA2B0E` unchanged. |
| ADV-5 | P3 | 0.52 | opus | Ancestor-awareness enlarges the tamper set of a forged on-disk `head_sha` from `{HEAD}` to `{any ancestor}`; interacts with excluded `B85DAEE8` / `F3844849`. | **Noted for joint follow-up.** SHA-shape guard + argv exec keep injection closed; only a real ancestor commit passes. An attacker who can edit the JSONL could already forge `head_sha == HEAD` under strict equality, so not a material increase. Flag when `B85DAEE8`/`F3844849` are addressed. |
| ADV-6 | P3 | — | sonnet | `headResolveError` formats the context `cause` with `%v` not `%w`, so `errors.Is(err, context.DeadlineExceeded)` is false (only `*GateBlockedError` is traversable). | **Accept as-is.** Intentional — `*GateBlockedError` is the routing type; no current caller needs to distinguish timeout-class blocks. Documented here for future maintainers. |
| ADV-7 | P3 | — | sonnet | `if ev.Enforced && shipmentHead != ""` at the drift site has a guaranteed-true `ev.Enforced` sub-condition (foreclosed by the `!ev.Enforced` early return). | **Remediated** (comment-only) in `7ede042` — added an explicit invariant comment; `shipmentHead != ""` marked as the load-bearing no-repo guard. |
| ADV-8 | P3 | — | sonnet | Test helper `initGitRepoWithCommits` appends `GIT_AUTHOR_*` identity vars to the end of `os.Environ()`; on some libc a first-occurrence-wins ambient var could shadow them. | **Accept as-is.** Belt-and-suspenders `git config user.name/email` local config already mitigates; Go's `os/exec` dedup keeps the last occurrence. Low practical risk; noted. |

## Remediation applied pre-push

- `7ede042` — comment-only clarification of the guaranteed-true `ev.Enforced` invariant at the
  drift-guard site (ADV-7). No behavior change. Full suite re-verified green afterward.

No P0/P1 findings required remediation.

## Quality gates (post-remediation)

- `go test ./...` — **PASS** (all packages)
- `go vet ./internal/core/` — **PASS**
- `golangci-lint run ./internal/core/...` — **PASS** (exit 0)
- `gofmt -l internal/core/shipment_gate.go` — **clean** (LF-normalized)

## Verdict

**PASS.** No gate-blocking (P0/P1) findings from any of the three independent reviewers. The
ancestor-aware change is confirmed **non-weakening** relative to the strict-equality baseline:
divergent heads remain rejected, all git/exec/timeout/cancel/malformed/head-drift paths fail
closed, untrusted input is SHA-shape-validated before exec, and out-of-scope bypasses are
preserved unchanged. Residual advisory items (ADV-2/ADV-3/ADV-5) are stashed as follow-ups and
are not regressions from the pre-fix baseline. Cleared to push and open the feature PR.
