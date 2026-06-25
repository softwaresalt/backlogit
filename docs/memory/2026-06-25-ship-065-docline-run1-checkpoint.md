---
title: "Ship session — 065-S docline frontmatter (run 1, tooling stack)"
doc_type: closure
date: 2026-06-25
---

# Ship Session Checkpoint — 065-S (run 1)

Branch: `feat/065-docline-frontmatter` (base `5b34ed1d` on main).
Shipment 065-S claimed (active). Goal: build the non-gated tooling stack,
take PR to merge-ready, HALT for operator sign-off on Q1/Q2 + merge approval.
Gated/deferred to run 2: 065.002-T (policy gate), 065.009-T (bulk migrate),
065.010-T (CI gate).

## Tasks completed (done + commit-associated)
- [x] 065.001-T — taxonomy/field-mapping decision doc — `32a03057`
- [x] 065.003-T — body-preserving codec (`internal/docline/codec.go`) — `023d1bce`
- [x] 065.004-T — policy + BaseFrontmatter model + validator — `df3087a5`
- [x] 065.005-T — classifier + idempotent normalizer — `07bfc6b4`
- [x] 065.006-T — application service (lint/plan/apply) — `c0dac99e`

## Remaining this run
- [ ] 065.007-T — `backlogit docs` CLI adapter
- [ ] 065.008-T — MCP parity tools
- [ ] 065.011-T — authoring guide + ARCHITECTURE/AGENTS update
- [ ] review gate → PR → CI green → Copilot threads → HALT

## Key invariants / lessons
- Backlog mutations STRICTLY sequential (SQLite lock deadlock if parallel):
  commit code → `update <id> --commit <sha>` → `move <id> --status done` → commit `.backlogit/`.
- gofmt gate: `git show :<file> | gofmt -d` on staged LF blobs (NOT `gofmt -l .`
  which is polluted by autocrlf CRLF noise).
- Per-package gates: `go test/vet ./internal/docline/...`, `golangci-lint run ./internal/docline/...`.
- `unused` linter: each task's new code must be referenced by its own test.

## Operator gate to surface at HALT (do NOT self-answer)
- Q1 `ingested_at`: recommend **seed-once at migration** (normalizer preserves
  existing value; idempotent).
- Q2 `source`: recommend **repo-relative POSIX path** (graphtor-docs compatible).
