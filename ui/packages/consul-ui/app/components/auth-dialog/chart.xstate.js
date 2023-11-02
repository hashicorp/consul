/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'auth-dialog',
  initial: 'idle',
  on: {
    CHANGE: [
      {
        target: 'authorized',
        cond: 'hasToken',
        actions: ['login'],
      },
      {
        target: 'unauthorized',
        actions: ['logout'],
      },
    ],
  },
  states: {
    idle: {
      on: {
        CHANGE: [
          {
            target: 'authorized',
            cond: 'hasToken',
          },
          {
            target: 'unauthorized',
          },
        ],
      },
    },
    unauthorized: {},
    authorized: {},
  },
};
