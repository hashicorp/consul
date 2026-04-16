# Outlier Detection and EDS Relationship in Consul

## Overview

This document explains the relationship between **Outlier Detection** (passive health checking) and **EDS** (Endpoint Discovery Service) in Consul's service mesh implementation.

## What is EDS?

**EDS (Endpoint Discovery Service)** is part of Envoy's xDS (discovery service) protocol. It dynamically provides Envoy with the list of healthy endpoints (IP addresses and ports) for upstream services.

### Key Points:
- EDS is used when service instances are addressed by **IP addresses** (not hostnames)
- Endpoints are discovered and updated dynamically through the xDS protocol
- EDS updates must arrive **after** CDS (Cluster Discovery Service) updates

## The Critical Relationship

### 1. **Outlier Detection REQUIRES EDS**

From [`agent/xds/clusters.go:480-482`](agent/xds/clusters.go:480):
```go
// Endpoints are managed separately by EDS
// Having an empty config enables outlier detection with default config.
OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
```

**Why?** Outlier detection monitors the health of individual endpoints. It needs:
- Multiple endpoints to monitor
- Dynamic endpoint updates to track health status
- The ability to temporarily remove unhealthy endpoints from the pool

### 2. **When EDS is Used**

From [`agent/xds/clusters.go:1344-1365`](agent/xds/clusters.go:1344):
```go
useEDS := true
if _, ok := cfgSnap.ConnectProxy.PeerUpstreamEndpointsUseHostnames[uid]; ok {
    // If we're using local mesh gw, the fact that upstreams use hostnames don't matter.
    // If we're not using local mesh gw, then resort to CDS.
    if upstreamConfig.MeshGateway.Mode != structs.MeshGatewayModeLocal {
        useEDS = false
    }
}

// If none of the service instances are addressed by a hostname we
// provide the endpoint IP addresses via EDS
if useEDS {
    c.ClusterDiscoveryType = &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS}
    c.EdsClusterConfig = &envoy_cluster_v3.Cluster_EdsClusterConfig{
        EdsConfig: &envoy_core_v3.ConfigSource{
            InitialFetchTimeout: cfgSnap.GetXDSCommonConfig(s.Logger).GetXDSFetchTimeout(),
            ResourceApiVersion:  envoy_core_v3.ApiVersion_V3,
            ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
                Ads: &envoy_core_v3.AggregatedConfigSource{},
            },
        },
    }
}
```

**EDS is used when:**
- Service instances are addressed by IP addresses (not hostnames)
- Using local mesh gateway mode (even with hostnames)
- For peered services in certain configurations

**EDS is NOT used when:**
- Service instances use hostnames (DNS-based discovery)
- Using remote mesh gateway mode with hostname-based services

### 3. **Why Hostnames Don't Work with EDS**

From [`agent/xds/endpoints.go:223-224`](agent/xds/endpoints.go:223):
```go
// Also skip gateways with a hostname as their address. EDS cannot resolve hostnames,
// so we provide them through CDS instead.
```

**Reason:** EDS expects IP addresses, not DNS names. For hostname-based services, Consul embeds the endpoints directly in the CDS (Cluster Discovery Service) configuration.

## The Flow: Outlier Detection + EDS

### Step-by-Step Process:

1. **Configuration Phase**
   - User defines `PassiveHealthCheck` in service-defaults config entry
   - Consul stores this in `UpstreamConfig.PassiveHealthCheck`

2. **Cluster Generation (CDS)**
   - Consul's XDS server generates Envoy cluster configuration
   - Determines if EDS should be used (IP-based vs hostname-based)
   - If using EDS:
     ```go
     ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_EDS}
     OutlierDetection: config.ToOutlierDetection(passiveHealthCheck, override, allowZero)
     ```
   - Cluster is sent to Envoy via CDS

3. **Endpoint Generation (EDS)**
   - Consul generates endpoint list with IP addresses
   - Endpoints are sent to Envoy via EDS
   - **Must arrive AFTER CDS updates** (ordering requirement)

