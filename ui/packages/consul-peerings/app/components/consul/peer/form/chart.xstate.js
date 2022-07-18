export default {
  id: 'consul-peer-form',
  initial: 'generate',
  on: {
    INITIATE: [
      {
        target: 'initiate',
      },
    ],
    GENERATE: [
      {
        target: 'generate',
      },
    ],
    SUCCESS: [
      {
        target: 'success',
      },
    ],
  },
  states: {
    initiate: {},
    generate: {},
    success: {},
  },
};
