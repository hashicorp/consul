// The root folder for this documentation category is `pages/docs`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category, or a "name" property to
//   serve as the category title in the sidebar

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
      { category: 'connect', content: ['ca', 'proxy', 'envoy', 'expose'] },
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
          'ingress-gateway',
          'proxy-defaults',
          'service-defaults',
          'service-resolver',
          'service-router',
          'service-splitter',
          'terminating-gateway',
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
      { category: 'auth-methods', content: ['kubernetes', 'jwt', 'oidc'] },
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
      'mesh-gateway',
      'wan-federation-via-mesh-gateways',
      'ingress-gateway',
      'terminating-gateway',
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
      { title: 'Kubernetes', href: '/docs/k8s/connect' },
    ],
  },
  {
    category: 'k8s',
    content: [
      {
        name: 'Installation',
        category: 'installation',
        content: [
          'overview',
          {
            category: 'platforms',
            name: 'Platform Guides',
            content: [
              {
                title: 'Minikube',
                href:
                  'https://learn.hashicorp.com/consul/kubernetes/minikube?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=mk',
              },
              {
                title: 'AKS (Azure)',
                href:
                  'https://learn.hashicorp.com/consul/kubernetes/azure-k8s?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=aks',
              },
              {
                title: 'EKS (AWS)',
                href:
                  'https://learn.hashicorp.com/consul/kubernetes/aws-k8s?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=eks',
              },
              {
                title: 'GKE (Google Cloud)',
                href:
                  'https://learn.hashicorp.com/consul/kubernetes/google-cloud-k8s?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=gke',
              },
              'self-hosted-kubernetes',
            ],
          },
          {
            category: 'deployment-configurations',
            name: 'Deployment Configurations',
            content: [
              'clients-outside-kubernetes',
              'servers-outside-kubernetes',
              'consul-enterprise',
            ],
          },
          {
            category: 'multi-cluster',
            name: 'Multi-Cluster Federation',
            content: ['overview', 'kubernetes', 'vms-and-kubernetes'],
          },
        ],
      },
      {
        category: 'operations',
        name: 'Operations',
        content: ['upgrading', 'tls-on-existing-cluster', 'uninstalling'],
      },
      {
        category: 'connect',
        name: 'Connect Service Mesh',
        content: ['overview', 'ingress-gateways', 'terminating-gateways'],
      },
      'service-sync',
      'dns',
      'ambassador',
      'helm',
    ],
  },
  '-------',
  'common-errors',
  'faq',
  '--------',
  'partnerships',
  {
    category: 'enterprise',
    content: [
      'audit-logging',
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