4. **Runtime Monitoring**
   - Envoy monitors traffic to each endpoint
   - Outlier detection tracks failures (5xx errors, timeouts, etc.)
   - Unhealthy endpoints are temporarily ejected from load balancing
   - Ejected endpoints are periodically re-tested and restored when healthy

### xDS Update Ordering

From [`agent/xds/delta.go:583-589`](agent/xds/delta.go:583):
```go
// 3. EDS updates (if any) must arrive after CDS updates for the respective clusters.
{TypeUrl: xdscommon.EndpointType, Upsert: true},
// 4. LDS updates must arrive after corresponding CDS/EDS updates.
{TypeUrl: xdscommon.ListenerType, Upsert: true, Remove: true},
```

**Critical:** EDS updates must come after CDS to ensure Envoy has the cluster configuration (including outlier detection settings) before receiving endpoints.

## Special Cases

### 1. Peered Services

From [`agent/xds/clusters.go:1316-1320`](agent/xds/clusters.go:1316):
```go
outlierDetection := config.ToOutlierDetection(upstreamConfig.PassiveHealthCheck, nil, true)
// We can't rely on health checks for services on cluster peers because they
// don't take into account service resolvers, splitters and routers. Setting
// MaxEjectionPercent to 100% gives outlier detection the power to eject the
// entire cluster.
```

**Special behavior:** For peered services, `MaxEjectionPercent` is set to 100% because:
- Traditional health checks don't work across cluster peers
- Outlier detection becomes the primary health mechanism
- Need ability to eject entire cluster if all instances are unhealthy

### 2. Ingress Gateways

From [`agent/xds/clusters.go:1198-1205`](agent/xds/clusters.go:1198):
```go
// Configure the outlier detector for upstream service
var override *structs.PassiveHealthCheck
if svc != nil {
    override = svc.PassiveHealthCheck
}
outlierDetection := config.ToOutlierDetection(cfgSnap.IngressGateway.Defaults.PassiveHealthCheck, override, false)
```

**Two-level configuration:**
- Gateway-level defaults apply to all upstreams
- Per-service overrides can customize behavior for specific services

## Debugging Tips

### 1. Enable Debug Logging
```bash
consul agent -dev -log-level=debug
```

### 2. Check Envoy Admin Interface
```bash
# View cluster configuration (includes outlier detection)
curl http://localhost:19000/config_dump | jq '.configs[1].dynamic_active_clusters'

# View endpoint health status
curl http://localhost:19000/clusters | grep -A 5 "outlier_detection"
```

### 3. Verify EDS is Being Used
Look for `ClusterDiscoveryType: EDS` in cluster config:
```bash
curl http://localhost:19000/config_dump | jq '.configs[1].dynamic_active_clusters[] | select(.cluster.name=="db") | .cluster.type'
```

### 4. Monitor Outlier Detection Events
```bash
# Check Envoy stats for ejections
curl http://localhost:19000/stats | grep outlier_detection
```

### 5. Use VS Code Debugger
Set breakpoints in:
- [`agent/xds/clusters.go:1316`](agent/xds/clusters.go:1316) - Where outlier detection is configured
- [`agent/xds/config/config.go:207`](agent/xds/config/config.go:207) - ToOutlierDetection conversion
- [`agent/xds/endpoints.go:32`](agent/xds/endpoints.go:32) - Endpoint generation

## Summary

**Outlier Detection and EDS are tightly coupled:**

1. **Outlier detection monitors individual endpoints** → Requires EDS to provide dynamic endpoint list
2. **EDS provides IP-based endpoints** → Enables outlier detection to track per-endpoint health
3. **Hostname-based services use CDS** → Cannot use outlier detection effectively (no dynamic endpoint tracking)
4. **xDS ordering matters** → CDS (with outlier config) must arrive before EDS (with endpoints)

**Key Takeaway:** For outlier detection to work properly, services must use IP-based discovery (EDS), not hostname-based discovery (CDS with embedded endpoints).