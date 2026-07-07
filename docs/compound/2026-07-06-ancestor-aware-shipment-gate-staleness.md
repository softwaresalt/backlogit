---
chunk_strategy: h1-h2-h3
description: 'A security-preserving pattern graduated from 084-S — when a completion gate compares a recorded member head to a current shipment head, strict SHA EQUALITY produces false-staleness rejections after a normal merge (the member head becomes an ancestor of the merge commit, never equal to it). Replace equality with an ancestor-or-equal test via `git merge-base --is-ancestor`, but ONLY for non-empty heads, and make the check fail closed on every timeout/cancel/exec-error/non-{0,1} exit. Ancestor-inclusion is a REACHABILITY guarantee (the gated work is in the shipment lineage), not a content-survival guarantee — the unchanged aggregate diff check backstops residual post-gate edits.'
doc_type: learning
docline:
    date: 2026-07-06T00:00:00Z
    severity: high
    tags:
        - security
        - gate
        - git
        - merge-base
        - fail-closed
        - staleness
        - ancestry
        - core
        - windows
schema_version: "1.0"
source: docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md
title: 'Ancestor-aware (not strict-equality) member-evidence staleness, with fail-closed git merge-base exit-code handling (084-S)'
---

# Ancestor-Aware Member-Evidence Staleness

Graduated from shipment 084-S (feature 084-F, task 084.001-T; PR #182, merge
`f49ce3c37b460afce81591ca6e354b8de3a14a17`). The fix bootstrapped its own
closure — see the bootstrapping note below. Cleared a 3-model pre-push
adversarial review (0 P0/P1) and one Copilot cycle.

## Rule — Compare lineage, not identity: a member head that is an ancestor of the shipment head is NOT stale

### Problem

`validateMemberGateEvidence` (`internal/core/shipment_gate.go`) originally judged a
member's recorded `head_sha` stale with strict inequality:
`h != "" && h != shipmentHead -> "stale"`. That is correct only while nothing
moves. The moment a feature branch is merged with a **merge commit**, the member's
recorded head (a feature-branch commit) becomes the **second parent's ancestor** of
the merge commit — it is now reachable from, but never equal to, the shipment head.
Strict equality then rejects the shipment's own members as "stale", blocking
closure of exactly the multi-commit shipments the gate exists to protect. This is a
self-inflicted denial of closure, not a security property.

### Fix

Keep the equality fast-path (identical head → accept), then for a **non-empty**,
`isGitObjectName`-validated member head, decide by lineage:

```
git merge-base --is-ancestor <member_head> <shipment_head>
  exit 0 -> member head is an ancestor or equal  -> accept (NOT stale)
  exit 1 -> definitively not an ancestor         -> reject (genuinely divergent)
  other  -> unverifiable lineage                 -> FAIL CLOSED (error, never accept)
```

Scope discipline (critical): ancestor-awareness applies to the **non-empty member
head** path only. Do not touch the empty-member-head bypass, the empty-shipment-head
fail-open, or the malformed-JSONL path — those are separate, separately-reasoned
seams.

### Why this does not weaken the guard

- **Reachability, not survival.** `--is-ancestor` proves the member's gated commit
  is *in the shipment's history*. It does not prove the gated lines survived later
  edits. That is acceptable because a **second, unchanged aggregate check** (the
  shipment-level diff/autoharness gate over the full release scope) backstops
  residual post-gate edits. Ancestor-inclusion + aggregate check together are
  strictly stronger than equality-alone was for the real merge case, and no weaker
  for the divergent case (exit 1 still rejects).
- **Divergent heads still rejected.** A head on an abandoned/divergent branch is
  not an ancestor → exit 1 → reject.
- **No TOCTOU widening.** The whole evaluation is bracketed by a head-drift guard:
  the shipment head is re-resolved as the *last read before* appending passing
  evidence; if it moved, fail closed.

## The fail-closed git exit-code discipline (reusable)

`git merge-base --is-ancestor` has a **three-way** result, and only two of the three
are a definite answer:

- Treat exit 0 as the pass and exit 1 as the *only* legitimate "no". Every other
  exit (128 bad-object/shallow boundary, git-missing, any non-ExitError) is
  **unverifiable** and must return an error (fail closed) — never fall through to a
  pass.
- **Windows gotcha (load-bearing):** a context-killed child reports a
  platform-dependent code — commonly **exit 1 on Windows**, -1 on POSIX. So you MUST
  check `runCtx.Err() != nil` (covers BOTH `DeadlineExceeded` and `Canceled`)
  **before** inspecting the ExitError code, or a timeout will be silently misread as
  the exit-1 "not-an-ancestor" answer — a fail-*open* on the exact path you meant to
  harden.
- Preserve the exec trust boundary: argv-array `exec.CommandContext` (no shell),
  `cmd.Dir = ws.RootPath`, `cmd.Env = gate.MinimalEnv()`, and validate both operands
  with `isGitObjectName` (full 40-hex SHA-1 / 64-hex SHA-256; rejects abbreviations,
  refs, and leading-dash argument injection) before handing them to git.

## Bootstrapping note — the fix closed its own shipment

084-S's own members recorded feature-branch heads that, post-merge, are ancestors of
the merge commit `f49ce3c`. Closing 084-S therefore *required* the ancestor-aware
binary: closure was run with `backlogit.exe` rebuilt from merged `main`, and
`shipment ship 084-S` passed the completion gate (exit 0, 6 items archived) where the
old strict-equality binary would have refused. If a self-closing security fix can
close its own shipment only with the new binary, that is the strongest possible
end-to-end proof the fix is real and complete.

## Related

- `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — the companion DoS
  lesson: even a *bounded* lineage/head helper must impose its own hard timeout cap.
- `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — bound the
  first lock-holding external call (082-S).
- `docs/closure/2026-07-06-084-S-feature-pr-adversarial-review.md` — non-weakening
  and fail-closed confirmation (3 models, 0 P0/P1).
- `docs/closure/2026-07-06-084-S-feature-pr-runtime-verification.md` — 5-scenario
  real-subprocess proof (ancestor pass, divergent refuse, equality pass, fail-closed).
