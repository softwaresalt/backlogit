---
chunk_strategy: h1-h2-h3
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-17-baseline-convergence-deliberation.md
title: "Repository Baseline Convergence Release Unit"
description: "Deliberation grouping two deferred-scope-expansion stash entries into one baseline-convergence covering feature that restores repository-wide test, lint, and format green."
topic: "Baseline convergence: CRLF golden-fixture failure + repository-wide lint/gofmt drift"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-09-17-baseline-convergence-plan.md"
tags:
  - "baseline-convergence"
  - "line-endings"
  - "errcheck"
  - "staticcheck"
  - "gofmt"
  - "deferred-scope-expansion"
---

## Problem Frame

Shipment `149-S` was governed-handback to `queued` after wave 1 (`168.001-T`
done). It cannot resume because the mandatory wave-convergence gates fail on
**inherited repository baseline debt** that is outside `149-S`'s authorized
change surface (`docs/memory/2026-09-17/149-s-wave-1-convergence-hard-stop.md`):

* `go test ./...` fails in
  `internal/faultline.TestU4aBehaviorCanonicalByteStable` — the golden fixture is
  read with CRLF while the canonical JSON marshaler emits LF.
* `golangci-lint run` reports 56 inherited findings: **50 `errcheck` + 6
  `staticcheck`**, none in `149-S`'s changed surface.
* `gofmt -l .` reports broad repository files consistent with a Windows-checkout
  line-ending baseline.

This deliberation groups the two active stash entries that own this debt into a
single **baseline-convergence covering feature** whose completion restores
`go test ./...`, `golangci-lint run`, and `gofmt -l .` to green and thereby
unblocks `149-S`.

### Grouped stash entries (both carry the `DEFERRED SCOPE EXPANSION` marker)

| Stash ID | Kind | Summary | requires-deliberation |
|---|---|---|---|
| `92F79833` | bug | `TestU4aBehaviorCanonicalByteStable` CRLF/LF golden-byte mismatch in `internal/faultline` | true |
| `4DB1DFF1` | tech-debt | 50 errcheck + 6 staticcheck + gofmt drift in non-faultline repository files | false (marker forces deliberation) |

Per the Step 1 precedence rule, the `DEFERRED SCOPE EXPANSION` marker forces the
`deliberate` route for BOTH entries regardless of shape/size/priority. `4DB1DFF1`
already owns the format-drift work; **no duplicate format-drift entry is created**.

## Deferred-Scope-Expansion Triage Record (P-021 C5/C6)

### (A) Duplicate detection — UNCONDITIONAL — CLEAN for both

Scanned all 60 active stash entries for duplicates of each expansion.

* `92F79833`: no other entry references CRLF / `U4aBehaviorCanonicalByteStable` /
  canonical-byte / `evidence_conformance`. **CLEAN scan.**
* `4DB1DFF1`: the only keyword hit (`D9E4EC6C`) is a false positive matching
  "ErrCheckpoint" (a checkpoint dry-run error-discrimination concern), unrelated
  to repository-wide lint remediation. **CLEAN scan.**

No duplicate merge or archival required.

### (B) Late-identifier reconciliation — triggered by `N/A` source-ref fields

Both entries were captured with `PR=N/A` and `review-thread=N/A` on the pre-PR
threadless path. Reconciliation searched the Ship-owned residual-risk records
citing each entry ID:

* `92F79833` sources `feature=157-F shipment=139-S`. Residual-risk records:
  `docs/closure/139-S-157-F-post-merge-closure.md` (records `N/A (pre-PR)`),
  `docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md`,
  `docs/closure/2026-09-13-148-s-operational-closure.md` (Windows-checkout-only
  CRLF flake, does not reproduce on Linux CI). **No late PR/thread identifier
  surfaced.**
* `4DB1DFF1` sources `feature=156-F shipment=138-S`. Residual-risk records:
  `docs/closure/138-S-156-F-post-merge-closure.md`,
  `docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md`. **No late identifier
  surfaced.**

**Outcome: no-op reconciliation for both.** The recorded `N/A` STANDS as a
truthful terminal record (genuine pre-PR findings that never reached a PR). This
is NON-BLOCKING and is NOT a C3/C6 shortfall. No stash edits were made; both
entries remain reconciled-in-place under their existing IDs.

## Research Findings

Learnings retrieval (medium confidence) surfaced directly applicable prior art:

* `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` and
  `2026-07-21-omitempty-defeats-arrays...md` — **normalize-on-write, assert the
  EXACT canonical form (LF), never a semantically-equal variant.** A golden
  regenerated from the same marshaler can pass falsely; byte-pin the fixture.
* `docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md` —
  unchecked returns are a **correctness bug class**, not lint noise; prefer a
  shared checked helper to reduce the errcheck surface.
* `docs/compound/2026-07-26-markdownlint-frontmatter-title-double-count.md` —
  **baseline empirical per-rule/per-package counts before editing**; split into
  mechanical/config-scoped vs. genuine structural fixes.
