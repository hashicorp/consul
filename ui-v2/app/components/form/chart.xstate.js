export default {
  id: 'form',
  initial: 'load',
  states: {
    load: {
      on: {
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
    },
    loading: {
      on: {
        SUCCESS: {
          target: 'idle',
        },
        ERROR: {
          target: 'loadError',
        },
      },
    },
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
    loadError: {
      on: {
        RESET: {
          target: 'idle',
        },
      },
    },
  },
};
