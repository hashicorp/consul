export default {
  id: 'data-loader',
  initial: 'load',
  on: {
    ERROR: {
      target: 'changeError',
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
  },
  states: {
    load: {},
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
    changeError: {
      on: {
        RETRY: {
          target: 'load',
        },
      },
    },
  },
};
