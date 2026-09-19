---
chunk_strategy: h1-h2-h3
description: "Operational new-session handoff for PR #448 (156-S baseline-convergence, 497 supported-platform finding union). The 156-S/175-F contract is FROZEN and must not be redesigned. Records exact frozen head, locked implementation facts, the single known unresolved CI-workflow bot-thread blocker, a strict surgical A-F decomposition with per-unit scope/acceptance/stop, and cost controls to prevent another full review/redesign loop."
doc_type: guide
docline:
  author: Stage
  date: 2026-09-19
  status: draft-for-transfer
ingested_at: "2026-09-19T00:00:00Z"
schema_version: "1.0"
source: docs/scratch/2026-09-19-pr448-surgical-decomposition-session-handoff.md
title: "PR #448 surgical decomposition session handoff — frozen 156-S contract, unresolved CI-workflow bot thread, strict A-F units"
---

> **Ephemeral operational handoff, not durable architecture.** This file lives
> under `docs/scratch/`, which is intentionally ephemeral per
> `.github/instructions/context-efficiency.instructions.md`. Scratch files may be
> archived or compacted once they age past the retention window, so this path is
> **not** durable provenance and may stop resolving. Its only job is to bootstrap
> the **next** working session cheaply. Durable state lives in the immutable Stage
> checkpoints, the committed backlog artifacts, and the PR itself.

## Purpose

Hand off PR #448 (`stage/baseline-convergence-main`, shipment 156-S baseline
convergence) to a fresh session with the minimum context needed to finish it
**without** re-running any full review or redesign loop. The planning/backlog
contract is **frozen**. The next session performs a small, bounded correction to
one CI workflow file, then normal PR lifecycle. Nothing else.

## Frozen state at handoff start

- **Frozen code/harness head (before this documentation commit):**
  `3b28b35ae3200a9824cf7b042f183df8b4cfb3d6`
