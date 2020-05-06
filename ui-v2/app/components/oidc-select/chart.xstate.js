export default {
  id: 'oidc-select',
  initial: 'loading',
  context: {},
  on: {},
  states: {
    loaded: {},
    loading: {
      on: {
        SUCCESS: [
          {
            target: 'loaded',
          },
        ],
      },
    },
  },
};
