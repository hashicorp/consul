import { setModifierManager, capabilities } from '@ember/modifier';

export default setModifierManager(
  () => ({
    capabilities: capabilities('3.13', { disableAutoTracking: true }),

    createModifier() {},

    installModifier(_state, element, { positional: [fn, ...args], named }) {
      let shadow;
      try {
        shadow = element.attachShadow({ mode: 'open' });
      } catch (e) {
        // shadow = false;
        console.error(e);
      }
      fn(shadow);
    },
    updateModifier() {},
    destroyModifier() {},
  }),
  class CustomElementModifier {}
);
