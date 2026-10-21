---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter (pipeline-topology gate reads them). Do NOT run
# `backlogit docs migrate --apply` against this file.
closure_status: READY
compaction_status: done
description: "Post-merge closure for 184-S / 184-F shipped-only shipment predecessor readiness docs (PR #555)."
doc_type: closure
schema_version: "1.0"
source: docs/closure/184-S-184-F-post-merge-closure.md
shipment_id: 184-S
title: "184-S / 184-F Post-Merge Closure"
---

# 184-S / 184-F Post-Merge Closure

* Shipment: 184-S (184-F, 184.016-T, 184.017-T, 184.018-T), shipped via governed `backlogit shipment ship 184-S --sha`; archived IDs 184.016-T, 184.017-T, 184.018-T, 184-F, 184-S; no returned IDs.
* PR: #555, merge commit `b826f7b55a322e93d5b079f1bd382053a0ca382c` (merge-commit strategy, P-009). Local main fast-forwarded to the merge SHA (`main == origin/main`).
* Content: docs/workflow.md "Shipment Predecessor Readiness" section; the Shipment Sequencing Protocol bullet in `.github/instructions/backlogit.instructions.md`; one appended `drift_reason` sentence in `.autoharness/harness-manifest.yaml` (`drift_allowed: true` unchanged).
* Local review: two independent shell-capable reviewers (fact-check against `internal/`; scope and integrity audit) on f89cdbe9, READY_WITH_FOLLOWUPS (P0 0, P1 0, P2 1, P3 3). Every finding was fixed in b06bc703 (the reviewed PR HEAD), so no follow-up item remains. Adversarial-review escalation was not triggered (docs and config only, no P0/P1, not security-sensitive).
* CI at b06bc703: required checks (Detect code changes, test, Docline frontmatter gate, Markdown lint) and the other checks passed. `pipeline-topology (ambient)` failed with `PREDECESSOR_NOT_SHIPPED` naming 183-S. 184-S has no dependency on 183-S; this is the known upstream defect A592FC1C (CI-pinned gate infers predecessors from ID sequence; see 183.004-T). The check is non-required and was not bypassed with admin rights.
* Copilot review of #555: reviewed b06bc703, "approval recommended", zero threads; `autoharness gate copilot-review` SATISFIED (re-run immediately before merge).
* Local gates: `go vet ./...`, `go test -timeout=30m ./... -count=1`, `go build ./cmd/backlogit`, `gofmt -l cmd internal tests` all clean. `golangci-lint` cannot run locally (toolchain export-data mismatch); CI lint is authoritative. Repo-wide `docs lint` reports 15 violations in untracked `docs/scratch/`, identical on main.

## R13 Statements (184.018-T)

* Binary: built from the merged tree at `b826f7b5`; `backlogit version` reports `1.11.1-0.20261021005436-b826f7b55a32` (the pseudo-version embeds the merge SHA; the commit field reports `unknown`).
* The core guard covers (P) only. 156-S and every other existing shipment are unchanged. The manual (P)+(C) policy from 074-DL stays in force until AF1E5075 lands and the 154-S attestation exists (it does; see the 188-S closure).

## R14 Live Impact Inventory (read-only, pre-merge)

Read-only `backlogit query` over `item_deps` joined to `items`: queued or blocked shipments with a `blocks` predecessor shipment that is `abandoned` or archived. The affected set is empty. Every archived predecessor (154-S, 186-S, 189-S, 192-S, 200-S) has Markdown `archived_status: shipped`. Queued dependents behind a still-queued predecessor are refused by the guard as intended (for example 185-S behind 194-S, 194-S behind 193-S).

## G-A Fixture Evidence

* Binding: source-bound binary above, every call passed `--cwd`. Fixture root `C:\Source\GitHub\backlogit\logs\6434a4d7-fixture\20261020175553`; the fixture storage directory was renamed from `.backlog` to `.backlogit` after init.
* Storage-root proof: `doctor` reports `storage_root=<fixture root>\.backlogit`. Negative sentinel: `149-S`, `154-S`, `169-S` return not found after init and again immediately before the claim. Positive sentinel: fixture `001-S` and `002-S` resolve, both `queued`, before the claim.
* Refusal: `shipment claim 002-S` (`blocks` 001-S, queued) exited 1 with `shipment_predecessor_not_shipped: predecessor 001-S (not_shipped:queued)`; 002-S stayed `queued`.
* An earlier fixture (`...20261020175512`) was created, seeded without the pre-seed proof, and not used for the claim.

## Releasability

READY. Docs and config only; rollback is a revert of the merge commit; owner: Ship; no runtime surface touched, so no validation window applies.

## Follow-ups

* Closure-evidence gate: `autoharness gate closure-evidence --shipment 184-S --path <this file> --json` exits 1 at `close_path` (and `close_evidence`) because the installed harness closes through the flat-manifest governed `backlogit shipment ship --sha` path, not `autoharness shipment cascade-close`, and no evidence record can be produced post-hoc without fabrication. This is the same documented condition as the 176-S and 199-S closures. The consumer predicate fields it reads for the pipeline-topology gate (`closure_status: READY`, `compaction_status: done`) are present.
* Operator approval needed to remove the two fixture directories under `logs/6434a4d7-fixture/` (Careful mode); stashed as a follow-up.
* Known open items unchanged: A592FC1C (ambient topology false positive), local `golangci-lint` toolchain mismatch.
* Source artifacts untouched: stash 6434A4D7 provenance and the deliberation `docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md`.

## Compaction (P-020)

Done: compact-context (target all) was evaluated. `docs/memory/` holds 14 files (about 63 KB), below the 40-file and 500 KB thresholds, and retained decision, plan, and closure documents still reference those memory paths, so no file was moved and nothing was compacted.
