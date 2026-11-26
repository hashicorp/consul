/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { ConsulClient } from '../../lib/consul-client.ts';

/**
 * Seeds `/ui/dc1/acls/auth-methods` with three auth methods spanning the
 * type badges the list renders, and — the actual point of this scenario —
 * a real, working OIDC identity provider (Dex, started by `local.sh`) that
 * `tests/auth-methods/` logs into for real through the browser popup flow.
 *
 * ONE auth-method type is browser-loggable through consul-ui: `oidc`. The
 * SSO tab in the login dialog only ever lists `oidc`-type methods (`GET
 * /v1/internal/ui/oidc-auth-methods` — traced in `consul-ui`'s
 * `app/services/repository/oidc-provider.js`). A `jwt`-type method (raw
 * bearer-token validation — e.g. a workload's own JWT) has NO UI login
 * path; it's only reachable via `consul login -bearer-token=...` or
 * `PUT /v1/acl/login` directly. So:
 *
 *   - `dex-oidc` (type oidc)   — what `tests/auth-methods/` drives through
 *                                 a real popup login.
 *   - `dex-jwt`  (type jwt)    — same Dex IdP's JWKS, validated below via a
 *                                 real `consul.login()` (see the bottom of
 *                                 this file) so the scenario proves this
 *                                 type actually works too, just not through
 *                                 the UI. List/detail coverage only.
 *   - `k8s-demo` (type kubernetes) — no real cluster, just list/detail type-
 *                                 badge variety. Also the only one exercising
 *                                 binding rules' `service` BindType branch
 *                                 (the other two use `role`).
 *
 * All three also carry `NamespaceRules` (Enterprise only — Consul stores it
 * on CE too, just inertly, since only "default" exists there) for the
 * detail page's "Namespace Rules" tab. dex-oidc/dex-jwt's first rule always
 * matches the seeded demo user and routes to "default" — a deliberate
 * no-op so the real login flows below/in tests/auth-methods/ are
 * unaffected; their other rules, and all of k8s-demo's, are decorative
 * (never actually traversed by a real login) list/detail coverage.
 *
 * Every URL below is `127.0.0.1`, never `localhost`. Two independent reasons
 * force this:
 *   1. OIDC discovery is spec-strict: the `issuer` a provider returns must
 *      exactly equal the URL used to fetch it. Consul (server-side), the
 *      browser, and Dex all have to agree on ONE literal string.
 *   2. Even ignoring (1), Docker Desktop's IPv6 loopback forwarding is
 *      unreliable enough that "localhost" (which a dual-stack resolver may
 *      try `::1` for first) can fail intermittently where `127.0.0.1`
 *      doesn't.
 * This is also why this scenario only supports `RUNTIME=local`
 * (`requiresLocalRuntime` in scenario.json) — see README.md.
 *
 * Because the redirect URI baked into Dex/the auth method is
 * `http://127.0.0.1:4200/ui/oidc/callback`, running `tests/auth-methods/`
 * needs `E2E_BASE_URL=http://127.0.0.1:4200` (not the usual `localhost`) —
 * see README.md.
 */

/** Must match `local.sh`'s `initial_management` token. */
const MANAGEMENT_TOKEN = '1234';

/** Must match `dex-config.yaml`'s `issuer`. */
const DEX_ISSUER = 'http://127.0.0.1:5556/dex';
/** Must match `dex-config.yaml`'s `staticClients[0].redirectURIs`. */
const REDIRECT_URI = 'http://127.0.0.1:4200/ui/oidc/callback';
/** Same callback when the agent serves the UI itself (UI_SOURCE=agent). */
const AGENT_REDIRECT_URI = 'http://127.0.0.1:8500/ui/oidc/callback';
/** Must match `dex-config.yaml`'s `staticClients[0].id`/`.secret`. */
const CLIENT_ID = 'consul';
const CLIENT_SECRET = 'consul-secret';
/** Must match `dex-config.yaml`'s `staticPasswords[0]` (email + bcrypt hash of this). */
const DEMO_EMAIL = 'demo@example.com';
const DEMO_PASSWORD = 'consul-demo-pw';

const DEMO_ROLE_NAME = 'oidc-demo-role';

