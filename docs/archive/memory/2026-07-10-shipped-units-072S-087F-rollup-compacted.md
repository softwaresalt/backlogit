---
chunk_strategy: h1-h2-h3
description: "Roll-up of fifteen per-shipment compacted memory summaries for shipped units 072-S through 085-S and 087-F (all shipped and archived); supersedes the individual 2026-07-10 compacted files now under docs/archive/memory/."
doc_type: memory
docline:
    date: 2026-08-24T00:00:00Z
    status: accepted
    tags:
        - compacted
        - rollup
        - 072-S
        - 087-F
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-shipped-units-072S-087F-rollup-compacted.md
title: "Compacted roll-up — shipped units 072-S through 087-F"
---

# Compacted roll-up: shipped units 072-S through 087-F

Second-order compaction performed 2026-08-24 to bring `docs/memory/` back under the
40-file trigger. Each section below is the verbatim body of one previously-compacted
per-shipment summary, with headings demoted one level. No substance was removed.
The fifteen verbose originals were archived (not deleted) to `docs/archive/memory/`.

| Unit | Archived source |
|---|---|
| 072-S-doctor-nil-headerdef | `docs/archive/memory/2026-07-10-072-S-doctor-nil-headerdef-compacted.md` |
| 073-S-artifacts-write-nil-headerdef | `docs/archive/memory/2026-07-10-073-S-artifacts-write-nil-headerdef-compacted.md` |
| 074-S-doctor-target-scope-io | `docs/archive/memory/2026-07-10-074-S-doctor-target-scope-io-compacted.md` |
| 075-S-covering-feature-display | `docs/archive/memory/2026-07-10-075-S-covering-feature-display-compacted.md` |
| 076-S-harvest-docline-frontmatter | `docs/archive/memory/2026-07-10-076-S-harvest-docline-frontmatter-compacted.md` |
| 077-S-shipment-items-normalization | `docs/archive/memory/2026-07-10-077-S-shipment-items-normalization-compacted.md` |
| 078-S-cli-mcp-parity-phase1 | `docs/archive/memory/2026-07-10-078-S-cli-mcp-parity-phase1-compacted.md` |
| 079-S-cli-mcp-parity-phase2 | `docs/archive/memory/2026-07-10-079-S-cli-mcp-parity-phase2-compacted.md` |
| 080-S-release-docs-hygiene | `docs/archive/memory/2026-07-10-080-S-release-docs-hygiene-compacted.md` |
| 081-S-closure-compaction-and-ci-gating-deferral | `docs/archive/memory/2026-07-10-081-S-closure-compaction-and-ci-gating-deferral-compacted.md` |
| 082-S-pre-task-completion-gate-broker | `docs/archive/memory/2026-07-10-082-S-pre-task-completion-gate-broker-compacted.md` |
| 083-S-gate-broker-phase2-hardening | `docs/archive/memory/2026-07-10-083-S-gate-broker-phase2-hardening-compacted.md` |
| 084-S-ancestor-aware-staleness | `docs/archive/memory/2026-07-10-084-S-ancestor-aware-staleness-compacted.md` |
| 085-S-empty-head-fail-closed | `docs/archive/memory/2026-07-10-085-S-empty-head-fail-closed-compacted.md` |
| 087-F-ci-gating-pipeline | `docs/archive/memory/2026-07-10-087-F-ci-gating-pipeline-compacted.md` |

## 072-S-doctor-nil-headerdef

### Summary

Shipment `072-S` closed the `doctor --target` nil-`HeaderDef` fail-open path. Stage promoted stash `C16DBBEB` into `072-F` and task `072.001-T`; Ship implemented the shared-core fix in `ValidateDoctorTargetResolved`, opened PR #158, resolved CI and Copilot feedback, and completed post-merge archival after operator P-014 approval.

### Archived originals

* `docs/archive/memory/2026-07-01-stage-072-S-doctor-nil-headerdef.md`
* `docs/archive/memory/2026-07-01-ship-072-S-checkpoint.md`

### Decisions and outcomes

