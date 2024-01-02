/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

const path = require('path');
module.exports = {
  description: 'Generates a Consul repository',

  availableOptions: [],

  root: __dirname,

  fileMapTokens(options) {
    return {
      __path__() {
        return path.join('services', 'repository');
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
