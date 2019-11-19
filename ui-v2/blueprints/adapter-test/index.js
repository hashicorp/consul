/*eslint node/no-extraneous-require: "off"*/
'use strict';

const useTestFrameworkDetector = require('@ember-data/-build-infra/src/utilities/test-framework-detector');
const path = require('path');

module.exports = useTestFrameworkDetector({
  description: 'Generates a Consul HTTP ember-data adapter unit and integration tests',

  root: __dirname,

  fileMapTokens(options) {
    return {
      __root__() {
        return 'tests';
      },
      __path__() {
        return '';
      }
    };
  },

  locals(options) {
    return {
    };
  },
});
