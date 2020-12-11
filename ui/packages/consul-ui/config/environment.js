'use strict';
const path = require('path');
const utils = require('./utils');

const repositoryRoot = path.resolve(__dirname, '../../../../');

const repositoryYear = utils.repositoryYear;
const repositorySHA = utils.repositorySHA;
const binaryVersion = utils.binaryVersion(repositoryRoot);

module.exports = function(environment, $ = process.env) {
  // basic 'get env var with fallback' accessor
  const env = function(flag, fallback) {
    // a fallback value MUST be set
    if (typeof fallback === 'undefined') {
      throw new Error(`Please provide a fallback value for $${flag}`);
    }
    // return the env var if set
    if (typeof $[flag] !== 'undefined') {
      if (typeof fallback === 'boolean') {
        // if we are expecting a boolean JSON parse strings to numbers/booleans
        return !!JSON.parse($[flag]);
      }
      return $[flag];
    }
    // If the fallback is a function call it and return the result.
    // Lazily calling the function means binaries used for fallback don't need
    // to be available if we are sure the environment variables will be set
    if (typeof fallback === 'function') {
      return fallback();
    }
    // just return the fallback value
    return fallback;
  };

  let ENV = {
    modulePrefix: 'consul-ui',
    environment,
    rootURL: '/ui/',
    locationType: 'auto',

    // We use a complete dynamically (from Consul) configured torii provider.
    // We provide this object here to prevent ember from giving a log message
    // when starting ember up
    torii: {},

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

  // The following 'environment variables' are set at build-time and compiled
  // into a meta tag in generated index.html file.
  // They can be accessed in the UI by using either:
  //
  // 1. The 'env' service from within javascript: `@service('env') env;` (../app/services/env.js)
  // 2. The 'env' helper from within hbs: `{{env 'VARIABLE_NAME'}}` (../app/helpers/env.js)
  //
  // These variables can be overwritten depending on certain environments.
  // For example for a production release the binary will overwrite some
  // variables at runtime, during development some variables can be
  // overwritten by adding cookie values using the browsers' Web Inspector

  // TODO: These should probably go onto APP
  ENV = Object.assign({}, ENV, {
    // The following variables are compile-time variables that are set during
    // the consul build process and baked into the generated assetsfs file that
    // is later added to the consul binary itself. Some values, if not set,
    // will automatically pull information from the git repository which means
    // these values are guaranteed to be set/correct during development.
    CONSUL_COPYRIGHT_YEAR: env('CONSUL_COPYRIGHT_YEAR', repositoryYear),
    CONSUL_GIT_SHA: env('CONSUL_GIT_SHA', repositorySHA),
    CONSUL_VERSION: env('CONSUL_VERSION', binaryVersion),
    CONSUL_BINARY_TYPE: env('CONSUL_BINARY_TYPE', 'oss'),

    // These can be overwritten by the UI user at runtime by setting localStorage values
    CONSUL_UI_DISABLE_REALTIME: env('CONSUL_UI_DISABLE_REALTIME', false),
    CONSUL_UI_DISABLE_ANCHOR_SELECTION: env('CONSUL_UI_DISABLE_ANCHOR_SELECTION', false),

    // The following variables are runtime variables that are overwritten when
    // the go binary services the index.html page
    CONSUL_ACLS_ENABLED: false,
    CONSUL_NSPACES_ENABLED: false,
    CONSUL_SSO_ENABLED: false,
    CONSUL_DATACENTER_LOCAL: env('CONSUL_DATACENTER_LOCAL', 'dc1'),

    // Static variables used in multiple places throughout the UI
    CONSUL_HOME_URL: 'https://www.consul.io',
    CONSUL_REPO_ISSUES_URL: 'https://github.com/hashicorp/consul/issues/new/choose',
    CONSUL_DOCS_URL: 'https://www.consul.io/docs',
    CONSUL_DOCS_LEARN_URL: 'https://learn.hashicorp.com',
    CONSUL_DOCS_API_URL: 'https://www.consul.io/api',
    CONSUL_COPYRIGHT_URL: 'https://www.hashicorp.com',
  });
  switch (true) {
    case environment === 'test':
      ENV = Object.assign({}, ENV, {
        locationType: 'none',

        // During testing ACLs default to being turned on
        CONSUL_ACLS_ENABLED: env('CONSUL_ACLS_ENABLED', true),
        CONSUL_NSPACES_ENABLED: env('CONSUL_NSPACES_ENABLED', false),
        CONSUL_SSO_ENABLED: env('CONSUL_SSO_ENABLED', false),

        '@hashicorp/ember-cli-api-double': {
          'auto-import': false,
          enabled: true,
          endpoints: {
            '/v1': '/mock-api/v1',
          },
        },
        APP: Object.assign({}, ENV.APP, {
          LOG_ACTIVE_GENERATION: false,
          LOG_VIEW_LOOKUPS: false,

          // LOG_RESOLVER: true,
          // LOG_ACTIVE_GENERATION: true,
          // LOG_TRANSITIONS: true,
          // LOG_TRANSITIONS_INTERNAL: true,

          rootElement: '#ember-testing',
          autoboot: false,
        }),
      });
      break;
    case environment === 'staging':
      ENV = Object.assign({}, ENV, {
        // On staging sites everything defaults to being turned on by
        // different staging sites can be built with certain features disabled
        // by setting an environment variable to 0 during building (e.g.
        // CONSUL_NSPACES_ENABLED=0 make build)
        CONSUL_ACLS_ENABLED: env('CONSUL_ACLS_ENABLED', true),
        CONSUL_NSPACES_ENABLED: env('CONSUL_NSPACES_ENABLED', true),
        CONSUL_SSO_ENABLED: env('CONSUL_SSO_ENABLED', true),

        '@hashicorp/ember-cli-api-double': {
          enabled: true,
          endpoints: {
            '/v1': '/mock-api/v1',
          },
        },
      });
      break;
    case environment === 'production':
      ENV = Object.assign({}, ENV, {
        // These values are placeholders that are replaced when Consul renders
        // the index.html based on runtime config. They can't use Go template
        // syntax since this object ends up JSON and URLencoded in an HTML meta
        // tag which obscured the Go template tag syntax.
        //
        // __RUNTIME_BOOL_Xxxx__ will be replaced with either "true" or "false"
        // depending on whether the named variable is true or false in the data
        // returned from `uiTemplateDataFromConfig`.
        //
        // __RUNTIME_STRING_Xxxx__ will be replaced with the literal string in
        // the named variable in the data returned from
        // `uiTemplateDataFromConfig`. It may be empty.
        CONSUL_ACLS_ENABLED: '__RUNTIME_BOOL_ACLsEnabled__',
        CONSUL_SSO_ENABLED: '__RUNTIME_BOOL_SSOEnabled__',
        CONSUL_NSPACES_ENABLED: '__RUNTIME_BOOL_NamespacesEnabled__',
        CONSUL_DATACENTER_LOCAL: '__RUNTIME_STRING_LocalDatacenter__',
      });
      break;
  }
  return ENV;
};
