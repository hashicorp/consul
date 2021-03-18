/* globals requirejs, hcl */
// TODO: Remove at least hcl global ^
import EmberRouter from '@ember/routing/router';
import { runInDebug } from '@ember/debug';
import merge from 'deepmerge';
import { env } from 'consul-ui/env';
import walk, { dump } from 'consul-ui/utils/routing/walk';

import routesOSS from './router.oss.hcl';
export let routes = routesOSS;

if (env('CONSUL_NSPACES_ENABLED')) {
  routes = merge(
    routes,
    hcl`
      route "dc" {
        route "nspaces" {
          path = "/namespaces"
          route "edit" {
            path = "/:name"
          }
          route "create" {
            path = "/create"
          }
        }
      }
      route "nspace" {
        path = "/:nspace"
      }
    `
  );
  routes.route.nspace.route = {
    dc: routes.route.dc,
  };
}

runInDebug(() => {
  // check to see if we are running docfy and if so add its routes to our
  // route config
  const docfyOutput = requirejs.entries['consul-ui/docfy-output'];
  if (typeof docfyOutput !== 'undefined') {
    const output = {};
    docfyOutput.callback(output);
    // see https://github.com/josemarluedke/docfy/blob/904529641279975586402431108895713d156b55/packages/ember/addon/index.ts
    (function addPage(route, page) {
      if (page.name !== '/') {
        if (typeof route.route === 'undefined') {
          route.route = {};
        }
        route = route.route[page.name] = {
          path: page.name,
        };
      }
      page.pages.forEach(page => {
        const url = page.relativeUrl;
        if (typeof url === 'string') {
          if (url !== '') {
            if (typeof route.route === 'undefined') {
              route.route = {};
            }
            route.route[url] = {
              path: url,
            };
          }
        }
      });
      page.children.forEach(child => {
        addPage(route, child);
      });
    })(routes, output.default.nested);
  }
});
export default class Router extends EmberRouter {
  location = env('locationType');
  rootURL = env('rootURL');
}
Router.map(walk(routes));

// To print the Ember route DSL use `Routes()` in Web Inspectors console
// or `javascript:Routes()` in the location bar of your browser
runInDebug(() => {
  window.Routes = (endpoint = env('DEBUG_ROUTES_ENDPOINT')) => {
    if (!endpoint) {
      endpoint = 'data:,%s';
    }
    let win;
    const str = dump(routes);
    if (endpoint.startsWith('data:,')) {
      win = window.open('', '_blank');
      win.document.write(`<body><pre>${str}</pre></body>`);
    } else {
      win = window.open(endpoint.replace('%s', encodeURIComponent(str)), '_blank');
    }
    win.focus();
    return;
  };
});
