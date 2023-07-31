// REDIRECTS FILE

// See the README file in this directory for documentation. Please do not
// modify or delete existing redirects without first verifying internally.
// Next.js redirect documentation: https://nextjs.org/docs/api-reference/next.config.js/redirects

module.exports = [
  {
    source: '/consul/docs/connect/cluster-peering/create-manage-peering',
    destination:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/usage/establish-peering',
    destination:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/k8s',
    destination: '/consul/docs/k8s/connect/cluster-peering/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions#intention-management-permissions',
    destination: `/consul/docs/connect/intentions/create-manage-intentions#acl-requirements`,
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions#intention-basics',
    destination: `/consul/docs/connect/intentions`,
    permanent: true,
  },
  {
    source: '/consul/docs/v1.16.x/connect/transparent-proxy',
    destination: '/consul/docs/v1.16.x/k8s/connect/transparent-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/1.16.x/agent/limits/init-rate-limits',
    destination: '/consul/docs/1.16.x/agent/limits/usage/init-rate-limits',
    permanent: true,
  },
  {
    source: '/consul/docs/1.16.x/agent/limits/set-global-traffic-rate-limits',
    destination:
      '/consul/docs/1.16.x/agent/limits/usage/set-global-traffic-rate-limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-peers',
    destination:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    permanent: true,
  },
]
