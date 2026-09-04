# Compacted memory: shipped units 068-S through 071-S (+ June stage checkpoints)

This compacted memory replaces 14 verbose checkpoints for already shipped and
closed release-unit work (068-S–071-S) plus the associated Stage session
checkpoints from the 2026-06-27 → 2026-06-30 window. Originals are archived flat
under `docs/archive/memory/` and listed in the Provenance section. The 072-S
Stage and Ship checkpoints are intentionally **preserved** (current unit, newest).
Continues the lineage compacted by `2026-06-28-shipped-units-060-067-compacted.md`.

## 068-S: shared frontmatter codec extraction

### Outcome

`068-S` (feature `068-F`, tasks `068.001-T`–`068.004-T`) shipped and archived.
Implementation PR **#148** merged as merge commit
`7450271a3334fc9c780ba757de6cf390a40edf3c` (P-009); post-merge closure on branch
`post-merge/068-codec-extraction`. Extracted the shared frontmatter codec into a
stdlib-only leaf package (`internal/mdfront`) with a type-alias seam and
golden-byte-equality tests, so `cmd/gen-docs` and the core share one encoder.

### Key decisions / learnings

* Deliberation Option B (relocate to a leaf package rather than duplicate the
  encoder). Source stash `8863C6C8`; deliberation
  `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md`.
* Durable learning: leaf-package extraction + type-alias compatibility + golden
  byte-equality is the safe pattern for relocating a serializer without churn.

Durable pointers: `docs/closure/2026-06-28-068-S-codec-extraction-closure.md`
(+ `-runtime-verification.md`, `-compound-refresh.md`).

## 069-S: docline + doctor robustness hardening

### Outcome

`069-S` (feature `069-F`, tasks `069.001-T`–`069.003-T`) shipped and archived.
PR **#152** merged as `1dd4e69a`; post-merge closure on branch
`post-merge/069-docline-doctor-hardening`. `doctor` dogfood clean (0 self-ref, 0
malformed `archived_from`); reconcile pre/post PROCEED. Task `069.003-T` delivered
full v1 `ValidateFields` schema (content_sha256 hex parity), commit `e6d5231f`.

### Key decisions / learnings

* Stage harvest disposition `9685B1AA` = option (a): clear malformed
  `archived_from` on 038-DL/039-DL.
* Follow-up stash `997574DD` carried forward.

Durable pointers:
`docs/closure/2026-06-28-069-S-docline-doctor-hardening-closure.md`
(+ `-runtime-verification.md`, `-compound-refresh.md`).

## 070-S: internal robustness cluster

### Outcome

`070-S` (feature `070-F`, tasks `070.001-T`–`070.003-T`) shipped and archived.
PR **#154** merged as merge commit `b4c317e` via operator-approved `--admin`
(required-review branch protection had no formal approving review — the same
admin-merge path used for 071-S and 072-S); P-009 preserved. Batched the
canonical-uniqueness scan in `CreateArtifact` (`CanonicalCache`, commit lineage
`e4fefd3`), added `warnOnDuplicateSourceIDs` (`60753ae`), and variadic
`RehydrateOption` + minLength parity (`1ab8f0d`).

### Key decisions / learnings

* Graduated the durable compound learning
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`
  — an exported short-circuit cache must treat its zero value (nil backing map) as
  "unseeded → re-scan", never as an authoritative empty set. (This is the learning
  that 072-S later reinforced at the doctor --target validation-precondition
  boundary.)

Durable pointers:
`docs/closure/2026-06-29-070-S-internal-robustness-cluster-closure.md`
(+ `-runtime-verification.md`, `-compound-refresh.md`).

## 071-S: deterministic-gates slice

### Outcome

`071-S` (feature `071-F`, tasks `071.001-T`–`071.009-T`) shipped and archived.
PR **#156** merged as merge commit
`531bd51fe6abf52b210fdf1268c12a23f8c24899`; post-merge closure on branch
`post-merge/071-deterministic-gates`. Delivered the doctor target-gate, task
locking, and size schema + mutation slice. Ran in CLI/file-backed mode
(`backlogit.exe` v1.3.0 fallback for MCP ops); branch created from a dirty `main`
with the operator's unrelated in-flux files carried as uncommitted — commits
scoped strictly to 071-S (the same isolation pattern reused for 072-S).

### Key decisions / learnings

* Versioned `doctor --target` exit-code contract established here
  (0 pass / 1 validation / 2 timeout / 3 scope|io / 4 busy) — later preserved
  unchanged by 072-S.
* Each task's implementing commit SHA associated via `backlogit update --commit`.
* PR #156 Copilot review raised follow-up **K** (nil-HeaderDef fail-open in
  `ValidateDoctorTargetResolved`), stashed `C16DBBEB` and later delivered as
  shipment **072-S** (PR #158, merge `d3f0fac`).

Durable pointers:
`docs/closure/2026-06-30-071-S-deterministic-gates-slice-closure.md`
(+ `-runtime-verification.md`).

## Stage session checkpoints (2026-06-27 → 2026-06-30)

* **Triage + grooming (2026-06-27)** — post-archival index 633 artifacts; selected
  068-S codec extraction (stash `8863C6C8`) as the shipped unit.
* **Plan-review PASS cycle 2 (2026-06-28)** — 068-S plan approved
  (`docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md`).
* **Stage session final (2026-06-28)** — 6 stash entries archived; feature 068-F +
  4 tasks harvested; shipment 068-S assembled (base merge `fd7b68f0`).
* **069-S harvest (2026-06-28)** — landed docline/doctor hardening backlog.
* **C55C5158 YAGNI determination (2026-06-28)** — durable high-water-mark ID
  counter: terminal **DO NOT BUILD / archived as YAGNI**; not genuinely additive
  over the shipped 066-S canonical pre-write detect + refuse
  (`FindingRootIDCollision` / `ErrIDCollision`).
* **Deterministic-gates slice stage (2026-06-30)** — stash `AE0838A9` → feature
  071-F + shipment 071-S via the full stash→backlog pipeline.

## Provenance

Verbose originals archived under `docs/archive/memory/`:

* `docs/archive/memory/2026-06-27-068-S-codec-extraction-ship.md`
* `docs/archive/memory/2026-06-28-068-S-codec-extraction-post-merge-closure.md`
* `docs/archive/memory/2026-06-28-069-S-post-merge-closure.md`
* `docs/archive/memory/2026-06-28-ship-069-S-docline-doctor-hardening.md`
* `docs/archive/memory/2026-06-28-stage-069-S.md`
* `docs/archive/memory/2026-06-29-ship-069S-pr-ready.md`
* `docs/archive/memory/20260627-223700-stage-triage-grooming-checkpoint.md`
* `docs/archive/memory/20260628-060000-stage-plan-review-pass-checkpoint.md`
* `docs/archive/memory/20260628-061500-stage-session-final.md`
* `docs/archive/memory/20260628-135500-stage-c55c5158-yagni-determination-final.md`
* `docs/archive/memory/2026-06-29-070-S-post-merge-closure.md`
* `docs/archive/memory/2026-06-29-ship-070-S-internal-robustness-cluster.md`
* `docs/archive/memory/2026-06-30-stage-deterministic-gates-slice.md`
* `docs/archive/memory/2026-06-30-ship-071-S-checkpoint.md`

Preserved (not compacted): `docs/archive/memory/2026-07-01-stage-072-S-doctor-nil-headerdef.md`,
`docs/archive/memory/2026-07-01-ship-072-S-checkpoint.md` (current unit 072-S, newest).
