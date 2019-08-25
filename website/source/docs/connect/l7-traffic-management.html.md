---
layout: "docs"
page_title: "Connect - L7 Traffic Management"
sidebar_current: "docs-connect-l7_traffic_management"
description: |-
  Layer 7 traffic management allows operators to divide L7 traffic between different subsets of service instances when using Connect.
---

-> **1.6.0+:**  This feature is available in Consul versions 1.6.0 and newer.

# L7 Traffic Management

Layer 7 traffic management allows operators to divide L7 traffic between
different
[subsets](/docs/agent/config-entries/service-resolver.html#service-subsets) of
service instances when using Connect.

There are many ways you may wish to carve up a single datacenter's pool of
services beyond simply returning all healthy instances for load balancing.
Canary testing, A/B tests, blue/green deploys, and soft multi-tenancy
(prod/qa/staging sharing compute resources) all require some mechanism of
carving out portions of the Consul catalog smaller than the level of a single
service and configuring when that subset should receive traffic.

-> **Note:** This feature is not compatible with the 
[built-in proxy](/docs/connect/proxies/built-in.html),
[native proxies](/docs/connect/native.html),
and some [Envoy proxy escape hatches](/docs/connect/proxies/envoy.html#escape-hatch-overrides).

## Stages

Connect proxy upstreams are discovered using a series of stages: routing,
splitting, and resolution. These stages represent different ways of managing L7
traffic.

![diagram showing l7 traffic discovery stages: routing to splitting to resolution](/assets/images/l7-traffic-stages.svg)

Each stage of this discovery process can be dynamically reconfigured via various
[configuration entries](/docs/agent/config_entries.html). When a configuration
entry is missing, that stage will fall back on reasonable default behavior.

### Routing

A [`service-router`](/docs/agent/config-entries/service-router.html) config
entry kind is the first configurable stage.

A router config entry allows for a user to intercept traffic using L7 criteria
such as path prefixes or http headers, and change behavior such as by sending
traffic to a different service or service subset.

These config entries may only reference `service-splitter` or
`service-resolver` entries.

[Examples](/docs/agent/config-entries/service-router.html#sample-config-entries)
can be found in the `service-router` documentation.

### Splitting

A [`service-splitter`](/docs/agent/config-entries/service-splitter.html) config
entry kind is the next stage after routing.

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

[Examples](/docs/agent/config-entries/service-splitter.html#sample-config-entries)
can be found in the `service-splitter` documentation.

### Resolution

A [`service-resolver`](/docs/agent/config-entries/service-resolver.html) config
entry kind is the last stage.

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
flows to the healthy instances of a service with the same name in the current
datacenter/namespace and discovery terminates.

This should feel similar in spirit to various uses of Prepared Queries, but is
not intended to be a drop-in replacement currently.

These config entries may only reference other `service-resolver` entries.

[Examples](/docs/agent/config-entries/service-resolver.html#sample-config-entries)
can be found in the `service-resolver` documentation.

-> **Note:** `service-resolver` config entries kinds function at L4 (unlike
`service-router` and `service-splitter` kinds). These can be created for
services of any protocol such as `tcp`.
