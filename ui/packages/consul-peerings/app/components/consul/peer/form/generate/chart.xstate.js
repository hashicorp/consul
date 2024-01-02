/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  id: 'consul-peer-generate-form',
  initial: 'idle',
  states: {
    idle: {
      on: {
        LOAD: {
          target: 'loading'
        }
      }
    },
    loading: {
      on: {
        SUCCESS: {
          target: 'success'
        },
        ERROR: {
          target: 'error'
        }
      }
    },
    success: {
      on: {
        RESET: {
          target: 'idle'
        }
      }
    },
    error: {},
  },
};
