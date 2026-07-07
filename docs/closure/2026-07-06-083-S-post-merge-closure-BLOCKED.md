---
chunk_strategy: h1-h2-h3
description: 'Post-merge closure of shipment 083-S is BLOCKED at the backlogit shipment-ship gate by a pre-existing member-evidence staleness bug. The feature PR #180 merged cleanly to main (2-parent merge commit ac41bb1). During post-merge closure, backlogit shipment ship 083-S was refused (exit 6) because validateMemberGateEvidence rejects member gate evidence whose recorded head_sha does not exactly equal the current shipment head. All nine 083 members recorded their feature-branch build commits (be1bf1e..c93080d) as head_sha; every one is a proven git ancestor of the merge commit ac41bb1, so the gated code IS included in the shipment head — this is FALSE staleness. Strict head equality is fundamentally incompatible with multi-commit post-merge shipment closure; 083-S is the first shipment built after head_sha recording became active and closed post-merge, so it is the first to expose the latent 082-F bug. No supported bypass exists (no hooks.yaml gate config, no --force/break-glass flag). Per operator genuine-wall STOP directive, closure was halted without forcing, log editing, or unplanned gate-semantics changes. Recommended fix stashed (885A7F65): make the staleness check ancestor-aware via git merge-base --is-ancestor. 083-S remains active and cleanly recoverable.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-083-S-post-merge-closure-BLOCKED.md
title: 083-S Gate-Broker Phase-2 Hardening — Post-Merge Closure BLOCKED (shipment-gate false staleness)
---

# Post-Merge Closure — 083-S (BLOCKED at shipment-ship gate)

**Date:** 2026-07-06
**Shipment:** 083-S — Gate-Broker Phase-2 Hardening
**Feature PR:** #180 — **MERGED** to `main`
**Merge commit:** `ac41bb1d2611fadd0fae6ccc49b3a8233468622d` (2 parents: `db8770d2` base + `453564e` feature tip)
**Closure branch:** `post-merge/083-S`
**Closure status:** **BLOCKED** — shipment archival cannot proceed; 083-S remains `active`

---

## Summary

The feature work shipped successfully: PR #180 is merged to `main` as a verified two-parent
merge commit, all quality gates and CI green, adversarial review PASS, and every Copilot thread
resolved. Post-merge closure (`backlogit shipment ship 083-S`) is **refused by the shipment gate
(exit 6)** due to a **pre-existing false-staleness bug** in member gate-evidence validation.

This is a **genuine wall**, not a transient error. Per the operator's explicit standing
guardrail — *"if you hit a genuine wall, STOP, checkpoint to docs/memory/, and report — do not
merge in a broken/uncertain state"* and *"leave items unsafe to implement without operator input
in the backlog rather than forcing them"* — closure was halted. No logs were edited, no gate
semantics were changed unilaterally, and the shipment was not force-archived.

---

## Precise root cause

`internal/core/shipment_gate.go` → `validateMemberGateEvidence` (≈lines 123–159):

```
shipmentHead := ws.headSHA(ctx)        // = git rev-parse HEAD = ac41bb1 (the merge commit)
for each member:
    h := latestGatePassEvidence(member).HeadSHA
    if h != "" && h != shipmentHead {  // <-- strict EQUALITY
        reject: "member <id> gate evidence is stale (recorded at a prior head)"
    }
```

Every one of the nine 083 members recorded a **non-empty** `head_sha` equal to the
feature-branch commit at which its pre-completion gate passed:

| Member        | Recorded head_sha | Ancestor of `ac41bb1`? |
|---------------|-------------------|------------------------|
| 083.001-T     | `be1bf1e`         | ✅ yes |
| 083.002-T     | `9cc241c`         | ✅ yes |
| 083.003-T     | `bcc5fba`         | ✅ yes |
| 083.004-T     | `6ed1fd3`         | ✅ yes |
| 083.005-T     | `c93080d`         | ✅ yes |
| 083.005.001-ST| `ffb67c8`         | ✅ yes |
| 083.005.002-ST| `e375956`         | ✅ yes |
| 083.005.003-ST| `5d2bc31`         | ✅ yes |
| 083.005.004-ST| `c93080d`         | ✅ yes |

`git merge-base --is-ancestor <head_sha> ac41bb1` returns 0 (true) for **all nine**. The gated
code is provably contained in the shipment head — so the "staleness" rejection is a **false
positive**. Strict head *equality* can never hold for a multi-commit shipment that is closed
**after** a merge commit is created, because the merge commit's SHA differs from every member's
build commit by construction.

### Why 082-S closed but 083-S cannot

