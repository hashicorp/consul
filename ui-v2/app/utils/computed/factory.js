/**
 * Gives you factory function to create a specified type of ComputedProperty
 * Largely taken from https://github.com/emberjs/ember.js/blob/v2.18.2/packages/ember-metal/lib/computed.js#L529
 * but configurable from the outside (IoC) so its reuseable
 *
 * @param {Class} ComputedProperty - ComputedProperty to use for the factory
 * @returns {function} - Ember-like `computed` function (see https://www.emberjs.com/api/ember/2.18/classes/ComputedProperty)
 */
export default function(ComputedProperty) {
  return function() {
    const args = [...arguments];
    const cp = new ComputedProperty(args.pop());
    if (args.length > 0) {
      cp.property(...args);
    }
    return cp;
  };
}
