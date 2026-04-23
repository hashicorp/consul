/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Cleanup Script: Delete all E2E test tokens
 *
 * This script finds and deletes all tokens with "E2E" in their description.
 * Run with: node ui/packages/consul-ui/e2e-tests/cleanup-e2e-tokens.js
 */

const http = require('http');

const CONSUL_TOKEN = process.env.CONSUL_UI_TEST_TOKEN;
const CONSUL_HOST = 'localhost';
const CONSUL_PORT = 8500; // dc1
const DATACENTER = 'dc1';

function makeRequest(method, path) {
  return new Promise((resolve, reject) => {
    const options = {
      hostname: CONSUL_HOST,
      port: CONSUL_PORT,
      path: path,
      method: method,
      headers: {
        'X-Consul-Token': CONSUL_TOKEN,
        'Content-Type': 'application/json',
      },
    };

    const req = http.request(options, (res) => {
      let data = '';

      res.on('data', (chunk) => {
        data += chunk;
      });

      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          try {
            resolve({ status: res.statusCode, data: data ? JSON.parse(data) : null });
          } catch (e) {
            resolve({ status: res.statusCode, data: data });
          }
        } else {
          reject(new Error(`HTTP ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', (error) => {
      reject(error);
    });

    req.end();
  });
}

async function listTokens() {
  try {
    const response = await makeRequest('GET', `/v1/acl/tokens?dc=${DATACENTER}`);
    return response.data || [];
  } catch (error) {
    console.error('❌ Failed to list tokens:', error.message);
    return [];
  }
}

async function deleteToken(tokenId, description) {
  try {
    await makeRequest('DELETE', `/v1/acl/token/${tokenId}?dc=${DATACENTER}`);
    console.log(`  ✓ Deleted: ${description} (${tokenId})`);
    return true;
  } catch (error) {
    if (error.message.includes('404')) {
      console.log(`  ✓ Already deleted: ${description} (${tokenId})`);
      return true;
    }
    console.error(`  ✗ Failed to delete: ${description} (${tokenId}) - ${error.message}`);
    return false;
  }
}

async function cleanupE2ETokens() {
  console.log('\n🔍 Searching for E2E test tokens...\n');

  const tokens = await listTokens();

  if (!Array.isArray(tokens)) {
    console.error('❌ Failed to retrieve tokens list');
    return;
  }

  console.log(`📋 Found ${tokens.length} total tokens\n`);

  const e2eTokens = tokens.filter((token) => {
    const description = token.Description || '';
    return description.toLowerCase().includes('e2e');
  });

  if (e2eTokens.length === 0) {
    console.log('✅ No E2E tokens found. Nothing to clean up!\n');
    return;
  }

  console.log(`🧹 Found ${e2eTokens.length} E2E tokens to delete:\n`);

  let successCount = 0;
  let failCount = 0;

  for (const token of e2eTokens) {
    const success = await deleteToken(token.AccessorID, token.Description || 'No description');
    if (success) {
      successCount++;
    } else {
      failCount++;
    }
  }

  console.log('\n' + '='.repeat(50));
  console.log(`✅ Successfully deleted: ${successCount}`);
  if (failCount > 0) {
    console.log(`❌ Failed to delete: ${failCount}`);
  }
  console.log('='.repeat(50) + '\n');
}

// Run the cleanup
cleanupE2ETokens().catch((error) => {
  console.error('❌ Cleanup script failed:', error);
  process.exit(1);
});

// Made with Bob
