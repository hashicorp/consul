// The root folder for this documentation category is `pages/docs`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category

export default [
  { category: 'install', content: ['ports', 'bootstrapping', 'performance'] },
  {
    category: 'upgrading', // todo: this folder didn't exist before
    content: ['compatibility', 'upgrade-specific'],
  },
  'glossary',
  {
    category: 'internals',
    content: [
      'architecture',
      'consensus',
      'gossip',
      'coordinates',
      'sessions',
      'anti-entropy',
      'security',
      'jepsen',
      'discovery-chain',
    ],
  },
  {
    category: 'commands',
    content: [
      {
        category: 'acl',
        content: [
          {
            category: 'auth-method', // these all had leafs converted to index files
            content: ['create', 'delete', 'list', 'read', 'update'],
          },
          {
            category: 'binding-rule',
            content: ['create', 'delete', 'list', 'read', 'update'],
          },
          'bootstrap',
          {
            category: 'policy',
            content: ['create', 'delete', 'list', 'read', 'update'],
          },
          {
            category: 'role',
            content: ['create', 'delete', 'list', 'read', 'update'],
          },
          'set-agent-token',
          {
            category: 'token',
            content: ['clone', 'create', 'delete', 'list', 'read', 'update'],
          },
          'translate-rules',
        ],
      },
      'agent',
      { category: 'catalog', content: ['datacenters', 'nodes', 'services'] }, // leaf to index
      { category: 'config', content: ['delete', 'list', 'read', 'write'] }, // leaf to index
      { category: 'connect', content: ['ca', 'proxy', 'envoy'] }, // leaf to index
      'debug',
      'event',
      'exec',
      'force-leave',
      'info',
      {
        category: 'intention', // leaf to index
        content: ['check', 'create', 'delete', 'get', 'match'],
      },
      'join',
      'keygen',
      'keyring',
      {
        category: 'kv', // leaf to index
        content: ['delete', 'export', 'get', 'import', 'put'],
      },
      'leave',
      'license',
      'lock',
      'login',
      'logout',
      'maint',
      'members',
      'monitor',
      {
        category: 'namespace', // leaf to index
        content: ['create', 'delete', 'list', 'read', 'update', 'write'],
      },
      {
        category: 'operator', // leaf to index
        content: ['area', 'autopilot', 'raft'],
      },
      'reload',
      'rtt',
      { category: 'services', content: ['register', 'deregister'] }, // leaf to index
      {
        category: 'snapshot', // leaf to index
        content: ['agent', 'inspect', 'restore', 'save'],
      },
      { category: 'tls', content: ['ca', 'cert'] }, // leaf to index
      'validate',
      'version',
      'watch',
    ],
  },
  {
    category: 'agent', // index was formerly "basics"
    content: [
      'dns',
      'options',
      {
        category: 'config-entries', // index was formerly config_entries
        content: [
          'service-router',
          'service-splitter',
          'service-resolver',
          'service-defaults',
          'proxy-defaults',
        ],
      },
      'cloud-auto-join',
      'services',
      'checks',
      'kv',
      'sentinel',
      'encryption',
      'telemetry',
      'watches',
    ],
  },
  {
    category: 'acl',
    content: [
      'acl-system',
      'acl-rules',
      'acl-legacy',
      'acl-migrate-tokens',
      { category: 'auth-methods', content: ['kubernetes'] }, // index was formerly 'acl-auth-methods'
    ],
  },
  {
    category: 'connect',
    content: [
      'configuration',
      'observability',
      'l7-traffic-management',
      'intentions',
      'connect-internals',
      {
        category: 'proxies', // index was formerly 'proxies'
        content: ['envoy', 'built-in', 'integrate'],
      },
      'mesh_gateway',
      {
        category: 'registration', // index was formerly 'registration'
        content: ['service-registration', 'sidecar-service'],
      },
      'security',
      {
        category: 'ca', // index was formerly 'ca'
        content: ['consul', 'vault', 'aws'],
      },
      { category: 'native', content: ['go'] }, // index was formerly 'native'
      'dev',
      'nomad', // todo - redirect, was /platform/nomad
      // todo - 'connect' was removed as an alias
    ],
  },
  {
    category: 'k8s', // formerly was nested inside /platform
    content: [
      {
        category: 'installation', // todo: these folders didn't exist before
        content: [
          // todo: the index here was formerly 'run'
          'aks',
          'eks',
          'gke',
          'minikube',
          'consul-enterprise',
          'clients-outside-kubernetes',
          'servers-outside-kubernetes',
          'predefined-pvcs',
        ],
      },
      {
        category: 'operations', // index was formerly 'operations'
        content: ['upgrading', 'tls-on-existing-cluster', 'uninstalling'],
      },
      'dns',
      'service-sync',
      'connect',
      'ambassador',
      'helm',
    ],
  },
  '-------',
  { category: 'guides', content: [] },
  'common-errors',
  'faq',
  '--------',
  'partnerships', // todo: add leaf redirects
  {
    category: 'enterprise',
    content: [
      'backups', // todo: add leaf redirects
      'upgrades',
      'read-scale',
      'redundancy',
      'federation',
      'network-segments',
      'namespaces',
      'sentinel',
    ],
  },
]
