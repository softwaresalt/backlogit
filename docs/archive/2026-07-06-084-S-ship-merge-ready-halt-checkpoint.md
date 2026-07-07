# Ship session checkpoint — 084-S (ancestor-aware shipment-gate staleness) — MERGE-READY HALT

**Status:** MERGE-READY — HALTED for operator P-014 merge approval. Feature PR #182 is green,
adversarially reviewed (0 P0/P1), Copilot-clean (0 unresolved threads), runtime-verified. **NOT
merged.** Remaining on feature branch `feat/084-ancestor-aware-staleness`.

## Scope delivered

Security-sensitive fix to the shipment completion gate broker (`internal/core`): replace strict
`head_sha` EQUALITY staleness check with an **ancestor-aware** check
(`git merge-base --is-ancestor <member_head> <shipment_head>`), unblocking post-merge multi-commit
shipment closure while keeping every git-exec/timeout/cancel/malformed/head-drift path FAIL-CLOSED.

- Ancestor-or-equal member head → accepted (the fix).
- Genuinely divergent (non-ancestor, exit 1) member head → still refused.
- Equality → still passes (fast path).
- Bounded self-timeout (`boundedHelperTimeout`, 5s HARD CAP honoring smaller configured values);
  timeout/cancel/exec-error/malformed-SHA → fail closed.
- Head-drift bracket is the last read before appending passing evidence (no TOCTOU window).
- Aggregate check #2 (unchanged) backstops residual post-gate edits.
- Out of scope (untouched): empty-member-head bypass `B85DAEE8`, empty-shipment-head fail-open
  `1AEA2B0E`, malformed-JSONL `F3844849`.

## Backlog state (pre-merge, correct)

- `084.001.001-ST`, `084.001.002-ST`, `084.001.003-ST` → **done**
- `084.001-T` → **done**
- `084-F` (feature) → **active** (transitions to shipped in post-merge closure)
- `084-S` (shipment) → **active** (claimed; `shipment ship` runs post-merge, after operator approval)

## Branch / PR

- Branch: `feat/084-ancestor-aware-staleness` (HEAD `3069195`, == origin, fully pushed)
- PR: **#182** — https://github.com/softwaresalt/backlogit/pull/182
- Commits: `a09f20d` (U1 helper+guard) · `c3d8db6` (U2 wire+tests) · `7f609e0` (U3 drift bracket)
  · `23c4805` (backlog archive relocate) · `7ede042` (enforced-invariant comment) · `b409980`
  (adversarial review doc) · `88888d0` (docline frontmatter) · `c29b189` (Copilot fix: cap
  bounded git-helper timeout + dedup abort msg) · `3069195` (runtime-verify + operational-closure
  docs).

## Quality gates (final, all green)

- `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` → clean.
- CI on `3069195`: test (1.23) pass · test (1.24) pass · Docline frontmatter gate pass ·
  CLI Reference Drift pass.

## Adversarial review (mandatory pre-push, 3-model)

- Models: gpt-5.4, claude-sonnet-4.6, claude-opus-4.8. **Consensus: ZERO gate-blocking (P0/P1).**
- Non-weakening CONFIRMED: ancestor-inclusion == member's gated work is reachable in the shipment
  head; divergent heads still rejected; residual post-gate edits covered by unchanged aggregate
  check #2. All git-exec/timeout/cancel/malformed/head-drift paths fail closed — confirmed.
- Advisory findings: ADV-1 (non-weakening = reachability not content-survival; documented),
  ADV-4 (empty-shipment-head fail-open = compensating control; documented), ADV-6 (`%v` cause,
  intentional; accept), ADV-7 (`ev.Enforced` guaranteed-true sub-condition; remediated `7ede042`),
  ADV-8 (test env-var order; accept). Deferred follow-ups for Stage: ADV-2 (ABA/there-and-back
  HEAD race — not a regression), ADV-3 (ambient-HEAD vs `--sha` anchor — pre-existing),
  ADV-5 (scope interaction with B85DAEE8/F3844849). Recorded in operational-closure §6.
- Findings doc: `docs/closure/2026-07-06-084-S-feature-pr-adversarial-review.md`.

## Copilot review

- Cycle 1 (substantive): 3 findings on `shipment_gate.go` — (a) `isAncestor` adopting 600s
  `TimeoutSeconds` → DoS lock-hold; (b) same in `headSHABounded`; (c) duplicated error message.
  All fixed in `c29b189` (`boundedHelperTimeout` 5s cap + dedup). Each thread: replied via REST
  `in_reply_to` referencing the fix commit, then resolved via GraphQL `resolveReviewThread`
  (`isResolved: true` confirmed).
- Cycle 2 (post docs push, fresh re-request on `3069195`): review submitted 06:02:58Z, **zero new
  threads, 0 unresolved.** Copilot loop complete.

## Runtime verification (real fix exercised)

- Scratch in-package driver exercised REAL `gateShipmentCompletion` + REAL
  `git merge-base --is-ancestor` subprocess against a real post-merge git repo. 5/5 PASS:
  (1) ancestor member head PASSES [the fix; strict-equality would reject], (2) divergent refused,
  (3) equality passes, (4) absent object fails closed, (5) cancelled context fails closed. Scratch
  driver deleted after capturing transcript (not committed).
- Transcript: `docs/closure/2026-07-06-084-S-feature-pr-runtime-verification.md`.
- Operational closure: `docs/closure/2026-07-06-084-S-feature-pr-operational-closure.md`.

## HALT

Presented PR #182 merge-ready and HALTED for operator P-014 merge approval. **Did NOT merge.**
Merge strategy guardrail (P-009): operator must merge with a merge commit (not squash/rebase).

## Resume (post-approval, Step 6 — only after operator confirms merge)

1. Merge Confirmation Gate: verify PR #182 `state == MERGED` + `merge-base --is-ancestor`
   {merge_sha} origin/main.
2. Post-merge closure branch `post-merge/084-ancestor-aware-staleness` off refreshed main.
3. Pre-archive reconcile (mode:pre, expected done) → `shipment ship 084-S <merge_sha>` →
   verify archive integrity (P-007) → post-archive reconcile → commit `.backlogit/`.
4. operational-closure mode=post-merge; knowledge graduation; compact-context target:all; sync.
5. Closure PR (chore: post-merge closure for 084-F) → await operator approval.

## Guardrails honored

Path-scoped `git add` only (never `-A`; ~198 pre-existing CRLF-noise files under `internal/core/`
left untouched). LF-normalized every created/edited file. Did NOT commit operator WIP
(`hooks_queue.jsonl`, `memories.json`, agent files, `.gitignore`, `start.ps1`, cli-reference CRLF).
Did NOT touch scope-excluded stash items. Follow-ups documented in operational-closure §6 (not
stashed — Ship role boundary lists stash operations as Stage's domain).
