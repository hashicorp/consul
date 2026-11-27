# Scenario: mesh-gateway

Registers a **`mesh-gateway`** service with `TaggedAddresses` (`lan`, `lan_ipv4`,
`wan`, `wan_ipv4`). The service-instance **Addresses** tab only appears when
`item.Service.Kind === 'mesh-gateway'`, and renders one card per tagged address.
The `lan` entry matches the instance `Address:Port`, so the UI marks it
`(default)`.

- UI: `http://localhost:4200/ui/dc1/services/mesh-gateway/instances/node-primary/mesh-gateway/addresses`
- Related: `consul-ui/docs/service-instance-addresses-hds-migration.md`.
- Tests: [`tests/mesh-gateway/`](../../tests/mesh-gateway/).
