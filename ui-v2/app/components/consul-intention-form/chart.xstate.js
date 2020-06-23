export default {
  id: 'form',
  initial: 'idle',
  states: {
    loading: {},
    idle: {
      on: {
        PERSIST: {
          target: 'persisting',
        },
        REMOVE: {
          target: 'removing',
        },
      },
    },
    removing: {
      on: {
        SUCCESS: {
          target: 'removed',
        },
        ERROR: {
          target: 'error',
        },
      },
    },
    persisting: {
      on: {
        SUCCESS: {
          target: 'persisted',
        },
        ERROR: {
          target: 'error',
        },
      },
    },
    removed: {},
    persisted: {},
    error: {
      on: {
        RESET: {
          target: 'idle',
        },
      },
    },
  },
};
