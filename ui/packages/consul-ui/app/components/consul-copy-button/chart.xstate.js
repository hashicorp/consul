/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'copy-button',
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
