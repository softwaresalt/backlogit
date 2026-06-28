---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for grouped docline+doctor hardening: doctor --fix-malformed clears the 2 malformed archived_from records, docline ApplyMigration apply-time TOCTOU re-read, and full JSON-schema frontmatter validation'
doc_type: plan
ingested_at: "2026-06-28T15:24:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-06-28-docline-doctor-hardening-plan.md
title: 'docline + doctor hardening: fix-malformed archived_from, apply-time TOCTOU re-read, JSON-schema validation'
---

## Source

- Deliberation: `docs/decisions/2026-06-28-docline-doctor-hardening-deliberation.md` (Decision: option (a) + group 3 tasks)
- Stash: `9685B1AA`, `AE53BC5C`, `B349CBED`
- Prior art: `docs/closure/2026-06-27-067-S-archived-from-integrity-closure.md`, `docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md`

## Requires plan hardening: no

Three independent, narrow tasks. No destructive bulk migration, no schema break,
no concurrency primitive. Repair is gated and body-preserving (precedent exists).
Blast radius is low; no hardening section required.

## Constitution Check

- 2-hour rule: each task <3 files, <5 functions, <4 test scenarios. PASS.
- Width isolation: each task is Go-code-only (impl + tests). PASS.
- TDD-first: every task adds failing tests before impl. PASS.
- P-009: merge commit, no self-merge. Honored at landing.
- Body-preserving repair (only target field changes) — direct 067 precedent. PASS.

## Tasks

### T1 — doctor --fix-malformed clears malformed archived_from (option a)
Files: `internal/core/doctor.go`, `cmd/.../doctor.go` flag, `internal/core/doctor_test.go`.
- Add `FixMalformed bool` to DoctorOptions; gate `--fix-malformed` behind `--check-archived-from`.
- Extend `repairArchivedFrom` (or sibling) to handle `archivedFromMalformed`: clear field via
  body-preserving rewrite helper (blank field), emit FixAction. Self-ref path unchanged.
- Repairs `038-DL`/`039-DL`; re-run audit = 0 malformed.
AC: test malformed cleared, body bytes preserved, --fix requires --check, self-ref untouched.

### T2 — docline ApplyMigration apply-time re-read (TOCTOU)
Files: `internal/docline/service.go`, `internal/docline/service_test.go`.
- Before write, re-read `w.abs`; if on-disk bytes != change `Before`, abort with new sentinel `ErrConcurrentEdit`; zero writes.
AC: test concurrent-edit aborts all writes; unchanged file applies normally; idempotent re-apply still no-op.

### T3 — ValidateFields full JSON-schema validation
Files: `internal/docline/validate.go`, `go.mod`, `internal/docline/validate_test.go`.
- Embed `base-frontmatter-v1.schema.json`; validate pattern/minLength/additionalProperties; map failures to Violations. Presence + doc_type vocab preserved.
AC: pattern/minLength/additionalProperties violations reported; valid frontmatter passes; existing tests green.

## Dependencies
T1 → T2 → T3 sequenced for one clean PR; technically independent.

## Verification
go build ./...; go test ./...; backlogit docs lint; doctor --check-archived-from = 0 malformed.
