# Scenario: service-mesh

Registers Connect services that exercise the **Upstreams** and **Exposed Paths**
tabs on the service-instance page, along with every conditional detail branch
those tables render.

| Service | Covers |
|---|---|
| `web` | Address branch (`LocalBindPort`) upstreams + fully-populated `Expose.Paths` (Protocol / Listener Port / Local Path Port / Path). |
| `web-variants` | Socket-path branch (`LocalBindSocketPath` + `LocalBindSocketMode`, Address suppressed) and cross-DC branch (upstream `Datacenter` != `dc1`). |
| `web-tproxy` | Sidecar `Proxy.Mode="transparent"` → transparent-proxy warning on the Upstreams tab. |
| `standalone` | Plain service, **no** sidecar → neither proxy tab appears. |
| `db`, `cache` | Upstream target services. |

The Upstreams / Exposed Paths tabs appear only when the instance has a sidecar
(`Kind=connect-proxy`), which the UI resolves via `/proxy-instance`.

- Requires the Connect CA (`requiresConnect: true`).
- UI: `http://localhost:4200/ui/dc1/services/web/instances`
- Ported from `consul/scripts/dev-service-mesh.sh`, then extended for branch coverage.
- Tests: [`tests/service-mesh/`](../../tests/service-mesh/).
