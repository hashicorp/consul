const test = require('tape');

const getEnvironment = require('../../config/environment.js');

test(
  'config has the correct environment settings',
  function(t) {
    [
      {
        environment: 'production',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: '{{.ACLsEnabled}}',
        CONSUL_NSPACES_ENABLED: '{{ if .NamespacesEnabled }}{{.NamespacesEnabled}}{{ else }}false{{ end }}',
      },
      {
        environment: 'test',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: true,
      },
      {
        environment: 'staging',
        CONSUL_BINARY_TYPE: 'oss',
        CONSUL_ACLS_ENABLED: true,
        CONSUL_NSPACES_ENABLED: true,
      }
    ].forEach(
      function(item) {
        const env = getEnvironment(item.environment);
        Object.keys(item).forEach(
          function(key) {
            t.equal(env[key], item[key], `Expect ${key} to equal ${item[key]} in the ${item.environment} environment`);
          }
        );
      }
    );
    t.end();
  }
);
