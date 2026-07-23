---
doc_type: memory
docline:
  date: 2026-07-19
  status: active
  tags: [ship, review, 099-S, 108-F, size-estimation, review-readiness]
schema_version: "1.0"
title: Ship 099-S — Local Review Readiness (108-F size estimation)
---

# Ship 099-S — Local Review Readiness

**Initial reviewed HEAD:** `eb38e2c` · **Current HEAD:** `94fa3e9` (branch `feat/108-F-size-estimation`)
**Outcome:** READY_WITH_FOLLOWUPS (initial); Copilot cycles 1–3 burned down and the task-only pivot applied — see "Cycle 1–3 burndown" below.
**Gates:** `go build ./...` ✓ · `go test ./...` ✓ (incl. contract+integration) · `go vet ./...` ✓ · `gofmt -l` (changed files) clean · `golangci-lint run` ✓

## Reviewers (cross-model, parallel)

| Persona | Model | Result |
|---|---|---|
| Constitution | claude-opus-4.8 | No P0/P1; P2/P3 hygiene |
| Go | gpt-5.6-terra | 4×P1, 3×P2 |
| Security | gpt-5.6-sol | 2×P1 (0.99/0.96), 2×P2 |
| Agent-Native Parity | gemini-3.1-pro | 3×P2, 1×P3 |
| Learnings | (default) | Confirmed decision-doc mandates (2-layer containment; whole-map caveat; F6 canonicalCache note) |

## Findings FIXED in eb38e2c (6)

1. **MCP plain-size actor=Human** (tools.go:804) — TRIPLE consensus (go P1 + security P1@0.99 + constitution P2). Agent transport forged `actor:"human"` into estimate_history via the SetArtifactSize wrapper. Fix: route plain-size through `SetArtifactSizeWithProvenance{Size, Actor: ActorContextAgent}`.
2. **findArtifact containment gap** (artifacts.go:793) — TRIPLE consensus (go P1 + security P1@0.96 + constitution P3). `findArtifact` lacked the SE-7b `ensureArtifactLookupContained` guard that `FindArtifactPath` applies; symlinked leaf could escape `.backlogit` via SizeComposition/get_item/shipment. Fix: mirror the guard in the walk.
3. **mergePreserveReservedSizingKeys off-seam injection** (artifact_size.go) — go P1 + security P2@0.9. Passed through caller-supplied reserved keys, bypassing validateSizeMutation + event. Fix: `delete(incoming,k)` then carry prior only (sole-writer integrity).
4. **get_item swallows composition errors** (tools.go:662) — go P2 + constitution P3. Fix: `slog.WarnContext` on cErr/pErr before plain-artifact fallback (read stays resilient, failures diagnosable).
5. **SizeComposition nil-ws panic** (size_composition.go) — go P2. Fix: nil-ws guard.
6. **validateSizeValue bare error** (artifact_size.go:226) — constitution P3. Fix: wrap `blerrors.ErrConfig`.

Harness safety verified before each change: no SE test pins plain-size event actor; `TestSE3a` merge subtest only tests the *omitted* case (carry-prior); `TestSE6` requires generic create to *permit* provenanced size (so C4 "reject all creates" was correctly NOT applied — tests win).

## Deferred follow-ups (create as stash entries at operational-closure)

- **[P2 residual] Generic UpdateArtifact lacks per-artifact lock** — cross-path lost-write race vs size seam. Go labeled P1; downgraded to P2 because the deployed MCP (stdio, sequential) + one-shot CLI model has no concurrent in-process writers to one artifact. Fix = serialize generic-update under the same lock (own designed task; broad blast radius).
- **[P2] Off-seam sizing hardening on CREATE** — `rejectUnprovenancedReservedSize` permits provenanced size with no enum/Rule-R validation + no event; add a validated+audited import seam. (Harness `TestSE6` requires *permit*, so this needs a designed seam, not a reject.)
- **[P2] custom_fields whole-map replacement** drops non-reserved keys — PRE-EXISTING on main (`main:artifacts.go` set `CustomFields = v`); my change only added reserved-key preservation. Ratified decision-doc caveat.
- **[P3] size_source / size_ruleset_version value-vs-enum** — seam validates presence + hardcoded `[human agent derived]`, not header-def enum (drifts if operator customizes). ruleset value intentionally presence-only (930-c1 passes non-empty "ruleset-alpha").
- **[P2/P3] CLI/MCP parity** — (a) CLI `--size` mutual-exclusivity omits gate-base/force-gates/force-reason/json; (b) MCP `size_source` desc doesn't state human-rejection; (c) `size_composition` undocumented in get_item + absent from get_shipment; (d) add reflective MCP schema→handler parity test.
- **[P3] F6 rollback canonicalCache** — frees file+DB row+seq id but not a shared `CanonicalCache` entry; bulk-create-continue edge (bulk callers typically abort on first error).

Dismissed: remove `ErrSizeEstimationNotImplemented` (harness references it via `errors.Is` — keep); defaults.go "misleading comment" (no such comment at 34-37).

## Cycle 1–3 Copilot burndown + task-only pivot

- **Cycles 1–2 (`985218b`):** original 6 Copilot threads + WDc/WDh resolved (MCP actor, findArtifact containment, off-seam key injection, composition error logging, nil-ws guard, bare-error wrap).
- **Cycle 3 (`94fa3e9`) — operator TASK-ONLY pivot:** stored/estimatable `size` is now task-only. Feature/shipment size fields removed from `defaults.go`; features/shipments expose computed-on-read rollup composition only. Composition counts task members (feature manifests expand to child tasks; the feature itself is not counted; non-task children excluded).
- **Cycle-3 findings dispositions:**
  - #1 estimate-event actor fidelity — FIXED (`appendItemEventWithActorErr`, actor stamped).
  - #2 feature/subtask sizing scope — RESOLVED by narrowing to task-only.
  - #3 `size_ruleset_version` enum-with-no-values — FIXED (enum→opaque string).
  - #4 MCP empty-size routing — FIXED (presence detection; parity with CLI).
  - #5 `findArtifact` O(N) per member — DEFER (perf backlog follow-up).
  - #6 review children in rollup — FIXED (task-only query).
  - #7 F6 rollback branch test-only — OPEN, escalated to operator (keep intentional fault-injection seam vs remove dead branch + drop F6 claim). Not resolved without operator decision.
  - #8 this memo stale — FIXED (this update).
- **Gates @ `94fa3e9`:** `go test ./...` ✓ · `go vet ./...` ✓ · `golangci-lint run` ✓ · `gofmt -l` (changed) clean.

## Next
Push `feat/108-F-size-estimation` → pr-lifecycle (PR, Copilot review §1.3-1.7, CI, §1.9 P-014 gate) → operator-approved merge-commit (P-009) → runtime-verification + operational-closure → post-merge ship_shipment 099-S + shipment-reconcile + compound-refresh + compact-context (and create the deferred stash entries then).
