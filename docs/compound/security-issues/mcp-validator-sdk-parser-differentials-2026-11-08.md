---
title: "Match MCP validation to the SDK argument parser"
description: "Avoid case-folded and duplicate-key shadowing between containment guards and MCP dispatch."
problem_type: "parser-differential"
category: "security-issues"
component: "internal/mcp/session_mux.go"
root_cause: "Typed guard decoding and exact-key SDK maps interpreted one JSON frame differently."
resolution_type: "code_fix"
severity: "high"
message: "Containment guard validates a different path from the tool handler."
file_path: "internal/mcp/session_mux.go"
citations:
  - "docs/memory/2026-11-08/212-S-M19-wave6-u3-resume.md"
  - ".backlogit/archive/211.003-T.md"
tags:
  - "mcp"
  - "containment"
  - "json"
  - "parser-differential"
---

## Problem

A stream-level workspace guard could approve an empty or safe path while
the SDK-dispatched documentation tool consumed a junction-escaping path.
Ordinary traversal tests were green; mixed-case argument names exposed
the mismatch during review.

## Root Cause

Go's typed JSON decoding matches field names case-insensitively. The MCP
SDK's argument map uses exact keys. A frame carrying both `path` and
`Path` could therefore give the validator and handler different values.
Duplicate keys introduce another ambiguity when independently decoded
objects merge or replace values differently.

## Resolution

Validate the actual exact-key map consumed by dispatch and reject
ambiguous duplicate-key frames. Preserve protocol-envelope validation
ordering before tool-specific validation. Verify with a real handler
and a workspace-contained junction fixture, not only a synthetic guard.

## Prevention

Treat every validator/consumer parser pair as a trust boundary. Include
case variants, duplicate objects, nulls and malformed envelopes in
focused verification. Never infer containment from lexical path checks
when the final handler can follow filesystem links.
