## backlogit query

Execute a read-only SQL query against the index

### Synopsis

Execute a gated read-only SQL query against the backlogit SQLite cache.

Only SELECT statements are allowed. Use this for token-efficient inspection of
items, dependencies, logs, and indexed stash data.

```
backlogit query "<sql>" [flags]
```

### Examples

```
  backlogit query "SELECT id, title, status FROM items ORDER BY updated_at DESC LIMIT 20"
  backlogit query "SELECT stash_id, kind, state FROM stash_entries ORDER BY updated_at DESC"
```

### Options

```
  -h, --help   help for query
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

