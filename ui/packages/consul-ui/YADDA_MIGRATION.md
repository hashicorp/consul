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
  `nspaceScenario` in the harness) and [tests/helpers/page.js](tests/helpers/page.js), whose
  only consumer is `yadda-annotations.js`.
- **Step DSL:** delete [tests/steps.js](tests/steps.js) and the entire `tests/steps/` tree
  (`doubles/`, `interactions/`, `assertions/`, `debug/`) — the request/interaction/assertion
  vocabulary, now reproduced as plain helpers in the harness.
- **Feature wiring:** delete the entire `tests/acceptance/steps/` tree (the `*-steps.js`
  stubs and `steps.js`) that bound each `.feature` to the shared steps.
- **Dictionary:** delete [tests/dictionary.js](tests/dictionary.js) (the yadda term converters
  and `@namespace` string substitution).
- **Step-listing CLI:** delete the `lib/commands/` addon (the `steps:list` ember command in
  [lib/commands/index.js](lib/commands/index.js) that listed available yadda steps), **and**
  remove its wiring: the `steps:list` script and the `"lib/commands"` entry in
  `ember-addon.paths` in [package.json](package.json), and the `steps:` target in
  [GNUmakefile](GNUmakefile). ember-cli throws on a missing `ember-addon.paths` entry, so
  deleting the folder alone breaks every build.
- **Lockfile:** run `pnpm install` from `ui/` so `ui/pnpm-lock.yaml` drops `ember-cli-yadda` and
  `yadda`.
- **Dictionary-only deps:** `js-yaml` (a direct devDependency in `package.json`) is imported only
  by `tests/dictionary.js` today. If nothing else uses it by then (including the new
  namespace-template helper, see Phase 0), remove it.
- **Docs:** rewrite the BDD/`ember generate feature` section of
  [docs/testing.mdx](docs/testing.mdx) (~line 210) to describe native QUnit acceptance tests.
- **This file:** delete it (or archive it) once the work is finished.

Keep (still used by native tests): [tests/helpers/api.js](tests/helpers/api.js) and the
api-double, [tests/pages.js](tests/pages.js) / `tests/pages/`, `tests/lib/page-object/`, and
[tests/helpers/acceptance.js](tests/helpers/acceptance.js).

**Final verification before declaring done:**

- `grep -ri "yadda" package.json tests lib docs ../../pnpm-lock.yaml` returns nothing
  (comments mentioning yadda in native tests should be reworded by then).
- `find tests -name '*.feature' -o -name '*-steps.js'` returns nothing.
- `pnpm build` and `pnpm lint` succeed (catches a dangling `lib/commands` addon path).
- `CONSUL_NSPACES_ENABLED=0` and `CONSUL_NSPACES_ENABLED=1` test runs both pass, and their sorted
  test-title lists match the Phase 0 baseline (see below) apart from intentional renames.
  `.github/workflows/frontend.yml` runs both modes (`ember-build-test` and
  `ember-build-test-ent`), so neither may be skipped locally.

## Phase 0: do before migrating further

1. **Namespace templating.** ~32 features use `@namespace` / `@!namespace` placeholders that
   `tests/dictionary.js` rewrites (differently for CE and ENT) in its `yaml`, `endpoint`,
   `json` and `url` terms. The harness only has `nspaceURL`. Add a shared helper that ports
   this logic exactly, with unit tests for each CE/ENT/nspace case, so tests cannot pass while
   asserting the wrong `ns=` query string or payload.
2. **Harness gaps.** `tests/helpers/acceptance.js` has far fewer helpers than the
   `tests/steps/{doubles,interactions,assertions}` vocabulary (model-count doubles,
   `settings`/`ui_config`/`permissions`/ACL setup, form filling, "see text/object" and
   `pause until` assertions, etc.). Add them once, in a single PR, preserving the strength of the
   original assertions, rather than re-implementing them per test.
