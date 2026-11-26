# Scenario: namespaces

Enterprise-only: seeds the **Namespaces** page
(`/ui/dc1/namespaces` — route name `dc.nspaces.index`, title "Namespaces",
gated by `can 'read nspaces'`) with a few named namespaces whose ACL defaults
are real policies/roles, plus real tokens minted under those namespaces so
`/ui/dc1/acls/tokens` and the per-namespace `Consul::Token::Ruleset::List`
badges have something to render.

| Namespace | Description | ACLs | Covers |
|---|---|---|---|
| `billing` | "Billing and payments team workloads" | `PolicyDefaults` + `RoleDefaults` | Policy + Role badges together. |
| `default` | *(implicit — always present)* | *(none)* | Un-deletable baseline, left alone. |
| `engineering` | "Engineering team workloads" | 2× `PolicyDefaults` (incl. the built-in `global-management`) + 2× `RoleDefaults` | The richest *narrative* row — Management + Policies + Rules badge groups all at once, and 2 roles in one row. |
| `operations` | "Operations team workloads (many attached policies/roles for badge-rendering checks)" | 10× `PolicyDefaults` + 10× `RoleDefaults` | Pure volume on ONE namespace (20 badges total) — how the Ruleset badges and the form's Policies/Roles fieldsets render/scroll when a single namespace has a lot attached, as opposed to list-level volume. |
| `platform` | "Platform infrastructure team workloads" | `RoleDefaults` only, no `PolicyDefaults` | The one branch nothing else covers — a role default with no policy default. |
| `security` | *(none)* | *(none)* | Description omitted and no Ruleset badges — the empty-details branch on both. Also left childless, so it's a safe target for a create-then-delete test. |
| `team-01` .. `team-50` | "Bulk-seeded namespace N of 50 for list volume/search testing." | *(none)* | 50 bulk-seeded namespaces — real volume for scroll behaviour and search/sort assertions at scale. Zero-padded so `Name:asc` string sort matches numeric order. |

