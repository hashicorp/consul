/*eslint node/no-extraneous-require: "off"*/
'use strict';

const path = require('path');
const isModuleUnificationProject = require('@ember-data/-build-infra/src/utilities/module-unification').isModuleUnificationProject;
module.exports = {
  description: 'Generates a Consul HTTP ember-data serializer',

  availableOptions: [{ name: 'base-class', type: String }],

  root: __dirname,

  fileMapTokens(options) {
    if (isModuleUnificationProject(this.project)) {
      return {
        __root__() {
          return 'src';
        },
        __path__(options) {
          return path.join('data', 'models', options.dasherizedModuleName);
        },
        __name__() {
          return 'serializer';
        },
      };
    }
  },
  locals(options) {
    // Return custom template variables here.
    return {
    };
  }

  // afterInstall(options) {
  //   // Perform extra work here.
  // }
};