3. **Baseline.** Before the first batch, record the sorted test-title list (and pass/skip
   counts) from `ember test` under both `CONSUL_NSPACES_ENABLED=0` and `=1`. Compare against
   it after every batch so dropped scenarios, `Where:` rows or tags are caught. Note that the
   test environment defaults to namespaces **off** (`config/environment.js`), so a plain
   `pnpm test` only exercises CE.
4. **Fix the known regressions** above, so later batches don't copy the weakened navigation
   pattern.
5. **Decide on `@ignore` scenarios.** Delete the 8 `Then ok` placeholders (⏭️) instead of
   porting them. For the other 8 ignored scenarios, either fix them or delete them with a
   reason; porting them as permanent `skip`s keeps dead code.

## How to use this document

When you migrate a feature:

1. Rewrite the `.feature` as a `-test.js` using the harness (copy the reference test).
2. Wrap each scenario in `nspaceScenario(...)`, carrying over `@onlyNamespaceable` /
   `@notNamespaceable` / `@ignore` as options so the CE/ENT namespace matrix is preserved.
   **Never add an option the original did not have.** If a scenario genuinely cannot run with a
   namespace, say why in a comment and flag it in this table. Keep the original `Scenario:`
   title in the test title so baselines can be diffed.
3. Port every assertion step one-for-one: exact URLs via
   `assert.equal(currentURL(), nspaceURL(nspace, ...))` (not `includes`), model counts, and
   every `a GET request was made to` (with its `ns=` handling). If the UI changed, update the
   assertion; don't delete it.
4. `git rm` the `.feature` **and** its `-steps.js` stub. ember-cli-yadda always imports the stub
   from `tests/acceptance/steps/<same path>/<name>-steps.js`, so the pairing is exact. The 19
   stubs with no matching `.feature` (e.g. `steps/dc/kv/index/view-kvs-steps.js`) are leftovers
   from long-deleted features and can be deleted at any time.
5. Update this table: set **Status** to ✅ Migrated and fill in the **Native test** link. To avoid
   merge conflicts between parallel PRs, only touch the rows for your own features.
6. Run both `CONSUL_NSPACES_ENABLED=0` and `=1` suites; run timing-sensitive tests (blocking
   queries / live updates) repeatedly to check for flakiness.

The **Description** column summarises what each feature verifies so you can reproduce it
manually in the running UI if needed.

**Status legend:** ☐ Not migrated &nbsp;·&nbsp; 🔄 In progress &nbsp;·&nbsp; ✅ Migrated &nbsp;·&nbsp; ⏭️ Won't migrate (delete/obsolete)

## Summary

| Metric | Count |
| --- | --- |
| Feature files remaining (yadda) | 102 |
| Scenarios remaining (yadda) | 253 (16 are `@ignore`, of which 8 are `Then ok` placeholders) |
| Feature files fully migrated | 27 |
| Scenarios migrated | 46 from the 27 features, plus 1 (3 `Where:` rows) from `page-navigation.feature` |

Scenario counts are not test counts. 24 remaining features use `Where:` tables (one test per
row), and under `CONSUL_NSPACES_ENABLED=1` each namespaceable scenario runs 4 times.

`tests/acceptance` contains 37 native `*-test.js` files: 28 come from migrated features (27 full +
the partial `page-navigation-create-test.js`) and 9 are native-only tests that never had a
`.feature` (`breadcrumbs/*`, `components/tab-error`, `components/tab-nav-focus`,
`dc/nodes/sessions/filter-sort`, `hcp-login`, `unavailable`). Counts are from disk; re-verify with
`find tests -name '*.feature' | wc -l`.

## Known regressions in completed migrations (fix before building on them)

Audited by comparing each native test with the original `.feature` removed in #23741 and #23911.
The #23741 batch (intentions/create, components, settings, dc/*) is faithful. The #23911 batch is
not:

