/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
