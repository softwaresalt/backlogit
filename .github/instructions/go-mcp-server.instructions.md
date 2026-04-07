---
description: 'Best practices for building Model Context Protocol servers in Go using the mcp-go SDK'
applyTo: '**/*.go'
---

# Go MCP Server Development Best Practices

Best practices for building Model Context Protocol servers using `github.com/mark3labs/mcp-go`, tailored to the backlogit file-backed task management system.

## Installation and Setup

### Dependencies

```text
// go.mod
module github.com/your-org/backlogit

go 1.22

require (
    github.com/mark3labs/mcp-go             v0.27.0
    gopkg.in/yaml.v3                        v3.0.1
    github.com/spf13/cobra                  v1.8.1
    github.com/go-playground/validator/v10  v10.22.0
    modernc.org/sqlite                      v1.34.0
)

require (
    github.com/stretchr/testify v1.9.0 // test
)
```

### Project Structure

```text
internal/mcp/
├── server.go        # Server factory, stdio lifecycle, shutdown
├── tools.go         # Tool definitions and dispatch
├── resources.go     # Resource handlers (config, schema)
├── prompts.go       # Prompt templates
├── gate.go          # Read-only SQL query gate
└── errors.go        # MCP-specific error formatting
cmd/
└── backlogit/
    └── main.go      # CLI entrypoint (cobra root command)
internal/
├── db/
│   ├── connection.go  # SQLite connection, WAL mode, pragma tuning
│   ├── schema.go      # CREATE TABLE, FTS5 indexes
│   └── queries.go     # Parameterized query execution
├── core/
│   ├── artifacts.go   # Artifact creation and hierarchy enforcement
│   ├── fields.go      # Custom field validation
│   └── routing.go     # File routing by registry.yaml state mappings
├── config/
│   ├── loader.go      # YAML config loading and validation
│   └── schema.go      # Config struct definitions with validator tags
└── events/
    ├── stream.go      # JSONL append-only event writer
    └── telemetry.go   # Agent telemetry logging
```

## Server Implementation

Create the MCP server using `server.NewMCPServer` with stdio transport:

```go
package mcp

import (
    "fmt"
    "log/slog"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// CreateServer builds a configured MCP server with all handlers registered.
func CreateServer() *server.MCPServer {
    s := server.NewMCPServer(
        "backlogit",
        "1.0.0",
        server.WithToolCapabilities(true),
        server.WithResourceCapabilities(false, true),
        server.WithPromptCapabilities(true),
        server.WithRecovery(),
    )

    RegisterTools(s)
    RegisterResources(s)
    RegisterPrompts(s)

    return s
}

// RunStdio starts the MCP server on stdio transport.
func RunStdio() error {
    s := CreateServer()
    slog.Info("starting backlogit MCP server on stdio")
    return server.ServeStdio(s)
}
```

Expose via Cobra in `cmd/backlogit/main.go`:

