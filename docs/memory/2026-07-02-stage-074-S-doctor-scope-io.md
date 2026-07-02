# Stage session — 074-S doctor --target scope-vs-io classification

- **Date:** 2026-07-02
- **Agent:** Stage
- **Entry point:** single stash entry `6B2C2E53` (kind=task, P3/low) routed by the Orchestrator.
- **Origin:** 071-S PR#156 Copilot follow-up "J" (review thread `PRRT_kwDORzozKM6Ngr6D`,
  merged `531bd51`). Last outstanding 071-S review follow-up; sibling "K" shipped as 072-S.

## Pipeline outcome

| Step | Result |
|---|---|
| 0.0 Tool gate | backlogit registry present; CLI `C:\Tools\backlogit.exe` v1.3.0; `ALL_TOOLS_OK` |
| 0.1 Index sync | `INDEX_SYNC_OK` (662 artifacts) |
| 1 Triage | 4 active stash entries; `6B2C2E53` = task-shaped, selected. Others deferred. |
| 1.5 Grouping | Single-entry fallback (operator-targeted). Solo group, implicit covering feature. |
| 1.8 Learnings | `docs/compound` (47 files) searched — no direct `confineToStorageRoot`/scope-io hit; governing prior = same classification-branch family cited by the 072-S plan. |
| 2 Deliberation | No standalone deliberation artifact (lean-plan directive; two-option contract-grounded decision). Rationale folded into plan Decisions. |
| 3 impl-plan | `docs/exec-plans/2026-07-02-doctor-target-scope-io-classification-plan.md` |
| 3.2 Hardening | `Requires plan hardening: no` (security-relevant containment branch untouched + regression-tested) |
| 4 plan-review | **PASS** (attempt 1). 5 personas. P2 (TDD compile-vs-assertion red) resolved by revision; P3s folded/acknowledged. |
| 5 Harvest | feature `074-F` + task `074.001-T` (single unit; no sub-epics/subtasks; no deps) |
| 5.5 Shipment | `074-S` (queued), items `[074-F, 074.001-T]`, parent-first. Scope guard applied. |
| 5.6 Archive | stash `6B2C2E53` archived with forward-link to 074-F/074.001-T/074-S |

## Core decision (scope-vs-io discriminator)

- **Mechanism:** reuse the existing `(ok, err)` two-value contract of `confineToStorageRoot`.
  `err != nil` ⟺ IO/path-resolution fault (only from the two `filepath.Abs` calls) → reclassify
  to `kind=io` with wrapped `Message`. `ok == false` (nil err) ⟺ genuine containment violation
  (incl. 071-S `!pathContained` symlink escape) → stays `kind=scope`. **No sentinel error needed.**
- **Security guarantee (071-S symlink escape): PRESERVED** — the `ok == false` branch and
  `confineToStorageRoot` internals are left byte-for-byte unmodified (Security Reviewer traced all
  return paths; existing `TestDoctorTarget_SymlinkEscapeRejectedAsScope` +
  `TestDoctorTarget_OutsideStorageRootRejectedAsScope` remain the regression guard).
- **Exit-code-neutral:** `doctorTargetExitCode` maps both scope and io → exit 3. No new kind, no
  `DoctorTargetResult` schema-version bump.
- **Testability:** unexported boundary seam `var confineFn = confineToStorageRoot` in
  `PrepareDoctorTarget` (the buggy branch is unreachable via normal inputs). Single non-parallel,
  defer-restored test forces the resolution error. Verified under `go test -race`.
- **Both surfaces:** single core fix in `PrepareDoctorTarget` inherited by CLI
  (`runDoctorTargetMode`) and MCP (`handleDoctor → DoctorTarget`).

## Handoff to Ship

- **Shipment:** `074-S` (queued) — the handoff token.
- Ship executes: TDD implementation of 074.001-T, quality gates (`go test -race ./...`, `go vet`,
  `golangci-lint`, `gofmt -l .`), branch/PR/merge.

## Deferred (untouched this session)

- `21E17BFC` (feature, singleton MCP server — contingency, low)
- `D070FD3C` (feature, surface covering feature ID in shipment views — low)
- `9140F65C` (task, npm publish in Release workflow — low)
- Pre-existing orphan `016.001-R` — not in scope.
