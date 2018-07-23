'use strict';
const fs = require('fs');
const path = require('path');
module.exports = function(environment) {
  let ENV = {
    modulePrefix: 'consul-ui',
    environment,
    rootURL: '/ui/',
    locationType: 'auto',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. 'with-controller': true
        'ds-improved-ajax': true,
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
  ENV = Object.assign({}, ENV, {
    CONSUL_GIT_SHA: (function() {
      if (process.env.CONSUL_GIT_SHA) {
        return process.env.CONSUL_GIT_SHA;
      }

      return require('child_process')
        .execSync('git rev-parse --short HEAD')
        .toString()
        .trim();
    })(),
    CONSUL_VERSION: (function() {
      if (process.env.CONSUL_VERSION) {
        return process.env.CONSUL_VERSION;
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
    })(),
    CONSUL_BINARY_TYPE: (function() {
      if (process.env.CONSUL_BINARY_TYPE) {
        return process.env.CONSUL_BINARY_TYPE;
      }
      return 'oss';
    })(),
    CONSUL_DOCUMENTATION_URL: 'https://www.consul.io/docs',
    CONSUL_COPYRIGHT_URL: 'https://www.hashicorp.com',
    CONSUL_COPYRIGHT_YEAR: '2018',
  });

  if (environment === 'development') {
    // ENV.APP.LOG_RESOLVER = true;
    // ENV.APP.LOG_ACTIVE_GENERATION = true;
    // ENV.APP.LOG_TRANSITIONS = true;
    // ENV.APP.LOG_TRANSITIONS_INTERNAL = true;
    // ENV.APP.LOG_VIEW_LOOKUPS = true;
    // ENV['ember-cli-mirage'] = {
    //   enabled: false,
    // };
  }

  if (environment === 'test') {
    // Testem prefers this...
    ENV.locationType = 'none';

    // keep test console output quieter
    ENV.APP.LOG_ACTIVE_GENERATION = false;
    ENV.APP.LOG_VIEW_LOOKUPS = false;

    ENV.APP.rootElement = '#ember-testing';
    ENV.APP.autoboot = false;
    ENV['ember-cli-api-double'] = {
      reader: 'html',
      endpoints: ['/node_modules/@hashicorp/consul-api-double/v1'],
    };
  }

  if (environment === 'production') {
    // here you can enable a production-specific feature
  }

  return ENV;
};
