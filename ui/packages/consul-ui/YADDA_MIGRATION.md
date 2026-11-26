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
- **Keep `js-yaml`:** it is a direct devDependency that `tests/helpers/nspace-template.js` now
  imports (as well as `tests/dictionary.js`), so it stays after the dictionary is deleted.
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

1. ✅ **Namespace templating.** ~32 features use `@namespace` / `@!namespace` placeholders that
   `tests/dictionary.js` rewrites (differently for CE and ENT). Ported to
   [tests/helpers/nspace-template.js](tests/helpers/nspace-template.js) as `nspaceJSON`,
   `nspaceYAML` and `nspaceEndpoint` (also re-exported from the harness), with unit tests in
   `tests/unit/helpers/nspace-template-test.js` for each CE/ENT/namespace case. Use them for any
   endpoint or fixture that contains a placeholder, so a test cannot pass while asserting the
   wrong `ns=` query string or payload.
2. ✅ **Harness gaps.** [tests/helpers/acceptance.js](tests/helpers/acceptance.js) now also has the
   fixture setup (`createModels`, `respondWith`, `setSettings`, `setUiConfig`, `setPermissions`,
   `disableACLs`, `enableSSO`, `enablePartitions`, `setDefaultACLPolicy`, `setMetricsProvider`,
   `setLocalDatacenter`, `setLatency`, `setApiPrefix`, `useLegacyToken`), the remaining request
   assertions (`requestsIncluded`, `requestMadeWithNoBody`, `requestMadeWithoutProperties`) and
   page-object queries (`find`, `assertVisibleCount`, `assertProperties`), `fillInForm` (plain
   inputs), `stubOIDCProvider` and `clickParentBreadcrumb`. `visit` clears the request history, as
   the yadda `I visit` steps did. Still missing and to be added by the batch that first needs
   them: nothing so far. Sort-order assertions use `assertPropertiesVertically`; code editors
   are filled by `fillInForm` / `fillInCodeEditor`; the yadda `pause until I see N models`
   steps are `waitForVisibleCount`; `visit(name, params, { method })` calls other visitables
   such as `visitExported`. Prefer plain
   `assert.dom(...)` for DOM assertions (pass `document.documentElement`
   to assert on `<html>`). Add shared helpers to the harness, not inside individual tests.
3. ✅ **Baseline.** Recorded from `main` before the ACL token batch: the sorted test-title list (and
   pass/skip counts) from `ember test` under both `CONSUL_NSPACES_ENABLED=0` and `=1`. Compare against
   it after every batch so dropped scenarios, `Where:` rows or tags are caught. Note that the
   test environment defaults to namespaces **off** (`config/environment.js`), so a plain
   `pnpm test` only exercises CE.
4. ✅ **Fix the regressions** below, so later batches don't copy the weakened navigation pattern.
5. ✅ **`@ignore` scenarios.** The 8 empty `Then ok` placeholders and the 19 orphaned
   `-steps.js` stubs were deleted. The remaining ignored scenario has a real body: the
   scenario-level `@ignore` in `dc/kvs/index`. (`dc/acls/tokens/use` "from the detail page" and most
   of `dc/acls/policies/as-many/add-new` pass once migrated and are no longer ignored.)
   When migrating their feature, fix and un-ignore them where possible. Otherwise port each as
   a skipped test (`{ ignore: true }`) that keeps the original steps in its body, with the
   reason in a comment, so it can be un-skipped later (see `dc/acls/policies/index-test.js` and
   `page-navigation-test.js`).

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
   from `tests/acceptance/steps/<same path>/<name>-steps.js`, so the pairing is exact. (19 orphaned stubs with no matching `.feature` were already removed.)
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
| Feature files remaining (yadda) | 48 |
| Scenarios remaining (yadda) | 125 (1 is `@ignore`, with a real body: the scenario-level tag in `dc/kvs/index`) |
| Feature files fully migrated | 81 |
| Scenarios migrated | 167 (46 from the first 27 features, 24 from the next 12, 14 from the 7 ACL token features, 23 from the 12 ACL policy features, 16 from the 8 ACL role and auth-method features, 44 from the 15 intention and peer features) |

Scenario counts are not test counts. 20 remaining features use `Where:` tables (one test per
row), and under `CONSUL_NSPACES_ENABLED=1` each namespaceable scenario runs 4 times.

`tests/acceptance` contains 90 native `*-test.js` files: 81 come from migrated features and 9 are native-only tests that never had a
`.feature` (`breadcrumbs/*`, `components/tab-error`, `components/tab-nav-focus`,
`dc/nodes/sessions/filter-sort`, `hcp-login`, `unavailable`). Counts are from disk; re-verify with
`find tests -name '*.feature' | wc -l`.

## Regressions found in completed migrations (fixed) and findings

Audited by comparing each native test with the original `.feature` removed in #23741 and #23911.
The #23741 batch (intentions/create, components, settings, dc/*) was faithful, although #23911
later rewrote its `components/text-input` port. The #23911 batch was not, and has been corrected:

1. **ENT namespace coverage restored.** 15 scenarios in 11 files had `notNamespaceable: true`
   although the original scenario had no namespace tag (all `*/navigation` tests, `components/text-input`,
   the sessions "ACLs disabled" scenario and the 3 `page-navigation` create rows). They run for
   every namespace again (about 60 more Enterprise tests); all pass.
2. **Assertions restored.** Exact URL checks (`assert.equal(currentURL(), nspaceURL(...))`)
   replace `includes(...)`, the "I see N models" counts are back, and
   `dc/services/instances/navigation-test.js` has its proxy/instance fixtures and namespaced
   `GET /v1/catalog/connect/...` / `GET /v1/health/service/...-proxy` request checks again.
   `dc/intentions/navigation` "Clicking the create button and back again" asserts the title
   again. The `dc-1` navigation ports create the datacenter with `createModels`, which sets it
   as the local datacenter like the yadda step, and `dc/nodes/navigation` uses the original
   node fixture again.
3. **Other weakened ports fixed.** `components/text-input` visits the KV create page and asserts
   `/dc-1/kv/create` again, as the #23741 port did. `dc/nodes/sessions/list` checks the selected
   tab and the full empty-state sentences in every scenario and has its original titles back.
4. **Dropped `@ignore` scenario restored.** `dc/acls/policies/index` keeps its ignored scenario
   as a skipped test with the original steps in its body, so it can be un-skipped later.

**Behaviour changes the exact assertions exposed** (these are product questions, not test
problems; the tests document the current behaviour):

- The breadcrumb link back to a list keeps the peer route segment: opening the first (peered)
  service or node and going back lands on `/:billing/dc-1/services` / `/:billing/dc-1/nodes`,
  where the old `[data-test-back] a` link went to `/dc-1/services`. This happens in CE as well
  as ENT. `app/services/breadcrumbs.js` says it intends to avoid inheriting peer segments, so
  this is likely a bug worth a separate fix. The nodes and services tests click the first
  (peered) item, as the originals did, and assert today's URL exactly with a `FIXME`, so fixing
  the bug makes them fail until they are updated to the original `/dc-1/...` URLs.
- Going "back" from a service instance now returns to that service's *instances* tab (the old
  back link landed on the topology tab).

## Completed