```go
package main

import (
    "fmt"
    "os"

    mcpserver "github.com/your-org/backlogit/internal/mcp"
    "github.com/spf13/cobra"
)

func main() {
    root := &cobra.Command{Use: "backlogit"}
    root.AddCommand(&cobra.Command{
        Use:   "mcp",
        Short: "Start MCP stdio server",
        RunE: func(cmd *cobra.Command, args []string) error {
            return mcpserver.RunStdio()
        },
    })

    if err := root.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

## Tool Implementation

### Registration

Register tools using `mcp.NewTool` with typed parameter builders and `AddTool` for handler binding:

```go
package mcp

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// RegisterTools adds all backlogit tools to the server.
func RegisterTools(s *server.MCPServer) {
    s.AddTool(
        mcp.NewTool("backlogit_create_item",
            mcp.WithDescription("Create a new artifact"),
            mcp.WithString("title",
                mcp.Required(),
                mcp.Description("Artifact title (1-200 chars)"),
            ),
            mcp.WithString("artifact_type",
                mcp.Required(),
                mcp.Description("Artifact type"),
                mcp.Enum("task", "bug", "story"),
            ),
            mcp.WithString("status",
                mcp.Description("Initial status"),
                mcp.DefaultString("todo"),
            ),
            mcp.WithString("description",
                mcp.Description("Artifact description body"),
            ),
        ),
        handleCreateItem,
    )

    s.AddTool(
        mcp.NewTool("backlogit_query_sql",
            mcp.WithDescription("Execute a read-only SQL query against index.db"),
            mcp.WithString("sql",
                mcp.Required(),
                mcp.Description("SELECT statement to execute"),
            ),
        ),
        handleQuerySQL,
    )
}
```

### Five-Step Tool Handler Pattern

Every tool handler follows this structure:

```go
func handleCreateItem(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 1. Validate workspace exists
    workspace := ".backlogit"
    if !dirExists(workspace) {
        return workspaceNotInitialized(), nil
    }

    // 2. Parse and validate parameters
    title, err := request.RequireString("title")
    if err != nil {
        return validationFailed(err.Error()), nil
    }
    artifactType, err := request.RequireString("artifact_type")
    if err != nil {
        return validationFailed(err.Error()), nil
    }
    status := request.GetString("status", "todo")
    description := request.GetString("description", "")

    // 3. Get DB connection
    conn, err := GetConnection(workspace + "/index.db")
    if err != nil {
        return internalError(fmt.Sprintf("database open failed: %v", err)), nil
    }
    defer conn.Close()

    // 4. Execute business logic
    artifact, err := CreateArtifact(conn, title, artifactType, status, description)
    if err != nil {
        return internalError(err.Error()), nil
    }

    // 5. Return JSON result
    result := map[string]string{
        "id":     artifact.ID,
        "title":  artifact.Title,
        "status": artifact.Status,
    }
    data, _ := json.MarshalIndent(result, "", "  ")
    return mcp.NewToolResultText(string(data)), nil
}
```

## Read-Only SQL Query Gate

The `backlogit_query_sql` tool uses a strict gate to enforce read-only access.

```go
package mcp

import (
    "database/sql"
    "fmt"
    "regexp"
    "strings"
)

var forbiddenPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|REPLACE)\b`),
    regexp.MustCompile(`(?i)\bATTACH\b`),
    regexp.MustCompile(`(?i)\bPRAGMA\s+(?!table_info|table_list|database_list)`),
    regexp.MustCompile(`--`),
    regexp.MustCompile(`(?m);.*\S+.*$`),
}

// MaxRows caps the number of rows returned by a gated query.
const MaxRows = 500

// GateResult reports whether a SQL statement passed the read-only gate.
type GateResult struct {
    Allowed bool
    Reason  string
}

// ValidateQuery checks whether a SQL statement is safe for read-only execution.
func ValidateQuery(sqlStr string) GateResult {
    stripped := strings.TrimSpace(sqlStr)
    if !strings.HasPrefix(strings.ToUpper(stripped), "SELECT") {
        return GateResult{Allowed: false, Reason: "Only SELECT statements are permitted"}
    }
    for _, pattern := range forbiddenPatterns {
        if loc := pattern.FindString(stripped); loc != "" {
            return GateResult{
                Allowed: false,
                Reason:  fmt.Sprintf("Forbidden pattern: %s", loc),
            }
        }
    }
    return GateResult{Allowed: true}
}

// ExecuteGatedQuery runs a validated read-only query capped at MaxRows.
func ExecuteGatedQuery(
    db *sql.DB,
    query string,
    params ...any,
) ([]map[string]interface{}, error) {
    gate := ValidateQuery(query)
    if !gate.Allowed {
        return nil, fmt.Errorf("query rejected: %s", gate.Reason)
    }

    rows, err := db.Query(query, params...)
    if err != nil {
        return nil, fmt.Errorf("query execution failed: %w", err)
    }
    defer rows.Close()

    columns, err := rows.Columns()
    if err != nil {
        return nil, fmt.Errorf("failed to read columns: %w", err)
    }

    var results []map[string]interface{}
    for rows.Next() && len(results) < MaxRows {
        values := make([]interface{}, len(columns))
        ptrs := make([]interface{}, len(columns))
        for i := range values {
            ptrs[i] = &values[i]
        }
        if err := rows.Scan(ptrs...); err != nil {
            return nil, fmt.Errorf("row scan failed: %w", err)
        }
        row := make(map[string]interface{}, len(columns))
        for i, col := range columns {
            row[col] = values[i]
        }
        results = append(results, row)
    }
    return results, rows.Err()
}
```

