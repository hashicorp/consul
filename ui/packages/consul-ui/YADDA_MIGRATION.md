# Yadda → QUnit acceptance test migration tracker

Running tracker for migrating the UI's yadda/Gherkin acceptance tests
(`tests/acceptance/**/*.feature`) to native QUnit acceptance tests.

- **Harness / scaffolding:** [tests/helpers/acceptance.js](tests/helpers/acceptance.js)
- **Reference migration (template):** [tests/acceptance/dc/intentions/create-test.js](tests/acceptance/dc/intentions/create-test.js)

## Goals

1. **Replace yadda/Gherkin acceptance tests with native QUnit acceptance tests**, one
   feature at a time, preserving behaviour and coverage (including the CE/ENT namespace
   matrix) at every step.
2. **Keep the suite green throughout.** Yadda and native QUnit tests compile into the same
   test bundle and can coexist, so no flag flips or freezes are required — migrate
   incrementally and opportunistically as features are touched.
3. **Reduce the custom testing surface.** Native tests use standard `@ember/test-helpers`,
   `qunit` and `assert.dom`, which are easier to debug, type-check and onboard onto than the
   bespoke yadda step DSL.
4. **Reach zero `.feature` files**, then remove the yadda toolchain entirely (see below).

## End state: removing yadda (do this only once the table below is 100% migrated)

When no `.feature` files remain, complete the migration by deleting the yadda toolchain and
its supporting infrastructure:

- **Dependency:** remove `ember-cli-yadda` from [package.json](package.json) (line ~141) and
  update the lockfile. This addon pulls in `yadda` transitively and is what compiled
  `.feature` files into test modules — nothing else should depend on it once features are gone.
- **Runner / gating:** delete [tests/helpers/yadda-annotations.js](tests/helpers/yadda-annotations.js)
  (the annotation-driven test generator and namespace matrix — now reproduced by
  `nspaceScenario` in the harness).
- **Step DSL:** delete [tests/steps.js](tests/steps.js) and the entire `tests/steps/` tree
  (`doubles/`, `interactions/`, `assertions/`, `debug/`) — the request/interaction/assertion
  vocabulary, now reproduced as plain helpers in the harness.
- **Feature wiring:** delete the entire `tests/acceptance/steps/` tree (the `*-steps.js`
  stubs and `steps.js`) that bound each `.feature` to the shared steps.
- **Dictionary:** delete [tests/dictionary.js](tests/dictionary.js) (the yadda term converters
  and `@namespace` string substitution).
- **Step-listing CLI:** delete the `lib/commands/` addon (the `steps:list` ember command in
  [lib/commands/index.js](lib/commands/index.js) that listed available yadda steps).

Keep (still used by native tests): [tests/helpers/api.js](tests/helpers/api.js) and the
api-double, [tests/pages.js](tests/pages.js) / `tests/pages/`, `tests/lib/page-object/`, and
[tests/helpers/acceptance.js](tests/helpers/acceptance.js).

**Final verification before declaring done:**

- `grep -r "yadda" package.json tests lib` returns nothing.
- `find tests -name '*.feature' -o -name '*-steps.js'` returns nothing.
- `pnpm test` (or `test:oss` for CE) passes with the same scenario/assertion count as before.

## How to use this document

When you migrate a feature:

1. Rewrite the `.feature` as a `-test.js` using the harness (copy the reference test).
2. Wrap each scenario in `nspaceScenario(...)`, carrying over `@onlyNamespaceable` /
   `@notNamespaceable` / `@ignore` as options so the CE/ENT namespace matrix is preserved.
3. `git rm` the `.feature` **and** its matching `tests/acceptance/steps/**/<name>-steps.js`.
4. Update this table: set **Status** to ✅ Migrated and fill in the **Native test** link.

The **Description** column summarises what each feature verifies so you can reproduce it
manually in the running UI if needed.

