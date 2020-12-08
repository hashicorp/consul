/**
 * Promise aware conditional function call
 *
 * @param {function} cb - The function to possibily call
 * @param {function} [what] - A function returning a boolean resolving promise
 * @returns {function} - function when called returns a Promise that resolves the argument it is called with
 */
export default function(cb, what) {
  return function(res) {
    return what.then(function(bool) {
      if (bool) {
        cb();
      }
      return res;
    });
  };
}
