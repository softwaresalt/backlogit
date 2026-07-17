---
description: "Ship post-merge closure for shipment 094-S — formal-gate architecture spike (feature 105-F, task 105.001-T). PR #248 merged 98b0522; spike deliverable is a PIVOT/medium-confidence findings artifact with a Q1–Q7 trust/atomicity contract sketch and F1–F6 bounded follow-ups; 8 Copilot review-fix cycles resolved; shipment shipped/archived; merge SHA backfilled via direct frontmatter edit."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-17T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-17-094-S-ship-closure.md
title: "094-S formal-gate architecture spike — Ship closure"
---

## Outcome

`ship next` executed queued shipment **094-S** (formal-gate architecture spike)
end-to-end and closed it. Merged via **PR #248**, merge commit **`98b0522`** (true
merge commit — parents `e32c4a5` base + `c02ccc3` PR head, P-009 satisfied).
Shipment `094-S` shipped/archived; feature `105-F` and task `105.001-T` archived.

The spike is **read-only and time-boxed**; its deliverable is a findings artifact,
not code. No implementation backlog items were created (charter non-goal).

## What the spike delivered

`docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` — a
**PIVOT / medium-confidence** conclusion plus a Q1–Q7 trust/atomicity contract
sketch and F1–F6 bounded follow-ups.

**Conclusion (PIVOT):** a formal PASS-only gate should be built **on the existing
gate-evidence log substrate** (082-F/083-F) — logs-as-source-of-truth,
append-before-commit ordering, `evidence_required` fail-closed refusal, a single
shared evidence predicate, argv-array/`MinimalEnv` broker, POSIX atomic file
replacement — **not** on the plan-digest design that PR #239 collapsed under.

**Five foundational gaps** (the Q-series) must be closed first:

| Q | Gap |
|---|---|
| Q1 | Trust is structural, not cryptographic — needs an authenticity proof anchored outside the mutable log + explicit anti-replay state. |
| Q2 | No cryptographic manifest↔evidence binding; the binding digest must be **covered by** Q1's proof (non-circular), not a standalone hash. |
| Q3 | Hash a single canonical serialization, never file-on-disk bytes (CRLF/frontmatter drift). |
| Q4 | Completion must read through one authoritative status taxonomy with context-specific predicates, not a single boolean. |
| Q5 | `dep_type` lives only in the disposable index; promote typed dependency objects into markdown so semantics survive rehydration. |
| Q6 | No all-or-nothing guarantee across multi-store mutation; even single-file replacement is non-atomic on the Windows remove-then-rename fallback. |
| Q7 | Governed ops need one shared core function with CLI/MCP behavioral parity. |

The **replacement-contract direction** adds a dedicated **formal-admission
predicate** distinct from the shared fail-open `Latest` predicate: it must require
an authenticated, non-forced **real** PASS, treat any later block/requeue as
invalidating a prior pass, and — because the broker maps exit 0 to proceed even on
empty/non-JSON stdout (`internal/core/gate/decision.go:56-60`) with a report
carrying only `repeated_failure` (`internal/core/gate/types.go:45-49`) —
additionally require a **schema-validated formal report with persona evidence**
bound into the authenticity proof.

## Members completed

| ID | Type | Work |
|---|---|---|
| `105.001-T` | task | Ran the formal-gate trust/atomicity architecture spike (2h max). |
| `105-F` | feature | Covering spike feature — done/archived. |
| `094-S` | shipment | Shipped (`shipment ship 094-S`), queue→archive. |

## Files changed (PR #248, merged 98b0522)

* `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` — **NEW**
  the spike deliverable (PIVOT conclusion + Q1–Q7 contract sketch + F1–F6
  follow-ups + residual questions + evidence inspected).
* `docs/decisions/2026-07-14-formal-gate-architecture-spike.md` — charter status
  update.
* `.backlogit/hooks_queue.jsonl` — audit events for the spike lifecycle; seq
  1445/1446 corrected from `from:"active"` to `from:"queued"` (see below).
* `.backlogit/archive/105-F.md`, `.backlogit/archive/105.001-T.md` — moved
  queue→archive on merge.

## Review findings addressed

Eight Copilot review-fix cycles, every thread verified against code, fixed,
replied, and resolved via GraphQL. Thread trend: **5 → 6 → 6 → 4 → 1 → 2 → 1 → 1
→ 0**. The operator authorized continuing past the standard 3-cycle cap because a
spike deliverable that seeds downstream deliberation and implementation must be
bullet-proof. Representative later-cycle fixes:

* **Q1/Q4 predicate (cycle 5):** added a dedicated formal-admission predicate
  distinct from the fail-open `Latest` predicate.
* **Q2 binding (cycle 6):** reworded to be non-circular — the binding digest must
  be covered by Q1's authenticity proof, with the proof stored outside the signed
  payload, closing the manifest-substitution path.
