import Route from '@ember/routing/route';
import config from 'consul-ui/config/environment';

import { routes } from 'consul-ui/router';
import flat from 'flat';

let initialize = function() {};
Route.reopen(
  ['modelFor', 'transitionTo', 'paramsFor'].reduce(function(prev, item) {
    prev[item] = function(routeName, ...rest) {
      const isNspaced = this.routeName.startsWith('nspace.');
      if (routeName === 'nspace') {
        if (isNspaced || this.routeName === 'nspace') {
          return this._super(...[routeName, ...rest]);
        } else {
          return {
            nspace: '~',
          };
        }
      }
      if (isNspaced && routeName.startsWith('dc')) {
        routeName = `nspace.${routeName}`;
      }
      return this._super(...[routeName, ...rest]);
    };
    return prev;
  }, {})
);
if (config.CONSUL_NSPACES_ENABLED) {
  const dotRe = /\./g;
  initialize = function(container) {
    const all = Object.keys(flat(routes))
      .filter(function(item) {
        return item.startsWith('dc');
      })
      .map(function(item) {
        return item.replace('._options.path', '').replace(dotRe, '/');
      });
    all.forEach(function(item) {
      let route = container.resolveRegistration(`route:${item}`);
      if (!route) {
        item = `${item}/index`;
        route = container.resolveRegistration(`route:${item}`);
      }
      route.reopen({
        templateName: item
          .replace('/root-create', '/create')
          .replace('/create', '/edit')
          .replace('/folder', '/index'),
      });
      container.register(`route:nspace/${item}`, route);
      const controller = container.resolveRegistration(`controller:${item}`);
      if (controller) {
        container.register(`controller:nspace/${item}`, controller);
      }
    });
  };
}
export default {
  initialize,
};
