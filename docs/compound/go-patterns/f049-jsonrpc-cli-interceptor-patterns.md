---
title: "F049 JSON-RPC CLI interceptor and Cobra error-path wrapping patterns"
description: "Patterns for wrapping Cobra CLI stdout in JSON-RPC 2.0 envelopes, handling Cobra's PostRun-skips-on-error limitation, and boolean flag pre-scanning."
problem_type: best_practice
category: best_practice
component: cli
root_cause: cobra_postrun_skip
resolution_type: implementation
severity: high
message: "Cobra skips PersistentPostRunE when RunE returns an error; wrapping the full Execute path is required for error-path JSON-RPC coverage."
file_path: "internal/cli/root.go"
resolved: true
tags:
  - backlogit
  - f049
  - jsonrpc
  - cobra
  - cli
---

## F049: JSON-RPC CLI Interceptor Patterns

## Context

Shipment 048-S / Feature 049-F added JSON-RPC 2.0 envelope output to the CLI
(`--jsonrpc` flag) and a `backlogit manifest` command. Several non-obvious
implementation patterns were discovered.

## Pattern 1: Cobra PostRun Skips on Error

### Problem

`PersistentPostRunE` is **not called** when a command's `RunE` returns an error.
If you redirect `cmd.SetOut` to a buffer in `PersistentPreRunE` and wrap the
buffer content in `PersistentPostRunE`, any command failure causes the output
to be silently lost — the caller gets nothing instead of a JSON-RPC error
envelope.

### Solution

Create a top-level `Execute()` function (separate from `NewRootCommand`) that:

1. Pre-scans `os.Args` for `--jsonrpc` **before** calling `root.Execute()`.
2. Sets `root.SilenceErrors = true` when `--jsonrpc` is detected.
3. After `root.Execute()` returns an error, writes a `WrapError` envelope
   directly to the original stdout.

```go
func Execute() error {
    jctx := &jsonrpcInterceptor{}
    root := newRootCommandImpl(jctx)

    jsonrpcRequested := false
    for _, arg := range os.Args[1:] {
        if arg == "--jsonrpc" ||
            (strings.HasPrefix(arg, "--jsonrpc=") &&
                arg != "--jsonrpc=false" && arg != "--jsonrpc=0") {
            jsonrpcRequested = true
            break
        }
    }
    if jsonrpcRequested {
        root.SilenceErrors = true
    }

    err := root.Execute()
    if err != nil && jsonrpcRequested {
        origOut := jctx.origOut
        if origOut == nil {
            origOut = os.Stdout
        }
        cmdPath := jctx.cmdPath
        if cmdPath == "" {
            cmdPath = "backlogit"
        }
        b, wrapErr := format.WrapError(cmdPath, format.ErrCodeServerError, err.Error())
        if wrapErr != nil {
            b = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":%d,"message":%q}}`,
                cmdPath, format.ErrCodeServerError, err.Error()))
        }
        fmt.Fprintf(origOut, "%s\n", b)
    }
    return err
}
```

**Key**: `main.go` calls `cli.Execute()`, not `cli.NewRootCommand().Execute()`.
`NewRootCommand()` is kept for test access only.

## Pattern 2: pflag Boolean Flag Pre-Scanning

### Problem

pflag accepts boolean flags in multiple forms:
- `--flag` (bare → true)
- `--flag=true`
- `--flag=1`
- `--flag=false`
- `--flag=0`

A naive pre-scan (`arg == "--jsonrpc"`) misses the `=true` and `=1` variants.

### Solution

```go
if arg == "--jsonrpc" ||
    (strings.HasPrefix(arg, "--jsonrpc=") &&
        arg != "--jsonrpc=false" && arg != "--jsonrpc=0") {
    jsonrpcRequested = true
}
```

## Pattern 3: WrapResult Forces `result` Key Even When Nil

### Problem

JSON-RPC 2.0 requires the `result` key to be present on success even when the
value is null. A plain struct with `omitempty` will omit `result: null`,
violating the spec.

### Solution

Use an anonymous inner struct to override `omitempty` behavior:

```go
func WrapResult(id string, result any) ([]byte, error) {
    return json.Marshal(struct {
        JSONRPC string `json:"jsonrpc"`
        ID      string `json:"id"`
        Result  any    `json:"result"` // no omitempty — spec requires key
    }{
        JSONRPC: "2.0",
        ID:      id,
        Result:  result,
    })
}
```

## Pattern 4: MCP Manifest via ToolDefs()

### Problem

The `backlogit manifest` command needs MCP tool definitions (including full
`inputSchema`) without starting a full workspace or MCP stdio server.

### Solution

1. Add `ToolDefs() []mcplib.Tool` to the `Server` struct — returns a copy of
   `s.toolDefs` (the slice populated during `RegisterTool` calls).
2. The manifest command calls `NewServerForRoot(*cwd)` which registers tools
   without requiring an initialized workspace.
3. `mcplib.Tool` already implements `json.Marshaler` with full `inputSchema`.
4. Sort by tool name for alphabetical consistency with `tools/list`.

```go
func newManifestCommand(cwd *string) *cobra.Command {
    return &cobra.Command{
        Use:   "manifest",
        Short: "Print the backlogit MCP tool manifest as JSON",
        RunE: func(cmd *cobra.Command, _ []string) error {
            s := mcpinternal.NewServerForRoot(*cwd)
            tools := s.ToolDefs()
            sort.Slice(tools, func(i, j int) bool {
                return tools[i].Name < tools[j].Name
            })
            out := struct {
                Tools []mcplib.Tool `json:"tools"`
            }{Tools: tools}
            b, err := json.MarshalIndent(out, "", "  ")
            if err != nil {
                return err
            }
            fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
            return nil
        },
    }
}
```

## Pattern 5: Test-Facing vs Production Execute Path

Keep `NewRootCommand()` for test access (test files can set `SetArgs` and
`SetOut` directly). The production entrypoint is `Execute()`. Do not merge
them or tests lose control over the command environment.

```go
// For tests
cmd := cli.NewRootCommand()
cmd.SetArgs([]string{"manifest"})
cmd.SetOut(&buf)
err := cmd.Execute()

// For main.go
func main() {
    if err := cli.Execute(); err != nil {
        os.Exit(1)
    }
}
```
