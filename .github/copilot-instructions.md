# backlogit Development Guidelines

Last updated: 2026-05-11

backlogit is a Go CLI tool and MCP server for structured backlog management.

## Technology Stack

| Layer           | Technology                | Notes                                 |
|-----------------|---------------------------|---------------------------------------|
| Language        | Go 1.24.0 | Module: github.com/softwaresalt/backlogit          |
| Build           | go            | `go build ./cmd/backlogit`                   |
| Test            | go test           | `go test ./...`                    |
| Lint            | golangci-lint                | `golangci-lint run`                    |
| Format          | gofmt             | `gofmt -l .`                  |
| CI              | GitHub Actions           | Runs on push and pull_request to main                          |
| MCP SDK | github.com/mark3labs/mcp-go v0.27.0 (stdio transport) |
| CLI Framework | github.com/spf13/cobra v1.10.2 |
| Validation | github.com/go-playground/validator/v10 v10.30.1 |
| Testing | github.com/stretchr/testify v1.9.0 |
| SQLite | modernc.org/sqlite (pure Go, CGo-free) |

## Project Structure

```text
```text
backlogit/
  cmd/backlogit/         # CLI entry point
  internal/
    cli/                 # CLI commands
    core/                # Core domain logic
    db/                  # SQLite database layer
    mcp/                 # MCP server and tools
    models/              # Domain models
    telemetry/           # Telemetry harvesting
  tests/                 # Integration tests
  docs/                  # Durable knowledge
  .backlogit/            # Backlog workspace
  .github/               # Agents, skills, instructions
```
```

## Commands

```bash
go build ./cmd/backlogit              # Build
go test ./...               # Run all tests
golangci-lint run               # Lint
gofmt -l .             # Format check
| `backlogit mcp` | Start MCP server (stdio transport) |
| `backlogit add` | Create a new backlog item |
| `backlogit list` | List backlog items |
| `backlogit query` | Run SQL queries against the index |
| `backlogit sync` | Synchronize SQLite index from Markdown |
| `backlogit queue view` | View prioritized work queue |
```

## Code Style and Conventions

### Error Handling

- Wrap all errors with `fmt.Errorf("context: %w", err)`
- Use sentinel errors for known failure modes (`var ErrNotFound = errors.New(...)`)
- Use typed errors for domain-specific categories
- Never ignore errors — handle or explicitly discard with `_ =`

### Naming

- Use `camelCase` for unexported identifiers, `PascalCase` for exported
- Acronyms are all-caps: `ID`, `HTTP`, `URL`, `SQL`, `MCP`
- Interface names: single-method interfaces use `-er` suffix (`Reader`, `Writer`)
- Package names: short, lowercase, no underscores (`mcp`, `db`, `cli`)
- Test files: `*_test.go` colocated with source

### Documentation

- Every exported function, type, and constant has a doc comment starting with the identifier name
- Package-level doc comments in `doc.go` for non-trivial packages
- Use `// Deprecated:` for deprecated APIs
- Examples in `*_test.go` files with `Example` prefix

### Testing

* TDD required: write tests first, verify they fail, then implement
* Test tiers in `tests/` directory:
- **Unit tests**: `*_test.go` colocated with source, `go test ./internal/...`
- **Integration tests**: `tests/` directory, `go test ./tests/...`
- All tests use `testify/assert` and `testify/require`
- Table-driven tests with `t.Run` subtests are the standard pattern

## Search Strategy

Use available workspace search tools before falling back to file-based search
(grep, glob, view). Indexed search returns precise results with minimal token
cost. File-based tools read raw content into the context window, consuming
tokens proportional to file size.

**Search tool preference order:**

1. When the `agent-engram` capability pack is enabled and reachable: `unified_search`, `query_memory`, `map_code`, `list_symbols`, `impact_analysis`, `query_graph`
2. Otherwise use workspace-indexed tools (if available): semantic search, symbol lookup, call graphs
3. File-based fallback: grep, glob, view — only when indexed results are insufficient

## Durable Knowledge Layout

