---
layout: "docs"
page_title: "Configuration Entry Kind: Service Splitter"
sidebar_current: "docs-agent-cfg_entries-service_splitter"
description: |-
  The service-splitter config entry kind controls how to split incoming Connect requests across different subsets of a single service (like during staged canary rollouts), or perhaps across different services (like during a v2 rewrite or other type of codebase migration).
---

-> **1.6.0+:**  This config entry is available in Consul versions 1.6.0 and newer.

# Service Splitter

The `service-splitter` config entry kind controls how to split incoming Connect
requests across different subsets of a single service (like during staged
canary rollouts), or perhaps across different services (like during a v2
rewrite or other type of codebase migration).

If no splitter config is defined for a service it is assumed 100% of traffic
flows to a service with the same name and discovery continues on to the
resolution stage.

## Interaction with other Config Entries

- Service splitter config entries are a component of [L7 Traffic
  Management](/docs/connect/l7-traffic-management.html).

- Service splitter config entries are restricted to only services that define
  their protocol as http-based via a corresponding
  [`service-defaults`](/docs/agent/config-entries/service-defaults.html) config
  entry or globally via
  [`proxy-defaults`](/docs/agent/config-entries/proxy-defaults.html) .

- Any split destination that specifies a different `Service` field and omits
  the `ServiceSubset` field is eligible for further splitting should a splitter
  be configured for that other service, otherwise resolution proceeds according
  to any configured
  [`service-resolver`](/docs/agent/config-entries/service-resolver.html).

## Sample Config Entries

Split traffic between two subsets of the same service:

```hcl
kind = "service-splitter"
name = "web"
splits = [
  {
    weight         = 90
    service_subset = "v1"
  },
  {
    weight         = 10
    service_subset = "v2"
  },
]
```

Split traffic between two services:

```hcl
kind = "service-splitter"
name = "web"
splits = [
  {
    weight = 50
    # will default to service with same name as config entry ("web")
  },
  {
    weight  = 10
    service = "web-rewrite"
  },
]
```

## Available Fields

- `Kind` - Must be set to `service-splitter`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `Splits` `(array<ServiceSplit>)` - Defines how much traffic to send to which
  set of service instances during a traffic split.  The sum of weights across
  all splits must add up to 100.

  - `Weight` `(float32: 0)` - A value between 0 and 100 reflecting what portion
    of traffic should be directed to this split. The smallest representable
    weight is 1/10000 or .01%

  - `Service` `(string: "")` - The service to resolve instead of the default.

  - `ServiceSubset` `(string: "")` - A named subset of the given service to
    resolve instead of one defined as that service's `DefaultSubset`. If empty
    the default subset is used.

  - `Namespace` `(string: "")` - The namespace to resolve the service from
    instead of the current namespace. If empty the current namespace is
    assumed.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `service-splitter` config entry requires `service:read` on itself.

Creating, updating, or deleting a `service-splitter` config entry requires
`service:write` on itself and `service:read` on any other service referenced by
name in these fields:

- [`Splits[].Service`](#service)
