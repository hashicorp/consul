/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'auth-form-tabs',
  initial: 'token',
  on: {
    TOKEN: [
      {
        target: 'token',
      },
    ],
    SSO: [
      {
        target: 'sso',
      },
    ],
  },
  states: {
    token: {},
    sso: {},
  },
};
