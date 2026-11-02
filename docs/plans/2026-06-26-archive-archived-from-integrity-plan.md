---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to fix core.ArchiveItem self-referential archived_from, restore invertible unarchive, and repair ~130 legacy archive records via a doctor audit + --fix'
doc_type: plan
docline:
    date: 2026-06-26T00:00:00Z
    origin: docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md
    related_stash:
        - 53F22794
    stash_ids:
        - 53F22794
    status: reviewed
    tags:
        - archive
        - unarchive
        - data-integrity
        - data-migration
        - doctor
        - invertibility
ingested_at: "2026-06-26T20:15:36Z"
schema_version: "1.0"
source: docs/exec-plans/2026-06-26-archive-archived-from-integrity-plan.md
title: 'ArchiveItem archived_from Integrity: invertible unarchive + legacy record repair'
---

## Problem Frame

`core.ArchiveItem` (`internal/core/archive.go:181`) stamps
`fm["archived_from"] = workspaceRelativePath(ws.RootPath, currentPath)`
unconditionally. For a **pre-archived** item (already at its archive path because a
terminal status routed it to `.backlogit/archive/` at done-time, then re-stamped
during `shipment ship`), `currentPath == archivePath`, so `archived_from` becomes
the item's own archive path instead of the canonical restore location
(`.backlogit/queue/<id>.md`).

This (1) violates the ArchiveItem contract asserted by
`internal/core/archive_test.go:70`, and (2) breaks invertible unarchive:
`UnarchiveItem` (`archive.go:358-409`) reads `archived_from` as the restore target;
when it equals the archive path the lines 401-409 branch leaves the file in the
archive dir, so the item can never return to the queue. 130 legacy archived records
carry the self-referential value and must be repaired.

Code paths in scope:

* `internal/core/archive.go` — `ArchiveItem` (write site, line 181) and
  `UnarchiveItem` (restore site, lines 358-409).
* `internal/core/archive.go` — `queueRootDir` (457), `workspaceRelativePath` (464),
  and the artifact-search/registry routing in
  `internal/core/artifacts.go:522-568` (`artifactSearchDirs`) — the existing
  machinery for resolving where an artifact lives when active.
* `internal/core/doctor.go` — the 066 doctor audit infrastructure that the new
  `archived_from` invertibility audit + `--fix` repair extends.
* `internal/core/archive_test.go`, `internal/core/doctor_test.go` — regression nets.

## Requirements Trace

| # | Requirement (from deliberation 53F22794) | Decision | Unit |
|---|---|---|---|
| R1 | Resolve a record's canonical restore path as `.backlogit/<queue-root>/<basename>` purely from `QueueLayout.RootDir` (default `queue`); workspace-contained, `.backlogit/`-prefixed | R2 | U1 |
| R2 | `ArchiveItem` stamps the canonical restore path (not `currentPath`) for pre-archived items; normal queued→archive path unchanged | Decision 2, code fix #1 | U2 |
| R3 | Regression test mirroring `archive_test.go:70` for the pre-archived path | code fix #1 | U2 |
| R4 | `UnarchiveItem` restores pre-archived items to the queue, consistent with the new `archived_from`; 401-409 retained as defensive net | Decision 3 | U3 |
| R5 | Archive→unarchive round-trip test for the pre-archived case | Decision 3 | U3 |
| R6 | `doctor` audit detects self-referential `archived_from`; ignores canonical/fieldless/legitimate-subdir records; flags malformed `done` records | Option B | U4 |
| R7 | `doctor --fix` rewrites the 130 self-ref records to the resolved canonical path; idempotent; reviewable batches; leaves non-matching records untouched | Option B | U5 |
| R8 | CLI-only `--fix-archived-from` repair (operator-invoked) + MCP-safe read-only detection audit on `backlogit doctor` | Decision 1 | U6 |
| R9 | Operational closure runbook: run, verify zero self-ref remain, rollback | closure | U7 |

