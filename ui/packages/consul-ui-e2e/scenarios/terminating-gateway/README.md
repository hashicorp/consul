# Scenario: terminating-gateway

Registers a **`terminating-gateway`** service (`api-gateway`) plus a wildcard
`terminating-gateway` config entry (`Services: [{Name: "*"}]`) linking it to
**25 backing services** (`backend-01` .. `backend-25`), so the service detail
page's **Linked Services** tab renders a fully populated table.

The tab (`app/templates/dc/services/show/services.hbs`) only shows when
`item.Service.Kind === 'terminating-gateway'` and the service has no
`PeerName` (`app/templates/dc/services/show.hbs`). It's backed by
`GET /v1/internal/ui/gateway-services-nodes/{gateway}?dc={dc}`
(`app/adapters/service.js`), which resolves the config entry, expands it
against the catalog, and returns one row per linked service name.

Backing services vary across every column `Consul::Service::List` renders:

- **Instance count** — cycles 1 -> 2 -> 3 across services, so counts aren't
  uniform (about a third have just 1 instance).
- **Health** — most instances are `passing`; roughly 1-in-5 services also
  carries a `warning`- or `critical`-status instance, so the aggregated
  row health varies too.
- **"Registered via X" badge** — 5 services (`backend-01`, `backend-06`,
  `backend-11`, `backend-16`, `backend-21`) each set
  `Meta: {'external-source': ...}` to a distinct value (`vault`,
  `kubernetes`, `terraform`, `consul-api-gateway`, `nomad`); the rest have no
  external-source meta.
- **Tag pills** — most services have 2-4 tags pulled from a fixed pool;
  `backend-03` and `backend-17` are left untagged.

Backing-service instances are registered as agentless catalog entries on
synthetic per-instance nodes (never on `node-primary`), so the live agent's
anti-entropy sync can't overwrite their hand-picked Check statuses.

- UI: `http://localhost:4200/ui/dc1/services/api-gateway/services`
- Tests: [`tests/terminating-gateway/`](../../tests/terminating-gateway/).
