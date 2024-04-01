/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'form',
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
        SUCCESS: [
          {
            target: 'success',
          },
        ],
        ERROR: [
          {
            target: 'error',
          },
        ],
      },
    },
    success: {},
    error: {},
  },
};