| Path | Purpose |
|---|---|
| `docs/compound/` | Reusable learnings and hard-won fixes |
| `docs/exec-plans/` | Implementation plans |
| `docs/decisions/` | Durable decisions and investigation outputs |
| `docs/memory/` | Session memory and checkpoints |
| `docs/closure/` | Review, runtime verification, and closure artifacts |
| `docs/design-docs/` | Graduated architecture and design rationale |
| `docs/product-specs/` | Product-oriented requirements |

## Session Memory Requirements

* Working agent sessions MUST persist output to `docs/memory/` before the session ends — do NOT rely on built-in AI assistant memory features, which write to their own managed locations.
* When the context window reaches approximately 65% capacity, checkpoint current work before continuing.
* For long sessions, save memory checkpoints after completing each phase or major task group.
* Content to capture: task IDs completed, files modified, decisions and rationale, failed approaches, open questions, and next steps.
* File convention: `docs/memory/{YYYY-MM-DD}/{descriptive-slug}-memory.md`
* After writing memory, invoke the **compact-context** skill to consolidate stale checkpoints and finalize decided-plans. This is a mandatory workflow step, not advisory.
* If context has grown from loading multiple skill definitions mid-session, consider invoking **compact-context** proactively before hitting hard thresholds.

## Foundational Protocols

| Protocol | Location | When |
|---|---|---|
| **Circuit Breaker** | `.github/instructions/circuit-breaker.instructions.md` | All retry loops and failure handling |
| **Concurrency Control** | `.github/instructions/concurrency.instructions.md` | Multi-agent or human+agent concurrent edits |
| **Skill Discovery** | `scripts/search.ps1` / `scripts/search.sh` | Finding capabilities by keyword (Primitive 6) |

## Optional Capability Packs

### agent-intercom

When the workspace enabled the `agent-intercom` capability pack:

* verify the intercom server / tool surface is reachable before depending on remote approval or operator steering
* call heartbeat / ping at session start and keep it alive during long-running work
* broadcast major workflow transitions so the operator can observe planning, build, review, verification, and closure progress
* route destructive terminal commands and destructive file operations through the intercom approval workflow
* use transmit / standby flows when blocked on operator clarification or when intentionally pausing for instructions
* if the intercom service is unreachable, warn that remote visibility is degraded and avoid pretending approval or operator awareness exists

### agent-engram

When the workspace enabled the `agent-engram` capability pack:

* verify the engram daemon / MCP surface is reachable before depending on indexed lookup
* prefer engram tools for conceptual search, symbol discovery, call-graph lookup, impact analysis, and workspace-memory retrieval
* verify the workspace binding state before relying on results; if the daemon auto-binds the workspace, prefer status checks over repeated rebinding
* use `sync_workspace` or the equivalent freshness operation when code changed outside the expected indexing flow
* if semantic search is unavailable or degraded, fall back to `list_symbols` + `map_code` + `impact_analysis` before resorting to broad file scans
* treat `.engram/` generated artifacts as tool-managed state rather than files to hand-edit casually

### backlogit

When the workspace enabled the `backlogit` capability pack:

* verify the backlogit MCP / CLI surface is reachable before depending on queue, dependency, memory, or traceability operations
* prefer backlogit query operations for targeted state lookup instead of reading many backlog markdown files into context
* use backlogit queue and dependency operations when available rather than inferring execution order from prose alone
* write concise memory summaries and checkpoints through backlogit operations at task and session boundaries when supported
* append significant task comments and associate commits with task IDs for execution traceability when those operations are available
* if backlog content was edited outside the normal mutation flow, refresh the backlogit index before relying on query results

### browser-verification

When the workspace enabled the `browser-verification` capability pack:

* verify the target server or preview environment is reachable before launching browser work
* choose headed vs headless mode intentionally and record the reason
* derive browser routes from changed pages, components, or affected user journeys
* treat OAuth, email, SMS, payments, CAPTCHAs, or other external flows as explicit human checkpoints
* carry browser findings into runtime verification and operational closure rather than leaving them as informal notes

