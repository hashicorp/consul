export default {
  id: 'data-loader',
  initial: 'load',
  on: {
    OPEN: {
      target: 'load',
    },
    ERROR: {
      target: 'disconnected',
    },
    LOAD: [
      {
        target: 'idle',
        cond: 'loaded',
      },
      {
        target: 'loading',
      },
    ],
    INVALIDATE: [
      {
        target: 'invalidating',
      },
    ],
  },
  states: {
    load: {},
    invalidating: {},
    loading: {
      on: {
        SUCCESS: {
          target: 'idle',
        },
        ERROR: {
          target: 'error',
        },
      },
    },
    idle: {},
    error: {
      on: {
        RETRY: {
          target: 'load',
        },
      },
    },
    disconnected: {
      on: {
        RETRY: {
          target: 'load',
        },
      },
    },
  },
};
