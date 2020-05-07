import EmberRouter from '@ember/routing/router';
import { env } from 'consul-ui/env';
import walk from 'consul-ui/utils/routing/walk';

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
        proxy: {
          _options: { path: '/proxy' },
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
        tags: {
          _options: { path: '/tags' },
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
          _options: { path: '/services' },
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
        _options: { path: '/:id' },
      },
      create: {
        _options: { path: '/create' },
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
        _options: { path: '/*key/create' },
      },
      'root-create': {
        _options: { path: '/create' },
      },
    },
    // ACLs
    acls: {
      _options: { path: '/acls' },
      edit: {
        _options: { path: '/:id' },
      },
      create: {
        _options: { path: '/create' },
      },
      policies: {
        _options: { path: '/policies' },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: { path: '/create' },
        },
      },
      roles: {
        _options: { path: '/roles' },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: { path: '/create' },
        },
      },
      tokens: {
        _options: { path: '/tokens' },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: { path: '/create' },
        },
      },
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
    _options: { path: '/*path' },
  },
};
if (env('CONSUL_NSPACES_ENABLED')) {
  routes.dc.nspaces = {
    _options: { path: '/namespaces' },
    edit: {
      _options: { path: '/:name' },
    },
    create: {
      _options: { path: '/create' },
    },
  };
  routes.nspace = {
    _options: { path: '/:nspace' },
    dc: routes.dc,
  };
}
export default class Router extends EmberRouter {
  location = env('locationType');
  rootURL = env('rootURL');
}

Router.map(walk(routes));
