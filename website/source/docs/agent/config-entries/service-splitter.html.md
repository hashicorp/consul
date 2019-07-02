---
layout: "docs"
page_title: "Configuration Entry Kind: Service Splitter (beta)"
sidebar_current: "docs-agent-cfg_entries-service_splitter"
description: |-
  Service splitters control how to split incoming requests across different subsets of a single service, or perhaps across different services.
---

# Service Splitter - `service-splitter` <sup>(beta)</sup>

Service splitters control how to split incoming Connect requests across
different subsets of a single service (like during staged canary rollouts), or
perhaps across different services (like during a v2 rewrite or other type of
codebase migration).

## Interaction with other Config Entries

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

- `Splits` `(array<ServiceSplit>)` - TODO

  - `Weight` `(float32: 0)` - TODO

  - `Service` `(string: "")` - TODO

  - `ServiceSubset` `(string: "")` - TODO

  - `Namespace` `(string: "")` - TODO
