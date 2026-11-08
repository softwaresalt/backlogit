---
title: "212-S six owned receipts canonicalized to LF with original-byte preservation"
doc_type: "memory"
schema_version: "1.0"
recorded_at_utc: "2026-11-07T07:05:06.1435569Z"
shipment_id: "212-S"
feature_id: "211-F"
head: "d6b23ed3efd6e021486fb4b857defaa1e9914299"
normalization_provenance: "bin/_e7d7bed1/provenance.json"
normalization_provenance_sha256: "308233ad0b1887025d04fb6152136c6364eef2b901eb529b757203095c643a7f"
raw_backup_root: "bin/_e7d7bed1/raw"
targeted_native_exit: 0
historical_convergence_failures: 5
unfiltered_gate_discharged: false
next_convergence_authorized: false
compaction: "Scoped live-continuity safe no-op; nothing archived"
---

## Exact authorized correction

Corrected only the six JSON receipts in
`docs/memory/2026-11-06/212-S-R3-stash-busy-a91/`:
`quality-receipts`, `red-receipt`, `preservation`, `green-receipt`,
`strict-green-receipt`, `strict-green2-receipt`.

Every original SHA256 was checked against the fifth-failure record, and all
six original files were copied byte-for-byte to exclusive new ignored backups
**before any working-file write**. Only CRLF->LF byte formatting followed;
there was no JSON reserialization or value change.

The provenance file records original/canonical SHA256, byte counts, CRLF/CR/LF
counts, JSON equality, Git blob IDs and byte comparisons. All six canonical
files exactly match both committed HEAD and index blobs. No index refresh,
stage/commit or source change was necessary. Six stat-only M flags were
observed despite blob equality; `git diff --numstat` has no content delta.

## Truthful historical hash references

Original raw files are at
`bin/_e7d7bed1/raw/<original filename>.json`.
Old recorded SHA256/CRLF counts describe these preserved original bytes,
**not** the canonical working copies. Current hashes resolve through
`provenance.json` records' `canonical_sha256`.

The failure checkpoint's original complete bytes are preserved at
`bin/_e7d7bed1/failure-checkpoint-before-reference-update.json`,
SHA256 `b0e1681636ebb6c9e3bb8daae0c5e6aa533caeb62f68859d1f1d00fddaf4518b`.
Its live memory copy now appends an explicit post-failure normalization event
and raw-backup mapping; its original failure evidence/outcome is not revised.
Other historical snapshot manifests remain capture-time records, with the
same original-hash resolution through this normalization provenance.

## One focused verification

Executed only:
`go test -count=1 -timeout=30m -json -run '^TestU175_001_BaselineEol$' ./tests`
with pinned cached offline Go 1.24, readonly modules and fresh short TEMP.

Native exit **0**; exactly **one named RUN/PASS**, zero FAIL/SKIP, no parse
errors. The baseline reported
`checked=1661 excluded=0 index-not-lf=0 work-not-lf=0 w/mixed=0`.

* Strict receipt: `bin/_e7d7bed1/strict-receipt.json`,
  SHA256 `2b548fbd34a28328ee672a425c018b79ac8199fa9d610bcdda9d3b7f50afad2e`.
* Native receipt: `bin/_e7d7bed1/native-result.json`,
  SHA256 `435554f3284ae69b31d2b8b85843cbf213188d6049dda00b6c3e8bf68f428705`.
* Native streams: `bin/_e7d7bed1/go-test.stdout.raw` and
  `bin/_e7d7bed1/go-test.stderr.raw`.

This focused PASS does **not** combine with the fifth full run's native 1 to
create an unfiltered PASS. All five convergence failures remain.
The known owned encoding defect is now corrected; request authorization for
**one next unfiltered current-wave convergence cycle**, without resetting
history. No sixth-cycle token or permission was created here.

## Preserved scope and continuity

No production/harness assertion, raw Go evidence, other receipt, official
checkpoint 210317, backlog/archive/DB state, configuration, attributes or test
exclusion changed. All original bytes remain available. Source HEAD remains
d6b23ed, M12/six done and U2-U7 claim-assigned/unstarted; no push/PR/merge.

Scoped compact-context assessment protects live gate history, latest receipts
and the unresolved cursor: safe no-op, no archives/moves and no other release
unit touched. Metadata-only byte/semantic checks suffice for this encoding
correction; no broad new code audit or unfiltered run was performed.