| Feature (was) | Description | Scenarios | Native test |
| --- | --- | --- | --- |
| `dc/intentions/create.feature` | Create an intention from the intentions form (source/destination, deny action) and assert the PUT payload; namespaces on and off. | 2 | [tests/acceptance/dc/intentions/create-test.js](tests/acceptance/dc/intentions/create-test.js) |
| `components/copy-button.feature` | Copy-to-clipboard button copies the expected value when clicked. Carried the feature-level `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/components/copy-button-test.js](tests/acceptance/components/copy-button-test.js) |
| `components/kv-filter.feature` | Free-text filter on the KV list narrows results by the typed text. | 1 (2 rows) | [tests/acceptance/components/kv-filter-test.js](tests/acceptance/components/kv-filter-test.js) |
| `components/text-input.feature` | Text-input component behaves correctly (KV create page enables submit once filled). | 1 | [tests/acceptance/components/text-input-test.js](tests/acceptance/components/text-input-test.js) |
| `settings/show.feature` | Settings page shows Blocking Queries; `CONSUL_UI_DISABLE_REALTIME` hides them. `@notNamespaceable` (CE-only, as before). | 2 | [tests/acceptance/settings/show-test.js](tests/acceptance/settings/show-test.js) |
| `settings/update.feature` | Saving settings with no input typed. Carried the feature-level `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/settings/update-test.js](tests/acceptance/settings/update-test.js) |
| `dc/error.feature` | Recovering from a datacenter 500 error: error status shown, then switching dc via the nav recovers and lists services. | 1 | [tests/acceptance/dc/error-test.js](tests/acceptance/dc/error-test.js) |
| `dc/forwarding.feature` | Arriving at a dc with only the dc in the URL redirects to its services overview. `@notNamespaceable` (CE-only, as before). | 1 | [tests/acceptance/dc/forwarding-test.js](tests/acceptance/dc/forwarding-test.js) |
| `dc/index.feature` | Datacenters index lands on the services page. Carried the scenario `@ignore`, preserved as a skipped test. | 1 | [tests/acceptance/dc/index-test.js](tests/acceptance/dc/index-test.js) |
| `dc/list.feature` | Generic model listing pages render (nodes/kv/tokens/policies). | 1 (4 rows) | [tests/acceptance/dc/list-test.js](tests/acceptance/dc/list-test.js) |
| `dc/list-blocking.feature` | Listing pages live-update via blocking queries when Consul changes externally. Second scenario carried a FIXME `@ignore`, preserved as a skipped test. | 2 | [tests/acceptance/dc/list-blocking-test.js](tests/acceptance/dc/list-blocking-test.js) |
| `dc/routing-config.feature` | View a routing config (no 404, correct title) and its source pill. | 2 | [tests/acceptance/dc/routing-config-test.js](tests/acceptance/dc/routing-config-test.js) |
| `dc/acls/auth-methods/index.feature` | Auth-methods list renders and is searchable (3 search terms). | 2 | [tests/acceptance/dc/acls/auth-methods/index-test.js](tests/acceptance/dc/acls/auth-methods/index-test.js) |
| `dc/acls/policies/index.feature` | Policies list renders and is searchable. The original `@ignore` scenario (global-management policy can't be deleted) is kept as a skipped test with its steps. | 3 | [tests/acceptance/dc/acls/policies/index-test.js](tests/acceptance/dc/acls/policies/index-test.js) |
| `dc/acls/roles/index.feature` | Roles list renders and is searchable (description, policy name, service identity). | 2 | [tests/acceptance/dc/acls/roles/index-test.js](tests/acceptance/dc/acls/roles/index-test.js) |
| `dc/acls/tokens/index.feature` | Token list: view, no-write access, search (description/policy/role/SI), legacy notice. | 5 | [tests/acceptance/dc/acls/tokens/index-test.js](tests/acceptance/dc/acls/tokens/index-test.js) |
| `dc/acls/auth-methods/navigation.feature` | Navigate into an auth-method from the list and back via breadcrumb. | 1 | [tests/acceptance/dc/acls/auth-methods/navigation-test.js](tests/acceptance/dc/acls/auth-methods/navigation-test.js) |
| `dc/acls/policies/navigation.feature` | Navigate into a policy from the list and back via breadcrumb. | 1 | [tests/acceptance/dc/acls/policies/navigation-test.js](tests/acceptance/dc/acls/policies/navigation-test.js) |
| `dc/acls/roles/navigation.feature` | Navigate into a role from the list and back via breadcrumb. | 1 | [tests/acceptance/dc/acls/roles/navigation-test.js](tests/acceptance/dc/acls/roles/navigation-test.js) |
| `dc/acls/tokens/navigation.feature` | Navigate into a token from the list and back via breadcrumb. | 1 | [tests/acceptance/dc/acls/tokens/navigation-test.js](tests/acceptance/dc/acls/tokens/navigation-test.js) |
| `dc/intentions/navigation.feature` | Navigate into an intention and to the create form and back via breadcrumb. | 2 | [tests/acceptance/dc/intentions/navigation-test.js](tests/acceptance/dc/intentions/navigation-test.js) |
| `dc/nodes/navigation.feature` | Navigate into a node from the list and back via breadcrumb. | 1 | [tests/acceptance/dc/nodes/navigation-test.js](tests/acceptance/dc/nodes/navigation-test.js) |
| `dc/services/navigation.feature` | Navigate into a (peered) service from the list and back via breadcrumb. | 2 | [tests/acceptance/dc/services/navigation-test.js](tests/acceptance/dc/services/navigation-test.js) |
| `dc/services/instances/navigation.feature` | Navigate into a service instance from the instances tab and back via breadcrumb. | 1 | [tests/acceptance/dc/services/instances/navigation-test.js](tests/acceptance/dc/services/instances/navigation-test.js) |
| `dc/nodes/show/health-checks.feature` | Node serf health check passing/failing display. Also adds a regression scenario for the "Check type" sort (Service to Node / Node to Service ordering must match the option labels). | 2 (+1 new) | [tests/acceptance/dc/nodes/show/health-checks-test.js](tests/acceptance/dc/nodes/show/health-checks-test.js) |
| `dc/nodes/sessions/list.feature` | Node lock-sessions HDS table: string TTLs, nanosecond LockDelay rendering, and ACL enabled/disabled empty states. | 4 | [tests/acceptance/dc/nodes/sessions/list-test.js](tests/acceptance/dc/nodes/sessions/list-test.js) |
| `dc/nodes/sessions/invalidate.feature` | Invalidate a lock session via the HDS confirmation Modal (success + forced-500 error toast). | 2 | [tests/acceptance/dc/nodes/sessions/invalidate-test.js](tests/acceptance/dc/nodes/sessions/invalidate-test.js) |
| `api-prefix.feature` | UI honours a configured API path prefix when making requests. | 1 | [tests/acceptance/api-prefix-test.js](tests/acceptance/api-prefix-test.js) |
| `dc/acls/access.feature` | ACLs access page behaviour when ACLs are disabled. | 1 | [tests/acceptance/dc/acls/access-test.js](tests/acceptance/dc/acls/access-test.js) |
| `dc/acls/index.feature` | ACL index page forwards/redirects to the correct sub-page. `@notNamespaceable`, as before. | 1 | [tests/acceptance/dc/acls/index-test.js](tests/acceptance/dc/acls/index-test.js) |
| `deleting.feature` | Generic delete flow: confirmation dialog plus success/error notifications. | 3 | [tests/acceptance/deleting-test.js](tests/acceptance/deleting-test.js) |
| `index-forwarding.feature` | Index page forwards straight through when there is only one datacenter. `@notNamespaceable`, as before. | 1 | [tests/acceptance/index-forwarding-test.js](tests/acceptance/index-forwarding-test.js) |
| `login.feature` | Logging in via an ACL token and via SSO. The SSO scenario stays `@onlyNamespaceable`. | 2 | [tests/acceptance/login-test.js](tests/acceptance/login-test.js) |
| `login-errors.feature` | Login 500-error handling (non-legacy-token case). | 1 | [tests/acceptance/login-errors-test.js](tests/acceptance/login-errors-test.js) |
| `navigation-links.feature` | Main-navigation link visibility (e.g. no KV read access, empty-state login button). | 2 | [tests/acceptance/navigation-links-test.js](tests/acceptance/navigation-links-test.js) |
| `page-navigation.feature` | Navigation across pages: index forwarding, main-navigation links, node and policies endpoint checks, and the create flows from the intentions/tokens/policies listings. The three `@ignore` kv scenarios are kept as skipped tests with their original steps; the empty placeholders were deleted. | 7 | [tests/acceptance/page-navigation-test.js](tests/acceptance/page-navigation-test.js) |
| `startup.feature` | App boots when loading index.html into a browser. `@notNamespaceable`, as before. | 1 | [tests/acceptance/startup-test.js](tests/acceptance/startup-test.js) |
| `submit-blank.feature` | Blank create forms keep the submit button disabled. | 1 | [tests/acceptance/submit-blank-test.js](tests/acceptance/submit-blank-test.js) |
| `token-header.feature` | API requests send the Consul token header after a token is set. `@notNamespaceable`, as before. | 2 | [tests/acceptance/token-header-test.js](tests/acceptance/token-header-test.js) |
| `dc/acls/tokens/anonymous-no-delete.feature` | The anonymous token has no delete action on the list and no delete confirmation on its detail page. | 2 | [tests/acceptance/dc/acls/tokens/anonymous-no-delete-test.js](tests/acceptance/dc/acls/tokens/anonymous-no-delete-test.js) |
| `dc/acls/tokens/clone.feature` | Clone (duplicate) a token from the list (namespaced `PUT .../clone`) and from its detail page. | 2 | [tests/acceptance/dc/acls/tokens/clone-test.js](tests/acceptance/dc/acls/tokens/clone-test.js) |
| `dc/acls/tokens/create.feature` | Create page title; create a token (2 `Where:` rows) with the namespaced `PUT`; no `Namespace` sent when namespaces are disabled (`@notNamespaceable`, as before). | 3 (4 tests) | [tests/acceptance/dc/acls/tokens/create-test.js](tests/acceptance/dc/acls/tokens/create-test.js) |
| `dc/acls/tokens/own-no-delete.feature` | After using a token it is stored in settings and has no delete action or confirmation. | 1 | [tests/acceptance/dc/acls/tokens/own-no-delete-test.js](tests/acceptance/dc/acls/tokens/own-no-delete-test.js) |
| `dc/acls/tokens/sorting.feature` | Sort tokens by creation time both ways (uses the new `assertPropertiesVertically` harness helper). The placeholder skip in `index-test.js` was removed. | 1 | [tests/acceptance/dc/acls/tokens/sorting-test.js](tests/acceptance/dc/acls/tokens/sorting-test.js) |
| `dc/acls/tokens/update.feature` | Update a token's description (2 `Where:` rows, now titled by description instead of the literal `Update to [Name]`), 500 error, and no `Namespace` when disabled. | 3 (4 tests) | [tests/acceptance/dc/acls/tokens/update-test.js](tests/acceptance/dc/acls/tokens/update-test.js) |
| `dc/acls/tokens/use.feature` | Use a token from the list and from the detail page. The detail-page scenario was `@ignore` (FIXME); it passes now and is un-ignored. | 2 | [tests/acceptance/dc/acls/tokens/use-test.js](tests/acceptance/dc/acls/tokens/use-test.js) |
| `dc/acls/policies/create.feature` | Create page title; create a policy (2 `Where:` rows) with the namespaced `PUT`; no `Namespace` when namespaces are disabled (`@notNamespaceable`, as before). | 3 (4 tests) | [tests/acceptance/dc/acls/policies/create-test.js](tests/acceptance/dc/acls/policies/create-test.js) |
| `dc/acls/policies/delete.feature` | Delete from the list (success and 500) and from the detail page. The commented-out detail-page error scenario was not ported. | 3 | [tests/acceptance/dc/acls/policies/delete-test.js](tests/acceptance/dc/acls/policies/delete-test.js) |
| `dc/acls/policies/sorting.feature` | Sort policies by name both ways. | 1 | [tests/acceptance/dc/acls/policies/sorting-test.js](tests/acceptance/dc/acls/policies/sorting-test.js) |
| `dc/acls/policies/update.feature` | Update name/rules (code editor)/description and datacenters (3 `Where:` rows), 500 error, no `Namespace` when disabled. | 3 (5 tests) | [tests/acceptance/dc/acls/policies/update-test.js](tests/acceptance/dc/acls/policies/update-test.js) |
| `dc/acls/policies/view-management.feature` | The global-management policy is read-only (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/policies/view-management-test.js](tests/acceptance/dc/acls/policies/view-management-test.js) |
| `dc/acls/policies/view-read-only.feature` | The builtin read-only policy is read-only (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/policies/view-read-only-test.js](tests/acceptance/dc/acls/policies/view-read-only-test.js) |
| `dc/acls/policies/as-many/add-existing.feature` | Attach two existing policies to a role through the power select. | 1 | [tests/acceptance/dc/acls/policies/as-many/add-existing-test.js](tests/acceptance/dc/acls/policies/as-many/add-existing-test.js) |
| `dc/acls/policies/as-many/add-new.feature` | Was feature-level `@ignore`. Now runs: add a new policy, the 500 error message (via a new `data-test-rules-error` hook; the old `[data-test-rules] strong` markup is gone), and read-only rules for identities. Still skipped (FIXME, body kept): adding a service/node identity, because saving the token/role then fails with an error toast and no `PUT`. | 6 (12 tests, 4 skipped) | [tests/acceptance/dc/acls/policies/as-many/add-new-test.js](tests/acceptance/dc/acls/policies/as-many/add-new-test.js) |
| `dc/acls/policies/as-many/list.feature` | Identities and policies are listed together on token/role pages. | 1 (2 rows) | [tests/acceptance/dc/acls/policies/as-many/list-test.js](tests/acceptance/dc/acls/policies/as-many/list-test.js) |
| `dc/acls/policies/as-many/nspaces.feature` | Namespace policies can't be service identities. `@onlyNamespaceable`, as before (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/policies/as-many/nspaces-test.js](tests/acceptance/dc/acls/policies/as-many/nspaces-test.js) |
| `dc/acls/policies/as-many/remove.feature` | Remove an attached policy from a token/role and save an empty list. | 1 (2 rows) | [tests/acceptance/dc/acls/policies/as-many/remove-test.js](tests/acceptance/dc/acls/policies/as-many/remove-test.js) |
| `dc/acls/policies/as-many/reset.feature` | Cancelling the new-policy form resets its fields (incl. the Rules editor). | 1 (2 rows) | [tests/acceptance/dc/acls/policies/as-many/reset-test.js](tests/acceptance/dc/acls/policies/as-many/reset-test.js) |
| `dc/acls/roles/create.feature` | Create page title; create a role (2 `Where:` rows); no `Namespace` when namespaces are disabled (`@notNamespaceable`, as before). | 3 (4 tests) | [tests/acceptance/dc/acls/roles/create-test.js](tests/acceptance/dc/acls/roles/create-test.js) |
| `dc/acls/roles/sorting.feature` | Sort roles by name and creation index, both ways. | 1 | [tests/acceptance/dc/acls/roles/sorting-test.js](tests/acceptance/dc/acls/roles/sorting-test.js) |
| `dc/acls/roles/update.feature` | Update name/description (3 `Where:` rows, titles without the literal `[Rules]`), 500 error, no `Namespace` when disabled. | 3 (5 tests) | [tests/acceptance/dc/acls/roles/update-test.js](tests/acceptance/dc/acls/roles/update-test.js) |
| `dc/acls/roles/as-many/add-existing.feature` | Attach two existing roles to a token (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/roles/as-many/add-existing-test.js](tests/acceptance/dc/acls/roles/as-many/add-existing-test.js) |
| `dc/acls/roles/as-many/add-new.feature` | Create a role inline from a token (no policy, existing policy, new policy) and cancel. `@notNamespaceable`, as before. "Add a new Service Identity" was `@ignore`d and still fails, so it stays skipped (FIXME, body kept). | 5 (1 skipped) | [tests/acceptance/dc/acls/roles/as-many/add-new-test.js](tests/acceptance/dc/acls/roles/as-many/add-new-test.js) |
| `dc/acls/roles/as-many/list.feature` | A token's roles are listed (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/roles/as-many/list-test.js](tests/acceptance/dc/acls/roles/as-many/list-test.js) |
| `dc/acls/roles/as-many/remove.feature` | Remove a role from a token and save an empty list (untitled scenario, now titled). | 1 | [tests/acceptance/dc/acls/roles/as-many/remove-test.js](tests/acceptance/dc/acls/roles/as-many/remove-test.js) |
| `dc/acls/auth-methods/sorting.feature` | Sort auth methods by name both ways. | 1 | [tests/acceptance/dc/acls/auth-methods/sorting-test.js](tests/acceptance/dc/acls/auth-methods/sorting-test.js) |
| `dc/intentions/delete.feature` | Delete from the list and detail pages, incl. 500 and duplicate-intention errors. | 4 | [tests/acceptance/dc/intentions/delete-test.js](tests/acceptance/dc/intentions/delete-test.js) |
| `dc/intentions/filtered-select.feature` | Source/destination dropdowns hide proxy services and de-duplicate names across namespaces. | 2 (4 tests) | [tests/acceptance/dc/intentions/filtered-select-test.js](tests/acceptance/dc/intentions/filtered-select-test.js) |
| `dc/intentions/form-select.feature` | Typing a future service into the source/destination selects it. | 1 (2 tests) | [tests/acceptance/dc/intentions/form-select-test.js](tests/acceptance/dc/intentions/form-select-test.js) |
| `dc/intentions/index.feature` | Listing, no-write access (create still shown, as before), live updates (new `waitForVisibleCount` helper), CRD notice, empty states with ACLs on/off. | 7 | [tests/acceptance/dc/intentions/index-test.js](tests/acceptance/dc/intentions/index-test.js) |
| `dc/intentions/read-only.feature` | A Kubernetes-managed intention is read-only. | 1 | [tests/acceptance/dc/intentions/read-only-test.js](tests/acceptance/dc/intentions/read-only-test.js) |
| `dc/intentions/sorting.feature` | Sort by source both ways. `@notNamespaceable`, as before. | 1 | [tests/acceptance/dc/intentions/sorting-test.js](tests/acceptance/dc/intentions/sorting-test.js) |
| `dc/intentions/update.feature` | Update description and action (request body now checked), 500 error. | 2 | [tests/acceptance/dc/intentions/update-test.js](tests/acceptance/dc/intentions/update-test.js) |
| `dc/intentions/permissions/create.feature` | Add an L7 permission with a header (untitled scenario, now titled). | 1 | [tests/acceptance/dc/intentions/permissions/create-test.js](tests/acceptance/dc/intentions/permissions/create-test.js) |
| `dc/intentions/permissions/warn.feature` | Was feature-level `@ignore`; passes now and runs again (untitled scenario, now titled). | 1 | [tests/acceptance/dc/intentions/permissions/warn-test.js](tests/acceptance/dc/intentions/permissions/warn-test.js) |
| `dc/peers/create.feature` | Add-peer page, generate token, establish peering, and cancel with/without unsaved input. | 5 | [tests/acceptance/dc/peers/create-test.js](tests/acceptance/dc/peers/create-test.js) |
| `dc/peers/delete.feature` | Delete from the list (success and 500); a deleting peer has no actions. | 3 | [tests/acceptance/dc/peers/delete-test.js](tests/acceptance/dc/peers/delete-test.js) |
| `dc/peers/index.feature` | List, sort by name, search, and empty search states with ACLs on/off. | 5 | [tests/acceptance/dc/peers/index-test.js](tests/acceptance/dc/peers/index-test.js) |
| `dc/peers/regenerate.feature` | Dialers can regenerate the token, receivers can't (untitled scenarios, now titled). | 2 | [tests/acceptance/dc/peers/regenerate-test.js](tests/acceptance/dc/peers/regenerate-test.js) |
| `dc/peers/show.feature` | Dialer/receiver tabs and empty/non-empty imported, exported and addresses tabs (via `visit(..., { method })`). | 8 | [tests/acceptance/dc/peers/show-test.js](tests/acceptance/dc/peers/show-test.js) |
| `dc/peers/sorting.feature` | Sort peers from the toolbar dropdown both ways. | 1 | [tests/acceptance/dc/peers/sorting-test.js](tests/acceptance/dc/peers/sorting-test.js) |

## Remaining features

| Feature file | Description | Scenarios | Status | Native test |
| --- | --- | --- | --- | --- |
| ~~`dc/acls/auth-methods/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/auth-methods/navigation-test.js) |
| ~~`dc/acls/auth-methods/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/acls/auth-methods/sorting-test.js) |
| ~~`dc/acls/policies/as-many/add-existing.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [add-existing-test.js](tests/acceptance/dc/acls/policies/as-many/add-existing-test.js) |
| ~~`dc/acls/policies/as-many/add-new.feature`~~ | *(deleted — migrated above)* | 6 | ✅ Migrated | [add-new-test.js](tests/acceptance/dc/acls/policies/as-many/add-new-test.js) |
| ~~`dc/acls/policies/as-many/list.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [list-test.js](tests/acceptance/dc/acls/policies/as-many/list-test.js) |
| ~~`dc/acls/policies/as-many/nspaces.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [nspaces-test.js](tests/acceptance/dc/acls/policies/as-many/nspaces-test.js) |
| ~~`dc/acls/policies/as-many/remove.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [remove-test.js](tests/acceptance/dc/acls/policies/as-many/remove-test.js) |
| ~~`dc/acls/policies/as-many/reset.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [reset-test.js](tests/acceptance/dc/acls/policies/as-many/reset-test.js) |
| ~~`dc/acls/policies/create.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [create-test.js](tests/acceptance/dc/acls/policies/create-test.js) |
| ~~`dc/acls/policies/delete.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [delete-test.js](tests/acceptance/dc/acls/policies/delete-test.js) |
| ~~`dc/acls/policies/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/policies/navigation-test.js) |
| ~~`dc/acls/policies/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/acls/policies/sorting-test.js) |
| ~~`dc/acls/policies/update.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [update-test.js](tests/acceptance/dc/acls/policies/update-test.js) |
| ~~`dc/acls/policies/view-management.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [view-management-test.js](tests/acceptance/dc/acls/policies/view-management-test.js) |
| ~~`dc/acls/policies/view-read-only.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [view-read-only-test.js](tests/acceptance/dc/acls/policies/view-read-only-test.js) |
| ~~`dc/acls/roles/as-many/add-existing.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [add-existing-test.js](tests/acceptance/dc/acls/roles/as-many/add-existing-test.js) |
| ~~`dc/acls/roles/as-many/add-new.feature`~~ | *(deleted — migrated above)* | 5 | ✅ Migrated | [add-new-test.js](tests/acceptance/dc/acls/roles/as-many/add-new-test.js) |
| ~~`dc/acls/roles/as-many/list.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [list-test.js](tests/acceptance/dc/acls/roles/as-many/list-test.js) |
| ~~`dc/acls/roles/as-many/remove.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [remove-test.js](tests/acceptance/dc/acls/roles/as-many/remove-test.js) |
| ~~`dc/acls/roles/create.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [create-test.js](tests/acceptance/dc/acls/roles/create-test.js) |
| ~~`dc/acls/roles/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/roles/navigation-test.js) |
| ~~`dc/acls/roles/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/acls/roles/sorting-test.js) |
| ~~`dc/acls/roles/update.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [update-test.js](tests/acceptance/dc/acls/roles/update-test.js) |
| ~~`dc/acls/tokens/anonymous-no-delete.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [anonymous-no-delete-test.js](tests/acceptance/dc/acls/tokens/anonymous-no-delete-test.js) |
| ~~`dc/acls/tokens/clone.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [clone-test.js](tests/acceptance/dc/acls/tokens/clone-test.js) |
| ~~`dc/acls/tokens/create.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [create-test.js](tests/acceptance/dc/acls/tokens/create-test.js) |
| ~~`dc/acls/tokens/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/acls/tokens/navigation-test.js) |
| ~~`dc/acls/tokens/own-no-delete.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [own-no-delete-test.js](tests/acceptance/dc/acls/tokens/own-no-delete-test.js) |
| ~~`dc/acls/tokens/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/acls/tokens/sorting-test.js) |
| ~~`dc/acls/tokens/update.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [update-test.js](tests/acceptance/dc/acls/tokens/update-test.js) |
| ~~`dc/acls/tokens/use.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [use-test.js](tests/acceptance/dc/acls/tokens/use-test.js) |
| ~~`dc/intentions/delete.feature`~~ | *(deleted — migrated above)* | 4 | ✅ Migrated | [delete-test.js](tests/acceptance/dc/intentions/delete-test.js) |
| ~~`dc/intentions/filtered-select.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [filtered-select-test.js](tests/acceptance/dc/intentions/filtered-select-test.js) |
| ~~`dc/intentions/form-select.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [form-select-test.js](tests/acceptance/dc/intentions/form-select-test.js) |
| ~~`dc/intentions/index.feature`~~ | *(deleted — migrated above)* | 7 | ✅ Migrated | [index-test.js](tests/acceptance/dc/intentions/index-test.js) |
| ~~`dc/intentions/navigation.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/intentions/navigation-test.js) |
| ~~`dc/intentions/permissions/create.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [create-test.js](tests/acceptance/dc/intentions/permissions/create-test.js) |
| ~~`dc/intentions/permissions/warn.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [warn-test.js](tests/acceptance/dc/intentions/permissions/warn-test.js) |
| ~~`dc/intentions/read-only.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [read-only-test.js](tests/acceptance/dc/intentions/read-only-test.js) |
| ~~`dc/intentions/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/intentions/sorting-test.js) |
| ~~`dc/intentions/update.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [update-test.js](tests/acceptance/dc/intentions/update-test.js) |
| [dc/kvs/create.feature](tests/acceptance/dc/kvs/create.feature) | Create a root KV and folders, incl. creating from within a folder. | 4 | ☐ Not migrated |  |
| [dc/kvs/delete.feature](tests/acceptance/dc/kvs/delete.feature) | Delete a KV from list/detail incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/kvs/edit.feature](tests/acceptance/dc/kvs/edit.feature) | View a KV: URL-unsafe characters, attached lock session, no-write / no-read access. | 6 | ☐ Not migrated |  |
| [dc/kvs/index.feature](tests/acceptance/dc/kvs/index.feature) | KV list view incl. no-write access. | 2 | ☐ Not migrated |  |
| [dc/kvs/list-order.feature](tests/acceptance/dc/kvs/list-order.feature) | KV keys/folders listed alphabetically. | 1 | ☐ Not migrated |  |
| [dc/kvs/sessions/invalidate.feature](tests/acceptance/dc/kvs/sessions/invalidate.feature) | Invalidate a lock session attached to a KV incl. error. | 2 | ☐ Not migrated |  |
| [dc/kvs/trailing-slash.feature](tests/acceptance/dc/kvs/trailing-slash.feature) | KV folder view resolves with and without a trailing slash. | 2 | ☐ Not migrated |  |
| [dc/kvs/update.feature](tests/acceptance/dc/kvs/update.feature) | Update KV values incl. whitespace/empty/newline values and error handling. | 5 | ☐ Not migrated |  |
| [dc/nodes/empty-ids.feature](tests/acceptance/dc/nodes/empty-ids.feature) | Node list handles nodes that arrive with no ID. | 1 | ☐ Not migrated |  |
| [dc/nodes/index.feature](tests/acceptance/dc/nodes/index.feature) | Nodes list: unhealthy node/service checks, synthetic nodes hidden, leader indicator, search, empty state. | 7 | ☐ Not migrated |  |
| ~~`dc/nodes/navigation.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [navigation-test.js](tests/acceptance/dc/nodes/navigation-test.js) |
| [dc/nodes/no-leader.feature](tests/acceptance/dc/nodes/no-leader.feature) | Behaviour when no leader has been elected. | 1 | ☐ Not migrated |  |
| [dc/nodes/services/list.feature](tests/acceptance/dc/nodes/services/list.feature) | Node → Services tab listing. | 1 | ☐ Not migrated |  |
| ~~`dc/nodes/sessions/invalidate.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [invalidate-test.js](tests/acceptance/dc/nodes/sessions/invalidate-test.js) |
| ~~`dc/nodes/sessions/list.feature`~~ | *(deleted — migrated above)* | 4 | ✅ Migrated | [list-test.js](tests/acceptance/dc/nodes/sessions/list-test.js) |
| [dc/nodes/show.feature](tests/acceptance/dc/nodes/show.feature) | Node detail: tab visibility/selection, deregister warning while blocking. | 3 | ☐ Not migrated |  |
| ~~`dc/nodes/show/health-checks.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [health-checks-test.js](tests/acceptance/dc/nodes/show/health-checks-test.js) |
| [dc/nodes/sorting.feature](tests/acceptance/dc/nodes/sorting.feature) | Sorting the nodes list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/create.feature](tests/acceptance/dc/nspaces/create.feature) | Create a namespace. | 2 | ☐ Not migrated |  |
| [dc/nspaces/delete.feature](tests/acceptance/dc/nspaces/delete.feature) | Delete a namespace from list/detail incl. error handling. | 3 | ☐ Not migrated |  |
| [dc/nspaces/index.feature](tests/acceptance/dc/nspaces/index.feature) | Namespaces list and search; the default namespace can't be deleted. | 3 | ☐ Not migrated |  |
| [dc/nspaces/manage.feature](tests/acceptance/dc/nspaces/manage.feature) | Managing namespaces. | 1 | ☐ Not migrated |  |
| [dc/nspaces/sorting.feature](tests/acceptance/dc/nspaces/sorting.feature) | Sorting the namespaces list. | 1 | ☐ Not migrated |  |
| [dc/nspaces/update.feature](tests/acceptance/dc/nspaces/update.feature) | Update a namespace (description); error handling. | 2 | ☐ Not migrated |  |
| ~~`dc/peers/create.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [create-test.js](tests/acceptance/dc/peers/create-test.js) |
| ~~`dc/peers/delete.feature`~~ | *(deleted — migrated above)* | 3 | ✅ Migrated | [delete-test.js](tests/acceptance/dc/peers/delete-test.js) |
| ~~`dc/peers/index.feature`~~ | *(deleted — migrated above)* | 5 | ✅ Migrated | [index-test.js](tests/acceptance/dc/peers/index-test.js) |
| ~~`dc/peers/regenerate.feature`~~ | *(deleted — migrated above)* | 2 | ✅ Migrated | [regenerate-test.js](tests/acceptance/dc/peers/regenerate-test.js) |
| ~~`dc/peers/show.feature`~~ | *(deleted — migrated above)* | 8 | ✅ Migrated | [show-test.js](tests/acceptance/dc/peers/show-test.js) |
| ~~`dc/peers/sorting.feature`~~ | *(deleted — migrated above)* | 1 | ✅ Migrated | [sorting-test.js](tests/acceptance/dc/peers/sorting-test.js) |
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
