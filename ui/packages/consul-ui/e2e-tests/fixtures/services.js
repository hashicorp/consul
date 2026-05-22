/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const base = require('@playwright/test');
const { loginWithToken } = require('../utils/auth-utils');

exports.test = base.test.extend({
  servicesPage: async ({ page }, use) => {
    await loginWithToken(page);

    const ui = {
      async gotoList() {
        await page.goto('/ui/dc1/services');
      },
      async gotoService(serviceName) {
        await page.goto(`/ui/dc1/services/${serviceName}`);
      },
      async navigateToService(serviceName) {
        await page.getByRole('link', { name: serviceName, exact: true }).first().click();
      },
      async clickTab(tabName) {
        await page.getByRole('link', { name: tabName, exact: true }).click();
      },
      async clickInstance(instanceText) {
        await page.getByRole('link', { name: instanceText }).first().click();
      },
    };

    await use({ page, ...ui });
  },

  servicesApi: async ({ page }, use) => {
    const token = process.env.CONSUL_UI_TEST_TOKEN;
    const getBaseURL = () => page.context()._options.baseURL || 'http://localhost:4200';
    const tracked = [];

    const api = {
      async register(serviceDef) {
        const response = await page.request.put(`${getBaseURL()}/v1/agent/service/register`, {
          headers: { 'X-Consul-Token': token, 'Content-Type': 'application/json' },
          data: serviceDef,
        });
        if (!response.ok()) {
          const body = await response.text().catch(() => '(no body)');
          throw new Error(
            `Failed to register ${
              serviceDef.Name
            }: HTTP ${response.status()} ${response.statusText()} — ${body}`
          );
        }
        const serviceID = serviceDef.ID || serviceDef.Name;
        tracked.push(serviceID);
        return serviceDef;
      },

      async deregister(serviceID) {
        try {
          await page.request.put(`${getBaseURL()}/v1/agent/service/deregister/${serviceID}`, {
            headers: { 'X-Consul-Token': token },
          });
        } catch (error) {
          console.warn(`Note: Could not deregister service ${serviceID} - ${error.message}`);
        }
      },
    };

    await use(api);

    for (const serviceID of tracked) {
      await api.deregister(serviceID);
    }
  },
});

exports.expect = base.expect;
