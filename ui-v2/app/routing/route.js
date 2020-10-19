import Route from '@ember/routing/route';
import { setProperties } from '@ember/object';

// paramsFor
import { routes } from 'consul-ui/router';
import wildcard from 'consul-ui/utils/routing/wildcard';
const isWildcard = wildcard(routes);

export default class BaseRoute extends Route {
  /**
   * Set the routeName for the controller so that it is available in the template
   * for the route/controller.. This is mainly used to give a route name to the
   * Outlet component
   */
  setupController(controller, model) {
    setProperties(controller, {
      routeName: this.routeName,
    });
    super.setupController(...arguments);
  }
  /**
   * Adds urldecoding to any wildcard route `params` passed into ember `model`
   * hooks, plus of course anywhere else where `paramsFor` is used. This means
   * the entire ember app is now changed so that all paramsFor calls returns
   * urldecoded params instead of raw ones.
   * For example we use this largely for URLs for the KV store:
   * /kv/*key > /ui/kv/%25-kv-name/%25-here > key = '%-kv-name/%-here'
   */
  paramsFor(name) {
    const params = super.paramsFor(...arguments);
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
  }
}
