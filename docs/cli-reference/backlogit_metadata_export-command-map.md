## backlogit metadata export-command-map

Write an agent-readable command map into the workspace

### Synopsis

Export a cacheable command map file into a workspace-relative path such as
.github\instructions\backlogit-command-map.instructions.md so agents can reason
over backlogit commands and metadata without re-discovering them each run.

```
backlogit metadata export-command-map <workspace-relative-path> [flags]
```

### Examples

```
  backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
  backlogit metadata export-command-map .github\instructions\backlogit-command-map.json --format json
```

### Options

```
      --format string   output format: markdown or json (default "markdown")
  -h, --help            help for export-command-map
```

### Options inherited from parent commands

```
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit metadata](backlogit_metadata.md)	 - Discover backlogit metadata for agents and tooling

