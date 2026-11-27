# Scenario: nodes

Registers several catalog nodes with deliberate variety across the columns the
Nodes-list HDS migration redesigns:

| Node | Health | Services |
|---|---|---|
| app-01 | passing | 3 |
| app-02 | warning | 1 |
| db-01 | critical | 4 |
| edge-01 | passing | 0 |
| cache-01 | passing | 2 |

Plus the live agent node `node-primary` (agentful). Use it to eyeball / assert
the Health icon+text, Address copy button, and Services count badge.

- UI: `http://localhost:4200/ui/dc1/nodes`
- Related: `consul-ui/docs/nodes-hds-migration.md`.
- Tests: [`tests/nodes/`](../../tests/nodes/).
