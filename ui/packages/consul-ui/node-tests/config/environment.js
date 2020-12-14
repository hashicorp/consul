/* eslint-env node */

const test = require('tape');

const getEnvironment = require('../../config/environment.js');

test(
  'config has the correct environment settings',
  function(t) {
    [
      {
        environment: 'production',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: '__RUNTIME_BOOL_ACLsEnabled__',
        CONSUL_SSO_ENABLED: '__RUNTIME_BOOL_SSOEnabled__',
        CONSUL_NSPACES_ENABLED: '__RUNTIME_BOOL_NamespacesEnabled__',
      },
      {
        environment: 'test',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: false,
        CONSUL_SSO_ENABLED: false,
      },
      {
        $: {
          CONSUL_NSPACES_ENABLED: 1
        },
        environment: 'test',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: true,
        CONSUL_SSO_ENABLED: false,
      },
      {
        $: {
          CONSUL_SSO_ENABLED: 1
        },
        environment: 'test',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: false,
        CONSUL_SSO_ENABLED: true,
      },
      {
        environment: 'staging',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: true,
        CONSUL_SSO_ENABLED: true,
      }
    ].forEach(
      function(item) {
        const env = getEnvironment(item.environment, typeof item.$ !== 'undefined' ? item.$ : undefined);
        Object.keys(item).forEach(
          function(key) {
            if(key === '$') {
              return;
            }
            t.equal(
              env[key],
              item[key],
              `Expect ${key} to equal ${item[key]} in the ${item.environment} environment ${typeof item.$ !== 'undefined' ? `(with ${JSON.stringify(item.$)})` : ''}`
            );
          }
        );
      }
    );
    t.end();
  }
);
