import Route from '@ember/routing/route';
import { routes } from 'consul-ui/router';
import { env } from 'consul-ui/env';
import flat from 'flat';

const withNspace = function(currentRouteName, requestedRouteName, ...rest) {
  const isNspaced = currentRouteName.startsWith('nspace.');
  if (isNspaced && requestedRouteName.startsWith('dc')) {
    return [`nspace.${requestedRouteName}`, ...rest];
  }
  return [requestedRouteName, ...rest];
};

const register = function(container, route, path) {
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

export function initialize(container) {
  // // patch Route routeName-like methods for navigation to support nspace relative routes
  // Route.reopen(
  //   ['transitionTo', 'replaceWith'].reduce(function(prev, item) {
  //     prev[item] = function(requestedRouteName, ...rest) {
  //       return this._super(...withNspace(this.routeName, requestedRouteName, ...rest));
  //     };
  //     return prev;
  //   }, {})
  // );

  // // patch Route routeName-like methods for data to support nspace relative routes
  // Route.reopen(
  //   ['modelFor', 'paramsFor'].reduce(function(prev, item) {
  //     prev[item] = function(requestedRouteName, ...rest) {
  //       const isNspaced = this.routeName.startsWith('nspace.');
  //       if (requestedRouteName === 'nspace' && !isNspaced && this.routeName !== 'nspace') {
  //         return {
  //           nspace: '~',
  //         };
  //       }
  //       return this._super(...withNspace(this.routeName, requestedRouteName, ...rest));
  //     };
  //     return prev;
  //   }, {})
  // );

  // // extend router service with a nspace aware router to support nspace relative routes
  // const nspacedRouter = container.resolveRegistration('service:router').extend({
  //   transitionTo: function(requestedRouteName, ...rest) {
  //     return this._super(...withNspace(this.currentRoute.name, requestedRouteName, ...rest));
  //   },
  //   replaceWith: function(requestedRouteName, ...rest) {
  //     return this._super(...withNspace(this.currentRoute.name, requestedRouteName, ...rest));
  //   },
  //   urlFor: function(requestedRouteName, ...rest) {
  //     return this._super(...withNspace(this.currentRoute.name, requestedRouteName, ...rest));
  //   },
  // });
  // container.register('service:router', nspacedRouter);

  if (env('CONSUL_NSPACES_ENABLED')) {
    // enable the nspace repo
    ['dc', 'settings', 'dc.intentions.edit', 'dc.intentions.create'].forEach(function(item) {
      container.inject(`route:${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
      container.inject(`route:nspace.${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
    });
    container.inject('route:application', 'nspacesRepo', 'service:repository/nspace/enabled');

    // const dotRe = /\./g;
    // // register automatic 'index' routes and controllers that start with 'dc'
    // Object.keys(flat(routes))
    //   .filter(function(item) {
    //     return item.startsWith('dc');
    //   })
    //   .filter(function(item) {
    //     return item.endsWith('path');
    //   })
    //   .map(function(item) {
    //     return item.replace('._options.path', '').replace(dotRe, '/');
    //   })
    //   .forEach(function(item) {
    //     let route = container.resolveRegistration(`route:${item}`);
    //     let indexed;
    //     // if the route doesn't exist it probably has an index route instead
    //     if (!route) {
    //       item = `${item}/index`;
    //       route = container.resolveRegistration(`route:${item}`);
    //     } else {
    //       // if the route does exist
    //       // then check to see if it also has an index route
    //       indexed = `${item}/index`;
    //       const index = container.resolveRegistration(`route:${indexed}`);
    //       if (typeof index !== 'undefined') {
    //         register(container, index, indexed);
    //       }
    //     }

    //     if (typeof route !== 'undefined') {
    //       register(container, route, item);
    //     }
    //   });
  }
}

export default {
  initialize,
};
