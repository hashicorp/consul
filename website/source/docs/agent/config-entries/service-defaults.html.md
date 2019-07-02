---
layout: "docs"
page_title: "Configuration Entry Kind: Service Defaults"
sidebar_current: "docs-agent-cfg_entries-service_defaults"
description: |-
  Service defaults control default global values for a service, such as its protocol.
---

# Service Defaults - `service-defaults`

Service defaults control default global values for a service, such as its
protocol.

## Sample Config Entries

Set the default protocol for a service to HTTP:

```hcl
Kind = "service-defaults"
Name = "web"
Protocol = "http"
```

## Available Fields

- `Kind` - Must be set to `service-defaults`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `Protocol` `(string: "tcp")` - Sets the protocol of the service. This is used
  by Connect proxies for things like observability features and to unlock usage
  of the [`service-splitter`
  <sup>(beta)</sup>](/docs/agent/config-entries/service-splitter.html) and
  [`service-router`
  <sup>(beta)</sup>](/docs/agent/config-entries/service-router.html) config
  entries for a service.
