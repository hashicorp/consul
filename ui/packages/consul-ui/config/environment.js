// All of the configuration here is shared between buildtime and runtime and
// is therefore added to ember's <meta> tag in the actual app, if the
// configuration is for buildtime only you should probably just use
// ember-cli-build to prevent values being outputted in the meta tag
'use strict';
const path = require('path');
const utils = require('./utils');

const repositoryRoot = path.resolve(__dirname, '../../../../');

const repositoryYear = utils.repositoryYear;
const repositorySHA = utils.repositorySHA;
const binaryVersion = utils.binaryVersion(repositoryRoot);

module.exports = function (environment, $ = process.env) {
  // available environments
  // ['production', 'development', 'staging', 'test'];
  const env = utils.env($);
  // basic 'get env var with fallback' accessor

  let ENV = {
    modulePrefix: 'consul-ui',
    environment,
    rootURL: '/ui/',
    locationType: 'fsm-with-optional',
    historySupportMiddleware: true,

    torii: {
      disableRedirectInitializer: false,
    },

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
    // TODO(spatel): CE refactor
    CONSUL_BINARY_TYPE: env('CONSUL_BINARY_TYPE', 'oss'),

    // These can be overwritten by the UI user at runtime by setting localStorage values
    CONSUL_UI_DISABLE_REALTIME: env('CONSUL_UI_DISABLE_REALTIME', false),
    CONSUL_UI_DISABLE_ANCHOR_SELECTION: env('CONSUL_UI_DISABLE_ANCHOR_SELECTION', false),

    // The following variables are runtime variables that are overwritten when
    // the go binary serves the index.html page
    operatorConfig: {
      ACLsEnabled: false,
      NamespacesEnabled: false,
      SSOEnabled: false,
      PeeringEnabled: false,
      PartitionsEnabled: false,
      HCPEnabled: false,
      LocalDatacenter: env('CONSUL_DATACENTER_LOCAL', 'dc1'),
      PrimaryDatacenter: env('CONSUL_DATACENTER_PRIMARY', 'dc1'),
      APIPrefix: env('CONSUL_API_PREFIX', ''),
    },

    // Static variables used in multiple places throughout the UI
    CONSUL_HOME_URL: 'https://www.consul.io',
    CONSUL_REPO_ISSUES_URL: 'https://github.com/hashicorp/consul/issues/new/choose',
    CONSUL_DOCS_URL: 'https://www.consul.io/docs',
    CONSUL_DOCS_LEARN_URL: 'https://learn.hashicorp.com',
    CONSUL_DOCS_API_URL: 'https://www.consul.io/api',
    CONSUL_DOCS_DEVELOPER_URL: 'https://developer.hashicorp.com/consul/docs',
    CONSUL_COPYRIGHT_URL: 'https://www.hashicorp.com',
  });
  switch (true) {
    case environment === 'test':
      ENV = Object.assign({}, ENV, {
        locationType: 'fsm-with-optional-test',

        // During testing ACLs default to being turned on
        operatorConfig: {
          ACLsEnabled: env('CONSUL_ACLS_ENABLED', true),
          NamespacesEnabled: env('CONSUL_NSPACES_ENABLED', false),
          SSOEnabled: env('CONSUL_SSO_ENABLED', false),
          // in testing peering feature is on by default
          PeeringEnabled: env('CONSUL_PEERINGS_ENABLED', true),
          PartitionsEnabled: env('CONSUL_PARTITIONS_ENABLED', false),
          HCPEnabled: env('CONSUL_HCP_ENABLED', false),
          LocalDatacenter: env('CONSUL_DATACENTER_LOCAL', 'dc1'),
          PrimaryDatacenter: env('CONSUL_DATACENTER_PRIMARY', 'dc1'),
          APIPrefix: env('CONSUL_API_PREFIX', ''),
        },

        '@hashicorp/ember-cli-api-double': {
          'auto-import': false,
          enabled: true,
          endpoints: {
            '/v1': '/mock-api/v1',
            '/prefixed-api': '/mock-api/prefixed-api',
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
    case environment === 'development':
      ENV = Object.assign({}, ENV, {
        torii: {
          disableRedirectInitializer: true,
        },
      });
      break;
    case environment === 'staging':
      ENV = Object.assign({}, ENV, {
        torii: {
          disableRedirectInitializer: true,
        },
        // On staging sites everything defaults to being turned on by
        // different staging sites can be built with certain features disabled
        // by setting an environment variable to 0 during building (e.g.
        // CONSUL_NSPACES_ENABLED=0 make build)

        // TODO: We should be able to remove this now we can link to different
        // scenarios in staging
        operatorConfig: {
          ACLsEnabled: env('CONSUL_ACLS_ENABLED', true),
          NamespacesEnabled: env('CONSUL_NSPACES_ENABLED', true),
          SSOEnabled: env('CONSUL_SSO_ENABLED', true),
          PeeringEnabled: env('CONSUL_PEERINGS_ENABLED', true),
          PartitionsEnabled: env('CONSUL_PARTITIONS_ENABLED', true),
          HCPEnabled: env('CONSUL_HCP_ENABLED', false),
          LocalDatacenter: env('CONSUL_DATACENTER_LOCAL', 'dc1'),
          PrimaryDatacenter: env('CONSUL_DATACENTER_PRIMARY', 'dc1'),
          APIPrefix: env('CONSUL_API_PREFIX', ''),
        },

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
        // in production operatorConfig is populated at consul runtime from
        // operator configuration
        operatorConfig: {
          APIPrefix: '',
        },
      });
      break;
  }
  return ENV;
};
