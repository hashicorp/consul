/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { waitFor } from './wait.ts';

/**
 * Consul's built-in "global-management" policy — server-generated at ACL
 * bootstrap with this exact, fixed ID, always in the "default" namespace
 * (confirmed against a real agent: it's a cluster-wide singleton, not
 * duplicated per namespace like the auto-created "namespace-management"
 * policies are). consul-ui's `policy/group` helper special-cases this exact
 * ID to render the "Management" badge group
 * (`app/helpers/policy/group.js`/`app/models/policy.js` MANAGEMENT_ID) — it's
 * the only way to get that badge to show, you can't create your own
 * "management" policy. Same cross-namespace-reference rule as any other
 * "default"-namespace policy applies: a token can only reference it if the
 * token itself lives in "default".
 */
export const MANAGEMENT_POLICY_ID = '00000000-0000-0000-0000-000000000001';

/**
 * Thin wrapper over the Consul HTTP API — centralizes the calls the old
 * `dev-*.sh` scripts made by hand (register connect service + sidecar, create
 * sessions, register catalog nodes) so scenario `seed.ts` files stay small.
 */
/**
 * Every scenario's agent serves HTTP here. It's fixed, and deliberately not
 * read from CONSUL_HTTP_ADDR / CONSUL_HTTP_TOKEN: those often point a
 * developer's shell at a real cluster, which seeding must never touch.
 */
export const AGENT_ADDR = 'http://127.0.0.1:8500';

export class ConsulClient {
  readonly addr: string;
  private token?: string;

  /** Defaults to the scenario agent, with no token; see `useToken`. */
  constructor(addr = AGENT_ADDR) {
    this.addr = addr.replace(/\/$/, '');
  }

  /**
   * Switch the token used for subsequent requests — e.g. a scenario whose
   * agent is configured with a fixed `acl.tokens.initial_management` value
   * (see `scenarios/namespaces`) calling this once at the top of `seed()`,
   * since a single long-lived client can't take a token in its constructor
   * mid-run.
   */
  useToken(token: string): void {
    this.token = token || undefined;
  }

