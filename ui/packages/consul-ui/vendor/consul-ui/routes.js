/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

(routes =>
  routes({
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
      show: {
        _options: {
          path: '/overview',
          abilities: ['access overview'],
        },
        serverstatus: {
          _options: {
            path: '/server-status',
            abilities: ['read servers'],
          },
        },
        cataloghealth: {
          _options: {
            path: '/catalog-health',
            abilities: ['access overview'],
          },
        },
        license: {
          _options: {
            path: '/license',
            abilities: ['read license'],
          },
        },
      },
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
                empty: [['Partition', 'Name', 'Tags', 'PeerName']],
              },
              search: {
                as: 'filter',
                replace: true,
              },
            },
          },
        },
        show: {
          _options: {
            path: '/:name',
          },
          instances: {
            _options: {
              path: '/instances',
              queryParams: {
                sortBy: 'sort',
                status: 'status',
                source: 'source',
                searchproperty: {
                  as: 'searchproperty',
                  empty: [
                    ['Name', 'Node', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta', 'Node.Meta'],
                  ],
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
                template: '../edit',
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
      nodes: {
        _options: { path: '/nodes' },
        index: {
          _options: {
            path: '',
            queryParams: {
              sortBy: 'sort',
              status: 'status',
              version: 'version',
              searchproperty: {
                as: 'searchproperty',
                empty: [['Node', 'Address', 'Meta', 'PeerName']],
              },
              search: {
                as: 'filter',
                replace: true,
              },
            },
          },
        },
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
          metadata: {
            _options: { path: '/metadata' },
          },
        },
      },
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
            template: '../edit',
            path: '/create',
            abilities: ['create intentions'],
          },
        },
      },
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
            template: '../index',
            path: '/*key',
          },
        },
        edit: {
          _options: { path: '/*key/edit' },
        },
        create: {
          _options: {
            template: '../edit',
            path: '/*key/create',
            abilities: ['create kvs'],
          },
        },
        'root-create': {
          _options: {
            template: '../edit',
            path: '/create',
            abilities: ['create kvs'],
          },
        },
      },
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
    index: {
      _options: { path: '/' },
      // root index redirects are currently dealt with in application.hbs
    },
    settings: {
      _options: {
        path: '/settings',
      },
    },
    /* This was introduced in 1.12. By the time we get to 1.15 */
    /* I'd say we are safe to remove, feel free to delete for 1.15 */
    setting: {
      _options: {
        path: '/setting',
        redirect: '../settings',
      },
    },
    notfound: {
      _options: { path: '/*notfound' },
    },
  }))(
  (
    json,
    data = typeof document !== 'undefined' ? document.currentScript.dataset : module.exports
  ) => {
    data[`routes`] = JSON.stringify(json);
  }
);
