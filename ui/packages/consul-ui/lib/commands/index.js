/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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
