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
  },
  states: {
    initiate: {},
    generate: {},
  },
};
