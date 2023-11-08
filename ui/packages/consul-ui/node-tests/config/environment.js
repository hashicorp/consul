/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */

const test = require('tape');

const getEnvironment = require('../../config/environment.js');

test('config has the correct environment settings', function (t) {
  [
    {
      environment: 'production',
      CONSUL_BINARY_TYPE: 'oss',
      operatorConfig: {
        APIPrefix: '',
      },
    },
    {
      environment: 'test',
      CONSUL_BINARY_TYPE: 'oss',
      operatorConfig: {
        ACLsEnabled: true,
        NamespacesEnabled: false,
        SSOEnabled: false,
        PartitionsEnabled: false,
        PeeringEnabled: true,
        HCPEnabled: false,
        LocalDatacenter: 'dc1',
        PrimaryDatacenter: 'dc1',
        APIPrefix: '',
      },
    },
    {
      $: {
        CONSUL_NSPACES_ENABLED: 1,
      },
      environment: 'test',
      CONSUL_BINARY_TYPE: 'oss',
      operatorConfig: {
        ACLsEnabled: true,
        NamespacesEnabled: true,
        SSOEnabled: false,
        PartitionsEnabled: false,
        PeeringEnabled: true,
        HCPEnabled: false,
        LocalDatacenter: 'dc1',
        PrimaryDatacenter: 'dc1',
        APIPrefix: '',
      },
    },
    {
      $: {
        CONSUL_SSO_ENABLED: 1,
      },
      environment: 'test',
      CONSUL_BINARY_TYPE: 'oss',
      operatorConfig: {
        ACLsEnabled: true,
        NamespacesEnabled: false,
        SSOEnabled: true,
        PartitionsEnabled: false,
        PeeringEnabled: true,
        HCPEnabled: false,
        LocalDatacenter: 'dc1',
        PrimaryDatacenter: 'dc1',
        APIPrefix: '',
      },
    },
    {
      environment: 'staging',
      CONSUL_BINARY_TYPE: 'oss',
      operatorConfig: {
        ACLsEnabled: true,
        NamespacesEnabled: true,
        SSOEnabled: true,
        PartitionsEnabled: true,
        PeeringEnabled: true,
        HCPEnabled: false,
        LocalDatacenter: 'dc1',
        PrimaryDatacenter: 'dc1',
        APIPrefix: '',
      },
    },
  ].forEach(function (item) {
    const env = getEnvironment(
      item.environment,
      typeof item.$ !== 'undefined' ? item.$ : undefined
    );
    Object.keys(item).forEach(function (key) {
      if (key === '$') {
        return;
      }
      t.deepEqual(
        env[key],
        item[key],
        `Expect ${key} to equal ${item[key]} in the ${item.environment} environment ${
          typeof item.$ !== 'undefined' ? `(with ${JSON.stringify(item.$)})` : ''
        }`
      );
    });
  });
  t.end();
});
