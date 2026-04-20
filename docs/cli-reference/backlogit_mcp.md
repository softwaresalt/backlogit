## backlogit mcp

Start the backlogit MCP stdio server

### Synopsis

Start the backlogit Model Context Protocol server over stdio.

Use this command from MCP-capable clients such as GitHub Copilot CLI, Claude
Code, or Cursor to expose backlogit workspace tools to agents.

```
backlogit mcp [flags]
```

### Examples

```
  backlogit mcp
  backlogit --cwd D:\Source\MyProject mcp
```

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

