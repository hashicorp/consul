# Scenario: intentions-enterprise

The Enterprise-only companion to [`scenarios/intentions`](../intentions/README.md).
Registers a second namespace and a second admin partition, then intentions
that cross them — the only way to exercise the Source/Destination Namespace
and Partition pickers in the create/edit form, which only render when
`can 'choose nspaces'` / `can 'choose partitions'` are true (both Enterprise
license checks).

| Source | Destination | Action | Covers |
|---|---|---|---|
| `web` (default/default) | `api` (default/default) | Allow | Parity with the CE scenario — sanity check nothing about licensing broke a plain intention. |
| `web` (default/default) | `payments` (default/**billing**) | Deny | Cross-**namespace** — Destination Namespace picker. |
| `web` (default/default) | `ledger` (**finance**/default) | Allow | Cross-**partition** — Destination Partition picker. |

## Requires a licensed Enterprise agent

Unlike every other scenario here, this one needs:
- A **Consul Enterprise binary** (or Docker image) — namespaces/partitions
  404 on CE with `"... are a Consul Enterprise feature"`.
- A **license**.

**Verified path: `RUNTIME=local`.** Set in `.env` (or inline):
```bash
CONSUL_BIN=/path/to/consul-enterprise/bin/consul
CONSUL_LICENSE_PATH=/path/to/your.hclic
```
then:
```bash
RUNTIME=local pnpm sc up intentions-enterprise
```
`local.sh` fails fast with a clear message if either var is unset or the
paths don't resolve, rather than a cryptic agent-startup error.

**`RUNTIME=docker` is scaffolded but unverified** — `docker-compose.yml`
references `hashicorp/consul-enterprise:1.20-ent` and passes the license
text through the `CONSUL_LICENSE` environment variable (never committed), but pulling that image needs registry access this repo doesn't
assume you have. Adjust the `image:` line to whatever registry you actually
use, or just use `RUNTIME=local`.

- Does **not** require the Connect CA (`requiresConnect: false`) — same
  reasoning as the CE scenario, intentions don't need sidecars.
- Does **not** enable ACLs — kept identical to the CE scenario aside from the
  namespace/partition surface, so an ACL-gated variant (testing the
  `can 'create intentions'` permission boundary itself) is a natural separate
  scenario later, not bundled in here.
- UI: `http://localhost:4200/ui/dc1/intentions`
- Tests: not yet written — see `docs/adding-a-test.md` /
  `tests/intentions/` for the pattern to follow once this scenario is
  verified end to end.