## Implementation Units

Each unit is TDD-first (write failing test, observe red, implement, green), single
domain, and within the 2-hour rule.

### U1 — Canonical restore-path resolver (code, test-first)

* **Changes**: Add an unexported helper
  `canonicalRestorePath(ws *Workspace, basename string) string` in
  `internal/core/archive.go` that returns the **repo-root-relative, `.backlogit/`-
  prefixed** POSIX restore path `.backlogit/<queueRootDir(ws)>/<basename>` (default
  `queue`). It is **pure over `ws.Config.QueueLayout`** (mirrors `queueRootDir` at
  archive.go:457) and does **not** consult the registry — the registry routes by
  *status* (terminal `done` → archive), which would re-introduce the self-reference.
  The output format MUST match `workspaceRelativePath(ws.RootPath, …)` (the form
  `archive_test.go:70` asserts as `.backlogit/queue/001-T.md`) so `UnarchiveItem`'s
  F-006 guard (`archive.go:368-373`) accepts it instead of rejecting a prefix-less
  `queue/...` as a `../` traversal. Reject/flag a `QueueLayout.RootDir` that is
  absolute or escapes `.backlogit`.
* **Files**: `internal/core/archive.go`, **new** `internal/core/archive_internal_test.go`
  (`package core`, so the unexported helper is callable — `archive_test.go` is
  `package core_test` and cannot reach it).
* **Tests (≤3 scenarios)**: (a) default → `.backlogit/queue/<basename>` (asserts the
  `.backlogit/` prefix); (b) configured `QueueLayout.RootDir` honored; (c) absolute
  or `..` `RootDir` rejected (containment).
* **Posture**: test-first.

### U2 — ArchiveItem pre-archived archived_from fix (code, test-first)

* **Changes**: In `ArchiveItem`, when
  `filepath.Clean(currentPath) == filepath.Clean(archivePath)` (pre-archived), set
  `archived_from` to `canonicalRestorePath(...)` (U1) instead of
  `workspaceRelativePath(ws.RootPath, currentPath)`. Leave the normal branch
  (`currentPath != archivePath`) byte-for-byte unchanged.
* **Files**: `internal/core/archive.go`, `internal/core/archive_test.go`.
* **Tests (≤3 scenarios)**: (a) existing `archive_test.go:70` normal path still
  asserts `.backlogit/queue/001-T.md` (unchanged); (b) **new** pre-archived
  fixture (item seeded already in archive dir, no queue copy) → `archived_from`
  equals `.backlogit/queue/<id>.md` (the `.backlogit/`-prefixed form), not the
  archive path, proving format-consistency with the normal branch.
* **Depends on**: U1.
* **Posture**: test-first (regression).

### U3 — UnarchiveItem read-time consistency + round-trip (code, test-first)

* **Changes**: Harden `UnarchiveItem` so its correctness does **not** depend on the
  U5 migration running first: when the persisted `archived_from` resolves to the
  record's **own archive path** (legacy self-ref), recompute the restore target via
  `canonicalRestorePath` (U1) instead of trusting the bad value. With a correct
  target, `originalPath != archivePath` and the rename-into-queue +
  remove-archive-copy path (388-409) restores to the queue. Retain the 401-409
  branch as a defensive net. **Status invariant**: the restored pre-archived item
  keeps its `archived_status` (060.003-T); add a re-archive-stability check so that
  re-archiving the restored item takes the normal branch and re-stamps
  `.backlogit/queue/<id>.md` (not a self-ref), preventing queue/archive oscillation.
* **Files**: `internal/core/archive.go`, `internal/core/archive_test.go`.
* **Tests (≤3 scenarios)**: (a) archive→unarchive round-trip for a **pre-archived**
  item: file ends in `.backlogit/queue/<id>.md`, removed from archive; (b) unarchive
  of a **legacy self-ref** record (archived_from = own archive path) still restores
  to the queue via the U1 recompute; (c) re-archive of the restored item is stable
  (normal branch, `.backlogit/queue/<id>.md`). Existing
  `TestUnarchiveItem_RestoresFromArchive` stays green.
