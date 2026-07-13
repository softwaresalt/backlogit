---
description: "Ship session memory for shipment 091-S — spike findings-artifact docline reconciliation: build, review, PR #231, merge ec2b859, and post-merge closure."
doc_type: memory
docline:
  ms.date: 2026-07-13T00:00:00Z
  ms.topic: memory
source: docs/memory/2026-07-13/091-S-spike-docline-ship-memory.md
title: "091-S spike findings-artifact docline reconciliation — Ship session memory"
---

## Outcome

`ship next` executed queued shipment `091-S` end-to-end and closed it. Merged via
**PR #231**, merge commit **`ec2b859`**. Shipment `091-S` shipped/archived; members
`102-F` and `102.001-T` archived recording `ec2b859`.

## Task IDs completed

* `102.001-T` (task) — reconcile spike findings-artifact frontmatter example to
  docline base schema. Implementation commit `fd0a30b`.
* `102-F` (covering feature) — done/archived.
* `091-S` (shipment) — shipped (`ship_shipment --sha ec2b859`).

## Files modified

**Implementation (feat branch `feat/091-S-spike-docline-reconciliation`, commit `fd0a30b`):**

* `plugin/skills/spike/SKILL.md` — Phase 5 example replaced with docline-conformant block.
* `.github/skills/spike/SKILL.md` — identical replacement.

**Closure (branch `chore/closure-091-S`):**

* `.backlogit/archive/091-S.md`, `102-F.md`, `102.001-T.md` — shipped/archived, merge SHA.
* `.backlogit/reconcile/091-S-pre-20260713-153100.md`, `091-S-post-20260713-153145.md`.
* `docs/memory/2026-07-13/spike-findings-docline-stage-memory.md` — corrected the
  overstated "plan-review (gate PASS)" wording (honest provenance).
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` — reinforcement (091-S).
* `docs/compound/2026-07-13-copilot-review-loop-convergence.md` — new learning.
* `docs/closure/2026-07-13-091-S-spike-docline-closure.md`, `...-091-S-compound-refresh.md`.

## Decisions

* **4-space docline indentation** in the reconciled example (matches the repo
  gold-standard spike artifacts), overriding the plan's 2-space rendering per the
  Ship instruction and verified against the two existing conformant artifacts.
* **Two commits on the feature branch**: `docs(harness):` for the implementation,
  `chore(harness):` for the `.backlogit/` claim/completion lifecycle — keeping the
  implementation diff cleanly separable from expected lifecycle mutations.
* **No formal plan-review gate** was ever run (Stage did an inline self-assessment
  only). Proceeded on LOW blast radius; Ship's own `review` gate ran normally.
  Corrected the Stage memory doc that had implied a satisfied gate.
* **Closure via a dedicated PR** (`chore/closure-091-S` → PR) because direct pushes
  to `main` are ruleset-blocked (PR required + required status checks + Copilot
  review-on-push with required thread resolution).

## Failed approaches / gotchas

* `gh pr edit 231 --add-reviewer "copilot"` returns `'' not found` — the CLI does not
  resolve the Copilot bot by that name. Not needed: the `main` ruleset auto-triggers
  `copilot_code_review` on every push, so the review appears without an explicit request.
* `ship_shipment` **overwrites** members' `commit` field with the merge SHA
  (`fd0a30b` → `ec2b859`). Both are recorded in the closure artifact for traceability.
* Backlogit **MCP tools resolve the installed-plugin workspace**, not the repo root —
  continue using the repo CLI (`.\backlogit.exe ... --cwd .`) for repo backlog work
  (carried from the Stage memory).

## Verification

* Docline gate: 0 findings (scratch artifact + full repo). `make verify-plugin`: pass.
  Go gates: pass (no Go files changed). CI on PR #231: all required checks green.
* GI/GR reconcile: pre PROCEED, post PROCEED (0 archive deletions).

## Follow-ups (open)

* Stash `7F0A6E89` (low) — upstream `spike/SKILL.md.tmpl` in external autoharness repo.
* Recommended task — normalize backlogit CLI `created_at`/`updated_at` to UTC `Z`.
* P3 (pre-existing) — `.github/skills/spike/SKILL.md` own frontmatter lacks `name:` key.
* Retained scratch file `docs/decisions/2026-07-13-scratch-spike.md` — untracked, awaiting
  a future operator deletion decision (Principle VII).

## Next steps

Closure PR from `chore/closure-091-S` is prepared; awaiting operator merge approval
(P-014). No further shipment work pending in this session.
