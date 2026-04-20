## backlogit move

Change artifact status

### Synopsis

Change an artifact status and relocate its file according to registry.yaml
routing rules.

```
backlogit move <id> [flags]
```

### Examples

```
  backlogit move 001-T --status review
  backlogit move 001-F --status done
```

### Options

```
  -h, --help            help for move
      --status string   new status (required)
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

