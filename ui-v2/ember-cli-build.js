'use strict';

const EmberApp = require('ember-cli/lib/broccoli/ember-app');
module.exports = function(defaults) {
  const env = EmberApp.env();
  const prodlike = ['production', 'staging'];
  const isProd = env === 'production';
  const isProdLike = prodlike.indexOf(env) > -1;
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
      only: ['capitalize']
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
    'ember-cli-string-helpers': {
      only: ['lowercase']
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
  let tree = app.toTree();
  return tree;
};
