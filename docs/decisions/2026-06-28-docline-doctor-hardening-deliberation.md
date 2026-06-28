---
chunk_strategy: h1-h2-h3
description: 'Disposition of the 2 malformed archived_from records (038-DL, 039-DL) plus grouped docline+doctor robustness hardening: apply-time TOCTOU re-read and full JSON-schema frontmatter validation'
doc_type: decision
docline:
    decision_status: decided
    depth: standard
    linked_artifacts:
        - docs/closure/2026-06-27-067-S-archived-from-integrity-closure.md
        - docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md
        - docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md
    promoted_to: none
    stash_ids:
        - 9685B1AA
        - AE53BC5C
        - B349CBED
    tags:
        - docline
        - doctor
        - archive-safety
        - hardening
ingested_at: "2026-06-28T15:24:21Z"
schema_version: "1.0"
source: docs/decisions/2026-06-28-docline-doctor-hardening-deliberation.md
title: 'docline + doctor hardening: malformed archived_from disposition + TOCTOU + JSON-schema validation'
---

## Context

Three low-priority stash entries form one cohesive "docline + doctor robustness
hardening" theme. They touch two adjacent surfaces — the docline migration/lint
engine (`internal/docline`) and the doctor archive-integrity audit
(`internal/core`) — and ship cleanly as one feature:

- `9685B1AA` (decision): permanent disposition of 2 malformed `archived_from`
  records (`038-DL`, `039-DL`, value `done`) flagged by `doctor --check-archived-from`
  every run, currently flag-only per the 067-S operator decision.
- `AE53BC5C`: docline `ApplyMigration` TOCTOU — apply writes plan-time `After`
  bytes blindly; concurrent edits between plan and apply can be clobbered (065-S L4).
- `B349CBED`: docline `ValidateFields` does field-presence only; upgrade to full
  JSON-schema validation against `schemas/docline/base-frontmatter-v1.schema.json`
  (pattern / minLength / additionalProperties) at lint time (065-S L2).

## Grounding (current code)

- `internal/docline/service.go` `ApplyMigration` (L159-194): preflight rejects
  `BodyBytesChanged`, then writes `[]byte(c.After)` via `atomicWrite`. The plan
  snapshots `Before/After` in `PlanMigration` (L137-138) — nothing re-reads disk
  at apply time, so a file edited after plan is silently overwritten.
- `internal/docline/validate.go` `ValidateFields` (L21-55): required-presence
  (`title/source/ingested_at/doc_type`) + closed-vocabulary `doc_type` only.
  No `pattern`, `minLength`, or `additionalProperties` enforcement. No JSON-schema
  library is present in `go.mod`.
- `internal/core/doctor.go`: `archived_from` malformed = present and
  `filepath.Ext(value) != ".md"` (L432). `repairArchivedFrom` (L457-519) repairs
  only `archivedFromSelfRef`; malformed is explicitly never rewritten (L466-467).
- `038-DL` / `039-DL` are `artifact_type: deliberation`, `status: archived`,
  `archived_from: done`. There are NO `.backlogit/queue/038-DL.md` / `039-DL.md`
  files — they are deliberation artifacts, never queue-routed.

## 9685B1AA disposition — option analysis

- **(a) Clear the bogus field.** archived_from is fieldless-tolerant
  (`value == ""` returns nil; no invertibility hazard). Removing the field on
  unrestorable deliberation archives is correct and eliminates the recurring noise.
- **(b) Stamp `.backlogit/queue/<id>.md` restore path.** INVALID: these `-DL`
  items have no queue restore target, so a stamped path would be a fabricated,
  unrestorable invariant — worse than blank.
- **(c) Keep flag-only + accept.** Leaves audit noise on every run permanently;
  no real safety value for non-restorable archives.

**Decision: Option (a)** — blank/remove `archived_from` on the malformed class,
delivered as a gated `doctor` repair (`--fix-malformed`, requires `--check-archived-from`)
that extends the 067 audit. Repair must be body-preserving (only the field changes,
CRLF/body untouched) like the existing self-ref repair. (b) is rejected as fabricating
a restore path; (c) is rejected as permanent noise.

## Decision

Group as one feature, ship three independent hardening tasks:
1. doctor `--fix-malformed`: clear malformed `archived_from`, repairs `038-DL`/`039-DL` (option a).
2. docline `ApplyMigration` apply-time re-read: verify on-disk `Before` still matches before write, else abort.
3. docline `ValidateFields` full JSON-schema validation against the v1 schema.

All ≤2h, single-domain (Go + tests), TDD-first.

## Open questions

- JSON-schema lib choice (santhosh-tekuri/jsonschema vs hand-rolled) — defer to impl-plan.