* Nil `Workspace.HeaderDef` is a system/config precondition fault, not user validation; it returns `DoctorTargetIO`, `OK=false`, and exit 3 without a new result kind or schema version.
* CLI and MCP parity is structural because both surfaces call `core.ValidateDoctorTargetResolved`.
* The plan reused the durable nil-zero-value fail-closed learning; no separate deliberation or new compound entry was warranted.
* PR #158 merged by true merge commit `d3f0facf530592c526e261b3818dc6e0d0dd0ced`; `backlogit shipment ship 072-S` archived `072.001-T`, `072-F`, and `072-S` with P-007 clean.

### Files and verification

* `internal/core/doctor_target.go` inverted the nil guard and broadened the `DoctorTargetIO` doc comment.
* `internal/core/doctor_target_test.go` added `TestDoctorTarget_NilHeaderDefFailsClosed` with loaded-vs-nil precedence coverage.
* Quality gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`, and changed-file formatting.
* Fix-CI added docline frontmatter to closure docs; Copilot requested removing a dated plan path from a code comment, then re-review was clean.
* Follow-up stash `266816CE` captured the sibling create/update write-path nil-`HeaderDef` defect, which became `073-S`.

## 073-S-artifacts-write-nil-headerdef

### Summary

Shipment `073-S` closed the create/update write-path nil-`HeaderDef` fail-open family. Stage promoted stash `266816CE` into `073-F` and `073.001-T`; Ship implemented a shared `requireHeaderDef` guard in core write paths and completed merge plus post-merge archival.

### Archived originals

* `docs/archive/memory/2026-07-02-stage-073-S-artifacts-nil-headerdef.md`
* `docs/archive/memory/2026-07-02-ship-073-S-checkpoint.md`
* `docs/archive/memory/2026-07-02-ship-073-S-post-merge-closure-session.md`

### Decisions and outcomes

* Nil `HeaderDef` on mutation writes is a hard fail-closed `blerrors.ErrConfig`, not `ErrValidation`, because the user cannot correct a missing workspace schema.
* `requireHeaderDef(ws)` must run before `ApplyFieldDefaults` and `ValidateArtifactFields`; otherwise `ResolveFieldSchema` can nil-deref before validation.
* The two write paths share one helper and one task because CLI and MCP both route through `core.CreateArtifact` and `core.UpdateArtifact`.
* PR #160 merged by true merge commit `00b9b1de4fa29b3776788df280fc8f75a648d04c`; post-merge `shipment ship 073-S` archived `073.001-T`, `073-F`, and `073-S` with clean reconcile.

### Files and verification

* `internal/core/artifacts.go` added `requireHeaderDef` and rewired create/update before defaults and validation.
* `internal/core/artifacts_headerdef_test.go` added red-to-green create/update fail-closed tests plus loaded-regression coverage.
* Full quality gates and code review passed; runtime verification built the CLI and checked loaded-path add/update still succeed.
* Knowledge graduation updated `exported-cache-zero-value-bypass-2026-06-29.md` as the third and final nil-precondition fail-open recurrence.

## 074-S-doctor-target-scope-io

### Summary

Shipment `074-S` resolved Copilot follow-up J from 071-S by distinguishing path-resolution IO faults from genuine containment-scope violations in `doctor --target` handling. Stage harvested `074-F` and `074.001-T`; Ship implemented the classification seam. PR #162 merged as `f2bdb7a6711c46326720026d3ff0bc6f822ece1e`, and `074-S`, `074-F`, and `074.001-T` were shipped and archived (post-merge closure complete).

### Archived originals

* `docs/archive/memory/2026-07-02-stage-074-S-doctor-scope-io.md`
* `docs/archive/memory/2026-07-02-ship-074-S-scope-io-classification.md`

### Decisions and outcomes

* The existing `(ok, err)` contract on `confineToStorageRoot` is the discriminator: `err != nil` means IO/path-resolution fault; `ok == false` with nil error means containment scope violation.
* Exit-code neutrality was preserved: both scope and IO still map to exit 3; no new result kind or schema version was introduced.
* An unexported `confineFn` seam made the normally unreachable IO branch testable without changing `confineToStorageRoot` behavior.
* Security-sensitive symlink-escape behavior from 071-S was preserved byte-for-byte and locked by existing scope tests.

### Files and verification

* `internal/core/doctor_target.go` reclassified the resolution-error branch and preserved wrapped error text.
* `internal/core/doctor_target_test.go` added IO classification and lexical out-of-scope tests.
* `go test -race ./...`, `go vet ./...`, `golangci-lint run`, and changed-file format checks passed.
* Code review found no P0/P1 and verified seam correctness plus no containment regression.

## 075-S-covering-feature-display

### Summary

Shipment `075-S` shipped a read-only covering-feature projection for shipment list/get surfaces. Stage promoted stash `D070FD3C` into `075-F` plus core, CLI, and MCP tasks; Ship implemented all three tasks, resolved an inherited Stage-commit halt, merged PR #164, and completed post-merge closure.

### Archived originals

* `docs/archive/memory/2026-07-02-stage-D070FD3C-covering-feature-display.md`
* `docs/archive/memory/2026-07-02-075-S-task3-mcp-checkpoint.md`
* `docs/archive/memory/2026-07-02-075-S-HALT-inherited-stage-commit.md`
* `docs/archive/memory/2026-07-02-ship-075-S-post-merge-closure-session.md`

### Decisions and outcomes

* The governing decision was forward-only display; existing shipment manifests and titles were never rewritten.
* Final shape is a top-level `covering_feature: {id,title}` object on JSON responses, not derived keys inside `custom_fields`, preventing write-path echo from persisting derived data.
* Resolution is read-only and uses `bldb.GetItem`, not `loadArtifact`, to avoid upserts on cache misses.
* Covering feature is the parent-first dotless root feature; zero-feature shipments omit the object everywhere.
* PR #164 merged as true merge commit `842e8883899ba25ce9c31840c89806ed2e032549`; post-merge closure shipped and archived `075-S` with clean reconcile.

### Files, failures, and verification

* Core shared shaper, CLI list/get, generated CLI docs, and MCP `handleGetShipment` / `handleListShipments` all surfaced the projection.
* MCP tests covered get/list shape, zero-feature omission, no custom-field leak, read-only invariant, and CLI/MCP parity.
* Plan review rejected the earlier custom-fields injection approach because agents could echo derived data into writes.
* Ship halted when PR #164 inherited unpushed Stage commit `f316dfd`, invalid docline frontmatter, and backlog state; the operator resolved the Stage issue before merge.
* Runtime verification confirmed projection present, zero-feature omit, and storage/index hashes unchanged. Follow-up stash `17D29DDC` captured normalization consolidation.

## 076-S-harvest-docline-frontmatter

### Summary

Shipment `076-S` hardened Stage plan/harvest docs so generated implementation plans are born docline-compliant and linted before harvest. It was triggered by the invalid `doc_type: exec-plan` frontmatter that blocked 075-S CI.

### Archived originals

* `docs/archive/memory/2026-07-02-stage-076-S-harvest-docline-frontmatter-hardening.md`
* `docs/archive/memory/2026-07-02-ship-076-S-post-merge-closure-session.md`

### Decisions and outcomes

* The root cause was split between `.github/skills/impl-plan/SKILL.md` lacking docline frontmatter guidance and `.github/skills/harvest/SKILL.md` lacking a pre-commit lint gate.
* The CI entrypoint `make docs-lint` was made authoritative for self-linting; stale installed binaries are not sufficient for lint-gate parity.
* Upstream template/harness drift was deferred to `EED25928` because it was larger and partly outside workspace containment.
* PR #166 merged by true merge commit `ef9dc20468d865bbaf7d7b1e9b982ff7f4045422`; `backlogit shipment ship 076-S` archived `076.001-T`, `076.002-T`, `076-F`, and `076-S` with clean reconcile.

### Files and verification

* `.github/skills/impl-plan/SKILL.md` received the docline contract for plans.
* `.github/skills/harvest/SKILL.md` received a pre-harvest docline lint gate.
* Runtime verification showed a compliant plan lints clean while the 075-S defect replica fails with three violations.
* Operator accepted three Copilot P3 comments as won't-fix after replies and thread resolution.
* Knowledge graduation reinforced `docs/compound/2026-06-26-docline-frontmatter-contract.md`; follow-up stash `B55985DD` captured wording cleanup for `make docs-lint --path` references.

## 077-S-shipment-items-normalization

### Summary

Shipment `077-S` consolidated duplicate shipment-item normalization logic. Stage promoted stash `17D29DDC` into `077-F` and `077.001-T`; Ship later resumed post-merge closure after a Windows update interrupted the original session.

### Archived originals

* `docs/archive/memory/2026-07-03-stage-17D29DDC-normalize-shipment-items-consolidation.md`
* `docs/archive/memory/2026-07-03-ship-077-S-post-merge-closure-session.md`

### Decisions and outcomes

* The true duplicate was the `[]any` to `[]string` mapping shared by core and MCP; the core mutator stayed separate because it wraps read normalization with writeback.
* `core.shipmentItems` became exported `core.NormalizeShipmentItems` and was hardened to never return nil, including empty `[]string{}` input.
* MCP `normalizeShipmentItems` was deleted; `handleListShipments` delegates to core while retaining the end-to-end never-null guard.
* PR #168 had already merged at `c8487407d5ddb19d26c754ce82606df929e35f46`; post-merge closure shipped `077-S` and archived `077.001-T`, `077-F`, and `077-S`.

### Files and verification

* `internal/core/shipment.go` exported `NormalizeShipmentItems` and became the single mapping source.
* Unit tests moved to `internal/core/shipment_normalize_test.go` with the empty-slice non-nil case.
* MCP response tests retained list never-null coverage.
* Merged-code gates passed on resume; gofmt noise was CRLF-only and not reformatted.
* No new compound entry was created because never-null and single-shaper invariants were already covered by existing learnings and 075-S closure.

## 078-S-cli-mcp-parity-phase1

### Summary

Shipment `078-S` established an honest CLI/MCP fallback map and filled the highest-value phase-1 gaps. Stage promoted `E16F4664` and folded `7ECBAC7E`; Ship delivered registry correction, drift tests, `shipment add`, `checkpoint create`, shipment-list items normalization, docs, and post-merge closure.

### Archived originals

* `docs/archive/memory/2026-07-03-stage-E16F4664-cli-mcp-parity-triage.md`
* `docs/archive/memory/2026-07-03-stage-E16F4664-cli-mcp-parity-shipment-complete.md`
* `docs/archive/memory/2026-07-03-ship-078-S-session.md`

### Decisions and outcomes

* Registry over-claims are more dangerous than missing fallbacks; incorrect `link` CLI mappings were stripped and marked MCP-only until a real CLI group exists.
* Drift detection must be driven from the typed MCP tool set, not prose or generated manifests.
* `shipment add` and `checkpoint create` were built before flipping their registry rows so the drift gate stayed honest.
* `docs/cli-reference/` is generated; authored review/design material belongs in `docs/reviews/` and `docs/design-docs/`.
* PR #170 merged by true merge commit `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`; post-merge closure shipped and archived 15 manifest items plus `078-S`.

### Files and verification

* `.autoharness/backlog-registry.yaml`, `internal/cli/registry_parity_test.go`, shipment/checkpoint CLI files and tests, parity matrix, and fallback guide were updated.
* CLI Reference Drift failed once until generated CLI docs were regenerated and CRLF noise restored.
* Copilot review drove safe-default corrections around `docs migrate` and routed deliberation-record drift to Stage.
* Runtime verification and §1.9 passed. New compound learning `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` captured the durable pattern.

## 079-S-cli-mcp-parity-phase2

### Summary

Shipment `079-S` built the deferred CLI fallbacks worth building from phase 1. Stage promoted `6C6ACE00`, corrected the phase-1 deliberation record via ride-along `2827CB5F`, and Ship delivered link, hook, memory, comment, and metadata CLI surfaces with registry/docs updates.

### Archived originals

* `docs/archive/memory/2026-07-03-stage-6C6ACE00-phase2-cli-mcp-parity-shipment-complete.md`
* `docs/archive/memory/2026-07-03-ship-079-S-session.md`

### Decisions and outcomes

* Build families: `link add/remove/list`, `hooks poll/ack`, `memory save`, `comment add`, and `metadata types/wit/templates`.
* Defer `merge_sync` to phase 3 because it writes by default and needs dry-run guardrails; keep `log_telemetry` permanently MCP-only.
* Registry flag parity is load-bearing; the drift test must assert required flags and positional/flag names, not only command path existence.
* PR #172 merged by true merge commit `a8e07ea38f8e153e9a29def264538bcab8222868`; post-merge closure shipped and archived all 15 items plus source deliberation `051-DL` and `079-S`.

### Files and verification

* CLI link, hooks, memory, comment, and metadata subcommands landed with tests.
* `core.AppendComment` extracted shared behavior; MCP passes shared `EventWriter`, CLI uses one-shot default.
* Copilot caught a valid concurrency issue: a fresh `EventWriter` per `AppendComment` call would drop in-process append serialization. The shared-writer fix was merged.
* Final gates passed: build, vet, tests, lint, generated docs idempotency, CI, runtime verification, Copilot freshness, and §1.9.
* New compound learning `2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` captured the shared-writer pattern.

## 080-S-release-docs-hygiene

### Summary

Shipment `080-S` shipped release-pipeline and docs hygiene. Stage bundled `9140F65C` and `B55985DD` into `080-F`; Ship guarded npm publish steps on `NPM_TOKEN`, characterized npm package output, fixed docs-lint wording, merged PR #174, and completed post-merge closure.

### Archived originals

* `docs/archive/memory/2026-07-04-stage-080-S-release-docs-hygiene-session.md`
* `docs/archive/memory/2026-07-04-ship-080-S-session.md`

### Decisions and outcomes

* Release workflow token presence is checked through env-indirection and boolean output; the token is never echoed and publish steps retain `continue-on-error: true`.
* `scripts/package-npm.sh` received characterization through a thin Go wrapper; the script itself stayed unchanged and `npm pack` remained optional.
* Docs were clarified to distinguish repo-wide `make docs-lint` from scoped `go run ./cmd/backlogit docs lint --path <file>`.
* PR #174 merged by true merge commit `d0ebb4f`; post-merge `shipment ship 080-S` archived all scoped tasks, feature, and shipment with clean reconcile.

### Files and verification

* `.github/workflows/release.yml`, `tests/integration/package_npm_characterization_test.go`, and two docs/backlog wording surfaces were updated.
* `actionlint`, YAML parse, scoped and repo-wide docs lint, Go tests, vet, lint, and CI all passed; Copilot produced no inline threads.
* Runtime verification was PASS WITH FOLLOW-UP: observe the guard on the next real tagged release.
* Compound refresh kept existing learnings and did not promote the Windows CRLF/gofmt gotcha.

## 081-S-closure-compaction-and-ci-gating-deferral

### Summary

This Stage session split two operator-selected hygiene items into separate risk units. Closure compaction (`2EF8B7AD`) passed review and became queued shipment `081-S`; CI cost-gating (`D760E508`) was deliberately deferred after repeated plan-review failures on merge-safety semantics.

### Archived originals

* `docs/archive/memory/2026-07-04-stage-ci-gating-closure-compaction-session.md`

### Decisions and outcomes

* `081-S` was harvested as feature `081-F` plus `081.001-T`, scoped to consolidating stale `docs/closure` records.
* `D760E508` was not harvested because three plan-review rounds failed on the core dorny/paths-filter gating design; self-certifying an unreviewed fourth revision would violate the review gate.
* Source verification of dorny `predicate-quantifier: every` corrected the mental model: disjoint positive patterns under `every` are constant-false; all-negated unsafe detection is the safe design.
* Required contexts must keep reporting on every PR type; job-level gating is acceptable, trigger-level `paths` or `paths-ignore` is not.

### Artifacts and handoff

* Created deliberations and plans for CI cost-gating and closure compaction.
* The CI plan holds a deferred corrected design dossier for a future fresh review.
* Queued `081-S` with items `[081-F, 081.001-T]` and archived source stash `2EF8B7AD`.
* Later 089-S work should be reconciled against the corrected D760E508 design rather than the failed early review assumptions.

## 082-S-pre-task-completion-gate-broker

### Summary

Shipment `082-S` delivered the pre-task-completion gate broker that composes backlogit status transitions with autoharness validation. Stage promoted `D23DFA0B` through a difficult plan-review cycle; Ship implemented the broker, CLI/MCP error surfaces, evidence, shipment gating, docs, reviews, PR #178, and post-merge closure PR #179.

### Archived originals

* `docs/archive/memory/2026-07-06-stage-D23DFA0B-gate-broker-session.md`
* `docs/archive/memory/2026-07-06-ship-082-S-session.md`

### Decisions and outcomes

* Gate execution holds the task lock, rereads state after acquire, and prevents partial completion on blocked/config/setup/timeout/in-progress results.
* `enabled:auto` fails open when no gates are configured or auto-discovery cannot resolve a base; `enabled:true` and explicit base override fail closed on setup/config failures.
* Force is CLI-only, requires `--force-reason`, records forced evidence, and is unavailable through MCP.
* Evidence is logs-only; the indexed read model was deferred to phase 2.
* PR #178 merged by true merge commit `e47e1291c49f906a4b257c60f117a2cd05107db7`; post-merge `shipment ship 082-S` archived 24 artifacts and closure PR #179 reached merge-ready.

### Files, review, and verification

* `internal/core/gate`, `internal/core`, `internal/cli`, and `internal/mcp` gained gate decision/running, transition, evidence, exit-code, doctor, and structured MCP result behavior.
* Pre-push adversarial review found and Ship fixed path-qualified binary local-RCE, timeout-before-probe, MinimalEnv, evidence-required parity, and base-override audit gaps.
* Copilot fixed `update --json` dropping `--section`, allowed-actions divergence, and dead runtime import.
* Real autoharness 1.4.7 runtime verification covered pass, fail-open, fail-closed setup exit 7, logs-only evidence, structured JSON, and update-section regression.
* New compound learnings captured bare-path binary validation, timeout-before-probe, and the autoharness gate broker integration contract.

## 083-S-gate-broker-phase2-hardening

### Summary

Shipment `083-S` delivered gate-broker phase-2 hardening and later re-closed after 084-S fixed ancestor-aware shipment staleness. Stage bundled five deferred findings into `083-F`; Ship implemented F1, F4, F5, F7, and Q3 indexed gate evidence, merged PR #180, hit a real post-merge closure wall, then completed re-closure after the 084-F fix landed.

### Archived originals

* `docs/archive/memory/2026-07-06-stage-gate-broker-phase2-session.md`
* `docs/archive/memory/2026-07-06-083-S-ship-feature-complete-checkpoint.md`
* `docs/archive/memory/2026-07-06-083-S-copilot-iter1-resolved-checkpoint.md`
* `docs/archive/memory/2026-07-06-083-S-post-merge-closure-BLOCKED-checkpoint.md`
* `docs/archive/memory/2026-07-07-083-S-reclosure-RESOLVED-checkpoint.md`

### Decisions and outcomes

* F4 accepts forced evidence unconditionally but requires `ran==true` for `passed` evidence.
* Q3 introduced a disposable `gate_evidence` projection table sourced from logs during rehydration; logs remain source of truth.
* Doctor uses the indexed projection with log-scan fallback when projection rows are absent or stale.
* PR #180 merged by true merge commit `ac41bb1d2611fadd0fae6ccc49b3a8233468622d`; feature value reached `main` but shipment closure initially refused under strict head-sha equality.
* After 084-S shipped ancestor-aware staleness, rerunning the same `shipment ship 083-S --sha ac41bb1d...` succeeded and archived 11 artifacts.

### Files, failures, and verification

* Core and DB gained shared gate-evidence predicate/constants, `gate_evidence` table/index, sync population, and doctor projection lookup.
* Rehydration was optimized after Copilot flagged double parsing: `rehydrateItemLogs` returns parsed events and `rehydrateGateEvidence` consumes them.
* Closure stopped rather than forcing because strict equality was a real semantic mismatch: member evidence heads were ancestors of the merge commit, not equal to it.
* Q3 sync idempotency was verified with repeated `backlogit sync` and stable doctor output.
* Compound learning `2026-07-06-ancestor-aware-shipment-gate-staleness.md` was updated with the 083-S exposure cross-reference.

## 084-S-ancestor-aware-staleness

### Summary

Shipment `084-S` fixed the false-staleness bug exposed by 083-S by making shipment-gate member evidence ancestor-aware. Stage promoted stash `885A7F65` after an operator-authorized plan-review confirmation cycle; Ship merged PR #182, shipped and archived `084-S`, and opened closure PR #183.

### Archived originals

* `docs/archive/memory/2026-07-06-stage-885A7F65-ancestor-staleness-checkpoint.md`
* `docs/archive/memory/2026-07-06-084-S-ship-session-complete.md`

### Decisions and outcomes

* Member evidence is fresh when `head_sha` is equal to or an ancestor of the shipment head, checked with `git merge-base --is-ancestor`.
* Git object names are restricted to 40- or 64-hex strings before use in git exec calls.
* Ancestor checks are bounded, use argv-array execution, `cmd.Dir=ws.RootPath`, `gate.MinimalEnv`, stderr capture, and fail-closed trichotomy.
* Shipment head is resolved once before evaluation, then re-resolved as the final read before success to catch drift.
* PR #182 merged by true merge commit `f49ce3c37b460afce81591ca6e354b8de3a14a17`; the fixed binary then shipped its own `084-S` closure successfully.

### Files and verification

* `internal/core/shipment_gate.go` gained ancestor-or-equal lineage checks and bounded head resolution.
* Tests covered included, divergent, malformed, timeout/cancel, head-drift, and no-repo/legacy skip behavior.
* Plan review required four attempts; H1 fixed `headSHABounded` so bounded-read timeout/cancel fails closed.
* Copilot found a bounded-helper hard-cap issue and dedup opportunities; fixed and re-review was clean.
* Runtime verification passed five real-subprocess scenarios. New compound learnings captured ancestor-aware staleness and bounded helper timeout hard cap.

## 085-S-empty-head-fail-closed

### Summary

Shipment `085-S` hardened empty shipment/member `head_sha` handling so enforced real-repo shipment gates fail closed instead of silently skipping lineage checks. Stage bundled `B85DAEE8` and `1AEA2B0E`; Ship merged PR #185, shipped `085-S`, and prepared closure PR work.

### Archived originals

* `docs/archive/memory/2026-07-07-stage-shipment-gate-empty-head-fail-closed-checkpoint.md`
* `docs/archive/memory/2026-07-07-085-S-ship-session-checkpoint.md`

### Decisions and outcomes

* The discriminator is a bounded fail-closed repo-presence probe (`git rev-parse --is-inside-work-tree`), not `ev.Enforced`, because test brokers can fake git probes.
* Enforced plus real worktree plus empty shipment or member head fails closed; no-repo, non-enforcement, or non-autoharness paths preserve legacy skip behavior.
* Forced evidence has no empty-head exception because forced-in-real-repo records a head.
* PR #185 merged by true merge commit `7c129b0`; post-merge `shipment ship 085-S --sha 7c129b0` archived six artifacts with clean P-007 reconcile.

### Files, review, and verification

* `internal/core/shipment_gate.go` gained `inGitWorktreeBounded`, empty shipment-head fail-closed, and empty member-head fail-closed behavior.
* Tests added git fixture coverage, flipped the real-repo empty-member-head acceptance test to refusal, and preserved no-repo skip regression.
* Adversarial review blocked on broken `.git` pointer fail-open, then follow-up fixes made broken-repo handling message-independent and fail-closed.
* Copilot rounds fixed indeterminate `.git` stat fail-closed behavior and guarded no-repo tests against ambient git state.
* Full quality gates passed; runtime verification and operational closure were recorded. New compound knowledge captured empty-head fail-closed behavior and cross-referenced 084-S.

## 087-F-ci-gating-pipeline

### Summary

This orchestrator memory recorded the CI gating pipeline (`087-F`) and a separate shipment-add parity verification. PR #189 merged the workflow-gating feature, while stash `8A87C3A7` was verified as already implemented on main and archived without a new PR.

### Archived originals

* `docs/archive/memory/orchestrator-087-ci-gating-pipeline-memory.md`

### Decisions and outcomes

* CI workflow gating used `dorny/paths-filter` with a fail-safe denylist and `predicate-quantifier: every`; the default `some` semantics would make a leading `**` match every file and defeat negations.
* Heavy test and drift jobs were gated at job/step level while docs-lint stayed always-on.
* Repository branch protection was actually a ruleset requiring approval, last-push approval, `test (1.24)`, thread resolution, and merge-method=merge; admin bypass was used only after substantive gates passed.
* PR #189 merged by true merge commit `305bd4ff494c3b8274183563490c1bdeaaa7f778` and branch cleanup completed.

### Files and verification

* `.github/workflows/ci.yml` and `cli-reference-drift.yml` gained the `changes` job, gated heavy matrix/drift behavior, and SHA-pinned `dorny/paths-filter`.
* Copilot caught the critical `predicate-quantifier: every` issue that three pre-PR adversarial reviewers missed; fixed in `9815866`, replied, resolved, and re-review was clean.
* Gate-run path was proven by the feature PR; skip-path behavior was validated from corrected dorny semantics, not a separate live docs-only PR.
* Remaining stash entries were evaluated and left in place because they were contingent, external, or too broad for autonomous action.