  private async request(method: string, path: string, body?: unknown): Promise<Response> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.token) headers['X-Consul-Token'] = this.token;

    const res = await fetch(`${this.addr}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`Consul ${method} ${path} -> ${res.status} ${res.statusText}${text ? `: ${text}` : ''}`);
    }
    return res;
  }

  private async json<T>(method: string, path: string, body?: unknown): Promise<T> {
    return (await this.request(method, path, body)).json() as Promise<T>;
  }

  // ── readiness ──────────────────────────────────────────────────────────────

  /** Wait until the agent has a leader (API is serving). */
  async waitReady(timeoutMs = 30_000): Promise<void> {
    await waitFor(
      async () => {
        const res = await fetch(`${this.addr}/v1/status/leader`);
        return res.ok;
      },
      { timeoutMs, label: 'Consul leader' },
    );
  }

  /** Wait for the Connect CA roots — required before SidecarService registration. */
  async waitConnectCA(timeoutMs = 30_000): Promise<void> {
    await waitFor(
      async () => {
        const roots = await this.json<{ Roots?: unknown[] }>('GET', '/v1/connect/ca/roots');
        return (roots.Roots?.length ?? 0) > 0;
      },
      { timeoutMs, label: 'Connect CA roots' },
    );
  }

  /**
   * Wait for a valid Enterprise license — required before namespace/partition
   * endpoints stop 404ing. Fails fast with the license API's own error text
   * (e.g. "No license configured") rather than a generic timeout, so a missing
   * `CONSUL_LICENSE_PATH` / bad license is obvious immediately.
   */
  async waitLicensed(timeoutMs = 30_000): Promise<void> {
    let lastError: string | undefined;
    try {
      await waitFor(
        async () => {
          try {
            await this.json('GET', '/v1/operator/license');
            return true;
          } catch (err) {
            lastError = (err as Error).message;
            return false;
          }
        },
        { timeoutMs, label: 'Consul Enterprise license' },
      );
    } catch (err) {
      throw new Error(`${(err as Error).message}${lastError ? ` — last response: ${lastError}` : ''}`);
    }
  }

  // ── introspection ───────────────────────────────────────────────────────────

  /** The live agent's node name — sessions must be created against this. */
  /** The agent's version, build revision and edition (Enterprise versions end in `+ent`). */
  async agentEdition(): Promise<{ version: string; revision: string; edition: 'ce' | 'ent' }> {
    const self = await this.json<{ Config: { Version: string; Revision?: string } }>('GET', '/v1/agent/self');
    const { Version: version, Revision: revision = '' } = self.Config;
    return { version, revision, edition: version.includes('+ent') ? 'ent' : 'ce' };
  }

  async nodeName(): Promise<string> {
    const self = await this.json<{ Config: { NodeName: string } }>('GET', '/v1/agent/self');
    return self.Config.NodeName;
  }

  async services(): Promise<Record<string, unknown>> {
    return this.json('GET', '/v1/catalog/services');
  }

  async agentService(id: string): Promise<{ Kind?: string } & Record<string, unknown>> {
    return this.json('GET', `/v1/agent/service/${id}`);
  }

  async nodeSessions(node: string): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/session/node/${node}`);
  }

  async catalogNodes(): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', '/v1/catalog/nodes');
  }

  /**
   * All intentions in `dc` (default dc1). No `ns` param: Consul OSS rejects
   * it outright on this endpoint ("Namespaces are a Consul Enterprise
   * feature") — even `ns=default` — unlike other endpoints that just ignore it.
   * Enterprise scenarios that need cross-namespace results should pass
   * `ns=*` explicitly via a second argument if that's ever needed.
   */
  async intentions(dc = 'dc1'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/connect/intentions?dc=${dc}`);
  }

  /** Enterprise only. */
  async namespaces(): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', '/v1/namespaces');
  }

  /** Enterprise only. */
  async partitions(): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', '/v1/partitions');
  }

  /**
   * ACL policies are a CE feature; `Namespace`/`Partition` scoping is
   * Enterprise only. Defaults to `ns=*` (all namespaces) so bulk-seeded
   * per-namespace policies are all visible in one call.
   */
  async policies(ns = '*'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/acl/policies?ns=${ns}`);
  }

  /**
   * ACL roles are a CE feature; `Namespace`/`Partition` scoping is
   * Enterprise only. Defaults to `ns=*` (all namespaces).
   */
  async roles(ns = '*'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/acl/roles?ns=${ns}`);
  }

  /**
   * ACL tokens are a CE feature; `Namespace`/`Partition` scoping is
   * Enterprise only. Defaults to `ns=*` (all namespaces).
   */
  async tokens(ns = '*'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/acl/tokens?ns=${ns}`);
  }

  /**
   * ACL auth methods are a CE feature; `Namespace`/`Partition` scoping is
   * Enterprise only. Defaults to `ns=*` (all namespaces).
   */
  async authMethods(ns = '*'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/acl/auth-methods?ns=${ns}`);
  }

  /** Binding rules for a given auth method (or all, if omitted). */
  async bindingRules(authMethod?: string, ns = '*'): Promise<Array<Record<string, unknown>>> {
    const query = new URLSearchParams({ ns });
    if (authMethod) query.set('authmethod', authMethod);
    return this.json('GET', `/v1/acl/binding-rules?${query}`);
  }

  // ── mutations ───────────────────────────────────────────────────────────────

  /** Register a service on the local agent (auto-registers a sidecar if Connect.SidecarService is set). */
  async registerService(service: Record<string, unknown>): Promise<void> {
    await this.request('PUT', '/v1/agent/service/register', service);
  }

  /** Create a session; returns its ID. Omit `TTL` to create a session without one. */
  async createSession(session: Record<string, unknown>): Promise<string> {
    const out = await this.json<{ ID: string }>('PUT', '/v1/session/create', session);
    return out.ID;
  }

  /** Register a catalog node (with optional Service and Checks). */
  async registerCatalogNode(entry: Record<string, unknown>): Promise<void> {
    await this.request('PUT', '/v1/catalog/register', entry);
  }

  /**
   * Register/update a config entry (e.g. `service-defaults` to set a
   * destination's `Protocol` — required before it can carry Layer 7 intention
   * permissions; Consul otherwise rejects them with
   * `has protocol "tcp", which is incompatible with L7 intentions permissions`).
   */
  async registerConfigEntry(entry: Record<string, unknown>): Promise<void> {
    await this.request('PUT', '/v1/config', entry);
  }

  /** Delete a config entry. Deleting one that doesn't exist is a no-op in Consul. */
  async deleteConfigEntry(kind: string, name: string): Promise<void> {
    await this.request('DELETE', `/v1/config/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`);
  }

  /**
   * Enterprise only. Defaults to the "default" partition unless `Partition`
   * is set. `ACLs.PolicyDefaults`/`RoleDefaults` mirror what the namespace
   * create/edit form actually sends (`app/adapters/nspace.js`) — just the
   * `{ID}` of policies/roles picked in the form, nothing else.
   */
  async createNamespace(payload: {
    Name: string;
    Description?: string;
    Partition?: string;
    ACLs?: { PolicyDefaults?: Array<{ ID: string }>; RoleDefaults?: Array<{ ID: string }> };
  }): Promise<void> {
    await this.request('PUT', '/v1/namespace', payload);
  }

  /** Enterprise only. Creating a partition also creates its own "default" namespace. */
  async createPartition(payload: { Name: string; Description?: string }): Promise<void> {
    await this.request('PUT', '/v1/partition', payload);
  }

  /**
   * ACL policies are a CE feature; `Namespace`/`Partition` are Enterprise
   * only. Returns the created policy, notably its `ID` — needed to reference
   * the policy from a role, a token, or a namespace's `ACLs.PolicyDefaults`.
   *
   * `acl:write` is never covered by `default_policy = "allow"` — unlike
   * every other resource type, ACL writes always require an explicit token
   * with `acl:write` (confirmed against a real agent: the anonymous token
   * 403s here even with `default_policy = "allow"` set). Call `useToken()`
   * with a token that actually has `acl:write` — e.g. a fixed
   * `acl.tokens.initial_management` value configured on the agent, see
   * `scenarios/namespaces` — before calling this.
   */
  async createPolicy(payload: {
    Name: string;
    Description?: string;
    Rules: string;
    Namespace?: string;
    Partition?: string;
  }): Promise<{ ID: string } & Record<string, unknown>> {
    return this.json('PUT', '/v1/acl/policy', payload);
  }

  /**
   * ACL roles are a CE feature; `Namespace`/`Partition` are Enterprise only.
   * Returns the created role, notably its `ID` — needed to reference the
   * role from a token or a namespace's `ACLs.RoleDefaults`.
   */
  async createRole(payload: {
    Name: string;
    Description?: string;
    Policies?: Array<{ ID: string }>;
    ServiceIdentities?: Array<{ ServiceName: string; Datacenters?: string[] }>;
    NodeIdentities?: Array<{ NodeName: string; Datacenter: string }>;
    Namespace?: string;
    Partition?: string;
  }): Promise<{ ID: string } & Record<string, unknown>> {
    return this.json('PUT', '/v1/acl/role', payload);
  }

  /**
   * ACL tokens are a CE feature; `Namespace`/`Partition` are Enterprise only.
   * Returns the created token, notably `AccessorID`/`SecretID`.
   */
  async createToken(payload: {
    Description?: string;
    Policies?: Array<{ ID: string }>;
    Roles?: Array<{ ID: string }>;
    ServiceIdentities?: Array<{ ServiceName: string; Datacenters?: string[] }>;
    NodeIdentities?: Array<{ NodeName: string; Datacenter: string }>;
    Local?: boolean;
    Namespace?: string;
    Partition?: string;
  }): Promise<{ AccessorID: string; SecretID: string } & Record<string, unknown>> {
    return this.json('PUT', '/v1/acl/token', payload);
  }

  /**
   * ACL auth methods are a CE feature; `Namespace`/`Partition` are
   * Enterprise only. `Config` is type-specific — e.g. for `oidc`:
   * `OIDCDiscoveryURL`/`OIDCClientID`/`OIDCClientSecret`/`OIDCScopes`,
   * `BoundAudiences`, `AllowedRedirectURIs`, `ClaimMappings`. For
   * `kubernetes`: `Host`/`CACert`/`ServiceAccountJWT`. Consul validates
   * `oidc`'s `OIDCDiscoveryURL` live (a real outbound HTTP GET) at creation
   * time — a wrong/unreachable URL fails the PUT immediately rather than at
   * login time, so this is a good fast-fail check for a scenario's own
   * cheap assertion.
   */
  async registerAuthMethod(payload: {
    Name: string;
    Type: 'oidc' | 'jwt' | 'kubernetes' | 'aws-iam' | string;
    Description?: string;
    Config: Record<string, unknown>;
    /** Required for type=oidc in practice, despite not being documented as such. */
    MaxTokenTTL?: string;
    Namespace?: string;
    Partition?: string;
    /**
     * Enterprise only (namespaces). Controls which namespace a token minted
     * by this auth method lands in — separate from (and evaluated before)
     * binding rules, which only ever run inside whichever namespace this
     * resolves to. First matching rule wins; an empty `Selector` always
     * matches, so put a catch-all last. Consul accepts and stores this on
     * CE too (doesn't validate `BindNamespace` exists at create time
     * either) — it's just inert there, since only "default" exists.
     */
    NamespaceRules?: Array<{ Selector?: string; BindNamespace: string }>;
  }): Promise<Record<string, unknown>> {
    return this.json('PUT', '/v1/acl/auth-method', payload);
  }

  /**
   * Binding rules translate a successful auth-method login into ACL
   * roles/service identities. `BindName` may reference `Selector` capture
   * groups or (for OIDC) claim values via `${value.<claim>}` templating —
   * see `docs/` / Consul's binding-rule docs for the exact syntax per type.
   */
  async createBindingRule(payload: {
    AuthMethod: string;
    Description?: string;
    Selector?: string;
    BindType: 'service' | 'role' | 'node' | 'templated-policy';
    BindName: string;
    Namespace?: string;
    Partition?: string;
  }): Promise<Record<string, unknown>> {
    return this.json('PUT', '/v1/acl/binding-rule', payload);
  }

  /**
   * Exchange a bearer token (a raw JWT for `jwt`/`kubernetes`-type methods,
   * or an OIDC provider's id_token) for a real Consul ACL token via the
   * given auth method's binding rules. Useful as a seeder's own cheap
   * assertion that an auth method + its binding rules actually resolve to a
   * usable token end-to-end, not just that the API accepted their shape —
   * see `scenarios/auth-methods` for a full example against a real IdP.
   */
  async login(payload: {
    AuthMethod: string;
    BearerToken: string;
    Meta?: Record<string, string>;
    Namespace?: string;
    Partition?: string;
  }): Promise<{ AccessorID: string; SecretID: string } & Record<string, unknown>> {
    // POST, not PUT — unlike every other ACL write in this file. Consul's
    // docs have historically shown PUT, but PUT 405s in practice; POST is
    // what actually works (and consul-ui's own oidc-provider adapter/CLI
    // `consul login` both use POST too).
    return this.json('POST', '/v1/acl/login', payload);
  }

  /**
   * Create/update an intention via the exact-match endpoint (mirrors the
   * consul-ui write path in `adapters/intention.js`). Namespace/partition
   * default to `default` (OSS); pass an `Action` for Allow/Deny, or
   * `Permissions` for an Application-Aware (Layer 7) intention — never both.
   */
  async registerIntention(
    intention: {
      SourceName: string;
      DestinationName: string;
      SourceNS?: string;
      DestinationNS?: string;
      SourcePartition?: string;
      DestinationPartition?: string;
      Description?: string;
    } & ({ Action: 'allow' | 'deny'; Permissions?: never } | { Action?: never; Permissions: Array<Record<string, unknown>> }),
    dc = 'dc1',
  ): Promise<void> {
    const {
      SourceName,
      DestinationName,
      SourceNS = 'default',
      DestinationNS = 'default',
      SourcePartition = 'default',
      DestinationPartition = 'default',
      ...rest
    } = intention;
    const query = intentionQuery(
      { SourceName, SourceNS, SourcePartition },
      { DestinationName, DestinationNS, DestinationPartition },
      dc,
    );
    await this.request('PUT', `/v1/connect/intentions/exact?${query}`, {
      SourceName,
      DestinationName,
      SourceNS,
      DestinationNS,
      SourcePartition,
      DestinationPartition,
      ...rest,
    });
  }

  /** Delete an intention by its Source/Destination pair (defaults mirror `registerIntention`). */
  async deleteIntention(
    pair: {
      SourceName: string;
      DestinationName: string;
      SourceNS?: string;
      DestinationNS?: string;
      SourcePartition?: string;
      DestinationPartition?: string;
    },
    dc = 'dc1',
  ): Promise<void> {
    const {
      SourceName,
      DestinationName,
      SourceNS = 'default',
      DestinationNS = 'default',
      SourcePartition = 'default',
      DestinationPartition = 'default',
    } = pair;
    const query = intentionQuery(
      { SourceName, SourceNS, SourcePartition },
      { DestinationName, DestinationNS, DestinationPartition },
      dc,
    );
    await this.request('DELETE', `/v1/connect/intentions/exact?${query}`);
  }

  // ── kv ──────────────────────────────────────────────────────────────────

  /**
   * Write a single KV entry. Consul's KV API takes the **raw** value as the
   * PUT body (not JSON-encoded) — unlike every other write in this file —
   * so this bypasses `request()`/`json()` (which always `JSON.stringify`
   * the body) and does its own fetch. A key ending in `/` written with an
   * empty value creates an empty "folder" marker: consul-ui's KV browser
   * derives folders from `/`-separated key prefixes on the fly, so a
   * populated folder (e.g. `app/settings/theme`) never needs its own
   * explicit `app/` / `app/settings/` marker key — only a folder with *no*
   * other keys under it needs one, otherwise nothing would ever list at
   * that prefix.
   */
  async putKV(key: string, value: string): Promise<void> {
    const headers: Record<string, string> = {};
    if (this.token) headers['X-Consul-Token'] = this.token;

    const res = await fetch(`${this.addr}/v1/kv/${key}`, {
      method: 'PUT',
      headers,
      body: value,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`Consul PUT /v1/kv/${key} -> ${res.status} ${res.statusText}${text ? `: ${text}` : ''}`);
    }
  }

  /**
   * All KV entries at/under `prefix` (default: every key in the store).
   * Consul's `?recurse` KV read returns each `Value` base64-encoded on the
   * wire; this decodes it back to a plain string so a seeder's cheap
   * assertion can compare against what it wrote. `Value` is `null` for a
   * key that was PUT with an empty body (folder markers included).
   */
  async kv(prefix = ''): Promise<Array<{ Key: string; Value: string | null }>> {
    const out = await this.json<Array<{ Key: string; Value: string | null }>>('GET', `/v1/kv/${prefix}?recurse=true`);
    return out.map((entry) => ({
      Key: entry.Key,
      Value: entry.Value == null ? null : Buffer.from(entry.Value, 'base64').toString('utf8'),
    }));
  }

  // ── peering ─────────────────────────────────────────────────────────────

  /**
   * Generate a peering token on this agent — the "acceptor" side of a
   * cluster peering. The returned token embeds this cluster's advertised
   * gRPC(-TLS) server address(es); whichever cluster calls
   * `establishPeering()` with it must actually be able to dial that
   * address, so the two agents need real network reachability to each
   * other first (see `scenarios/peering`'s README for the local/Docker
   * setup this requires — it's the hard part). Calling this also
   * immediately creates a `PENDING` entry on this side in `peerings()`,
   * even before any dialer ever establishes against it — handy for seeding
   * a second, deliberately-never-established peer to get status-badge
   * variety in the UI's peers list.
   */
  async generatePeeringToken(payload: { PeerName: string; Meta?: Record<string, string> }): Promise<{ PeeringToken: string }> {
    return this.json('POST', '/v1/peering/token', payload);
  }

  /**
   * Consume a peering token on this agent — the "dialer" side. Call this
   * against the *other* cluster's `ConsulClient` than the one
   * `generatePeeringToken()` was called on (they're two separate Consul
   * processes/addresses in `scenarios/peering`, unlike every other scenario's
   * single agent). Resolves once the establish request is accepted, not
   * once the peering is actually connected — poll `peerings()` for
   * `State: "ACTIVE"` via `lib/wait.ts`'s `waitFor` for that.
   */
  async establishPeering(payload: { PeerName: string; PeeringToken: string; Meta?: Record<string, string> }): Promise<void> {
    await this.request('POST', '/v1/peering/establish', payload);
  }

  /**
   * All peerings known to this agent. Each entry's `State` is one of
   * `PENDING` / `ESTABLISHING` / `ACTIVE` / `FAILING` / `TERMINATED` /
   * `DELETING` (mirrors consul-ui's `models/peer.js`), and `StreamStatus`
   * carries the imported/exported service name lists plus heartbeat
   * timestamps once the stream has actually connected.
   */
  async peerings(): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', '/v1/peerings');
  }

  /** A single peering by name — same shape as one entry of `peerings()`. */
  async peering(name: string): Promise<Record<string, unknown>> {
    return this.json('GET', `/v1/peering/${name}`);
  }

  /** Delete (unpeer) a peering by name. Soft delete: `State` moves to `DELETING` before the entry actually disappears. */
  async deletePeering(name: string): Promise<void> {
    await this.request('DELETE', `/v1/peering/${name}`);
  }

  // ── internal UI endpoints ────────────────────────────────────────────────

  /**
   * The exact endpoint backing the service detail page's "Linked Services"
   * tab (`app/templates/dc/services/show/services.hbs`, gated to
   * `Service.Kind === 'terminating-gateway'`): resolves the `terminating-gateway`
   * config entry named `gateway`, expands its `Services[]` (or a `{Name: "*"}`
   * wildcard) against the real catalog, and returns one row per linked
   * service name with instance/check/tag/external-source data already
   * aggregated server-side — exactly what `Consul::Service::List` renders.
   * Good as a scenario's own cheap assertion that the config entry + backing
   * services actually resolve into gateway rows before Playwright starts.
   */
  async gatewayServicesNodes(gateway: string, dc = 'dc1'): Promise<Array<Record<string, unknown>>> {
    return this.json('GET', `/v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}`);
  }
}

function intentionQuery(
  source: { SourceName: string; SourceNS: string; SourcePartition: string },
  destination: { DestinationName: string; DestinationNS: string; DestinationPartition: string },
  dc: string,
): URLSearchParams {
  return new URLSearchParams({
    source: `${source.SourcePartition}/${source.SourceNS}/${source.SourceName}`,
    destination: `${destination.DestinationPartition}/${destination.DestinationNS}/${destination.DestinationName}`,
    dc,
  });
}
