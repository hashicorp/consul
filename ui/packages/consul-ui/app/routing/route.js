import Route from '@ember/routing/route';
import { get, setProperties, action } from '@ember/object';
import { inject as service } from '@ember/service';

// paramsFor
import { routes } from 'consul-ui/router';
import wildcard from 'consul-ui/utils/routing/wildcard';

const isWildcard = wildcard(routes);

export default class BaseRoute extends Route {
  @service('container') container;
  @service('env') env;
  @service('repository/permission') permissions;
  @service('router') router;

  /**
   * Inspects a custom `abilities` array on the router for this route. Every
   * abililty needs to 'pass' for the route not to throw a 403 error. Anything
   * more complex then this (say ORs) should use a single ability and perform
   * the OR logic in the test for the ability. Note, this ability check happens
   * before any calls to the backend for this model/route.
   */
  async beforeModel() {
    // remove any references to index as it is the same as the root routeName
    const routeName = this.routeName
      .split('.')
      .filter(item => item !== 'index')
      .join('.');
    const abilities = get(routes, `${routeName}._options.abilities`) || [];
    if (abilities.length > 0) {
      if (!abilities.every(ability => this.permissions.can(ability))) {
        throw new HTTPError('403');
      }
    }
  }

  /**
   * By default any empty string query parameters should remove the query
   * parameter from the URL. This is the most common behavior if you don't
   * require this behavior overwrite this method in the specific Route for the
   * specific queryParam key.
   * If the behaviour should be different add an empty: [] parameter to the
   * queryParameter configuration to configure what is deemed 'empty'
   */
  serializeQueryParam(value, key, type) {
    if (typeof value !== 'undefined') {
      const empty = get(this, `queryParams.${key}.empty`);
      if (typeof empty === 'undefined') {
        // by default any queryParams when an empty string mean undefined,
        // therefore remove the queryParam from the URL
        if (value === '') {
          value = undefined;
        }
      } else {
        const possible = empty[0];
        let actual = value;
        if (Array.isArray(actual)) {
          actual = actual.split(',');
        }
        const diff = possible.filter(item => !actual.includes(item));
        if (diff.length === 0) {
          value = undefined;
        }
      }
    }
    return value;
  }

  // FIXME: this is only required due to intention_id trying to do too much
  model() {
    const model = {};
    if (
      typeof this.queryParams !== 'undefined' &&
      typeof this.queryParams.searchproperty !== 'undefined'
    ) {
      model.searchProperties = this.queryParams.searchproperty.empty[0];
    }
    return model;
  }

  /**
   * Set the routeName for the controller so that it is available in the template
   * for the route/controller.. This is mainly used to give a route name to the
   * Outlet component
   */
  setupController(controller, model) {
    setProperties(controller, {
      ...model,
      routeName: this.routeName,
    });
    super.setupController(...arguments);
  }

  optionalParams() {
    return this.container.get(`location:${this.env.var('locationType')}`).optionalParams();
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

  @action
  async replaceWith(routeName, obj) {
    await Promise.resolve();
    let params = [];
    if (typeof obj === 'string') {
      params = [obj];
    }
    if (typeof obj !== 'undefined' && !Array.isArray(obj) && typeof obj !== 'string') {
      params = Object.values(obj);
    }
    return super.replaceWith(routeName, ...params);
  }

  @action
  async transitionTo(routeName, obj) {
    await Promise.resolve();
    let params = [];
    if (typeof obj === 'string') {
      params = [obj];
    }
    if (typeof obj !== 'undefined' && !Array.isArray(obj) && typeof obj !== 'string') {
      params = Object.values(obj);
    }
    return super.transitionTo(routeName, ...params);
  }
}
