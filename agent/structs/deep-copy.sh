#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


readonly PACKAGE_DIR="$(dirname "${BASH_SOURCE[0]}")"
cd $PACKAGE_DIR

# Uses: https://github.com/globusdigital/deep-copy
deep-copy \
  -pointer-receiver \
  -o ./structs.deepcopy.go \
  -type APIGatewayListener \
  -type BoundAPIGatewayListener \
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
  -type ExportedServicesConfigEntry \
  -type GatewayService \
  -type GatewayServiceTLSConfig \
  -type HTTPHeaderModifiers \
  -type HTTPRouteConfigEntry \
  -type HashPolicy \
  -type HealthCheck \
  -type IndexedCARoots \
  -type IngressListener \
  -type InlineCertificateConfigEntry \
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
  -type TCPRouteConfigEntry \
  -type Upstream \
  -type UpstreamConfiguration \
  -type Status \
  -type BoundAPIGatewayConfigEntry \
  ./
