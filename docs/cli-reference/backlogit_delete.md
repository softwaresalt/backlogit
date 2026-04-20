## backlogit delete

Delete an artifact

### Synopsis

Delete an artifact from the workspace and remove it from the index.

This is a destructive operation and requires --force.

```
backlogit delete <id> [flags]
```

### Examples

```
  backlogit delete 001-T --force
```

### Options

```
      --force   skip confirmation and delete immediately
  -h, --help    help for delete
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

