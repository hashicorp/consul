/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'boolean',
  initial: 'false',
  states: {
    true: {
      on: {
        TOGGLE: [
          {
            target: 'false',
          },
        ],
        FALSE: [
          {
            target: 'false',
          },
        ],
      },
    },
    false: {
      on: {
        TOGGLE: [
          {
            target: 'true',
          },
        ],
        TRUE: [
          {
            target: 'true',
          },
        ],
      },
    },
  },
};
