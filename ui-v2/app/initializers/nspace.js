import Route from '@ember/routing/route';
import { env } from 'consul-ui/env';

import { routes } from 'consul-ui/router';
import flat from 'flat';

let initialize = function() {};
Route.reopen(
  ['modelFor', 'transitionTo', 'replaceWith', 'paramsFor'].reduce(function(prev, item) {
    prev[item] = function(routeName, ...rest) {
      const isNspaced = this.routeName.startsWith('nspace.');
      if (routeName === 'nspace') {
        if (isNspaced || this.routeName === 'nspace') {
          return this._super(...arguments);
        } else {
          return {
            nspace: '~',
          };
        }
      }
      if (isNspaced && routeName.startsWith('dc')) {
        return this._super(...[`nspace.${routeName}`, ...rest]);
      }
      return this._super(...arguments);
    };
    return prev;
  }, {})
);
if (env('CONSUL_NSPACES_ENABLED')) {
  const dotRe = /\./g;
  initialize = function(container) {
    const register = function(route, path) {
      route.reopen({
        templateName: path
          .replace('/root-create', '/create')
          .replace('/create', '/edit')
          .replace('/folder', '/index'),
      });
      container.register(`route:nspace/${path}`, route);
      const controller = container.resolveRegistration(`controller:${path}`);
      if (controller) {
        container.register(`controller:nspace/${path}`, controller);
      }
    };
    const all = Object.keys(flat(routes))
      .filter(function(item) {
        return item.startsWith('dc');
      })
      .map(function(item) {
        return item.replace('._options.path', '').replace(dotRe, '/');
      });
    all.forEach(function(item) {
      let route = container.resolveRegistration(`route:${item}`);
      let indexed;
      // if the route doesn't exist it probably has an index route instead
      if (!route) {
        item = `${item}/index`;
        route = container.resolveRegistration(`route:${item}`);
      } else {
        // if the route does exists
        // then check to see if it also has an index route
        indexed = `${item}/index`;
        const index = container.resolveRegistration(`route:${indexed}`);
        if (typeof index !== 'undefined') {
          register(index, indexed);
        }
      }
      register(route, item);
    });
  };
}
export default {
  initialize,
};
