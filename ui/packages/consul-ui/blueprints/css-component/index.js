'use strict';

const path = require('path');
module.exports = {
  description: 'Generates a CSS component',

  availableOptions: [],

  root: __dirname,

  fileMapTokens(options) {
    return {
      __path__() {
        return path.join('styles', 'components');
      },
    };
  },
  locals(options) {
    // Return custom template variables here.
    return {};
  },

  // afterInstall(options) {
  //   // Perform extra work here.
  // }
};
