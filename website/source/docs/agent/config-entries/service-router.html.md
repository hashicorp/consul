---
layout: "docs"
page_title: "Configuration Entry Kind: Service Router (beta)"
sidebar_current: "docs-agent-cfg_entries-service_router"
description: |-
  Service routers control Connect traffic routing and manipulation at networking layer 7 (e.g. HTTP).
---

# Service Router - `service-router` <sup>(beta)</sup>

Service routers control Connect traffic routing and manipulation at networking
layer 7 (e.g. HTTP).

If a router is not explicitly configured or is configured with no routes then
the system behaves as if a router were configured sending all traffic to a
service of the same name.

## Interaction with other Config Entries

- Service router config entries are restricted to only services that define
  their protocol as http-based via a corresponding
  [`service-defaults`](/docs/agent/config-entries/service-defaults.html) config
  entry or globally via
  [`proxy-defaults`](/docs/agent/config-entries/proxy-defaults.html) .

- Any route destination that omits the `ServiceSubset` field is eligible for
  splitting via a
  [`service-splitter`](/docs/agent/config-entries/service-splitter.html) should
  one be configured for that service, otherwise resolution proceeds according
  to any configured
  [`service-resolver`](/docs/agent/config-entries/service-resolver.html).

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

## Available Fields

- `Kind` - Must be set to `service-router`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `Routes` `(array<ServiceRoute>)` - The list of routes to consider when
  processing L7 requests. The first route to match in the list is terminal and
  stops further evaluation. Traffic that fails to match any of the provided
  routes will be routed to the default service.

  - `Match` `(ServiceRouteMatch: <optional>)` - A set of criteria that can
    match incoming L7 requests. If empty or omitted it acts as a catch-all.

    - `HTTP` `(ServiceRouteHTTPMatch: <optional>)` - A set of http-specific match criteria.

      - `PathExact` `(string: "")` - Exact path to match on the HTTP request path.

            At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

      - `PathPrefix` `(string: "")` - Path prefix to match on the HTTP request path.

            At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

      - `PathRegex` `(string: "")` - Regular expression to match on the HTTP
        request path.
        
            The syntax when using the Envoy proxy is [documented here](https://en.cppreference.com/w/cpp/regex/ecmascript).
      
            At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

      - `Header` `(array<ServiceRouteHTTPMatchHeader>)` - A set of criteria
        that can match on HTTP request headers. If more than one is configured
        all must match for the overall match to apply.

        - `Name` `(string: <required>)` - Name of the header to match on.

        - `Present` `(bool: false)` - Match if the header with the given name
          is present with any value.

            At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or
            `Present` may be configured.

        - `Exact` `(string: "")` - Match if the header with the given name is
          this value.

            At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or
            `Present` may be configured.

        - `Prefix` `(string: "")` - Match if the header with the given name has
          this prefix.

            At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or
            `Present` may be configured.

        - `Suffix` `(string: "")` - Match if the header with the given name has
          this suffix.

            At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or
            `Present` may be configured.

        - `Regex` `(string: "")` - Match if the header with the given name
          matches this pattern.

            The syntax when using the Envoy proxy is [documented here](https://en.cppreference.com/w/cpp/regex/ecmascript).

            At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or
            `Present` may be configured.

        - `Invert` `(bool: false)` - Inverts the logic of the match.

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
