import { get, computed } from '@ember/object';

/**
 * Converts a conventional non-pure Ember `computed` function into a pure one
 * (see https://github.com/emberjs/rfcs/blob/be351b059f08ac0fe709bc7697860d5064717a7f/text/0000-tracked-properties.md#avoiding-dependency-hell)
 *
 * @param {function} computed - a computed function to 'purify' (convert to a pure function)
 * @param {function} filter - Optional string filter function to pre-process the names of computed properties
 * @returns {function} - A pure `computed` function
 */
const _success = function(value) {
  return value;
};
const purify = function(computed, filter = args => args) {
  return function() {
    let args = [...arguments];
    let success = _success;
    // pop the user function off the end
    if (typeof args[args.length - 1] === 'function') {
      success = args.pop();
    }
    args = filter(args);
    // this is the 'conventional' `computed`
    const cb = function(name) {
      return success.apply(
        this,
        args.map(item => {
          // Right now this just takes the first part of the path so:
          // `items.[]` or `items.@each.prop` etc
          // gives you `items` which is 'probably' what you expect
          // it won't work with something like `item.objects.[]`
          // it could potentially be made to do so, but we don't need that right now at least
          return get(this, item.split('.')[0]);
        })
      );
    };
    // concat/push the user function back on
    return computed(...args.concat([cb]));
  };
};
export const subscribe = purify(computed);
export default purify;
