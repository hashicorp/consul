# Adding a scenario

A **scenario** is one Consul topology + a seeder that registers the data a UI
use case needs. One scenario runs at a time on the fixed `:8500`. Scenarios
don't know about tests; a spec file picks the scenario it needs with
`test.use({ scenario })`.

## Recipe

1. **Copy the template**

   ```bash
   cp -r scenarios/_template scenarios/<name>
   ```

2. **`scenario.json`** — metadata:

   | field | meaning |
   |---|---|
   | `description` | one-line summary (shown by `pnpm sc list`) |
   | `routes` | UI routes this scenario exercises (for docs/status output) |
   | `requiresConnect` | `true` if seeding needs the Connect CA (service mesh) |
   | `requiresEnterprise` | `true` if the data needs Consul Enterprise; omit otherwise (runs on either edition) |
   | `requiresLocalRuntime` | `true` if it only works with `local.sh` (always started that way) |

3. **`docker-compose.yml`** — usually leave as-is (single dev agent, node
   `node-primary`, Consul on `:8500`). Add containers only if the use-case needs
   a real topology (e.g. multiple servers). Keep the Consul HTTP port at `8500`.

4. **`seed.ts`** — export `async function seed(consul)` and register data using
   the [`ConsulClient`](../lib/consul-client.ts) helpers rather than raw fetch:

   - `registerService(payload)` — services + sidecars (Connect).
   - `createSession(payload)` / `nodeSessions(node)` — lock sessions.
   - `registerCatalogNode(entry)` / `catalogNodes()` — catalog nodes.
   - `nodeName()` — the live agent node (sessions must target it).

   End the seeder with a **cheap assertion** that the data landed (see the
   existing scenarios) so a broken seed fails fast, before Playwright starts.

5. **`local.sh`** — leave as-is unless the topology changed; it's the
   `RUNTIME=local` fallback that boots `CONSUL_BIN`. For a `requiresEnterprise` scenario,
   copy `scenarios/namespaces/` instead (needs `CONSUL_LICENSE_PATH`).

6. **`README.md`** — what it sets up, which UI route, and a link to its tests.

## Verify

```bash
pnpm sc up <name>            # boots + seeds
pnpm sc status <name>        # shows node/services/routes
open http://localhost:8500/v1/catalog/services
pnpm sc down <name>
```

## Conventions

- **Fixed ports**: Consul `:8500`, ember `:4200`. Don't remap — the ember proxy
  config depends on them.
- **Deterministic data**: no random values; tests assert on them.
- **Node name is `node-primary`** unless the topology genuinely needs more.
- **Idempotent-ish**: `pnpm sc down <name>` uses `docker compose down -v`,
  so re-running `up` gives a clean agent. `up` first stops any scenario this
  package left running (`KEEP_UP=1`, an earlier `pnpm sc up`, a crashed run),
  refuses if something else answers on `:8500`, and stops whatever it started
  if it fails part-way. `down` waits until `:8500` is free.
- Port an existing hand-written dev script? Move its `curl`/`jq` calls into
  `seed.ts` via `ConsulClient` — see `scenarios/service-mesh` and
  `scenarios/lock-sessions`, ported from `consul/scripts/dev-*.sh`.
