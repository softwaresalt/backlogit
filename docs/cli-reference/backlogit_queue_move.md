## backlogit queue move

Reorder an item in the queue

```
backlogit queue move <item-id> [flags]
```

### Examples

```
  backlogit queue move 001-T --position 1
```

### Options

```
  -h, --help           help for move
      --position int   target position in the queue
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit queue](backlogit_queue.md)	 - Manage the work queue

