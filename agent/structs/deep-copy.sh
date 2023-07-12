#!/usr/bin/env bash

readonly PACKAGE_DIR="$(dirname "${BASH_SOURCE[0]}")"
cd $PACKAGE_DIR

# Uses: https://github.com/globusdigital/deep-copy
deep-copy \
  -pointer-receiver \
  -o ./structs.deepcopy.go \
  -type CARoot \
  -type CheckServiceNode \
  -type CheckType \
  -type CompiledDiscoveryChain \
  -type ConnectProxyConfig \
  -type DiscoveryFailover \
  -type DiscoveryGraphNode \
  -type DiscoveryResolver \
  -type DiscoveryRoute \
  -type DiscoverySplit \
  -type ExposeConfig \
  -type GatewayService \
  -type GatewayServiceTLSConfig \
  -type HTTPHeaderModifiers \
  -type HashPolicy \
  -type HealthCheck \
  -type IndexedCARoots \
  -type IngressListener \
  -type Intention \
  -type IntentionPermission \
  -type LoadBalancer \
  -type MeshConfigEntry \
  -type MeshDirectionalTLSConfig \
  -type MeshTLSConfig \
  -type Node \
  -type NodeService \
  -type PeeringServiceMeta \
  -type ServiceConfigEntry \
  -type ServiceConfigResponse \
  -type ServiceConnect \
  -type ServiceDefinition \
  -type ServiceResolverConfigEntry \
  -type ServiceResolverFailover \
  -type ServiceRoute \
  -type ServiceRouteDestination \
  -type ServiceRouteMatch \
  -type Upstream \
  -type UpstreamConfiguration \
  ./
