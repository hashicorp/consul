export default {
  id: 'data-writer',
  initial: 'idle',
  states: {
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
    removed: {
      on: {
        RESET: {
          target: 'idle',
        },
      },
    },
    persisted: {
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
