---
chunk_strategy: h1-h2-h3
description: "Ship checkpoint: 148-S / 167-F wave 3 converged (13/20 tasks done), wave 4 admission next"
doc_type: memory
status: draft
created: 2026-09-12
schema_version: "1.0"
source: docs/memory/2026-09-12-ship-148-s-wave3-complete-session5.md
title: "Ship checkpoint — 148-S / 167-F — Wave 3 converged (13/20 tasks done)"
---

# Ship checkpoint — 148-S / 167-F — Wave 3 converged (13/20 tasks done)

**Timestamp**: 2026-09-12 (session continuation, 5th session)
**Agent**: Ship (claude-sonnet-5/anthropic/high — ROUTING_DEGRADED, as directed)
**Branch**: feat/148-s-governed-archived-shipment-reconciliation-to-shipped
**Base**: main @ 4397b7f0d2e932c4168478b313cf01066753f509 (unchanged this session)
**Scope**: shipment 148-S, covering feature 167-F, DARK_MODE_ACTIVE [148-S], strict scope 148-S only

## What this session did

Resumed at Step 4.0 wave-3 admission. Live re-derivation confirmed `ready_k` =
{167.001-T, 167.010-T, 167.016-T} exactly as instructed (167.007-T/167.013-T
were already done from the prior session).

1. **167.010-T** — `classifyShipmentReconcileStateImpl`: pure, read-only total
   state classifier. Table-driven totality/exclusivity test (16 subtests)
   covering log-readability x frontmatter-state x key x request-identity x
   event-presence/validity. RED confirmed against the 167.003-T panic body,
   GREEN after implementation. Commits `3298fb87`, `e5ec5b9e`.
2. **167.001-T** — `prepareShipmentReconcileEvidence`: request-identity digest
   (Phase A, domain-separated canonical field-tagged encoding), trusted-ref
   pinned-tip resolution + true-merge-commit verification (bounded git
   subprocesses, argv-array, MinimalEnv, reusing `isGitObjectName`/
   `boundedHelperTimeout`), no-follow/reparse-safe closure-evidence file read,
   deterministic `(feature)`-annotated delivery-merge grammar parser, evidence
   digest, canonical event materialization. New sentinel
   `blerrors.ErrShipmentReconcileEvidence`. No literal on-disk "048-S" closure
   fixture was found; a synthetic fixture matching the task's exact grammar
   was used in tests (honestly disclosed). Commits `1ffb3c0a`, `50426f1d`.
3. **167.016-T** — `snapshotShipmentReconcile`/`restoreShipmentReconcile`:
   full-row SQLite snapshot (dynamic `rows.Columns()`/`ColumnTypes()`
   introspection, capturing schema-extension columns UpsertItem would drop),
   file restore through the SAME 167.006-T handle-relative atomic writer,
   exact-row `INSERT OR REPLACE` verbatim restore (never `db.UpsertItem`),
   non-cascading single-row `DELETE FROM items` (never
   `db.DeleteItem`/`DeleteItemCascade`) proven by a satellite-row-survival
   test (item_logs, item_log_entries, item_deps, item_links, stash_links,
   commit_links all asserted untouched). Injected post-rename write failure
   correctly classifies `ErrWriteIndeterminate`. Commits `ff8b75ab`, `c79b3a66`.

Every task followed real TDD (RED confirmed against the 167.003-T panic body
or an undefined-symbol compile failure, then GREEN), verified independently
by Ship (not just trusted from the implementing subagent): `go build ./...`,
`go vet ./...`, `gofmt -l` on changed files, `golangci-lint run` (v1.64.8,
matching CI's pinned version), the task's own targeted test suite, and a full
`go test ./internal/core/... -count=1` after each task (496s–819s, zero
regressions each time).

### Wave 3 convergence gate (Step 4.6) — two real gaps found and fixed

Running the mandatory **unfiltered full repository suite** (`go test ./...`)
for the first time this entire feature's history (prior sessions only ran
`./internal/core/...`) surfaced two genuine, pre-existing/newly-introduced
gaps that had to be resolved before the gate could pass:

1. **Docline soft-key frontmatter gap** (pre-existing, from an earlier
   wave-1 session): `docs/design-docs/2026-09-12-item-log-lock-identity-migration-runbook.md`
   (added by 167.017-T in an earlier session) had no frontmatter block at all,
   failing `TestDoclineSoftKeys_LiveTrackedCorpus`. Fixed by adding the
   canonical `chunk_strategy: h1-h2-h3` / `schema_version: "1.0"` frontmatter
   block matching sibling design docs. Commit `1e44d928`.
