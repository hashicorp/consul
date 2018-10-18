'use strict';

const EmberApp = require('ember-cli/lib/broccoli/ember-app');
module.exports = function(defaults) {
  const env = EmberApp.env();
  const prodlike = ['production', 'staging'];
  const isProd = env === 'production';
  // leave this in for now for when I start a proper staging env
  // const isProdLike = prodlike.indexOf(env) > -1;
  const sourcemaps = !isProd;
  let app = new EmberApp(
    Object.assign(
      {},
      defaults,
      {
        productionEnvironments: prodlike
      }
    ), {
    'ember-cli-babel': {
      includePolyfill: true
    },
    'ember-cli-string-helpers': {
      only: ['capitalize', 'lowercase', 'truncate']
    },
    'ember-cli-math-helpers': {
      only: ['div']
    },
    'babel': {
      plugins: [
        'transform-object-rest-spread'
      ]
    },
    'codemirror': {
      modes: ['javascript','ruby'],
      keyMaps: ['sublime']
    },
    'ember-cli-uglify': {
      uglify: {
        compress: {
          keep_fargs: false,
        },
      },
    },
    'sassOptions': {
      implementation: require('node-sass'),
      sourceMapEmbed: sourcemaps,
    },
    'autoprefixer': {
      sourcemap: sourcemaps,
      grid: true,
      browsers: [
        "defaults",
        "ie 11"
      ]
    },
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
  app.import('node_modules/text-encoding/lib/encoding-indexes.js', {outputFile: 'assets/encoding-indexes.js'})
  app.import('node_modules/text-encoding/lib/encoding.js', {outputFile: 'assets/encoding.js'})
  let tree = app.toTree();
  return tree;
};
