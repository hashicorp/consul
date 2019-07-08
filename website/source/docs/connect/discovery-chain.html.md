---
layout: "docs"
page_title: "Connect - Discovery Chain (beta)"
sidebar_current: "docs-connect-discovery_chain"
description: |-
  A discovery chain is a series of distinct stages that a service discovery request flows through to discover and configure Connect proxy upstreams of the service type.
---

# Discovery Chain <sup>(beta)</sup>

-> **Note:** This feature is not compatible with the 
[built-in proxy](/docs/connect/proxies/built-in.html)
or [native proxies](/docs/connect/native.html).

A discovery chain is a series of distinct stages that a service discovery
request flows through to discover and configure Connect proxy upstreams of the
`service` type.

![diagram of discovery chain stages](/assets/images/discovery-chain-simple.svg)

Each stage of this discovery chain can be dynamically reconfigured via various
[configuration entries](/docs/agent/config_entries.html). When a configuration
entry is missing, that stage will fall back on reasonable default behavior.

## Terminology

### Service Subsets

There are many ways you may wish to carve up a single datacenter's pool of
services beyond simply returning all healthy instances for load balancing.
Canary testing, A/B tests, blue/green deploys, and soft multi-tenancy
(prod/qa/staging sharing compute resources) all require some mechanism of
carving out portions of the Consul catalog smaller than the level of a single
service.

A service subset assigns a concrete name to a specific partitioning of the
overall set of available service instances within a datacenter used during
service discovery.

A service subset name is useful only when composed with an actual service name,
a specific datacenter, and namespace.

All services have an unnamed default subset that will return all healthy
instances unfiltered.

Subsets are defined in
[`service-resolver`](/docs/agent/config-entries/service-resolver.html)
configuration entries, but are referenced by their names throughout the other
configuration entry kinds.

## Stages

### Routing

-> **Note:** A service must define its protocol to be http-based to configure a router.

A [`service-router`](/docs/agent/config-entries/service-router.html) config
entry kind represents the topmost part of the discovery chain.

You can use this to intercept traffic using layer-7 criteria (such as path
prefixes or http headers) and change behavior such as sending traffic to a
different service or service subset.

These config entries may only reference `service-splitter` or
`service-resolver` entries.

### Splitting

-> **Note:** A service must define its protocol to be http-based to configure a splitter.

A [`service-splitter`](/docs/agent/config-entries/service-splitter.html) config
entry kind represents the next hop of the discovery chain after routing.

If no splitter config is defined for a service it is assumed 100% of traffic
flows to a service with the same name as the chain and discovery continues on
to the resolution stage.

A splitter config entry allows for a user to choose to split incoming requests
across different subsets of a single service (like during staged canary
rollouts), or perhaps across different services (like during a v2 rewrite or
other type of codebase migration).

These config entries may only reference `service-splitter` or
`service-resolver` entries.

If one splitter references another splitter the overall effects are flattened
into one effective splitter config entry which reflects the multiplicative
union. For instance:

	splitter[A]:           A_v1=50%, A_v2=50%
	splitter[B]:           A=50%,    B=50%
	---------------------
	splitter[effective_B]: A_v1=25%, A_v2=25%, B=50%

### Resolution

A [`service-resolver`](/docs/agent/config-entries/service-resolver.html) config
entry kind represents the next hop of the discovery chain after splitting.

A resolver config entry allows for a user to define which instances of a
service should satisfy discovery requests for the provided name.

Examples of things you can do with resolver config entries:

- Control where to send traffic if all instances of `api` in the current
datacenter are unhealthy.

- Configure service subsets based on `Service.Meta.version` values.

- Send all traffic for `web` that does not specify a service subset to the
`version1` subset.

- Send all traffic for `api` to `new-api`.

- Send all traffic for `api` in all datacenters to instances of `api` in `dc2`.

- Create a "virtual service" `api-dc2` that sends traffic to instances of `api`
in `dc2`. This can be referenced in upstreams or in other config entries.

If no resolver config is defined for a service it is assumed 100% of traffic
flows to the healthy instances of a service with the same name as the chain in
the current datacenter/namespace and discovery terminates.

This should feel similar in spirit to various uses of Prepared Queries, but is
not intended to be a drop-in replacement currently.

These config entries may only reference other `service-resolver` entries.
