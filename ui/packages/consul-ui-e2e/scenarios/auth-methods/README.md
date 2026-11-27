# Scenario: auth-methods

`/ui/dc1/acls/auth-methods` — list, detail, binding rules — plus a **real
OIDC login through the UI**, popup and all, against a local Dex identity
provider. Not a UI happy-path click-through: Dex is a real OIDC server, the
login form is real, and the ACL token consul-ui ends up with is real.

## Why this scenario looks different from the others

Every other scenario is "one Consul agent + `seed.ts`". This one is:

- **`RUNTIME=local` only** (`requiresLocalRuntime` in `scenario.json` —
  `pnpm sc up auth-methods --docker` refuses immediately with a clear
  error rather than hanging). OIDC discovery requires Consul, the browser,
  and Dex to all agree on the exact same issuer URL string. If Consul runs
  in its own Docker network, it needs a different hostname to reach Dex
  (`host.docker.internal`) than the browser does (`localhost`/`127.0.0.1`) —
  and the returned `issuer` has to exactly match whichever URL was used to
  fetch discovery, so that split breaks it. Running Consul as a local
  process sidesteps the whole problem: everyone uses `127.0.0.1`.
- **`docker-compose.yml` only defines `dex`**, not `consul`. `local.sh`
  starts Dex via `docker compose up -d` against that file, then starts
  Consul itself as a local binary — see `local.sh`.
- **Every URL is `127.0.0.1`, never `localhost`** — both for the OIDC
  issuer-matching reason above, and because Docker Desktop's IPv6 loopback
  forwarding is unreliable enough that "localhost" (which can resolve to
  `::1` first) intermittently fails where `127.0.0.1` doesn't.

## What it seeds

Three auth methods, spanning the type badges the list renders:

- **`dex-oidc`** (`oidc`) — the one type consul-ui's SSO tab can actually
  log in with. `tests/auth-methods/` drives this for real: click Log in →
  SSO tab → a genuine popup opens to Dex → fill in the login form → the
  popup closes itself → consul-ui ends up holding a real ACL token.
- **`dex-jwt`** (`jwt`) — same Dex IdP's JWKS, but the `jwt` auth-method
  type. **Consul-ui has no UI login path for this type at all** — the SSO
  tab only ever lists `oidc`-type methods. `seed.ts` still proves it works,
  just not through a browser: it drives Dex's login form purely over HTTP
  (no Playwright needed) and trades the resulting id_token for a real
  Consul token via `PUT /v1/acl/login`, and fails the seed if that doesn't
  bind the expected role. List/detail coverage only in the UI.
- **`k8s-demo`** (`kubernetes`) — no real cluster; pure list/detail
  type-badge variety, and the only one of the three exercising binding
  rules' `service` BindType + a templated `BindName`
  (`${serviceaccount.name}`) — the other two use `role`.

Login credentials for the Dex demo user (also printed by `pnpm sc up`):

```
demo@example.com / consul-demo-pw
```

## Running it

Enterprise only: the seeded auth methods carry Enterprise-only
`NamespaceRules`, which a CE agent rejects, and a production UI build only
offers SSO login when the agent reports Enterprise SSO support. Set
`CONSUL_BIN` and `CONSUL_LICENSE_PATH` (see `../../.env.example`).

```bash
pnpm sc up auth-methods --local
open http://localhost:8500/v1/acl/auth-methods    # (needs -H "X-Consul-Token: 1234")

# Playwright needs to load the UI from 127.0.0.1, not localhost — see above:
EDITION=ent E2E_BASE_URL=http://127.0.0.1:4200 pnpm e2e tests/auth-methods

pnpm sc down auth-methods --local
```

## Gotchas

- **Docker must be running** even though Consul itself is local — it's
  still used for the Dex sidecar.
- **Stale agents on :8500 from a *different* scenario** will make `up`
  silently talk to the wrong (already-running) agent instead of the fresh
  one it just started, since `local.sh` backgrounds the process without
  confirming it actually won the port. Symptom: seeding fails on a
  "already exists" collision, or the data just looks wrong. Check
  `lsof -iTCP:8500 -sTCP:LISTEN -n -P` and `docker ps` for leftovers from
  another scenario before filing this as a bug in this one.
- If `pnpm sc up` hangs waiting for Dex, check `docker compose -p
  consul-e2e-auth-methods-dex logs` in this directory.
- Two Consul API quirks this seeder/`ConsulClient` had to work around,
  worth knowing if you're debugging past this scenario: `type: "oidc"`
  auth methods require `MaxTokenTTL` set (undocumented, 400s without it),
  and `/v1/acl/login` is `POST` in practice despite being documented as
  `PUT` (`PUT` 405s).
