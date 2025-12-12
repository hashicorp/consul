/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'token-source',
  initial: 'idle',
  on: {
    RESTART: [
      {
        target: 'secret',
        cond: 'isSecret',
      },
      {
        target: 'provider',
      },
    ],
  },
  states: {
    idle: {},
    secret: {},
    provider: {
      on: {
        SUCCESS: 'jwt',
      },
    },
    jwt: {
      on: {
        SUCCESS: 'token',
      },
    },
    token: {},
  },
};
