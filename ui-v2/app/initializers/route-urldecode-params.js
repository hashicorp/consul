import Route from '@ember/routing/route';
import { routes } from 'consul-ui/router';
import wildcard from 'consul-ui/utils/routing/wildcard';
const isWildcard = wildcard(routes);
/**
 * This initializer adds urldecoding to the `params` passed into
 * ember `model` hooks, plus of course anywhere else where `paramsFor`
 * is used. This means the entire ember app is now changed so that all
 * paramsFor calls returns urldecoded params instead of raw ones
 */
Route.reopen({
  paramsFor: function() {
    const params = this._super(...arguments);
    if (isWildcard(this.routeName)) {
      return Object.keys(params).reduce(function(prev, item) {
        if (typeof params[item] !== 'undefined') {
          prev[item] = decodeURIComponent(params[item]);
        } else {
          prev[item] = params[item];
        }
        return prev;
      }, {});
    } else {
      return params;
    }
  },
});
export function initialize() {}

export default {
  initialize,
};
