import Route from '@ember/routing/route';
import { get, setProperties, action } from '@ember/object';
import { inject as service } from '@ember/service';
import resolve from 'consul-ui/utils/path/resolve';

import { routes } from 'consul-ui/router';

export default class BaseRoute extends Route {
  @service('container') container;
  @service('env') env;
  @service('repository/permission') permissions;
  @service('router') router;
  @service('routlet') routlet;

  _setRouteName() {
    super._setRouteName(...arguments);

    const template = get(routes, `${this.routeName}._options.template`);
    if (typeof template !== 'undefined') {
      this.templateName = resolve(this.routeName.split('.').join('/'), template);
    }

    const queryParams = get(routes, `${this.routeName}._options.queryParams`);
    if (typeof queryParams !== 'undefined') {
      this.queryParams = queryParams;
    }
  }

  redirect(model, transition) {
    let to = get(routes, `${this.routeName}._options.redirect`);
    if (typeof to !== 'undefined') {
      // TODO: Does this need to return?
      // Almost remember things getting strange if you returned from here
      // which is why I didn't do it originally so be sure to look properly if
      // you feel like adding a return
      this.replaceWith(
        resolve(this.routeName.split('.').join('/'), to)
          .split('/')
          .join('.'),
        model
      );
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

  // TODO: this is only required due to intention_id trying to do too much
  // therefore we need to change the route parameter intention_id to just
  // intention or id or similar then we can revert to only returning a model if
  // we have searchProps (or a child route overwrites model)
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
   * Normalizes any params passed into ember `model` hooks, plus of course
   * anywhere else where `paramsFor` is used. This means the entire ember app
   * is now changed so that all paramsFor calls returns normalized params
   * instead of raw ones. For example we use this largely for URLs for the KV
   * store: /kv/*key > /ui/kv/%25-kv-name/%25-here > key = '%-kv-name/%-here'
   */
  paramsFor(name) {
    return this.routlet.normalizeParamsFor(this.routeName, super.paramsFor(...arguments));
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
