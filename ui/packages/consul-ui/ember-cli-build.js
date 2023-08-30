/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/*eslint ember/no-jquery: "off", ember/no-global-jquery: "off"*/
'use strict';
const path = require('path');
const exists = require('fs').existsSync;

const Funnel = require('broccoli-funnel');
const mergeTrees = require('broccoli-merge-trees');
const EmberApp = require('ember-cli/lib/broccoli/ember-app');
const utils = require('./config/utils');

// const BroccoliDebug = require('broccoli-debug');
// const debug = BroccoliDebug.buildDebugCallback(`app:consul-ui`)

module.exports = function (defaults, $ = process.env) {
  // available environments
  // ['production', 'development', 'staging', 'test'];

  $ = utils.env($);
  const env = EmberApp.env();
  const isProd = ['production'].includes(env);
  const prodlike = ['production', 'staging'];
  const devlike = ['development', 'staging'];
  const sourcemaps = !['production'].includes(env) && !$('BABEL_DISABLE_SOURCEMAPS', false);

  const trees = {};
  const addons = {};
  const outputPaths = {};
  let excludeFiles = [];

  const apps = [
    'consul-ui',
    'consul-acls',
    'consul-lock-sessions',
    'consul-peerings',
    'consul-partitions',
    'consul-nspaces',
    'consul-hcp',
  ].map((item) => {
    return {
      name: item,
      path: path.dirname(require.resolve(`${item}/package.json`)),
    };
  });

  const babel = {
    plugins: ['@babel/plugin-proposal-object-rest-spread'],
    sourceMaps: sourcemaps ? 'inline' : false,
  };

  // setup up different build configuration depending on environment
  if (!['test'].includes(env)) {
    // exclude any component/pageobject.js files from anything but test
    excludeFiles = excludeFiles.concat([
      'components/**/pageobject.js',
      'components/**/test-support.js',
      'components/**/*.test-support.js',
      'components/**/*.test.js',
    ]);
  }

  if (['test', 'production'].includes(env)) {
    // exclude our debug initializer, route and template
    excludeFiles = excludeFiles.concat([
      'instance-initializers/debug.js',
      'routing/**/*-debug.js',
      'helpers/**/*-debug.js',
      'modifiers/**/*-debug.js',
      'services/**/*-debug.js',
      'templates/debug.hbs',
      'components/debug/**/*.*',
    ]);
    // inspect *-debug configuration files for files to exclude
    excludeFiles = apps.reduce((prev, item) => {
      return ['services', 'routes'].reduce((prev, type) => {
        const path = `${item.path}/vendor/${item.name}/${type}-debug.js`;
        if (exists(path)) {
          return Object.entries(JSON.parse(require(path)[type])).reduce(
            (prev, [key, definition]) => {
              if (typeof definition.class !== 'undefined') {
                return prev.concat(`${definition.class.replace(`${item.name}/`, '')}.js`);
              }
              return prev;
            },
            prev
          );
        }
        return prev;
      }, prev);
    }, excludeFiles);
    // exclude any debug like addons from production or test environments
    addons.blacklist = [
      // exclude docfy
      '@docfy/ember',
    ];
  }

  if (['production'].includes(env)) {
    // everything apart from production is 'debug', including test
    // which means this and everything it affects is never tested
    babel.plugins.push(['strip-function-call', { strip: ['Ember.runInDebug'] }]);
  }

  //
  (function (apps) {
    trees.app = mergeTrees(
      [new Funnel('app', { exclude: excludeFiles })].concat(
        apps
          .filter((item) => exists(`${item.path}/app`))
          .map((item) => new Funnel(`${item.path}/app`, { exclude: excludeFiles }))
      ),
      {
        overwrite: true,
      }
    );
    // we switched to postcss - because ember-cli-postcss only operates on the
    // styles tree we need to make sure we write the css files from "sub-apps"
    // into `app/styles` manually and prefix them with `consul-ui` because that
    // is what the codebase expects from before when using ember-cli-sass.
    trees.styles = mergeTrees(
      [
        new Funnel('app/styles', { include: ['**/*.{scss,css}'] }),
        new Funnel('app', { include: ['components/**/*.{scss,css}'], destDir: 'consul-ui' }),
      ].concat(
        apps
          .filter((item) => exists(`${item.path}/app`))
          .map(
            (item) =>
              new Funnel(`${item.path}/app`, {
                include: ['**/*.{scss,css}'],
                destDir: 'consul-ui',
              })
          )
      ),
      {
        overwrite: true,
      }
    );
    trees.vendor = mergeTrees(
      [new Funnel('vendor')].concat(apps.map((item) => new Funnel(`${item.path}/vendor`)))
    );
  })(
    // consul-ui will eventually be a separate app just like the others
    // at which point we can remove this filter/extra scope
    apps.filter((item) => item.name !== 'consul-ui')
  );
  //

  let app = new EmberApp(
    Object.assign({}, defaults, {
      productionEnvironments: prodlike,
    }),
    {
      trees: trees,
      addons: addons,
      outputPaths: outputPaths,
      'ember-cli-babel': {
        includePolyfill: true,
      },
      postcssOptions: {
        compile: {
          extension: 'scss',
          plugins: [
            {
              module: require('@csstools/postcss-sass'),
              options: {
                includePaths: [
                  '../../node_modules/@hashicorp/design-system-tokens/dist/products/css',
                ],
              },
            },
            {
              module: require('tailwindcss'),
              options: {
                config: './tailwind.config.js',
              },
            },
            {
              module: require('autoprefixer'),
            },
          ],
        },
      },
      'ember-cli-string-helpers': {
        only: [
          'capitalize',
          'lowercase',
          'truncate',
          'uppercase',
          'humanize',
          'titleize',
          'classify',
        ],
      },
      'ember-cli-math-helpers': {
        only: ['div'],
      },
      babel: babel,
      autoImport: {
        // allows use of a CSP without 'unsafe-eval' directive
        forbidEval: true,
        publicAssetURL: isProd ? '{{.ContentPath}}assets' : undefined,
      },
      codemirror: {
        keyMaps: ['sublime'],
        addonFiles: [
          'lint/lint.css',
          'lint/lint.js',
          'lint/json-lint.js',
          'lint/yaml-lint.js',
          'mode/loadmode.js',
        ],
      },
      sassOptions: {
        implementation: require('sass'),
        sourceMapEmbed: sourcemaps,
      },
    }
  );
  const build = function (path, options) {
    const { root, ...rest } = options;
    if (exists(`${root}/${path}`)) {
      app.import(path, rest);
    }
  };
  apps.forEach((item) => {
    build(`vendor/${item.name}/routes.js`, {
      root: item.path,
      outputFile: `assets/${item.name}/routes.js`,
    });
    build(`vendor/${item.name}/services.js`, {
      root: item.path,
      outputFile: `assets/${item.name}/services.js`,
    });
    if (devlike) {
      build(`vendor/${item.name}/routes-debug.js`, {
        root: item.path,
        outputFile: `assets/${item.name}/routes-debug.js`,
      });
      build(`vendor/${item.name}/services-debug.js`, {
        root: item.path,
        outputFile: `assets/${item.name}/services-debug.js`,
      });
    }
  });
  // Use `app.import` to add additional libraries to the generated
  // output files.
  //
  // If you need to use different assets in different
  // environments, specify an object as the first parameter. That
  // object's keys should be the environment name and the values
  // should be the asset to use in that environment.
  //
  // If the library that you are including contains AMD or ES6
  // modules that you would like to import into your application
  // please specify an object with the list of modules as keys
  // along with the exports of each module as its value.

  // TextEncoder/Decoder polyfill. See assets/index.html
  app.import('node_modules/text-encoding/lib/encoding.js', {
    outputFile: 'assets/encoding.js',
  });
  app.import('node_modules/text-encoding/lib/encoding-indexes.js', {
    outputFile: 'assets/encoding-indexes.js',
  });

  // CSS.escape polyfill
  app.import('node_modules/css.escape/css.escape.js', { outputFile: 'assets/css.escape.js' });

  // JSON linting support. Possibly dynamically loaded via CodeMirror linting. See components/code-editor.js
  app.import('node_modules/jsonlint/lib/jsonlint.js', {
    outputFile: 'assets/codemirror/mode/javascript/javascript.js',
  });
  app.import('node_modules/codemirror/mode/javascript/javascript.js', {
    outputFile: 'assets/codemirror/mode/javascript/javascript.js',
  });

  // HCL/Ruby linting support. Possibly dynamically loaded via CodeMirror linting. See components/code-editor.js
  app.import('node_modules/codemirror/mode/ruby/ruby.js', {
    outputFile: 'assets/codemirror/mode/ruby/ruby.js',
  });

  // YAML linting support. Possibly dynamically loaded via CodeMirror linting. See components/code-editor.js
  app.import('node_modules/js-yaml/dist/js-yaml.js', {
    outputFile: 'assets/codemirror/mode/yaml/yaml.js',
  });
  app.import('node_modules/codemirror/mode/yaml/yaml.js', {
    outputFile: 'assets/codemirror/mode/yaml/yaml.js',
  });
  // XML linting support. Possibly dynamically loaded via CodeMirror linting. See services/code-mirror/linter.js
  app.import('node_modules/codemirror/mode/xml/xml.js', {
    outputFile: 'assets/codemirror/mode/xml/xml.js',
  });
  // metrics-providers
  app.import('vendor/metrics-providers/consul.js', {
    outputFile: 'assets/metrics-providers/consul.js',
  });
  app.import('vendor/metrics-providers/prometheus.js', {
    outputFile: 'assets/metrics-providers/prometheus.js',
  });
  app.import('vendor/init.js', {
    outputFile: 'assets/init.js',
  });
  return app.toTree();
};
