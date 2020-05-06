export default {
  id: 'auth-dialog',
  initial: 'idle',
  context: {},
  on: {
    CHANGE: [
      {
        target: 'authorized',
        cond: 'hasToken',
        actions: ['login'],
      },
      {
        target: 'unauthorized',
        actions: ['logout'],
      },
    ],
  },
  states: {
    idle: {
      on: {
        CHANGE: [
          {
            target: 'authorized',
            cond: 'hasToken',
          },
          {
            target: 'unauthorized',
          },
        ],
      },
    },
    unauthorized: {},
    authorized: {},
  },
};
