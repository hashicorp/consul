/*eslint node/no-extraneous-require: "off"*/
const useTestFrameworkDetector = require('@ember-data/private-build-infra/src/utilities/test-framework-detector');

module.exports = useTestFrameworkDetector({
  description: 'Generates Consul repository unit and integration tests',

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
      screamingSnakeCaseModuleName: options.entity.name.replace('-', '_').toUpperCase()
    };
  },
});
