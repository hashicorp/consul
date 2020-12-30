import Route from '@ember/routing/route';
import { get, setProperties } from '@ember/object';

// paramsFor
import { routes } from 'consul-ui/router';
import wildcard from 'consul-ui/utils/routing/wildcard';
const isWildcard = wildcard(routes);

export default class BaseRoute extends Route {
  /**
   * By default any empty string query parameters should remove the query
   * parameter from the URL. This is the most common behavior if you don't
   * require this behavior overwrite this method in the specific Route for the
   * specific queryParam key.
   * If the behaviour should be different add an empty: [] parameter to the
   * queryParameter configuration to configure what is deemed 'empty'
   */
  serializeQueryParam(value, key, type) {
    if(typeof value !== 'undefined') {
      const empty = get(this, `queryParams.${key}.empty`);
      if(typeof empty === 'undefined') {
        // by default any queryParams when an empty string mean undefined,
        // therefore remove the queryParam from the URL
        if(value === '') {
          value = undefined;
        }
      } else {
        const possible = empty[0];
        let actual = value;
        if(Array.isArray(actual)) {
          actual = actual.split(',');
        }
        const diff = possible.filter(item => !actual.includes(item))
        if(diff.length === 0) {
          value = undefined;
        }
      }
    }
    return value;
  }
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
