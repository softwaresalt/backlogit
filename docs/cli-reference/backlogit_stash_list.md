## backlogit stash list

List the current active stash entries

```
backlogit stash list [flags]
```

### Examples

```
  backlogit stash list
  backlogit stash list --priority high
  backlogit stash list --kind feature
  backlogit stash list --group-by-priority
```

### Options

```
      --format string       output format: table, json, tile (default "json")
      --group-by-priority   group stash entries by priority
  -h, --help                help for list
      --kind string         filter stash entries by kind (feature, task, bug, epic, unknown)
      --priority string     filter stash entries by priority
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

