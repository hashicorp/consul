/* globals requirejs */
import EmberRouter from '@ember/routing/router';
import config from './config/environment';
import { runInDebug } from '@ember/debug';
import merge from 'deepmerge';
import { env } from 'consul-ui/env';
import walk, { dump } from 'consul-ui/utils/routing/walk';

const doc = document;
const appName = config.modulePrefix;

export const routes = merge.all(
  [...doc.querySelectorAll(`script[data-routes]`)].map($item => JSON.parse($item.dataset[`routes`]))
);

runInDebug(() => {
  // check to see if we are running docfy and if so add its routes to our
  // route config
  const docfyOutput = requirejs.entries[`${appName}/docfy-output`];
  if (typeof docfyOutput !== 'undefined') {
    const output = {};
    docfyOutput.callback(output);
    // see https://github.com/josemarluedke/docfy/blob/904529641279975586402431108895713d156b55/packages/ember/addon/index.ts
    (function addPage(route, page) {
      if (page.name !== '/') {
        route = route[page.name] = {
          _options: { path: page.name },
        };
      }
      page.pages.forEach(page => {
        const url = page.relativeUrl;
        if (typeof url === 'string') {
          if (url !== '') {
            route[url] = {
              _options: { path: url },
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

// Consul UIs routes are kept in individual configuration files Please see for
// example /ui/pacakges/consul-ui/vendor/routes.js Routing for additional
// applications/features are kept in the corresponding configuration files for
// the application/feature and optional merged at runtime depending on a
// Consul backend feature flag. Please see for example
// /ui/packages/consul-nspaces/vendor/route.js
export default class Router extends EmberRouter {
  location = env('locationType');
  rootURL = env('rootURL');
}
Router.map(walk(routes));
