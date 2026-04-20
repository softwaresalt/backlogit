---
title: "backlogit deliberate"
description: "Create a deliberation artifact linked to a stash entry"
---

## backlogit deliberate

Create a deliberation artifact linked to a stash entry

### Synopsis

Create a first-class deliberation artifact in .backlogit\queue and link it
back to an active stash entry so future planning and implementation can recover
the full collaborative context.

```text
backlogit deliberate <stash-id> [flags]
```

### Examples

```text
  backlogit deliberate ABCD1234 --title "Audit dashboard split follow-up"
  backlogit deliberate ABCD1234 --options "- Keep the current feature set narrow\n- Pull the work into the next feature wave"
  backlogit deliberate ABCD1234 --chosen-direction "Split the backlog work and defer reporting polish"
```

### Options

```text
      --chosen-direction string   chosen direction and rationale
  -h, --help                      help for deliberate
      --notes string              supplementary notes or research
      --open-questions string     outstanding questions or risks
      --options string            option set or alternatives considered
      --problem-frame string      problem frame content
      --title string              override the deliberation title (defaults to stash text)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