**Status legend:** ☐ Not migrated &nbsp;·&nbsp; 🔄 In progress &nbsp;·&nbsp; ✅ Migrated &nbsp;·&nbsp; ⏭️ Won't migrate (delete/obsolete)

## Summary

| Metric | Count |
| --- | --- |
| Feature files remaining (yadda) | 125 |
| Scenarios remaining (yadda) | 288 |
| Feature files migrated | 4 |
| Scenarios migrated | 5 |

## Completed

| Feature (was) | Description | Scenarios | Native test |
| --- | --- | --- | --- |
| `dc/intentions/create.feature` | Create an intention from the intentions form (source/destination, deny action) and assert the PUT payload; namespaces on and off. | 2 | [tests/acceptance/dc/intentions/create-test.js](tests/acceptance/dc/intentions/create-test.js) |
| `components/copy-button.feature` | Copy-to-clipboard button copies the expected value when clicked. Carried the feature-level `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/components/copy-button-test.js](tests/acceptance/components/copy-button-test.js) |
| `components/kv-filter.feature` | Free-text filter on the KV list narrows results by the typed text. | 1 (2 rows) | [tests/acceptance/components/kv-filter-test.js](tests/acceptance/components/kv-filter-test.js) |
| `components/text-input.feature` | Text-input component behaves correctly (KV create page enables submit once filled). | 1 | [tests/acceptance/components/text-input-test.js](tests/acceptance/components/text-input-test.js) |

## Remaining features

| Feature file | Description | Scenarios | Status | Native test |
| --- | --- | --- | --- | --- |
| [api-prefix.feature](tests/acceptance/api-prefix.feature) | UI honours a configured API path prefix when making requests. | 1 | ☐ Not migrated |  |
| [dc/acls/access.feature](tests/acceptance/dc/acls/access.feature) | ACLs access page behaviour when ACLs are disabled. | 1 | ☐ Not migrated |  |
| [dc/acls/auth-methods/index.feature](tests/acceptance/dc/acls/auth-methods/index.feature) | Auth-methods list renders and is searchable. | 2 | ☐ Not migrated |  |
| [dc/acls/auth-methods/navigation.feature](tests/acceptance/dc/acls/auth-methods/navigation.feature) | Navigate into an auth-method from the list and back. | 1 | ☐ Not migrated |  |
| [dc/acls/auth-methods/sorting.feature](tests/acceptance/dc/acls/auth-methods/sorting.feature) | Sorting the auth-methods list. | 1 | ☐ Not migrated |  |
| [dc/acls/index.feature](tests/acceptance/dc/acls/index.feature) | ACL index page forwards/redirects to the correct sub-page. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/add-existing.feature](tests/acceptance/dc/acls/policies/as-many/add-existing.feature) | Attach an existing policy as a child of a token/role. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/add-new.feature](tests/acceptance/dc/acls/policies/as-many/add-new.feature) | Add a new policy, service identity or node identity to a token/role, incl. error and code-editor cases. | 6 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/list.feature](tests/acceptance/dc/acls/policies/as-many/list.feature) | List the policies attached to a token/role. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/nspaces.feature](tests/acceptance/dc/acls/policies/as-many/nspaces.feature) | Policy "as many" selector behaviour across namespaces. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/remove.feature](tests/acceptance/dc/acls/policies/as-many/remove.feature) | Remove attached policies from a token/role. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/as-many/reset.feature](tests/acceptance/dc/acls/policies/as-many/reset.feature) | The attached-policy sub-form resets correctly. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/create.feature](tests/acceptance/dc/acls/policies/create.feature) | Create an ACL policy; no Namespace sent when namespaces are disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/policies/delete.feature](tests/acceptance/dc/acls/policies/delete.feature) | Delete a policy from the list and detail pages, incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/acls/policies/index.feature](tests/acceptance/dc/acls/policies/index.feature) | Policies list renders and is searchable; global-management can't be deleted. | 3 | ☐ Not migrated |  |
| [dc/acls/policies/navigation.feature](tests/acceptance/dc/acls/policies/navigation.feature) | Navigate into a policy from the list and back. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/sorting.feature](tests/acceptance/dc/acls/policies/sorting.feature) | Sorting the policies list. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/update.feature](tests/acceptance/dc/acls/policies/update.feature) | Update a policy (name/rules/description); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/policies/view-management.feature](tests/acceptance/dc/acls/policies/view-management.feature) | The built-in management policy is shown read-only. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/view-read-only.feature](tests/acceptance/dc/acls/policies/view-read-only.feature) | Read-only policy view. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/add-existing.feature](tests/acceptance/dc/acls/roles/as-many/add-existing.feature) | Attach an existing role as a child of a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/add-new.feature](tests/acceptance/dc/acls/roles/as-many/add-new.feature) | Add a new role with/without policies and service identities; cancel the form. | 5 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/list.feature](tests/acceptance/dc/acls/roles/as-many/list.feature) | List the roles attached to a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/remove.feature](tests/acceptance/dc/acls/roles/as-many/remove.feature) | Remove attached roles from a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/create.feature](tests/acceptance/dc/acls/roles/create.feature) | Create an ACL role; no Namespace sent when namespaces are disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/roles/index.feature](tests/acceptance/dc/acls/roles/index.feature) | Roles list renders and is searchable. | 2 | ☐ Not migrated |  |
| [dc/acls/roles/navigation.feature](tests/acceptance/dc/acls/roles/navigation.feature) | Navigate into a role from the list and back. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/sorting.feature](tests/acceptance/dc/acls/roles/sorting.feature) | Sorting the roles list. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/update.feature](tests/acceptance/dc/acls/roles/update.feature) | Update a role (name/rules/description); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/tokens/anonymous-no-delete.feature](tests/acceptance/dc/acls/tokens/anonymous-no-delete.feature) | The anonymous token exposes no delete buttons on list or detail pages. | 2 | ☐ Not migrated |  |
| [dc/acls/tokens/clone.feature](tests/acceptance/dc/acls/tokens/clone.feature) | Clone an ACL token from the list and detail pages. | 2 | ☐ Not migrated |  |
| [dc/acls/tokens/create.feature](tests/acceptance/dc/acls/tokens/create.feature) | Create an ACL token; no Namespace sent when namespaces are disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/tokens/index.feature](tests/acceptance/dc/acls/tokens/index.feature) | Token list: view, no-write access, search, and legacy-token message. | 5 | ☐ Not migrated |  |
| [dc/acls/tokens/navigation.feature](tests/acceptance/dc/acls/tokens/navigation.feature) | Navigate into a token from the list and back. | 1 | ☐ Not migrated |  |
| [dc/acls/tokens/own-no-delete.feature](tests/acceptance/dc/acls/tokens/own-no-delete.feature) | Your currently-used token exposes no delete button. | 1 | ☐ Not migrated |  |
| [dc/acls/tokens/sorting.feature](tests/acceptance/dc/acls/tokens/sorting.feature) | Sorting the tokens list. | 1 | ☐ Not migrated |  |
| [dc/acls/tokens/update.feature](tests/acceptance/dc/acls/tokens/update.feature) | Update a token (name); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/tokens/use.feature](tests/acceptance/dc/acls/tokens/use.feature) | Switch to (use) an ACL token from the list and detail pages. | 2 | ☐ Not migrated |  |
| [dc/error.feature](tests/acceptance/dc/error.feature) | Recovering from a datacenter 500 error. | 1 | ☐ Not migrated |  |
| [dc/forwarding.feature](tests/acceptance/dc/forwarding.feature) | Datacenter index forwards to the right page when no other URL info is present. | 1 | ☐ Not migrated |  |
| [dc/index.feature](tests/acceptance/dc/index.feature) | Datacenters index lands on the services page. | 1 | ☐ Not migrated |  |
| [dc/intentions/delete.feature](tests/acceptance/dc/intentions/delete.feature) | Delete intentions from list/detail incl. error and duplicate-intention error. | 4 | ☐ Not migrated |  |
| [dc/intentions/filtered-select.feature](tests/acceptance/dc/intentions/filtered-select.feature) | Service select dropdowns show services but exclude proxy services; handles same-name services in different namespaces. | 2 | ☐ Not migrated |  |
| [dc/intentions/form-select.feature](tests/acceptance/dc/intentions/form-select.feature) | Type into the autocomplete and select a custom/future service. | 1 | ☐ Not migrated |  |
| [dc/intentions/index.feature](tests/acceptance/dc/intentions/index.feature) | Intentions list: view, no-write access, live updates, with/without CRDs, empty states. | 7 | ☐ Not migrated |  |
| [dc/intentions/navigation.feature](tests/acceptance/dc/intentions/navigation.feature) | Navigate into an intention and to the create form, and back. | 2 | ☐ Not migrated |  |
| [dc/intentions/permissions/create.feature](tests/acceptance/dc/intentions/permissions/create.feature) | Create an intention with L7 permissions. | 1 | ☐ Not migrated |  |
| [dc/intentions/permissions/warn.feature](tests/acceptance/dc/intentions/permissions/warn.feature) | Warning shown when adding permissions to an intention. | 1 | ☐ Not migrated |  |
| [dc/intentions/read-only.feature](tests/acceptance/dc/intentions/read-only.feature) | Viewing a read-only intention. | 1 | ☐ Not migrated |  |
| [dc/intentions/sorting.feature](tests/acceptance/dc/intentions/sorting.feature) | Sorting the intentions list. | 1 | ☐ Not migrated |  |
| [dc/intentions/update.feature](tests/acceptance/dc/intentions/update.feature) | Update an intention (description/action); error handling. | 2 | ☐ Not migrated |  |
| [dc/kvs/create.feature](tests/acceptance/dc/kvs/create.feature) | Create a root KV and folders, incl. creating from within a folder. | 4 | ☐ Not migrated |  |
| [dc/kvs/delete.feature](tests/acceptance/dc/kvs/delete.feature) | Delete a KV from list/detail incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/kvs/edit.feature](tests/acceptance/dc/kvs/edit.feature) | View a KV: URL-unsafe characters, attached lock session, no-write / no-read access. | 6 | ☐ Not migrated |  |
| [dc/kvs/index.feature](tests/acceptance/dc/kvs/index.feature) | KV list view incl. no-write access. | 2 | ☐ Not migrated |  |
| [dc/kvs/list-order.feature](tests/acceptance/dc/kvs/list-order.feature) | KV keys/folders listed alphabetically. | 1 | ☐ Not migrated |  |
| [dc/kvs/sessions/invalidate.feature](tests/acceptance/dc/kvs/sessions/invalidate.feature) | Invalidate a lock session attached to a KV incl. error. | 2 | ☐ Not migrated |  |
| [dc/kvs/trailing-slash.feature](tests/acceptance/dc/kvs/trailing-slash.feature) | KV folder view resolves with and without a trailing slash. | 2 | ☐ Not migrated |  |
| [dc/kvs/update.feature](tests/acceptance/dc/kvs/update.feature) | Update KV values incl. whitespace/empty/newline values and error handling. | 7 | ☐ Not migrated |  |
| [dc/list-blocking.feature](tests/acceptance/dc/list-blocking.feature) | Listing/detail pages live-update via blocking queries when Consul changes externally. | 2 | ☐ Not migrated |  |
| [dc/list.feature](tests/acceptance/dc/list.feature) | Generic model listing pages render. | 1 | ☐ Not migrated |  |
| [dc/nodes/empty-ids.feature](tests/acceptance/dc/nodes/empty-ids.feature) | Node list handles nodes that arrive with no ID. | 1 | ☐ Not migrated |  |
| [dc/nodes/index.feature](tests/acceptance/dc/nodes/index.feature) | Nodes list: unhealthy node/service checks, synthetic nodes hidden, leader indicator, search, empty state. | 7 | ☐ Not migrated |  |
| [dc/nodes/navigation.feature](tests/acceptance/dc/nodes/navigation.feature) | Navigate into a node from the list and back. | 1 | ☐ Not migrated |  |
| [dc/nodes/no-leader.feature](tests/acceptance/dc/nodes/no-leader.feature) | Behaviour when no leader has been elected. | 1 | ☐ Not migrated |  |
| [dc/nodes/services/list.feature](tests/acceptance/dc/nodes/services/list.feature) | Node → Services tab listing. | 1 | ☐ Not migrated |  |
| [dc/nodes/sessions/invalidate.feature](tests/acceptance/dc/nodes/sessions/invalidate.feature) | Invalidate a lock session on a node incl. error. | 2 | ☐ Not migrated |  |
| [dc/nodes/sessions/list.feature](tests/acceptance/dc/nodes/sessions/list.feature) | Node lock-sessions list: string TTLs, nanosecond LockDelay, ACL enabled/disabled empty states. | 4 | ☐ Not migrated |  |
| [dc/nodes/show.feature](tests/acceptance/dc/nodes/show.feature) | Node detail: tab visibility/selection, deregister warning while blocking, RTT display. | 5 | ☐ Not migrated |  |
| [dc/nodes/show/health-checks.feature](tests/acceptance/dc/nodes/show/health-checks.feature) | Node serf health check passing/failing display. | 2 | ☐ Not migrated |  |
| [dc/nodes/sorting.feature](tests/acceptance/dc/nodes/sorting.feature) | Sorting the nodes list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/create.feature](tests/acceptance/dc/nspaces/create.feature) | Create a namespace. | 2 | ☐ Not migrated |  |
| [dc/nspaces/delete.feature](tests/acceptance/dc/nspaces/delete.feature) | Delete a namespace from list/detail incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/nspaces/index.feature](tests/acceptance/dc/nspaces/index.feature) | Namespaces list and search; the default namespace can't be deleted. | 3 | ☐ Not migrated |  |
| [dc/nspaces/manage.feature](tests/acceptance/dc/nspaces/manage.feature) | Managing namespaces. | 1 | ☐ Not migrated |  |
| [dc/nspaces/sorting.feature](tests/acceptance/dc/nspaces/sorting.feature) | Sorting the namespaces list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/update.feature](tests/acceptance/dc/nspaces/update.feature) | Update a namespace (description); error handling. | 2 | ☐ Not migrated |  |
| [dc/peers/create.feature](tests/acceptance/dc/peers/create.feature) | Generate a peering token (create peer). | 1 | ☐ Not migrated |  |
| [dc/peers/delete.feature](tests/acceptance/dc/peers/delete.feature) | Delete a peer incl. error; a peer already deleting can't be deleted again. | 3 | ☐ Not migrated |  |
| [dc/peers/establish.feature](tests/acceptance/dc/peers/establish.feature) | Establish peering from a peering token. | 1 | ☐ Not migrated |  |
| [dc/peers/index.feature](tests/acceptance/dc/peers/index.feature) | Peers list: view, sort, search, and empty states (ACLs on/off). | 5 | ☐ Not migrated |  |
| [dc/peers/regenerate.feature](tests/acceptance/dc/peers/regenerate.feature) | Regenerate a peering token. | 2 | ☐ Not migrated |  |
| [dc/peers/show.feature](tests/acceptance/dc/peers/show.feature) | Peer detail tabs: dialer/receiver, imported/exported services (empty & populated), addresses. | 8 | ☐ Not migrated |  |
| [dc/routing-config.feature](tests/acceptance/dc/routing-config.feature) | View a routing config and its source pill. | 2 | ☐ Not migrated |  |
| [dc/services/dc-switch.feature](tests/acceptance/dc/services/dc-switch.feature) | Services list reflects a datacenter switch. | 1 | ☐ Not migrated |  |
| [dc/services/error.feature](tests/acceptance/dc/services/error.feature) | Service page error / not-found handling. | 2 | ☐ Not migrated |  |
| [dc/services/index.feature](tests/acceptance/dc/services/index.feature) | Services list: services, gateways, mesh state, associated-service counts, empty states. | 6 | ☐ Not migrated |  |
| [dc/services/instances/error.feature](tests/acceptance/dc/services/instances/error.feature) | Missing service-instance handling. | 1 | ☐ Not migrated |  |
| [dc/services/instances/exposed-paths.feature](tests/acceptance/dc/services/instances/exposed-paths.feature) | Exposed Paths tab shown only when the instance has a proxy. | 2 | ☐ Not migrated |  |
| [dc/services/instances/gateway.feature](tests/acceptance/dc/services/instances/gateway.feature) | Gateway service-instance detail page. | 1 | ☐ Not migrated |  |
| [dc/services/instances/health-checks.feature](tests/acceptance/dc/services/instances/health-checks.feature) | Instance serf checks pass/fail; node health-check visibility on agentless vs non-agentless. | 4 | ☐ Not migrated |  |
| [dc/services/instances/navigation.feature](tests/acceptance/dc/services/instances/navigation.feature) | Navigate into a service instance from the list and back. | 1 | ☐ Not migrated |  |
| [dc/services/instances/show.feature](tests/acceptance/dc/services/instances/show.feature) | Service-instance detail: proxy presence, deregister warning while blocking, synthetic node. | 4 | ☐ Not migrated |  |
| [dc/services/instances/upstreams.feature](tests/acceptance/dc/services/instances/upstreams.feature) | Upstreams tab shown only when the instance has a proxy. | 2 | ☐ Not migrated |  |
| [dc/services/list-blocking.feature](tests/acceptance/dc/services/list-blocking.feature) | Service listing live-updates via blocking queries. | 1 | ☐ Not migrated |  |
| [dc/services/list.feature](tests/acceptance/dc/services/list.feature) | Listing services incl. peered services. | 2 | ☐ Not migrated |  |
| [dc/services/navigation.feature](tests/acceptance/dc/services/navigation.feature) | Navigate into a (peered) service from the list and back. | 2 | ☐ Not migrated |  |
| [dc/services/show-routing.feature](tests/acceptance/dc/services/show-routing.feature) | Routing tab display; hidden/no error when connect is disabled. | 2 | ☐ Not migrated |  |
| [dc/services/show-with-slashes.feature](tests/acceptance/dc/services/show-with-slashes.feature) | A service with slashes in its name lists and opens correctly. | 1 | ☐ Not migrated |  |
| [dc/services/show.feature](tests/acceptance/dc/services/show.feature) | Service detail: external-source logos, tags, instance nodes, dashboard template, access removal. | 10 | ☐ Not migrated |  |
| [dc/services/show/dc-switch.feature](tests/acceptance/dc/services/show/dc-switch.feature) | Service detail reflects a datacenter switch. | 1 | ☐ Not migrated |  |
| [dc/services/show/intentions/create.feature](tests/acceptance/dc/services/show/intentions/create.feature) | Create an intention from a service page (namespaces on/off). | 2 | ☐ Not migrated |  |
| [dc/services/show/intentions/index.feature](tests/acceptance/dc/services/show/intentions/index.feature) | Per-service intentions tab: view and delete. | 2 | ☐ Not migrated |  |
| [dc/services/show/navigation.feature](tests/acceptance/dc/services/show/navigation.feature) | Accessing a peered service directly by URL. | 1 | ☐ Not migrated |  |
| [dc/services/show/services.feature](tests/acceptance/dc/services/show/services.feature) | Linked Services tab visibility and list. | 3 | ☐ Not migrated |  |
| [dc/services/show/tags.feature](tests/acceptance/dc/services/show/tags.feature) | Service tags display incl. duplicate tags. | 2 | ☐ Not migrated |  |
| [dc/services/show/topology/index.feature](tests/acceptance/dc/services/show/topology/index.feature) | Topology tab display; hidden/no error when connect is disabled. | 2 | ☐ Not migrated |  |
| [dc/services/show/topology/intentions.feature](tests/acceptance/dc/services/show/topology/intentions.feature) | Allow a connection by saving an intention from topology; error handling. | 2 | ☐ Not migrated |  |
| [dc/services/show/topology/metrics.feature](tests/acceptance/dc/services/show/topology/metrics.feature) | Topology metrics with/without a prometheus provider; API Gateway source. | 3 | ☐ Not migrated |  |
| [dc/services/show/topology/notices.feature](tests/acceptance/dc/services/show/topology/notices.feature) | Topology notices: default ACL policy, wildcard intentions, ACL-filtered response, TProxy states. | 5 | ☐ Not migrated |  |
| [dc/services/show/topology/routing-config.feature](tests/acceptance/dc/services/show/topology/routing-config.feature) | Topology shows source type and redirects to the Routing Config page. | 2 | ☐ Not migrated |  |
| [dc/services/show/topology/stats.feature](tests/acceptance/dc/services/show/topology/stats.feature) | Topology metric stats enabled/disabled incl. ingress-gateway cases. | 4 | ☐ Not migrated |  |
| [dc/services/show/upstreams.feature](tests/acceptance/dc/services/show/upstreams.feature) | Upstreams tab visibility and list. | 3 | ☐ Not migrated |  |
| [dc/services/sorting.feature](tests/acceptance/dc/services/sorting.feature) | Sorting the services list. | 1 | ☐ Not migrated |  |
| [deleting.feature](tests/acceptance/deleting.feature) | Generic delete flow: confirmation dialog plus success/error notifications. | 3 | ☐ Not migrated |  |
| [index-forwarding.feature](tests/acceptance/index-forwarding.feature) | Index page forwards straight through when there is only one datacenter. | 1 | ☐ Not migrated |  |
| [login-errors.feature](tests/acceptance/login-errors.feature) | Login 500-error handling (non-legacy-token case). | 1 | ☐ Not migrated |  |
| [login.feature](tests/acceptance/login.feature) | Logging in via an ACL token and via SSO. | 2 | ☐ Not migrated |  |
| [navigation-links.feature](tests/acceptance/navigation-links.feature) | Main-navigation link visibility (e.g. no KV read access, empty-state login button). | 2 | ☐ Not migrated |  |
| [page-navigation.feature](tests/acceptance/page-navigation.feature) | Navigation across pages routes correctly and calls the expected API endpoints; cancel/create flows. | 10 | ☐ Not migrated |  |
| [settings/show.feature](tests/acceptance/settings/show.feature) | Settings page shows Blocking Queries; CONSUL_UI_DISABLE_REALTIME hides them. | 2 | ☐ Not migrated |  |
| [settings/update.feature](tests/acceptance/settings/update.feature) | Saving settings with no input typed. | 1 | ☐ Not migrated |  |
| [startup.feature](tests/acceptance/startup.feature) | App boots when loading index.html into a browser. | 1 | ☐ Not migrated |  |
| [submit-blank.feature](tests/acceptance/submit-blank.feature) | Blank create forms keep the submit button disabled. | 2 | ☐ Not migrated |  |
| [token-header.feature](tests/acceptance/token-header.feature) | API requests send the Consul token header after a token is set. | 2 | ☐ Not migrated |  |
