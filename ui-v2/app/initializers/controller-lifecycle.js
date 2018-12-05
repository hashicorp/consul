import Route from '@ember/routing/route';
/**
 * This initializer is very similar to:
 * https://github.com/kellyselden/ember-controller-lifecycle
 *
 * Why is this included here:
 * 1. Make sure lifecycle functions are functions, not just truthy.
 * 2. Right now we don't want a setup function (at least until we are definitely decided that we want one)
 * This is possibly a very personal opinion so it makes sense to just include this file here.
 */
Route.reopen({
  resetController(controller, exiting, transition) {
    this._super(...arguments);
    if (typeof controller.reset === 'function') {
      controller.reset(exiting);
    }
  },
});
export function initialize() {}

export default {
  initialize,
};
