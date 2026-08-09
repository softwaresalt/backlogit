---
type: dark-mode-event
event: DARK_MODE_START
timestamp: 2026-08-09T21:30:00Z
agent: Ship
shipment: 118-S
---

# DARK_MODE_START — 118-S

- scope: 118-S (106.012-T through 106.018-T) — F4 durable dependency type persistence
- merge_approval_pre_authorized: true
- admin_fallback_pre_authorized: true (REVIEW_REQUIRED_BLOCK / CONVERSATION_RESOLUTION_BLOCK only)
- stop_conditions: checks_block, strategy_block, secrets_risk, scope_mismatch, unknown_block
- visibility_mode: local (agent-intercom unavailable; events persisted to docs/memory/)
- worktree: dark-factory-worktree (P-016 single worktree)
- base: origin/main @ c8925e01
