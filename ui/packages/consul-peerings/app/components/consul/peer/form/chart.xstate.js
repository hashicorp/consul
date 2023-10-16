/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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