1. **ENT namespace coverage dropped.** These scenarios had no namespace tag (so they ran once per
   namespace under ENT) but the ports pass `notNamespaceable: true`, with no explanation:
   - all 8 `*/navigation` ports (auth-methods, policies, roles, tokens, nodes, intentions ×2,
     services ×2, services/instances)
   - `components/text-input`
   - `dc/nodes/sessions/list` "Given 0 sessions with ACLs disabled"
   - the 3 `page-navigation` create rows

   That is 15 scenario-rows × 4 namespaces = **60 ENT tests that no longer run**. Restore the
   namespace matrix, or document in the test why the scenario cannot run with a namespace.
2. **Weaker assertions in the navigation ports.** Exact URL checks became
   `currentURL().includes(...)`, which is nearly vacuous (e.g. `includes('/dc-1/services')`), and
   the "I see N models" counts were dropped. `dc/services/instances/navigation-test.js` also drops
   the proxy fixture and all four namespaced `GET /v1/catalog/connect/...` and
   `GET /v1/health/service/...-proxy` request assertions.
3. **Dropped `@ignore` scenario.** `dc/acls/policies/index` lost its ignored scenario instead of
   keeping it as a skip. Harmless, but the scenario count no longer matches.

## Completed

| Feature (was) | Description | Scenarios | Native test |
| --- | --- | --- | --- |
| `dc/intentions/create.feature` | Create an intention from the intentions form (source/destination, deny action) and assert the PUT payload; namespaces on and off. | 2 | [tests/acceptance/dc/intentions/create-test.js](tests/acceptance/dc/intentions/create-test.js) |
| `components/copy-button.feature` | Copy-to-clipboard button copies the expected value when clicked. Carried the feature-level `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/components/copy-button-test.js](tests/acceptance/components/copy-button-test.js) |
| `components/kv-filter.feature` | Free-text filter on the KV list narrows results by the typed text. | 1 (2 rows) | [tests/acceptance/components/kv-filter-test.js](tests/acceptance/components/kv-filter-test.js) |
| `components/text-input.feature` | Text-input component behaves correctly (KV create page enables submit once filled). ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/components/text-input-test.js](tests/acceptance/components/text-input-test.js) |
| `settings/show.feature` | Settings page shows Blocking Queries; `CONSUL_UI_DISABLE_REALTIME` hides them. `@notNamespaceable` (CE-only, as before). | 2 | [tests/acceptance/settings/show-test.js](tests/acceptance/settings/show-test.js) |
| `settings/update.feature` | Saving settings with no input typed. Carried the feature-level `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/settings/update-test.js](tests/acceptance/settings/update-test.js) |
| `dc/error.feature` | Recovering from a datacenter 500 error: error status shown, then switching dc via the nav recovers and lists services. | 1 | [tests/acceptance/dc/error-test.js](tests/acceptance/dc/error-test.js) |
| `dc/forwarding.feature` | Arriving at a dc with only the dc in the URL redirects to its services overview. `@notNamespaceable` (CE-only, as before). | 1 | [tests/acceptance/dc/forwarding-test.js](tests/acceptance/dc/forwarding-test.js) |
| `dc/index.feature` | Datacenters index lands on the services page. Carried the scenario `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/dc/index-test.js](tests/acceptance/dc/index-test.js) |
| `dc/list.feature` | Generic model listing pages render (nodes/kv/tokens/policies). | 1 (4 rows) | [tests/acceptance/dc/list-test.js](tests/acceptance/dc/list-test.js) |
| `dc/list-blocking.feature` | Listing pages live-update via blocking queries when Consul changes externally. Second scenario carried a FIXME `@ignore`, preserved as a skipped test. | 2 | [tests/acceptance/dc/list-blocking-test.js](tests/acceptance/dc/list-blocking-test.js) |
| `dc/routing-config.feature` | View a routing config (no 404, correct title) and its source pill. | 2 | [tests/acceptance/dc/routing-config-test.js](tests/acceptance/dc/routing-config-test.js) |
| `dc/acls/auth-methods/index.feature` | Auth-methods list renders and is searchable (3 search terms). | 2 | [tests/acceptance/dc/acls/auth-methods/index-test.js](tests/acceptance/dc/acls/auth-methods/index-test.js) |
| `dc/acls/policies/index.feature` | Policies list renders and is searchable. The original `@ignore` scenario (global-management policy can't be deleted) was dropped rather than kept as a skip. | 2 | [tests/acceptance/dc/acls/policies/index-test.js](tests/acceptance/dc/acls/policies/index-test.js) |
| `dc/acls/roles/index.feature` | Roles list renders and is searchable (description, policy name, service identity). | 2 | [tests/acceptance/dc/acls/roles/index-test.js](tests/acceptance/dc/acls/roles/index-test.js) |
| `dc/acls/tokens/index.feature` | Token list: view, no-write access, search (description/policy/role/SI), legacy notice. | 5 | [tests/acceptance/dc/acls/tokens/index-test.js](tests/acceptance/dc/acls/tokens/index-test.js) |
| `dc/acls/auth-methods/navigation.feature` | Navigate into an auth-method from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/acls/auth-methods/navigation-test.js](tests/acceptance/dc/acls/auth-methods/navigation-test.js) |
| `dc/acls/policies/navigation.feature` | Navigate into a policy from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/acls/policies/navigation-test.js](tests/acceptance/dc/acls/policies/navigation-test.js) |
| `dc/acls/roles/navigation.feature` | Navigate into a role from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/acls/roles/navigation-test.js](tests/acceptance/dc/acls/roles/navigation-test.js) |
| `dc/acls/tokens/navigation.feature` | Navigate into a token from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/acls/tokens/navigation-test.js](tests/acceptance/dc/acls/tokens/navigation-test.js) |
| `dc/intentions/navigation.feature` | Navigate into an intention and to the create form and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 2 | [tests/acceptance/dc/intentions/navigation-test.js](tests/acceptance/dc/intentions/navigation-test.js) |
| `dc/nodes/navigation.feature` | Navigate into a node from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/nodes/navigation-test.js](tests/acceptance/dc/nodes/navigation-test.js) |
| `dc/services/navigation.feature` | Navigate into a (peered) service from the list and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 2 | [tests/acceptance/dc/services/navigation-test.js](tests/acceptance/dc/services/navigation-test.js) |
| `dc/services/instances/navigation.feature` | Navigate into a service instance from the instances tab and back via breadcrumb. ⚠️ Original was namespaceable; the port added `notNamespaceable` (see *Known regressions*). | 1 | [tests/acceptance/dc/services/instances/navigation-test.js](tests/acceptance/dc/services/instances/navigation-test.js) |
| `dc/nodes/show/health-checks.feature` | Node serf health check passing/failing display. Also adds a regression scenario for the "Check type" sort (Service to Node / Node to Service ordering must match the option labels). | 2 (+1 new) | [tests/acceptance/dc/nodes/show/health-checks-test.js](tests/acceptance/dc/nodes/show/health-checks-test.js) |
| `dc/nodes/sessions/list.feature` | Node lock-sessions HDS table: string TTLs, nanosecond LockDelay rendering, and ACL enabled/disabled empty states. ⚠️ The ACLs-disabled scenario gained `notNamespaceable`. | 4 | [tests/acceptance/dc/nodes/sessions/list-test.js](tests/acceptance/dc/nodes/sessions/list-test.js) |
| `dc/nodes/sessions/invalidate.feature` | Invalidate a lock session via the HDS confirmation Modal (success + forced-500 error toast). | 2 | [tests/acceptance/dc/nodes/sessions/invalidate-test.js](tests/acceptance/dc/nodes/sessions/invalidate-test.js) |

## Remaining features

| Feature file | Description | Scenarios | Status | Native test |
| --- | --- | --- | --- | --- |
| [api-prefix.feature](tests/acceptance/api-prefix.feature) | UI honours a configured API path prefix when making requests. | 1 | ☐ Not migrated |  |
| [dc/acls/access.feature](tests/acceptance/dc/acls/access.feature) | ACLs access page behaviour when ACLs are disabled. | 1 | ☐ Not migrated |  |
| ~~`dc/acls/auth-methods/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/auth-methods/navigation-test.js) |
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
| ~~`dc/acls/policies/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/policies/navigation-test.js) |
| [dc/acls/policies/sorting.feature](tests/acceptance/dc/acls/policies/sorting.feature) | Sorting the policies list. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/update.feature](tests/acceptance/dc/acls/policies/update.feature) | Update a policy (name/rules/description); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/policies/view-management.feature](tests/acceptance/dc/acls/policies/view-management.feature) | The built-in management policy is shown read-only. | 1 | ☐ Not migrated |  |
| [dc/acls/policies/view-read-only.feature](tests/acceptance/dc/acls/policies/view-read-only.feature) | Read-only policy view. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/add-existing.feature](tests/acceptance/dc/acls/roles/as-many/add-existing.feature) | Attach an existing role as a child of a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/add-new.feature](tests/acceptance/dc/acls/roles/as-many/add-new.feature) | Add a new role with/without policies and service identities; cancel the form. | 5 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/list.feature](tests/acceptance/dc/acls/roles/as-many/list.feature) | List the roles attached to a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/as-many/remove.feature](tests/acceptance/dc/acls/roles/as-many/remove.feature) | Remove attached roles from a token. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/create.feature](tests/acceptance/dc/acls/roles/create.feature) | Create an ACL role; no Namespace sent when namespaces are disabled. | 3 | ☐ Not migrated |  |
| ~~`dc/acls/roles/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/roles/navigation-test.js) |
| [dc/acls/roles/sorting.feature](tests/acceptance/dc/acls/roles/sorting.feature) | Sorting the roles list. | 1 | ☐ Not migrated |  |
| [dc/acls/roles/update.feature](tests/acceptance/dc/acls/roles/update.feature) | Update a role (name/rules/description); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/tokens/anonymous-no-delete.feature](tests/acceptance/dc/acls/tokens/anonymous-no-delete.feature) | The anonymous token exposes no delete buttons on list or detail pages. | 2 | ☐ Not migrated |  |
| [dc/acls/tokens/clone.feature](tests/acceptance/dc/acls/tokens/clone.feature) | Clone an ACL token from the list and detail pages. | 2 | ☐ Not migrated |  |
| [dc/acls/tokens/create.feature](tests/acceptance/dc/acls/tokens/create.feature) | Create an ACL token; no Namespace sent when namespaces are disabled. | 3 | ☐ Not migrated |  |
| ~~`dc/acls/tokens/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/tokens/navigation-test.js) |
| [dc/acls/tokens/own-no-delete.feature](tests/acceptance/dc/acls/tokens/own-no-delete.feature) | Your currently-used token exposes no delete button. | 1 | ☐ Not migrated |  |
| [dc/acls/tokens/sorting.feature](tests/acceptance/dc/acls/tokens/sorting.feature) | Sorting the tokens list. | 1 | ☐ Not migrated |  |
| [dc/acls/tokens/update.feature](tests/acceptance/dc/acls/tokens/update.feature) | Update a token (name); error handling; no Namespace when disabled. | 3 | ☐ Not migrated |  |
| [dc/acls/tokens/use.feature](tests/acceptance/dc/acls/tokens/use.feature) | Switch to (use) an ACL token from the list and detail pages. | 2 | ☐ Not migrated |  |
| [dc/intentions/delete.feature](tests/acceptance/dc/intentions/delete.feature) | Delete intentions from list/detail incl. error and duplicate-intention error. | 4 | ☐ Not migrated |  |
| [dc/intentions/filtered-select.feature](tests/acceptance/dc/intentions/filtered-select.feature) | Service select dropdowns show services but exclude proxy services; handles same-name services in different namespaces. | 2 | ☐ Not migrated |  |
| [dc/intentions/form-select.feature](tests/acceptance/dc/intentions/form-select.feature) | Type into the autocomplete and select a custom/future service. | 1 | ☐ Not migrated |  |
| [dc/intentions/index.feature](tests/acceptance/dc/intentions/index.feature) | Intentions list: view, no-write access, live updates, with/without CRDs, empty states. | 7 | ☐ Not migrated |  |
| ~~`dc/intentions/navigation.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/intentions/navigation-test.js) |
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
| [dc/nodes/empty-ids.feature](tests/acceptance/dc/nodes/empty-ids.feature) | Node list handles nodes that arrive with no ID. | 1 | ☐ Not migrated |  |
| [dc/nodes/index.feature](tests/acceptance/dc/nodes/index.feature) | Nodes list: unhealthy node/service checks, synthetic nodes hidden, leader indicator, search, empty state. | 7 | ☐ Not migrated |  |
| ~~`dc/nodes/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/nodes/navigation-test.js) |
| [dc/nodes/no-leader.feature](tests/acceptance/dc/nodes/no-leader.feature) | Behaviour when no leader has been elected. | 1 | ☐ Not migrated |  |
| [dc/nodes/services/list.feature](tests/acceptance/dc/nodes/services/list.feature) | Node → Services tab listing. | 1 | ☐ Not migrated |  |
| ~~`dc/nodes/sessions/invalidate.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [invalidate-test.js](tests/acceptance/dc/nodes/sessions/invalidate-test.js) |
| ~~`dc/nodes/sessions/list.feature`~~ | *(deleted — migrated above)* | 4 | ✅ Migrated | [list-test.js](tests/acceptance/dc/nodes/sessions/list-test.js) |
| [dc/nodes/show.feature](tests/acceptance/dc/nodes/show.feature) | Node detail: tab visibility/selection, deregister warning while blocking, RTT display. | 5 | ☐ Not migrated |  |
| ~~`dc/nodes/show/health-checks.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [health-checks-test.js](tests/acceptance/dc/nodes/show/health-checks-test.js) |
| [dc/nodes/sorting.feature](tests/acceptance/dc/nodes/sorting.feature) | Sorting the nodes list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/create.feature](tests/acceptance/dc/nspaces/create.feature) | Create a namespace. | 2 | ☐ Not migrated |  |
| [dc/nspaces/delete.feature](tests/acceptance/dc/nspaces/delete.feature) | Delete a namespace from list/detail incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/nspaces/index.feature](tests/acceptance/dc/nspaces/index.feature) | Namespaces list and search; the default namespace can't be deleted. | 3 | ☐ Not migrated |  |
| [dc/nspaces/manage.feature](tests/acceptance/dc/nspaces/manage.feature) | Managing namespaces. | 1 | ☐ Not migrated |  |
| [dc/nspaces/sorting.feature](tests/acceptance/dc/nspaces/sorting.feature) | Sorting the namespaces list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/update.feature](tests/acceptance/dc/nspaces/update.feature) | Update a namespace (description); error handling. | 2 | ☐ Not migrated |  |
| [dc/peers/create.feature](tests/acceptance/dc/peers/create.feature) | Generate a peering token (create peer). | 1 | ☐ Not migrated |  |
| [dc/peers/delete.feature](tests/acceptance/dc/peers/delete.feature) | Delete a peer incl. error; a peer already deleting can't be deleted again. | 3 | ☐ Not migrated |  |
| [dc/peers/index.feature](tests/acceptance/dc/peers/index.feature) | Peers list: view, sort, search, and empty states (ACLs on/off). | 5 | ☐ Not migrated |  |
| [dc/peers/regenerate.feature](tests/acceptance/dc/peers/regenerate.feature) | Regenerate a peering token. | 2 | ☐ Not migrated |  |
| [dc/peers/show.feature](tests/acceptance/dc/peers/show.feature) | Peer detail tabs: dialer/receiver, imported/exported services (empty & populated), addresses. | 8 | ☐ Not migrated |  |
| [dc/peers/sorting.feature](tests/acceptance/dc/peers/sorting.feature) | Sorting peers by name, ascending and descending. Added in #23911; `dc/peers/establish.feature` was removed in the same PR and its scenario folded into `dc/peers/create.feature`. | 1 | ☐ Not migrated |  |
| [dc/services/dc-switch.feature](tests/acceptance/dc/services/dc-switch.feature) | Services list reflects a datacenter switch. | 1 | ☐ Not migrated |  |
| [dc/services/error.feature](tests/acceptance/dc/services/error.feature) | Service page error / not-found handling. | 2 | ☐ Not migrated |  |
| [dc/services/index.feature](tests/acceptance/dc/services/index.feature) | Services list: services, gateways, mesh state, associated-service counts, empty states. | 6 | ☐ Not migrated |  |
| [dc/services/instances/error.feature](tests/acceptance/dc/services/instances/error.feature) | Missing service-instance handling. | 1 | ☐ Not migrated |  |
| [dc/services/instances/exposed-paths.feature](tests/acceptance/dc/services/instances/exposed-paths.feature) | Exposed Paths tab shown only when the instance has a proxy. | 2 | ☐ Not migrated |  |
| [dc/services/instances/gateway.feature](tests/acceptance/dc/services/instances/gateway.feature) | Gateway service-instance detail page. | 1 | ☐ Not migrated |  |
| [dc/services/instances/health-checks.feature](tests/acceptance/dc/services/instances/health-checks.feature) | Instance serf checks pass/fail; node health-check visibility on agentless vs non-agentless. | 4 | ☐ Not migrated |  |
| ~~`dc/services/instances/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/services/instances/navigation-test.js) |
| [dc/services/instances/show.feature](tests/acceptance/dc/services/instances/show.feature) | Service-instance detail: proxy presence, deregister warning while blocking, synthetic node. | 4 | ☐ Not migrated |  |
| [dc/services/instances/upstreams.feature](tests/acceptance/dc/services/instances/upstreams.feature) | Upstreams tab shown only when the instance has a proxy. | 2 | ☐ Not migrated |  |
| [dc/services/list-blocking.feature](tests/acceptance/dc/services/list-blocking.feature) | Service listing live-updates via blocking queries. | 1 | ☐ Not migrated |  |
| [dc/services/list.feature](tests/acceptance/dc/services/list.feature) | Listing services incl. peered services. | 2 | ☐ Not migrated |  |
| ~~`dc/services/navigation.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/services/navigation-test.js) |
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
| [page-navigation.feature](tests/acceptance/page-navigation.feature) | Navigation across pages. 4 active scenarios still need porting (index page, nav links, node and policies endpoint checks). 5 are pre-existing `@ignore` stubs; 1 is a placeholder for the create flow already ported (as `notNamespaceable`, see *Known regressions*). | 10 | 🔄 In progress | [page-navigation-create-test.js](tests/acceptance/page-navigation-create-test.js) |
| [startup.feature](tests/acceptance/startup.feature) | App boots when loading index.html into a browser. | 1 | ☐ Not migrated |  |
| [submit-blank.feature](tests/acceptance/submit-blank.feature) | Blank create forms keep the submit button disabled. | 2 | ☐ Not migrated |  |
| [token-header.feature](tests/acceptance/token-header.feature) | API requests send the Consul token header after a token is set. | 2 | ☐ Not migrated |  |