- **Branch:** `stage/baseline-convergence-main`
- **PR #448:** open; **7/7 CI checks green**; **not merged**; no active shipment.
- **Shipment topology state:** `156-S` queued; `149-S` queued and **blocked by
  156-S**; `168.001-T` archived/**done** at commit `c976315f`.
- **Worktree:** clean at handoff start.

## Locked implementation facts (DO NOT re-derive or redesign)

- Supported-platform finding **union = 497** (489 Windows-primary = 459 errcheck
  + 30 staticcheck; 8 Linux-only errcheck additions; 4 Windows-only exclusions).
- **98 executable task members / 99 shipment members** (feature 175-F + 98 tasks).
- **184 dependency edges; 13 bounded waves.**
- Validation greens: **queue-backed sim 406/406**, **runtime tests 47/47**,
  **generic sim 367/367**.
- Canonical lifecycle-aware **task / baseline / terminal** PowerShell runners in
  use; **golangci-lint v2.13.2** pin everywhere.
- Latest Stage checkpoint: **`checkpoint-20260919-084516.json`** (resolved);
  checkpoint commit **`f4d1447c`**; bound content head **`6e1ca7d1`**.
- Bounded CI lifecycle fix already landed: **`3b28b35a`** (current head).

## Why the session was stopped

Repeated full-review and redesign loops drove excessive AI compute cost (AIC).
The 156-S / 175-F contract (finding union, task/member/edge/wave counts, DAG,
runner design, GOOS policy, claim-union design) is **frozen and correct**. The
next session **must not** reopen it. Remaining work is a single small CI-workflow
correction plus normal PR lifecycle.

## Known unresolved current blocker (the only one)

- **Bot review thread:** `PRRT_kwDORzozKM6kCLVu`
- **Discussion URL:** https://github.com/softwaresalt/backlogit/pull/448#discussion_r4054220876
- **Location:** `.github/workflows/ci.yml:122`
- **Defect:** sequential native `pwsh` verification commands run without an
  immediate `$LASTEXITCODE` check after each command, so a later command's
  success can mask an earlier command's failure (silent false-green in CI).
- **Fix shape (do not over-scope):** after each native verification command,
  check `$LASTEXITCODE` immediately and fail closed (non-zero exit) before the
  next command runs. Correction is confined to `.github/workflows/ci.yml`.

## Surgical decomposition — separate work units (strict scope)

Execute in order. Each unit has a single owner, strict inputs, explicit
acceptance, and a hard stop. **Aggregate all threads before fixing any.**

### Unit A — Read-only unresolved-thread census
- **Scope:** enumerate ALL unresolved PR #448 review threads, fully paginated.
- **Inputs:** PR #448; GitHub GraphQL `reviewThreads` (paginate to `hasNextPage:false`).
- **Acceptance:** a frozen, complete list of unresolved threads (id, path, line,
  author, body, isResolved) — including `PRRT_kwDORzozKM6kCLVu`.
- **Stop:** produce the census only. **No edits, no replies, no resolutions.**

### Unit B — One batched correction commit
- **Scope:** fix all VALID defects from the frozen census in a single commit.
- **Inputs:** the Unit A census; strict path allowlist (expected: only
  `.github/workflows/ci.yml`; extend the allowlist only if the census proves an
  additional valid defect, and record it).
- **Acceptance:** one targeted validation of the changed surface (e.g. workflow
  lint / the specific check), one CI run green; no design changes; no backlog or
  contract mutation.
- **Stop:** one correction cycle. If a fix would require redesigning the frozen
  contract, STOP and escalate — do not expand scope.

### Unit C — Metadata-only PR hygiene
- **Scope:** PR body rewrite, thread replies + resolutions, Copilot review gate.
- **Inputs:** Unit B commit SHA; census thread IDs.
- **Acceptance:** every valid thread replied to (citing the fixing commit) and
  resolved; Copilot gate satisfied; PR body current. **No repository file changes.**
- **Stop:** metadata only. No full review for metadata-only work.

### Unit D — Normal merge + safe local-main sync
- **Scope:** merge-commit merge of PR #448 (operator merge pre-authorized);
  then safe local `main` ff-only sync.
- **Inputs:** green CI; satisfied gates.
- **Acceptance:** MERGE_SUCCEEDED via **merge commit** (P-009); local `main`
  fast-forwarded to `origin/main`; unrelated local state preserved.
- **Stop:** **no admin fallback** (not authorized). If blocked, halt to operator.

### Unit E — Execute 156-S using the frozen contract
- **Scope:** Ship executes shipment 156-S against the frozen 497-union DAG.
- **Inputs:** merged `main`; frozen contract; canonical runners.
- **Acceptance:** waves scaffold/build/review per the frozen topology.
- **Stop:** **pause for immediate operator-only approval before U1's destructive
  worktree renormalization/refresh action.** Do not auto-approve.

### Unit F — Resume 149-S (separate session)
- **Scope:** once 156-S closes, Ship resumes 149-S.
- **Inputs:** 156-S closed; `168.001-T` **done** (preserve).
- **Acceptance:** 149-S proceeds with `168.001-T` completion preserved; no repeat
  of completed Wave 1 work.
- **Stop:** separate session; do not fold into 156-S execution.

## Cost controls (mandatory)

- One owner per unit; no reviewer fan-out unless a **new P0/P1** is demonstrated.
- Max **one** correction cycle per unit; defer P2/P3 to backlog.
- **No full review for metadata-only** work (Unit C).
- Stop immediately on scope expansion; **aggregate all threads before fixing any**.
- Reuse the frozen census; do not re-poll threads mid-fix.

## Startup commands / checks (Windows one-line; NOT executed by this doc commit)

These are for the next session to run at its start. They are documentation only
and were **not** executed as part of this documentation commit.

```text
git -C C:\Source\GitHub\backlogit rev-parse HEAD
git -C C:\Source\GitHub\backlogit status --porcelain
gh pr view 448 --repo softwaresalt/backlogit --json state,mergeStateStatus,statusCheckRollup
gh api graphql -f query='query{repository(owner:\"softwaresalt\",name:\"backlogit\"){pullRequest(number:448){reviewThreads(first:100){nodes{id isResolved path line} pageInfo{hasNextPage endCursor}}}}}'
go run ./cmd/backlogit get 156-S
go run ./cmd/backlogit get 149-S
go run ./cmd/backlogit get 168.001-T
```

## Do NOT (frozen-contract guardrails)

Do not re-open or re-litigate any of the following:

- 56 vs 489 vs 497 finding count (**497 is authoritative**).
- Task count / member count (**98 tasks / 99 members**).
- DAG shape, edge count, or wave count (**184 edges / 13 waves**).
- Runner design (canonical lifecycle-aware task/baseline/terminal runners).
- GOOS / supported-platform policy.
- Claim-union design.
- BDD retrofit into current shipments 156-S or 149-S (see the deferred product
  spec below — pilot on a NEW small shipment only).

## Related

- Product spec (deferred, no handoff):
  `docs/product-specs/2026-09-19-shipment-bdd-contracts-requirements.md`
