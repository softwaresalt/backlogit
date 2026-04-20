## backlogit adopt

Adopt an orphaned item under a new parent feature

```
backlogit adopt <item-id> [flags]
```

### Examples

```
  backlogit adopt 015.009-T --parent 016-F
```

### Options

```
  -h, --help            help for adopt
      --parent string   New parent feature ID (required)
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

