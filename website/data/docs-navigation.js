// The root folder for this documentation category is `pages/docs`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category

export default [
  { category: 'install', content: ['ports', 'bootstrapping', 'performance'] },
  {
    category: 'upgrading',
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
            category: 'auth-method',
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
      { category: 'catalog', content: ['datacenters', 'nodes', 'services'] },
      { category: 'config', content: ['delete', 'list', 'read', 'write'] },
      { category: 'connect', content: ['ca', 'proxy', 'envoy'] },
      'debug',
      'event',
      'exec',
      'force-leave',
      'info',
      {
        category: 'intention',
        content: ['check', 'create', 'delete', 'get', 'match'],
      },
      'join',
      'keygen',
      'keyring',
      {
        category: 'kv',
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
        category: 'namespace',
        content: ['create', 'delete', 'list', 'read', 'update', 'write'],
      },
      {
        category: 'operator',
        content: ['area', 'autopilot', 'raft'],
      },
      'reload',
      'rtt',
      { category: 'services', content: ['register', 'deregister'] },
      {
        category: 'snapshot',
        content: ['agent', 'inspect', 'restore', 'save'],
      },
      { category: 'tls', content: ['ca', 'cert'] },
      'validate',
      'version',
      'watch',
    ],
  },
  {
    category: 'agent',
    content: [
      'dns',
      'options',
      {
        category: 'config-entries',
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
      { category: 'auth-methods', content: ['kubernetes'] },
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
        category: 'proxies',
        content: ['envoy', 'built-in', 'integrate'],
      },
      'mesh_gateway',
      {
        category: 'registration',
        content: ['service-registration', 'sidecar-service'],
      },
      'security',
      {
        category: 'ca',
        content: ['consul', 'vault', 'aws'],
      },
      { category: 'native', content: ['go'] },
      'dev',
      'nomad',
      // TODO: 'k8s/connect' was removed as an alias here
    ],
  },
  {
    category: 'k8s',
    content: [
      {
        category: 'installation',
        content: [
          'aks',
          'gke',
          'minikube',
          'consul-enterprise',
          'clients-outside-kubernetes',
          'servers-outside-kubernetes',
          'predefined-pvcs',
        ],
      },
      {
        category: 'operations',
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
  'partnerships',
  {
    category: 'enterprise',
    content: [
      'backups',
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
