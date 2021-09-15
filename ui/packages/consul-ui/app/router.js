/* globals requirejs */
import EmberRouter from '@ember/routing/router';
import { runInDebug } from '@ember/debug';
import { env } from 'consul-ui/env';
import walk, { dump } from 'consul-ui/utils/routing/walk';

export const routes = {
  // Our parent datacenter resource sets the namespace
  // for the entire application
  dc: {
    _options: { path: '/:dc' },
    // Services represent a consul service
    services: {
      _options: { path: '/services' },
      // Show an individual service
      show: {
        _options: { path: '/:name' },
        instances: {
          _options: { path: '/instances' },
        },
        intentions: {
          _options: { path: '/intentions' },
          edit: {
            _options: { path: '/:intention_id' },
          },
          create: {
            _options: { path: '/create' },
          },
        },
        topology: {
          _options: { path: '/topology' },
        },
        services: {
          _options: { path: '/services' },
        },
        upstreams: {
          _options: { path: '/upstreams' },
        },
        routing: {
          _options: { path: '/routing' },
        },
        tags: {
          _options: { path: '/tags' },
        },
      },
      instance: {
        _options: { path: '/:name/instances/:node/:id' },
        healthchecks: {
          _options: { path: '/health-checks' },
        },
        upstreams: {
          _options: { path: '/upstreams' },
        },
        exposedpaths: {
          _options: { path: '/exposed-paths' },
        },
        addresses: {
          _options: { path: '/addresses' },
        },
        metadata: {
          _options: { path: '/metadata' },
        },
      },
      notfound: {
        _options: { path: '/:name/:node/:id' },
      },
    },
    // Nodes represent a consul node
    nodes: {
      _options: { path: '/nodes' },
      // Show an individual node
      show: {
        _options: { path: '/:name' },
        healthchecks: {
          _options: { path: '/health-checks' },
        },
        services: {
          _options: { path: '/service-instances' },
        },
        rtt: {
          _options: { path: '/round-trip-time' },
        },
        sessions: {
          _options: { path: '/lock-sessions' },
        },
        metadata: {
          _options: { path: '/metadata' },
        },
      },
    },
    // Intentions represent a consul intention
    intentions: {
      _options: { path: '/intentions' },
      edit: {
        _options: {
          path: '/:intention_id',
          abilities: ['read intentions'],
        },
      },
      create: {
        _options: {
          path: '/create',
          abilities: ['create intentions'],
        },
      },
    },
    // Key/Value
    kv: {
      _options: { path: '/kv' },
      folder: {
        _options: { path: '/*key' },
      },
      edit: {
        _options: { path: '/*key/edit' },
      },
      create: {
        _options: {
          path: '/*key/create',
          abilities: ['create kvs'],
        },
      },
      'root-create': {
        _options: {
          path: '/create',
          abilities: ['create kvs'],
        },
      },
    },
    // ACLs
    acls: {
      _options: {
        path: '/acls',
        abilities: ['access acls'],
      },
      edit: {
        _options: { path: '/:acl' },
      },
      create: {
        _options: {
          path: '/create',
          abilities: ['create acls'],
        },
      },
      policies: {
        _options: {
          path: '/policies',
          abilities: ['read policies'],
        },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: {
            path: '/create',
            abilities: ['create policies'],
          },
        },
      },
      roles: {
        _options: {
          path: '/roles',
          abilities: ['read roles'],
        },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: {
            path: '/create',
            abilities: ['create roles'],
          },
        },
      },
      tokens: {
        _options: {
          path: '/tokens',
          abilities: env('CONSUL_ACLS_ENABLED') ? ['read tokens'] : ['access acls'],
        },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: {
            path: '/create',
            abilities: ['create tokens'],
          },
        },
      },
      'auth-methods': {
        _options: {
          path: '/auth-methods',
          abilities: ['read auth-methods'],
        },
        show: {
          _options: { path: '/:id' },
          'auth-method': {
            _options: { path: '/auth-method' },
          },
          'binding-rules': {
            _options: { path: '/binding-rules' },
          },
          'nspace-rules': {
            _options: { path: '/nspace-rules' },
          },
        },
      },
    },
    'routing-config': {
      _options: { path: '/routing-config/:name' },
    },
  },
  // Shows a datacenter picker. If you only have one
  // it just redirects you through.
  index: {
    _options: { path: '/' },
  },
  // The settings page is global.
  settings: {
    _options: { path: '/setting' },
  },
  notfound: {
    _options: { path: '/*notfound' },
  },
};
if (env('CONSUL_NSPACES_ENABLED')) {
  routes.dc.nspaces = {
    _options: {
      path: '/namespaces',
      abilities: ['read nspaces'],
    },
    edit: {
      _options: { path: '/:name' },
    },
    create: {
      _options: {
        path: '/create',
        abilities: ['create nspaces'],
      },
    },
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
