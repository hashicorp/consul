export default {
  id: 'oidc-select',
  initial: 'loading',
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
