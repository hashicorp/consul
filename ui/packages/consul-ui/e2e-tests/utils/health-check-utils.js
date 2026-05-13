/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const http = require('http');
const https = require('https');

/**
 * Check if a service is healthy by making an HTTP/HTTPS request
 * @param {string} url - The URL to check
 * @param {number} [timeout=5000] - Request timeout in milliseconds
 * @returns {Promise<boolean>} - True if service is healthy, false otherwise
 */
async function checkServiceHealth(url, timeout = 5000) {
  return new Promise((resolve) => {
    const client = new URL(url).protocol === 'https:' ? https : http;
    const req = client.get(url, { timeout }, (res) => resolve(!!res.statusCode));
    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
  });
}

function getServices(baseURL = 'http://localhost:8500') {
  return [
    {
      name: 'Consul HTTP API (8500)',
      url: 'http://localhost:8500/v1/status/leader',
      required: true,
    },
    {
      name: 'Consul HTTP API (8501)',
      url: 'http://localhost:8501/v1/status/leader',
      required: true,
    },
    { name: 'Consul UI', url: `${baseURL}/ui`, required: true },
  ];
}

/**
 * Check all services and return health status
 * @param {string} [baseURL] - Base URL for the UI (defaults to PLAYWRIGHT_BASE_URL or http://localhost:8500)
 * @returns {Promise<Array>} - Array of services with health status
 */
async function checkAllServices(baseURL) {
  // Use environment variable if available, otherwise default to 8500
  const defaultBaseURL =
    process.env.PLAYWRIGHT_BASE_URL || process.env.BASE_URL || 'http://localhost:8500';
  const effectiveBaseURL = baseURL || defaultBaseURL;

  const services = getServices(effectiveBaseURL);
  return await Promise.all(
    services.map(async (s) => ({ ...s, isHealthy: await checkServiceHealth(s.url) }))
  );
}

/**
 * Print helpful error messages based on which services failed
 * @param {Array} failedServices - Array of failed service objects
 */
function printServiceErrors(failedServices) {
  const hasConsulAPIFailure = failedServices.some(
    (s) => s.url.includes(':8500') || s.url.includes(':8501')
  );
  const hasUIFailure = failedServices.some((s) => s.name === 'Consul UI');

  if (hasConsulAPIFailure) {
    console.log('📍 Consul API servers (8500/8501) not running:');
    console.log('   → Start servers in consul-ui-testing repo');
    console.log('   → Run: yarn start consul:local');
    console.log('   → Or: yarn start hashicorppreview/consul-enterprise:<VERSION>\n');
  }

  if (hasUIFailure) {
    console.log('📍 Consul UI not accessible:');
    console.log('   → For local dev: Start UI on port 4200 with: pnpm run start:consul');
    console.log('   → For CI: Consul UI should be on port 8500 (built-in UI)\n');
  }
}

module.exports = { checkServiceHealth, getServices, checkAllServices, printServiceErrors };