// Self-signed, not tied to any real cluster — Consul only checks this parses
// as a PEM certificate at auth-method creation time, unlike `oidc`'s
// `OIDCDiscoveryURL`, which it fetches live. Regenerate with:
//   openssl req -x509 -newkey rsa:2048 -nodes -keyout /dev/null -days 3650 -subj "/CN=k8s-demo-ca"
const K8S_DEMO_CA_CERT = `-----BEGIN CERTIFICATE-----
MIICqDCCAZACCQC4Bh43tOTOeDANBgkqhkiG9w0BAQsFADAWMRQwEgYDVQQDDAtr
OHMtZGVtby1jYTAeFw0yNjA4MjAwMzI2MjRaFw0zNjA4MTcwMzI2MjRaMBYxFDAS
BgNVBAMMC2s4cy1kZW1vLWNhMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKC
AQEA6F26nOXSz/wtl9kIMnaOkfEwsrFC8FuTeYVhE6oEOGDXKw4azPAVbQYKGlIa
7KwOgYq+bzZ/o3rOJqpDxJX60ihfVI6Y1ItQvvrplxu5jn41YHIbil9p13LwywjS
ZeDcNwioqHSpvKASKi8LMRjhksmykJu9KFNWMdZ9DPgXsqE52Q0rox8wyI6PS7O/
XNt5PVEhn9lfkWNCsB8QKMWerQKtaa526rk0ka2xiMVJJcEPea2fHmtZsLMwXxgu
rAPx+UB+Uuf1BLp01jVtcqbLMlBNQW+QTwJy688/qLC/cEbHDE/h0s0v9WCgx7Fw
o0hwfxBcARleqZ0iDw6rOftquwIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBp3mDQ
lJd4ZNpZedCZjra6JZ/FhKWiBQqQTMmmsio4mFYG8rHYphU8ZzvYyDxN2OpLAIRw
wNnyE0VDPLsJK4melqMFHd7XQuEHi1DyGaXIJ02WZP9W1JD0U6572NA8ICWMWiNP
SalJEZFD+gFVSLkbWy3AyrSkDMgUv2c2SQAqyFXQcTymgDX4ldlT29WyP2XAidHy
+Zn64hXGPhmvoGJn5BgL/+N3h7lNX6aIYVFdtUbUFW45XRqw1xkBM5qeeUnwF/Zu
cQF3ECtlqcmW8hwfdL2S3uKJgTx6PbJn1z2KK7n1WFOte6jZ1ayy0aC7wYDrZ5mz
46eY9iKaunRDpvGd
-----END CERTIFICATE-----`;

// Not a functioning credential — never presented to a real cluster, and
// Consul does not validate a `kubernetes`-type method's ServiceAccountJWT
// live at creation time. Just needs to look like a JWT.
const K8S_DEMO_DUMMY_JWT =
  'eyJhbGciOiJSUzI1NiIsImtpZCI6ImRlbW8ifQ.' +
  'eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6ZGVtbyJ9.' +
  'ZGVtby1zaWduYXR1cmU';

