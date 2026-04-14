/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { chromium } = require('@playwright/test');
const { checkAllServices, printServiceErrors } = require('./utils/health-check-utils');
const { loginWithToken } = require('./utils/auth-utils');

async function globalSetup(config) {
  console.log('\n🚀 Starting E2E Test Setup...\n');

  const baseURL = config.projects?.[0]?.use?.baseURL || 'http://localhost:8500';
  console.log(`📍 Using baseURL: ${baseURL}\n`);

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
      console.log('✅ Authentication successful.\n');
    } else {
      console.log('⚠️  Authentication skipped: ACL login is unavailable in this environment.\n');
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
