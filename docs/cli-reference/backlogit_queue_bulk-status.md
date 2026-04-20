## backlogit queue bulk-status

Update status for multiple items

```
backlogit queue bulk-status [flags]
```

### Examples

```
  backlogit queue bulk-status --ids 001-T,002-T,003-T --status active
```

### Options

```
  -h, --help            help for bulk-status
      --ids string      comma-separated list of item IDs
      --status string   target status
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit queue](backlogit_queue.md)	 - Manage the work queue

