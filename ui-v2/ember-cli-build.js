'use strict';

const EmberApp = require('ember-cli/lib/broccoli/ember-app');
const stew = require('broccoli-stew');
module.exports = function(defaults) {
  let app = new EmberApp(defaults, {
    'ember-cli-babel': {
      includePolyfill: true
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
    'autoprefixer': {
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
  let tree = app.toTree();
  if (app.env === 'production') {
    tree = stew.rm(tree, 'consul-api-double');
  }
  return tree;
};
