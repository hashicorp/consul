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
  {
    source: '/consul/docs/ecs/terraform/install',
    destination: '/consul/docs/ecs/deploy/terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/terraform/secure-configuration',
    destination: '/consul/docs/ecs/deploy/terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/terraform/migrate-existing-tasks',
    destination: '/consul/docs/ecs/deploy/migrate-existing-tasks',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/manual/install',
    destination: '/consul/docs/ecs/deploy/manual',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/manual/secure-configuration',
    destination: '/consul/docs/ecs/deploy/manual',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/manual/acl-controller',
    destination: '/consul/docs/ecs/deploy/manual',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/task-resource-usage',
    destination: '/consul/docs/ecs/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/requirements',
    destination: '/consul/docs/ecs/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/configuration-reference',
    destination: '/consul/docs/ecs/reference/configuration-reference',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/compatibility',
    destination: '/consul/docs/ecs/reference/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/usage',
    destination:
      '/consul/docs/connect/gateways/api-gateway/deploy/listeners-vms',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway',
    destination: '/consul/docs/connect/gateways/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/install',
    destination: '/consul/docs/connect/gateways/api-gateway/install-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/usage/reroute-http-requests',
    destination:
      '/consul/docs/connect/gateways/api-gateway/define-routes/reroute-http-requests',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/usage/route-to-peered-services',
    destination:
      '/consul/docs/connect/gateways/api-gateway/define-routes/route-to-peered-services',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/usage/errors',
    destination: '/consul/docs/connect/gateways/api-gateway/errors',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/usage/usage',
    destination:
      '/consul/docs/connect/gateways/api-gateway/deploy/listeners-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/upgrades',
    destination: '/consul/docs/connect/gateways/api-gateway/upgrades-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/configuration',
    destination: '/consul/docs/connect/gateways/api-gateway/configuration',
    permanent: true,
  },
  {
    source: '/consul/docs/api-gateway/configuration/:slug',
    destination:
      '/consul/docs/connect/gateways/api-gateway/configuration/:slug',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/failover',
    destination: '/consul/docs/connect/manage-traffic/failover',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/l7',
    destination: '/consul/docs/connect/manage-traffic',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/l7/:slug',
    destination: '/consul/docs/connect/manage-traffic/:slug',
    permanent: true,
  },
  {
    source: '/consul/docs/v1.8.x/connect/config-entries/:slug',
    destination: '/consul/docs/v1.8.x/agent/config-entries/:slug',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/catalog/v1/:slug',
    destination: '/consul/docs/architecture/catalog/:slug',
    permanent: true,
  },
  {
    source: '/consul/docs/:version(v1\.(?:8|9|10|11|12|13|14|15|16|17)\.x)/architecture/catalog/:slug*',
    destination: '/consul/docs/:version/architecture/catalog/v1/:slug',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/terraform-cloud',
    destination: '/consul/docs/nia/network-drivers/hcp-terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/:version(v1\.(?:8|9|10|11|12|13|14|15|16|17)\.x)/nia/network-drivers/hcp-terraform',
    destination: '/consul/docs/:version/nia/network-drivers/terraform-cloud',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/:slug',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/v2/:slug*',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/commands/resource/:slug',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns',
    destination: '/consul/docs/k8s/dns/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/:version(v1\.(?:11|12|13|14|15|16|17|18)\.x)/k8s/dns/enable',
    destination: '/consul/docs/:version/k8s/dns',
    permanent: true,
  },
  {
    source: '/consul/api-docs/hcp-link',
    destination: '/hcp/docs/consul/concepts/consul-central',
    permanent: true,
  },
]
