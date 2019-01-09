import Route from '@ember/routing/route';

/**
 * This initializer adds urldecoding to the `params` passed into
 * ember `model` hooks, plus of course anywhere else where `paramsFor`
 * is used. This means the entire ember app is now changed so that all
 * paramsFor calls returns urldecoded params instead of raw ones
 */
Route.reopen({
  paramsFor: function() {
    const params = this._super(...arguments);
    return Object.keys(params).reduce(function(prev, item) {
      if (typeof params[item] !== 'undefined') {
        prev[item] = decodeURIComponent(params[item]);
      } else {
        prev[item] = params[item];
      }
      return prev;
    }, {});
  },
});
export function initialize() {}

export default {
  initialize,
};
