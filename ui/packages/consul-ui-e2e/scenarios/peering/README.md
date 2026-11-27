# Scenario: peering

`/ui/dc1/peers` — cluster peering: the peers list, a peer's detail page, and
its Imported/Exported services tabs. A real, ACTIVE peering between the
primary cluster and a second, independent Consul cluster, with a real
Connect service exported across it — not two fake rows with a status string.

## Why this scenario looks different from the others

Every other scenario is "one Consul agent + `seed.ts`". Cluster peering is
inherently **two independent Consul clusters**: an "acceptor" generates a
token, a "dialer" consumes it. So this scenario runs:

- The **primary** agent, node `node-primary`, on the fixed `:8500` — the one
  Playwright/ember actually talks to, same as every other scenario.
- A **second, wholly separate** agent, node `node-peer`, published on
  `:8510` — started alongside the primary by `local.sh`/`docker-compose.yml`,
  but only ever addressed directly by `seed.ts` (via a second `ConsulClient`
  pointed at `http://127.0.0.1:8510`). consul-ui itself never talks to it.

### The hard part: the peering token's baked-in address

A peering token embeds the acceptor's advertised gRPC-TLS address (confirmed
by decoding a real token — `ServerAddresses`/`PeerServerAddresses` in
`/v1/peerings`, always a literal `IP:port`, never a DNS name). The dialer
has to actually be able to reach that address, which is the entire
networking problem this scenario has to solve:

- **`RUNTIME=local`** — both agents run as local processes on `127.0.0.1`
  with distinct ports for everything (HTTP, gRPC, gRPC-TLS, Serf LAN/WAN,
  server RPC — see `local.sh`). Since both are on loopback, the token's
  `127.0.0.1:<peer's grpc-tls port>` is trivially reachable from the other
  agent. No Docker networking tricks needed. Verified end-to-end: peering
  reaches `ACTIVE` in well under 30s.
- **`RUNTIME=docker`** — also verified end-to-end. Two containers on a
  compose network with a **static subnet**, each pinned to a **static IP**
  via `-advertise=<that IP>`, so the token's baked-in address is one the
  other container can actually dial (plain Docker service-name DNS doesn't
  help — the token only ever contains an IP, never a hostname). This alone
  wasn't enough, though: Consul's `-dev` mode force-binds cluster ports
  (server RPC, Serf LAN/WAN) to `127.0.0.1` regardless of `-advertise`
  *unless `-bind` is also set explicitly* — without it, token generation
  itself failed with the agent unable to dial **its own** advertised
  address ("connection refused"). Adding `-bind 0.0.0.0` fixed it. See
  `docker-compose.yml`'s comment for the full story.

Both runtimes are fully supported (no `requiresLocalRuntime` needed here,
unlike `scenarios/auth-methods`) — this scenario doesn't have that one's
hostname-agreement problem, since nothing here needs to match a browser-
visible URL.

## What it seeds

`billing-cluster` is deliberately set up so **all three** of consul-ui's peer
detail tabs — Imported Services, Exported Services, and Server Addresses —
have real data, not just Imported Services:

1. On the **peer** cluster: a real Connect service, `payments`, with a
   sidecar, exported via an `exported-services` config entry to a consumer
   named `primary-cluster` (the name the peer calls the primary) — this is
   what shows up under **Imported Services** on the primary's side.
2. On the **primary**: a real Connect service, `invoices`, with a sidecar,
   exported the same way to a consumer named `billing-cluster` — this is
   what shows up under **Exported Services**.
3. A peering token generated **on the peer** (the acceptor here), for a peer
   it calls `primary-cluster`.
