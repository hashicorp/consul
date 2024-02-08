/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// REDIRECTS FILE

// See the README file in this directory for documentation. Please do not
// modify or delete existing redirects without first verifying internally.
// Next.js redirect documentation: https://nextjs.org/docs/api-reference/next.config.js/redirects

module.exports = [
  // External and links redirect to new docs
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
    source: '/consul/docs/consul-vs-other',
    destination: '/consul/docs/why',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/service-mesh-compare',
    destination: '/consul/docs/why/service-mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/dns-tools-compare',
    destination: '/consul/docs/why/dns',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/config-management-compare',
    destination: '/consul/docs/why/config-management',
    permanent: true,
  },
  {
    source: '/consul/docs/consul-vs-other/api-gateway-compare',
    destination: '/consul/docs/why/api-gateway',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/connect-internals',
    destination: '/consul/docs/concepts/service-mesh',
    permanent: true,
  },

  {
    source: '/consul/docs/install/ports',
    destination: '/consul/docs/deploy/server/ports',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/fips',
    destination: '/consul/docs/deploy/server/fips',
    permanent: true,
  },

  {
    source: '/consul/docs/install/bootstrapping',
    destination: '/consul/docs/deploy/server/vm/bootstrap',
    permanent: true,
  },
  {
    source: '/consul/docs/install/cloud-auto-join',
    destination: '/consul/docs/deploy/server/vm/cloud-auto-join',
    permanent: true,
  },
  {
    source: '/consul/docs/install/performance',
    destination: '/consul/docs/deploy/server/vm/requirements',
    permanent: true,
  },

  {
    source: '/consul/k8s/compatibility',
    destination: '/consul/docs/deploy/server/k8s/requirements',
    permanent: true,
  },

  {
    source: '/consul/tutorials/kubernetes/kubernetes-minikube',
    destination: '/consul/docs/deploy/server/k8s/plaform/minikube',
    permanent: true,
  },
  {
    source: '/consul/tutorials/kubernetes/kubernetes-kind',
    destination: '/consul/docs/deploy/server/k8s/platform/kind',
    permanent: true,
  },
  {
    source: '/consul/tutorials/kubernetes/kubernetes-aks-azure',
    destination: '/consul/docs/deploy/server/k8s/platform/aks',
    permanent: true,
  },
  {
    source: '/consul/tutorials/kubernetes/kubernetes-eks-aws',
    destination: '/consul/docs/deploy/server/k8s/platform/eks',
    permanent: true,
  },
  {
    source: '/consul/tutorials/kubernetes/kubernetes-gke-google',
    destination: '/consul/docs/deploy/server/k8s/platform/gke',
    permanent: true,
  },
  {
    source: '/consul/tutorials/kubernetes/kubernetes-openshift-red-hat',
    destination: '/consul/docs/deploy/server/k8s/platform/openshift',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/platforms/self-hosted-kubernetes',
    destination: '/consul/docs/deploy/server/k8s/platform/self-hosted',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/servers-outside-kubernetes',
    destination: '/consul/docs/deploy/server/k8s/examples/servers-outside',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/single-dc-multi-k8s',
    destination: '/consul/docs/deploy/server/k8s/examples/multiple-clusters',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/consul-enterprise',
    destination: '/consul/docs/deploy/server/k8s/enterprise',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/multi-cluster',
    destination: '/consul/docs/deploy/server/k8s/examples/federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/kubernetes',
    destination: '/consul/docs/deploy/server/k8s/examples/federation/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes',
    destination: '/consul/docs/deploy/server/k8s/examples/federation/k8s-vm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault',
    destination: '/consul/docs/deploy/server/k8s/examples/vault',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/systems-integration',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/system-integration',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault/data-integration',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/bootstrap-token',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/bootstrap-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/enterprise-license',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/enterprise-license',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/gossip',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/gossip-encryption-key',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/partition-token',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/partition-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/replication-token',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/replication-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/server-tls',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/server-tls',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/connect-ca',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/service-mesh-certificate',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/snapshot-agent-config',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/snapshot-agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/vault/data-integration/webhook-certs',
    destination:
      '/consul/docs/deploy/server/k8s/examples/vault/data-integration/webhook-certificate',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/vault/wan-federation',
    destination: '/consul/docs/deploy/server/k8s/examples/vault/wan',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore',
    destination: '/consul/docs/deploy/server/wal/',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/wal-logstore/enable',
    destination: '/consul/docs/deploy/server/wal/enable',
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
    source:
      '/consul/docs/k8s/deployment-configurations/clients-outside-kubernetes',
    destination: '/consul/docs/deploy/control-plane/k8s',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/configuration',
    destination: '/consul/docs/deploy/service-mesh/enable',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-service-mesh-proxies',
    destination: '/consul/docs/deploy/service-mesh/deploy-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-sidecar-services',
    destination: '/consul/docs/deploy/service-mesh/deploy-sidecar',
    permanent: true,
  },
  {
    source: '/consul/docs/architecture/scale',
    destination: '/consul/docs/deploy/scale',
    permanent: true,
  },

  {
    source: '/consul/docs/security',
    destination: '/consul/docs/secure-consul/',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl',
    destination: '/consul/docs/secure-consul/acl',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens',
    destination: '/consul/docs/secure-consul/acl/token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-policies',
    destination: '/consul/docs/secure-consul/acl/policies',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-roles',
    destination: '/consul/docs/reference/acl/rule',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/acl-rules',
    destination: '/consul/docs/secure-consul/acl/rules',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods',
    destination: '/consul/docs/secure-consul/acl/auth-methods',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/kubernetes',
    destination: '/consul/docs/secure-consul/acl/auth-methods/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/jwt',
    destination: '/consul/docs/secure-consul/acl/auth-methods/jwt',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/oidc',
    destination: '/consul/docs/secure-consul/acl/auth-methods/oidc',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/aws-iam',
    destination: '/consul/docs/secure-consul/acl/auth-methods/aws-iam',
    permanent: true,
  },

  {
    source: '/consul/docs/security/acl/acl-federated-datacenters',
    destination: '/consul/docs/secure-consul/acl/federation',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-service-token',
    destination: '/consul/docs/secure-consul/acl/token/service-token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-an-agent-token',
    destination: '/consul/docs/secure-consul/acl/token/agent-token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-ui-token',
    destination: '/consul/docs/secure-consul/acl/token/ui-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-mesh-gateway-token',
    destination: '/consul/docs/secure-consul/acl/token/mesh-gateway-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-an-ingress-gateway-token',
    destination: '/consul/docs/secure-consul/acl/token/ingress-gateway-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-terminating-gateway-token',
    destination:
      '/consul/docs/secure-consul/acl/token/terminating-gateway-token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-dns-token',
    destination: '/consul/docs/secure-consul/acl/token/dns-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-replication-token',
    destination: '/consul/docs/secure-consul/acl/token/replication-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-snapshot-agent-token',
    destination: '/consul/docs/secure-consul/acl/token/snapshot-agent-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/security/acl/tokens/create/create-a-token-for-vault-consul-storage',
    destination:
      '/consul/docs/secure-consul/acl/token/vault-storage-backend-token',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/tokens/create/create-a-consul-esm-token',
    destination: '/consul/docs/secure-consul/acl/token/esm-token',
    permanent: true,
  },
  {
    source:
      '/consul/docs/consul/tutorials/security-operations/access-control-token-migration',
    destination: '/consul/docs/secure-consul/acl/legacy',
    permanent: true,
  },
  {
    source: '/consul/tutorials/security/access-control-manage-policies',
    destination: '/consul/docs/secure-consul/acl/troubleshoot',
    permanent: true,
  },
  {
    source: '/consul/docs/security/encryption',
    destination: '/consul/docs/secure-consul/encryption',
    permanent: true,
  },
  {
    source: '/consul/tutorials/security/gossip-encryption-secure',
    destination: '/consul/docs/secure-consul/encryption/gossip/enable',
    permanent: true,
  },
  {
    source:
      '/consul/consul/tutorials/security-operations/tls-encryption-secure-existing-datacenter',
    destination: '/consul/docs/secure-consul/encryption/gossip/existing',
    permanent: true,
  },
  {
    source: '/consul/tutorials/security/tls-encryption-secure',
    destination: '/consul/docs/secure-consul/encryption/tls/builtin',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/security-operations/tls-encryption-openssl-secure',
    destination: '/consul/docs/secure-consul/encryption/tls/openssl',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/security-operations/tls-encryption-secure-existing-datacenter',
    destination: '/consul/docs/secure-consul/encryption/tls/existing',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models',
    destination: '/consul/docs/secure-consul/security-model',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models/core',
    destination: '/consul/docs/secure-consul/security-model/consul',
    permanent: true,
  },
  {
    source: '/consul/docs/security/security-models/nia',
    destination: '/consul/docs/secure-consul/security-model/nia',
    permanent: true,
  },

  {
    source: '/consul/docs/agent/telemetry',
    destination: '/consul/docs/monitor-consul/telemetry/',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/audit-logging',
    destination: '/consul/docs/monitor-consul/log/audit-log',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/backups',
    destination: '/consul/docs/manage-consul/automated-backups',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/redundancy',
    destination: '/consul/docs/manage-consul/redundancy-zones',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/read-scale',
    destination: '/consul/docs/manage-consul/read-replicas',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits',
    destination: '/consul/docs/manage-consul/agent-rate-limit',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/init-rate-limits',
    destination: '/consul/docs/manage-consul/agent-rate-limit/initialize',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/monitor-rate-limits',
    destination: '/consul/docs/manage-consul/agent-rate-limit/monitor',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/set-global-traffic-rate-limits',
    destination: '/consul/docs/manage-consul/agent-rate-limit/set-global',
    permanent: true,
  },
  {
    source: '/consul/docs/agent/limits/usage/limit-request-rates-from-ips',
    destination:
      '/consul/docs/manage-consul/agent-rate-limit/source-ip-addresses',
    permanent: true,
  },

  {
    source: '/consul/docs/upgrading',
    destination: '/consul/docs/upgrade',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/upgrades',
    destination: '/consul/docs/upgrade/automated',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/compatibility',
    destination: '/consul/docs/upgrade/compatibility-promise',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/upgrade-specific',
    destination: '/consul/docs/upgrade/version-specific',
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
    destination: '/consul/docs/upgrade/instructions/1-10-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-8-x',
    destination: '/consul/docs/upgrade/instructions/1-8-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-6-x',
    destination: '/consul/docs/upgrade/instructions/1-6-x',
    permanent: true,
  },
  {
    source: '/consul/docs/upgrading/instructions/upgrade-to-1-2-x',
    destination: '/consul/docs/upgrade/instructions/1-2-x',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/upgrade',
    destination: '/consul/docs/upgrade/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/compatibility',
    destination: '/consul/docs/upgrade/k8s/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/upgrade/upgrade-cli',
    destination: '/consul/docs/upgrade/k8s/consul-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/crds/upgrade-to-crds',
    destination: '/consul/docs/upgrade/k8s/crds',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/compatibility',
    destination: '/consul/docs/upgrade/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/upgrade-to-dataplanes',
    destination: '/consul/docs/upgrade/ecs/dataplane',
    permanent: true,
  },

  {
    source: '/consul/docs/release-notes/consul-api-gateway',
    destination: '/consul/docs/release-notes/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/release-notes/consul-ecs',
    destination: '/consul/docs/release-notes/ecs',
    permanent: true,
  },

  {
    source: '/consul/docs/services/services',
    destination: '/consul/docs/register',
    permanent: true,
  },

  {
    source: '/consul/docs/services/usage/define-services',
    destination: '/consul/docs/register/service/vm/define',
    permanent: true,
  },
  {
    source: '/consul/docs/services/usage/checks',
    destination: '/consul/docs/register/service/vm/health-checks',
    permanent: true,
  },
  {
    source: '/consul/docs/services/usage/register-services-checks',
    destination: '/consul/docs/register/service/vm',
    permanent: true,
  },

  {
    source: '/consul/docs/k8s/service-sync',
    destination: '/consul/docs/register/service/k8s/service-sync',
    permanent: true,
  },

  {
    source: '/consul/docs/ecs/tech-specs',
    destination: '/consul/docs/register/service/ecs/requirements',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/deploy/terraform',
    destination: '/consul/docs/register/service/ecs',
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
    source: '/consul/docs/ecs/deploy/bind-addresses',
    destination:
      '/consul/docs/register/service/ecs/configure-task-bind-address',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/enterprise',
    destination: '/consul/docs/deploy/server/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda',
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
    source: '/consul/docs/k8s/connect/terminating-gateways',
    destination: '/consul/docs/register/external/terminating-gateway/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/onboarding-tproxy-mode',
    destination: '/consul/docs/register/external/permissive-mtls',
    permanent: true,
  },

  {
    source: '/consul/docs/services/discovery/dns-overview',
    destination: '/consul/docs/discover/dns',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-configuration',
    destination: '/consul/docs/discover/dns/configure',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-static-lookups',
    destination: '/consul/docs/discover/dns/static-lookup',
    permanent: true,
  },
  {
    source: '/consul/docs/services/discovery/dns-dynamic-lookups',
    destination: '/consul/docs/discover/dns/dynamic-lookup',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/dns',
    destination: '/consul/docs/discover/dns/k8s',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/connect-internals',
    destination: '/consul/docsLINK TO /concept/service-mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/configuration',
    destination: '/consul/docs/connect/configuration-entries',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport',
    destination: '/consul/docs/connect/multi-port',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/configure',
    destination: '/consul/docs/connect/multi-port/configure',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/invocation',
    destination: '/consul/docs/connect/lambda/invoke-function',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda/invoke-from-lambda',
    destination: '/consul/docs/connect/lambda/invoke-service',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies',
    destination: '/consul/docs/connect/proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-service-mesh-proxies',
    destination: '/consul/docs/connect/proxy/deploy-service-mesh',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-sidecar-services',
    destination: '/consul/docs/connect/proxy/deploy-sidecar',
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
    destination: '/consul/docs/connect/proxy/transparent-proxy/enable',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/dev',
    destination: '/consul/docs/connect/troubleshoot/develop-debug',
    permanent: true,
  },
  {
    source: '/consul/docs/troubleshoot/troubleshoot-services',
    destination: '/consul/docs/connect/troubleshoot/service-to-service',
    permanent: true,
  },

  {
    source: '/consul/docs/ecs/deploy/configure-routes',
    destination: '/consul/docs/connect/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/lambda',
    destination: '/consul/docs/connect/lambda',
    permanent: true,
  },

  {
    source: '/consul/docs/gateways',
    destination: '/consul/docs/access',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway',
    destination: '/consul/docs/access/api-gateway/',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/tech-specs',
    destination: '/consul/docs/access/api-gateway/tech-specs',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/install-k8s',
    destination: '/consul/docs/access/api-gateway/install',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/upgrades-k8s',
    destination: '/consul/docs/upgrade/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/deploy/listeners-vms',
    destination: '/consul/docs/access/api-gateway/deploy-listener/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/deploy/listeners-k8s',
    destination: '/consul/docs/access/api-gateway/deploy-listener/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/define-routes/routes-vms',
    destination: '/consul/docs/access/api-gateway/define-route/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/define-routes/routes-k8s',
    destination: '/consul/docs/access/api-gateway/define-route/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/gateways/api-gateway/define-routes/reroute-http-requests',
    destination: '/consul/docs/access/api-gateway/define-route/reroute',
    permanent: true,
  },
  {
    source:
      '/consul/docs/gateways/api-gateway/define-routes/route-to-peered-services',
    destination: '/consul/docs/access/api-gateway/route/peer',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/secure-traffic/encrypt-vms',
    destination: '/consul/docs/access/api-gateway/secure/encrypt',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/secure-traffic/verify-jwts-vms',
    destination: '/consul/docs/access/api-gateway/secure/jwt-vm',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/secure-traffic/verify-jwts-k8s',
    destination: '/consul/docs/access/api-gateway/secure/jwt-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/api-gateway/errors',
    destination: '/consul/docs/common-error-messages/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/ingress-gateway',
    destination: '/consul/docs/access/ingress-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/ingress-gateway/usage',
    destination: '/consul/docs/access/ingress-gateway/create/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/ingress-gateway/usage',
    destination: '/consul/docs/access/ingress-gateway/create/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/gateways/ingress-gateway/tls-external-service',
    destination: '/consul/docs/access/ingress-gateway/tls-external-service',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/admin-partitions',
    destination: '/consul/docs/segment/admin-partition',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/namespaces',
    destination: '/consul/docs/segment/namespace',
    permanent: true,
  },

  {
    source:
      '/consul/docs/k8s/connect/cluster-peering/usage/create-sameness-groups',
    destination: '/consul/docs/link/sameness-group',
    permanent: true,
  },
  {
    source:
      '/consul/docs/enterprise/network-segments/network-segments-overview',
    destination: '/consul/docs/segment/network-segment',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/network-segments/create-network-segment',
    destination: '/consul/docs/segment/network-segment/vm',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/cluster-peering',
    destination: '/consul/docs/link/cluster-peering',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/tech-specs',
    destination: '/consul/docs/link/cluster-peering/tech-specs',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/cluster-peering/usage/establish-cluster-peering',
    destination: '/consul/docs/link/cluster-peering/establish-connection/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/establish-peering',
    destination: '/consul/docs/link/cluster-peering/establish-connection/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/cluster-peering/usage/manage-connections',
    destination: '/consul/docs/link/cluster-peering/manage-connection/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/manage-peering',
    destination: '/consul/docs/link/cluster-peering/manage-connection/k8s',
    permanent: true,
  },

  {
    source: '/consul/tutorials/datacenter-operations/federation-network-areas',
    destination: '/consul/docs/link/network-area',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/deployment-configurations/multi-cluster',
    destination: '/consul/docs/link/wan-federation/',
    permanent: true,
  },

  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/kubernetes',
    destination: '/consul/docs/link/wan-federation/k8s',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes',
    destination: '/consul/docs/link/wan-federation/k8s-vm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/mesh-gateway',
    destination: '/consul/docs/link/mesh-gateway/',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/wan-federation-via-mesh-gateways',
    destination: '/consul/docs/link/mesh-gateway/enable',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-wan-datacenters',
    destination: '/consul/docs/link/mesh-gateway/federation',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/peering-via-mesh-gateways',
    destination: '/consul/docs/link/mesh-gateway/cluster-peer',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/mesh-gateway/service-to-service-traffic-partitions',
    destination: '/consul/docs/link/mesh-gateway/admin-partition',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/intentions',
    destination: '/consul/docs/secure-mesh/intentions',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions/create-manage-intentions',
    destination: '/consul/docs/secure-mesh/intentions/create',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/intentions/jwt-authorization',
    destination: '/consul/docs/secure-mesh/intentions/jwt',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/ca',
    destination: '/consul/docs/secure-mesh/certificate',
    permanent: true,
  },
  {
    source:
      '/consul/tutorials/developer-mesh/service-mesh-production-checklist#bootstrap-certificate-authority-for-consul-service-mesh',
    destination: '/consul/docs/secure-mesh/certificate/bootstrap/vm',
    permanent: true,
  },

  {
    source: '/consul/docs/k8s/operations/certificate-rotation',
    destination: '/consul/docs/secure-mesh/certificate/rotate/tls',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/gossip-encryption-key-rotation',
    destination: '/consul/docs/secure-mesh/certificate/rotate/gossip',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/operations/tls-on-existing-cluster',
    destination: '/consul/docs/secure-mesh/certificate/existing',
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
    source: '/consul/docs/connect/ca/aws',
    destination: '/consul/docs/secure-mesh/certificate/acm',
    permanent: true,
  },
  {
    source:
      '/consul/docs/k8s/connect/onboarding-tproxy-mode#enable-permissive-mtls-mode',
    destination: '/consul/docs/secure-mesh/certificate/enable-permissive-mtls',
    permanent: true,
  },

  {
    source: '/consul/docs/manage-traffic',
    destination: '/consul/docs/manage-traffic',
    permanent: true,
  },
  {
    source: '/consul/docs/manage-traffic/route-to-local-upstreams',
    destination: '/consul/docs/manage-traffic/route-local',
    permanent: true,
  },
  {
    source: '/consul/docs/manage-traffic/limit-request-rates',
    destination: '/consul/docs/manage-traffic/limit-rate',
    permanent: true,
  },

  {
    source:
      '/consul/docs/connect/cluster-peering/usage/peering-traffic-management',
    destination: '/consul/docs/manage-traffic/cluster-peering/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/cluster-peering/usage/l7-traffic',
    destination: '/consul/docs/manage-traffic/cluster-peering/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/failover',
    destination: '/consul/docs/manage-traffic/failover',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/l7-traffic/failover-tproxy',
    destination: '/consul/docs/manage-traffic/failover/configure-k8s',
    permanent: true,
  },
  {
    source: '/consul/tutorials/developer-discovery/automate-geo-failover',
    destination: '/consul/docs/manage-traffic/failover/prepared-query',
    permanent: true,
  },
  {
    source: '/consul/docstraffic/route-to-virtual-services',
    destination: '/consul/docs/manage-traffic/failover/virtual-services',
    permanent: true,
  },
  {
    source: '/consul/docs/manage-traffic/failover/sameness',
    destination: '/consul/docs/manage-traffic/failover/sameness-group',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/observability/access-logs',
    destination: '/consul/docs/observe/access-logs',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/observability/ui-visualization',
    destination: '/consul/docs/observe/mesh/vm',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/connect/observability/metrics',
    destination: '/consul/docs/observe/mesh/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/distributed-tracing',
    destination: '/consul/docs/observe/distributed-tracing',
    permanent: true,
  },

  {
    source: '/consul/docs/dynamic-app-config/kv',
    destination: '/consul/docs/dynamic-app/kv',
    permanent: true,
  },
  {
    source: '/consul/tutorials/interactive/get-started-key-value-store',
    destination: '/consul/docs/dynamic-app/kv/store',
    permanent: true,
  },

  {
    source: '/consul/docs/dynamic-app-config/sessions',
    destination: '/consul/docs/dynamic-app/sessions',
    permanent: true,
  },
  {
    source: '/consul/docs/dynamic-app-config/watches',
    destination: '/consul/docs/dynamic-app/watches',
    permanent: true,
  },

  {
    source: '/consul/docs/enterprise/license/overview',
    destination: '/consul/docs/enterprise/license',
    permanent: true,
  },
  {
    source: '/consul/docs/enterprise/license/utilization-reporting',
    destination:
      '/consul/docs/enterprise/license/automated-entitlement-utilization',
    permanent: true,
  },

  {
    source: '/consul/docs/integrate/download-tools',
    destination: '/consul/docs/integrate/consul-tools',
    permanent: true,
  },
  {
    source: '/consul/docs/integrate/partnerships',
    destination: '/consul/docs/integrate/consul',
    permanent: true,
  },
  {
    source: '/consul/docs/integrate/nia-integration',
    destination: '/consul/docs/integrate/nia',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/integrate',
    destination: '/consul/docs/integrate/proxy',
    permanent: true,
  },

  {
    source: '/consul/docs/connect/proxies/envoy-extensions',
    destination: '/consul/docs/envoy-extension/',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/apigee-ext-authz',
    destination: '/consul/docs/envoy-extension/apigee-external-authz',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/ext-authz',
    destination: '/consul/docs/envoy-extension/external-authz',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/lua',
    destination: '/consul/docs/envoy-extension/lua',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/lambda',
    destination: '/consul/docs/envoy-extension/lambda',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/property-override',
    destination: '/consul/docs/envoy-extension/configure-envoy',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/proxies/envoy-extensions/usage/otel-access-logging',
    destination: '/consul/docs/envoy-extension/otel-access-logging',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy-extensions/usage/wasm',
    destination: '/consul/docs/envoy-extension/wasm',
    permanent: true,
  },

  {
    source: '/consul/docs/nia',
    destination: '/consul/docs/cts',
    permanent: true,
  },
  {
    source: '/consul/docshitecture',
    destination: '/consul/docs/cts/architecture',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/requirements',
    destination: '/consul/docs/cts/requirements',
    permanent: true,
  },

  {
    source: '/consul/docs/nia/installation/install',
    destination: '/consul/docs/cts/deploy/install',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/installation/configure',
    destination: '/consul/docs/cts/configure',
    permanent: true,
  },

  {
    source: '/consul/docs/nia/tasks',
    destination: '/consul/docs/cts/configure/task',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers',
    destination: '/consul/docs/cts/configure/network-driver',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/terraform',
    destination: '/consul/docs/cts/configure/network-driver/terraform',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/network-drivers/terraform-cloud',
    destination: '/consul/docs/cts/configure/network-driver/terraform-cloud',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/enterprise/license',
    destination: '/consul/docs/cts/configure/license',
    permanent: true,
  },
  {
    source:
      '/consul/docs/nia/terraform-modules#how-to-create-a-compatible-terraform-module',
    destination: '/consul/docs/cts/create-terraform-module',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/compatibility',
    destination: '/consul/docs/reference/cts/compatibility',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/enterprise',
    destination: '/consul/docs/cts/enterprise',
    permanent: true,
  },

  {
    source: '/consul/docs/k8s/k8s-cli',
    destination: '/consul/docsreference-cli/consul-k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/dataplane/consul-dataplane',
    destination: '/consul/docsreference-cli/consul-dataplane',
    permanent: true,
  },

  {
    source: '/consul/docs/troubleshoot/common-errors',
    destination: '/consul/docs/error-messages/consul',
    permanent: true,
  },
  {
    source:
      '/consul/docs/troubleshoot/common-errors#common-errors-on-kubernetes',
    destination: '/consul/docs/error-messages/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/errors',
    destination: '/consul/docs/error-messages/api-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/usage/errors-ref',
    destination: '/consul/docs/error-messages/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/install/glossary',
    destination: '/consul/docs/reference/glossary',
    permanent: true,
  },

  {
    source: '/consul/docs/agent/config/config-files',
    destination: '/consul/docs/reference/agent',
    permanent: true,
  },
  {
    source:
      '/consul/docs/services/configuration/services-configuration-reference',
    destination: '/consul/docs/reference/service',
    permanent: true,
  },
  {
    source:
      '/consul/docs/services/configuration/checks-configuration-reference',
    destination: '/consul/docs/reference/service/health-checks',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/api-gateway',
    destination: '/consul/docs/reference/config-entry/api-gateway',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/http-route',
    destination: '/consul/docs/reference/config-entry/http-route',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration/tcp-route',
    destination: '/consul/docs/reference/config-entry/tcp-route',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/inline-certificate',
    destination: '/consul/docs/reference/config-entry/inline-certificate',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/ingress-gateway',
    destination: '/consul/docs/reference/config-entry/ingress-gateway',
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
    source: '/consul/docs/connect/config-entries/exported-services',
    destination: '/consul/docs/reference/config-entry/exported-services',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/proxy-defaults',
    destination: '/consul/docs/reference/config-entry/proxy-default',
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
    source: '/consul/docs/connect/config-entries/terminating-gateway',
    destination: '/consul/docs/reference/config-entry/terminating-gateway',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/config-entries/control-plane-request-limit',
    destination:
      '/consul/docs/reference/config-entry/control-plane-request-limit',
    permanent: true,
  },

  {
    source: '/consul/docs/security/acl/acl-rules',
    destination: '/consul/docs/reference/acl/rule',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/kubernetes',
    destination: '/consul/docs/reference/acl/auth-method/k8s',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/jwt',
    destination: '/consul/docs/reference/acl/auth-method/jwt',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/oidc',
    destination: '/consul/docs/reference/acl/auth-method/oidc',
    permanent: true,
  },
  {
    source: '/consul/docs/security/acl/auth-methods/aws-iam',
    destination: '/consul/docs/reference/acl/auth-method/aws-iam',
    permanent: true,
  },

  {
    source: '/consul/docs/k8s/annotations-and-labels',
    destination: '/consul/docs/reference/k8s/annotation-label',
    permanent: true,
  },

  {
    source: '/consul/docs/k8s/helm',
    destination: '/consul/docs/reference/k8s/helm',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration',
    destination: '/consul/docs/reference/k8s/api-gateway',
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
      '/consul/docs/connect/gateways/api-gateway/configuration/routeauthfilter',
    destination: '/consul/docs/reference/k8s/api-gateway/routeauthfilter',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/gateways/api-gateway/configuration/routes',
    destination: '/consul/docs/reference/k8s/api-gateway/routes',
    permanent: true,
  },
  {
    source:
      '/consul/docs/connect/gateways/api-gateway/configuration/routeretryfilter',
    destination: '/consul/docs/reference/k8s/api-gateway/routeretryfilter',
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
      '/consul/docs/connect/gateways/api-gateway/configuration/meshservice',
    destination: '/consul/docs/reference/k8s/api-gateway/mesh-service',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/configuration-reference',
    destination: '/consul/docs/reference/ecs',
    permanent: true,
  },
  {
    source: '/consul/docs/ecs/reference/consul-server-json',
    destination: '/consul/docs/reference/ecs/server-json',
    permanent: true,
  },
  {
    source: '/consul/docs',
    destination: '/consul/docs/reference/cts',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api',
    destination: '/consul/docs/reference/cts/api',
    permanent: true,
  },
  {
    source: '/consul/docs',
    destination: '/consul/docs/reference/cts/api/status',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api/tasks',
    destination: '/consul/docs/reference/cts/api/tasks',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/api/health',
    destination: '/consul/docs/reference/cts/api/health',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli',
    destination: '/consul/docs/reference/cts/cli',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli/task',
    destination: '/consul/docs/reference/cts/cli/task',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/cli/start',
    destination: '/consul/docs/reference/cts/cli/start',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/configuration',
    destination: '/consul/docs/reference/cts/configuration',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/tasks',
    destination: '/consul/docs/reference/cts/task',
    permanent: true,
  },
  {
    source: '/consul/docs/nia/terraform-modules',
    destination: '/consul/docs/reference/cts/terraform-module',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/built-in',
    destination: '/consul/docs/reference/proxy/built-in',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/proxy-config-reference',
    destination: '/consul/docs/reference/proxy/connect-proxy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/envoy',
    destination: '/consul/docs/reference/proxy/envoy',
    permanent: true,
  },
  {
    source: '/consul/docs/connect/proxies/deploy-sidecar-services',
    destination: '/consul/docs/reference/proxy/sidecar',
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
    source: '/consul/docs/k8s/multiport/reference/grpcroute',
    destination: '/consul/docs/reference/resource/grpcroute',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/reference/httproute',
    destination: '/consul/docs/reference/resource/httproute',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/reference/proxyconfiguration',
    destination: '/consul/docs/reference/resource/proxyconfiguration',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/reference/tcproute',
    destination: '/consul/docs/reference/resource/tcproute',
    permanent: true,
  },
  {
    source: '/consul/docs/k8s/multiport/reference/trafficpermissions',
    destination: '/consul/docs/reference/resource/trafficpermissions',
    permanent: true,
  },

  // Links to previous versions redirect correctly

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/use-case/service-discovery/:slug*',
    destination: '/consul/docs/:version/concepts/service-discovery/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/use-case/service-mesh/:slug*',
    destination: '/consul/docs/:version/concepts/service-mesh/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/why/:slug*',
    destination: '/consul/docs/:version/consul-vs-other/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/why/service-mesh/:slug*',
    destination:
      '/consul/docs/:version/consul-vs-other/service-mesh-compare/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/why/dns/:slug*',
    destination:
      '/consul/docs/:version/consul-vs-other/dns-tools-compare/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/why/config-management/:slug*',
    destination:
      '/consul/docs/:version/consul-vs-other/config-management-compare/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/why/api-gateway/:slug*',
    destination:
      '/consul/docs/:version/consul-vs-other/api-gateway-compare/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/concepts/service-mesh/:slug*',
    destination: '/consul/docs/:version/connect/connect-internals/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/ports/:slug*',
    destination: '/consul/docs/:version/install/ports/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/fips/:slug*',
    destination: '/consul/docs/:version/enterprise/fips/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/vm/bootstrap/:slug*',
    destination: '/consul/docs/:version/install/bootstrapping/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/vm/cloud-auto-join/:slug*',
    destination: '/consul/docs/:version/install/cloud-auto-join/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/vm/requirements/:slug*',
    destination: '/consul/docs/:version/install/performance/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/requirements/:slug*',
    destination: '/consul/k8s/compatibility/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/plaform/minikube/:slug*',
    destination: '/consul/tutorials/kubernetes/kubernetes-minikube/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/kind/:slug*',
    destination: '/consul/tutorials/kubernetes/kubernetes-kind/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/aks/:slug*',
    destination: '/consul/tutorials/kubernetes/kubernetes-aks-azure/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/eks/:slug*',
    destination: '/consul/tutorials/kubernetes/kubernetes-eks-aws/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/gke/:slug*',
    destination: '/consul/tutorials/kubernetes/kubernetes-gke-google/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/openshift/:slug*',
    destination:
      '/consul/tutorials/kubernetes/kubernetes-openshift-red-hat/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/platform/self-hosted/:slug*',
    destination:
      '/consul/docs/:version/k8s/platforms/self-hosted-kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/servers-outside/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/servers-outside-kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/multiple-clusters/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/single-dc-multi-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/enterprise/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/consul-enterprise/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/federation/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/federation/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/federation/k8s-vm/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/system-integration/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/systems-integration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/bootstrap-token/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/bootstrap-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/enterprise-license/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/enterprise-license/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/gossip-encryption-key/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/gossip/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/partition-token/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/partition-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/replication-token/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/replication-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/server-tls/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/server-tls/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/service-mesh-certificate/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/connect-ca/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/snapshot-agent/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/snapshot-agent-config/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/data-integration/webhook-certificate/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/data-integration/webhook-certs/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/k8s/examples/vault/wan/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/vault/wan-federation/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/wal//:slug*',
    destination: '/consul/docs/:version/agent/wal-logstore/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/wal/enable/:slug*',
    destination: '/consul/docs/:version/agent/wal-logstore/enable/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/wal/monitor-raft/:slug*',
    destination: '/consul/docs/:version/agent/wal-logstore/monitoring/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/wal/revert-boltdb/:slug*',
    destination:
      '/consul/docs/:version/agent/wal-logstore/revert-to-boltdb/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/control-plane/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/clients-outside-kubernetes/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/service-mesh/enable/:slug*',
    destination: '/consul/docs/:version/connect/configuration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/service-mesh/deploy-proxy/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-service-mesh-proxies/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/service-mesh/deploy-sidecar/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-sidecar-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/scale/:slug*',
    destination: '/consul/docs/:version/architecture/scale/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul//:slug*',
    destination: '/consul/docs/:version/security/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/:slug*',
    destination: '/consul/docs/:version/security/acl/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/:slug*',
    destination: '/consul/docs/:version/security/acl/tokens/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/policies/:slug*',
    destination: '/consul/docs/:version/security/acl/acl-policies/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/rule/:slug*',
    destination: '/consul/docs/:version/security/acl/acl-roles/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/rules/:slug*',
    destination: '/consul/docs/:version/security/acl/acl-rules/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/auth-methods/:slug*',
    destination: '/consul/docs/:version/security/acl/auth-methods/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/auth-methods/k8s/:slug*',
    destination:
      '/consul/docs/:version/security/acl/auth-methods/kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/auth-methods/jwt/:slug*',
    destination: '/consul/docs/:version/security/acl/auth-methods/jwt/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/auth-methods/oidc/:slug*',
    destination: '/consul/docs/:version/security/acl/auth-methods/oidc/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/auth-methods/aws-iam/:slug*',
    destination:
      '/consul/docs/:version/security/acl/auth-methods/aws-iam/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/federation/:slug*',
    destination:
      '/consul/docs/:version/security/acl/acl-federated-datacenters/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/service-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-service-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/agent-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-an-agent-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/ui-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-ui-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/mesh-gateway-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-mesh-gateway-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/ingress-gateway-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-an-ingress-gateway-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/terminating-gateway-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-terminating-gateway-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/dns-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-dns-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/replication-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-replication-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/snapshot-agent-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-snapshot-agent-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/vault-storage-backend-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-token-for-vault-consul-storage/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/token/esm-token/:slug*',
    destination:
      '/consul/docs/:version/security/acl/tokens/create/create-a-consul-esm-token/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/legacy/:slug*',
    destination:
      '/consul/docs/:version/consul/tutorials/security-operations/access-control-token-migration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/acl/troubleshoot/:slug*',
    destination:
      '/consul/tutorials/security/access-control-manage-policies/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/:slug*',
    destination: '/consul/docs/:version/security/encryption/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/gossip/enable/:slug*',
    destination: '/consul/tutorials/security/gossip-encryption-secure/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/gossip/existing/:slug*',
    destination:
      '/consul/consul/tutorials/security-operations/tls-encryption-secure-existing-datacenter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/tls/builtin/:slug*',
    destination: '/consul/tutorials/security/tls-encryption-secure/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/tls/openssl/:slug*',
    destination:
      '/consul/tutorials/security-operations/tls-encryption-openssl-secure/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/encryption/tls/existing/:slug*',
    destination:
      '/consul/tutorials/security-operations/tls-encryption-secure-existing-datacenter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/security-model/:slug*',
    destination: '/consul/docs/:version/security/security-models/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/security-model/consul/:slug*',
    destination: '/consul/docs/:version/security/security-models/core/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-consul/security-model/nia/:slug*',
    destination: '/consul/docs/:version/security/security-models/nia/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/monitor-consul/telemetry//:slug*',
    destination: '/consul/docs/:version/agent/telemetry/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/monitor-consul/log/audit-log/:slug*',
    destination: '/consul/docs/:version/enterprise/audit-logging/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/automated-backups/:slug*',
    destination: '/consul/docs/:version/enterprise/backups/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/redundancy-zones/:slug*',
    destination: '/consul/docs/:version/enterprise/redundancy/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/read-replicas/:slug*',
    destination: '/consul/docs/:version/enterprise/read-scale/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/agent-rate-limit/:slug*',
    destination: '/consul/docs/:version/agent/limits/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/agent-rate-limit/initialize/:slug*',
    destination:
      '/consul/docs/:version/agent/limits/usage/init-rate-limits/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/agent-rate-limit/monitor/:slug*',
    destination:
      '/consul/docs/:version/agent/limits/usage/monitor-rate-limits/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/agent-rate-limit/set-global/:slug*',
    destination:
      '/consul/docs/:version/agent/limits/usage/set-global-traffic-rate-limits/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-consul/agent-rate-limit/source-ip-addresses/:slug*',
    destination:
      '/consul/docs/:version/agent/limits/usage/limit-request-rates-from-ips/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/:slug*',
    destination: '/consul/docs/:version/upgrading/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/automated/:slug*',
    destination: '/consul/docs/:version/enterprise/upgrades/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/compatibility-promise/:slug*',
    destination: '/consul/docs/:version/upgrading/compatibility/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/version-specific/:slug*',
    destination: '/consul/docs/:version/upgrading/upgrade-specific/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/:slug*',
    destination: '/consul/docs/:version/upgrading/instructions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/general/:slug*',
    destination:
      '/consul/docs/:version/upgrading/instructions/general-process/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/1-10-x/:slug*',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-10-x/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/1-8-x/:slug*',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-8-x/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/1-6-x/:slug*',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-6-x/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/instructions/1-2-x/:slug*',
    destination:
      '/consul/docs/:version/upgrading/instructions/upgrade-to-1-2-x/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/k8s/:slug*',
    destination: '/consul/docs/:version/k8s/upgrade/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/k8s/compatibility/:slug*',
    destination: '/consul/docs/:version/k8s/compatibility/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/k8s/consul-k8s/:slug*',
    destination: '/consul/docs/:version/k8s/upgrade/upgrade-cli/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/k8s/crds/:slug*',
    destination: '/consul/docs/:version/k8s/crds/upgrade-to-crds/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/ecs/:slug*',
    destination: '/consul/docs/:version/ecs/compatibility/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/ecs/dataplane/:slug*',
    destination: '/consul/docs/:version/ecs/upgrade-to-dataplanes/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/release-notes/api-gateway/:slug*',
    destination: '/consul/docs/:version/release-notes/consul-api-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/release-notes/ecs/:slug*',
    destination: '/consul/docs/:version/release-notes/consul-ecs/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/:slug*',
    destination: '/consul/docs/:version/services/services/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/vm/define/:slug*',
    destination: '/consul/docs/:version/services/usage/define-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/vm/health-checks/:slug*',
    destination: '/consul/docs/:version/services/usage/checks/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/vm/:slug*',
    destination:
      '/consul/docs/:version/services/usage/register-services-checks/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/k8s/service-sync/:slug*',
    destination: '/consul/docs/:version/k8s/service-sync/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/ecs/requirements/:slug*',
    destination: '/consul/docs/:version/ecs/tech-specs/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/ecs/:slug*',
    destination: '/consul/docs/:version/ecs/deploy/terraform/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/ecs/manual/:slug*',
    destination: '/consul/docs/:version/ecs/deploy/manual/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/ecs/migrate/:slug*',
    destination:
      '/consul/docs/:version/ecs/deploy/migrate-existing-tasks/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/ecs/configure-task-bind-address/:slug*',
    destination: '/consul/docs/:version/ecs/deploy/bind-addresses/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/deploy/server/ecs/:slug*',
    destination: '/consul/docs/:version/ecs/enterprise/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/lambda/:slug*',
    destination: '/consul/docs/:version/lambda/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/lambda/automatic/:slug*',
    destination: '/consul/docs/:version/lambda/registration/automate/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/service/lambda/manual/:slug*',
    destination: '/consul/docs/:version/lambda/registration/manual/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/external/terminating-gateway/k8s/:slug*',
    destination: '/consul/docs/:version/k8s/connect/terminating-gateways/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/register/external/permissive-mtls/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/onboarding-tproxy-mode/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/discover/dns/:slug*',
    destination: '/consul/docs/:version/services/discovery/dns-overview/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/discover/dns/configure/:slug*',
    destination:
      '/consul/docs/:version/services/discovery/dns-configuration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/discover/dns/static-lookup/:slug*',
    destination:
      '/consul/docs/:version/services/discovery/dns-static-lookups/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/discover/dns/dynamic-lookup/:slug*',
    destination:
      '/consul/docs/:version/services/discovery/dns-dynamic-lookups/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/discover/dns/k8s/:slug*',
    destination: '/consul/docs/:version/k8s/dns/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)LINK TO /concept/service-mesh/:slug*',
    destination: '/consul/docs/:version/connect/connect-internals/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/configuration-entries/:slug*',
    destination: '/consul/docs/:version/connect/configuration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/multi-port/:slug*',
    destination: '/consul/docs/:version/k8s/multiport/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/multi-port/configure/:slug*',
    destination: '/consul/docs/:version/k8s/multiport/configure/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/lambda/invoke-function/:slug*',
    destination: '/consul/docs/:version/lambda/invocation/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/lambda/invoke-service/:slug*',
    destination: '/consul/docs/:version/lambda/invoke-from-lambda/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/proxy/:slug*',
    destination: '/consul/docs/:version/connect/proxies/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/proxy/deploy-service-mesh/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-service-mesh-proxies/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/proxy/deploy-sidecar/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-sidecar-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/proxy/transparent-proxy/:slug*',
    destination: '/consul/docs/:version/k8s/connect/transparent-proxy/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/proxy/transparent-proxy/enable/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/transparent-proxy/enable-transparent-proxy/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/troubleshoot/develop-debug/:slug*',
    destination: '/consul/docs/:version/connect/dev/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/troubleshoot/service-to-service/:slug*',
    destination:
      '/consul/docs/:version/troubleshoot/troubleshoot-services/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/ecs/:slug*',
    destination: '/consul/docs/:version/ecs/deploy/configure-routes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/connect/lambda/:slug*',
    destination: '/consul/docs/:version/lambda/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/:slug*',
    destination: '/consul/docs/:version/gateways/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway//:slug*',
    destination: '/consul/docs/:version/gateways/api-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/tech-specs/:slug*',
    destination: '/consul/docs/:version/gateways/api-gateway/tech-specs/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/install/:slug*',
    destination: '/consul/docs/:version/gateways/api-gateway/install-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/upgrade/api-gateway/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/upgrades-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/deploy-listener/vm/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/deploy/listeners-vms/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/deploy-listener/k8s/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/deploy/listeners-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/define-route/vm/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/define-routes/routes-vms/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/define-route/k8s/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/define-routes/routes-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/define-route/reroute/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/define-routes/reroute-http-requests/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/route/peer/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/define-routes/route-to-peered-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/secure/encrypt/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/secure-traffic/encrypt-vms/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/secure/jwt-vm/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/secure-traffic/verify-jwts-vms/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/api-gateway/secure/jwt-k8s/:slug*',
    destination:
      '/consul/docs/:version/gateways/api-gateway/secure-traffic/verify-jwts-k8s/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/common-error-messages/api-gateway/:slug*',
    destination: '/consul/docs/:version/gateways/api-gateway/errors/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/ingress-gateway/:slug*',
    destination: '/consul/docs/:version/gateways/ingress-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/ingress-gateway/create/vm/:slug*',
    destination: '/consul/docs/:version/gateways/ingress-gateway/usage/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/ingress-gateway/create/vm/:slug*',
    destination: '/consul/docs/:version/gateways/ingress-gateway/usage/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/access/ingress-gateway/tls-external-service/:slug*',
    destination:
      '/consul/docs/:version/gateways/ingress-gateway/tls-external-service/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/segment/admin-partition/:slug*',
    destination: '/consul/docs/:version/enterprise/admin-partitions/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/segment/namespace/:slug*',
    destination: '/consul/docs/:version/enterprise/namespaces/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/sameness-group/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/create-sameness-groups/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/segment/network-segment/:slug*',
    destination:
      '/consul/docs/:version/enterprise/network-segments/network-segments-overview/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/segment/network-segment/vm/:slug*',
    destination:
      '/consul/docs/:version/enterprise/network-segments/create-network-segment/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/:slug*',
    destination: '/consul/docs/:version/connect/cluster-peering/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/tech-specs/:slug*',
    destination:
      '/consul/docs/:version/connect/cluster-peering/tech-specs/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/establish-connection/vm/:slug*',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/establish-cluster-peering/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/establish-connection/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/establish-peering/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/manage-connection/vm/:slug*',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/manage-connections/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/cluster-peering/manage-connection/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/manage-peering/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/network-area/:slug*',
    destination:
      '/consul/tutorials/datacenter-operations/federation-network-areas/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/wan-federation//:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/wan-federation/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/wan-federation/k8s-vm/:slug*',
    destination:
      '/consul/docs/:version/k8s/deployment-configurations/multi-cluster/vms-and-kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/mesh-gateway//:slug*',
    destination: '/consul/docs/:version/connect/gateways/mesh-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/mesh-gateway/enable/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/wan-federation-via-mesh-gateways/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/mesh-gateway/federation/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/service-to-service-traffic-wan-datacenters/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/mesh-gateway/cluster-peer/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/peering-via-mesh-gateways/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/link/mesh-gateway/admin-partition/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/mesh-gateway/service-to-service-traffic-partitions/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/intentions/:slug*',
    destination: '/consul/docs/:version/connect/intentions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/intentions/create/:slug*',
    destination:
      '/consul/docs/:version/connect/intentions/create-manage-intentions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/intentions/jwt/:slug*',
    destination:
      '/consul/docs/:version/connect/intentions/jwt-authorization/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/:slug*',
    destination: '/consul/docs/:version/connect/ca/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/bootstrap/vm/:slug*',
    destination:
      '/consul/tutorials/developer-mesh/service-mesh-production-checklist#bootstrap-certificate-authority-for-consul-service-mesh/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/rotate/tls/:slug*',
    destination:
      '/consul/docs/:version/k8s/operations/certificate-rotation/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/rotate/gossip/:slug*',
    destination:
      '/consul/docs/:version/k8s/operations/gossip-encryption-key-rotation/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/existing/:slug*',
    destination:
      '/consul/docs/:version/k8s/operations/tls-on-existing-cluster/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/built-in/:slug*',
    destination: '/consul/docs/:version/connect/ca/consul/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/vault/:slug*',
    destination: '/consul/docs/:version/connect/ca/vault/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/acm/:slug*',
    destination: '/consul/docs/:version/connect/ca/aws/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/secure-mesh/certificate/enable-permissive-mtls/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/onboarding-tproxy-mode#enable-permissive-mtls-mode/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/:slug*',
    destination: '/consul/docs/:version/manage-traffic/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/route-local/:slug*',
    destination:
      '/consul/docs/:version/manage-traffic/route-to-local-upstreams/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/limit-rate/:slug*',
    destination:
      '/consul/docs/:version/manage-traffic/limit-request-rates/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/cluster-peering/vm/:slug*',
    destination:
      '/consul/docs/:version/connect/cluster-peering/usage/peering-traffic-management/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/cluster-peering/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/cluster-peering/usage/l7-traffic/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/failover/:slug*',
    destination: '/consul/docs/:version/connect/failover/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/failover/configure-k8s/:slug*',
    destination: '/consul/docs/:version/k8s/l7-traffic/failover-tproxy/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/failover/prepared-query/:slug*',
    destination:
      '/consul/tutorials/developer-discovery/automate-geo-failover/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/failover/virtual-services/:slug*',
    destination: '/consul/docs/:versiontraffic/route-to-virtual-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/manage-traffic/failover/sameness-group/:slug*',
    destination: '/consul/docs/:version/manage-traffic/failover/sameness/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/observe/access-logs/:slug*',
    destination:
      '/consul/docs/:version/connect/observability/access-logs/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/observe/mesh/vm/:slug*',
    destination:
      '/consul/docs/:version/connect/observability/ui-visualization/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/observe/mesh/k8s/:slug*',
    destination:
      '/consul/docs/:version/k8s/connect/observability/metrics/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/observe/distributed-tracing/:slug*',
    destination: '/consul/docs/:version/connect/distributed-tracing/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/dynamic-app/kv/:slug*',
    destination: '/consul/docs/:version/dynamic-app-config/kv/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/dynamic-app/kv/store/:slug*',
    destination:
      '/consul/tutorials/interactive/get-started-key-value-store/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/dynamic-app/sessions/:slug*',
    destination: '/consul/docs/:version/dynamic-app-config/sessions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/dynamic-app/watches/:slug*',
    destination: '/consul/docs/:version/dynamic-app-config/watches/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/enterprise/license/:slug*',
    destination: '/consul/docs/:version/enterprise/license/overview/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/enterprise/license/automated-entitlement-utilization/:slug*',
    destination:
      '/consul/docs/:version/enterprise/license/utilization-reporting/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/integrate/consul-tools/:slug*',
    destination: '/consul/docs/:version/integrate/download-tools/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/integrate/consul/:slug*',
    destination: '/consul/docs/:version/integrate/partnerships/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/integrate/nia/:slug*',
    destination: '/consul/docs/:version/integrate/nia-integration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/integrate/proxy/:slug*',
    destination: '/consul/docs/:version/connect/proxies/integrate/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension//:slug*',
    destination: '/consul/docs/:version/connect/proxies/envoy-extensions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/apigee-external-authz/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/apigee-ext-authz/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/external-authz/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/ext-authz/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/lua/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/lua/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/lambda/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/lambda/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/configure-envoy/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/property-override/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/otel-access-logging/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/otel-access-logging/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/envoy-extension/wasm/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/usage/wasm/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/:slug*',
    destination: '/consul/docs/:version/nia/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/architecture/:slug*',
    destination: '/consul/docs/:versionhitecture/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/requirements/:slug*',
    destination: '/consul/docs/:version/nia/usage/requirements/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/deploy/install/:slug*',
    destination: '/consul/docs/:version/nia/installation/install/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/:slug*',
    destination: '/consul/docs/:version/nia/installation/configure/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/task/:slug*',
    destination: '/consul/docs/:version/nia/tasks/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/network-driver/:slug*',
    destination: '/consul/docs/:version/nia/network-drivers/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/network-driver/terraform/:slug*',
    destination: '/consul/docs/:version/nia/network-drivers/terraform/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/network-driver/terraform-cloud/:slug*',
    destination:
      '/consul/docs/:version/nia/network-drivers/terraform-cloud/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/configure/license/:slug*',
    destination: '/consul/docs/:version/nia/enterprise/license/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/create-terraform-module/:slug*',
    destination:
      '/consul/docs/:version/nia/terraform-modules#how-to-create-a-compatible-terraform-module/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/compatibility/:slug*',
    destination: '/consul/docs/:version/nia/compatibility/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/cts/enterprise/:slug*',
    destination: '/consul/docs/:version/nia/enterprise/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)reference-cli/consul-k8s/:slug*',
    destination: '/consul/docs/:version/k8s/k8s-cli/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)reference-cli/consul-dataplane/:slug*',
    destination:
      '/consul/docs/:version/connect/dataplane/consul-dataplane/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/error-messages/consul/:slug*',
    destination: '/consul/docs/:version/troubleshoot/common-errors/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/error-messages/k8s/:slug*',
    destination:
      '/consul/docs/:version/troubleshoot/common-errors#common-errors-on-kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/error-messages/api-gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/errors/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/error-messages/cts/:slug*',
    destination: '/consul/docs/:version/nia/usage/errors-ref/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/glossary/:slug*',
    destination: '/consul/docs/:version/install/glossary/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/agent/:slug*',
    destination: '/consul/docs/:version/agent/config/config-files/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/service/:slug*',
    destination:
      '/consul/docs/:version/services/configuration/services-configuration-reference/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/service/health-checks/:slug*',
    destination:
      '/consul/docs/:version/services/configuration/checks-configuration-reference/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/api-gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/api-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/http-route/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/http-route/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/tcp-route/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/tcp-route/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/inline-certificate/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/inline-certificate/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/ingress-gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/ingress-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/jwt-provider/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/jwt-provider/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/mesh/:slug*',
    destination: '/consul/docs/:version/connect/config-entries/mesh/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/exported-services/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/exported-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/proxy-default/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/proxy-defaults/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/sameness-group/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/sameness-group/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/service-defaults/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/service-defaults/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/service-intentions/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/service-intentions/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/service-resolver/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/service-resolver/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/service-router/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/service-router/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/service-splitter/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/service-splitter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/terminating-gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/terminating-gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/config-entry/control-plane-request-limit/:slug*',
    destination:
      '/consul/docs/:version/connect/config-entries/control-plane-request-limit/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/rule/:slug*',
    destination: '/consul/docs/:version/security/acl/acl-rules/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/auth-method/k8s/:slug*',
    destination:
      '/consul/docs/:version/security/acl/auth-methods/kubernetes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/auth-method/jwt/:slug*',
    destination: '/consul/docs/:version/security/acl/auth-methods/jwt/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/auth-method/oidc/:slug*',
    destination: '/consul/docs/:version/security/acl/auth-methods/oidc/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/acl/auth-method/aws-iam/:slug*',
    destination:
      '/consul/docs/:version/security/acl/auth-methods/aws-iam/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/annotation-label/:slug*',
    destination: '/consul/docs/:version/k8s/annotations-and-labels/:slug',
    permanent: true,
  },

  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/helm/:slug*',
    destination: '/consul/docs/:version/k8s/helm/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/gateway/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gateway/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/gatewayclass/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewayclass/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/gatewayclassconfig/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewayclassconfig/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/gatewaypolicy/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/gatewaypolicy/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/routeauthfilter/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routeauthfilter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/routes/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routes/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/routeretryfilter/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routeretryfilter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/routetimeoutfilter/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/routetimeoutfilter/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/k8s/api-gateway/mesh-service/:slug*',
    destination:
      '/consul/docs/:version/connect/gateways/api-gateway/configuration/meshservice/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/ecs/:slug*',
    destination: '/consul/docs/:version/ecs/configuration-reference/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/ecs/server-json/:slug*',
    destination: '/consul/docs/:version/ecs/reference/consul-server-json/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/:slug*',
    destination: '/consul/docs/:version/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/api/:slug*',
    destination: '/consul/docs/:version/nia/api/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/api/status/:slug*',
    destination: '/consul/docs/:version/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/api/tasks/:slug*',
    destination: '/consul/docs/:version/nia/api/tasks/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/api/health/:slug*',
    destination: '/consul/docs/:version/nia/api/health/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/cli/:slug*',
    destination: '/consul/docs/:version/nia/cli/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/cli/task/:slug*',
    destination: '/consul/docs/:version/nia/cli/task/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/cli/start/:slug*',
    destination: '/consul/docs/:version/nia/cli/start/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/configuration/:slug*',
    destination: '/consul/docs/:version/nia/configuration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/task/:slug*',
    destination: '/consul/docs/:version/nia/tasks/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/cts/terraform-module/:slug*',
    destination: '/consul/docs/:version/nia/terraform-modules/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/built-in/:slug*',
    destination: '/consul/docs/:version/connect/proxies/built-in/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/connect-proxy/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/proxy-config-reference/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/envoy/:slug*',
    destination: '/consul/docs/:version/connect/proxies/envoy/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/sidecar/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/deploy-sidecar-services/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/extensions/ext-authz/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/ext-authz/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/extensions/otel/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/otel-access-logging/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/extensions/property-override/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/property-override/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/proxy/extensions/wasm/:slug*',
    destination:
      '/consul/docs/:version/connect/proxies/envoy-extensions/configuration/wasm/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/resource/grpcroute/:slug*',
    destination:
      '/consul/docs/:version/k8s/multiport/reference/grpcroute/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/resource/httproute/:slug*',
    destination:
      '/consul/docs/:version/k8s/multiport/reference/httproute/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/resource/proxyconfiguration/:slug*',
    destination:
      '/consul/docs/:version/k8s/multiport/reference/proxyconfiguration/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/resource/tcproute/:slug*',
    destination: '/consul/docs/:version/k8s/multiport/reference/tcproute/:slug',
    permanent: true,
  },
  {
    source:
      '/consul/docs/:version(v1.(?:8|9|10|11|12|13|14|15|16|17).x)/reference/resource/trafficpermissions/:slug*',
    destination:
      '/consul/docs/:version/k8s/multiport/reference/trafficpermissions/:slug',
    permanent: true,
  },
]