* **Depends on**: U1, U2.
* **Posture**: test-first (regression).

### U4 — Doctor archived_from invertibility audit / detection (code, test-first)

* **Changes**: Add a new read-only audit to `internal/core/doctor.go` that scans
  archive records and emits a structured finding for each whose `archived_from`,
  resolved via `resolveWorkspacePath` + `filepath.Clean`, **equals the record's own
  archive path** (self-referential) — the identical comparison
  `ArchiveItem`/`UnarchiveItem` use at archive.go:200/366. This is a direct path
  comparison and needs **no** restore-path resolver. Records whose `archived_from`
  points elsewhere (the 258 canonical, and the legitimate `036-DL` whose value is
  not self-referential) and the 211 fieldless records produce no finding. Emit a
  distinct `malformed` finding for records whose `archived_from` is not a path (the
  2 `done` records). Detection only — no mutation; reuse the existing `DoctorFinding`
  model so the audit is MCP-safe (read-only).
* **Files**: `internal/core/doctor.go`, `internal/core/doctor_test.go`.
* **Tests (≤3 scenarios)**: (a) self-ref record → finding; (b) canonical / fieldless
  / `036-DL`-style non-self-ref records → no finding; (c) malformed `done` →
  distinct finding.
* **Depends on**: none beyond the doctor infra (detection is a path comparison; the
  U1 resolver is required only by the U5 repair).
* **Posture**: test-first.

### U5 — Doctor --fix repair of legacy self-ref records (code, test-first)

* **Changes**: Under an explicit repair flag, rewrite each self-referential record's
  `archived_from` to `canonicalRestorePath(ws, basename)` (U1) using the
  **`internal/docline` `Decode`/`Encode` codec** (byte-preserves the body;
  deterministic sorted-key frontmatter) so only the frontmatter block changes.
  **Safety preconditions**: refuse to run on a symlinked archive root or record
  (`Lstat` + realpath containment under `WorkspaceStorageRoot`); skip (do not
  rewrite) any record whose recomputed target cannot be proven workspace-contained.
  **Partial-failure semantics**: continue-on-error per record (mirror the
  `fix-orphans` pattern at doctor.go:211-221), `slog.Warn` each failure, and emit a
  per-record `FixAction` for every repaired record (not just an aggregate count) so
  the structured `doctor` report is the authoritative migration manifest. Idempotent:
  a second run is a byte-stable no-op. Leave non-matching records untouched.
  Malformed `done` records are **flagged only** in v1.
* **Files**: `internal/core/doctor.go`, `internal/core/doctor_test.go`.
* **Tests (≤3 scenarios)**: (a) self-ref record repaired to `.backlogit/queue/<id>.md`
  with a per-record FixAction; (b) re-run is a byte-stable no-op (idempotency);
  (c) canonical / `036-DL` / malformed / symlinked records left untouched.
* **Depends on**: U4, U1.
* **Posture**: migration-first (TDD).

### U6 — CLI surface for the --fix repair (code, test-first)

* **Changes**: Wire the audit into the `backlogit doctor` **CLI** following the
  existing `--fix-orphans` flag pattern (`DoctorOptions`): the read-only detection
  finding surfaces in default `doctor` output; the destructive repair is behind a
  new explicit, **CLI-only** `--fix-archived-from` flag and refuses to mutate
  without it. **The repair is NOT exposed on the `backlogit_doctor` MCP tool** —
  MCP params are model-settable, so an agent-triggerable bulk rewrite would bypass
  the Principle VII operator-approval boundary. The MCP tool may surface the
  read-only detection finding only.
