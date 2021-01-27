// REDIRECTS FILE

// See the README file in this directory for documentation. Please do not
// modify or delete existing redirects without first verifying internally.
// Next.js redirect documentation: https://nextjs.org/docs/api-reference/next.config.js/redirects

module.exports = [
  {
    source: '/discovery',
    destination: '/use-cases/service-discovery-and-health-checking',
    permanent: true,
  },
  {
    source: '/mesh',
    destination: '/use-cases/multi-platform-service-mesh',
    permanent: true,
  },
  {
    source: '/segmentation',
    destination: '/use-cases/multi-platform-service-mesh',
    permanent: true,
  },
  {
    source: '/docs/agent/acl-rules',
    destination: '/docs/security/acl/acl-rules',
    permanent: true,
  },
  {
    source: '/docs/acl/acl-rules',
    destination: '/docs/security/acl/acl-rules',
    permanent: true,
  },
  {
    source: '/docs/agent/acl-system',
    destination: '/docs/security/acl/acl-system',
    permanent: true,
  },
  {
    source: '/docs/acl/acl-system',
    destination: '/docs/security/acl/acl-system',
    permanent: true,
  },
  { source: '/docs/agent/http', destination: '/api-docs', permanent: true },
  {
    source: '/docs/guides/acl-legacy',
    destination: '/docs/security/acl/acl-legacy',
    permanent: true,
  },
  {
    source: '/docs/acl/acl-legacy',
    destination: '/docs/security/acl/acl-legacy',
    permanent: true,
  },
  {
    source: '/docs/guides/acl-migrate-tokens',
    destination: '/docs/security/acl/acl-migrate-tokens',
    permanent: true,
  },
  {
    source: '/docs/acl/acl-migrate-tokens',
    destination: '/docs/security/acl/acl-migrate-tokens',
    permanent: true,
  },
  {
    source: '/docs/guides/bootstrapping',
    destination: '/docs/install/bootstrapping',
    permanent: true,
  },
  {
    source: '/docs/guides/sentinel',
    destination: '/docs/agent/sentinel',
    permanent: true,
  },
  {
    source: '/docs/connect/proxies/sidecar-service',
    destination: '/docs/connect/registration/sidecar-service',
    permanent: true,
  },
  {
    source: '/docs/enterprise/connect-multi-datacenter',
    destination: '/docs/enterprise',
    permanent: true,
  },
  { source: '/configuration', destination: '/', permanent: true },
  {
    source: '/docs/connect/mesh(_|-)gateway',
    destination: '/docs/connect/gateways/mesh-gateway',
    permanent: true,
  },
  {
    source: '/docs/connect/ingress(_|-)gateway',
    destination: '/docs/connect/gateways/ingress-gateway',
    permanent: true,
  },
  {
    source: '/docs/connect/terminating(_|-)gateway',
    destination: '/docs/connect/gateways/terminating-gateway',
    permanent: true,
  },
  {
    source: '/docs/k8s/connect/overview',
    destination: '/docs/k8s/connect',
    permanent: true,
  },
  {
    source: '/docs/agent/cloud-auto-join',
    destination: '/docs/install/cloud-auto-join',
    permanent: true,
  },
  {
    source: '/docs/internals/security',
    destination: '/docs/security',
    permanent: true,
  },
  { source: '/docs/acl', destination: '/docs/security/acl', permanent: true },
  {
    source: '/docs/acl/auth-methods',
    destination: '/docs/security/acl/auth-methods',
    permanent: true,
  },
  {
    source: '/docs/acl/auth-methods/kubernetes',
    destination: '/docs/security/acl/auth-methods/kubernetes',
    permanent: true,
  },
  {
    source: '/docs/acl/auth-methods/jwt',
    destination: '/docs/security/acl/auth-methods/jwt',
    permanent: true,
  },
  {
    source: '/docs/acl/auth-methods/oidc',
    destination: '/docs/security/acl/auth-methods/oidc',
    permanent: true,
  },
  {
    source: '/docs/agent/kv',
    destination: '/docs/dynamic-app-config/kv',
    permanent: true,
  },
  {
    source: '/docs/internals/sessions',
    destination: '/docs/dynamic-app-config/sessions',
    permanent: true,
  },
  {
    source: '/docs/agent/watches',
    destination: '/docs/dynamic-app-config/watches',
    permanent: true,
  },
  {
    source: '/docs/connect/l7-traffic-management',
    destination: '/docs/connect/l7-traffic',
    permanent: true,
  },
  {
    source: '/docs/internals/discovery-chain',
    destination: '/docs/connect/l7-traffic/discovery-chain',
    permanent: true,
  },
  {
    source: '/docs/k8s/operations/upgrading',
    destination: '/docs/k8s/upgrade',
    permanent: true,
  },
  {
    source: '/docs/k8s/operations/uninstalling',
    destination: '/docs/k8s/uninstall',
    permanent: true,
  },
  {
    source: '/docs/k8s/operations/tls-on-existing-cluster',
    destination: '/docs/k8s/tls-on-existing-cluster',
    permanent: true,
  },
  {
    source: '/docs/agent/services',
    destination: '/docs/discovery/services',
    permanent: true,
  },
  {
    source: '/docs/agent/checks',
    destination: '/docs/discovery/checks',
    permanent: true,
  },
  {
    source: '/docs/agent/dns',
    destination: '/docs/discovery/dns',
    permanent: true,
  },
  {
    source: '/docs/agent/encryption',
    destination: '/docs/security/encryption',
    permanent: true,
  },
  {
    source: '/docs/internals/architecture',
    destination: '/docs/architecture',
    permanent: true,
  },
  {
    source: '/docs/internals/anti-entropy',
    destination: '/docs/architecture/anti-entropy',
    permanent: true,
  },
  {
    source: '/docs/internals/consensus',
    destination: '/docs/architecture/consensus',
    permanent: true,
  },
  {
    source: '/docs/internals/gossip',
    destination: '/docs/architecture/gossip',
    permanent: true,
  },
  {
    source: '/docs/internals/jepsen',
    destination: '/docs/architecture/jepsen',
    permanent: true,
  },
  {
    source: '/docs/internals/coordinates',
    destination: '/docs/architecture/coordinates',
    permanent: true,
  },
  {
    source: '/docs/glossary',
    destination: '/docs/install/glossary',
    permanent: true,
  },
  {
    source: '/docs/connect/gateways/mesh-gateways',
    destination: '/docs/connect/gateways/mesh-gateway',
    permanent: true,
  },
  {
    source: '/docs/faq',
    destination: '/docs/troubleshoot/faq',
    permanent: true,
  },
  {
    source: '/docs/common-errors',
    destination: '/docs/troubleshoot/common-errors',
    permanent: true,
  },
  { source: '/intro', destination: '/docs/intro', permanent: true },
  { source: '/intro/vs', destination: '/docs/intro/vs', permanent: true },
  {
    source: '/intro/vs/zookeeper',
    destination: '/docs/intro/vs/zookeeper',
    permanent: true,
  },
  {
    source: '/intro/vs/chef-puppet',
    destination: '/docs/intro/vs/chef-puppet',
    permanent: true,
  },
  {
    source: '/intro/vs/nagios-sensu',
    destination: '/docs/intro/vs/nagios-sensu',
    permanent: true,
  },
  {
    source: '/intro/vs/skydns',
    destination: '/docs/intro/vs/skydns',
    permanent: true,
  },
  {
    source: '/intro/vs/smartstack',
    destination: '/docs/intro/vs/smartstack',
    permanent: true,
  },
  {
    source: '/intro/vs/serf',
    destination: '/docs/intro/vs/serf',
    permanent: true,
  },
  {
    source: '/intro/vs/eureka',
    destination: '/docs/intro/vs/eureka',
    permanent: true,
  },
  {
    source: '/intro/vs/istio',
    destination: '/docs/intro/vs/istio',
    permanent: true,
  },
  {
    source: '/intro/vs/proxies',
    destination: '/docs/intro/vs/proxies',
    permanent: true,
  },
  {
    source: '/intro/vs/custom',
    destination: '/docs/intro/vs/custom',
    permanent: true,
  },
  {
    source: '/download-tools',
    destination: '/docs/download-tools',
    permanent: true,
  },
  {
    source: '/downloads_tools',
    destination: '/docs/download-tools',
    permanent: true,
  },
  {
    source: '/download_tools',
    destination: '/docs/download-tools',
    permanent: true,
  },
  {
    source: '/downloads_tools',
    destination: '/docs/download-tools',
    permanent: true,
  },
  {
    source: '/docs/k8s/ambassador',
    destination: '/docs/k8s/connect/ambassador',
    permanent: true,
  },
  {
    source: '/docs/k8s/installation/overview',
    destination: '/docs/k8s/installation/install',
    permanent: true,
  },
  {
    source: '/docs/k8s/installation/muti-cluster/overview',
    destination: '/docs/k8s/installation/multi-cluster',
    permanent: true,
  },
  {
    source: '/docs/partnerships',
    destination: '/docs/integrate/partnerships',
    permanent: true,
  },
  {
    source: '/docs/connect/gateways/wan-federation-via-mesh-gateways',
    destination:
      '/docs/connect/gateways/mesh-gateway/wan-federation-via-mesh-gateways',
    permanent: true,
  },
  {
    source: '/docs/agent/http/:path*',
    destination: '/api/:path*',
    permanent: true,
  },
  { source: '/docs/agent/http', destination: '/api', permanent: true },
  // CLI Redirects
  { source: '/docs/commands', destination: '/commands', permanent: true },
  {
    source: '/docs/commands/acl',
    destination: '/commands/acl',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method',
    destination: '/commands/acl/auth-method',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method/create',
    destination: '/commands/acl/auth-method/create',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method/delete',
    destination: '/commands/acl/auth-method/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method/list',
    destination: '/commands/acl/auth-method/list',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method/read',
    destination: '/commands/acl/auth-method/read',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/auth-method/update',
    destination: '/commands/acl/auth-method/update',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule',
    destination: '/commands/acl/binding-rule',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule/create',
    destination: '/commands/acl/binding-rule/create',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule/delete',
    destination: '/commands/acl/binding-rule/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule/list',
    destination: '/commands/acl/binding-rule/list',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule/read',
    destination: '/commands/acl/binding-rule/read',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/binding-rule/update',
    destination: '/commands/acl/binding-rule/update',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/bootstrap',
    destination: '/commands/acl/bootstrap',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy',
    destination: '/commands/acl/policy',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy/create',
    destination: '/commands/acl/policy/create',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy/delete',
    destination: '/commands/acl/policy/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy/list',
    destination: '/commands/acl/policy/list',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy/read',
    destination: '/commands/acl/policy/read',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/policy/update',
    destination: '/commands/acl/policy/update',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/set-agent-token',
    destination: '/commands/acl/set-agent-token',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token',
    destination: '/commands/acl/token',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/clone',
    destination: '/commands/acl/token/clone',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/create',
    destination: '/commands/acl/token/create',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/delete',
    destination: '/commands/acl/token/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/list',
    destination: '/commands/acl/token/list',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/read',
    destination: '/commands/acl/token/read',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/token/update',
    destination: '/commands/acl/token/update',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/translate-rules',
    destination: '/commands/acl/translate-rules',
    permanent: true,
  },
  {
    source: '/docs/commands/agent',
    destination: '/commands/agent',
    permanent: true,
  },
  {
    source: '/docs/commands/catalog',
    destination: '/commands/catalog',
    permanent: true,
  },
  {
    source: '/docs/commands/catalog/datacenters',
    destination: '/commands/catalog/datacenters',
    permanent: true,
  },
  {
    source: '/docs/commands/catalog/nodes',
    destination: '/commands/catalog/nodes',
    permanent: true,
  },
  {
    source: '/docs/commands/catalog/services',
    destination: '/commands/catalog/services',
    permanent: true,
  },
  {
    source: '/docs/commands/config',
    destination: '/commands/config',
    permanent: true,
  },
  {
    source: '/docs/commands/config/delete',
    destination: '/commands/config/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/config/list',
    destination: '/commands/config/list',
    permanent: true,
  },
  {
    source: '/docs/commands/config/read',
    destination: '/commands/config/read',
    permanent: true,
  },
  {
    source: '/docs/commands/config/write',
    destination: '/commands/config/write',
    permanent: true,
  },
  {
    source: '/docs/commands/connect',
    destination: '/commands/connect',
    permanent: true,
  },
  {
    source: '/docs/commands/connect/ca',
    destination: '/commands/connect/ca',
    permanent: true,
  },
  {
    source: '/docs/commands/connect/proxy',
    destination: '/commands/connect/proxy',
    permanent: true,
  },
  {
    source: '/docs/commands/connect/envoy',
    destination: '/commands/connect/envoy',
    permanent: true,
  },
  {
    source: '/docs/commands/connect/expose',
    destination: '/commands/connect/expose',
    permanent: true,
  },
  {
    source: '/docs/commands/debug',
    destination: '/commands/debug',
    permanent: true,
  },
  {
    source: '/docs/commands/event',
    destination: '/commands/event',
    permanent: true,
  },
  {
    source: '/docs/commands/exec',
    destination: '/commands/exec',
    permanent: true,
  },
  {
    source: '/docs/commands/force-leave',
    destination: '/commands/force-leave',
    permanent: true,
  },
  {
    source: '/docs/commands/info',
    destination: '/commands/info',
    permanent: true,
  },
  {
    source: '/docs/commands/intention',
    destination: '/commands/intention',
    permanent: true,
  },
  {
    source: '/docs/commands/intention/check',
    destination: '/commands/intention/check',
    permanent: true,
  },
  {
    source: '/docs/commands/intention/create',
    destination: '/commands/intention/create',
    permanent: true,
  },
  {
    source: '/docs/commands/intention/delete',
    destination: '/commands/intention/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/intention/get',
    destination: '/commands/intention/get',
    permanent: true,
  },
  {
    source: '/docs/commands/intention/match',
    destination: '/commands/intention/match',
    permanent: true,
  },
  {
    source: '/docs/commands/join',
    destination: '/commands/join',
    permanent: true,
  },
  {
    source: '/docs/commands/keygen',
    destination: '/commands/keygen',
    permanent: true,
  },
  {
    source: '/docs/commands/keyring',
    destination: '/commands/keyring',
    permanent: true,
  },
  { source: '/docs/commands/kv', destination: '/commands/kv', permanent: true },
  {
    source: '/docs/commands/kv/delete',
    destination: '/commands/kv/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/kv/export',
    destination: '/commands/kv/export',
    permanent: true,
  },
  {
    source: '/docs/commands/kv/get',
    destination: '/commands/kv/get',
    permanent: true,
  },
  {
    source: '/docs/commands/kv/import',
    destination: '/commands/kv/import',
    permanent: true,
  },
  {
    source: '/docs/commands/kv/put',
    destination: '/commands/kv/put',
    permanent: true,
  },
  {
    source: '/docs/commands/leave',
    destination: '/commands/leave',
    permanent: true,
  },
  {
    source: '/docs/commands/license',
    destination: '/commands/license',
    permanent: true,
  },
  {
    source: '/docs/commands/lock',
    destination: '/commands/lock',
    permanent: true,
  },
  {
    source: '/docs/commands/login',
    destination: '/commands/login',
    permanent: true,
  },
  {
    source: '/docs/commands/logout',
    destination: '/commands/logout',
    permanent: true,
  },
  {
    source: '/docs/commands/maint',
    destination: '/commands/maint',
    permanent: true,
  },
  {
    source: '/docs/commands/members',
    destination: '/commands/members',
    permanent: true,
  },
  {
    source: '/docs/commands/monitor',
    destination: '/commands/monitor',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace',
    destination: '/commands/namespace',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/create',
    destination: '/commands/namespace/create',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/delete',
    destination: '/commands/namespace/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/list',
    destination: '/commands/namespace/list',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/read',
    destination: '/commands/namespace/read',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/update',
    destination: '/commands/namespace/update',
    permanent: true,
  },
  {
    source: '/docs/commands/namespace/write',
    destination: '/commands/namespace/write',
    permanent: true,
  },
  {
    source: '/docs/commands/operator',
    destination: '/commands/operator',
    permanent: true,
  },
  {
    source: '/docs/commands/operator/area',
    destination: '/commands/operator/area',
    permanent: true,
  },
  {
    source: '/docs/commands/operator/autopilot',
    destination: '/commands/operator/autopilot',
    permanent: true,
  },
  {
    source: '/docs/commands/operator/raft',
    destination: '/commands/operator/raft',
    permanent: true,
  },
  {
    source: '/docs/commands/reload',
    destination: '/commands/reload',
    permanent: true,
  },
  {
    source: '/docs/commands/rft',
    destination: '/commands/rft',
    permanent: true,
  },
  {
    source: '/docs/commands/rtt',
    destination: '/commands/rtt',
    permanent: true,
  },
  {
    source: '/docs/commands/services',
    destination: '/commands/services',
    permanent: true,
  },
  {
    source: '/docs/commands/services/register',
    destination: '/commands/services/register',
    permanent: true,
  },
  {
    source: '/docs/commands/services/deregister',
    destination: '/commands/services/deregister',
    permanent: true,
  },
  {
    source: '/docs/commands/snapshot',
    destination: '/commands/snapshot',
    permanent: true,
  },
  {
    source: '/docs/commands/snapshot/agent',
    destination: '/commands/snapshot/agent',
    permanent: true,
  },
  {
    source: '/docs/commands/snapshot/inspect',
    destination: '/commands/snapshot/inspect',
    permanent: true,
  },
  {
    source: '/docs/commands/snapshot/restore',
    destination: '/commands/snapshot/restore',
    permanent: true,
  },
  {
    source: '/docs/commands/snapshot/save',
    destination: '/commands/snapshot/save',
    permanent: true,
  },
  {
    source: '/docs/commands/tls',
    destination: '/commands/tls',
    permanent: true,
  },
  {
    source: '/docs/commands/tls/ca',
    destination: '/commands/tls/ca',
    permanent: true,
  },
  {
    source: '/docs/commands/tls/cert',
    destination: '/commands/tls/cert',
    permanent: true,
  },
  {
    source: '/docs/commands/validate',
    destination: '/commands/validate',
    permanent: true,
  },
  {
    source: '/docs/commands/version',
    destination: '/commands/version',
    permanent: true,
  },
  {
    source: '/docs/commands/watch',
    destination: '/commands/watch',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/acl-bootstrap',
    destination: '/commands/acl/bootstrap',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/acl-policy',
    destination: '/commands/acl/policy',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/acl-set-agent-token',
    destination: '/commands/acl/set-agent-token',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/acl-token',
    destination: '/commands/acl/token',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/acl-translate-rules',
    destination: '/commands/acl/translate-rules',
    permanent: true,
  },
  // Learn Redirects
  {
    source: '/docs/guides/acl',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/production-acls',
    permanent: true,
  },
  {
    source: '/docs/guides/agent-encryption',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/agent-encryption',
    permanent: true,
  },
  {
    source: '/docs/guides/autopilot',
    destination:
      'https://learn.hashicorp.com/consul/day-2-operations/autopilot',
    permanent: true,
  },
  {
    source: '/docs/guides/backup',
    destination: 'https://learn.hashicorp.com/consul/datacenter-deploy/backup',
    permanent: true,
  },
  {
    source: '/docs/guides/cluster-monitoring-metrics',
    destination:
      'https://learn.hashicorp.com/consul/day-2-operations/monitoring',
    permanent: true,
  },
  {
    source: '/docs/guides/creating-certificates',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/certificates',
    permanent: true,
  },
  {
    source: '/docs/guides/deployment-guide',
    destination:
      'https://learn.hashicorp.com/consul/datacenter-deploy/deployment-guide',
    permanent: true,
  },
  {
    source: '/docs/guides/deployment',
    destination:
      'https://learn.hashicorp.com/consul/datacenter-deploy/reference-architecture',
    permanent: true,
  },
  {
    source: '/docs/guides/dns-cache',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/dns-caching',
    permanent: true,
  },
  {
    source: '/docs/guides/minikube',
    destination:
      'https://learn.hashicorp.com/consul/getting-started-k8s/minikube',
    permanent: true,
  },
  {
    source: '/docs/guides/connect-production',
    destination:
      'https://learn.hashicorp.com/consul/developer-segmentation/connect-production',
    permanent: true,
  },
  {
    source: '/docs/guides/connect-envoy',
    destination:
      'https://learn.hashicorp.com/consul/developer-segmentation/connect-envoy',
    permanent: true,
  },
  {
    source: '/docs/guides/consul-template',
    destination:
      'https://learn.hashicorp.com/consul/developer-configuration/consul-template',
    permanent: true,
  },
  {
    source: '/docs/guides/consul-aws',
    destination:
      'https://learn.hashicorp.com/consul/cloud-integrations/consul-aws',
    permanent: true,
  },
  {
    source: '/docs/guides/forwarding',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/forwarding',
    permanent: true,
  },
  {
    source: '/docs/guides/external',
    destination:
      'https://learn.hashicorp.com/consul/developer-discovery/external',
    permanent: true,
  },
  {
    source: '/docs/guides/advanced-federation',
    destination:
      'https://learn.hashicorp.com/consul/day-2-operations/advanced-federation',
    permanent: true,
  },
  {
    source: '/docs/guides/datacenters',
    destination:
      'https://learn.hashicorp.com/consul/security-networking/datacenters',
    permanent: true,
  },
  {
    source: '/docs/guides/geo-failover',
    destination:
      'https://learn.hashicorp.com/consul/developer-discovery/geo-failover',
    permanent: true,
  },
  {
    source: '/docs/guides/leader-election',
    destination:
      'https://learn.hashicorp.com/consul/developer-configuration/elections',
    permanent: true,
  },
  {
    source: '/docs/guides/monitoring-telegraf',
    destination: 'https://learn.hashicorp.com/consul/integrations/telegraf',
    permanent: true,
  },
  {
    source: '/docs/guides/network-segments',
    destination:
      'https://learn.hashicorp.com/consul/day-2-operations/network-segments',
    permanent: true,
  },
  {
    source: '/docs/guides/semaphore',
    destination:
      'https://learn.hashicorp.com/consul/developer-configuration/semaphore',
    permanent: true,
  },
  {
    source: '/docs/guides/windows-guide',
    destination: 'https://learn.hashicorp.com/consul/datacenter-deploy/windows',
    permanent: true,
  },
  {
    source: '/docs/guides/consul-containers',
    destination: 'https://hub.docker.com/_/consul',
    permanent: true,
  },
  {
    source: '/docs/guides/kubernetes-reference',
    destination:
      'https://learn.hashicorp.com/consul/day-1-operations/kubernetes-reference',
    permanent: true,
  },
  {
    source: '/docs/guides/outage',
    destination: 'https://learn.hashicorp.com/consul/day-2-operations/outage',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/minikube',
    destination: 'https://learn.hashicorp.com/consul/kubernetes/minikube',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/aks',
    destination: 'https://learn.hashicorp.com/consul/kubernetes/azure-k8s',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/eks',
    destination: 'https://learn.hashicorp.com/consul/kubernetes/aws-k8s',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/gke',
    destination:
      'https://learn.hashicorp.com/consul/kubernetes/google-cloud-k8s',
    permanent: true,
  },
  {
    source: '/intro/getting-started',
    destination:
      'https://learn.hashicorp.com/consul?track=getting-started#getting-started',
    permanent: true,
  },
  {
    source: '/intro/getting-started/:path*',
    destination: 'https://learn.hashicorp.com/consul/getting-started/:path*',
    permanent: true,
  },
  // Replatforming redirects
  { source: '/guides', destination: '/docs/guides', permanent: true },
  { source: '/api/acl/acl', destination: '/api-docs/acl', permanent: true },
  {
    source: '/api-docs/features',
    destination: '/api-docs/features/consistency',
    permanent: true,
  },
  {
    source: '/docs/upgrade-specific',
    destination: '/docs/upgrading/upgrade-specific',
    permanent: true,
  },
  {
    source: '/docs/compatibility',
    destination: '/docs/upgrading/compatibility',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/role/create',
    destination: '/commands/acl/role/create',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/role/delete',
    destination: '/commands/acl/role/delete',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/role/list',
    destination: '/commands/acl/role/list',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/role/read',
    destination: '/commands/acl/role/read',
    permanent: true,
  },
  {
    source: '/docs/commands/acl/role/update',
    destination: '/commands/acl/role/update',
    permanent: true,
  },
  { source: '/docs/agent/basics', destination: '/docs/agent', permanent: true },
  {
    source: '/docs/agent/config_entries',
    destination: '/docs/agent/config-entries',
    permanent: true,
  },
  {
    source: '/docs/acl/acl-auth-methods',
    destination: '/docs/acl/auth-methods',
    permanent: true,
  },
  {
    source: '/docs/connect/platform/nomad',
    destination: '/docs/connect/nomad',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/run',
    destination: '/docs/k8s/installation/install',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/consul-enterprise',
    destination:
      '/docs/k8s/installation/deployment-configurations/consul-enterprise',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/clients-outside-kubernetes',
    destination:
      '/docs/k8s/installation/deployment-configurations/clients-outside-kubernetes',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/servers-outside-kubernetes',
    destination:
      '/docs/k8s/installation/deployment-configurations/servers-outside-kubernetes',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/predefined-pvcs',
    destination: '/docs/k8s/installation/platforms/self-hosted-kubernetes',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/operations',
    destination: '/docs/k8s/operations',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/upgrading',
    destination: '/docs/k8s/operations/upgrading',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/tls-on-existing-cluster',
    destination: '/docs/k8s/operations/tls-on-existing-cluster',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/uninstalling',
    destination: '/docs/k8s/operations/upgrading',
    permanent: true,
  },
  {
    source: '/docs/platform/k8s/:path*',
    destination: '/docs/k8s/:path*',
    permanent: true,
  },
  {
    source: '/docs/nia/installation/configuration',
    destination: '/docs/nia/configuration',
    permanent: true,
  },
  {
    source: '/use-cases/network-middleware-automation',
    destination: '/use-cases/network-infrastructure-automation',
    permanent: true,
  },
  {
    source: '/(/downloads?[-_]tools)',
    destination: '/docs/download-tools',
    permanent: true,
  },
  {
    source: '/docs/k8s/ambassador',
    destination: '/docs/k8s/connect/ambassador',
    permanent: true,
  },
  {
    source: '/docs/agent/config-entries/:path',
    destination: '/docs/connect/config-entries/:path*',
    permanent: true,
  },
  {
    source: '/docs/partnerships',
    destination: '/docs/integrate/partnerships',
    permanent: true,
  },
  {
    source: '/docs/k8s/installation/overview',
    destination: '/docs/k8s/installation/install',
    permanent: true,
  },
  {
    source: '/docs/k8s/installation/multi-cluster/overview',
    destination: '/docs/k8s/installation/multi-cluster',
    permanent: true,
  },
  // disallow '.html' or '/index.html' in favor of cleaner, simpler paths
  { source: '/:path*/index', destination: '/:path*', permanent: true },
  { source: '/:path*.html', destination: '/:path*', permanent: true },
]
