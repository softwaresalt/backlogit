## backlogit list

List artifacts in the workspace

### Synopsis

List artifacts from the backlogit index with optional filters.

Use this command for quick operator views, grouped summaries, or JSON output
that can be piped into other tooling.

```
backlogit list [flags]
```

### Examples

```
  backlogit list
  backlogit list --status active --type task
  backlogit list --group-by status
  backlogit list --json
```

### Options

```
      --assigned-to string   filter by assignee
      --format string        output format: table, json, tile (default "table")
      --group-by string      group output by field (status, type, priority)
  -h, --help                 help for list
      --json                 output as JSON array
      --owner string         filter by owner
      --priority string      filter by priority
      --sprint string        filter by sprint ID
      --status string        filter by status
      --type string          filter by artifact type
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