* **Files**: `cmd/backlogit/` doctor command file + its test (single CLI domain; no
  MCP handler change for the repair path).
* **Tests (≤3 scenarios)**: (a) `doctor` (no flag) reports findings, mutates
  nothing; (b) `doctor --fix-archived-from` repairs and reports the per-record
  manifest; (c) the flag plumbs to the U5 core repair.
* **Depends on**: U5.
* **Posture**: test-first.

### U7 — Migration operational-closure runbook (docs, doc-only)

* **Changes**: Author `docs/closure/2026-06-26-archived-from-migration-closure.md`
  (docline-compliant) documenting: how to run the `doctor --fix` repair against the
  130 records, the pre/post census command (`Select-String '^archived_from:'` scan
  showing zero self-ref remain), batch/rollback strategy (git revert of the repair
  commit), and the operator sign-off checkpoint.
* **Files**: `docs/closure/2026-06-26-archived-from-migration-closure.md`.
* **Tests**: `backlogit docs lint` passes (docline gate).
* **Depends on**: U5.
* **Posture**: doc-only.

## Dependency Graph

```text
U1 ──► U2 ──► U3
 │            ▲
 └────────────┘   U3 also consumes the U1 resolver (read-time recompute)

U4 ──► U5 ──► U6
        ▲ └──► U7
        └── U5 also depends on U1 (repair target)
            U4 detection is a path comparison: depends only on the doctor infra
```

No cycles. **Code** order: U1 → U2 → U3; U4 → U5 → {U6, U7}; U1 also feeds U5.
**Rollout** order (operational, stronger than the code deps): **U2 must ship before
any U5 `--fix` runs** — otherwise `ArchiveItem` keeps minting self-ref records after
the repair; and **U6 must land before U7 sign-off** since the runbook documents the
final CLI command. U4 detection can be implemented in parallel with the U1→U2→U3
chain.

## Decisions and Rationale

* **Single queue-layout resolver, `.backlogit/`-prefixed (R2 refined)**: the
  resolver derives `.backlogit/<queueRootDir>/<basename>` purely from
  `QueueLayout.RootDir` and deliberately does **not** consult the status-keyed
  registry routing (terminal `done` routes to archive, which would re-create the
  self-reference). **Correction from plan review**: there is no
  artifact-type→directory mapping in the registry, and the `036-DL` record
  (`archived_from: .backlogit/deliberations/036-DL.md`) is left untouched **not**
  because the resolver reproduces that path, but because its value is **not
  self-referential** — the U4 self-ref comparator excludes it. One resolver (U1)
  serves both the U2 fix and the U5 repair, guaranteeing value + format consistency.
* **Detection in doctor (MCP-safe), repair CLI-only (review-driven)**: the read-only
  invertibility audit stays in `doctor` for durable regression detection and may be
  MCP-surfaced; the destructive `--fix-archived-from` repair is **CLI-only** and
  never model-triggerable (Principle VII).
* **UnarchiveItem self-heals (review-driven)**: U3 recomputes the restore target
  when `archived_from` is self-referential, so unarchive correctness does not depend
  on the U5 migration having run. U5 is data cleanup, not a runtime prerequisite.
* **Doctor audit + `--fix` over one-shot (Option B)**: reuses the 066 doctor
  infrastructure, converts a one-time repair into durable regression detection, and
  verifies via a clean `doctor` run. *Flagged for operator confirmation at merge*
  (fallback: dedicated one-shot command using the same U1 resolver).
* **Retain UnarchiveItem 401-409 as a defensive net**: keeps unarchive safe for any
  residual self-ref records that have not yet been migrated.
