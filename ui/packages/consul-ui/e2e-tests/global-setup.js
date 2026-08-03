/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { chromium } = require('@playwright/test');
const { checkAllServices, printServiceErrors } = require('./utils/health-check-utils');
const { loginWithToken } = require('./utils/auth-utils');

async function globalSetup(config) {
  console.log('\n🚀 Starting E2E Test Setup...\n');

  // Get baseURL from config, which already handles CI vs local environment
  const baseURL =
    config.projects?.[0]?.use?.baseURL ||
    config.use?.baseURL ||
    (process.env.CI ? 'http://localhost:8500' : 'http://localhost:4200');

  console.log(`📍 Using baseURL: ${baseURL}`);
  console.log(`🌍 Environment: ${process.env.CI ? 'CI (port 8500)' : 'Local (port 4200)'}\n`);

  console.log('🔍 Checking service health...\n');

  const healthChecks = await checkAllServices(baseURL);

  let allHealthy = true;
  const failedServices = [];

  healthChecks.forEach((s) => {
    console.log(`${s.isHealthy ? '✅' : '❌'} ${s.name}: ${s.url}`);
    if (!s.isHealthy) {
      allHealthy = false;
      failedServices.push(s);
    }
  });

  if (!allHealthy) {
    console.log('\n⚠️  Some services are not accessible. Tests may fail.\n');
    printServiceErrors(failedServices);
  }

  // Perform authentication and save state
  console.log('\n🔐 Authenticating to Consul UI...\n');

  const browser = await chromium.launch();
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // Login using the token from environment, passing baseURL
    const authResult = await loginWithToken(page, process.env.CONSUL_UI_TEST_TOKEN, baseURL);
    if (authResult?.authenticated) {
      console.log('✅ Authentication successful via UI login.\n');
    } else {
      console.log('⚠️  UI login unavailable (ACLs disabled in dev build). Setting token via localStorage.\n');

      // Fallback: set the token directly in localStorage when the UI login flow
      // is not available (e.g. ember serve where operatorConfig.ACLsEnabled defaults to false).
      // Resolve the full token shape from the API so the UI has AccessorID, Namespace, etc.
      const secretID = process.env.CONSUL_UI_TEST_TOKEN;
      if (secretID) {
        let tokenPayload = { SecretID: secretID };
        try {
          const apiBase = baseURL.replace(':4200', ':8500'); // proxy → real Consul API
          const resp = await page.request.get(`${apiBase}/v1/acl/token/self`, {
            headers: { 'X-Consul-Token': secretID },
          });
          if (resp.ok()) {
            const data = await resp.json();
            tokenPayload = {
              AccessorID: data.AccessorID,
              SecretID: data.SecretID,
              Namespace: data.Namespace || 'default',
              Partition: data.Partition || 'default',
            };
          }
        } catch (_) {
          // If API lookup fails, fall back to minimal shape
        }

        await page.goto(`${baseURL}/ui/dc1/kv`, { waitUntil: 'domcontentloaded' });
        await page.evaluate((payload) => {
          localStorage.setItem('consul:token', JSON.stringify(payload));
        }, tokenPayload);
        console.log(`✅ Token set via localStorage fallback (AccessorID: ${tokenPayload.AccessorID || 'unknown'}).\n`);
      }
    }

    // Save the authenticated state for all tests to reuse
    await context.storageState({ path: 'e2e-tests/auth-state.json' });

    console.log('💾 Saved authentication state.\n');
  } catch (error) {
    console.error('❌ Authentication failed:', error.message);
    throw error;
  } finally {
    await browser.close();
  }

  console.log('✅ Setup complete!\n');
}

module.exports = globalSetup;