## Resource and Prompt Handlers

### Resources

Expose workspace metadata as MCP resources:

```go
package mcp

import (
    "context"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// RegisterResources adds workspace metadata resources to the server.
func RegisterResources(s *server.MCPServer) {
    s.AddResource(
        mcp.NewResource(
            "backlogit://config",
            "Workspace Configuration",
            mcp.WithResourceDescription("Current config.yaml"),
            mcp.WithMIMEType("application/x-yaml"),
        ),
        handleConfigResource,
    )

    s.AddResource(
        mcp.NewResource(
            "backlogit://schema",
            "Database Schema",
            mcp.WithResourceDescription("SQLite table definitions"),
            mcp.WithMIMEType("text/plain"),
        ),
        handleSchemaResource,
    )
}

func handleConfigResource(
    ctx context.Context,
    req mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
    data, err := readConfig()
    if err != nil {
        return nil, err
    }
    return []mcp.ResourceContents{
        mcp.TextResourceContents{
            URI:      req.Params.URI,
            MIMEType: "application/x-yaml",
            Text:     data,
        },
    }, nil
}
```

### Prompts

Register templates that agents invoke for guided workflows:

```go
// RegisterPrompts adds prompt templates to the server.
func RegisterPrompts(s *server.MCPServer) {
    s.AddPrompt(
        mcp.NewPrompt("create_sprint",
            mcp.WithPromptDescription("Sprint planning guide"),
            mcp.WithArgument("goal",
                mcp.ArgumentDescription("Sprint goal statement"),
                mcp.RequiredArgument(),
            ),
        ),
        handleCreateSprintPrompt,
    )
}

func handleCreateSprintPrompt(
    ctx context.Context,
    req mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
    goal := ""
    if args := req.Params.Arguments; args != nil {
        if g, ok := args["goal"].(string); ok {
            goal = g
        }
    }
    return &mcp.GetPromptResult{
        Description: "Plan a new sprint with the given goal",
        Messages: []mcp.PromptMessage{
            {
                Role:    mcp.RoleUser,
                Content: mcp.NewTextContent("Create sprint: " + goal),
            },
        },
    }, nil
}
```

## Error Handling

### Sentinel Errors

Define package-level sentinel errors and use `fmt.Errorf` wrapping:

```go
package mcp

import "errors"

var (
    ErrWorkspaceNotInitialized = errors.New("no .backlogit directory found")
    ErrValidationFailed        = errors.New("parameter validation failed")
    ErrQueryRejected           = errors.New("query rejected by read-only gate")
)
```

### Structured Error Responses

Return consistent error content from tool handlers using helper functions:

```go
func workspaceNotInitialized() *mcp.CallToolResult {
    return mcp.NewToolResultError(
        `{"error":"workspace_not_initialized",` +
            `"message":"No .backlogit directory found. Run backlogit init first."}`,
    )
}

func validationFailed(detail string) *mcp.CallToolResult {
    return mcp.NewToolResultError(
        fmt.Sprintf(`{"error":"validation_failed","message":"%s"}`, detail),
    )
}

func internalError(detail string) *mcp.CallToolResult {
    return mcp.NewToolResultError(
        fmt.Sprintf(`{"error":"internal","message":"%s"}`, detail),
    )
}
```

## Testing MCP Tools

### Test Helpers

Create temporary workspaces with a SQLite database for integration tests:

```go
package mcp_test

import (
    "database/sql"
    "os"
    "path/filepath"
    "testing"

    _ "modernc.org/sqlite"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupWorkspace(t *testing.T) (string, *sql.DB) {
    t.Helper()
    ws := filepath.Join(t.TempDir(), ".backlogit")
    require.NoError(t, os.MkdirAll(ws, 0o755))

    configPath := filepath.Join(ws, "config.yaml")
    require.NoError(t, os.WriteFile(
        configPath,
        []byte("artifact_types: [task, bug, story]\n"),
        0o644,
    ))

    dbPath := filepath.Join(ws, "index.db")
    db, err := sql.Open("sqlite", dbPath)
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })

    _, err = db.Exec(`
        CREATE TABLE items (
            id TEXT PRIMARY KEY, title TEXT, status TEXT, type TEXT
        );
        INSERT INTO items VALUES ('001-T', 'Sample task', 'todo', 'task');
    `)
    require.NoError(t, err)

    return ws, db
}
```

