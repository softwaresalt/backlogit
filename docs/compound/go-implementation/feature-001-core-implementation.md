# Compound Learnings: Feature 001 — Backlogit Core Implementation

## Session Summary
- **Feature**: TASK-001 Backlogit Core Implementation
- **Branch**: 001-backlogit-core-implementation
- **Tasks Completed**: Phases 2–10 (all packages)
- **Commits**: 5 implementation commits
- **Final State**: all tests GREEN, go build clean, go vet clean

---

## Key Patterns Discovered

### 1. Package-Level Validator Instance
```go
var validate = validator.New()

func (a Artifact) Validate() error {
    return validate.Struct(a)
}
```
**Why**: Instantiating validator.New() per-call is expensive. Cache at package level.

### 2. SQLite FTS5 with Content Table
```sql
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
    id UNINDEXED,
    title,
    description,
    content='items',
    content_rowid='rowid'
);
-- Requires 3 triggers to keep FTS in sync with items table:
-- items_ai (after insert), items_ad (after delete), items_au (after update)
```
**Why**: Content tables avoid data duplication but require explicit triggers for sync.

### 3. Atomic File Writes Pattern
```go
tmp, err := os.CreateTemp(filepath.Dir(dest), ".tmp-*")
if err != nil { return err }
// write to tmp
tmp.Close()
return os.Rename(tmp.Name(), dest)
```
**Why**: Prevents partial writes from corrupting workspace files on crash.

### 4. SafeResolve Absolute Path Comparison
```go
// CRITICAL: Convert workspaceRoot to absolute FIRST before comparison
absRoot, err := filepath.Abs(workspaceRoot)
cleanRoot := filepath.Clean(absRoot)
abs, err := filepath.Abs(filepath.Join(absRoot, target))
// Then: strings.HasPrefix(abs, cleanRoot+separator)
```
**Why**: filepath.Clean does NOT make paths absolute. If workspaceRoot is relative ("." or "../foo"), the prefix check fails and allows path traversal bypasses.

### 5. YAML Frontmatter Parsing
```go
func ParseFrontmatter(content string) (map[string]any, string, error) {
    if !strings.HasPrefix(content, "---\n") {
        return nil, content, nil  // No frontmatter - return body as-is
    }
    // Find closing ---
    rest := content[4:]
    idx := strings.Index(rest, "\n---")
    if idx == -1 {
        return nil, content, nil  // Malformed - treat as plain text
    }
    yamlBlock := rest[:idx]
    body := strings.TrimPrefix(rest[idx+4:], "\n")
    // Unmarshal yamlBlock
}
```

### 6. Import Cycle Prevention in Rehydration
**Problem**: `rehydration.go` in `db` package cannot import `parser` package if `parser` imports `models` and `db`. Creates a cycle.  
**Solution**: Use `models.ParseFrontmatter` + `models.ArtifactFromFrontmatter` directly in `rehydration.go`, bypassing the `parser` package.

### 7. modernc.org/sqlite Driver Name
```go
import _ "modernc.org/sqlite"
db, err := sql.Open("sqlite", dbPath)  // NOT "sqlite3" — that's mattn/go-sqlite3
```

### 8. SaveMemory Concurrent Access
```go
var memoriesMu sync.Mutex

func SaveMemory(ctx context.Context, memoriesPath string, key, summary string) error {
    memoriesMu.Lock()
    defer memoriesMu.Unlock()
    // read-modify-write
}
```
**Why**: MCP tools can be called concurrently. Without mutex, concurrent SaveMemory calls cause lost updates.

### 9. mcp-go v0.27.0 API
```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

mcpServer := mcp.NewServer("backlogit", "1.0.0")
tool := mcp.NewTool("tool_name",
    mcp.WithDescription("description"),
    mcp.WithString("param", mcp.Required(), mcp.Description("desc")),
)
mcpServer.AddTool(tool, handlerFunc)

stdioServer := server.NewStdioServer(mcpServer)
stdioServer.Listen(ctx, os.Stdin, os.Stdout)
```

### 10. go-playground/validator/v10 Struct Tags
```go
type Artifact struct {
    Status ArtifactStatus `validate:"required,oneof=todo in_progress blocked review done accepted rejected"`
    Title  string         `validate:"required,max=200"`
}
```
The `oneof` validator uses spaces as separators, not commas.

---

## Failed Approaches (What NOT to Do)

1. **Don't use `filepath.Clean` for absolute path comparison** — use `filepath.Abs` then `filepath.Clean` together.
2. **Don't create validator per call** — expensive, use package-level cached instance.
3. **Don't import `parser` from `db`** — import cycle. Use `models` directly.
4. **Don't use `sql.Open("sqlite3", ...)` with modernc driver** — use `"sqlite"`.

---

## Test Patterns That Work

```go
// Use t.TempDir() for all file system tests
func TestSomething(t *testing.T) {
    dir := t.TempDir() // auto-cleaned after test
    // ...
}
```
