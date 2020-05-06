export default {
  id: 'auth-form',
  initial: 'idle',
  context: {},
  on: {
    RESET: [
      {
        target: 'idle',
      },
    ],
  },
  states: {
    idle: {
      entry: ['clearError'],
      on: {
        SUBMIT: [
          {
            target: 'loading',
            cond: 'hasValue',
          },
          {
            target: 'error',
          },
        ],
      },
    },
    loading: {
      on: {
        ERROR: [
          {
            target: 'error',
          },
        ],
      },
    },
    error: {
      exit: ['clearError'],
      on: {
        TYPING: [
          {
            target: 'idle',
          },
        ],
        SUBMIT: [
          {
            target: 'loading',
            cond: 'hasValue',
          },
          {
            target: 'error',
          },
        ],
      },
      states: {
        user: {},
        client: {},
        server: {},
      },
    },
  },
};
