/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { MANAGEMENT_POLICY_ID, type ConsulClient } from '../../lib/consul-client.ts';

/**
 * Enterprise-only: seeds the Namespaces page (`/ui/dc1/namespaces`,
 * route `dc.nspaces.index`, gated by `can 'read nspaces'`) with named
 * namespaces whose ACL defaults are real policies/roles, plus real tokens
 * minted under those namespaces — the latter is what actually populates
 * `/ui/dc1/acls/tokens` and the per-namespace `Consul::Token::Ruleset::List`
 * badges rendered on each namespace row.
 *
 * Policies/role live in the "default" namespace, NOT in the namespaces they
 * end up attached to. This is non-obvious but required: the namespace
 * create/edit form's Policies and Roles picker fieldsets
 * (`app/components/consul/nspace/form/index.hbs`) are hardcoded to
 * `@nspace="default"` — they only ever list policies/roles that live in
 * `default`, no matter which namespace is being edited. So anything meant to
 * be pickable as a namespace's `ACLs.PolicyDefaults`/`RoleDefaults` has to be
 * created in `default` first.
 *
 * The namespace write payload only ever sends `{ID}` for policy/role
 * defaults (`app/adapters/nspace.js`) — not `Name` — so this seed captures
 * the `.ID` returned from `createPolicy`/`createRole` and threads it through
 * rather than re-deriving anything by name.
 *
 * A SEPARATE gotcha applies to tokens (confirmed against a real agent): a
 * token's own `Policies`/`Roles` must live in the *same* namespace as the
 * token — `PUT /v1/acl/token` 500s with `No such ACL policy with ID "..."`
 * if you hand it a policy ID from another namespace. That cross-namespace
 * reference is a special case Consul only honours for a namespace's own
 * `ACLs.PolicyDefaults`/`RoleDefaults`, not for ad-hoc token creation. So the
 * "default"-namespace policies/role above are for namespace ACL defaults
 * ONLY — tokens get their own namespace-local policies/role, created after
 * their namespace exists.
 *
 * Namespaces (Name:asc, the page's default sort):
 *
 *   billing     — Description, PolicyDefaults + RoleDefaults
 *   default     — (implicit, un-deletable, left alone)
 *   engineering — Description, 2 PolicyDefaults (incl. the built-in
 *                 "global-management" — see MANAGEMENT_POLICY_ID below) and
 *                 2 RoleDefaults — the richest row, exercises every Ruleset
 *                 badge group (Management + Policies + Rules) at once
 *   platform    — Description, RoleDefaults ONLY, no PolicyDefaults — the
 *                 one branch nothing else covers
 *   security    — no Description, no ACLs — exercises the empty-details
 *                 branch on both the description and the Ruleset badges;
 *                 left childless so it's a safe create-then-delete target
 *                 (same rationale as admin-partitions' "security" partition)
 *   team-01..team-50 — bulk, zero-padded, no ACLs — list volume/search
 *                 testing at scale, identical pattern to admin-partitions'
 *                 team-01..team-50 partitions
 *
 * Roles, all in "default" (same "@nspace=default" reason as the policies
 * above), spread across 3 namespaces' RoleDefaults so more than one row
 * shows a Rules badge, and one row shows two:
 *   namespace-operator       — wraps namespace-write         — engineering
 *   namespace-viewer         — wraps namespace-readonly       — engineering
 *   namespace-auditor        — wraps namespace-readonly       — billing
 *   namespace-platform-admin — wraps namespace-write + a
 *                               ServiceIdentity("platform")   — platform
 *
 * "operations" is a separate, dedicated namespace (not a variant of the ones
 * above) that gets 10 policies + 10 roles ALL attached at once, 1:1-wrapped
 * (operations-role-NN wraps operations-policy-NN) — purely to check how the
 * Ruleset badges/Policies+Roles fieldsets render/scroll with real volume on
 * a single namespace, as opposed to the list-level volume the team-NN bulk
 * namespaces exist for.
 *
 * "Identity" policies aren't a real Consul resource — consul-ui's Ruleset
 * badges classify an entry as an identity purely by an ad-hoc `template`
 * field the frontend stamps onto normalized `ServiceIdentities`/
 * `NodeIdentities` entries (`app/mixins/policy/as-many.js`), never by
 * creating an actual `Policy`. Two places carry one:
 *   - the "namespace-platform-admin" role above (ServiceIdentity, visible on
 *     the role's own page — RoleDefaults badges on the *namespace* row don't
 *     decompose into identity sub-badges, only a token's own Policies do,
 *     see `app/components/consul/token/ruleset/list/index.hbs`)
 *   - two of the named tokens below (one ServiceIdentity, one NodeIdentity)
 *
 * Tokens (`/ui/dc1/acls/tokens`):
 *   - 2 tokens in "engineering" (one policy-only, one role-only), each
 *     referencing engineering-local policies/role
 *   - 1 local token in "billing" (policy-only), referencing a billing-local
 *     policy
 *   - 1 token in "default" referencing the built-in "global-management"
 *     policy (MANAGEMENT_POLICY_ID) — only works there: that policy is a
 *     cluster-wide singleton that lives in "default", same cross-namespace
 *     rule as everything else (confirmed against a real agent — 500s from
 *     any other namespace)
 *   - 2 tokens in "platform": one with a ServiceIdentity, one with a
 *     NodeIdentity — renders under the "Identities" badge group
 *   - 50 bulk tokens, one per team-NN namespace, no Policies/Roles (pure
 *     volume — a real per-namespace policy for each would be 50 more calls
 *     for no UI-observable benefit), alternating Local/Global by parity so
 *     the Local/Global column has real variation at scale
 *   - 20 more bulk policies + 20 more bulk tokens, all in "default" and
 *     1:1-linked (bulk-token-NN -> bulk-policy-NN) — unlike the team-NN
 *     bulk tokens above, these give `/ui/dc1/acls/policies` real list volume
 *     too, and let a token-list test assert a real attached-policy badge at
 *     scale, not just Description
 *
 * ACL token: `acl:write` is never covered by `default_policy = "allow"`
 * (verified against a real agent — the anonymous token 403s creating a
 * policy even with the default policy set to allow). Rather than bootstrap
 * the ACL system at seed time (which returns a fresh, unpredictable token
 * every run — no good for a human to paste into the UI login form),
 * `docker-compose.yml`/`local.sh` configure the agent with a **fixed**
 * `acl.tokens.initial_management` value, MANAGEMENT_TOKEN below. Same value
 * every `up`, so it doubles as this scenario's UI login token — see the
 * README's "Log in" section. It's a throwaway dev-mode fixture, not a
 * secret: the agent is in-memory and torn down between runs.
 */
