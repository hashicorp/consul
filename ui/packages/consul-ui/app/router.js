/* globals requirejs */
import EmberRouter from '@ember/routing/router';
import config from './config/environment';
import { runInDebug } from '@ember/debug';
import merge from 'deepmerge';
import { env } from 'consul-ui/env';
import walk, { dump } from 'consul-ui/utils/routing/walk';

const doc = document;
const appName = config.modulePrefix;
const appNameJS = appName
  .split('-')
  .map((item, i) => (i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item))
  .join('');

export const routes = merge.all(
  [
    {
      // Our parent datacenter resource sets the namespace
      // for the entire application
      dc: {
        _options: {
          path: '/:dc',
        },
        index: {
          _options: {
            path: '/',
            redirect: '../services',
          },
        },
        // Services represent a consul service
        services: {
          _options: { path: '/services' },
          index: {
            _options: {
              path: '/',
              queryParams: {
                sortBy: 'sort',
                status: 'status',
                source: 'source',
                kind: 'kind',
                searchproperty: {
                  as: 'searchproperty',
                  empty: [['Name', 'Tags']],
                },
                search: {
                  as: 'filter',
                  replace: true,
                },
              },
            },
          },
          // Show an individual service
          show: {
            _options: { path: '/:name' },
            instances: {
              _options: {
                path: '/instances',
                queryParams: {
                  sortBy: 'sort',
                  status: 'status',
                  source: 'source',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Node', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta', 'Node.Meta']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
            },
            intentions: {
              _options: { path: '/intentions' },
              index: {
                _options: {
                  path: '',
                  queryParams: {
                    sortBy: 'sort',
                    access: 'access',
                    searchproperty: {
                      as: 'searchproperty',
                      empty: [['SourceName', 'DestinationName']],
                    },
                    search: {
                      as: 'filter',
                      replace: true,
                    },
                  },
                },
              },
              edit: {
                _options: { path: '/:intention_id' },
              },
              create: {
                _options: {
                  template: 'dc/services/show/intentions/edit',
                  path: '/create',
                },
              },
            },
            topology: {
              _options: { path: '/topology' },
            },
            services: {
              _options: {
                path: '/services',
                queryParams: {
                  sortBy: 'sort',
                  instance: 'instance',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Tags']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
            },
            upstreams: {
              _options: {
                path: '/upstreams',
                queryParams: {
                  sortBy: 'sort',
                  instance: 'instance',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Tags']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
            },
            routing: {
              _options: { path: '/routing' },
            },
            tags: {
              _options: { path: '/tags' },
            },
          },
          instance: {
            _options: {
              path: '/:name/instances/:node/:id',
              redirect: './healthchecks',
            },
            healthchecks: {
              _options: {
                path: '/health-checks',
                queryParams: {
                  sortBy: 'sort',
                  status: 'status',
                  check: 'check',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Node', 'CheckID', 'Notes', 'Output', 'ServiceTags']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
            },
            upstreams: {
              _options: {
                path: '/upstreams',
                queryParams: {
                  sortBy: 'sort',
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['DestinationName', 'LocalBindAddress', 'LocalBindPort']],
                  },
                },
              },
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
          index: {
            _options: {
              path: '',
              queryParams: {
                sortBy: 'sort',
                status: 'status',
                searchproperty: {
                  as: 'searchproperty',
                  empty: [['Node', 'Address', 'Meta']],
                },
                search: {
                  as: 'filter',
                  replace: true,
                },
              },
            },
          },
          // Show an individual node
          show: {
            _options: { path: '/:name' },
            healthchecks: {
              _options: {
                path: '/health-checks',
                queryParams: {
                  sortBy: 'sort',
                  status: 'status',
                  kind: 'kind',
                  check: 'check',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Service', 'CheckID', 'Notes', 'Output', 'ServiceTags']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
            },
            services: {
              _options: {
                path: '/service-instances',
                queryParams: {
                  sortBy: 'sort',
                  status: 'status',
                  source: 'source',
                  searchproperty: {
                    as: 'searchproperty',
                    empty: [['Name', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta']],
                  },
                  search: {
                    as: 'filter',
                    replace: true,
                  },
                },
              },
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
          index: {
            _options: {
              path: '/',
              queryParams: {
                sortBy: 'sort',
                access: 'access',
                searchproperty: {
                  as: 'searchproperty',
                  empty: [['SourceName', 'DestinationName']],
                },
                search: {
                  as: 'filter',
                  replace: true,
                },
              },
            },
          },
          edit: {
            _options: {
              path: '/:intention_id',
              abilities: ['read intentions'],
            },
          },
          create: {
            _options: {
              template: 'dc/intentions/edit',
              path: '/create',
              abilities: ['create intentions'],
            },
          },
        },
        // Key/Value
        kv: {
          _options: { path: '/kv' },
          index: {
            _options: {
              path: '/',
              queryParams: {
                sortBy: 'sort',
                kind: 'kind',
                search: {
                  as: 'filter',
                  replace: true,
                },
              },
            },
          },
          folder: {
            _options: {
              template: 'dc/kv/index',
              path: '/*key',
            },
          },
          edit: {
            _options: { path: '/*key/edit' },
          },
          create: {
            _options: {
              template: 'dc/kv/edit',
              path: '/*key/create',
              abilities: ['create kvs'],
            },
          },
          'root-create': {
            _options: {
              template: 'dc/kv/edit',
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
              abilities: ['access acls'],
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
    },
  ].concat(
    ...[...doc.querySelectorAll(`script[data-${appName}-routes]`)].map($item =>
      JSON.parse($item.dataset[`${appNameJS}Routes`])
    )
  )
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

export default class Router extends EmberRouter {
  location = env('locationType');
  rootURL = env('rootURL');
}
Router.map(walk(routes));
