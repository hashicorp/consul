# consul-ui-e2e

Scenario-based end-to-end tests for the Consul UI. Each **scenario** boots a
single Consul agent and seeds exactly the data one UI use case needs. The
**Playwright** tests then drive the real UI against it.

## How it works

```
Playwright ──drives──▶ UI under test ──HTTP API──▶ Consul agent :8500 (seeded by seed.ts)
                        ├─ UI_SOURCE=agent: :8500/ui, served by the binary itself
                        └─ UI_SOURCE=ember: :4200/ui, Ember dev server proxied to :8500
```

- A **scenario** is a directory under [`scenarios/`](scenarios/): a
  `scenario.json`, a `docker-compose.yml` (or `local.sh` for a local binary),
  and a `seed.ts` that registers the data through the HTTP API.
- Scenarios don't know about tests. Each spec file picks the one it needs with
  `test.use({ scenario: '<name>' })`, so `tests/` can be grouped by feature.
- Playwright starts and seeds a spec's scenario before its tests and stops it
  afterwards. Every scenario uses the fixed `:8500`, so they run one at a time.
- `EDITION` picks the Consul edition under test, `ce` (default) or `ent`. Tests
  tagged `@ent` run only on Enterprise, `@ce` only on CE, untagged on both.
- `UI_SOURCE` picks the UI under test:
  - `agent`: the UI the agent serves itself, from a Consul binary built with
    the UI embedded. CI uses this.
  - `ember` (default): the working-tree Ember dev server on `:4200`, which
    Playwright starts and proxies to the agent. Use this while iterating.

## Prerequisites

- Node 24 and `pnpm` (installed with the rest of the UI workspace).
- Docker (default `RUNTIME=docker`), or a `consul` binary for `RUNTIME=local`.
- `EDITION=ent` runs need a Consul Enterprise binary or image, plus a
  license (`CONSUL_LICENSE_PATH`, or `CONSUL_LICENSE` for Docker).

## Quick start

```bash
cd ui && pnpm install                 # installs the whole UI workspace
cd packages/consul-ui-e2e
pnpm install-browsers                 # one-off: Playwright's Chromium
cp .env.example .env                  # optional overrides

# Bring a scenario up by hand and poke at it:
pnpm sc up nodes
pnpm sc status nodes
pnpm sc down nodes

# Run tests against the Ember dev server (each spec starts its own scenario):
pnpm e2e                              # every CE test
pnpm e2e tests/nodes                  # one folder or file

# Local-binary runtime instead of Docker:
RUNTIME=local pnpm e2e tests/lock-sessions

# Enterprise: @ent tests plus untagged ones, against an Enterprise binary:
EDITION=ent RUNTIME=local CONSUL_BIN=/path/to/consul-ent \
  CONSUL_LICENSE_PATH=/path/to/license.hclic pnpm e2e
```

## Testing a built Consul binary

Production builds strip `data-test-*` attributes, so build the UI with
`KEEP_TEST_SELECTORS=1`. From the repository root:

```bash
KEEP_TEST_SELECTORS=1 make ui-regen   # rebuilds agent/uiserver/dist
make dev                              # bin/consul with the UI embedded
git checkout -- agent/uiserver/dist   # don't commit the rebuilt assets
```

Then, from this package:

```bash
UI_SOURCE=agent RUNTIME=local CONSUL_BIN=../../../bin/consul pnpm e2e
```

For Docker scenarios, point `CONSUL_IMAGE` at an image built from that binary,
e.g. `make dev-docker`. Every scenario in a run uses the same binary or image.

So a run can't quietly test the wrong UI, agent mode fails before any test if:

- `CONSUL_BIN` isn't a path (a bare `consul` would use whatever is on PATH), or
  `CONSUL_IMAGE` isn't set for Docker;
- the served UI has no `data-test-*` attributes, i.e. it wasn't built with
  `KEEP_TEST_SELECTORS=1` (this includes plain `make dev`, which embeds the
  committed `agent/uiserver/dist`);
- the agent's edition doesn't match `EDITION`.

Each scenario logs the Consul version and UI bundle it tested, e.g.
`[scenario] "nodes" on Consul 3.0.0, UI: consul-ui-e62f96ae….js`. These checks
can't tell whether the UI was rebuilt after your last `ui/` change, so re-run
`make ui-regen` after editing the UI.

## Scenario CLI

```bash
pnpm sc up     <name|#> [--local|--docker]   # boot + wait ready + seed
pnpm sc seed   <name|#>                      # (re-)seed a running agent
pnpm sc down   <name|#> [--local|--docker]   # tear down
pnpm sc status <name|#>                      # show agent + seeded data
pnpm sc list                                 # list available scenarios (with numbers)
```

`<name|#>` takes either the scenario name or its number from `pnpm sc list`,
e.g. `pnpm sc up 2`.

## Adding more

- New use case: [docs/adding-a-scenario.md](docs/adding-a-scenario.md)
- New test: [docs/adding-a-test.md](docs/adding-a-test.md)
