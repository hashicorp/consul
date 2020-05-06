'use strict';
const fs = require('fs');
const path = require('path');
module.exports = function(environment, $ = process.env) {
  let ENV = {
    modulePrefix: 'consul-ui',
    environment,
    rootURL: '/ui/',
    locationType: 'auto',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. EMBER_NATIVE_DECORATOR_SUPPORT: true
      },
      EXTEND_PROTOTYPES: {
        // Prevent Ember Data from overriding Date.parse.
        Date: false,
      },
    },
    APP: {
      // Here you can pass flags/options to your application instance
      // when it is created
    },
    resizeServiceDefaults: {
      injectionFactories: ['view', 'controller', 'component'],
    },
  };
  // TODO: These should probably go onto APP
  ENV = Object.assign({}, ENV, {
    CONSUL_UI_DISABLE_REALTIME: typeof process.env.CONSUL_UI_DISABLE_REALTIME !== 'undefined',
    CONSUL_UI_DISABLE_ANCHOR_SELECTION:
      typeof process.env.CONSUL_UI_DISABLE_ANCHOR_SELECTION !== 'undefined',
    CONSUL_COPYRIGHT_YEAR: (function(val) {
      if (val) {
        return val;
      }
      return require('child_process')
        .execSync('git show -s --format=%ci HEAD')
        .toString()
        .trim()
        .split('-')
        .shift();
    })(process.env.CONSUL_COPYRIGHT_YEAR),
    CONSUL_GIT_SHA: (function(val) {
      if (val) {
        return val;
      }

      return require('child_process')
        .execSync('git rev-parse --short HEAD')
        .toString()
        .trim();
    })(process.env.CONSUL_GIT_SHA),
    CONSUL_VERSION: (function(val) {
      if (val) {
        return val;
      }
      // see /scripts/dist.sh:8
      const version_go = `${path.dirname(path.dirname(__dirname))}/version/version.go`;
      const contents = fs.readFileSync(version_go).toString();
      return contents
        .split('\n')
        .find(function(item, i, arr) {
          return item.indexOf('Version =') !== -1;
        })
        .trim()
        .split('"')[1];
    })(process.env.CONSUL_VERSION),
    CONSUL_BINARY_TYPE: process.env.CONSUL_BINARY_TYPE ? process.env.CONSUL_BINARY_TYPE : 'oss',
    CONSUL_ACLS_ENABLED: false,
    CONSUL_NSPACES_ENABLED: false,
    CONSUL_SSO_ENABLED: false,

    CONSUL_HOME_URL: 'https://www.consul.io',
    CONSUL_REPO_ISSUES_URL: 'https://github.com/hashicorp/consul/issues/new/choose',
    CONSUL_DOCS_URL: 'https://www.consul.io/docs',
    CONSUL_DOCS_LEARN_URL: 'https://learn.hashicorp.com',
    CONSUL_DOCS_API_URL: 'https://www.consul.io/api',
    CONSUL_COPYRIGHT_URL: 'https://www.hashicorp.com',
  });
  const isTestLike = ['staging', 'test'].indexOf(environment) > -1;
  const isDevLike = ['development', 'staging', 'test'].indexOf(environment) > -1;
  const isProdLike = ['production', 'staging'].indexOf(environment) > -1;
  switch (true) {
    case environment === 'test':
      ENV = Object.assign({}, ENV, {
        locationType: 'none',
        CONSUL_NSPACES_TEST: true,
        CONSUL_NSPACES_ENABLED:
          typeof $['CONSUL_NSPACES_ENABLED'] !== 'undefined'
            ? !!JSON.parse(String($['CONSUL_NSPACES_ENABLED']).toLowerCase())
            : true,
        CONSUL_SSO_ENABLED:
          typeof $['CONSUL_SSO_ENABLED'] !== 'undefined'
            ? !!JSON.parse(String($['CONSUL_SSO_ENABLED']).toLowerCase())
            : false,
        '@hashicorp/ember-cli-api-double': {
          'auto-import': false,
          enabled: true,
          endpoints: {
            '/v1': '/node_modules/@hashicorp/consul-api-double/v1',
          },
        },
        APP: Object.assign({}, ENV.APP, {
          LOG_ACTIVE_GENERATION: false,
          LOG_VIEW_LOOKUPS: false,

          rootElement: '#ember-testing',
          autoboot: false,
        }),
      });
      break;
    case environment === 'staging':
      ENV = Object.assign({}, ENV, {
        CONSUL_NSPACES_ENABLED: true,
        CONSUL_SSO_ENABLED: true,
        '@hashicorp/ember-cli-api-double': {
          enabled: true,
          endpoints: {
            '/v1': '/node_modules/@hashicorp/consul-api-double/v1',
          },
        },
      });
      break;
    case environment === 'production':
      ENV = Object.assign({}, ENV, {
        CONSUL_ACLS_ENABLED: '{{.ACLsEnabled}}',
        CONSUL_SSO_ENABLED: '{{.SSOEnabled}}',
        CONSUL_NSPACES_ENABLED:
          '{{ if .NamespacesEnabled }}{{.NamespacesEnabled}}{{ else }}false{{ end }}',
      });
      break;
  }
  switch (true) {
    case isTestLike:
      ENV = Object.assign({}, ENV, {
        CONSUL_ACLS_ENABLED: true,
        // 'APP': Object.assign({}, ENV.APP, {
        //   'LOG_RESOLVER': true,
        //   'LOG_ACTIVE_GENERATION': true,
        //   'LOG_TRANSITIONS': true,
        //   'LOG_TRANSITIONS_INTERNAL': true,
        //   'LOG_VIEW_LOOKUPS': true,
        // })
      });
      break;
    case isProdLike:
      break;
  }
  return ENV;
};
