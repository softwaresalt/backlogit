---
description: "Graphtor-docs workflow rules for indexed local documentation search, semantic retrieval, and doc-graph traversal"
applyTo: '**'
---

# Graphtor-Docs Instructions

Use these rules when the workspace has enabled the `graphtor-docs` capability pack. This pack is not
a generic documentation search hint. It weaves indexed local documentation retrieval through the
harness workflow so agents resolve domain concepts and referenced APIs from indexed sources before
resorting to broad web search or raw filesystem scan.

When any retrieval-enforced pack is enabled, `capability-pack-enforcement.instructions.md` is the
coordinator that routes retrieval across packs before any raw grep/glob or public web search. It
defers pack-specific mechanics to this file: use graphtor-docs for documentation and API concepts.
Honor that coordinator's safeguards (pack deferral, direct-search exemptions, per-phase health reuse,
internal-first / no-public-web) in addition to the rules below.

## Workspace Usage Rule (NON-NEGOTIABLE)

Inside this workspace graphtor-docs is used **exclusively in a read-only manner**, and it
**never indexes this workspace**.

graphtor-docs performs an index sync only under specific conditions, against raw content that has
been curated in advance for ingestion. A workspace is not such a corpus. Therefore, in this
workspace:

* **Never** run `graphtor-docs sync`, or any other ingestion or index-write operation.
* **Never** register this repository — including `docs/` — as a source in
  `.graphtor/config/sources.yaml`. That file stays empty here.
* Index sync is never part of harness install, tune, or pack enablement.
* The MCP server is registered with `serve --read-only`, which forces every database to ReadOnly
  posture regardless of resolved sources.

Only the read verbs in the tool surface below are in scope. If no curated corpus is bound, retrieval
degradation is the expected and correct outcome — take the fallback path in the Fallback Protocol
below and note reduced confidence. It is never a reason to index the workspace.

## Required Tool Surface

The workspace exposes an MCP tool surface from the graphtor-docs server. Tool names are registered
through MCP configuration (`.mcp.json`, `.vscode/mcp.json`, or editor settings) — not through the
source-index configuration. The canonical tool surface includes:

| Tool | Purpose |
|---|---|
| `search_local_docs` | Keyword search across all indexed local documentation sources |
| `search_semantic` | Vector-similarity search for conceptual or natural-language queries |
| `research_topic` | Multi-source topic research that combines keyword and semantic results |
| `traverse_doc_links` | Follow hyperlinks within indexed documents to trace related content |
| `list_sources` | List all indexed documentation sources and their index status |
| `get_chunk_by_id` | Retrieve a specific indexed chunk by its stable identifier |
| `get_document` | Retrieve the full indexed content of a document by path or URL |
| `get_status` | Check graphtor-docs server health and index freshness |

Do not bypass indexed search by defaulting immediately to browser search or raw `grep` over
`docs/` when the graphtor-docs server is reachable and sources are indexed.

## Server Lifecycle Protocol

Before relying on graphtor-docs results:

1. Call `get_status` to verify the server is reachable and the index is fresh.
2. If the status reports index staleness, note it but do not halt — stale results are usable with
   reduced confidence.
3. If the server is unreachable, fall back to the Fallback Protocol below and log
   `GRAPHTOR_UNAVAILABLE`.

Do not call `get_status` on every individual tool invocation. Check once per major workflow phase
or when results appear incomplete.

## Search Protocol

Use the most specific graphtor-docs tool first:

| Need | Preferred Tool |
|---|---|
| Keyword lookup in docs | `search_local_docs` |
| Conceptual or natural-language query | `search_semantic` |
| Multi-angle topic research | `research_topic` |
| Follow a doc hyperlink chain | `traverse_doc_links` |
| Know what sources are indexed | `list_sources` |
| Retrieve a specific chunk by id | `get_chunk_by_id` |
| Retrieve a full document | `get_document` |

Prefer these before file-based fallback whenever the question is conceptual, API-oriented, or
relates to documentation content rather than code structure.

## Fallback Protocol

Fall back to `grep`, glob, or direct file reading only when:

* the graphtor-docs server is unavailable (`GRAPHTOR_UNAVAILABLE`)
* no indexed sources cover the area being queried (confirm via `list_sources`)
* the query is literal-text or regex-oriented rather than conceptual
* you already know the exact file path and need line-level confirmation

If semantic search returns no results, try `search_local_docs` before falling back to filesystem
scan. Do not retry the same broad semantic query more than twice.

## Source Configuration

Documentation sources are configured at `.graphtor/config/sources.yaml`. This file declares which
local paths, Git repositories, and URLs graphtor-docs indexes. Agents must not modify this file in
this workspace — see the Workspace Usage Rule above. It is intentionally empty, and binding a
curated corpus is an operator decision made outside the harness.

When the binary is not on PATH, it may be found at `.graphtor/bin/graphtor-docs.exe`.

### Reproducible MCP Registration

Registration is machine-local and deliberately not committed (`.graphtor/` is
gitignored). To make the read-only tool surface callable, register the server in
your editor's MCP configuration (`.vscode/mcp.json`, `.mcp.json`, or editor
settings):

```json
{
  "servers": {
    "graphtor-docs": {
      "command": "graphtor-docs",
      "args": ["serve", "--read-only"]
    }
  }
}
```

Replace `"graphtor-docs"` with `.graphtor/bin/graphtor-docs.exe` when the binary
is not on PATH. The `--read-only` flag is mandatory — it forces every database to
ReadOnly posture and is what makes the Workspace Usage Rule enforceable rather
than merely advisory.

The eight read verbs in the tool surface table above are granted to `_stage`,
`_ship`, and `_orchestrator` by name in their `tools:` allowlists. They are
enumerated individually rather than granted as a `graphtor-docs/*` wildcard so
that no ingestion or index-write verb is ever reachable from an agent, even if
the server later registers one.

Until this registration exists, the pack is enabled but its tools are not
callable, and agents will correctly report `GRAPHTOR_UNAVAILABLE` and take the
Fallback Protocol path.

The local sentence-transformer embedding model used for offline vector search is
configured via the `GRAPHTOR_EMBED_MODEL_DIR` environment variable, set in the
workspace-root `.env.local` and defaulting to `.graphtor/models/all-MiniLM-L6-v2`.

## Data Ownership Rule

Treat `.graphtor/` artifacts as tool-managed state. Do not hand-edit index databases, cached
embeddings, or chunk registries. `.graphtor/` is entirely gitignored and machine-local; no part of
it is committed from this workspace.

Generated by autoharness | Template: graphtor-docs.instructions.md.tmpl
