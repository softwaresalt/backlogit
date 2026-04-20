## backlogit shipment create

Create a shipment

```
backlogit shipment create [flags]
```

### Examples

```
  backlogit shipment create --title "Sprint 1" --items 001-T,002-T
```

### Options

```
  -h, --help           help for create
      --items string   comma-separated item IDs
      --title string   shipment title
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

