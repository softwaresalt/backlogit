# Ship RUN 2 — 065-S docline migration checkpoint

Date: 2026-06-25
Branch: `feat/065-docline-migration` (off `main` @ 2a5df85b)
Shipment: 065-S (active — leave for post-merge closure)

## Completed this session

- **065.002-T (DONE, committed 4b5680eb)**: operator sign-off recorded (Q1 seed-once
  ingested_at; Q2 repo-relative POSIX source default, full-URI allowed exception).
  Decision doc gate flipped CLOSED. Task archived.
- **Phase 2a (committed 9c5f8a15)**: docline normalizer preserves a pre-existing
  full-URI `source` (Q2 nuance). Added `hasURIScheme`/`uriSchemeRE` to classify.go.
- **Phase 2b / Option A (committed 336622c2 + 3 doc batches)**: `cmd/gen-docs` now
  emits docline-compliant frontmatter (reference doc_type, repo-relative source,
  seed-once ingested_at preserved across regen). 4 new TDD tests. Regenerated 63
  cli-reference pages (born-compliant). CLI-Reference-Drift check verified stable.
- **065.009-T (DONE)**: bulk migration via `backlogit docs migrate --apply` in
  ≤25-file batches:
  - closure 41 (2 commits), compound 40 (2), exec-plans 43 (2),
    decisions+reviews+research+cli-readme 14 (1), docs-root 10 (1),
    repo-root README/AGENTS + 3 title fixes (1).
  - 3 docs lacked derivable titles (AGENTS.md, compound/feature-001, research/
    architecture-design) — seeded from H1, re-canonicalized.
  - **`docs lint` clean (0 violations)**.
  - **Global idempotency PASS**: full-tree `docs migrate` dry-run = 0 non-noop,
    0 body-byte changes (213 entries all noop).

## cli-reference decision: OPTION A (durable)
gen-docs made born-compliant so the CI gate is sustainable. Documented in decision doc.

## Remaining
- 065.010-T: CI enforcement gate (`make docs-lint` + ci.yml job) + negative smoke
  test (TDD). Then move 065.010-T → done.
- Final quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`,
  `gofmt -l` on changed Go files.
- Review gate, PR lifecycle, Copilot review resolution. HALT for operator merge.

## Notes
- backlogit.exe rebuilt from branch HEAD (binary == in-repo codec).
- autocrlf=true: working tree CRLF, committed blobs LF (CI on Linux clean).
- Migration never changed body bytes (verified per-batch + global).
