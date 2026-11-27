# Adding a test

Tests are Playwright specs that drive the Consul UI against a seeded
scenario. Each spec file says which scenario it needs; folders under `tests/`
are free to group specs by feature.

## Recipe

1. **Pick / create the scenario** that seeds the data your test needs
   (see [adding-a-scenario.md](adding-a-scenario.md)).

2. **Create `tests/<folder>/<thing>.spec.ts`**, import the fixture (not
   `@playwright/test` directly), and declare the scenario at the top level of
   the file:

   ```ts
   import { test, expect } from '../../fixtures/consul-scenario.ts';

   test.use({ scenario: '<scenario>' });

   test('renders X', async ({ page, consul }) => {
     await page.goto('/ui/dc1/...');
     await expect(page.locator('[data-test-...]')).toBeVisible();
   });
   ```

   The fixture starts and seeds that scenario before the file's tests and
   stops it afterwards. The `consul` fixture is a
   [`ConsulClient`](../lib/consul-client.ts) pointed at the running agent — use
   it to assert catalog state or mutate data mid-test.

3. **Tag the edition** if the test doesn't apply to both CE and Enterprise:

   | Tag | Runs on |
   |---|---|
   | none | CE and Enterprise |
   | `@ent` | Enterprise only (namespaces, partitions, SSO, …) |
   | `@ce` | CE only (behaviour that differs on Enterprise) |

   ```ts
   test.describe('Admin partitions', { tag: '@ent' }, () => { ... });
   test('no Namespace column', { tag: '@ce' }, async ({ page }) => { ... });
   ```

   A test that uses a scenario with `"requiresEnterprise": true` must be tagged
   `@ent`; a CE run fails it otherwise.

4. **Run it:**

   ```bash
   pnpm e2e tests/<folder>/<thing>.spec.ts                 # CE (default)
   EDITION=ent pnpm e2e tests/<folder>/<thing>.spec.ts     # Enterprise binary
   KEEP_UP=1 pnpm e2e:headed tests/<folder>/<thing>.spec.ts
   ```

## Selector rules

- **Prefer `data-test-*` attributes.** consul-ui uses them throughout
  (`[data-test-node]`, `[data-test-node-address]`, `[data-test-node-services]`,
  `[data-test-status]`, `[data-test-session-name]`, `[data-test-session-behavior]`,
  `[data-test-delete]`, …). They are stable across the HDS visual migration, so
  specs don't churn when styling changes.
- For tabs / buttons without a hook, use `getByRole('tab', { name })` /
  `getByRole('button', { name })` — role + accessible name, not CSS classes.
- **Never** assert on HDS class names or DOM structure — those are exactly what
  the migration changes.

## Conventions

- One scenario per spec file (`test.use({ scenario })` must be top-level).
  Split a file if its tests need different data.
- Any number of files can share a scenario; neighbouring files reuse one run of
  it. A failing test restarts it, so later tests and retries get fresh data.
- Assert on **seeded, deterministic** values (the scenario controls them).
- Keep specs independent — each `goto`s its own route; don't rely on order.
- Clean up data a test creates in `afterEach`, not at the end of the test.
- `KEEP_UP=1` leaves the last scenario running after a run so you can inspect
  the UI. The next run (or `pnpm sc down <scenario>`) stops it.
