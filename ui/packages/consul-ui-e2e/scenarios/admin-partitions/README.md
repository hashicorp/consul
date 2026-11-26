# Scenario: admin-partitions

Enterprise-only: seeds the **Admin Partitions** page
(`/ui/dc1/partitions` — route name `dc.partitions`, title "Admin Partitions",
gated by `can 'read partitions'`) with a few named partitions, varied on
Description so both branches of the list row render:

| Partition | Description | Covers |
|---|---|---|
| `billing` | "Billing and payments team workloads" | Description shown. |
| `default` | *(implicit — always present)* | Un-deletable baseline (`abilities/partition.js`: `canDelete` is false for `Name === 'default'`). |
| `finance` | "Finance and ledger team workloads" | Description shown; alphabetical sort sanity check (default sort is `Name:asc`). |
| `security` | *(none)* | Description omitted — the list's `<:details>` block is conditional on `item.Description`. Also left childless, so it's a safe target for a create-then-delete test. |
| `team-01` .. `team-50` | "Bulk-seeded partition N of 50 for list volume/search testing." | 50 bulk-seeded partitions — real volume for scroll behaviour and search/sort assertions at scale. Zero-padded so `Name:asc` string sort matches numeric order. |

Full alphabetical order (`Name:asc`, the page's default sort):
`billing`, `default`, `finance`, `security`, `team-01`, `team-02`, …, `team-50`
(54 partitions total).

## Requires a licensed Enterprise agent

Like [`scenarios/intentions-enterprise`](../intentions-enterprise/README.md),
this needs:
- A **Consul Enterprise binary** (or Docker image) — `/v1/partition*` 404s on
  CE with `"Partitions are a Consul Enterprise feature"`.
- A **license**.

**Verified path: `RUNTIME=local`.** Set in `.env` (or inline):

```bash
CONSUL_BIN=/path/to/consul-enterprise/bin/consul
CONSUL_LICENSE_PATH=/path/to/your.hclic
```

then:

```bash
RUNTIME=local pnpm sc up admin-partitions
```

`local.sh` fails fast with a clear message if either var is unset or the
paths don't resolve, rather than a cryptic agent-startup error.

**`RUNTIME=docker` is scaffolded but unverified** — `docker-compose.yml`
references `hashicorp/consul-enterprise:1.20-ent` and passes the license
text through the `CONSUL_LICENSE` environment variable (never committed), but pulling that image needs registry access this repo doesn't
assume you have. Adjust the `image:` line to whatever registry you actually
use, or just use `RUNTIME=local`.

- Does **not** require the Connect CA (`requiresConnect: false`) — partitions
  don't need sidecars.
- The `create` route (`/ui/dc1/partitions/create`) is gated by
  `can 'create partitions'` — the same license check, so it renders once the
  agent is licensed.
- UI: `http://localhost:4200/ui/dc1/partitions`
- Tests: not yet written — see [`docs/adding-a-test.md`](../../docs/adding-a-test.md)
  and `tests/intentions/` for the pattern to follow.
