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

function getServices(baseURL = 'http://localhost:4200') {
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
    { name: 'Consul UI', url: baseURL, required: true },
  ];
}

/**
 * Check all services and return health status
 * @param {string} [baseURL='http://localhost:4200'] - Base URL for the UI
 * @returns {Promise<Array>} - Array of services with health status
 */
async function checkAllServices(baseURL = 'http://localhost:4200') {
  const services = getServices(baseURL);
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
  const hasUIFailure = failedServices.some((s) => s.url.includes(':4200'));

  if (hasConsulAPIFailure) {
    console.log('📍 Consul API servers (8500/8501) not running:');
    console.log('   → Start servers in consul-ui-testing repo');
    console.log('   → Run: yarn start hashicorppreview/consul-enterprise:<VERSION> --quiet');
    console.log('   → Example: yarn start hashicorppreview/consul-enterprise:1.22 --quiet\n');
  }

  if (hasUIFailure) {
    console.log('📍 Consul UI (4200) not running:');
    console.log('   → Start UI in consul repo');
    console.log('   → Run: pnpm run start:consul\n');
  }
}

module.exports = { checkServiceHealth, getServices, checkAllServices, printServiceErrors };
