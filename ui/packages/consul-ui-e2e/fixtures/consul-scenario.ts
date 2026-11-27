/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test as base, type Page } from '@playwright/test';
import { ConsulClient } from '../lib/consul-client.ts';
import {
  agentBinaryProblem,
  checkServedUi,
  loadMeta,
  runtimeFor,
  seedScenario,
  startAgent,
  stopAgent,
  type Edition,
  type Runtime,
} from '../lib/scenario.ts';

type WorkerFixtures = {
  /** The scenario this spec file needs. Set with `test.use({ scenario: '<name>' })`. */
  scenario: string;
  scenarioAgent: void;
};

/**
 * Playwright's `test`, extended so each spec file declares the scenario it
 * needs and gets a `consul` API client pointed at it:
 *
 *   test.use({ scenario: 'intentions' });
 *
 * The scenario is a worker-scoped option, so Playwright runs files that need
 * different scenarios in separate workers, one after another (`workers: 1`).
 * `scenarioAgent` starts and seeds the scenario when a worker starts and stops
 * it when the worker ends. A failed test also ends its worker, so the
 * remaining tests and any retry get a freshly seeded scenario.
 *
 * It also fails fast when the run's edition (`EDITION`, see
 * playwright.config.ts) doesn't fit: an Enterprise-only scenario in a CE run
 * (the test is missing an `@ent` tag), or an agent binary of the wrong edition.
 *
 * Import `test` and `expect` from this module in specs instead of
 * `@playwright/test`.
 */
export const test = base.extend<{ consul: ConsulClient }, WorkerFixtures>({
  scenario: ['', { scope: 'worker', option: true }],

  scenarioAgent: [
    async ({ scenario }, use, workerInfo) => {
      if (!scenario) {
        throw new Error(
          'This spec file doesn\'t say which scenario it needs. Add test.use({ scenario: \'<name>\' }) ' +
            'at the top of the file (see docs/adding-a-test.md).',
        );
      }
      const runEdition = workerInfo.project.metadata.edition as Edition;
      const meta = loadMeta(scenario);
      if (meta.requiresEnterprise && runEdition === 'ce') {
        throw new Error(
          `Scenario "${scenario}" needs Consul Enterprise, but this is a CE run. ` +
            'Tag the tests that use it @ent.',
        );
      }

      const runtime = runtimeFor(meta, (process.env.RUNTIME as Runtime) ?? 'docker');
      const uiFromAgent = process.env.UI_SOURCE === 'agent';
      const binaryProblem = uiFromAgent ? agentBinaryProblem(runtime) : undefined;
      if (binaryProblem) throw new Error(binaryProblem);

      const consul = new ConsulClient();
      await startAgent(scenario, runtime, consul);
      try {
        await consul.waitReady();
        const agent = await consul.agentEdition();
        if (agent.edition !== runEdition) {
          const label = (e: Edition) => (e === 'ent' ? 'an Enterprise' : 'a CE');
          throw new Error(
            `This is an EDITION=${runEdition} run, but the agent is ${label(agent.edition)} build ` +
              `(version ${agent.version}). Point CONSUL_BIN or CONSUL_IMAGE at ${label(runEdition)} build, ` +
              `or set EDITION=${agent.edition}.`,
          );
        }
        let ui = 'Ember dev server';
        if (uiFromAgent) {
          ui = await checkServedUi(workerInfo.project.use.baseURL ?? consul.addr);
        }
        console.log(
          `[scenario] "${scenario}" on Consul ${agent.version}` +
            `${agent.revision ? ` (${agent.revision.slice(0, 10)})` : ''}, UI: ${ui}`,
        );
        await seedScenario(scenario, consul);
        await use();
      } finally {
        if (process.env.KEEP_UP === '1') {
          console.log(
            `[scenario] KEEP_UP=1: leaving "${scenario}" running. The next run, or "pnpm sc down ${scenario}", stops it.`,
          );
        } else {
          await stopAgent(scenario, runtime, consul);
        }
      }
    },
    { scope: 'worker', auto: true, timeout: 180_000 },
  ],

  consul: async ({}, use) => {
    await use(new ConsulClient());
  },
});

export { expect } from '@playwright/test';

/**
 * Logs the browser into consul-ui as the given ACL SecretID — for
 * ACL-gated routes (`/ui/dc1/acls/*`): `acl:read`/`acl:write` are never
 * covered by `default_policy = "allow"` (same rule `lib/consul-client.ts`
 * documents for seeding), so an unauthenticated Playwright session gets a
 * 401 and the "please log in" view instead of the page under test.
 *
 * Writes the same `consul:token` localStorage value consul-ui's own login
 * flow would (`app/services/settings.js` — `SCHEME = 'consul'`) via a real
 * `/v1/acl/token/self` lookup, not a hand-rolled shape — consul-ui's
 * AuthDialog treats a token object's `AccessorID` presence as "already
 * logged in" on boot (`app/components/auth-dialog/index.js#hasToken`), so a
 * token missing it is treated as unauthenticated regardless of `SecretID`.
 *
 * Must run before the first `page.goto` — localStorage is read at app boot.
 */
export async function loginAs(page: Page, consul: ConsulClient, secretId: string): Promise<void> {
  const res = await fetch(`${consul.addr}/v1/acl/token/self`, { headers: { 'X-Consul-Token': secretId } });
  if (!res.ok) throw new Error(`token self lookup failed for login: ${res.status} ${await res.text()}`);
  const token = await res.json();
  await page.addInitScript((t) => window.localStorage.setItem('consul:token', JSON.stringify(t)), token);
}
