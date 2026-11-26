# Scenario: ingress-gateway

Registers an **`ingress-gateway`** service (`ingress-gateway`) plus an
`ingress-gateway` config entry with **6 `Listeners`** linking **5 upstream
services** (`upstream-01` .. `upstream-05`), so the service detail page's
**Upstreams** tab renders a populated list with real branch coverage.

The tab (`app/templates/dc/services/show/upstreams.hbs`) only shows when
`item.Service.Kind === 'ingress-gateway'` and the service has no `PeerName`
(`app/templates/dc/services/show.hbs`). It's backed by the *same* endpoint as
terminating-gateway's "Linked Services" tab —
`GET /v1/internal/ui/gateway-services-nodes/{gateway}?dc={dc}`
(`app/adapters/service.js` -> `UIGatewayServicesNodes` ->
`Internal.GatewayServiceDump` in Consul's Go agent) — which resolves the
gateway's config entry, expands it against the real catalog, and returns one
aggregated row per linked service name.

## Why it needs `Listeners`, not a flat `Services[]`

`terminating-gateway` config entries link services with a flat
`Services: [{Name}]` list (or a `{Name: "*"}` wildcard) and no port per
service. `ingress-gateway` config entries instead group services under
per-port `Listeners: [{Port, Protocol, Services: [{Name}]}]`. That port is
what lets the server compute a real `host:port` address per linked service
(`GatewayService.Addresses` in `agent/structs/config_entry_gateways.go`,
invoked from `modifySummaryForGatewayService` in `agent/ui_endpoint.go`),
defaulting to a DNS name of the form `<service>.ingress.<dc>.<domain>` (here,
the dev agent's default domain, `consul.`) when the listener has no explicit
`Hosts`. TCP listeners only support one linked service each (multiplexing by
service name only works for HTTP-family listeners), so this scenario declares
one listener per linked service — except `upstream-04`, which is deliberately
declared on **two** listeners/ports to produce **two** addresses on one row.

## What `Consul::Upstream::List` actually renders

The Upstreams tab renders rows with `Consul::Upstream::List`
(`app/components/consul/upstream/list/index.hbs`), which is a much simpler
card list than terminating-gateway's `Consul::Service::Table` — verified by
reading both templates directly, not assumed:

- the service **name**, linked only if `item.InstanceCount > 0` (a never-
  registered upstream renders a plain, unlinked `<p>` instead of an `<a>`)
- a **health indicator** derived from `item.MeshStatus` — but only alongside
  the *linked* name, i.e. only when `InstanceCount > 0`; an unregistered
  upstream shows neither a link nor any health indicator at all
- `Consul::Bucket::List` (namespace/partition/peer pills — hidden in OSS with
  default namespace/partition, same as elsewhere)
- one **"Address"** row (with a copy button) per entry in
  `item.GatewayConfig.Addresses`

It does **not** render tags, the "Registered via X" external-source badge, a
numeric instance count, or the service `Kind` — all present on
`Consul::Service::Table`/`Consul::Service::List` but simply absent from this
template. So, unlike `scenarios/terminating-gateway`, this seeder doesn't
bother varying tags or `external-source` meta on the backing services —
nothing in this tab would ever show them.

## Data variation (only what the component renders)

- **Instance count**: `upstream-01` has 1 real instance, `upstream-02` has 2,
  `upstream-03` has 3, `upstream-05` has **0** (never registered in the
  catalog at all — only linked via the config entry, same as the "web" case
  in Consul's own `TestInternal_GatewayServiceDump_Ingress` server test).
  `upstream-04` has 1 *real* catalog instance but **reports `InstanceCount:
  2`** — see the quirk below.
- **Aggregated health**: `upstream-01`/`upstream-04` are all-passing;
  `upstream-02`'s second instance is `critical` (aggregated `MeshStatus`
  "critical" while the first instance still passes); `upstream-03`'s third
  instance is `warning` (aggregated "warning"). `upstream-05` has no checks at
  all (and, per the above, renders no health indicator regardless).
- **Address count**: every service has exactly one address from its one
  listener, except `upstream-04`, which is linked from two listeners
  (ports `8084` and `8090`) and so has two addresses on the same row.

### Quirk: `upstream-04`'s `InstanceCount` is inflated to 2

`Internal.GatewayServiceDump` (`agent/consul/internal_endpoint.go`) fetches
catalog instances **once per matching `GatewayService` mapping** — i.e. once
per listener a service is linked from — not once per service overall.
`summarizeServices` (`agent/ui_endpoint.go`) then does an unconditional
`sum.InstanceCount += 1` per dump row with **no de-duplication** by
node/service ID (unlike `Tags`/`ExternalSources`/`Nodes`, which do dedupe
explicitly). Checks *do* dedupe (keyed by node + check ID), so aggregated
health stays correct regardless.

`upstream-04` has exactly one registered catalog instance, but because it's
linked from two listeners, `GET /v1/internal/ui/gateway-services-nodes/ingress-gateway`
reports `"InstanceCount": 2` for it — confirmed against a live agent, and
enforced by `seed.ts`'s own assertion. This is real, deterministic backend
behavior, not a seeding bug — the Playwright spec should assert
`InstanceCount` for `upstream-04` is `2`, not `1`.

Backing-service instances are registered as agentless catalog entries on
synthetic per-instance nodes (never on `node-primary`), so the live agent's
anti-entropy sync can't overwrite their hand-picked Check statuses.

- UI: `http://localhost:4200/ui/dc1/services/ingress-gateway/upstreams`
- Tests: [`tests/ingress-gateway/`](../../tests/ingress-gateway/).
