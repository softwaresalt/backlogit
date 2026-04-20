## backlogit shipment return-blocked

Return a blocked item from a shipment

```
backlogit shipment return-blocked [flags]
```

### Examples

```
  backlogit shipment return-blocked --shipment 001-S --item 001-T --reason "blocked"
```

### Options

```
  -h, --help              help for return-blocked
      --item string       item ID
      --reason string     blocked reason
      --shipment string   shipment ID
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

