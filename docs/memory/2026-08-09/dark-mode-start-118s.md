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
---
type: dark-mode-event
event: DARK_MODE_COMPLETE
timestamp: 2026-08-10T00:10:00Z
---

## DARK_MODE_COMPLETE — 118-S F4

- shipments_shipped: 118-S
- items_closed: 106.012-T through 106.018-T (7 tasks done)
- reviewed_head: b827ade41cbc3f59448f7c9297e41ab7803d3dd5
- merge_outcome: PR #335 merged as 39a3dbaf (merge commit, P-009 compliant)
- closure_pr: #336 merged as a2db9b81
- gate_outcomes: all PASS (tests, vet, lint, format, CI, Copilot review)
- closure_status: READY
- final_origin_main: a2db9b81
- follow_up_items:
    - stash EA3BC800: invoke Cobra CLI dep list in parity test (P3)