const BULK_COUNT = 50;

/**
 * Must match `acl.tokens.initial_management` in both
 * `docker-compose.yml` and `local.sh` — if you change one, change all three.
 */
const MANAGEMENT_TOKEN = '1234';

export async function seed(consul: ConsulClient): Promise<void> {
  consul.useToken(MANAGEMENT_TOKEN);

  // ── ACL policies + role, created in the "default" namespace ──────────────
  // (the namespace form's pickers only ever query "default" — see doc comment)
  const namespaceReadonlyPolicy = await consul.createPolicy({
    Name: 'namespace-readonly',
    Description: 'Read-only access to nodes and services.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }',
  });
  const namespaceWritePolicy = await consul.createPolicy({
    Name: 'namespace-write',
    Description: 'Read nodes, write services.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "write" }',
  });
  const namespaceOperatorRole = await consul.createRole({
    Name: 'namespace-operator',
    Description: 'Bundles namespace-write for operator tokens.',
    Policies: [{ ID: namespaceWritePolicy.ID }],
  });
  const namespaceViewerRole = await consul.createRole({
    Name: 'namespace-viewer',
    Description: 'Bundles namespace-readonly for viewer tokens.',
    Policies: [{ ID: namespaceReadonlyPolicy.ID }],
  });
  const namespaceAuditorRole = await consul.createRole({
    Name: 'namespace-auditor',
    Description: 'Bundles namespace-readonly for audit tokens.',
    Policies: [{ ID: namespaceReadonlyPolicy.ID }],
  });
  const namespacePlatformAdminRole = await consul.createRole({
    Name: 'namespace-platform-admin',
    Description: 'Bundles namespace-write with a ServiceIdentity for the platform service.',
    Policies: [{ ID: namespaceWritePolicy.ID }],
    ServiceIdentities: [{ ServiceName: 'platform' }],
  });

  // ── 10 policies + 10 roles, all in "default", all attached to ONE
  // namespace ("operations" below) — badge/fieldset rendering at volume on a
  // single namespace, see doc comment ────────────────────────────────────
  const OPERATIONS_ACL_COUNT = 10;
  const operationsPolicyIds: string[] = [];
  const operationsRoleIds: string[] = [];
  for (let i = 1; i <= OPERATIONS_ACL_COUNT; i++) {
    const n = String(i).padStart(2, '0');
    const policy = await consul.createPolicy({
      Name: `operations-policy-${n}`,
      Description: `Operations policy ${i} of ${OPERATIONS_ACL_COUNT} for namespace badge rendering at volume.`,
      Rules: 'node_prefix "" { policy = "read" }',
    });
    operationsPolicyIds.push(policy.ID);
    const role = await consul.createRole({
      Name: `operations-role-${n}`,
      Description: `Wraps operations-policy-${n}.`,
      Policies: [{ ID: policy.ID }],
    });
    operationsRoleIds.push(role.ID);
  }

  // ── namespaces ─────────────────────────────────────────────────────────
  await consul.createNamespace({
    Name: 'engineering',
    Description: 'Engineering team workloads',
    ACLs: {
      // MANAGEMENT_POLICY_ID alongside a regular policy: this row exercises
      // the Management + Policies + Rules badge groups all at once.
      PolicyDefaults: [{ ID: namespaceReadonlyPolicy.ID }, { ID: MANAGEMENT_POLICY_ID }],
      RoleDefaults: [{ ID: namespaceOperatorRole.ID }, { ID: namespaceViewerRole.ID }],
    },
  });
  await consul.createNamespace({
    Name: 'billing',
    Description: 'Billing and payments team workloads',
    ACLs: {
      PolicyDefaults: [{ ID: namespaceReadonlyPolicy.ID }],
      RoleDefaults: [{ ID: namespaceAuditorRole.ID }],
    },
  });
  await consul.createNamespace({
    Name: 'platform',
    Description: 'Platform infrastructure team workloads',
    ACLs: {
      // RoleDefaults with NO PolicyDefaults — the one branch nothing else covers.
      RoleDefaults: [{ ID: namespacePlatformAdminRole.ID }],
    },
  });
  await consul.createNamespace({
    Name: 'operations',
    Description: 'Operations team workloads (many attached policies/roles for badge-rendering checks)',
    ACLs: {
      PolicyDefaults: operationsPolicyIds.map((ID) => ({ ID })),
      RoleDefaults: operationsRoleIds.map((ID) => ({ ID })),
    },
  });
  await consul.createNamespace({ Name: 'security' });

  const bulkNames: string[] = [];
  for (let i = 1; i <= BULK_COUNT; i++) {
    const name = `team-${String(i).padStart(2, '0')}`;
    bulkNames.push(name);
    await consul.createNamespace({
      Name: name,
      Description: `Bulk-seeded namespace ${i} of ${BULK_COUNT} for list volume/search testing.`,
    });
  }

  // ── namespace-local policies/role for tokens (see doc comment: a token's
  // Policies/Roles must live in the same namespace as the token itself) ────
  const engineeringReadonlyPolicy = await consul.createPolicy({
    Name: 'engineering-readonly',
    Description: 'Read-only access to nodes and services.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }',
    Namespace: 'engineering',
  });
  const engineeringWritePolicy = await consul.createPolicy({
    Name: 'engineering-write',
    Description: 'Read nodes, write services.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "write" }',
    Namespace: 'engineering',
  });
  const engineeringOperatorRole = await consul.createRole({
    Name: 'engineering-operator',
    Description: 'Bundles engineering-write for operator tokens.',
    Policies: [{ ID: engineeringWritePolicy.ID }],
    Namespace: 'engineering',
  });
  const billingReadonlyPolicy = await consul.createPolicy({
    Name: 'billing-readonly',
    Description: 'Read-only access to nodes and services.',
    Rules: 'node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }',
    Namespace: 'billing',
  });

  // ── tokens (populates /ui/dc1/acls/tokens + per-namespace Ruleset badges) ─
  await consul.createToken({
    Description: 'Engineering read-only token',
    Namespace: 'engineering',
    Policies: [{ ID: engineeringReadonlyPolicy.ID }],
  });
  await consul.createToken({
    Description: 'Engineering operator token',
    Namespace: 'engineering',
    Roles: [{ ID: engineeringOperatorRole.ID }],
  });
  await consul.createToken({
    Description: 'Billing read-only token (local)',
    Namespace: 'billing',
    Policies: [{ ID: billingReadonlyPolicy.ID }],
    Local: true,
  });
  await consul.createToken({
    Description: 'Global management token',
    Namespace: 'default',
    // Only "default" can reference this — see doc comment.
    Policies: [{ ID: MANAGEMENT_POLICY_ID }],
  });
  await consul.createToken({
    Description: 'Platform service identity token',
    Namespace: 'platform',
    ServiceIdentities: [{ ServiceName: 'platform' }],
  });
  await consul.createToken({
    Description: 'Platform node identity token',
    Namespace: 'platform',
    NodeIdentities: [{ NodeName: 'node-primary', Datacenter: 'dc1' }],
  });

  for (let i = 1; i <= BULK_COUNT; i++) {
    const namespace = `team-${String(i).padStart(2, '0')}`;
    await consul.createToken({
      Description: `Bulk-seeded token ${i} of ${BULK_COUNT} for token list volume/search testing.`,
      Namespace: namespace,
      // No Policies/Roles — pure volume, see doc comment.
      // Even -> local, odd -> global, so the Local/Global column has real
      // variation at scale.
      Local: i % 2 === 0,
    });
  }

  // ── bulk ACL policies + tokens, in "default", 1:1-linked ──────────────────
  // (list volume for /ui/dc1/acls/{policies,tokens}; separate from the
  // team-NN bulk tokens above — see doc comment)
  const ACL_BULK_COUNT = 20;
  const bulkPolicyIds: string[] = [];
  for (let i = 1; i <= ACL_BULK_COUNT; i++) {
    const policy = await consul.createPolicy({
      Name: `bulk-policy-${String(i).padStart(2, '0')}`,
      Description: `Bulk-seeded policy ${i} of ${ACL_BULK_COUNT} for policy list volume/search testing.`,
      Rules: 'node_prefix "" { policy = "read" }',
    });
    bulkPolicyIds.push(policy.ID);
  }
  for (let i = 1; i <= ACL_BULK_COUNT; i++) {
    await consul.createToken({
      Description: `Bulk-seeded token ${i} of ${ACL_BULK_COUNT} for token list volume/search testing.`,
      Policies: [{ ID: bulkPolicyIds[i - 1] }],
      // Even -> local, odd -> global, same convention as the team-NN bulk tokens.
      Local: i % 2 === 0,
    });
  }

  // ── cheap assertions that the data landed before Playwright starts ───────
  for (const [label, resource] of [
    ['"namespace-readonly" policy', namespaceReadonlyPolicy],
    ['"namespace-write" policy', namespaceWritePolicy],
    ['"namespace-operator" role', namespaceOperatorRole],
    ['"namespace-viewer" role', namespaceViewerRole],
    ['"namespace-auditor" role', namespaceAuditorRole],
    ['"namespace-platform-admin" role', namespacePlatformAdminRole],
    ['"engineering-readonly" policy', engineeringReadonlyPolicy],
    ['"engineering-write" policy', engineeringWritePolicy],
    ['"engineering-operator" role', engineeringOperatorRole],
    ['"billing-readonly" policy', billingReadonlyPolicy],
  ] as const) {
    if (!resource.ID) {
      throw new Error(`${label} did not return an ID — found: ${JSON.stringify(resource)}`);
    }
  }

  const namespaces = (await consul.namespaces()).map((n) => n.Name);
  for (const name of ['engineering', 'billing', 'platform', 'operations', 'security', ...bulkNames]) {
    if (!namespaces.includes(name)) {
      throw new Error(`"${name}" namespace missing — found: ${namespaces.join(', ')}`);
    }
  }

  const tokens = await consul.tokens('*');
  const expectedTokenCount = 6 + BULK_COUNT + ACL_BULK_COUNT;
  if (tokens.length < expectedTokenCount) {
    throw new Error(`expected at least ${expectedTokenCount} tokens, found ${tokens.length}`);
  }

  const roles = (await consul.roles('*')).map((r) => r.Name);
  for (const name of ['namespace-operator', 'namespace-viewer', 'namespace-auditor', 'namespace-platform-admin', 'engineering-operator']) {
    if (!roles.includes(name)) {
      throw new Error(`"${name}" role missing — found: ${roles.join(', ')}`);
    }
  }
  for (let i = 1; i <= OPERATIONS_ACL_COUNT; i++) {
    const name = `operations-role-${String(i).padStart(2, '0')}`;
    if (!roles.includes(name)) {
      throw new Error(`"${name}" role missing — found: ${roles.join(', ')}`);
    }
  }

  const policies = (await consul.policies('*')).map((p) => p.Name);
  for (let i = 1; i <= ACL_BULK_COUNT; i++) {
    const name = `bulk-policy-${String(i).padStart(2, '0')}`;
    if (!policies.includes(name)) {
      throw new Error(`"${name}" policy missing — found: ${policies.join(', ')}`);
    }
  }
  for (let i = 1; i <= OPERATIONS_ACL_COUNT; i++) {
    const name = `operations-policy-${String(i).padStart(2, '0')}`;
    if (!policies.includes(name)) {
      throw new Error(`"${name}" policy missing — found: ${policies.join(', ')}`);
    }
  }

  console.log(`  UI login token (management): ${MANAGEMENT_TOKEN}`);
}
