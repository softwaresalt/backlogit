# Compacted Session Memory — Ship 135-S (2026-09-03)

## Scope
Shipment 135-S: Checkpoint disposition security hardening (153-F).

## Items Completed
- 153.001-T: Symlink/reparse rejection in checkpoint read verbs (O_NOFOLLOW, Windows reparse point detection, path chain checks, archive destination guards)
- 153.002-T: Sidecar create-only write (atomic temp+Link, durable fsync, double-fault priority fix)
- 153.003-T: 64 KiB size guard + multi-pass secret scan (token-stream decoded, word-boundary matching, MCP error mapping)
- 153.004-T: RemediationCommand field removal (MarshalJSON shim removed, design doc updated)
- 153.005-T: Double-refusal invariant pinned (test-only)
- 153.006-T: CLI resolve-on-abandoned coverage (test-only)

## Key Decisions
- Sidecar is create-only (not upsert): callers must not retry on ErrCheckpointDestinationOccupied blindly
- eyJ detection uses word-boundary + 20-char minimum to avoid honeyJar false positives
- Distinctive secret prefixes (ghp_, github_pat_) use Contains; collision-prone (AKIA, SG.) use word-boundary
- Platform-specific reparse point detection via Windows GetFileAttributes
- checkpointDir and archive dirs validated for symlinks before use

## PR
#408, merged at a3e26445402c6ab50619f9a9efda77ab101bf661

## Follow-up Deferred
- Stash 3F06493B: openat2 directory-FD-relative read (deeper TOCTOU hardening)
- Stash 7B71AD77: CleanupCheckpoints read-classify-then-rename TOCTOU

## Compaction Status
P-020 compact-context: invoked, 3 older files archived to docs/memory/archive/

## Next Session
136-S is now eligible for pre-claim.
