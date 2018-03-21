export default function(obj, stub) {
  const _super =
    typeof stub === 'undefined'
      ? function() {
          return arguments;
        }
      : stub;
  return function(message, _cb) {
    const cb = typeof message === 'function' ? message : _cb;

    let orig = obj._super;
    Object.defineProperty(Object.getPrototypeOf(obj), '_super', {
      set: function() {},
      get: function() {
        return _super;
      },
    });
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