Member `head_sha` recording (`internal/core/gate_transition.go` — `delta["head_sha"] =
outcome.HeadSHA`, guarded by `if outcome.HeadSHA != ""`) is **pre-existing at the 083 base
commit `be8d93f`** — it was **not** introduced by 083 work. 082-S members recorded an **empty**
`head_sha`, and the staleness check is skipped when `h == ""`. 083-S is simply the **first
shipment built after head_sha population became active AND closed post-merge**, so it is the
first to trip the latent bug. This is a defect in the 082-F gate broker, exposed (not caused)
by 083-S.

---

## Why no safe closure path exists from Ship's boundary

- **No gate config to relax:** no `hooks.yaml` / gate configuration exists under `.backlogit/`.
- **No break-glass flag:** `backlogit shipment ship` accepts only `--sha`, `--message`,
  `--author` — no `--force` / bypass. The gate is `Enforced`.
- **No evidence-refresh path for archived members:** members were moved to `.backlogit/archive/`
  with `done` status during the build; there is no supported command to re-run the
  pre-completion gate at the merge head and re-stamp their evidence.
- **Forcing would require** hand-editing member `head_sha` values in `.backlogit/logs/*.jsonl`
  (audit-log tampering) or an unplanned, unreviewed gate-semantics change compiled into the
  shipping binary — both explicitly out of bounds under the operator's STOP-and-don't-force
  directive and Ship's role boundary (no unilateral scope/semantics changes).

The refused `shipment ship` was verified to be a **clean no-op**: member-evidence validation
returns before any state mutation or evidence append. 083-S is still `active`, its log contains
only `shipment_created` + `shipment_status_changed`, and no blocked event was written. State is
fully recoverable.

---

## Recommended fix (stashed for Stage — requires deliberation)

**Stash `885A7F65` (kind: bug):** Make the member-evidence staleness check **ancestor-aware**.
In `validateMemberGateEvidence` (shipment_gate.go ≈152–156), when `h != "" && h != shipmentHead`,
accept the evidence if `git merge-base --is-ancestor h shipmentHead` succeeds (the gated code is
included in the shipment head); only reject genuinely **divergent** heads (not ancestors of the
shipment head). This preserves the gate's intent — "prove members passed on code that is actually
being shipped" — while being compatible with post-merge multi-commit closure.

This is a **gate-semantics change**, unplanned and outside the reviewed 083 plan. It affects the
shipped 082-F gate broker and warrants a Stage deliberation + plan-review before implementation,
so it is left in the stash rather than force-fixed inside this Ship session.

### Related follow-ups also stashed
- **`B85DAEE8` (bug):** empty `head_sha` bypasses the staleness comparison entirely — evidence
  authored without a head SHA can never be flagged stale (adversarial reviewer B, advisory).
- **`F3844849` (task):** unify malformed-JSONL-line handling between `parseItemLogFile`
  (errors) and `events.ReadAllEvents` (skips) (adversarial reviewer A P3, advisory).

---

## Feature-PR delivery record (for completeness)

- **Items:** all 9 executable items (F4/F1/F5/F7 + Q3.0–Q3.3) implemented TDD red→green, merged.
- **Quality gates:** `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` — all green.
- **Adversarial review (pre-push):** 3 models; 2 HIGH-confidence P1 gate-blockers found and
  remediated before push (doctor trusting stale projection over logs; F5 exit code not reaching
  the CLI). See `docs/closure/2026-07-06-083-S-feature-pr-adversarial-review.md`.
- **Copilot:** 3 threads across 4 review rounds, each replied-to + resolved via graphql; final
  two rounds returned zero new comments / zero unresolved threads.
- **Runtime verification:** Q3 projection confirmed (`idx_gate_evidence_status` present,
  9 `passed` rows, idempotent rebuild across two `backlogit sync` runs; logs authoritative).
- **§1.9 gate:** PASS — see `docs/closure/2026-07-06-083-S-feature-pr-operational-closure.md`.

---

## Operator decision required

Closure of 083-S (and therefore the closure PR the operator requested) **cannot proceed** until
the shipment-gate false-staleness bug is resolved. Recommended sequence:

1. Route stash `885A7F65` through **Stage** (deliberate → plan → review) for the ancestor-aware
   staleness fix (small, low-risk, high-value; unblocks all future post-merge shipment closures).
2. Once the fix ships, re-run `backlogit shipment ship 083-S --sha ac41bb1…` on a
   `post-merge/083-S` branch; run `shipment-reconcile` post-gate; then complete
   compound-refresh / compact-context / final memory and open the closure PR.

Alternatively, if the operator **explicitly approves** the ancestor-aware change as an in-scope
bug fix now, Ship can implement it under that approval, rebuild, re-run `shipment ship 083-S`,
and produce the closure PR in a follow-up session.

**Ship will not force closure without that decision.**