* **Body-preserving repair**: the migration writes only the frontmatter block per
  the docline codec learning, keeping diffs reviewable and idempotency provable.

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| Repair corrupts the legitimate DL-subdir or canonical records | U5 matches **only** records whose `archived_from` resolves to their own archive path (the U4 comparator); explicit leave-alone fixtures in U4/U5 |
| Restore path missing `.backlogit/` prefix → F-006 rejects unarchive | U1 returns the `.backlogit/`-prefixed form matching `workspaceRelativePath`; U1 test (a) asserts the prefix; U3 round-trip proves restore succeeds |
| Restored `done` item oscillates between queue and archive routing | U3 re-archive-stability test asserts the normal branch re-stamps `.backlogit/queue/<id>.md` |
| Agent triggers the destructive repair via MCP | repair is CLI-only (`--fix-archived-from`); MCP surfaces detection only |
| Symlinked archive record/root escapes the workspace on rewrite | U5 `Lstat` + realpath containment refuses symlinks before mutation |
| Dirty working tree defeats `git revert` rollback | U7 runbook requires a clean tree (target records committed) before `--fix` |
| Code fix regresses the normal queued→archive contract | Normal branch unchanged; `archive_test.go:70` is the guard; pre-archived test added alongside |
| Fix and unarchive drift | U3 round-trip exercises both halves together |
| Non-idempotent migration | U5 idempotency test asserts a byte-stable re-run no-op + post-scan zero self-ref |
| Malformed `done` records mishandled | v1 flags only; not auto-rewritten; follow-up captured as open question |
| Path traversal via restore path | U1 stays workspace-relative; `UnarchiveItem` F-006 guard (`archive.go:368-373`) remains the enforcement point |

## Plan Hardening Signals (REQUIRED)

* **Public API / schema / contract change** — **present (minor)**: changes the
  semantics of `archived_from` for pre-archived items and adds a new `doctor`
  audit + CLI/MCP flag surface.
* **Security / auth / permission / compliance** — **present**: restore-path
  resolution must remain within the workspace (path-traversal sensitivity;
  F-006 guard interaction).
* **Migration / backfill / destructive / irreversible step** — **present (high)**:
  one-time rewrite of 130 archived records' frontmatter under `doctor --fix`.
* **External integration / operator checkpoint / external dependency** — **present**:
  requires operator merge approval; `--fix` is operator-invoked and must refuse to
  mutate without the explicit flag.
* **High runtime / rollout / rollback risk** — **present**: data migration with a
  defined rollback (git revert of the repair commit) and post-scan verification.

**Requires plan hardening: yes**

## Runtime Verification and Closure

| Unit | Runtime surface | Verification before "absorbed" | Closure artifact |
|---|---|---|---|
| U2/U3 | `backlogit archive` / `unarchive` CLI + core | `go test ./internal/core/...` green incl. new pre-archived + round-trip tests | covered by U7 runbook |
| U4 | `backlogit doctor` output | `doctor` reports the 130 self-ref + 2 malformed findings on the live tree | U7 runbook pre-census |
| U5 | `backlogit doctor --fix` | post-`--fix` scan shows **zero** self-ref records; re-run is a no-op | U7 runbook post-census + rollback trigger |
| U6 | CLI/MCP `doctor` flag | flag refuses mutation without explicit opt-in; integration test green | — |

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | All changes in Go 1.24; errors wrapped with `%w`; `golangci-lint`, `go vet`, `gofmt` gates apply. |
| II. Test-First (NON-NEGOTIABLE) | Every unit is TDD-first (red→green); U2/U3 are regression tests; U4/U5 detection/repair tests precede implementation. |
| III. Workspace Isolation | U1 returns workspace-relative paths; restore stays within `.backlogit`; F-006 traversal guard retained. |
| IV. CLI Containment | All writes target `.backlogit/` under cwd; no out-of-tree writes. |
| V. Structured Observability | `doctor` emits structured findings; repair reports counts; commits document changes. |
| VI. Single Responsibility | No new dependencies; reuses existing doctor infra, registry resolver, and frontmatter codec. |
| VII. Destructive Approval (NON-NEGOTIABLE) | The 130-record repair is destructive: **CLI-only** `--fix-archived-from` (never MCP/model-triggerable), refuses to mutate by default, requires a clean working tree, refuses symlinked targets, and routes operator approval via agent-intercom when enabled (careful mode). |
| VIII. Safety Modes | Migration runs in careful + freeze-scope mode (scope limited to `archived_from` self-ref records). |
| IX. Git-Friendly Persistence | Frontmatter-only, body-preserving rewrites via the `internal/docline` codec; reviewable diffs; atomic writes. |
| X. Context Efficiency | `doctor` returns structured, token-efficient findings + per-record `FixAction` entries rather than bulk file dumps. |
| XI. Merge Commit (NON-NEGOTIABLE) | Staging PR and the eventual implementation PR MUST merge via merge commit; no squash/rebase. |
| Overlay: agent-intercom | The destructive `--fix-archived-from` routes through intercom destructive-approval when enabled; the agent halts-and-prompts (never self-approves) if intercom is unavailable. |

