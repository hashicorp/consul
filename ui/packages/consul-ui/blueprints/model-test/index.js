/*eslint node/no-extraneous-require: "off"*/
const useTestFrameworkDetector = require('@ember-data/private-build-infra/src/utilities/test-framework-detector');

module.exports = useTestFrameworkDetector({
  description: 'Generates a Consul ember-data model unit test.',

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
