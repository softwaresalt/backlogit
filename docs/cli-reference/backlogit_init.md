---
title: "backlogit init"
description: "Initialize a new backlogit workspace"
---

## backlogit init

Initialize a new backlogit workspace

### Synopsis

Initialize a backlogit workspace in the target directory.

This creates the .backlogit storage root, logs directory, canonical stash JSONL
file, default YAML configuration files, and default artifact templates.

```text
backlogit init [path] [flags]
```

### Examples

```text
  backlogit init
  backlogit init D:\Source\MyProject
```

### Options

```text
  -h, --help   help for init
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