### continuous-learning

When the workspace enabled the `continuous-learning` capability pack:

* store observation state under `docs/learnings/`
* keep hook capture optional and environment-specific; manual capture is still valid
* use `observe` to capture recurring workflow signals, `learn` to infer instincts, and `evolve` to promote mature patterns into `learned-*` artifacts
* do not harden a rule into a learned instruction or skill until it has enough corroborating observations to justify the promotion
* treat learned artifacts as explicit repository knowledge rather than invisible prompt-only behavior

### strict-safety

When the workspace enabled the `strict-safety` capability pack:

* follow `.github/instructions/strict-safety.instructions.md`
* express risky work as `ProposedAction` entries with `ActionRisk` and `ActionResult`
* require explicit approval before destructive actions and prefer approval for high-blast-radius actions
* keep risky action records visible in plan hardening, review, runtime verification, and operational closure

### release-observability

When the workspace enabled the `release-observability` capability pack:

* follow `.github/instructions/release-observability.instructions.md`
* produce monitoring plans with SLIs, dashboards, baselines, and alert thresholds before merge
* complete pre-deploy audit checklists for runtime, migration, or rollout-risk changes
* define explicit post-deploy observation windows with owner and duration
* declare rollback triggers with named metrics and thresholds
* carry all release-observability artifacts into operational closure

### adversarial-review

When the workspace enabled the `adversarial-review` capability pack:

* follow `.github/instructions/adversarial-review.instructions.md`
* escalate from standard review when 3+ P0/P1 findings appear or the work is security-sensitive
* dispatch parallel reviewer instances across different model tiers for cross-model diversity
* assemble consensus-weighted findings (HIGH / MEDIUM / LOW confidence)
* treat HIGH-confidence P0/P1 findings as gate-blocking
* feed remediation queue entries into backlog

### graphtor-docs

When the workspace enabled the `graphtor-docs` capability pack:

* follow `.github/instructions/graphtor-docs.instructions.md`
* verify the graphtor-docs server / MCP surface is reachable and sources are indexed before depending on indexed documentation retrieval
* prefer `search_local_docs`, `search_semantic`, and `research_topic` for conceptual, API-oriented, or documentation questions before broad web search or raw file scans
* use `traverse_doc_links` to follow related documentation and `list_sources` to confirm indexed coverage
* if the server is unavailable or sources are not indexed, fall back to grep, glob, or direct file reading and note reduced confidence
* treat `.graphtor/` generated artifacts as tool-managed state rather than files to hand-edit casually
* use graphtor-docs **read-only** in this workspace and **never index the workspace** — never run `graphtor-docs sync`, never register this repository as a source, and never treat index sync as part of harness install or tune

## Remote Operator Integration

### agent-intercom

When `agent-intercom` is available:

* Call `ping` at the start of any multi-step session to confirm liveness.
* Broadcast progress at meaningful phase transitions — do not broadcast every trivial step.
* Route approval for destructive actions through the intercom approval workflow before executing.
* If intercom becomes unreachable mid-task, warn that operator visibility is degraded and continue only with safe, non-destructive work.

The `ping-loop.prompt.md` prompt is available in `.github/prompts/` for sustained heartbeat sessions when the pack is installed.

### agent-engram

When `agent-engram` is available:

* Verify workspace binding before relying on indexed results.
* If the workspace is not bound or indexed, run `sync_workspace` or the workspace's equivalent freshness operation before searching.
* Fall back to grep, glob, or direct file reads only when indexed results are unavailable or insufficient.

## Backlog Workflow Expectations

When a backlog tool is active in the workspace:

* prefer queue-aware and dependency-aware operations over prose-only sequencing when the tool surface supports them
* use comments, checkpoints, and commit-tracking operations when they add traceability
* refresh the backlog index or query cache after out-of-band edits before trusting query results
* avoid inventing parallel markdown trackers outside the configured backlog tool surface

Generated by autoharness | Template: copilot-instructions.md.tmpl
