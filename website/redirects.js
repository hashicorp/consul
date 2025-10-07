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
    destination: '/consul/docs/connect/proxies',
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
    source: '/consul/docs/connect/l7/:slug*',
    destination: '/consul/docs/connect/manage-traffic/:slug*',
    permanent: true,
  },
  {
    source: '/consul/docs/v1.8.x/connect/config-entries/:slug*',
    destination: '/consul/docs/v1.8.x/agent/config-entries/:slug*',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/catalog/v1/:slug*',
    destination: '/consul/docs/architecture/catalog/:slug*',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/architecture/catalog/:slug*',
    destination: '/consul/docs/:version/architecture/catalog/v1/:slug*',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/terraform-cloud',
    destination: '/consul/docs/nia/network-drivers/hcp-terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/nia/network-drivers/hcp-terraform',
    destination: '/consul/docs/:version/nia/network-drivers/terraform-cloud',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/:slug*',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/v2/:slug*',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/commands/resource/:slug*',
    destination: '/consul/docs/architecture/catalog#v2-catalog',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns',
    destination: '/consul/docs/k8s/dns/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:11|12|13|14|15|16|17|18).x)/k8s/dns/enable',
    destination: '/consul/docs/:version/k8s/dns',
    permanent: true,
  },
  {
    source: '/consul/api-docs/hcp-link',
    destination: '/hcp/docs/consul/concepts/consul-central',
    permanent: true,
  },
  ///////////////////////////////////
  // BEGIN new Consul IA redirects //
  ///////////////////////////////////
  ///////////////////////////////////
  //    Old path ---> New path     //
  ///////////////////////////////////

  {
    source: '/consul/docs/agent',
    destination: '/consul/docs/fundamentals/agent',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/config',
    destination: '/consul/docs/fundamentals/agent',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/config-entries',
    destination: '/consul/docs/fundamentals/config-entry',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/config/cli-flags',
    destination: '/consul/commands/agent',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/config/config-files',
    destination: '/consul/docs/reference/agent/configuration-file',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits',
    destination: '/consul/docs/manage/rate-limit',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/init-rate-limits',
    destination: '/consul/docs/manage/rate-limit/initialize',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/limit-request-rates-from-ips',
    destination: '/consul/docs/manage/rate-limit/source',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/monitor-rate-limits',
    destination: '/consul/docs/manage/rate-limit/monitor',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/set-global-traffic-rate-limits',
    destination: '/consul/docs/manage/rate-limit/global',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/monitor/alerts',
    destination: '/consul/docs/monitor/alerts',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/monitor/components',
    destination: '/consul/docs/monitor',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/monitor/telemetry',
    destination: '/consul/docs/monitor/telemetry/agent',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/sentinel',
    destination: '/consul/docs/secure/acl/sentinel',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore',
    destination: '/consul/docs/deploy/server/wal',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore/enable',
    destination: '/consul/docs/deploy/server/wal',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore/monitoring',
    destination: '/consul/docs/deploy/server/wal/monitor-raft',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore/revert-to-boltdb',
    destination: '/consul/docs/deploy/server/wal/revert-boltdb',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture',
    destination: '/consul/docs/architecture/control-plane',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/anti-entropy',
    destination: '/consul/docs/concept/consistency',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/capacity-planning',
    destination: '/consul/docs/reference/architecture/capacity',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/catalog',
    destination: '/consul/docs/concept/catalog',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/consensus',
    destination: '/consul/docs/concept/consensus',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/gossip',
    destination: '/consul/docs/concept/gossip',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/improving-consul-resilience',
    destination: '/consul/docs/concept/reliability',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/jepsen',
    destination: '/consul/docs/concept/consistency',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/scale',
    destination: '/consul/docs/manage/scale',
    permanent: true,
  },
  {
    source: '/consul/docs/concepts/service-discovery',
    destination: '/consul/docs/use-case/service-discovery',
    permanent: true,
  },
  {
    source: '/consul/docs/concepts/service-mesh',
    destination: '/consul/docs/use-case/service-mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/ca',
    destination: '/consul/docs/secure-mesh/certificate',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/ca/aws',
    destination: '/consul/docs/secure-mesh/certificate/acm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/ca/consul',
    destination: '/consul/docs/secure-mesh/certificate/built-in',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/ca/vault',
    destination: '/consul/docs/secure-mesh/certificate/vault',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering',
    destination: '/consul/docs/east-west/cluster-peering',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/tech-specs',
    destination: '/consul/docs/east-west/cluster-peering/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/usage/create-sameness-groups',
    destination: '/consul/docs/multi-tenant/sameness-group/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    destination: '/consul/docs/east-west/cluster-peering/establish/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/usage/manage-connections',
    destination: '/consul/docs/east-west/cluster-peering/manage/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/cluster-peering/usage/peering-traffic-management',
    destination: '/consul/docs/manage-traffic/cluster-peering/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries',
    destination: '/consul/docs/fundamentals/config-entry',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/api-gateway',
    destination: '/consul/docs/reference/config-entry/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/control-plane-request-limit',
    destination:
      '/consul/docs/reference/config-entry/control-plane-request-limit',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/exported-services',
    destination: '/consul/docs/reference/config-entry/exported-services',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/file-system-certificate',
    destination: '/consul/docs/reference/config-entry/file-system-certificate',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/http-route',
    destination: '/consul/docs/reference/config-entry/http-route',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/ingress-gateway',
    destination: '/consul/docs/reference/config-entry/ingress-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/inline-certificate',
    destination: '/consul/docs/reference/config-entry/inline-certificate',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/jwt-provider',
    destination: '/consul/docs/reference/config-entry/jwt-provider',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/mesh',
    destination: '/consul/docs/reference/config-entry/mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/proxy-defaults',
    destination: '/consul/docs/reference/config-entry/proxy-defaults',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/registration',
    destination: '/consul/docs/reference/config-entry/registration',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/sameness-group',
    destination: '/consul/docs/reference/config-entry/sameness-group',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/service-defaults',
    destination: '/consul/docs/reference/config-entry/service-defaults',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/service-intentions',
    destination: '/consul/docs/reference/config-entry/service-intentions',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/service-resolver',
    destination: '/consul/docs/reference/config-entry/service-resolver',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/service-router',
    destination: '/consul/docs/reference/config-entry/service-router',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/service-splitter',
    destination: '/consul/docs/reference/config-entry/service-splitter',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/tcp-route',
    destination: '/consul/docs/reference/config-entry/tcp-route',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/terminating-gateway',
    destination: '/consul/docs/reference/config-entry/terminating-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/connect-internals',
    destination: '/consul/docs/architecture/data-plane/connect',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/connectivity-tasks',
    destination: '/consul/docs/architecture/data-plane/gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/dataplane',
    destination: '/consul/docs/architecture/control-plane/dataplane',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/dataplane/consul-dataplane',
    destination: '/consul/docs/reference/dataplane/cli',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/dataplane/telemetry',
    destination: '/consul/docs/reference/dataplane/telemetry',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/dev',
    destination: '/consul/docs/troubleshoot/mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/distributed-tracing',
    destination: '/consul/docs/observe/distributed-tracing',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways',
    destination: '/consul/docs/architecture/data-plane/gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway',
    destination: '/consul/docs/north-south/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration',
    destination: '/consul/docs/north-south/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration/gateway',
    destination: '/consul/docs/reference/k8s/api-gateway/gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/gatewayclass',
    destination: '/consul/docs/reference/k8s/api-gateway/gatewayclass',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/gatewayclassconfig',
    destination: '/consul/docs/reference/k8s/api-gateway/gatewayclassconfig',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/gatewaypolicy',
    destination: '/consul/docs/reference/k8s/api-gateway/gatewaypolicy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/meshservice',
    destination: '/consul/docs/reference/k8s/api-gateway/meshservice',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/routeauthfilter',
    destination: '/consul/docs/reference/k8s/api-gateway/routeauthfilter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/routeretryfilter',
    destination: '/consul/docs/reference/k8s/api-gateway/routeretryfilter',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration/routes',
    destination: '/consul/docs/reference/k8s/api-gateway/routes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/routetimeoutfilter',
    destination: '/consul/docs/reference/k8s/api-gateway/routetimeoutfilter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/define-routes/reroute-http-requests',
    destination: '/consul/docs/north-south/api-gateway/k8s/reroute',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/define-routes/route-to-peered-services',
    destination: '/consul/docs/north-south/api-gateway/k8s/peer',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/define-routes/routes-k8s',
    destination: '/consul/docs/north-south/api-gateway/k8s/route',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/define-routes/routes-vms',
    destination: '/consul/docs/north-south/api-gateway/vm/route',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/deploy/listeners-k8s',
    destination: '/consul/docs/north-south/api-gateway/k8s/listener',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/deploy/listeners-vms',
    destination: '/consul/docs/north-south/api-gateway/vm/listener',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/errors',
    destination: '/consul/docs/error-messages/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/install-k8s',
    destination: '/consul/docs/north-south/api-gateway/k8s/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/secure-traffic/encrypt-vms',
    destination: '/consul/docs/north-south/api-gateway/secure-traffic/encrypt',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/secure-traffic/verify-jwts-k8s',
    destination: '/consul/docs/north-south/api-gateway/secure-traffic/jwt/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/secure-traffic/verify-jwts-vms',
    destination: '/consul/docs/north-south/api-gateway/secure-traffic/jwt/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/tech-specs',
    destination: '/consul/docs/north-south/api-gateway/k8s/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/upgrades-k8s',
    destination: '/consul/docs/upgrade/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/ingress-gateway',
    destination: '/consul/docs/north-south/ingress-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/ingress-gateway/tls-external-service',
    destination: '/consul/docs/north-south/ingress-gateway/external',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/ingress-gateway/usage',
    destination: '/consul/docs/north-south/ingress-gateway/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/mesh-gateway',
    destination: '/consul/docs/east-west/mesh-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/peering-via-mesh-gateways',
    destination: '/consul/docs/east-west/mesh-gateway/cluster-peer',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-partitions',
    destination: '/consul/docs/east-west/mesh-gateway/admin-partition',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-wan-datacenters',
    destination: '/consul/docs/east-west/mesh-gateway/federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/wan-federation-via-mesh-gateways',
    destination: '/consul/docs/east-west/mesh-gateway/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/terminating-gateway',
    destination: '/consul/docs/north-south/terminating-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions',
    destination: '/consul/docs/secure-mesh/intention',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions/create-manage-intentions',
    destination: '/consul/docs/secure-mesh/intention/create',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions/jwt-authorization',
    destination: '/consul/docs/secure-mesh/intention/jwt',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions/legacy',
    destination: '/consul/docs/secure-mesh/intention',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic',
    destination: '/consul/docs/manage-traffic',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/discovery-chain',
    destination: '/consul/docs/manage-traffic/discovery-chain',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/failover',
    destination: '/consul/docs/manage-traffic/failover',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/failover/sameness',
    destination: '/consul/docs/manage-traffic/failover/sameness-group',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/fault-injection',
    destination: '/consul/docs/troubleshoot/fault-injection',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/limit-request-rates',
    destination: '/consul/docs/manage-traffic/rate-limit',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/manage-traffic/route-to-local-upstreams',
    destination: '/consul/docs/manage-traffic/route-local',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/native',
    destination: '/consul/docs/automate/native',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/native/go',
    destination: '/consul/docs/automate/native/go',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/observability',
    destination: '/consul/docs/observe/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/observability/access-logs',
    destination: '/consul/docs/observe/access-log',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/observability/grafanadashboards',
    destination: '/consul/docs/observe/grafana',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/observability/grafanadashboards/consuldataplanedashboard',
    destination: '/consul/docs/observe/grafana/dataplane',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/observability/grafanadashboards/consulk8sdashboard',
    destination: '/consul/docs/observe/grafana/consul-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/observability/grafanadashboards/consulserverdashboard',
    destination: '/consul/docs/observe/grafana/server',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/observability/grafanadashboards/service-to-servicedashboard',
    destination: '/consul/docs/observe/grafana/service-to-service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/observability/grafanadashboards/servicedashboard',
    destination: '/consul/docs/observe/grafana/service',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/observability/service',
    destination: '/consul/docs/observe/grafana/service-to-service',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/observability/ui-visualization',
    destination: '/consul/docs/observe/telemetry/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies',
    destination: '/consul/docs/connect/proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/built-in',
    destination: '/consul/docs/reference/proxy/built-in',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-service-mesh-proxies',
    destination: '/consul/docs/connect/proxy/mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-sidecar-services',
    destination: '/consul/docs/connect/proxy/sidecar',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy',
    destination: '/consul/docs/reference/proxy/envoy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions',
    destination: '/consul/docs/envoy-extension',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/configuration/ext-authz',
    destination: '/consul/docs/reference/proxy/extensions/ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/configuration/otel-access-logging',
    destination: '/consul/docs/reference/proxy/extensions/otel',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/configuration/property-override',
    destination: '/consul/docs/reference/proxy/extensions/property-override',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/configuration/wasm',
    destination: '/consul/docs/reference/proxy/extensions/wasm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/apigee-ext-authz',
    destination: '/consul/docs/envoy-extension/apigee',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/ext-authz',
    destination: '/consul/docs/envoy-extension/ext',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/lambda',
    destination: '/consul/docs/envoy-extension/lambda',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/lua',
    destination: '/consul/docs/envoy-extension/lua',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/otel-access-logging',
    destination: '/consul/docs/envoy-extension/otel-access-logging',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/property-override',
    destination: '/consul/docs/envoy-extension/property-override',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/wasm',
    destination: '/consul/docs/envoy-extension/wasm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/integrate',
    destination: '/consul/docs/connect/proxy/custom',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/proxy-config-reference',
    destination: '/consul/docs/reference/proxy/connect-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/security',
    destination: '/consul/docs/secure-mesh/best-practice',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other',
    destination: '/consul/docs/use-case',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/api-gateway-compare',
    destination: '/consul/docs/use-case/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/config-management-compare',
    destination: '/consul/docs/use-case/config-management',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/dns-tools-compare',
    destination: '/consul/docs/use-case/dns',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/service-mesh-compare',
    destination: '/consul/docs/use-case/service-mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/dynamic-app-config/kv',
    destination: '/consul/docs/automate/kv',
    permanent: true,
  },
  {
    source: '/consul/docs/dynamic-app-config/kv/store',
    destination: '/consul/docs/automate/kv/store',
    permanent: true,
  },
  {
    source: '/consul/docs/dynamic-app-config/sessions',
    destination: '/consul/docs/automate/session',
    permanent: true,
  },
  {
    source:
      '/consul/docs/dynamic-app-config/sessions/application-leader-election',
    destination: '/consul/docs/automate/application-leader-election',
    permanent: true,
  },
  {
    source: '/consul/docs/dynamic-app-config/watches',
    destination: '/consul/docs/automate/watch',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/architecture',
    destination: '/consul/docs/reference/architecture/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/bind-addresses',
    destination: '/consul/docs/register/service/ecs/task-bind-address',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/configure-routes',
    destination: '/consul/docs/connect/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/manual',
    destination: '/consul/docs/register/service/ecs/manual',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/migrate-existing-tasks',
    destination: '/consul/docs/register/service/ecs/migrate',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/terraform',
    destination: '/consul/docs/register/service/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/enterprise',
    destination: '/consul/docs/enterprise/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/reference/compatibility',
    destination: '/consul/docs/upgrade/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/reference/configuration-reference',
    destination: '/consul/docs/reference/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/reference/consul-server-json',
    destination: '/consul/docs/reference/ecs/server-json',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/tech-specs',
    destination: '/consul/docs/reference/ecs/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/upgrade-to-dataplanes',
    destination: '/consul/docs/upgrade/ecs/dataplane',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/admin-partitions',
    destination: '/consul/docs/multi-tenant/admin-partition',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/audit-logging',
    destination: '/consul/docs/monitor/log/audit',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/backups',
    destination: '/consul/docs/manage/scale/automated-backup',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/ent-to-ce-downgrades',
    destination: '/consul/docs/enterprise/downgrade',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/federation',
    destination: '/consul/docs/east-west/network-area',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/fips',
    destination: '/consul/docs/deploy/server/fips',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/license/overview',
    destination: '/consul/docs/enterprise/license',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/license/utilization-reporting',
    destination: '/consul/docs/enterprise/license/reporting',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/long-term-support',
    destination: '/consul/docs/upgrade/lts',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/namespaces',
    destination: '/consul/docs/multi-tenant/namespace',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/network-segments/create-network-segment',
    destination: '/consul/docs/multi-tenant/network-segment/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/enterprise/network-segments/network-segments-overview',
    destination: '/consul/docs/multi-tenant/network-segment',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/read-scale',
    destination: '/consul/docs/manage/scale/read-replica',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/redundancy',
    destination: '/consul/docs/manage/scale/redundancy-zone',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/upgrades',
    destination: '/consul/docs/upgrade/automated',
    permanent: true,
  },
  {
    source: '/consul/docs/guides',
    destination: '/consul/tutorials',
    permanent: true,
  },
  {
    source: '/consul/docs/install',
    destination: '/consul/docs/fundamentals/install',
    permanent: true,
  },
  {
    source: '/consul/docs/install/bootstrapping',
    destination: '/consul/docs/deploy/server/vm/bootstrap',
    permanent: true,
  },
  {
    source: '/consul/docs/install/cloud-auto-join',
    destination: '/consul/docs/deploy/server/cloud-auto-join',
    permanent: true,
  },
  {
    source: '/consul/docs/install/glossary',
    destination: '/consul/docs/glossary',
    permanent: true,
  },
  {
    source: '/consul/docs/install/manual-bootstrap',
    destination: '/consul/docs/deploy/server/vm/bootstrap',
    permanent: true,
  },
  {
    source: '/consul/docs/install/performance',
    destination: '/consul/docs/reference/architecture/server',
    permanent: true,
  },
  {
    source: '/consul/docs/install/ports',
    destination: '/consul/docs/reference/architecture/ports',
    permanent: true,
  },
  {
    source: '/consul/docs/integrate/download-tools',
    destination: '/consul/docs/integrate/consul-tools',
    permanent: true,
  },
  {
    source: '/consul/docs/integrate/nia-integration',
    destination: '/consul/docs/integrate/nia',
    permanent: true,
  },
  {
    source: '/consul/docs/integrate/partnerships',
    destination: '/consul/docs/integrate/consul',
    permanent: true,
  },
  {
    source: '/consul/docs/internals',
    destination: '/consul/docs/architecture',
    permanent: true,
  },
  {
    source: '/consul/docs/internals/acl',
    destination: '/consul/docs/secure/acl',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/annotations-and-labels',
    destination: '/consul/docs/reference/k8s/annotation-label',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/architecture',
    destination: '/consul/docs/architecture/control-plane/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/compatibility',
    destination: '/consul/docs/upgrade/k8s/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect',
    destination: '/consul/docs/connect/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/tech-specs',
    destination: '/consul/docs/east-west/cluster-peering/tech-specs/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/connect/cluster-peering/usage/create-sameness-groups',
    destination: '/consul/docs/multi-tenant/sameness-group/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/establish-peering',
    destination: '/consul/docs/east-west/cluster-peering/establish/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/l7-traffic',
    destination: '/consul/docs/manage-traffic/cluster-peering/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/manage-peering',
    destination: '/consul/docs/east-west/cluster-peering/manage/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/connect-ca-provider',
    destination: '/consul/docs/secure-mesh/certificate/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/health',
    destination: '/consul/docs/register/health-check/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/ingress-controllers',
    destination: '/consul/docs/north-south/ingress-controller',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/ingress-gateways',
    destination: '/consul/docs/north-south/ingress-gateway/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/observability/metrics',
    destination: '/consul/docs/observe/telemetry/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/onboarding-tproxy-mode',
    destination: '/consul/docs/register/service/k8s/transparent-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/terminating-gateways',
    destination: '/consul/docs/register/external/terminating-gateway/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/transparent-proxy',
    destination: '/consul/docs/connect/proxy/transparent-proxy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/connect/transparent-proxy/enable-transparent-proxy',
    destination: '/consul/docs/connect/proxy/transparent-proxy/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/crds',
    destination: '/consul/docs/fundamentals/config-entry',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/crds/upgrade-to-crds',
    destination: '/consul/docs/fundamentals/config-entry',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/argo-rollouts-configuration',
    destination: '/consul/docs/manage-traffic/progressive-rollouts/argo',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/clients-outside-kubernetes',
    destination: '/consul/docs/register/service/k8s/external',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/consul-enterprise',
    destination: '/consul/docs/deploy/server/k8s/enterprise',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/datadog',
    destination: '/consul/docs/monitor/datadog',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/external-service',
    destination: '/consul/docs/register/external/esm/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/multi-cluster',
    destination: '/consul/docs/east-west/wan-federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/kubernetes',
    destination: '/consul/docs/east-west/wan-federation/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes',
    destination: '/consul/docs/east-west/wan-federation/k8s-vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/servers-outside-kubernetes',
    destination: '/consul/docs/deploy/server/k8s/external',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/single-dc-multi-k8s',
    destination: '/consul/docs/deploy/server/k8s/multi-cluster',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault',
    destination: '/consul/docs/deploy/server/k8s/vault',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault/data-integration',
    destination: '/consul/docs/deploy/server/k8s/vault/data',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/bootstrap-token',
    destination: '/consul/docs/deploy/server/k8s/vault/data/bootstrap-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/enterprise-license',
    destination: '/consul/docs/deploy/server/k8s/vault/data/enterprise-license',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/gossip',
    destination: '/consul/docs/deploy/server/k8s/vault/data/gossip-key',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/partition-token',
    destination: '/consul/docs/deploy/server/k8s/vault/data/partition-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/replication-token',
    destination: '/consul/docs/deploy/server/k8s/vault/data/replication-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/server-tls',
    destination: '/consul/docs/deploy/server/k8s/vault/data/tls-certificate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/snapshot-agent-config',
    destination: '/consul/docs/deploy/server/k8s/vault/data/snapshot-agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/webhook-certs',
    destination:
      '/consul/docs/deploy/server/k8s/vault/data/webhook-certificate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/systems-integration',
    destination: '/consul/docs/deploy/server/k8s/vault/backend',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault/wan-federation',
    destination: '/consul/docs/east-west/wan-federation/vault-backend',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns/enable',
    destination: '/consul/docs/manage/dns/forwarding/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns/views',
    destination: '/consul/docs/manage/dns/views',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns/views/enable',
    destination: '/consul/docs/manage/dns/views/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/helm',
    destination: '/consul/docs/reference/k8s/helm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/installation/install',
    destination: '/consul/docs/deploy/server/k8s/helm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/installation/install-cli',
    destination: '/consul/docs/reference/cli/consul-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/k8s-cli',
    destination: '/consul/docs/reference/cli/consul-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/l7-traffic/failover-tproxy',
    destination: '/consul/docs/manage-traffic/failover/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/l7-traffic/route-to-virtual-services',
    destination: '/consul/docs/manage-traffic/virtual-service',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/certificate-rotation',
    destination: '/consul/docs/secure/encryption/tls/rotate/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/gossip-encryption-key-rotation',
    destination: '/consul/docs/secure/encryption/gossip/rotate/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/tls-on-existing-cluster',
    destination: '/consul/docs/secure/encryption/tls/rotate/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/uninstall',
    destination: '/consul/docs/upgrade/k8s/uninstall',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/platforms/self-hosted-kubernetes',
    destination: '/consul/docs/deploy/server/k8s/platform/self-hosted',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/service-sync',
    destination: '/consul/docs/register/service/k8s/service-sync',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/upgrade',
    destination: '/consul/docs/upgrade/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/upgrade/upgrade-cli',
    destination: '/consul/docs/upgrade/k8s/consul-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda',
    destination: '/consul/docs/connect/lambda',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/invocation',
    destination: '/consul/docs/connect/lambda/function',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/invoke-from-lambda',
    destination: '/consul/docs/connect/lambda/service',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/registration',
    destination: '/consul/docs/register/service/lambda',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/registration/automate',
    destination: '/consul/docs/register/service/lambda/automatic',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/registration/manual',
    destination: '/consul/docs/register/service/lambda/manual',
    permanent: true,
  },
  {
    source: '/consul/docs/nia',
    destination: '/consul/docs/automate/infrastructure',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api',
    destination: '/consul/docs/reference/cts/api',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api/health',
    destination: '/consul/docs/reference/cts/api/health',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api/status',
    destination: '/consul/docs/reference/cts/api/status',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api/tasks',
    destination: '/consul/docs/reference/cts/api/tasks',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/architecture',
    destination: '/consul/docs/architecture/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli',
    destination: '/consul/docs/reference/cli/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli/start',
    destination: '/consul/docs/reference/cli/cts/start',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli/task',
    destination: '/consul/docs/reference/cli/cts/task',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/compatibility',
    destination: '/consul/docs/reference/cts/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/configuration',
    destination: '/consul/docs/automate/infrastructure/configure',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/enterprise',
    destination: '/consul/docs/enterprise/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/enterprise/license',
    destination: '/consul/docs/enterprise/license/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/installation/configure',
    destination: '/consul/docs/automate/infrastructure/configure',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/installation/install',
    destination: '/consul/docs/automate/infrastructure/install',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers',
    destination: '/consul/docs/automate/infrastructure/network-driver',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/hcp-terraform',
    destination:
      '/consul/docs/automate/infrastructure/network-driver/hcp-terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/terraform',
    destination:
      '/consul/docs/automate/infrastructure/network-driver/terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/tasks',
    destination: '/consul/docs/automate/infrastructure/task',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/terraform-modules',
    destination: '/consul/docs/automate/infrastructure/module',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/errors-ref',
    destination: '/consul/docs/error-messages/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/requirements',
    destination: '/consul/docs/automate/infrastructure/requirements',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/run',
    destination: '/consul/docs/automate/infrastructure/run',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/run-ha',
    destination: '/consul/docs/automate/infrastructure/high-availability',
    permanent: true,
  },

  {
    source: '/consul/docs/security',
    destination: '/consul/docs/secure',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl',
    destination: '/consul/docs/secure/acl',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-federated-datacenters',
    destination: '/consul/docs/secure/acl/token/federation',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-policies',
    destination: '/consul/docs/secure/acl/policy',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-roles',
    destination: '/consul/docs/secure/acl/role',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-rules',
    destination: '/consul/docs/reference/acl/rule',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods',
    destination: '/consul/docs/secure/acl/auth-method',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/aws-iam',
    destination: '/consul/docs/secure/acl/auth-method/aws',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/jwt',
    destination: '/consul/docs/secure/acl/auth-method/jwt',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/kubernetes',
    destination: '/consul/docs/secure/acl/auth-method/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/oidc',
    destination: '/consul/docs/secure/acl/auth-method/oidc',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens',
    destination: '/consul/docs/secure/acl/token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-consul-esm-token',
    destination: '/consul/docs/secure/acl/token/esm',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-dns-token',
    destination: '/consul/docs/secure/acl/token/dns',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-mesh-gateway-token',
    destination: '/consul/docs/secure/acl/token/mesh-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-replication-token',
    destination: '/consul/docs/secure/acl/token/replication',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-service-token',
    destination: '/consul/docs/secure/acl/token/service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-snapshot-agent-token',
    destination: '/consul/docs/secure/acl/token/snapshot-agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-terminating-gateway-token',
    destination: '/consul/docs/secure/acl/token/terminating-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-token-for-vault-consul-storage',
    destination: '/consul/docs/secure/acl/token/vault-backend',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-ui-token',
    destination: '/consul/docs/secure/acl/token/ui',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-an-agent-token',
    destination: '/consul/docs/secure/acl/token/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-an-ingress-gateway-token',
    destination: '/consul/docs/secure/acl/token/ingress-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/security/encryption',
    destination: '/consul/docs/secure/encryption',
    permanent: true,
  },
  {
    source: '/consul/docs/security/encryption/gossip',
    destination: '/consul/docs/secure/encryption/gossip/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/security/encryption/mtls',
    destination: '/consul/docs/secure/encryption/tls/mtls',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models',
    destination: '/consul/docs/secure/security-model',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models/core',
    destination: '/consul/docs/secure/security-model/core',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models/nia',
    destination: '/consul/docs/secure/security-model/cts',
    permanent: true,
  },
  {
    source:
      '/consul/docs/services/configuration/checks-configuration-reference',
    destination: '/consul/docs/reference/service/health-check',
    permanent: true,
  },
  {
    source:
      '/consul/docs/services/configuration/services-configuration-overview',
    destination: '/consul/docs/fundamentals/service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/services/configuration/services-configuration-reference',
    destination: '/consul/docs/reference/service',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-cache',
    destination: '/consul/docs/discover/dns/scale',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-configuration',
    destination: '/consul/docs/discover/dns/configure',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-dynamic-lookups',
    destination: '/consul/docs/discover/service/dynamic',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-forwarding',
    destination: '/consul/docs/manage/dns/forwarding',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-forwarding/enable',
    destination: '/consul/docs/manage/dns/forwarding/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-overview',
    destination: '/consul/docs/discover/dns',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-static-lookups',
    destination: '/consul/docs/discover/service/static',
    permanent: true,
  },
  {
    source: '/consul/docs/services/services',
    destination: '/consul/docs/fundamentals/service',
    permanent: true,
  },
  {
    source: '/consul/docs/services/usage/checks',
    destination: '/consul/docs/register/health-check/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/services/usage/define-services',
    destination: '/consul/docs/register/service/vm/define',
    permanent: true,
  },
  {
    source: '/consul/docs/services/usage/register-services-checks',
    destination: '/consul/docs/register/service/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/troubleshoot/common-errors',
    destination: '/consul/docs/error-messages/consul',
    permanent: true,
  },

  {
    source: '/consul/docs/troubleshoot/troubleshoot-services',
    destination: '/consul/docs/troubleshoot/service-communication',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading',
    destination: '/consul/docs/upgrade',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/compatibility',
    destination: '/consul/docs/upgrade/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions',
    destination: '/consul/docs/upgrade/instructions',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/general-process',
    destination: '/consul/docs/upgrade/instructions/general',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-10-x',
    destination: '/consul/docs/upgrade/instructions/upgrade-to-1-10-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-2-x',
    destination: '/consul/docs/upgrade/instructions/upgrade-to-1-2-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-6-x',
    destination: '/consul/docs/upgrade/instructions/upgrade-to-1-6-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-8-x',
    destination: '/consul/docs/upgrade/instructions/upgrade-to-1-8-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/upgrade-specific',
    destination: '/consul/docs/upgrade/version-specific',
    permanent: true,
  },
  ///////////////////////////////////
  // Versioned docs compatibility  //
  ///////////////////////////////////
  {
    source:
      '/consul/docs/:version(v1\\.(?:11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/agent',
    destination: '/consul/docs/:version/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/config-entry',
    destination: '/consul/docs/:version/agent/config-entries',
    permanent: true,
  },
  {
    source:
      '/consul/commands/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/agent',
    destination: '/consul/docs/:version/agent/config/cli-flags',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/agent/configuration-file',
    destination: '/consul/docs/:version/agent/config/config-files',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/rate-limit',
    destination: '/consul/docs/:version/agent/limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/rate-limit/initialize',
    destination: '/consul/docs/:version/agent/limits/usage/init-rate-limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/rate-limit/source',
    destination:
      '/consul/docs/:version/agent/limits/usage/limit-request-rates-from-ips',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/rate-limit/monitor',
    destination: '/consul/docs/:version/agent/limits/usage/monitor-rate-limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/rate-limit/global',
    destination:
      '/consul/docs/:version/agent/limits/usage/set-global-traffic-rate-limits',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/alerts',
    destination: '/consul/docs/:version/agent/monitor/alerts',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor',
    destination: '/consul/docs/:version/agent/monitor/components',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/telemetry/agent',
    destination: '/consul/docs/:version/agent/monitor/telemetry',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/sentinel',
    destination: '/consul/docs/:version/agent/sentinel',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/wal',
    destination: '/consul/docs/:version/agent/wal-logstore',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/wal',
    destination: '/consul/docs/:version/agent/wal-logstore/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/wal/monitor-raft',
    destination: '/consul/docs/:version/agent/wal-logstore/monitoring',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/wal/revert-boltdb',
    destination: '/consul/docs/:version/agent/wal-logstore/revert-to-boltdb',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/control-plane',
    destination: '/consul/docs/:version/architecture',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/consistency',
    destination: '/consul/docs/:version/architecture/anti-entropy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/architecture/capacity',
    destination: '/consul/docs/:version/architecture/capacity-planning',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/catalog',
    destination: '/consul/docs/:version/architecture/catalog',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/consensus',
    destination: '/consul/docs/:version/architecture/consensus',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/gossip',
    destination: '/consul/docs/:version/architecture/gossip',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/reliability',
    destination:
      '/consul/docs/:version/architecture/improving-consul-resilience',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/concept/consistency',
    destination: '/consul/docs/:version/architecture/jepsen',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/scale',
    destination: '/consul/docs/:version/architecture/scale',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/service-discovery',
    destination: '/consul/docs/:version/concepts/service-discovery',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/service-mesh',
    destination: '/consul/docs/:version/concepts/service-mesh',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate',
    destination: '/consul/docs/:version/connect/ca',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/acm',
    destination: '/consul/docs/:version/connect/ca/aws',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/built-in',
    destination: '/consul/docs/:version/connect/ca/consul',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/vault',
    destination: '/consul/docs/:version/connect/ca/vault',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering',
    destination: '/consul/docs/:version/connect/cluster-peering',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/tech-specs',
    destination: '/consul/docs/:version/connect/cluster-peering/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/sameness-group/vm',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/create-sameness-groups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/establish/vm',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/establish-cluster-peering',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/manage/vm',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/manage-connections',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/cluster-peering/vm',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/peering-traffic-management',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/config-entry',
    destination: '/consul/docs/:version/connect/config-entries',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/api-gateway',
    destination: '/consul/docs/:version/connect/config-entries/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/control-plane-request-limit',
    destination:
      '/consul/docs/:version/connect/config-entries/control-plane-request-limit',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/exported-services',
    destination:
      '/consul/docs/:version/connect/config-entries/exported-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/file-system-certificate',
    destination:
      '/consul/docs/:version/connect/config-entries/file-system-certificate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/http-route',
    destination: '/consul/docs/:version/connect/config-entries/http-route',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/ingress-gateway',
    destination: '/consul/docs/:version/connect/config-entries/ingress-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/inline-certificate',
    destination:
      '/consul/docs/:version/connect/config-entries/inline-certificate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/jwt-provider',
    destination: '/consul/docs/:version/connect/config-entries/jwt-provider',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/mesh',
    destination: '/consul/docs/:version/connect/config-entries/mesh',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/proxy-defaults',
    destination: '/consul/docs/:version/connect/config-entries/proxy-defaults',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/registration',
    destination: '/consul/docs/:version/connect/config-entries/registration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/sameness-group',
    destination: '/consul/docs/:version/connect/config-entries/sameness-group',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/service-defaults',
    destination:
      '/consul/docs/:version/connect/config-entries/service-defaults',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/service-intentions',
    destination:
      '/consul/docs/:version/connect/config-entries/service-intentions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/service-resolver',
    destination:
      '/consul/docs/:version/connect/config-entries/service-resolver',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/service-router',
    destination: '/consul/docs/:version/connect/config-entries/service-router',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/service-splitter',
    destination:
      '/consul/docs/:version/connect/config-entries/service-splitter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/tcp-route',
    destination: '/consul/docs/:version/connect/config-entries/tcp-route',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/config-entry/terminating-gateway',
    destination:
      '/consul/docs/:version/connect/config-entries/terminating-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/data-plane/connect',
    destination: '/consul/docs/:version/connect/connect-internals',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/data-plane/gateway',
    destination: '/consul/docs/:version/connect/connectivity-tasks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/control-plane/dataplane',
    destination: '/consul/docs/:version/connect/dataplane',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/dataplane/cli',
    destination: '/consul/docs/:version/connect/dataplane/consul-dataplane',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/dataplane/telemetry',
    destination: '/consul/docs/:version/connect/dataplane/telemetry',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/troubleshoot/mesh',
    destination: '/consul/docs/:version/connect/dev',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/distributed-tracing',
    destination: '/consul/docs/:version/connect/distributed-tracing',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/data-plane/gateway',
    destination: '/consul/docs/:version/connect/gateways',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south',
    destination: '/consul/docs/:version/connect/gateways/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/gateway',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/gatewayclass',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewayclass',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/gatewayclassconfig',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewayclassconfig',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/gatewaypolicy',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewaypolicy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/meshservice',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/meshservice',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/routeauthfilter',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routeauthfilter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/routeretryfilter',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routeretryfilter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/routes',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/api-gateway/routetimeoutfilter',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routetimeoutfilter',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/reroute',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/define-routes/reroute-http-requests',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/peer',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/define-routes/route-to-peered-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/route',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/define-routes/routes-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/vm/route',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/define-routes/routes-vms',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/listener',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/deploy/listeners-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/vm/listener',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/deploy/listeners-vms',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/error-messages/api-gateway',
    destination: '/consul/docs/:version/connect/gateways/api-gateway/errors',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/enable',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/install-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/secure-traffic/encrypt',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/secure-traffic/encrypt-vms',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/secure-traffic/jwt/k8s',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/secure-traffic/verify-jwts-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/secure-traffic/jwt/vm',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/secure-traffic/verify-jwts-vms',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/api-gateway/k8s/tech-specs',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/api-gateway',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/upgrades-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/ingress-gateway',
    destination: '/consul/docs/:version/connect/gateways/ingress-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/ingress-gateway/external',
    destination:
      '/consul/docs/:version/connect/gateways/ingress-gateway/tls-external-service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/ingress-gateway/vm',
    destination: '/consul/docs/:version/connect/gateways/ingress-gateway/usage',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/mesh-gateway',
    destination: '/consul/docs/:version/connect/gateways/mesh-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/mesh-gateway/cluster-peer',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/peering-via-mesh-gateways',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/mesh-gateway/admin-partition',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/service-to-service-traffic-partitions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/mesh-gateway/federation',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/service-to-service-traffic-wan-datacenters',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/mesh-gateway/enable',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/wan-federation-via-mesh-gateways',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/terminating-gateway',
    destination: '/consul/docs/:version/connect/gateways/terminating-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/intention',
    destination: '/consul/docs/:version/connect/intentions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/intention/create',
    destination:
      '/consul/docs/:version/connect/intentions/create-manage-intentions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/intention/jwt',
    destination: '/consul/docs/:version/connect/intentions/jwt-authorization',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/intention',
    destination: '/consul/docs/:version/connect/intentions/legacy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic',
    destination: '/consul/docs/:version/connect/manage-traffic',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/discovery-chain',
    destination: '/consul/docs/:version/connect/manage-traffic/discovery-chain',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/failover',
    destination: '/consul/docs/:version/connect/manage-traffic/failover',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/failover/sameness-group',
    destination:
      '/consul/docs/:version/connect/manage-traffic/failover/sameness',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/troubleshoot/fault-injection',
    destination: '/consul/docs/:version/connect/manage-traffic/fault-injection',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/rate-limit',
    destination:
      '/consul/docs/:version/connect/manage-traffic/limit-request-rates',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/route-local',
    destination:
      '/consul/docs/:version/connect/manage-traffic/route-to-local-upstreams',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/native',
    destination: '/consul/docs/:version/connect/native',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/native/go',
    destination: '/consul/docs/:version/connect/native/go',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/tech-specs',
    destination: '/consul/docs/:version/connect/observability',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/access-log',
    destination: '/consul/docs/:version/connect/observability/access-logs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/dataplane',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards/consuldataplanedashboard',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/consul-k8s',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards/consulk8sdashboard',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/server',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards/consulserverdashboard',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/service-to-service',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards/service-to-servicedashboard',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/service',
    destination:
      '/consul/docs/:version/connect/observability/grafanadashboards/servicedashboard',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/grafana/service-to-service',
    destination: '/consul/docs/:version/connect/observability/service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/telemetry/vm',
    destination: '/consul/docs/:version/connect/observability/ui-visualization',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy',
    destination: '/consul/docs/:version/connect/proxies',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/built-in',
    destination: '/consul/docs/:version/connect/proxies/built-in',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/mesh',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-service-mesh-proxies',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/sidecar',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-sidecar-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/envoy',
    destination: '/consul/docs/:version/connect/proxies/envoy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension',
    destination: '/consul/docs/:version/connect/proxies/envoy-extensions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/extensions/ext-authz',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/extensions/otel',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/otel-access-logging',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/extensions/property-override',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/property-override',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/extensions/wasm',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/wasm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/apigee-external-authz',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/apigee-ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/external-authz',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/lambda',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/lambda',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/lua',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/lua',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/otel-access-logging',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/otel-access-logging',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/property-override',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/property-override',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/wasm',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/wasm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/custom',
    destination: '/consul/docs/:version/connect/proxies/integrate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/connect-proxy',
    destination: '/consul/docs/:version/connect/proxies/proxy-config-reference',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/best-practice',
    destination: '/consul/docs/:version/connect/security',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case',
    destination: '/consul/docs/:version/consul-vs-other',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/api-gateway',
    destination: '/consul/docs/:version/consul-vs-other/api-gateway-compare',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/config-management',
    destination:
      '/consul/docs/:version/consul-vs-other/config-management-compare',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/dns',
    destination: '/consul/docs/:version/consul-vs-other/dns-tools-compare',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/use-case/service-mesh',
    destination: '/consul/docs/:version/consul-vs-other/service-mesh-compare',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/kv',
    destination: '/consul/docs/:version/dynamic-app-config/kv',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/kv/store',
    destination: '/consul/docs/:version/dynamic-app-config/kv/store',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/session',
    destination: '/consul/docs/:version/dynamic-app-config/sessions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/application-leader-election',
    destination:
      '/consul/docs/:version/dynamic-app-config/sessions/application-leader-election',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/watch',
    destination: '/consul/docs/:version/dynamic-app-config/watches',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/architecture/ecs',
    destination: '/consul/docs/:version/ecs/architecture',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs/task-bind-address',
    destination: '/consul/docs/:version/ecs/deploy/bind-addresses',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/ecs',
    destination: '/consul/docs/:version/ecs/deploy/configure-routes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs/manual',
    destination: '/consul/docs/:version/ecs/deploy/manual',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs/migrate',
    destination: '/consul/docs/:version/ecs/deploy/migrate-existing-tasks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs/',
    destination: '/consul/docs/:version/ecs/deploy/terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/ecs',
    destination: '/consul/docs/:version/ecs/enterprise',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/ecs',
    destination: '/consul/docs/:version/ecs/reference/compatibility',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/ecs',
    destination: '/consul/docs/:version/ecs/reference/configuration-reference',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/ecs/server-json',
    destination: '/consul/docs/:version/ecs/reference/consul-server-json',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/ecs/tech-specs',
    destination: '/consul/docs/:version/ecs/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/ecs/dataplane',
    destination: '/consul/docs/:version/ecs/upgrade-to-dataplanes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/admin-partition',
    destination: '/consul/docs/:version/enterprise/admin-partitions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/log/audit',
    destination: '/consul/docs/:version/enterprise/audit-logging',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/scale/automated-backup',
    destination: '/consul/docs/:version/enterprise/backups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/downgrade',
    destination: '/consul/docs/:version/enterprise/ent-to-ce-downgrades',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/network-area',
    destination: '/consul/docs/:version/enterprise/federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/fips',
    destination: '/consul/docs/:version/enterprise/fips',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/license',
    destination: '/consul/docs/:version/enterprise/license/overview',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/license/reporting',
    destination:
      '/consul/docs/:version/enterprise/license/utilization-reporting',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/lts',
    destination: '/consul/docs/:version/enterprise/long-term-support',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/namespace',
    destination: '/consul/docs/:version/enterprise/namespaces',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/network-segment/vm',
    destination:
      '/consul/docs/:version/enterprise/network-segments/create-network-segment',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/network-segment',
    destination:
      '/consul/docs/:version/enterprise/network-segments/network-segments-overview',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/scale/read-replica',
    destination: '/consul/docs/:version/enterprise/read-scale',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/scale/redundancy-zone',
    destination: '/consul/docs/:version/enterprise/redundancy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/automated',
    destination: '/consul/docs/:version/enterprise/upgrades',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/install',
    destination: '/consul/docs/:version/install',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/vm/bootstrap',
    destination: '/consul/docs/:version/install/bootstrapping',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/cloud-auto-join',
    destination: '/consul/docs/:version/install/cloud-auto-join',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/glossary',
    destination: '/consul/docs/:version/install/glossary',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/vm/bootstrap',
    destination: '/consul/docs/:version/install/manual-bootstrap',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/architecture/server',
    destination: '/consul/docs/:version/install/performance',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/architecture/ports',
    destination: '/consul/docs/:version/install/ports',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate/consul-tools',
    destination: '/consul/docs/:version/integrate/download-tools',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate/nia',
    destination: '/consul/docs/:version/integrate/nia-integration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate/consul',
    destination: '/consul/docs/:version/integrate/partnerships',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture',
    destination: '/consul/docs/:version/internals',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl',
    destination: '/consul/docs/:version/internals/acl',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/annotation-label',
    destination: '/consul/docs/:version/k8s/annotations-and-labels',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/control-plane/k8s',
    destination: '/consul/docs/:version/k8s/architecture',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s/compatibility',
    destination: '/consul/docs/:version/k8s/compatibility',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/k8s',
    destination: '/consul/docs/:version/k8s/connect',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/tech-specs/k8s',
    destination: '/consul/docs/:version/k8s/connect/cluster-peering/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/sameness-group/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/create-sameness-groups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/establish/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/establish-peering',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/cluster-peering/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/l7-traffic',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/cluster-peering/manage/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/manage-peering',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/k8s',
    destination: '/consul/docs/:version/k8s/connect/connect-ca-provider',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/health-check/k8s',
    destination: '/consul/docs/:version/k8s/connect/health',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/ingress-controller',
    destination: '/consul/docs/:version/k8s/connect/ingress-controllers',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/ingress-gateway/k8s',
    destination: '/consul/docs/:version/k8s/connect/ingress-gateways',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/telemetry/k8s',
    destination: '/consul/docs/:version/k8s/connect/observability/metrics',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/k8s/transparent-proxy',
    destination: '/consul/docs/:version/k8s/connect/onboarding-tproxy-mode',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/terminating-gateway/k8s',
    destination: '/consul/docs/:version/k8s/connect/terminating-gateways',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/transparent-proxy',
    destination: '/consul/docs/:version/k8s/connect/transparent-proxy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/transparent-proxy/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/transparent-proxy/enable-transparent-proxy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/config-entry',
    destination: '/consul/docs/:version/k8s/crds',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/config-entry',
    destination: '/consul/docs/:version/k8s/crds/upgrade-to-crds',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/progressive-rollouts/argo',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/argo-rollouts-configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/k8s/external',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/clients-outside-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/enterprise',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/consul-enterprise',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/datadog',
    destination: '/consul/docs/:version/k8s/deployment-configurations/datadog',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/k8s',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/external-service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/wan-federation',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/wan-federation/k8s',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/wan-federation/k8s-vm',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/external',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/servers-outside-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/multi-cluster',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/single-dc-multi-k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault',
    destination: '/consul/docs/:version/k8s/deployment-configurations/vault',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/bootstrap-token',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/bootstrap-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/enterprise-license',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/enterprise-license',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/gossip-key',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/gossip',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/partition-token',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/partition-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/replication-token',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/replication-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/tls-certificate',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/server-tls',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/snapshot-agent',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/snapshot-agent-config',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/webhook-certificate',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/webhook-certs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/backend',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/systems-integration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/wan-federation/vault-backend',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/wan-federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/dns/forwarding/k8s',
    destination: '/consul/docs/:version/k8s/dns/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/dns/views',
    destination: '/consul/docs/:version/k8s/dns/views',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/dns/views/enable',
    destination: '/consul/docs/:version/k8s/dns/views/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/k8s/helm',
    destination: '/consul/docs/:version/k8s/helm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/helm',
    destination: '/consul/docs/:version/k8s/installation/install',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/consul-k8s',
    destination: '/consul/docs/:version/k8s/installation/install-cli',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/consul-k8s',
    destination: '/consul/docs/:version/k8s/k8s-cli',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/failover/k8s',
    destination: '/consul/docs/:version/k8s/l7-traffic/failover-tproxy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/virtual-service',
    destination:
      '/consul/docs/:version/k8s/l7-traffic/route-to-virtual-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/rotate/k8s',
    destination: '/consul/docs/:version/k8s/operations/certificate-rotation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/gossip/rotate/k8s',
    destination:
      '/consul/docs/:version/k8s/operations/gossip-encryption-key-rotation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/rotate/k8s',
    destination: '/consul/docs/:version/k8s/operations/tls-on-existing-cluster',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s/uninstall',
    destination: '/consul/docs/:version/k8s/operations/uninstall',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/self-hosted',
    destination: '/consul/docs/:version/k8s/platforms/self-hosted-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/k8s/service-sync',
    destination: '/consul/docs/:version/k8s/service-sync',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s',
    destination: '/consul/docs/:version/k8s/upgrade',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s/consul-k8s',
    destination: '/consul/docs/:version/k8s/upgrade/upgrade-cli',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/lambda',
    destination: '/consul/docs/:version/lambda',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/lambda/function',
    destination: '/consul/docs/:version/lambda/invocation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/lambda/service',
    destination: '/consul/docs/:version/lambda/invoke-from-lambda',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/lambda',
    destination: '/consul/docs/:version/lambda/registration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/lambda/automatic',
    destination: '/consul/docs/:version/lambda/registration/automate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/lambda/manual',
    destination: '/consul/docs/:version/lambda/registration/manual',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure',
    destination: '/consul/docs/:version/nia',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts/api',
    destination: '/consul/docs/:version/nia/api',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts/api/health',
    destination: '/consul/docs/:version/nia/api/health',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts/api/status',
    destination: '/consul/docs/:version/nia/api/status',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts/api/tasks',
    destination: '/consul/docs/:version/nia/api/tasks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/cts',
    destination: '/consul/docs/:version/nia/architecture',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/cts',
    destination: '/consul/docs/:version/nia/cli',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/cts/start',
    destination: '/consul/docs/:version/nia/cli/start',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/cts/task',
    destination: '/consul/docs/:version/nia/cli/task',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts/compatibility',
    destination: '/consul/docs/:version/nia/compatibility',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/configure',
    destination: '/consul/docs/:version/nia/configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/cts',
    destination: '/consul/docs/:version/nia/enterprise',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/enterprise/license/cts',
    destination: '/consul/docs/:version/nia/enterprise/license',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/configure',
    destination: '/consul/docs/:version/nia/installation/configure',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/install',
    destination: '/consul/docs/:version/nia/installation/install',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/network-driver',
    destination: '/consul/docs/:version/nia/network-drivers',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/network-driver/hcp-terraform',
    destination: '/consul/docs/:version/nia/network-drivers/hcp-terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/network-driver/terraform',
    destination: '/consul/docs/:version/nia/network-drivers/terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/task',
    destination: '/consul/docs/:version/nia/tasks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/module',
    destination: '/consul/docs/:version/nia/terraform-modules',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/error-messages/cts',
    destination: '/consul/docs/:version/nia/usage/errors-ref',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/requirements',
    destination: '/consul/docs/:version/nia/usage/requirements',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/run',
    destination: '/consul/docs/:version/nia/usage/run',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/infrastructure/high-availability',
    destination: '/consul/docs/:version/nia/usage/run-ha',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure',
    destination: '/consul/docs/:version/security',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl',
    destination: '/consul/docs/:version/security/acl',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/federation',
    destination: '/consul/docs/:version/security/acl/acl-federated-datacenters',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/policy',
    destination: '/consul/docs/:version/security/acl/acl-policies',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/role',
    destination: '/consul/docs/:version/security/acl/acl-roles',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/rule',
    destination: '/consul/docs/:version/security/acl/acl-rules',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/auth-method',
    destination: '/consul/docs/:version/security/acl/auth-methods',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/auth-method/aws',
    destination: '/consul/docs/:version/security/acl/auth-methods/aws-iam',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/auth-method/jwt',
    destination: '/consul/docs/:version/security/acl/auth-methods/jwt',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/auth-method/k8s',
    destination: '/consul/docs/:version/security/acl/auth-methods/kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/auth-method/oidc',
    destination: '/consul/docs/:version/security/acl/auth-methods/oidc',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token',
    destination: '/consul/docs/:version/security/acl/tokens',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/esm',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-consul-esm-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/dns',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-dns-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/mesh-gateway',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-mesh-gateway-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/replication',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-replication-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/service',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-service-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/snapshot-agent',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-snapshot-agent-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/terminating-gateway',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-terminating-gateway-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/vault-backend',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-token-for-vault-consul-storage',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/ui',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-ui-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/agent',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-an-agent-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/token/ingress-gateway',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-an-ingress-gateway-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption',
    destination: '/consul/docs/:version/security/encryption',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/gossip/enable',
    destination: '/consul/docs/:version/security/encryption/gossip',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/mtls',
    destination: '/consul/docs/:version/security/encryption/mtls',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/security-model',
    destination: '/consul/docs/:version/security/security-models',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/security-model/core',
    destination: '/consul/docs/:version/security/security-models/core',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/security-model/cts',
    destination: '/consul/docs/:version/security/security-models/nia',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/service/health-check',
    destination:
      '/consul/docs/:version/services/configuration/checks-configuration-reference',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/service',
    destination:
      '/consul/docs/:version/services/configuration/services-configuration-overview',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/service',
    destination:
      '/consul/docs/:version/services/configuration/services-configuration-reference',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns/scale',
    destination: '/consul/docs/:version/services/discovery/dns-cache',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns/configure',
    destination: '/consul/docs/:version/services/discovery/dns-configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/service/dynamic',
    destination: '/consul/docs/:version/services/discovery/dns-dynamic-lookups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/dns/forwarding',
    destination: '/consul/docs/:version/services/discovery/dns-forwarding',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/dns/forwarding/enable',
    destination:
      '/consul/docs/:version/services/discovery/dns-forwarding/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns',
    destination: '/consul/docs/:version/services/discovery/dns-overview',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/service/static',
    destination: '/consul/docs/:version/services/discovery/dns-static-lookups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/service',
    destination: '/consul/docs/:version/services/services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/health-check/vm',
    destination: '/consul/docs/:version/services/usage/checks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/vm/define',
    destination: '/consul/docs/:version/services/usage/define-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/vm',
    destination:
      '/consul/docs/:version/services/usage/register-services-checks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/error-messages/consul',
    destination: '/consul/docs/:version/troubleshoot/common-errors',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/troubleshoot/service-communication',
    destination: '/consul/docs/:version/troubleshoot/troubleshoot-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade',
    destination: '/consul/docs/:version/upgrading',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/compatibility',
    destination: '/consul/docs/:version/upgrading/compatibility',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions',
    destination: '/consul/docs/:version/upgrading/instructions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions/general',
    destination: '/consul/docs/:version/upgrading/instructions/general-process',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions/upgrade-to-1-10-x',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-10-x',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions/upgrade-to-1-2-x',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-2-x',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions/upgrade-to-1-6-x',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-6-x',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/instructions/upgrade-to-1-8-x',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-8-x',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/version-specific',
    destination: '/consul/docs/:version/upgrading/upgrade-specific',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh',
    destination: '/consul/docs/:version/connect/security',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/data-plane',
    destination: '/consul/docs/:version/connect/connect-internals',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/data-plane/service',
    destination: '/consul/docs/:version/services/usage/define-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/backend',
    destination: '/consul/docs/:version/agent/wal-logstore',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/architecture/security',
    destination: '/consul/docs/:version/security',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/editions',
    destination: '/consul/docs/:version/enterprise',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/install/dev',
    destination: '/consul/docs/:version/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/interface/api',
    destination: '/consul/api-docs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/interface/cli',
    destination: '/consul/docs/commands',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/interface/ui',
    destination: '/consul/docs/:version/connect/observability/ui-visualization',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/identity',
    destination: '/consul/docs/:version/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/fundamentals/tf',
    destination:
      'https://registry.terraform.io/providers/hashicorp/consul/latest/docs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy',
    destination: '/consul/docs/:version/install/bootstrapping',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server',
    destination: '/consul/docs/:version/install/bootstrapping',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/vm',
    destination: '/consul/docs/:version/install/bootstrapping',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/vm/requirements',
    destination: '/consul/docs/:version/install/performance',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s',
    destination: '/consul/:version/k8s/installation/install',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/requirements',
    destination: '/consul/docs/:version/k8s/compatibility',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/consul-k8s',
    destination: '/consul/docs/:version/installation/install-cli',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/minikube',
    destination: '/consul/docs/deploy/server/k8s/platform/minikube',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/kind',
    destination: '/consul/docs/deploy/server/k8s/platform/kind',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/aks',
    destination: '/consul/docs/deploy/server/k8s/platform/aks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/eks',
    destination: '/consul/docs/deploy/server/k8s/platform/eks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/gke',
    destination: '/consul/docs/deploy/server/k8s/platform/gke',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/platform/openshift',
    destination: '/consul/docs/deploy/server/k8s/platform/openshift',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/vault/data/mesh-ca',
    destination: '/consul/docs/:version/connect/ca/vault',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/k8s/uninstall',
    destination: '/consul/docs/deploy/server/k8s/uninstall',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/ecs',
    destination: '/consul/docs/:version/ecs/deploy/terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/server/ecs/manual',
    destination: '/consul/docs/:version/ecs/deploy/manual',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/workload',
    destination: '/consul/docs/deploy/workload',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/workload/client/vm',
    destination: '/consul/docs/deploy/workload/client/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/workload/client/k8s',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/clients-outside-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/workload/client/docker',
    destination: '/consul/docs//deploy/workload/client/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/workload/dataplane/k8s',
    destination: '/consul/docs/:version/connect/dataplane',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/deploy/workload/dataplane/ecs',
    destination: '/consul/docs/:version/ecs/upgrade-to-dataplanes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl',
    destination: '/consul/docs/secure/acl',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/rule',
    destination: '/consul/docs/:version/security/acl/acl-rules',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/bootstrap',
    destination: '/consul/docs/secure/acl/bootstrap',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/reset',
    destination: '/consul/docs/secure/acl/reset',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/legacy',
    destination: '/consul/docs/secure/acl/legacy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/vault/vm',
    destination: '/consul/docs/secure/acl/vault/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/acl/troubleshoot',
    destination: '/consul/docs/secure/acl/troubleshoot',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/gossip/enable/existing',
    destination: '/consul/docs/:version/security/encryption',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/gossip/rotate/vm',
    destination: '/consul/docs/secure/encryption/gossip/rotate/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/mtls',
    destination: '/consul/docs/secure/encryption/tls/mtls',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/enable/new/builtin',
    destination: '/consul/docs/secure/encryption/tls/enable/new/builtin',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/enable/new/openssl',
    destination: '/consul/docs/secure/encryption/tls/enable/new/openssl',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/enable/existing/vm',
    destination: '/consul/docs/secure/encryption/tls/enable/existing/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/enable/existing/k8s',
    destination: '/consul/docs/:version/k8s/operations/tls-on-existing-cluster',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/encryption/tls/rotate/vm',
    destination: '/consul/docs/secure/encryption/tls/rotate/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/auto-config/docker',
    destination: '/consul/docs/secure/auto-config/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure/sso/auth0',
    destination: '/consul/docs/secure/sso/auth0',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant',
    destination: '/consul/docs/multi-tenant',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/admin-partition/k8s',
    destination: '/consul/docs/:version/enterprise/admin-partitions',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/namespace/vm/:slug*',
    destination: '/consul/docs/:version/enterprise/namespaces',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/multi-tenant/namespace/k8s',
    destination: '/consul/docs/:version/enterprise/namespaces',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage',
    destination: '/consul/docs/manage',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/k8s/dns/enable',
    destination: '/consul/docs/:version/k8s/dns',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/disaster-recovery/:slug*',
    destination: '/consul/docs/manage/disaster-recovery/:slug*',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage/scale/autopilot',
    destination: '/consul/docs/manage/scale/autopilot',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/telemetry/dataplane',
    destination: '/consul/docs/:version/connect/dataplane/telemetry',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/telemetry/telegraf',
    destination: '/consul/docs/monitor/telemetry/telegraf',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/telemetry/appdynamics',
    destination: '/consul/docs/monitor/telemetry/appdynamics',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/monitor/log/agent',
    destination: '/consul/docs/monitor/log/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/federated',
    destination: '/consul/docs/upgrade/federated',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s/openshift',
    destination: '/consul/docs/upgrade/k8s/openshift',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/upgrade/k8s/crds',
    destination: '/consul/docs/:version/k8s/crds/upgrade-to-crds',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/release-notes/consul/v1_21_x',
    destination: '/consul/docs/release-notes/consul/v1_21_x',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register',
    destination: '/consul/docs/:version/services/services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/k8s/annotations',
    destination:
      '/consul/docs/:version/k8s/annotations-and-labels#service-sync',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/namespace',
    destination: '/consul/docs/register/service/namespace',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/docker',
    destination: '/consul/docs/register/service/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/aws',
    destination: '/consul/docs/register/service/aws',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs/requirements',
    destination: '/consul/docs/:version/ecs/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/ecs',
    destination: '/consul/docs/:version/ecs/deploy/terraform',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/service/nomad',
    destination: '/consul/docs/:version/nomad',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/esm',
    destination: '/consul/docs/register/external/esm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/esm/k8s',
    destination: '/consul/docs/register/external/esm/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/terminating-gateway',
    destination: '/consul/docs/register/external/terminating-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/register/external/permissive-mtls',
    destination: '/consul/docs/:version/k8s/connect/onboarding-tproxy-mode',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover',
    destination: '/consul/docs/:version/services/discovery/dns-overview',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns/k8s',
    destination: '/consul/docs/:version/k8s/dns/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns/docker',
    destination: '/consul/docs/discover/dns/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/dns/pas',
    destination: '/consul/docs/discover/dns/pas',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/load-balancer/:slug*',
    destination: '/consul/docs/discover/load-balancer/:slug*',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/vm',
    destination: '/consul/docs/discover/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/k8s',
    destination: '/consul/docs/discover/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/discover/docker',
    destination: '/consul/docs/discover/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/enable',
    destination: '/consul/docs/:version/connect/configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/proxy/transparent-proxy/ecs',
    destination: '/consul/docs/connect/proxy/transparent-proxy/ecs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/troubleshoot',
    destination: '/consul/docs/:version/troubleshoot/troubleshoot-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/troubleshoot/debug',
    destination: '/consul/docs/:version/connect/dev',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/troubleshoot/service-to-service',
    destination: '/consul/docs/:version/troubleshoot/troubleshoot-services',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/vm',
    destination: '/consul/docs/connect/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/k8s/inject',
    destination: '/consul/docs/:version/k8s/connect',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/k8s/crds',
    destination: '/consul/docs/:version/k8s/crds',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/connect/k8s/workload',
    destination: '/consul/docs/:version/k8s/connect',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south',
    destination: '/consul/docs/:version/connect/gateways/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/vm',
    destination: '/consul/docs/:version/connect/gateways/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/north-south/k8s',
    destination: '/consul/docs/:version/connect/gateways/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west',
    destination: '/consul/docs/:version/connect/gateways/mesh-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/wan-federation/vms',
    destination: '/consul/docs/east-west/wan-federation/vms',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/vm',
    destination: '/consul/docs/east-west/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/east-west/k8s',
    destination: '/consul/docs/:version/east-west/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh',
    destination: '/consul/docs/secure-mesh',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/bootstrap',
    destination: '/consul/docs/secure-mesh/certificate/bootstrap',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/rotate',
    destination: '/consul/docs/secure-mesh/certificate/rotate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/existing',
    destination: '/consul/docs/:version/k8s/operations/tls-on-existing-cluster',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/certificate/permissive-mtls',
    destination: '/consul/docs/secure-mesh/certificate/permissive-mtls',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/vm',
    destination: '/consul/docs/:version/secure-mesh/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/secure-mesh/k8s',
    destination: '/consul/docs/secure-mesh/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/failover/prepared-query',
    destination: '/consul/docs/manage-traffic/failover/prepared-query',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/vm',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/peering-traffic-management',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/manage-traffic/k8s',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/l7-traffic',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe',
    destination: '/consul/docs/:version/connect/observability',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/observe/docker',
    destination: '/consul/docs/observe/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate',
    destination: '/consul/docs/automate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/automate/consul-template/:slug*',
    destination: '/consul/docs/automate/consul-template/:slug*',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/vm',
    destination: '/consul/docs/vm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/docker',
    destination: '/consul/docs/docker',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/openshift',
    destination: '/consul/docs/:version/openshift',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate',
    destination: '/consul/docs/:version/integrate/partnerships',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate/hcdiag',
    destination: '/consul/docs/integrate/hcdiag',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/integrate/vault/k8s',
    destination: '/consul/docs/integrate/vault/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/apigee',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/apigee-ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/ext',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/ext-authz',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/envoy-extension/otel',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/otel-access-logging',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/error-messages/k8s',
    destination:
      '/consul/docs/:version/troubleshoot/common-errors#common-errors-on-kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/troubleshoot',
    destination: '/consul/docs/troubleshoot',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cli/consul-aws',
    destination: '/consul/docs/reference/cli/consul-aws',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/auth-method/aws-iam',
    destination: '/consul/docs/:version/security/acl/auth-methods/aws-iam',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/auth-method/jwt',
    destination: '/consul/docs/:version/security/acl/auth-methods/jwt',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/auth-method/k8s',
    destination: '/consul/docs/:version/security/acl/auth-methods/kubernetes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/auth-method/oidc',
    destination: '/consul/docs/:version/security/acl/auth-methods/oidc',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/token',
    destination: '/consul/docs/:version/security/acl/tokens',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/role',
    destination: '/consul/docs/:version/security/acl/acl-roles',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/acl/policy',
    destination: '/consul/docs/:version/security/acl/acl-policies',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/agent/configuration-file/:slug*',
    destination: '/consul/docs/:version/agent/config/config-files',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/agent/telemetry',
    destination: '/consul/docs/:version/agent/monitor/telemetry',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/docs/reference/consul-template/:slug*',
    destination: '/consul/docs/docs/reference/consul-template/:slug*',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/cts',
    destination: '/consul/docs/:version/nia/configuration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/dns',
    destination: '/consul/docs/:version/services/discovery/dns-static-lookups',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/namespace',
    destination: '/consul/docs/:version/enterprise/namespaces',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1\\.(?:8|9|10|11|12|13|14|15|16|17|18|19|20)\\.x)/reference/proxy/sidecar',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-sidecar-services',
    permanent: true,
  },
  ///////////////////////////////////
  // Tutorial --> Docs conversions //
  ///////////////////////////////////
  {
    source: '/consul/tutorials/production-vms/security',
    destination: '/consul/docs/secure',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/docker-container-agents',
    destination: '/consul/docs/docker',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/tls-encryption-secure',
    destination: '/consul/docs/secure/encryption/tls/mtls',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/dns-forwarding',
    destination: '/consul/docs/manage/dns/forwarding',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/get-started-explore-the-ui',
    destination: '/consul/docs/fundamentals/interface/ui',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/application-leader-elections',
    destination: '/consul/docs/automate/application-leader-election',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/vault-consul-secrets',
    destination: '/consul/docs/secure/acl/vault/vm',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/docker-compose-auto-config',
    destination: '/consul/docs/secure/auto-config/docker',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/load-balancing-haproxy',
    destination: '/consul/docs/discover/load-balancer/ha',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/tls-encryption-openssl-secure',
    destination: '/consul/docs/secure/encryption/tls/enable/new/openssl',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/backup-and-restore',
    destination: '/consul/docs/manage/disaster-recovery/backup-restore',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/vault-pki-consul-secure-tls',
    destination: '/consul/docs/automate/consul-template/vault/mtls',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/federation-gossip-wan',
    destination: '/consul/docs/east-west/wan-federation',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/docker-compose-observability',
    destination: '/consul/docs/observe/docker',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/troubleshooting',
    destination: '/consul/docs/fundamentals/troubleshoot',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/disaster-recovery',
    destination: '/consul/docs/manage/disaster-recovery',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/archive/tls-encryption-secure-existing-datacenter',
    destination: '/consul/docs/secure/encryption/tls/enable/existing/vm',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/operate-consul/kubernetes-vault-consul-secrets-management',
    destination: '/consul/docs/integrate/vault/k8s',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/kubernetes-openshift-red-hat',
    destination: '/consul/docs/deploy/server/k8s/platform/openshift',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/autopilot-datacenter-operations',
    destination: '/consul/docs/manage/scale/autopilot',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/vault-kv-consul-secure-gossip',
    destination: '/consul/docs/automate/consul-template/vault/gossip',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/single-sign-on-auth0',
    destination: '/consul/docs/secure/sso/auth0',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/federation-network-areas',
    destination: '/consul/docs/east-west/network-area',
    permanent: true,
  },
  {
    source: '/consul/tutorials/observe-your-network/audit-logging',
    destination: '/consul/docs/monitor/log/audit',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/implement-multi-tenancy/namespaces-share-datacenter-access',
    destination: '/consul/docs/register/service/namespace',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/recovery-outage-primary',
    destination: '/consul/docs/manage/disaster-recovery/federation',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/upgrade-automation',
    destination: '/consul/docs/upgrade/automated',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/sync-aws-services',
    destination: '/consul/docs/register/service/aws',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/gossip-encryption-rotate',
    destination: '/consul/docs/secure/encryption/gossip/rotate/vm',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/monitor-health-telegraf',
    destination: '/consul/docs/monitor/telemetry/telegraf',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/production-multi-cluster/multi-disaster-recovery',
    destination: '/consul/docs/manage/disaster-recovery',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/implement-multi-tenancy/namespaces-secure-shared-access',
    destination: '/consul/docs/multi-tenant/namespace/vm',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/upgrade-federated-environment',
    destination: '/consul/docs/upgrade/federated',
    permanent: true,
  },
  {
    source: '/consul/tutorials/operate-consul/hcdiag-with-consul',
    destination: '/consul/docs/integrate/hcdiag',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/sync-pivotal-cloud-services',
    destination: '/consul/docs/discover/dns/pas',
    permanent: true,
  },
  {
    source: '/consul/tutorials/archive/monitor-with-appdynamics',
    destination: '/consul/docs/monitor/telemetry/appdynamics',
    permanent: true,
  },
]