4. That token established **from the primary** (the dialer here), naming the
   peer `billing-cluster` — this is the name that shows up in the primary's
   own `/v1/peerings` and consul-ui's peers list, and the route segment:
   `/ui/dc1/peers/billing-cluster`.

   **Why the primary has to be the one that establishes, not generates:**
   consul-ui only renders the **Server Addresses** tab when the local
   peering record has a `PeerID` — `Peer#isReceiver` in consul-ui's
   `app/models/peer.js`, whose own comment says "only the side that
   establishes will hold this property". Generate-on-primary /
   establish-on-peer (the more obvious "acceptor/dialer" reading) leaves the
   primary's own record with no `PeerID`, so the tab never renders there —
   confirmed against a real agent. Flipping it so the primary calls
   `/v1/peering/establish` is what makes `PeerServerAddresses` (the peer's
   own address, baked into the token) show up under Server Addresses.
5. Polls the primary's `peerings()` until `billing-cluster` is `"ACTIVE"`,
   then hard-asserts `PeerID` and `PeerServerAddresses` are both populated —
   the seeder fails fast (not a Playwright timeout) if either the peering
   never connects or the Server Addresses tab would end up empty.
6. Best-effort (warns, doesn't fail): confirms `payments`/`invoices` actually
   show up in `StreamStatus.ImportedServices`/`ExportedServices` for
   `billing-cluster` — see the Gotcha below on why this is soft, not hard.
7. A **second** token, `reporting-cluster`, generated on the primary (as an
   acceptor would) and deliberately never established — generating a token
   alone creates a `PENDING` peering entry immediately (confirmed against a
   real agent), so the peers list shows both an `ACTIVE` and a `PENDING`
   status badge. Nothing establishes it, so it never gets a `PeerID` or
   addresses — it's only there for list-page badge variety.

## Running it

```bash
pnpm sc up peering              # RUNTIME=local by default per .env, or:
pnpm sc up peering --local
pnpm sc up peering --docker

pnpm sc status peering
curl -s localhost:8500/v1/peerings | jq

open http://localhost:4200/ui/dc1/peers
open http://localhost:4200/ui/dc1/peers/billing-cluster

pnpm sc down peering            # match whichever runtime you used
```

## Gotchas

- **`ImportedServiceCount` replication can be timing-sensitive on some
  `CONSUL_BIN` builds — `seed.ts` treats it as best-effort, not a hard
  failure, because of this.** Verified reliably (multiple clean runs) end-
  to-end against both `hashicorp/consul:1.20` (the image
  `docker-compose.yml` pins) and a local `2.1.0-dev` snapshot-style binary
  with an experimental "v2 resources" controller layer for exported
  services — `payments` shows up in `StreamStatus.ImportedServices` within
  seconds of the `exported-services` config entry landing on the peer, via
  the normal `pnpm sc up peering` flow. However, while debugging on
  that `2.1.0-dev` binary directly (repeatedly PUTing/deleting the same
  `exported-services` config entry by hand, out of the normal one-shot
  seed order), it got stuck logging `existing exported-services config
  entry is not managed with v2 resources...` and stopped replicating the
  export at all until the agent was restarted — the controller's
  reconcile loop appears to only fire reliably once, at start. If you hit
  this for real (not just via manual poking), the peering connection
  itself still reaches `ACTIVE` regardless — check `seed.ts`'s printed
  WARNING and, if needed, point `CONSUL_BIN` at a standard release binary
  instead — same category of fix as `scenarios/auth-methods`'s `CONSUL_BIN`
  gotcha.
- **Port collisions with a stale scenario.** If a previous scenario's agent
  (Docker or local) is still holding `:8500` or `:8510`, `up` may end up
  talking to the wrong one. `pnpm sc down <other-scenario>` (or check
  `lsof -iTCP:8500,8510 -sTCP:LISTEN -n -P` / `docker ps`) before filing
  that as a bug here.
- The peer names on each side (`billing-cluster` from the primary, and
  `primary-cluster` from the peer) are independent by design — same as
  consul-ui's own "Add peer connection" dialog, which only ever lets you
  name the *other* end from your own agent's perspective.

## Tests

[`tests/peering/`](../../tests/peering/) — `peers-list.spec.ts` (both peers'
status badges + imported/exported counts) and `peer-detail.spec.ts`
(`billing-cluster`'s Imported Services, Exported Services, and Server
Addresses tabs, plus the underlying `PeerID`/`PeerServerAddresses` sanity
check).
