---
layout: "docs"
page_title: "Configuration Entry Kind: Service Resolver (beta)"
sidebar_current: "docs-agent-cfg_entries-service_resolver"
description: |-
  Service resolvers control which service instances should satisfy Connect upstream discovery requests for a given service name.
---

# Service Resolver - `service-resolver` <sup>(beta)</sup>

Service resolvers control which service instances should satisfy Connect
upstream discovery requests for a given service name.

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

- `ConnectTimeout` `(duration: 0s)` - TODO

- `DefaultSubset` `(string: "")` - TODO

- `Subsets` `(map[string]ServiceResolverSubset)` - TODO

  - `Filter` `(string: "")` - TODO

  - `OnlyPassing` `(bool: false)` - TODO

- `Redirect` `(ServiceResolverRedirect: <optional>)` - TODO

  - `Service` `(string: "")` - TODO

  - `ServiceSubset` `(string: "")` - TODO

  - `Namespace` `(string: "")` - TODO

  - `Datacenter` `(string: "")` - TODO

- `Failover` `(map[string]ServiceResolverFailover`) - TODO

  - `Service` `(string: "")` - TODO

  - `ServiceSubset` `(string: "")` - TODO

  - `Namespace` `(string: "")` - TODO

  - `Datacenters` `(array<string>)` - TODO

  - `OverprovisioningFactor` `(int: 0)` - TODO

