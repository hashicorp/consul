import { setModifierManager, capabilities } from '@ember/modifier';
import { gte } from 'ember-compatibility-helpers';

const createEventLike = state => {
  return {
    target: state.element,
    currentTarget: state.element,
  };
};
export default setModifierManager(
  () => ({
    capabilities: capabilities(gte('3.22.0') ? '3.22' : '3.13', { disableAutoTracking: true }),

    createModifier() {
      return { element: null };
    },

    installModifier(state, element, args) {
      state.element = element;
      if (gte('3.22.0')) {
        // Consume individual properties to entangle tracking.
        // https://github.com/emberjs/ember.js/issues/19277
        // https://github.com/ember-modifier/ember-modifier/pull/63#issuecomment-815908201
        args.positional.forEach(() => {});
        args.named && Object.values(args.named);
      }
      const [fn, ...positional] = args.positional;
      fn(createEventLike(state), positional, args.named);
    },

    updateModifier(state, args) {
      if (gte('3.22.0')) {
        // Consume individual properties to entangle tracking.
        // https://github.com/emberjs/ember.js/issues/19277
        // https://github.com/ember-modifier/ember-modifier/pull/63#issuecomment-815908201
        args.positional.forEach(() => {});
        args.named && Object.values(args.named);
      }
      const [fn, ...positional] = args.positional;
      fn(createEventLike(state), positional, args.named);
    },

    destroyModifier() {},
  }),
  class DidUpsertModifier {}
);
