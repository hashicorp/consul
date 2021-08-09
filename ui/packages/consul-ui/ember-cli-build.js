'use strict';
const Funnel = require('broccoli-funnel');
const EmberApp = require('ember-cli/lib/broccoli/ember-app');
const utils = require('./config/utils');

module.exports = function(defaults, $ = process.env) {
  // available environments
  // ['production', 'development', 'staging', 'test'];

  $ = utils.env($);
  const env = EmberApp.env();
  const prodlike = ['production', 'staging'];
  const sourcemaps = !['production'].includes(env) && !$('BABEL_DISABLE_SOURCEMAPS', false);

  const trees = {};
  const addons = {};
  const outputPaths = {};
  let excludeFiles = [];

  const babel = {
    plugins: [
      '@babel/plugin-proposal-object-rest-spread',
    ],
    sourceMaps: sourcemaps ? 'inline' : false,
  }

  // setup up different build configuration depending on environment
  if(!['test'].includes(env)) {
    // exclude any component/pageobject.js files from anything but test
    excludeFiles = excludeFiles.concat([
      'components/**/pageobject.js',
      'components/**/*.test-support.js',
      'components/**/*.test.js',
    ])
  }

  if(['test', 'production'].includes(env)) {
    // exclude our debug initializer, route and template
    excludeFiles = excludeFiles.concat([
      'instance-initializers/debug.js',
      'templates/debug.hbs',
      'components/debug/**/*.*'
    ])
    // exclude any debug like addons from production or test environments
    addons.blacklist = [
      // exclude docfy
      '@docfy/ember'
    ];
  } else {
    // add debug css is we are not in test or production environments
    outputPaths.app = {
      css: {
        'debug': '/assets/debug.css'
      }
    }
  }
  if(['production'].includes(env)) {
    // everything apart from production is 'debug', including test
    // which means this and everything it affects is never tested
    babel.plugins.push(
      ['strip-function-call', {'strip': ['Ember.runInDebug']}]
    )
  }
  //

  trees.app = new Funnel('app', {
    exclude: excludeFiles
  });

  const app = new EmberApp(
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
      'ember-cli-string-helpers': {
        only: ['capitalize', 'lowercase', 'truncate', 'uppercase', 'humanize', 'titleize', 'classify'],
      },
      'ember-cli-math-helpers': {
        only: ['div'],
      },
      babel: babel,
      autoImport: {
        // allows use of a CSP without 'unsafe-eval' directive
        forbidEval: true,
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
      'ember-cli-uglify': {
        uglify: {
          compress: {
            keep_fargs: false,
          },
        },
      },
      sassOptions: {
        implementation: require('sass'),
        sourceMapEmbed: sourcemaps,
      },
    }
  );
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
