/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
