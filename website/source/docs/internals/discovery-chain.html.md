---
layout: "docs"
page_title: "Discovery Chain"
sidebar_current: "docs-internals-discovery-chain"
description: |-
  The service discovery process can be modeled as a "discovery chain" which passes through three distinct stages: routing, splitting, and resolution. Each of these stages is controlled by a set of configuration entries.
---

-> **1.6.0+:**  This feature is available in Consul versions 1.6.0 and newer.

# Discovery Chain

~> This topic is part of a [low-level API](/api/discovery-chain.html)
primarily targeted at developers building external [Connect proxy
integrations](/docs/connect/proxies/integrate.html).

The service discovery process can be modeled as a "discovery chain" which
passes through three distinct stages: routing, splitting, and resolution. Each
of these stages is controlled by a set of [configuration
entries](/docs/agent/config_entries.html). By configuring different phases of
the discovery chain a user can control how proxy upstreams are ultimately
resolved to specific instances for load balancing.

-> **Note:** The discovery chain is currently only used to discover
[Connect](/docs/connect/index.html) proxy upstreams.

## Configuration

The configuration entries used in the discovery chain are designed to be simple
to read and modify for narrowly tailored changes, but at discovery-time the
various configuration entries interact in more complex ways. For example:

* If a [`service-resolver`](/docs/agent/config-entries/service-resolver.html)
  is created with a [service
  redirect](/docs/agent/config-entries/service-resolver.html#service) defined,
  then all references made to the original service in any other configuration
  entry is replaced with the redirect destination.

* If a [`service-resolver`](/docs/agent/config-entries/service-resolver.html)
  is created with a [default
  subset](/docs/agent/config-entries/service-resolver.html#defaultsubset)
  defined then all references made to the original service in any other
  configuration entry that did not specify a subset will be replaced with the
  default.

* If a [`service-splitter`](/docs/agent/config-entries/service-splitter.html)
  is created with a [service
  split](/docs/agent/config-entries/service-splitter.html#splits), and the target service has its
  own `service-splitter` then the overall effect is flattened and only a single 
  aggregate traffic split is ultimately configured in the proxy.

* [`service-resolver`](/docs/agent/config-entries/service-resolver.html)
  redirect loops must be rejected as invalid.

* [`service-router`](/docs/agent/config-entries/service-router.html) and
  [`service-splitter`](/docs/agent/config-entries/service-splitter.html)
  configuration entries require an L7 compatible protocol be set for the
  service via either a
  [`service-defaults`](/docs/agent/config-entries/service-defaults.html) or
  [`proxy-defaults`](/docs/agent/config-entries/proxy-defaults.html) config
  entry. Violations must be rejected as invalid.

* If an [upstream
  configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
  [`datacenter`](/docs/connect/registration/service-registration.html#datacenter)
  parameter is defined then any configuration entry that does not explicitly
  refer to a desired datacenter should use that value from the upstream.

## Compilation

To correctly interpret a collection of configuration entries as a valid
discovery chain, we first compile them into a form more directly usable by the
layers responsible for configuring Connect sidecar proxies.

You can interact with the compiler directly using the [discovery-chain
API](/api/discovery-chain.html).

### Compilation Parameters

* **Service Name** - The service being discovered by name.
* **Datacenter** - The datacenter to use as the basis of compilation. 
* **Overrides** - Discovery-time tweaks to apply when compiling. These should
  be derived from either the
  [proxy](/docs/connect/registration/service-registration.html#complete-configuration-example)
  or
  [upstream](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
  configurations if either are set.

### Compilation Results

The response is a single wrapped `CompiledDiscoveryChain` field:

```json
{
    "Chain": {...<CompiledDiscoveryChain>...}
}
```

#### `CompiledDiscoveryChain`

The chain encodes a digraph of [nodes](#discoverygraphnode) and
[targets](#discoverytarget). Nodes are the compiled representation of various
discovery chain stages and targets are instructions on how to use the [health
API](/api/health.html#list-nodes-for-connect-capable-service) to retrieve
relevant service instance lists.

You should traverse the nodes starting with [`StartNode`](#startnode). The
nodes can be resolved by name using the [`Nodes`](#nodes) field. Targets can be
resolved by name using the [`Targets`](#targets) field.

- `ServiceName` `(string)` - The requested service.
- `Namespace` `(string)` - The requested namespace.
- `Datacenter` `(string)` - The requested datacenter.

- `CustomizationHash` `(string: <optional>)` - A unique hash of any overrides
  that affected the compilation of the discovery chain.

    If set, this value should be used to prefix/suffix any generated load
    balancer data plane objects to avoid sharing customized and non-customized
    versions.

- `Protocol` `(string)` - The overall protocol shared by everything in the
  chain.

- `StartNode` `(string)` - The first key into the `Nodes` map that should be
  followed when traversing the discovery chain.

- `Nodes` `(map<string|DiscoveryGraphNode>)` - All nodes available for traversal in
  the chain keyed by a unique name. You can walk this by starting with
  `StartNode`.

    -> The names should be treated as opaque values and are only guaranteed to be
    consistent within a single compilation.

- `Targets` `(map<string|DiscoveryTarget>)` - A list of all targets used in this chain.

    -> The names should be treated as opaque values and are only guaranteed to be
    consistent within a single compilation.

#### `DiscoveryGraphNode`

A single node in the compiled discovery chain.

- `Type` `(string)` - The type of the node. Valid values are: `router`,
  `splitter`, and `resolver`.

- `Name` `(string)` - The unique name of the node.

- `Routes` `(array<DiscoveryRoute>)` - Only set for `Type:router`. List of routes to
  render.

  - `Definition` `(ServiceRoute)` - Relevant portion of underlying
    `service-router`
    [route](/docs/agent/config-entries/service-router.html#routes).

  - `NextNode` `(string)` - The name of the next node in the chain in [`Nodes`](#nodes).

- `Splits` `(array<DiscoverySplit>)` - Only set for `Type:splitter`. List of traffic
  splits.

  - `Weight` `(float32)` - Copy of underlying `service-splitter`
    [`weight`](/docs/agent/config-entries/service-splitter.html#weight) field.

  - `NextNode` `(string)` - The name of the next node in the chain in [`Nodes`](#nodes).

- `Resolver` `(DiscoveryResolver: <optional>)` - Only set for `Type:resolver`. How
  to resolve the service instances.

  - `Default` `(bool)` - Set to true if no `service-resolver` config entry is
    defined for this node and the default was synthesized.

  - `ConnectTimeout` `(duration)` - Copy of the underlying `service-resolver`
    [`ConnectTimeout`](/docs/agent/config-entries/service-resolver.html#connecttimeout)
    field. If one is not defined the default of `5s` is returned.

  - `Target` `(string)` - The name of the target to use found in [`Targets`](#targets).

  - `Failover` `(DiscoveryFailover: <optional>)` - Compiled form of the
    underlying `service-resolver`
    [`Failover`](/docs/agent/config-entries/service-resolver.html#failover)
    definition to use for this request.

    - `Targets` `(array<string>)` - List of targets found in
      [`Targets`](#targets) to failover to in order of preference.

#### `DiscoveryTarget`

- `ID` `(string)` - The unique name of this target.

- `Service` `(string)` - The service to query when resolving a list of service instances.

- `ServiceSubset` `(string: <optional>)` - The
  [subset](/docs/agent/config-entries/service-resolver.html#service-subsets) of
  the service to resolve.

- `Namespace` `(string)` - The namespace to use when resolving a list of service instances.

- `Datacenter` `(string)` - The datacenter to use when resolving a list of service instances.

- `Subset` `(ServiceResolverSubset)` - Copy of the underlying
  `service-resolver`
  [`Subsets`](/docs/agent/config-entries/service-resolver.html#subsets)
  definition for this target.

  - `Filter` `(string: "")` - The 
    [filter expression](/api/features/filtering.html) to be used for selecting
    instances of the requested service. If empty all healthy instances are
    returned.

  - `OnlyPassing` `(bool: false)` - Specifies the behavior of the resolver's
    health check interpretation. If this is set to false, instances with checks
    in the passing as well as the warning states will be considered healthy. If
    this is set to true, only instances with checks in the passing state will
    be considered healthy.

- `MeshGateway` `(MeshGatewayConfig)` - The [mesh gateway
  configuration](/docs/connect/mesh_gateway.html#connect-proxy-configuration)
  to use when connecting to this target's service instances.

  - `Mode` `(string: "")` - One of `none`, `local`, or `remote`.

- `External` `(bool: false)` - True if this target is outside of this consul cluster.

- `SNI` `(string)` - This value should be used as the
  [SNI](https://en.wikipedia.org/wiki/Server_Name_Indication) value when
  connecting to this set of endpoints over TLS.

- `Name` `(string)` - The unique name for this target for use when generating
  load balancer objects. This has a structure similar to [SNI](#sni), but will
  not be affected by SNI customizations such as
  [`ExternalSNI`](/docs/agent/config-entries/service-defaults.html#externalsni).
