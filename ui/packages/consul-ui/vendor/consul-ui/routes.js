/**
 * Copyright IBM Corp. 2024, 2026
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
          breadcrumb: { label: 'Overview' },
        },
        serverstatus: {
          _options: {
            path: '/server-status',
            abilities: ['read servers'],
            breadcrumb: { label: 'Overview' },
          },
        },
        cataloghealth: {
          _options: {
            path: '/catalog-health',
            abilities: ['access overview'],
            breadcrumb: { label: 'Overview' },
          },
        },
        license: {
          _options: {
            path: '/license',
            abilities: ['read license'],
            breadcrumb: { label: 'Overview' },
          },
        },
      },
      services: {
        _options: { path: '/services', breadcrumb: { label: 'Services' } },
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
            breadcrumb: { label: 'Services' },
          },
        },
        show: {
          _options: {
            path: '/:name',
            breadcrumb: { label: 'name', parent: 'dc.services' },
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
              breadcrumb: { label: 'Instances', parent: 'dc.services.show' },
            },
          },
          intentions: {
            _options: { path: '/intentions', breadcrumb: { label: 'Intentions', parent: 'dc.services.show' } },
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
                breadcrumb: { label: 'Intentions', parent: 'dc.services.show' },
              },
            },
            edit: {
              _options: { path: '/:intention_id', breadcrumb: { label: 'intention_id', parent: 'dc.services.show.intentions' } },
            },
            create: {
              _options: {
                template: '../edit',
                path: '/create',
                breadcrumb: { label: 'Create', parent: 'dc.services.show.intentions' },
              },
            },
          },
          topology: {
            _options: { path: '/topology', breadcrumb: { label: 'Topology', parent: 'dc.services.show' } },
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
              breadcrumb: { label: 'Services', parent: 'dc.services.show' },
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
              breadcrumb: { label: 'Upstreams', parent: 'dc.services.show' },
            },
          },
          routing: {
            _options: { path: '/routing', breadcrumb: { label: 'Routing', parent: 'dc.services.show' } },
          },
          tags: {
            _options: { path: '/tags', breadcrumb: { label: 'Tags', parent: 'dc.services.show' } },
          },
        },
        instance: {
          _options: {
            path: '/:name/instances/:node/:id',
            redirect: './healthchecks',
            breadcrumb: { show: false },
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
              breadcrumb: { label: 'Health Checks', parent: 'dc.services.show.instances' },
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
              breadcrumb: { label: 'Upstreams', parent: 'dc.services.show.instances' },
            },
          },
          exposedpaths: {
            _options: { path: '/exposed-paths', breadcrumb: { label: 'Exposed Paths', parent: 'dc.services.show.instances' } },
          },
          addresses: {
            _options: { path: '/addresses', breadcrumb: { label: 'Addresses', parent: 'dc.services.show.instances' } },
          },
          metadata: {
            _options: { path: '/metadata', breadcrumb: { label: 'Metadata', parent: 'dc.services.show.instances' } },
          },
        },
        notfound: {
          _options: { path: '/:name/:node/:id', breadcrumb: { show: false } },
        },
      },
      nodes: {
        _options: { path: '/nodes', breadcrumb: { label: 'Nodes' } },
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
            breadcrumb: { label: 'Nodes' },
          },
        },
        show: {
          _options: { path: '/:name', breadcrumb: { label: 'name', parent: 'dc.nodes' } },
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
              breadcrumb: { label: 'Health Checks', parent: 'dc.nodes.show' },
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
              breadcrumb: { label: 'Services', parent: 'dc.nodes.show' },
            },
          },
          rtt: {
            _options: { path: '/round-trip-time', breadcrumb: { label: 'Round Trip Time', parent: 'dc.nodes.show' } },
          },
          metadata: {
            _options: { path: '/metadata', breadcrumb: { label: 'Metadata', parent: 'dc.nodes.show' } },
          },
          sessions: {
            _options: { path: '/lock-sessions', breadcrumb: { label: 'Lock Sessions', parent: 'dc.nodes.show' } },
          },
        },
      },
      peers: {
        _options: {
          path: '/peers',
          breadcrumb: { label: 'Peers' },
        },
        index: {
          _options: {
            path: '/',
            queryParams: {
              sortBy: 'sort',
              state: 'state',
              searchproperty: {
                as: 'searchproperty',
                empty: [['Name', 'ID']],
              },
              search: {
                as: 'filter',
                replace: true,
              },
            },
            breadcrumb: { label: 'Peers' },
          },
        },
        show: {
          _options: {
            path: '/:name',
            breadcrumb: { label: 'name', parent: 'dc.peers' },
          },
          imported: {
            _options: {
              path: '/imported-services',
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
              breadcrumb: { label: 'Imported Services', parent: 'dc.peers.show' },
            },
          },
          exported: {
            _options: {
              path: '/exported-services',
              queryParams: {
                search: {
                  as: 'filter',
                  replace: true,
                },
              },
              breadcrumb: { label: 'Exported Services', parent: 'dc.peers.show' },
            },
          },
          addresses: {
            _options: {
              path: '/addresses',
              breadcrumb: { label: 'Addresses', parent: 'dc.peers.show' },
            },
          },
        },
      },
      intentions: {
        _options: { path: '/intentions', breadcrumb: { label: 'Intentions' } },
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
            breadcrumb: { label: 'Intentions' },
          },
        },
        edit: {
          _options: {
            path: '/:intention_id',
            abilities: ['read intentions'],
            breadcrumb: { label: 'intention_id', parent: 'dc.intentions' },
          },
        },
        create: {
          _options: {
            template: '../edit',
            path: '/create',
            abilities: ['create intentions'],
            breadcrumb: { label: 'Create', parent: 'dc.intentions' },
          },
        },
      },
      kv: {
        _options: { path: '/kv', breadcrumb: { label: 'Key/Value' } },
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
            breadcrumb: { label: 'Key/Value' },
          },
        },
        folder: {
          _options: {
            template: '../index',
            path: '/*key',
            queryParams: {
              sortBy: 'sort',
              kind: 'kind',
              search: {
                as: 'filter',
                replace: true,
              },
            },
            breadcrumb: { label: 'key', parent: 'dc.kv' },
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
          breadcrumb: { show: false },
        },
        policies: {
          _options: {
            path: '/policies',
            abilities: ['read policies'],
            breadcrumb: { label: 'Policies' },
          },
          index: {
            _options: {
              path: '/',
              breadcrumb: { label: 'Policies' },
            },
          },
          edit: {
            _options: { path: '/:id', breadcrumb: { label: 'id', parent: 'dc.acls.policies' } },
          },
          create: {
            _options: {
              path: '/create',
              abilities: ['create policies'],
              breadcrumb: { label: 'Create', parent: 'dc.acls.policies' },
            },
          },
        },
        roles: {
          _options: {
            path: '/roles',
            abilities: ['read roles'],
            breadcrumb: { label: 'Roles' },
          },
          index: {
            _options: {
              path: '/',
              breadcrumb: { label: 'Roles' },
            },
          },
          edit: {
            _options: { path: '/:id', breadcrumb: { label: 'id', parent: 'dc.acls.roles' } },
          },
          create: {
            _options: {
              path: '/create',
              abilities: ['create roles'],
              breadcrumb: { label: 'Create', parent: 'dc.acls.roles' },
            },
          },
        },
        tokens: {
          _options: {
            path: '/tokens',
            abilities: ['access acls', 'read tokens'],
            breadcrumb: { label: 'Tokens' },
          },
          index: {
            _options: {
              path: '/',
              breadcrumb: { label: 'Tokens' },
            },
          },
          edit: {
            _options: { path: '/:id', breadcrumb: { label: 'id', parent: 'dc.acls.tokens' } },
          },
          create: {
            _options: {
              path: '/create',
              abilities: ['create tokens'],
              breadcrumb: { label: 'Create', parent: 'dc.acls.tokens' },
            },
          },
        },
        'auth-methods': {
          _options: {
            path: '/auth-methods',
            abilities: ['read auth-methods'],
            breadcrumb: { label: 'Auth Methods' },
          },
          index: {
            _options: {
              path: '/',
              breadcrumb: { label: 'Auth Methods' },
            },
          },
          show: {
            _options: { path: '/:id', breadcrumb: { label: 'id', parent: 'dc.acls.auth-methods' } },
            'auth-method': {
              _options: { path: '/auth-method', breadcrumb: { label: 'Auth Method', parent: 'dc.acls.auth-methods.show' } },
            },
            'binding-rules': {
              _options: { path: '/binding-rules', breadcrumb: { label: 'Binding Rules', parent: 'dc.acls.auth-methods.show' } },
            },
            'nspace-rules': {
              _options: { path: '/nspace-rules', breadcrumb: { label: 'Namespace Rules', parent: 'dc.acls.auth-methods.show' } },
            },
          },
        },
      },
      partitions: {
        _options: {
          path: '/partitions',
          abilities: ['read partitions'],
          breadcrumb: { label: 'Admin Partitions' },
        },
        index: {
          _options: {
            path: '/',
            queryParams: {
              sortBy: 'sort',
              searchproperty: {
                as: 'searchproperty',
                empty: [['Name', 'Description']],
              },
              search: {
                as: 'filter',
                replace: true,
              },
            },
            breadcrumb: { label: 'Admin Partitions' },
          },
        },
        edit: {
          _options: { path: '/:name', breadcrumb: { label: 'name', parent: 'dc.partitions' } },
        },
        create: {
          _options: {
            template: '../edit',
            path: '/create',
            abilities: ['create partitions'],
            breadcrumb: { label: 'Create', parent: 'dc.partitions' },
          },
        },
      },
      nspaces: {
        _options: {
          path: '/namespaces',
          abilities: ['read nspaces'],
          breadcrumb: { label: 'Namespaces' },
        },
        index: {
          _options: {
            path: '/',
            queryParams: {
              sortBy: 'sort',
              searchproperty: {
                as: 'searchproperty',
                empty: [['Name', 'Description', 'Role', 'Policy']],
              },
              search: {
                as: 'filter',
                replace: true,
              },
            },
            breadcrumb: { label: 'Namespaces' },
          },
        },
        edit: {
          _options: { path: '/:name', breadcrumb: { label: 'name', parent: 'dc.nspaces' } },
        },
        create: {
          _options: {
            template: '../edit',
            path: '/create',
            abilities: ['create nspaces'],
            breadcrumb: { label: 'Create', parent: 'dc.nspaces' },
          },
        },
      },
      'routing-config': {
        _options: { path: '/routing-config/:name', breadcrumb: { label: 'Routing Config', parent: 'dc.services' } },
      },
    },
    index: {
      _options: { path: '/', breadcrumb: { show: false } },
      // root index redirects are currently dealt with in application.hbs
    },
    settings: {
      _options: {
        path: '/settings',
        breadcrumb: { show: false },
      },
    },
    /* This was introduced in 1.12. By the time we get to 1.15 */
    /* I'd say we are safe to remove, feel free to delete for 1.15 */
    setting: {
      _options: {
        path: '/setting',
        redirect: '../settings',
        breadcrumb: { show: false },
      },
    },
    unavailable: {
      _options: { path: '/unavailable', breadcrumb: { show: false } },
    },
    notfound: {
      _options: { path: '/*notfound', breadcrumb: { show: false } },
    },
  }))(
  (
    json,
    data = typeof document !== 'undefined' ? document.currentScript.dataset : module.exports
  ) => {
    data[`routes`] = JSON.stringify(json);
  }
);
