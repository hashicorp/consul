/**
 * super stubber
 * Ember's `_super` functionality is a little challenging to stub.
 * The following will essentially let you stub `_super`, letting
 * you test single units of code that use `_super`
 * The return value is a function with the same signature as
 * traditional `test` or `it` test functions. Any test code
 * used within the cb 'sandbox' will use the stubbed `_super`.
 * It's done this way to attempt to make it easy to reuse for various tests
 * using the same stub in a recognisable way.
 *
 * @param {object} obj - The instance with the that `_super` belongs to
 * @param {object} [stub=function(){return arguments}] -
 *    The stub to use in place of `_super`, if not specified `_super` will
 *    simply return the `arguments` passed to it
 *
 * @returns {function}
 */
export default function(obj, stub) {
  const _super =
    typeof stub === 'undefined'
      ? function() {
          return arguments;
        }
      : stub;
  /**
   * @param {string} message - Message to accompany the test concept (currently unused)
   * @param {function} cb - Callback that performs the test, will use the stubbed `_super`
   * @returns The result of `cb`, and therefore maintain the same API
   */
  return function(message, _cb) {
    const cb = typeof message === 'function' ? message : _cb;

    let orig = obj._super;
    Object.defineProperty(Object.getPrototypeOf(obj), '_super', {
      set: function() {},
      get: function() {
        return _super;
      },
    });
    // TODO: try/catch this?
    const actual = cb();
    Object.defineProperty(Object.getPrototypeOf(obj), '_super', {
      set: function(val) {
        orig = val;
      },
      get: function() {
        return orig;
      },
    });
    return actual;
  };
}
