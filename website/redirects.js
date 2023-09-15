/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    source: '/consul/docs/connect/transparent-proxy',
    destination: '/consul/docs/k8s/connect/transparent-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/init-rate-limits',
    destination: '/consul/docs/agent/limits/usage/init-rate-limits',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/set-global-traffic-rate-limits',
    destination:
      '/consul/docs/agent/limits/usage/set-global-traffic-rate-limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-peers',
    destination:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/sentinel',
    destination:
      '/consul/docs/dynamic-app-config/kv#using-sentinel-to-apply-policies-for-consul-kv',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-datacenters',
    destination: '/consul/docs/k8s/deployment-configurations/multi-cluster',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/registration/service-registration',
    destination: '/consul/docs/connect/proxies/proxy-config-reference',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/registration',
    destination: '/consul/docs/connect/proxies/',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/registration/sidecar-service',
    destination: '/consul/docs/connect/proxies/deploy-sidecar-services',
    permanent: true,
  },
]