Full alphabetical order (`Name:asc`, the page's default sort):
`billing`, `default`, `engineering`, `operations`, `platform`, `security`, `team-01`, `team-02`, …, `team-50`
(56 namespaces total).

## ACL policies, roles and tokens

The namespace create/edit form's Policies and Roles picker fieldsets
(`app/components/consul/nspace/form/index.hbs`) are hardcoded to
`@nspace="default"` — they only ever list policies/roles that live in the
`default` namespace, regardless of which namespace is being edited. So
everything referenced by any namespace's `ACLs.PolicyDefaults`/`RoleDefaults`
is created in `default` first:

| Object | Namespace | Notes |
|---|---|---|
| `namespace-readonly` (policy) | `default` | `node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }`. |
| `namespace-write` (policy) | `default` | `node_prefix "" { policy = "read" } service_prefix "" { policy = "write" }`. |
| `global-management` (policy, **built-in**) | `default` | Not created by this seed — Consul's own built-in policy, fixed ID `00000000-0000-0000-0000-000000000001` (`ConsulClient.MANAGEMENT_POLICY_ID`). Referencing it (rather than creating a custom policy) is the *only* way to get consul-ui's "Management" badge group to render — see `app/helpers/policy/group.js`. It's a cluster-wide singleton, always in `default`, never duplicated per namespace. |
| `namespace-operator` (role) | `default` | Wraps `namespace-write`; `RoleDefaults` on `engineering`. |
| `namespace-viewer` (role) | `default` | Wraps `namespace-readonly`; `RoleDefaults` on `engineering` (alongside `namespace-operator` — 2 roles on one namespace). |
| `namespace-auditor` (role) | `default` | Wraps `namespace-readonly`; `RoleDefaults` on `billing`. |
| `namespace-platform-admin` (role) | `default` | Wraps `namespace-write` + a `ServiceIdentity("platform")`; `RoleDefaults` on `platform`. The ServiceIdentity is visible on this role's own page — RoleDefaults badges on a *namespace* row don't decompose into identity sub-badges (only a token's own `Policies` do), so this is the only way to get an identity "into" a namespace's ACL defaults at all. |
| `operations-policy-01`..`operations-policy-10` (policies) | `default` | Trivial `node_prefix "" { policy = "read" }` each; all 10 are `PolicyDefaults` on `operations`. |
| `operations-role-01`..`operations-role-10` (roles) | `default` | Each 1:1-wraps the same-numbered policy above; all 10 are `RoleDefaults` on `operations`. |

Real tokens are minted *inside* the target namespaces (this is what actually
populates `/ui/dc1/acls/tokens` and the per-row Ruleset badges — the
namespace's `ACLs` defaults alone don't create any tokens). Tokens have a
**separate** constraint from the namespace-defaults case above: a token's own
`Policies`/`Roles` must live in the *same* namespace as the token — confirmed
against a real agent, `PUT /v1/acl/token` 500s with `No such ACL policy with
ID "..."` if handed a policy ID from another namespace (the built-in
`global-management` policy is no exception — a token can only reference it if
the token itself is in `default`). That cross-namespace reference is a
special case Consul only honours for a namespace's own
`ACLs.PolicyDefaults`/`RoleDefaults`, not for token creation. So the named
tokens below get their own namespace-local policies/role (`engineering-*`,
`billing-readonly` — separate objects from the `default`-namespace ones
above, despite the similar names/rules):

| Token | Namespace | Grant | Local |
|---|---|---|---|
| "Engineering read-only token" | `engineering` | `engineering-readonly` policy (namespace-local) | global |
| "Engineering operator token" | `engineering` | `engineering-operator` role (namespace-local, wraps `engineering-write`) | global |
| "Billing read-only token (local)" | `billing` | `billing-readonly` policy (namespace-local) | local |
| "Global management token" | `default` | built-in `global-management` policy — the "Management" badge, on a token this time | global |
| "Platform service identity token" | `platform` | `ServiceIdentities: [{ServiceName: "platform"}]` — the "Identities" badge | global |
| "Platform node identity token" | `platform` | `NodeIdentities: [{NodeName: "node-primary"}]` — the "Identities" badge | global |
| "Bulk-seeded token N of 50 …" (×50) | `team-01`..`team-50` | *(none — pure volume, a real policy per namespace would be 50 more calls for no UI-observable benefit)* | alternates by parity of N (even → local, odd → global) so the Local/Global column has real variation at scale |
| "Bulk-seeded token N of 20 …" (×20) | `default` | `bulk-policy-NN` (1:1, own policy per token) | alternates by parity of N |

On top of the 5 named policies above, 20 more — `bulk-policy-01`..`bulk-policy-20`,
all in `default`, each with a trivial `node_prefix "" { policy = "read" }`
rule — exist purely to give `/ui/dc1/acls/policies` real list volume and to
give the 20 bulk tokens above a real, distinct attached-policy badge each,
rather than sharing one policy across all of them.

**"Identity" policies aren't a real Consul resource** — worth calling out
since it's easy to assume otherwise. consul-ui's Ruleset badges classify an
entry as an identity purely by an ad-hoc `template` field the *frontend*
stamps onto normalized `ServiceIdentities`/`NodeIdentities` entries
(`app/mixins/policy/as-many.js`) — there's no way to create an actual
`Policy` resource of an "identity" type. Same story for "management": you
can't create your own — only the one built-in `global-management` policy
(above) gets that treatment, matched by its fixed ID.

## Requires a licensed Enterprise agent, with ACLs enabled

Like [`scenarios/admin-partitions`](../admin-partitions/README.md), this
needs:
- A **Consul Enterprise binary** (or Docker image) — `/v1/namespace*` 404s on
  CE with `"Namespaces are a Consul Enterprise feature"`.
- A **license**.
- **ACLs enabled, with a fixed management token** — new for this scenario,
  since it's the first one to need the `/ui/dc1/acls/*` pages and the
  namespace form's Policies/Roles pickers, both of which are gated on the
  agent's `ACLsEnabled` operator config. **Important:** `default_policy =
  "allow"` does *not* extend to the `acl` resource type — confirmed against a
  real agent, the anonymous token 403s on `acl:write`/`acl:read` regardless
  of the default policy. So the agent is configured with a fixed
  `acl.tokens.initial_management` value instead of bootstrapping one at seed
  time (which would mint a fresh, unpredictable token every run — no good for
  a human to paste into the UI):

  ```
  acl {
    enabled = true
    default_policy = "allow"
    tokens { initial_management = "1234" }
  }
  ```

  Same literal value in `docker-compose.yml`, `local.sh`, and
  `MANAGEMENT_TOKEN` in `seed.ts` — if you change one, change all three.
  `seed.ts` calls `consul.useToken(MANAGEMENT_TOKEN)` before any ACL call,
  and prints the token again at the end of a successful seed so it's visible
  in `pnpm sc up` output. It's a throwaway dev-mode fixture, not a real
  secret — the agent is in-memory and torn down between runs.

  `1234` isn't a UUID, which the agent logs as deprecated
  (`Configuring a non-UUID management token is deprecated`) but still honours
  fully — confirmed working end-to-end against a real agent. If a future
  Consul version starts rejecting non-UUID tokens outright, swap it for any
  UUID-shaped string in all three files.

**Verified path: `RUNTIME=local`.** Set in `.env` (or inline):

```bash
CONSUL_BIN=/path/to/consul-enterprise/bin/consul
CONSUL_LICENSE_PATH=/path/to/your.hclic
```

then:

```bash
RUNTIME=local pnpm sc up namespaces
```

`local.sh` fails fast with a clear message if either var is unset or the
paths don't resolve, rather than a cryptic agent-startup error.

**`RUNTIME=docker` is scaffolded but unverified** — `docker-compose.yml`
references `hashicorp/consul-enterprise:1.20-ent` and passes the license
text through the `CONSUL_LICENSE` environment variable (never committed), but pulling that image needs registry access this repo doesn't
assume you have. Adjust the `image:` line to whatever registry you actually
use, or just use `RUNTIME=local`.

- Does **not** require the Connect CA (`requiresConnect: false`) — namespaces
  don't need sidecars.
- The `create` route (`/ui/dc1/namespaces/create`) is gated by
  `can 'create nspaces'` — the same license check, so it renders once the
  agent is licensed.

## Log in to see the ACL pages

The Namespaces list works with no login. But `/ui/dc1/acls/*` and the
namespace create/edit form's Policies/Roles **picker** dropdowns need the
browser session logged in — click "Log in" in the top nav and paste:

```
1234
```

(same value `seed.ts` prints at the end of a successful `pnpm sc up
namespaces`). See below for exactly what does and doesn't need this.

## Confirmed: unauthenticated UI session cannot see ACL data

Verified end-to-end against a real licensed Enterprise agent
(`RUNTIME=local`, seeded via `pnpm sc up namespaces`). Two different
things are true here, and they don't split the way you'd guess:

- **The Namespaces list/edit pages work with no login at all.**
  `GET /v1/namespaces` and `GET /v1/namespace/:name` are *not* gated by
  `acl:read` — they return fine for the anonymous token (confirmed by
  `curl` with no `X-Consul-Token`), and the response already embeds each
  namespace's full `ACLs.PolicyDefaults`/`RoleDefaults` **with names**. So
  the namespace list, its per-row `Consul::Token::Ruleset::List` badges, and
  the read-only view of an existing namespace's Policies/Roles fieldsets all
  render correctly for an anonymous Playwright session — no UI login needed.
- **Everything under the `acl` resource type does not.** `GET
  /v1/acl/policies`, `GET /v1/acl/roles`, `GET /v1/acl/tokens`, and
  `GET /v1/acl/token/self` all 403/404 for the anonymous token regardless of
  `default_policy = "allow"` (same special-casing as the write side, see
  above) — confirmed by `curl` with no token: `Permission denied: anonymous
  token lacks permission 'acl:read'`. This means, without the browser session
  holding a real token:
  - `/ui/dc1/acls/{tokens,policies,roles}` will render empty/gated, not the
    seeded data.
  - The namespace **create/edit form's** Policies/Roles *picker* fieldsets
    (`PolicySelector`/`RoleSelector`, which fetch `/v1/acl/policies?ns=default`
    / `/v1/acl/roles?ns=default` live to populate the dropdown — a separate
    code path from the read-only list view above) will also render
    empty/gated, even though the already-attached defaults display fine.

**For future test-writing:** tests against `/ui/dc1/namespaces` (list, and
read-only viewing of an existing namespace's attached Policies/Roles) need no
special setup. Tests against `/ui/dc1/acls/*` or the create/edit form's
pickers need the Playwright session logged into the UI first with
`MANAGEMENT_TOKEN` (`1234`, see "Log in"
above) — e.g. via the UI's own login form, or by seeding `localStorage`
directly before navigating (check what key `consul-ui`'s login flow actually
uses before assuming).

- UI: `http://localhost:4200/ui/dc1/namespaces`
- Tests: not yet written — see [`docs/adding-a-test.md`](../../docs/adding-a-test.md)
  and `tests/intentions/` for the pattern to follow.