export async function seed(consul: ConsulClient): Promise<void> {
  consul.useToken(MANAGEMENT_TOKEN);

  // ── ACL policy + role bound by both real (Dex-backed) auth methods ──────
  const readPolicy = await consul.createPolicy({
    Name: 'auth-methods-demo-read',
    Description: 'Read-only access to nodes and services — bound by the demo auth methods.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }',
  });
  await consul.createRole({
    Name: DEMO_ROLE_NAME,
    Description: 'Bound by dex-oidc/dex-jwt binding rules on a successful Dex login.',
    Policies: [{ ID: readPolicy.ID }],
  });

  // ── dex-oidc: browser-loggable (SSO tab) — what tests/auth-methods/
  // drives through a real popup login ──────────────────────────────────────
  await consul.registerAuthMethod({
    Name: 'dex-oidc',
    Type: 'oidc',
    Description: "Real OIDC login against a local Dex IdP — the only auth-method type consul-ui's SSO tab lists.",
    // Required for type=oidc (Consul rejects the create without it — not
    // documented as required in the API docs, but it 400s in practice).
    MaxTokenTTL: '15m',
    Config: {
      OIDCDiscoveryURL: DEX_ISSUER,
      OIDCClientID: CLIENT_ID,
      OIDCClientSecret: CLIENT_SECRET,
      OIDCScopes: ['email'],
      BoundAudiences: [CLIENT_ID],
      AllowedRedirectURIs: [REDIRECT_URI, AGENT_REDIRECT_URI],
      ClaimMappings: { email: 'email' },
    },
    // Namespace-rules coverage for the detail page's "Namespace Rules" tab.
    // Rule 1 always matches the seeded demo user and routes to "default" —
    // a deliberate no-op so the real popup login below still lands where
    // tests/auth-methods/ expects it. Rule 2 is decorative (list/detail
    // rendering only): nothing in dex-config.yaml issues an admin@ email,
    // so it never actually fires.
    NamespaceRules: [
      { Selector: `value.email == "${DEMO_EMAIL}"`, BindNamespace: 'default' },
      { Selector: 'value.email matches "^admin@"', BindNamespace: 'platform-admin' },
    ],
  });
  await consul.createBindingRule({
    AuthMethod: 'dex-oidc',
    BindType: 'role',
    BindName: DEMO_ROLE_NAME,
    Selector: `value.email == "${DEMO_EMAIL}"`,
  });

  // ── dex-jwt: same IdP's JWKS, but the "jwt" type — bearer-token only, no
  // UI login path (see doc comment) — validated below via a real login ────
  await consul.registerAuthMethod({
    Name: 'dex-jwt',
    Type: 'jwt',
    Description: "Bearer-token validation against the same Dex IdP's JWKS — no UI login path, only `consul login`/the API.",
    Config: {
      OIDCDiscoveryURL: DEX_ISSUER,
      BoundAudiences: [CLIENT_ID],
      ClaimMappings: { email: 'email' },
    },
    // Same pattern as dex-oidc above: rule 1 always matches the demo user
    // and is a no-op (routes to "default"), so the real login()
    // round-trip at the bottom of this file is unaffected. Rule 2 is
    // decorative only — no svc- prefixed email is ever actually issued.
    NamespaceRules: [
      { Selector: `value.email == "${DEMO_EMAIL}"`, BindNamespace: 'default' },
      { Selector: 'value.email matches "^svc-.*@"', BindNamespace: 'service-accounts' },
    ],
  });
  await consul.createBindingRule({
    AuthMethod: 'dex-jwt',
    BindType: 'role',
    BindName: DEMO_ROLE_NAME,
    Selector: `value.email == "${DEMO_EMAIL}"`,
  });

  // ── k8s-demo: list/detail type-badge coverage only; also the only method
  // exercising binding rules' "service" BindType + templated BindName ─────
  await consul.registerAuthMethod({
    Name: 'k8s-demo',
    Type: 'kubernetes',
    Description: 'Kubernetes-type, for list/detail type-badge coverage — Host is not a real cluster.',
    Config: {
      Host: 'https://k8s-demo.example.com:443',
      CACert: K8S_DEMO_CA_CERT,
      ServiceAccountJWT: K8S_DEMO_DUMMY_JWT,
    },
    // Not exercised through a real login (no live k8s cluster), so free to
    // be the richest example: three rules echoing the binding rule below's
    // serviceaccount.namespace selector, incl. an explicit default->default
    // mapping plus two more BindNamespace targets for list/detail variety.
    NamespaceRules: [
      { Selector: 'serviceaccount.namespace==default', BindNamespace: 'default' },
      { Selector: 'serviceaccount.namespace==engineering', BindNamespace: 'engineering' },
      { Selector: 'serviceaccount.namespace==payments', BindNamespace: 'payments' },
    ],
  });
  await consul.createBindingRule({
    AuthMethod: 'k8s-demo',
    BindType: 'service',
    BindName: '${serviceaccount.name}',
    Selector: 'serviceaccount.namespace==default',
  });

  // ── verify dex-jwt for real, purely over HTTP (no browser): logs into
  // Dex's static-password connector the same way a human would in the
  // popup, but via fetch, then trades the resulting id_token for a real
  // Consul token via `consul.login()`. Proves the auth method + binding
  // rule genuinely resolve end-to-end — not just that the create calls
  // above succeeded — for the one type Playwright can't drive through a
  // browser. (`tests/auth-methods/` covers dex-oidc the real way, through
  // the actual popup.) ───────────────────────────────────────────────────
  const idToken = await loginToDexAndGetIdToken();
  const loginToken = await consul.login({ AuthMethod: 'dex-jwt', BearerToken: idToken });
  if (!loginToken.AccessorID) {
    throw new Error(`dex-jwt login did not return a token — found: ${JSON.stringify(loginToken)}`);
  }
  const roleNames = ((loginToken.Roles as Array<{ Name?: string }> | undefined) ?? []).map((r) => r.Name);
  if (!roleNames.includes(DEMO_ROLE_NAME)) {
    throw new Error(`dex-jwt login did not bind "${DEMO_ROLE_NAME}" — got roles: ${roleNames.join(', ') || '(none)'}`);
  }

  // ── cheap assertions the rest of the data landed ─────────────────────────
  const methodNames = (await consul.authMethods()).map((m) => m.Name);
  for (const name of ['dex-oidc', 'dex-jwt', 'k8s-demo']) {
    if (!methodNames.includes(name)) {
      throw new Error(`"${name}" auth method missing — found: ${methodNames.join(', ')}`);
    }
  }
  const rules = await consul.bindingRules();
  if (rules.length < 3) {
    throw new Error(`expected at least 3 binding rules, found ${rules.length}`);
  }

  console.log(`  UI login token (management): ${MANAGEMENT_TOKEN}`);
  console.log(`  Dex demo login: ${DEMO_EMAIL} / ${DEMO_PASSWORD}`);
}