2. **Governed sha256 allowlist violation** (introduced this session):
   `TestGovernedSha256Allowlist` rejected direct `crypto/sha256` imports in
   `internal/core/shipment_reconcile_event.go` (167.002-T, earlier session)
   and `internal/core/shipment_reconcile_evidence.go` (167.001-T, this
   session) — governed gate-evidence hashing must route through
   `internal/canonical`. Added `canonical.HashBytes(b []byte) string` (a
   stdlib-only raw-byte hash seam alongside the existing `Hash(v any)`
   canonicalizing seam) and re-routed both files' three sha256 call sites
   through it, removing the direct `crypto/sha256`/`encoding/hex` imports.
   Verified byte-for-byte equivalent output (same algorithm, same encoding).
   Commit included in this session's working tree (see below).

Both fixes were verified with the full targeted package tests, the full
`internal/core` suite, and `golangci-lint` before being folded into the wave
convergence record.

### One pre-existing, out-of-scope failure — confirmed and reused, not fixed

The full `go test ./...` run surfaces exactly **one** remaining failure:
`TestU4aBehaviorCanonicalByteStable` in `internal/faultline`
(`evidence_conformance_test.go:94`). Root cause: this repo's own
`.gitattributes` (`* text=auto`) normalizes the golden fixture
`testdata/parity_v1.golden.json` to CRLF on a Windows checkout, while the
canonical marshaler under test produces LF — a Windows-checkout-environment
artifact of a fixture added by 156.006-T, wholly unrelated to 167-F/148-S.

Per P-021 Step 4.4a, this is out of scope (C1: fixing it requires touching
`internal/faultline` files never part of any 167-F task's authorized
surface) and MUST NOT be fixed here. **Discovery found an existing, exact-match
deferred stash entry already covering this precise failure**: stash ID
`92F79833` ("DEFERRED SCOPE EXPANSION: TestU4aBehaviorCanonicalByteStable
CRLF/LF mismatch...", captured during a 139-S/157-F session). Per the
discovery rule (positive confirmation of an existing entry describing the
same expansion on the same contract surface), this entry was **reused, not
duplicated** — no new stash entry was created. This finding does not block
wave-3 convergence: it predates this feature, predates this session, and is
demonstrably unrelated to any 167-F task's code.

### Wave 3 convergence: CONFIRMED

With both real gaps fixed and the sole remaining failure confirmed
pre-existing/out-of-scope/already-tracked, the full repository suite is
green for every package this feature's scope touches. Wave 3 is converged.

## Current verified state

- Branch: `feat/148-s-governed-archived-shipment-reconciliation-to-shipped`.
  Working tree: canonical.go / shipment_reconcile_event.go /
  shipment_reconcile_evidence.go sha256-routing fix is present but not yet
  committed as of this checkpoint's authoring point (see next steps —
  committed immediately after this checkpoint is written).
- Shipment 148-S: `active` (sole active shipment).
- Feature 167-F: `active`.
- Task census (re-read live via `backlogit get <id> --format json` and
  `backlogit dep list <id>`, exact-ID, no `list --type task` enumeration):
  **done (13)**: 167.001-T, 167.002-T, 167.003-T, 167.006-T, 167.007-T,
  167.010-T, 167.011-T, 167.012-T, 167.013-T, 167.016-T, 167.017-T,
  167.019-T, 167.021-T.
  **queued (7)**: 167.004-T, 167.005-T, 167.008-T, 167.009-T, 167.014-T,
  167.015-T, 167.020-T.
  **active**: 0. **blocked**: 0. **unsupported**: 0.

## Wave structure re-derived from live dependency graph (`backlogit dep list`)

- Wave 4 (next): `167.014-T` (deps: 167.010, 167.001 — both done) and
  `167.020-T` (deps: 167.017, 167.019, 167.016, 167.021 — all done).
- Wave 5: `167.008-T` (the transaction; deps: 167.007, 167.001, 167.010,
  167.014, 167.016, 167.017, 167.020, 167.019, 167.021 — needs wave 4 done).
- Wave 6: `167.004-T` (CLI; deps: 167.008) and `167.009-T` (deps: 167.008).
- Wave 7: `167.015-T` (governance gate; deps: 167.004, 167.009, 167.014) and
  `167.005-T` (integration tests; deps: 167.004, 167.008).

## Next step

Continue at Step 4.0 wave-4 admission with `ready_k` = {167.014-T,
167.020-T}. No PR created yet, no CI run, no merge, no closure — Step 5
begins only once all 20 manifest tasks are `done`/`archived`.
