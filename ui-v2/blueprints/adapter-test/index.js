const path = require('path');

const testInfo = require('ember-cli-test-info');
const useTestFrameworkDetector = require('@ember-data/private-build-infra/src/utilities/test-framework-detector');

module.exports = useTestFrameworkDetector({
  description: 'Generates Consul HTTP ember-data adapter unit and integration tests',

  root: __dirname,

  fileMapTokens(options) {
    return {
      __root__() {
        return 'tests';
      },
      __path__() {
        return '';
      },
    };
  },

  locals(options) {
    return {
    };
  },
});
