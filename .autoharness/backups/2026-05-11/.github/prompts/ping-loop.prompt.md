---
description: "Heartbeat loop pattern for MCP agents connected to agent-intercom. Ensures stall detection remains satisfied and the session stays alive."
---

# Ping Loop Pattern

Use this pattern in any autonomous agent session connected to agent-intercom.
Call the `ping` MCP tool at regular intervals to:

1. Reset the stall detection timer (prevents false-positive idle alerts)
2. Signal liveness to the operator's Slack channel
3. Report progress snapshots for remote monitoring

## Basic Pattern

Call `ping` every 60–120 seconds during active work. Include a brief
`status_message` describing what the agent is currently doing.

```text
Loop:
  1. Do a unit of work (edit file, run test, read context)
  2. Call ping with status_message = "<what you just did or are about to do>"
  3. Repeat
```

## Example Tool Call

```json
{
  "tool": "ping",
  "arguments": {
    "status_message": "Implementing user authentication middleware — 3 of 7 tasks complete"
  }
}
```

## When to Ping

| Situation | Action |
|---|---|
| After completing a task or sub-task | Ping with completion summary |
| Before a long-running operation (compile, test suite) | Ping with "running tests…" |
| After reading/analyzing a large file | Ping with analysis summary |
| Every ~90 seconds during sustained work | Ping with current focus |
| On error or unexpected result | Ping with error context |

## Stall Detection Thresholds

The default stall configuration (adjustable in `config.toml`):

- **Inactivity threshold**: 300 seconds (5 minutes) — ping before this elapses
- **Escalation threshold**: 120 seconds (2 minutes) — operator notified after this
- **Max retries**: 3 — auto-nudge attempts before marking session as blocked

## Integration with `standby`

When calling `standby` (wait-for-instruction), you do **not** need to ping.
The standby tool is blocking — stall detection is paused while waiting for
operator input. Resume the ping loop after `standby` returns.

## Integration with `check_clearance`

The `check_clearance` tool blocks until operator approval. Stall detection
understands blocking tools, so pinging during the wait is not required.
Resume the ping loop after approval/rejection is received.
