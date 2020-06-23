export default {
  id: 'list',
  initial: 'idle',
  states: {
    loading: {},
    idle: {
      on: {
        SUCCESS: {
          target: 'removed',
        },
        ERROR: {
          target: 'error',
        },
      },
    },
    removed: {
      on: {
        RESET: {
          target: 'idle',
        },
      },
    },
    error: {
      on: {
        RESET: {
          target: 'idle',
        },
      },
    },
  },
};
