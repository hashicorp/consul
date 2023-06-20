/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

'use strict';

module.exports = {
  description: 'Generates a Consul HTTP ember-data serializer',

  availableOptions: [{ name: 'base-class', type: String }],

  root: __dirname,

  fileMapTokens(options) {},
  locals(options) {
    // Return custom template variables here.
    return {};
  },

  // afterInstall(options) {
  //   // Perform extra work here.
  // }
};
