/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  id: 'consul-peer-form',
  initial: 'generate',
  on: {
    INITIATE: [
      {
        target: 'initiate',
      },
    ],
    GENERATE: [
      {
        target: 'generate',
      },
    ],
  },
  states: {
    initiate: {},
    generate: {},
  },
};
