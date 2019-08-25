---
layout: "docs"
page_title: "Configuration Entry Kind: Service Resolver"
sidebar_current: "docs-agent-cfg_entries-service_resolver"
description: |-
  The `service-resolver` config entry kind controls which service instances should satisfy Connect upstream discovery requests for a given service name.
---

-> **1.6.0+:**  This config entry is available in Consul versions 1.6.0 and newer.

# Service Resolver

The `service-resolver` config entry kind controls which service instances
should satisfy Connect upstream discovery requests for a given service name.

If no resolver config is defined the chain assumes 100% of traffic goes to the
healthy instances of the default service in the current datacenter+namespace
and discovery terminates.

## Interaction with other Config Entries

- Service resolver config entries are a component of [L7 Traffic
  Management](/docs/connect/l7-traffic-management.html).

## Sample Config Entries

Create service subsets based on a version metadata and override the defaults:

```hcl
kind           = "service-resolver"
name           = "web"
default_subset = "v1"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
```

Expose a set of services in another datacenter as a virtual service:

```hcl
kind = "service-resolver"
name = "web-dc2"
redirect {
  service    = "web"
  datacenter = "dc2"
}
```

Enable failover for all subsets:

```hcl
kind            = "service-resolver"
name            = "web"
connect_timeout = "15s"
failover = {
  "*" = {
    datacenters = ["dc3", "dc4"]
  }
}
```

Representation of the defaults when a resolver is not configured:

```hcl
kind = "service-resolver"
name = "web"
```

## Available Fields

- `Kind` - Must be set to `service-resolver`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `ConnectTimeout` `(duration: 0s)` - The timeout for establishing new network
  connections to this service.

- `DefaultSubset` `(string: "")` - The subset to use when no explicit subset is
  requested. If empty the unnamed subset is used.

- `Subsets` `(map[string]ServiceResolverSubset)` - A map of subset name to
  subset definition for all usable named subsets of this service. The map key
  is the name of the subset and all names must be valid DNS subdomain elements.

    This may be empty, in which case only the unnamed default subset will be
    usable.

  - `Filter` `(string: "")` - The 
    [filter expression](/api/features/filtering.html) to be used for selecting
    instances of the requested service. If empty all healthy instances are
    returned.

  - `OnlyPassing` `(bool: false)` - Specifies the behavior of the resolver's
    health check interpretation. If this is set to false, instances with checks
    in the passing as well as the warning states will be considered healthy. If
    this is set to true, only instances with checks in the passing state will
    be considered healthy.

- `Redirect` `(ServiceResolverRedirect: <optional>)` - When configured, all
  attempts to resolve the service this resolver defines will be substituted for
  the supplied redirect EXCEPT when the redirect has already been applied.

    When substituting the supplied redirect into the all other fields besides
    `Kind`, `Name`, and `Redirect` will be ignored.

  - `Service` `(string: "")` - A service to resolve instead of the current
    service.

  - `ServiceSubset` `(string: "")` - A named subset of the given service to
    resolve instead of one defined as that service's DefaultSubset If empty the
    default subset is used.

        If this is specified at least one of Service, Datacenter, or Namespace
        should be configured.

  - `Namespace` `(string: "")` - The namespace to resolve the service from
    instead of the current one.

  - `Datacenter` `(string: "")` - The datacenter to resolve the service from
    instead of the current one.

- `Failover` `(map[string]ServiceResolverFailover`) - Controls when and how to
  reroute traffic to an alternate pool of service instances.

    The map is keyed by the service subset it applies to and the special
    string `"*"` is a wildcard that applies to any subset not otherwise
    specified here.

    `Service`, `ServiceSubset`, `Namespace`, and `Datacenters` cannot all be
    empty at once.

  - `Service` `(string: "")` - The service to resolve instead of the default as
    the failover group of instances during failover.

  - `ServiceSubset` `(string: "")` - The named subset of the requested service
    to resolve as the failover group of instances. If empty the default subset
    for the requested service is used.

  - `Namespace` `(string: "")` - The namespace to resolve the requested service
    from to form the failover group of instances. If empty the current
    namespace is used.

  - `Datacenters` `(array<string>)` - A fixed list of datacenters to try during
    failover.

## Service Subsets

A service subset assigns a concrete name to a specific subset of discoverable
service instances within a datacenter, such as `"version2"` or `"canary"`.

A service subset name is useful only when composed with an actual service name,
a specific datacenter, and namespace.

All services have an unnamed default subset that will return all healthy
instances unfiltered.

Subsets are defined in `service-resolver` configuration entries, but are
referenced by their names throughout the other configuration entry kinds.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `service-resolver` config entry requires `service:read` on itself.

Creating, updating, or deleting a `service-resolver` config entry requires
`service:write` on itself and `service:read` on any other service referenced by
name in these fields:

- [`Redirect.Service`](#service)

- [`Failover[].Service`](#service-1)

