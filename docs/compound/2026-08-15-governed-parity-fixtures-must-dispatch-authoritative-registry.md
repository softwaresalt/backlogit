---
chunk_strategy: h1-h2-h3
title: "Governed parity fixtures must dispatch through the authoritative registry"
date: 2026-08-15
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md
tags:
  - governed-operations
  - registry
  - parity-tests
  - mcp
---

# Governed parity fixtures must dispatch through the authoritative registry

## Problem

A parity test can appear comprehensive while bypassing the production routing
contract. Calling a shared core function directly does not prove that the
registered operation key, governed name, MCP handler, argument extraction, and
CLI mapping remain aligned.

## Durable pattern

Select governed operations from the registry, assert the exact expected
operation key, governed name, MCP tool, and CLI command, then invoke the
registered MCP handler in-process. Use fixtures that verify both durable source
artifacts and disposable projections: comments must be checked in JSONL and
the index, while dependencies must be checked in Markdown frontmatter and the
index.

## Scope rule

Keep the governed set explicit and bounded. If shipment-specific routing is not
part of the approved task, record it as a follow-up boundary instead of
weakening the authoritative dispatch rule or expanding implementation scope.

## Evidence

Feature 142-F, PR #364, merge commit `17530fe30f68034bff502362e489eff82fb86fe7`,
validated the pattern with handler-backed comment and dependency fixtures and
exact registry mapping assertions.