/**
 * Drives Dex's static-password authorization-code flow purely over HTTP —
 * the exact sequence of requests a browser popup makes, minus the browser.
 * `fetch()` doesn't keep a cookie jar between calls the way a browser does,
 * so cookies are captured/replayed by hand across the redirect chain.
 *
 * Returns the id_token, ready to hand to `consul.login()` as a BearerToken.
 */
async function loginToDexAndGetIdToken(): Promise<string> {
  const cookies = new Map<string, string>();

  const step = async (url: string, init: RequestInit = {}): Promise<Response> => {
    const res = await fetch(url, {
      ...init,
      redirect: 'manual',
      headers: {
        ...init.headers,
        cookie: [...cookies].map(([k, v]) => `${k}=${v}`).join('; '),
      },
    });
    for (const raw of res.headers.getSetCookie?.() ?? []) {
      const [pair] = raw.split(';');
      const eq = pair.indexOf('=');
      cookies.set(pair.slice(0, eq), pair.slice(eq + 1));
    }
    return res;
  };

  const location = (res: Response, from: string): URL => {
    const loc = res.headers.get('location');
    if (!loc) throw new Error(`Dex login: expected a redirect from ${from}, got ${res.status} with no Location`);
    return new URL(loc, DEX_ISSUER);
  };

  const state = 'consul-ui-e2e-seed';
  const authUrl = `${DEX_ISSUER}/auth?${new URLSearchParams({
    client_id: CLIENT_ID,
    redirect_uri: REDIRECT_URI,
    response_type: 'code',
    scope: 'openid email',
    state,
  })}`;

  // /auth -> /auth/local (redirect to the static-password connector)
  let res = await step(authUrl);
  let url = location(res, '/auth');

  // /auth/local -> /auth/local/login?state=... (sets the login-session cookie)
  res = await step(url.toString());
  url = location(res, '/auth/local');

  // GET the login form (just to establish the session; static-password
  // connector doesn't need a CSRF token from the HTML itself)
  const loginFormUrl = url;
  await step(loginFormUrl.toString());

  // POST credentials -> redirect to /auth?req=...&hmac=... (approval)
  res = await step(loginFormUrl.toString(), {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ login: DEMO_EMAIL, password: DEMO_PASSWORD }),
  });
  url = location(res, 'POST /auth/local/login');

  // /auth?req=...&hmac=... -> our redirect_uri with ?code=...&state=...
  res = await step(url.toString());
  const finalLocation = res.headers.get('location');
  if (!finalLocation) throw new Error(`Dex login: expected the final redirect to ${REDIRECT_URI}, got ${res.status}`);
  const callback = new URL(finalLocation);
  const code = callback.searchParams.get('code');
  if (!code || callback.searchParams.get('state') !== state) {
    throw new Error(`Dex login: bad final redirect — ${finalLocation}`);
  }

  // Exchange the code for an id_token, same as Consul's own /v1/acl/oidc/callback would.
  const tokenRes = await fetch(`${DEX_ISSUER}/token`, {
    method: 'POST',
    headers: {
      'content-type': 'application/x-www-form-urlencoded',
      authorization: `Basic ${Buffer.from(`${CLIENT_ID}:${CLIENT_SECRET}`).toString('base64')}`,
    },
    body: new URLSearchParams({ grant_type: 'authorization_code', code, redirect_uri: REDIRECT_URI }),
  });
  if (!tokenRes.ok) {
    throw new Error(`Dex token exchange failed: ${tokenRes.status} ${await tokenRes.text()}`);
  }
  const { id_token: idToken } = (await tokenRes.json()) as { id_token?: string };
  if (!idToken) throw new Error('Dex token exchange did not return an id_token');
  return idToken;
}