No justified violations. The destructive migration (VII) is the only elevated-risk
item and is handled via explicit opt-in + operator approval, which `plan-harden`
will tighten.

## Plan Hardening

**Hardening required: yes.** Triggers: a destructive one-time rewrite of 130
archived-record frontmatter blocks (`doctor --fix`), a frontmatter-contract
semantics change for `archived_from`, and path-resolution that must stay
workspace-contained. Hardening targets U5/U6 (the migration) primarily, with
U2/U3 (correctness) secondary.

### Context consulted

* `docs/compound/2026-06-26-docline-frontmatter-contract.md` — body-preserving
  codec + idempotent seed-once migration pattern; the `--fix` repair MUST reuse a
  frontmatter-only codec and prove idempotency with a byte-stable re-run.
* `internal/core/archive.go:368-373` (F-006 traversal guard) and
  `archive.go:103-108` (060.002-T half-archive recovery) — invariants to preserve.
* `internal/core/doctor.go` (066 audit infra) — the surface being extended.
* `.github/instructions/constitution.instructions.md` — Principle VII
  (Destructive Approval, NON-NEGOTIABLE) and VIII (Safety Modes).

### Protected invariants

1. The normal queued→archive path (`currentPath != archivePath`) is byte-for-byte
   unchanged; `archive_test.go:70` stays green.
2. The 258 canonical, 211 fieldless, and 1 legitimate `036-DL` DL-subdir records
   are **never** mutated by `--fix`.
3. `archived_from` always resolves to a path **inside** `.backlogit` (F-006).
4. Re-running `--fix` is a no-op (idempotent, byte-stable).
5. `doctor` without the repair flag mutates nothing.

### Risky actions

| ProposedAction | ActionRisk | Approval | Expected ActionResult |
|---|---|---|---|
| `doctor --fix` rewrites 130 self-ref `archived_from` records to resolved canonical paths | **High** (destructive, bulk frontmatter mutation, irreversible without VCS) | Operator approval required; explicit `--fix` flag (no default mutation) | 130 records repaired; post-scan reports 0 self-ref; bodies byte-identical; commit isolates the change |
| `ArchiveItem` semantics change for pre-archived `archived_from` | **Medium** (contract behavior change) | Covered by plan-review + CI | New pre-archived test green; normal-path test unchanged |
| `UnarchiveItem` restore-path behavior change | **Medium** (could strand items if wrong) | Covered by plan-review + CI | Round-trip test green; F-006 guard intact |

### Reinforced runtime verification

* **Pre-migration census (U7 runbook, before U5 `--fix`)**: capture the baseline
  count via `Select-String '^archived_from:' .backlogit/archive/*.md` classification
  (expect 130 self-ref, 258 canonical, 211 fieldless, 3 other). Record the exact
  130 file list as the migration manifest.
* **Dry-run first**: `doctor` (detection only) MUST report exactly the 130 self-ref
  + 2 malformed findings before any `--fix` is run. If the count drifts from 130,
  **halt** and reconcile before mutating.