### Gate Tests

```go
func TestValidateQuery(t *testing.T) {
    allowed := []string{
        "SELECT * FROM items",
        "SELECT id FROM items WHERE status = ?",
        "SELECT COUNT(*) FROM items",
    }
    for _, sql := range allowed {
        t.Run("allows/"+sql, func(t *testing.T) {
            result := ValidateQuery(sql)
            assert.True(t, result.Allowed, "expected allowed for: %s", sql)
        })
    }

    rejected := []struct {
        sql     string
        keyword string
    }{
        {"DROP TABLE items", "DROP"},
        {"DELETE FROM items", "DELETE"},
        {"INSERT INTO items VALUES ('X','Y','Z','W')", "INSERT"},
        {"UPDATE items SET status='done'", "UPDATE"},
        {"ATTACH DATABASE 'other.db' AS other", "ATTACH"},
    }
    for _, tc := range rejected {
        t.Run("rejects/"+tc.keyword, func(t *testing.T) {
            result := ValidateQuery(tc.sql)
            assert.False(t, result.Allowed)
            assert.Contains(t, result.Reason, tc.keyword)
        })
    }
}
```

## Configuration

### Environment Variables

| Variable                    | Default       | Purpose                        |
|-----------------------------|---------------|--------------------------------|
| `BACKLOGIT_WORKSPACE`       | `.backlogit/` | Workspace root directory       |
| `BACKLOGIT_LOG_LEVEL`       | `INFO`        | Logging verbosity              |
| `BACKLOGIT_LOG_FORMAT`      | `text`        | Log format (`text` or `json`)  |
| `BACKLOGIT_QUERY_TIMEOUT`   | `10`          | SQL query timeout in seconds   |
| `BACKLOGIT_QUERY_ROW_LIMIT` | `500`         | Maximum rows per query         |

### YAML Configuration

Three files in `.backlogit/` control workspace behavior:

* `config.yaml`: Artifact types, hierarchy, ID prefixes, naming templates
* `registry.yaml`: Status-to-directory and type-to-directory path mappings
* `hooks.yaml`: External integration triggers (Git hooks, CI notifications)

Load with struct validation at startup:

```go
package config

import (
    "os"

    "github.com/go-playground/validator/v10"
    "gopkg.in/yaml.v3"
)

// WorkspaceConfig holds validated workspace settings from config.yaml.
type WorkspaceConfig struct {
    ArtifactTypes []string          `yaml:"artifact_types" validate:"required,min=1,dive,alpha"`
    IDPrefixMap   map[string]string `yaml:"id_prefix_map"  validate:"required"`
    MaxSlugLength int               `yaml:"max_slug_length" validate:"gte=10,lte=200"`
}

// Load reads and validates config.yaml from the workspace root.
func Load(path string) (*WorkspaceConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg WorkspaceConfig
    cfg.MaxSlugLength = 60
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    if err := validator.New().Struct(cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

## Backlogit MCP Tool Reference

| Tool                    | Purpose                                           | Write | Gate |
|-------------------------|---------------------------------------------------|-------|------|
| `backlogit_create_item` | Create artifact with Markdown file and DB row     | Yes   | N/A  |
| `backlogit_edit_item`   | Update frontmatter fields on an existing artifact | Yes   | N/A  |
| `backlogit_query_sql`   | Run a read-only SELECT against the SQLite index   | No    | Yes  |
| `backlogit_list_items`  | List artifacts with optional status/type filters  | No    | No   |
| `backlogit_get_item`    | Retrieve full Markdown content and metadata by ID | No    | No   |
| `backlogit_move_item`   | Change status and move to the mapped folder       | Yes   | N/A  |
| `backlogit_sync`        | Rehydrate the SQLite index from Markdown files    | Yes   | N/A  |
| `backlogit_init`        | Initialize a new `.backlogit/` workspace          | Yes   | N/A  |

All write tools follow the five-step handler pattern. All read tools validate workspace existence before querying. `backlogit_query_sql` applies the SQL gate before execution.