* **Q6 atomicity (cycle 6):** reframed the Windows `os.Remove`-then-rename fallback
  as a **non-atomic** crash window, folded into Q6's rollback contract.
* **Formal-admission semantic evidence (cycle 7):** verified against
  `decision.go:56-60` / `types.go:45-49` — a non-forced PASS proves only that the
  broker ran; require a schema-validated persona-evidence report bound into the
  authenticity proof.
* **PR-summary consistency (cycle 8):** qualified the PR description's atomic-write
  claim as POSIX-only, matching the Q6 finding.

**hooks_queue.jsonl correction:** seq 1445 (`105.001-T`) and 1446 (`105-F`)
recorded `from:"active"`, but `origin/main` had both items `queued` and the head
archive shows `done` with no `active` ever committed — an impossible history.
Corrected to `from:"queued"` to match the committed markdown source of truth. Root
cause is a product-behavior gap: shipment claim mutates member status queued→active
on disk without emitting a per-item status hook event, and that transient state was
never committed. Noted as a follow-up, not fabricated as a synthetic event.

## Gate evidence

* CI on merged HEAD `c02ccc3`: **4/4 green** (`test`, CLI Reference Drift, Detect
  code changes, Docline frontmatter gate).
* §1.9 pre-merge readiness: Copilot review fresh on HEAD (`c02ccc3`, submitted
  21:37:46Z), **0 unresolved Copilot threads**, 0 human/other threads,
  `reviewDecision: null` (no branch-protection block).
* Docs lint PASSED on the findings doc every cycle (`docs lint --path`,
  `valid: true, violation_count: 0`).
* No Go code changed (docs + backlog artifacts only), so no Go build/test/lint
  regression surface.
* **P-009:** true merge commit `98b0522` (two parents). **P-014:** operator
  approved merge explicitly.

## Post-merge closure

* `shipment ship 094-S` → `shipped`; archived `094-S`, `105-F`, `105.001-T`.
* **Release-SHA traceability:** the initial `shipment ship 094-S` ran without
  `--sha`, so the merge SHA was not attached at ship time. Re-running `ship --sha`
  was not possible (the ship path requires `status: active` and the shipment was
  already `shipped`), so the merge commit
  `98b05221ba3dd8d6f862a2f14c2db9b02da86e4c` was backfilled onto every archived
  scope item (`094-S`, `105-F`, `105.001-T`) via a **direct, body-preserving
  frontmatter edit**. This deliberately avoids `backlogit update --commit`, which
  round-trips archived records through the generic artifact model/writer and
  silently drops the archive-only `archived_from`/`archived_status` provenance (the
  094-S session compound learning). Verified post-edit: `commit` indexed on all
  three items and `archived_from`/`archived_status` preserved.
* Future ships should attach the SHA at ship time via
  `shipment ship <id> --sha <merge-sha> --message ... --author ...` in one step.
* Shipment-reconcile GI/GR (pre + post): all manifest items confirmed archived;
  `094-S.md` moved queue→archive.
* Branch `spike/094-S-formal-gate` deleted (local + remote) after merge.
* Post-merge backlog state committed and shipped via closure branch
  `chore/094-S-post-merge-closure` (direct push to `main` blocked by branch
  protection).

## Follow-ups (Stage to harvest; not created here)

F-series bounded (~2h) units recommended for a follow-up implementation plan:

* **F1 — Evidence authenticity primitive (Q1, Q2):** externally anchored proof +
  anti-replay + non-circular manifest↔evidence binding digest. *Needs a
  micro-decision → medium confidence.*
* **F2 — Canonical serialization + hash (Q3):** one canonicalizer reused by
  evidence and manifest hashing.
* **F3 — Authoritative status taxonomy (Q4):** one taxonomy with named
  context-specific predicates.
* **F4 — Durable dependency type (Q5):** typed dependency objects in frontmatter.
* **F5 — Journaled multi-mutation wrapper (Q6):** all-or-nothing or
  idempotent-replay envelope for governed create+link mutations. *Non-trivial →
  medium confidence.*
* **F6 — Governed-op parity hardening (Q7):** one commit-association operation
  updating frontmatter scalar + `commit_links` + JSONL, routed by both CLI and MCP
  (relates to the `update --commit` provenance-drop product bug).

Recommended ordering: F2 and F3 first (cheap, unblock F1); F1 next; F4/F6 in
parallel; F5 last.

## Residual / unresolved (carried into the F-series)

* The exact authenticity mechanism is not resolved — deferred to F1. A keyless
  in-log hash-chain alone is not viable under the Q1 threat model.
* The journaled-mutation design (write-ahead journal vs idempotent replay) is not
  resolved — deferred to F5.
* Machine-waiver / ADVISORY admission stays out of scope (charter non-goal) and
  must not re-enter via the F-series.