* Knowledge gap flagged: no existing compound doc documents a `.gitattributes`
  `eol=lf` normalization playbook for a Go repo on a Windows checkout — this task
  should capture that as net-new institutional knowledge.

### Codebase root-cause evidence (read-only)

* `.gitattributes` contains only `* text=auto`. On a Windows checkout this
  converts LF→CRLF for every file Git classifies as text.
* `git check-attr -a internal/faultline/testdata/parity_v1.golden.json` →
  `text: auto`. Git stores the blob as LF (`git show HEAD:...` starts
  `{"applicability":...` with LF), but the working-tree file contains a CRLF
  pair. `a.Canonical()` emits LF, so `bytes.Equal(got, want)` fails.
* **Single shared root cause:** `* text=auto` + Windows checkout drives BOTH the
  golden-fixture CRLF mismatch (`92F79833`) AND the broad `gofmt -l .` `.go`
  drift (part of `4DB1DFF1`). The 50 errcheck + 6 staticcheck findings are
  genuine code issues and are independent of line endings.

## Options Evaluated

### Option A — Config-first line-ending normalization + package-isolated lint sweep (CHOSEN)

Fix the root cause once via explicit `.gitattributes` `eol=lf` rules and
`git add --renormalize .`, which simultaneously resolves the golden-fixture CRLF
and the bulk of the gofmt drift. Then remediate errcheck per-package and
staticcheck as isolated ~2h tasks, and finish with a repository convergence
verification task.

* **Pros:** attacks the shared root cause once; each task isolated by technical
  surface; matches learnings (baseline-then-fix, byte-pin fixtures, checked
  helpers); minimal blast radius per task; deterministic verification.
* **Cons:** renormalization touches many tracked files in one config task
  (large diff, but mechanical and reviewable via `git diff --stat`).

### Option B — Per-file spot fixes without `.gitattributes` change

Manually `gofmt -w` and hand-fix the golden fixture without pinning line endings.

* **Pros:** smaller individual diffs.
* **Cons:** does not fix the root cause — CRLF drift reappears on the next
  Windows checkout; violates the learnings' "normalize-on-write" guidance;
  fragile and non-durable. **Rejected.**

### Option C — One combined "fix all 56 + CRLF" task

* **Pros:** fewer backlog items.
* **Cons:** explicitly forbidden by the operator directive and the 2-hour rule;
  mixes config + code + test surfaces in one oversized unit. **Rejected.**

## Trade-off Comparison

| Criterion | Option A (chosen) | Option B | Option C |
|---|---|---|---|
| Root-cause durability | High | Low | Medium |
| 2-hour / width isolation | Pass | Pass | Fail |
| Blast radius per task | Low–medium | Low | High |
| Alignment with learnings | High | Low | Low |
| Re-drift risk on Windows | Eliminated | High | Eliminated |

## Decision

Adopt **Option A**. Synthesize one covering **feature** (the shipment
covering-item contract in `internal/core/shipment_covering.go` requires
`artifact_type == "feature"` with a dotless root ID, so a chore cannot be the
shipment root even though the work is maintenance-natured). Decompose into
technical-surface-isolated ~2h tasks:

1. `.gitattributes` `eol=lf` hardening + `git add --renormalize .` (config).
2. Residual genuine `gofmt -l .` remediation after renormalization (code-format).
3. Golden-fixture LF normalization + `TestU4aBehaviorCanonicalByteStable` green
   (test) — the concrete fix for `92F79833`.
4–10. errcheck remediation, one task per package surface
   (`internal/cli`, `internal/db`, `internal/telemetry`, `internal/stash`,
   `internal/events`, `tests/integration`, + residual sweep).
11. staticcheck remediation (6 findings).
12. Repository convergence verification (`go test ./...`,
   `golangci-lint run`, `gofmt -l .` all green) — unblocks `149-S`.

## Rejected Alternatives

* Option B (spot fixes) — non-durable, re-drifts on Windows.
* Option C (single mega-task) — violates 2-hour rule and operator directive.
* Chore as shipment covering root — rejected: shipment covering-item derivation
  and the manifest-binding digest require a feature root.

## Unresolved Questions

* Exact per-package errcheck distribution is unknown because Stage may not run
  linters (role boundary). Each errcheck task's first step is to enumerate its
  package's findings (baseline discipline). If a package's errcheck surface
  exceeds the 2-hour file bound (≥3 files), Ship splits it into a follow-up
  sub-task. Carried into `plan-harden`.

## Risks and Mitigations

* **Renormalization diff size** — mitigate by isolating it to one config task,
  reviewing via `git diff --stat`, and gating on `git ls-files --eol`.
* **Golden false-green** — assert exact LF bytes; do not regenerate the golden
  from the same marshaler and call it verified (byte-pin per learnings).
* **errcheck scope creep** — bound each task to one package; forbid unrelated
  refactors; `//nolint` only with a justified inline reason.
* **Unblocking coupling** — a `blocks` dependency makes `149-S` depend on the
  new baseline shipment so `149-S` is ineligible until baseline ships.
