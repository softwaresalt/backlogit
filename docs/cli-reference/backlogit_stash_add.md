## backlogit stash add

Add an item to the stash

```
backlogit stash add <text> [flags]
```

### Examples

```
  backlogit stash add "Investigate tenant-specific rate limits" --kind feature --priority high
  backlogit stash add "Document migration edge cases" --kind task
```

### Options

```
  -h, --help              help for add
      --kind string       stash item kind (feature, task, bug, epic, unknown) (default "task")
      --priority string   stash priority (low, medium, high, critical) (default "medium")
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

