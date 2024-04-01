/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'oidc-select',
  initial: 'idle',
  on: {
    RESET: [
      {
        target: 'idle',
      },
    ],
  },
  states: {
    idle: {
      on: {
        LOAD: [
          {
            target: 'loading',
          },
        ],
      },
    },
    loaded: {},
    loading: {
      on: {
        SUCCESS: [
          {
            target: 'loaded',
          },
        ],
      },
    },
  },
};
