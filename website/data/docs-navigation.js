// The root folder for this documentation category is `pages/docs`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category, or a "name" property to
//   serve as the category title in the sidebar

export default [
  {
    category: 'intro',
    content: [
      {
        category: 'vs',
        content: [
          'zookeeper',
          'chef-puppet',
          'nagios',
          'skydns',
          'smartstack',
          'serf',
          'eureka',
          'istio',
          'proxies',
          'custom',
        ],
      },
    ],
  },

  {
    category: 'install',
    content: [
      { title: 'Consul Agent', href: '/docs/agent' },
      'glossary',
      'ports',
      'bootstrapping',
      'cloud-auto-join',
      'performance',
      { title: 'Kubernetes', href: '/docs/k8s' },
    ],
  },

  { title: 'API', href: '/api-docs' },

  { title: 'Commands (CLI)', href: '/commands' },

  {
    category: 'discovery',
    name: 'Service Discovery',
    content: ['services', 'dns', 'checks'],
  },

  {
    category: 'connect',
    content: [
      'connect-internals',
      'configuration',
      {
        category: 'config-entries',
        content: [
          'ingress-gateway',
          'proxy-defaults',
          'service-defaults',
          'service-intentions',
          'service-resolver',
          'service-router',
          'service-splitter',
          'terminating-gateway',
        ],
      },
      {
        category: 'proxies',
        content: ['envoy', 'built-in', 'integrate'],
      },
      {
        category: 'registration',
        content: ['service-registration', 'sidecar-service'],
      },
      'intentions',
      'intentions-legacy',
      {
        category: 'observability',
        content: ['ui-visualization'],
      },
      {
        category: 'l7-traffic',
        content: ['discovery-chain'],
      },
      'connectivity-tasks',
      {
        category: 'gateways',
        content: [
          {
            category: 'mesh-gateway',
            content: ['wan-federation-via-mesh-gateways'],
          },

          'ingress-gateway',
          'terminating-gateway',
        ],
      },
      'nomad',
      { title: 'Kubernetes', href: '/docs/k8s/connect' },
      { category: 'native', content: ['go'] },
      {
        category: 'ca',
        content: ['consul', 'vault', 'aws'],
      },
      'dev',
    ],
  },
  {
    category: 'k8s',
    content: [
      {
        category: 'installation',
        name: 'Get Started',
        content: [
          'install',
          {
            category: 'platforms',
            name: 'Platform Guides',
            content: [
              {
                title: 'Minikube',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-minikube?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=mk',
              },
              {
                title: 'Kind',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-kind?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=kind',
              },
              {
                title: 'AKS (Azure)',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-aks-azure?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=aks',
              },
              {
                title: 'EKS (AWS)',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-eks-aws?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=eks',
              },
              {
                title: 'GKE (Google Cloud)',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-gke-google?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=gke',
              },
              {
                title: 'Red Hat OpenShift',
                href:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-openshift-red-hat?utm_source=consul.io&utm_medium=docs&utm_content=k8s&utm_term=openshift',
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
            content: ['kubernetes', 'vms-and-kubernetes'],
          },
        ],
      },
      {
        category: 'connect',
        content: [
          'ingress-gateways',
          'terminating-gateways',
          'connect-ca-provider',
          'ambassador',
          'health',
          { category: 'observability', content: ['metrics'], name: "Observability" },
        ],
      },
      'service-sync',
      {
        category: 'crds',
        content: ['upgrade-to-crds'],
      },
      'dns',
      {
        category: 'upgrade',
        content: ['compatibility'],
      },
      {
        category: 'operations',
        name: 'Operations',
        content: ['uninstall', 'certificate-rotation', 'tls-on-existing-cluster'],
      },
      {
        name: 'Troubleshoot',
        content: [
          {
            title: 'Common Error Messages',
            href:
              '/docs/troubleshoot/common-errors#common-errors-on-kubernetes',
          },
          {
            title: 'FAQ',
            href: '/docs/troubleshoot/faq#consul-on-kubernetes',
          },
        ],
      },
      'helm',
    ],
  },

  {
    category: 'nia',
    content: [
      {
        category: 'installation',
        name: 'Get Started',
        content: ['install', 'requirements', 'configure', 'run'],
      },
      'architecture',
      {
        category: 'api',
        name: 'API',
        content: ['api-overview', 'status', 'tasks'],
      },
      {
        category: 'cli',
        name: 'CLI',
        content: ['cli-overview', 'task'],
      },
      'configuration',
      'tasks',
      'terraform-modules',
      'network-drivers',
      'compatibility',
    ],
  },

  {
    category: 'dynamic-app-config',
    name: 'Dynamic App Configuration',
    content: ['kv', 'sessions', 'watches'],
  },
  {
    category: 'agent',
    content: ['options', 'config-entries', 'telemetry'],
  },
  {
    category: 'security',
    content: [
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
      'encryption',
      {
        category: 'security-models',
        content: ['core', 'nia'],
      },
    ],
  },
  {
    category: 'enterprise',
    content: [
      'audit-logging',
      'backups',
      'upgrades',
      'read-scale',
      {
        title: 'Single sign-on - OIDC',
        href: '/docs/security/acl/auth-methods/oidc',
      },
      'redundancy',
      'federation',
      'namespaces',
      'network-segments',
      'sentinel',
    ],
  },
  {
    category: 'architecture',
    content: ['anti-entropy', 'consensus', 'gossip', 'jepsen', 'coordinates'],
  },
  {
    category: 'integrate',
    name: 'Integrations',
    content: [
      'partnerships',
      'nia-integration',
      {
        title: 'Vault Integration',
        href: '/docs/connect/ca/vault',
      },
      {
        title: 'Ambassador Integration',
        href: '/docs/k8s/connect/ambassador',
      },
      {
        title: 'Proxy Integration',
        href: '/docs/connect/proxies/integrate',
      },
    ],
  },
  'download-tools',
  {
    category: 'upgrading',
    content: [
      'compatibility',
      'upgrade-specific',
      {
        category: 'instructions',
        content: [
          'general-process',
          'upgrade-to-1-2-x',
          'upgrade-to-1-6-x',
          'upgrade-to-1-8-x',
        ],
      },
    ],
  },
  {
    category: 'troubleshoot',
    name: 'Troubleshoot',
    content: ['common-errors', 'faq'],
  },
  {
    category: 'release-notes',
    name: 'Release Notes',
    content: ['1-9-0'],
  },
]
