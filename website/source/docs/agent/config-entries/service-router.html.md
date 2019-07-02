---
layout: "docs"
page_title: "Configuration Entry Kind: Service Router (beta)"
sidebar_current: "docs-agent-cfg_entries-service_router"
description: |-
  TODO
---

# Service Router - `service-router` <sup>(beta)</sup>

Service routers control traffic routing and redirection at networking layer 7
(e.g. HTTP).

This is where the bulk of the L7 inspection and manipulation happens.

Service router config entries will be restricted to only services that define
their protocol as http-based via a corresponding
[`service-defaults`](/docs/agent/config-entries/service-defaults.html) config
entry or globally via
[`proxy-defaults`](/docs/agent/config-entries/proxy-defaults.html) .

## Sample Config Entries

Route HTTP requests with a path starting with `/admin` to a different service:

```hcl
kind = "service-router"
name = "web"
routes = [
  {
    match {
      http {
        path_prefix = "/admin"
      }
    }

    destination {
      service = "admin"
    }
  },
  # NOTE: a default catch-all will send unmatched traffic to "web"
]
```

Route HTTP requests with a special url parameter or header to a canary subset:

```hcl
kind = "service-router"
name = "web"
routes = [
  {
    match {
      http {
        header = [
          {
            name  = "x-debug"
            exact = "1"
          },
        ]
      }
    }
    destination {
      service        = "web"
      service_subset = "canary"
    }
  },
  {
    match {
      http {
        query_param = [
          {
            name  = "x-debug"
            value = "1"
          },
        ]
      }
    }
    destination {
      service        = "web"
      service_subset = "canary"
    }
  },
  # NOTE: a default catch-all will send unmatched traffic to "web"
]
```

Representation of the defaults when a router is not configured:

```hcl
kind = "service-router"
name = "web"
```

## Available Fields

- `Kind` - Must be set to `service-router`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `Routes` `(array<ServiceRoute>)` - TODO

  - `Match` `(ServiceRouteMatch: <optional>)` - TODO

    - `HTTP` `(ServiceRouteHTTPMatch: <optional>)` - TODO

      - `PathExact` `(string: "")` - TODO

      - `PathPrefix` `(string: "")` - TODO

      - `PathRegex` `(string: "")` - TODO

      - `Header` `(array<ServiceRouteHTTPMatchHeader>)` - TODO

        - `Name` `(string: <required>)` - TODO

        - `Present` `(bool: false)` - TODO

        - `Exact` `(string: "")` - TODO

        - `Prefix` `(string: "")` - TODO

        - `Suffix` `(string: "")` - TODO

        - `Regex` `(string: "")` - TODO

        - `Invert` `(bool: false)` - TODO

      - `QueryParam` `(array<ServiceRouteHTTPMatchQueryParam>)` - TODO

        - `Name  string
        - `Value string `json:",omitempty"`
        - `Regex bool   `json:",omitempty"`

  - `Destination` `(ServiceRouteDestination: <optional>)` - TODO

    - `Service` `(string: "")` - TODO

    - `ServiceSubset` `(string: "")` - TODO

    - `Namespace` `(string: "")` - TODO

    - `PrefixRewrite` `(string: "")` - TODO

    - `RequestTimeout` `(duration: 0s)` - TODO

    - `NumRetries` `(int: 0)` - TODO

    - `RetryOnConnectFailure` `(bool: false)` - TODO

    - `RetryOnStatusCodes` `(array<int>)` - TODO
