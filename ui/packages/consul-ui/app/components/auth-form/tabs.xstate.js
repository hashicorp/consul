/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
