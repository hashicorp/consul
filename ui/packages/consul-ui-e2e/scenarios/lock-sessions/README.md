# Scenario: lock-sessions

Creates `SESSION_COUNT` (default 32) lock sessions on the agent node with varied
Name / Behavior / TTL / LockDelay so the **Lock Sessions** tab and the
**Invalidate** flow have realistic data.

- Sessions must be created against the live agent node (`node-primary`).
- UI: `http://localhost:4200/ui/dc1/nodes/node-primary/lock-sessions`
- Ported from `consul/scripts/dev-lock-sessions.sh`.
- Tests: [`tests/lock-sessions/`](../../tests/lock-sessions/).
