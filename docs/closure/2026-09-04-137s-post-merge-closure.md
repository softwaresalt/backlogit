---
schema_version: "1.0"
doc_type: closure
title: "Post-merge closure: 137-S CLI/MCP parity"
shipment: 137-S
feature: 155-F
merge_commit: "967a1bf449d6dc3ce61512ab4fc0dc3358f3d782"
merge_pr: 420
closed_at: "2026-09-04T19:35:00-07:00"
releasability: READY
compaction_status: done
---

# Post-Merge Closure: 137-S — S3 CLI/MCP Parity

## Scope Delivered

| Unit | Task | Description |
|------|------|-------------|
| U1a | 155.001-T | Transport-neutral bounded error DTO/builder; BoundedFields() + bounded Error() + MCP 3 scalars |
| U1b | 155.005-T | CLI JSON-RPC WrapErrorData + Data field + typed-nil guard |
| U2  | 155.002-T | CLI errors.As checkpoint structured envelope in Execute() |
| U3  | 155.003-T | backlogit_docs_classify MCP tool + ValidateClassifyPath shared helper |
| U4  | 155.004-T | create_checkpoint governed:true + fixture + design doc |

## Runtime Verification Evidence

- **Test suite**: All packages PASS (go test ./... exit 0)
- **CLI Reference Drift**: PASS (gen-docs updated for new Short description)
- **Docline frontmatter gate**: PASS
- **Markdown lint (P-008)**: PASS
- **pipeline-topology (ambient)**: PASS
- **test CI**: PASS (4m34s)
- **Local build**: go build ./cmd/backlogit exit 0

Surface adapters verified:
- MCP backlogit_docs_classify: path=REQUIRED, validates via ValidateClassifyPath, returns {doc_type}
- MCP checkpointUnknownFieldsResponse: 3 non-omitempty scalars always present
- CLI Execute(): errors.As CheckpointUnknownFieldError → WrapErrorData with bounded data
- CLI backlogit docs classify: emits JSON {doc_type} (jsonrpc-compatible shape)
- docline.ValidateClassifyPath: rejects empty/absolute/volume/UNC/dot-segment on both surfaces

## Releasability Assessment

- **Verdict**: READY
- **Backward compatibility**: All changes additive; WrapError byte-identical; legacy errors unaffected
- **Breaking changes**: None
- **Rollback**: Remove BoundedFields() call in checkpointUnknownFields → reverts to unbounded response; registry governed marker removable
- **Owner**: softwaresalt/backlogit team
- **Validation window**: 24h post-merge observation; no production traffic concerns (internal tool)
- **Monitoring**: Go test suite; CI pipeline

## Archive Members

Archived by backlogit_ship_shipment: 155-F, 155.001-T through 155.005-T, 062-DL (deliberation), 137-S

## Source Artifact Cleanup

- 062-DL: Archived as shipment manifest member (no separate stash_id to retire)
- 155-F: No source_stash_id or source_deliberation_id in custom_fields

## Deferred Follow-Ups (stash entries)

- 84918E2E: CLI -32000 code for validation errors (pre-existing pattern, needs deliberation)
- 9C3648AF: ValidateClassifyPath → ErrPathEscapesWorkspace wrapping
- C62BD3CF: BoundedFields/BoundedFieldPaths naming asymmetry
- 7074AF4E: shared validateRelativePath helper for docline
- 48FC866D: wave-scheduler fixture staleness for completed 130-S

## Compaction Status

pending (will be updated after compact-context invocation)

