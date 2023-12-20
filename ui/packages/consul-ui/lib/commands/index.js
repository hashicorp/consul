/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint no-console: "off" */
/* eslint-env node */
'use strict';
module.exports = {
  name: 'commands',
  includedCommands: function () {
    return {
      'steps:list': {
        name: 'steps:list',
        run: function (config, args) {
          require('./lib/list.js')(`${process.cwd()}/tests/steps.js`);
        },
      },
    };
  },
  isDevelopingAddon() {
    return true;
  },
};