* **Post-migration verification**: re-run the census scan — self-ref count MUST be
  0; canonical count MUST be 258 + 130 = 388; fieldless and DL-subdir counts
  unchanged. Re-run `doctor --fix` once more and confirm a no-op (idempotency).
* **Index reconciliation**: run `backlogit sync` after the migration; confirm
  `doctor` is clean.

### Reinforced operational closure & rollback

* **Rollback trigger**: any post-scan self-ref count > 0, any body-byte diff in a
  migrated record, or any mutation of a canonical/DL/fieldless record.
* **Rollback procedure**: the `--fix` repair lands as a **single isolated commit**
  (separate from code changes) so rollback is `git revert <repair-commit>`; the
  archive records are Git-tracked, so revert fully restores prior frontmatter.
* **Batching**: process in reviewable batches; the commit diff must be inspectable
  (frontmatter-only hunks).
* **Owner / validation window**: the implementing Ship session owns verification;
  validation window closes when `doctor` is clean and the post-scan shows 0
  self-ref on a fresh checkout.
* **Safety mode**: run U5/U6 in **careful + freeze-scope** mode — scope frozen to
  `archived_from` self-referential records; no other frontmatter keys touched.

### Review-driven hardening additions (attempt 1 → 2)

Plan review (attempt 1) surfaced grounded corrections that tighten the destructive
path further. These are now binding:

* **Clean-tree precondition**: `--fix-archived-from` MUST verify the working tree is
  clean (target records committed) before mutating; `git revert` is the rollback and
  is only a valid backup if the pre-migration bytes are committed. Halt if dirty.
* **CLI-only repair**: the destructive repair is never exposed on the
  `backlogit_doctor` MCP tool (model-settable params would bypass operator approval).
* **Symlink refusal**: `Lstat` + realpath containment under `WorkspaceStorageRoot`
  before any rewrite; refuse symlinked archive roots/records.
* **Per-record manifest**: emit one `FixAction` per repaired record so the structured
  `doctor` output (not just a manual `Select-String` census) is the authoritative
  migration manifest.
* **Read-time self-heal**: `UnarchiveItem` (U3) recomputes the restore target for
  self-ref records, decoupling runtime invertibility from the migration.
* **Path-format invariant**: the resolver returns the `.backlogit/`-prefixed form so
  the F-006 guard accepts the restore; a prefix-less `queue/...` would be rejected as
  a `../` traversal and strand every restored record.

### Unresolved operator decisions (carry into review/merge)

1. **Doctor-audit (Option B) vs one-shot migration (Option A)** — recommended B;
   operator confirms at merge.
2. **Malformed `done` records (038-DL, 039-DL)** — v1 flags only; auto-repair
   deferred. Operator confirms flag-only is acceptable.
3. **Whether the migration commit ships with the code fix or as a follow-up** —
   recommend same shipment, separate commit, gated behind operator approval.

<!-- plan-review-attempt: 2 -->

## Plan Review

### Attempt 1 — Gate: FAIL → revised

Multi-persona plan-review (Constitution Reviewer, Go Reviewer, Scope Boundary
Auditor, Architecture Strategist (gpt-5.4), Security Lens Reviewer (gpt-5.4); the
Learnings Researcher pass was performed in Step 1.8 and found no archive/unarchive
prior art). The personas read the cited code and surfaced grounded P1 corrections.
Each was resolved by the revisions above:

| # | Sev | Finding | Resolution |
|---|---|---|---|
| 1 | P1 | `archived_from` must be `.backlogit/`-prefixed (per `workspaceRelativePath` / `archive_test.go:70`); a prefix-less `queue/<basename>` makes the F-006 guard reject the restore and strand records | U1 now returns `.backlogit/<queueRootDir>/<basename>`; U1 test (a) asserts the prefix; U2/U3 assert the prefixed form |
| 2 | P1 | No artifact-type→directory mapping exists in the registry (it routes by *status*); the "R2 / 036-DL type-dependence" rationale was unfounded | U1 re-scoped to resolve purely from `QueueLayout.RootDir` (no registry); Decisions corrected — `036-DL` is safe because its value is not self-referential, excluded by the U4 comparator |
| 3 | P1 | U1's unexported helper cannot be tested from `package core_test` (`archive_test.go`) | U1 tests moved to a new in-package `internal/core/archive_internal_test.go` |
| 4 | P1 | Exposing destructive `--fix` on the `backlogit_doctor` MCP tool makes the bulk rewrite agent-triggerable, bypassing Principle VII | U6 repair is **CLI-only** (`--fix-archived-from`); MCP surfaces detection only; R8 trace corrected |
| 5 | P1 | UnarchiveItem correctness was coupled to the migration running first | U3 now recomputes the restore target at read time for self-ref records (self-heal); U5 is cleanup, not a prerequisite |
| 6 | P2 | `git revert` rollback assumes a clean committed tree | Clean-tree precondition added to U5/U7 and hardening |
| 7 | P2 | Restored `done` item could oscillate (queue vs archive routing) | U3 adds a re-archive-stability test + status invariant |
| 8 | P2 | U5 partial-failure semantics undefined | U5 specifies continue-on-error + per-record `FixAction` (mirrors `fix-orphans`) |
| 9 | P2 | Symlink-following hazard on bulk rewrite | U5 adds `Lstat` + realpath containment, refuses symlinks |
| 10 | P2 | U1 resolver should be pure (no registry I/O) → bare-string return is safe | U1 re-scoped to pure-over-`QueueLayout`; no registry I/O |
| 11 | P2 | Constitution Check omitted X + intercom overlay | Added rows for X and the agent-intercom approval overlay |

Architecture P1 (path-policy could drift across `routing.go`/`artifacts.go`/`archive.go`)
and Scope P2 (Option A could collapse U4+U5+U6) are acknowledged: U1 is intentionally
a thin pure helper over `QueueLayout` (not a third routing engine), and the Option A/B
decision remains an explicit operator checkpoint (Unresolved Decision 1). P3 advisories
(artifactType param dropped from U1, structured FixAction, materialize units in the
backlog — done at harvest) are folded in or captured as backlog context.

### Attempt 2 — Gate: PASS

Re-review (Go Reviewer, on the revised plan) confirmed the P1 corrections are closed:
the resolver contract is now code-grounded (`QueueLayout.RootDir`, `.backlogit/`-prefixed,
in-package test), the destructive surface is CLI-only, and UnarchiveItem self-heals.
Plan hardening is present and materially complete for the elevated-risk migration. No
P0/P1 findings remain. **Proceed to harvest.** The two operator checkpoints (Option A/B;
malformed-`done` handling) are carried forward as harvest/merge-time confirmations, not
gate blockers.

**Acknowledged advisories (non-blocking, carried to implementation):**

* **P2 (U5 diff shape)**: `internal/docline` `Encode` re-serializes with deterministic
  sorted-key frontmatter, while legacy records were written by
  `models.SerializeFrontmatter`. The first `--fix` pass may produce full-frontmatter
  key-reorder hunks in the 130 target files (body bytes preserved; idempotency holds).
  The implementer/reviewer should expect reordered-key diffs and assert **no semantic
  key change beyond `archived_from`**. A pre-canonicalization commit is an option if a
  one-line diff is required.
* **P3 (MCP precedent)**: `backlogit_doctor` already exposes a destructive op
  (`fix_orphans` → `ArchiveItem`) as a model-settable bool over MCP. This plan
  correctly does NOT extend that pattern for the bulk rewrite; the existing
  `fix_orphans` surface is an adjacent Principle-VII consideration worth a future
  operator note (no action for this plan).
