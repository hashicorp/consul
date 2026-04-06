#!/usr/bin/env node
const { checkAllServices, printServiceErrors } = require('./utils/health-check-utils');

async function runHealthCheck() {
  console.log('\n🏥 E2E Environment Health Check\n');

  const healthChecks = await checkAllServices();

  let allRequiredHealthy = true;
  const failedServices = [];

  healthChecks.forEach((s) => {
    const status = s.isHealthy ? '✅' : '❌';
    const label = s.required ? 'REQUIRED' : 'OPTIONAL';
    console.log(`${status} [${label}] ${s.name}: ${s.url}`);
    if (s.required && !s.isHealthy) {
      allRequiredHealthy = false;
      failedServices.push(s);
    }
  });

  if (allRequiredHealthy) {
    console.log('\n✅ Ready! Run: pnpm run test:e2e:basic\n');
    process.exit(0);
  } else {
    console.log('\n❌ Missing required services!\n');
    printServiceErrors(failedServices);
    process.exit(1);
  }
}

runHealthCheck().catch((err) => {
  console.error('❌ Error:', err.message);
  process.exit(1);
});
