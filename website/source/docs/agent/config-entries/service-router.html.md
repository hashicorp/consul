---
layout: "docs"
page_title: "Configuration Entry Kind: Service Router"
sidebar_current: "docs-agent-cfg_entries-service_router"
description: |-
  The service-router config entry kind controls Connect traffic routing and manipulation at networking layer 7 (e.g. HTTP).
---

-> **1.6.0+:**  This config entry is available in Consul versions 1.6.0 and newer.

# Service Router

The `service-router` config entry kind controls Connect traffic routing and
manipulation at networking layer 7 (e.g. HTTP).

If a router is not explicitly configured or is configured with no routes then
the system behaves as if a router were configured sending all traffic to a
service of the same name.

## Interaction with other Config Entries

- Service router config entries are a component of [L7 Traffic
  Management](/docs/connect/l7-traffic-management.html).

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
  # NOTE: a default catch-all will send unmatched traffic to "web"
]
```

Re-route a gRPC method to another service. Since gRPC method calls [are
HTTP2](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md), we can use an HTTP path match rule to re-route traffic:

```hcl
kind = "service-router"
name = "billing"
routes = [
  {
    match {
      http {
        path_exact = "/mycompany.BillingService/GenerateInvoice"
      }
    }

    destination {
      service = "invoice-generator"
    }
  },
  # NOTE: a default catch-all will send unmatched traffic to "billing"
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

        - `HTTP` `(ServiceRouteHTTPMatch: <optional>)` - A set of
          [http-specific match criteria](#serviceroutehttpmatch).

    - `Destination` `(ServiceRouteDestination: <optional>)` - Controls [how to
      proxy](#serviceroutedestination) the matching request(s) to a
      service.

### `ServiceRouteHTTPMatch`

- `PathExact` `(string: "")` - Exact path to match on the HTTP request path.

    At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

- `PathPrefix` `(string: "")` - Path prefix to match on the HTTP request path.

      At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

- `PathRegex` `(string: "")` - Regular expression to match on the HTTP
  request path.

      The syntax when using the Envoy proxy is [documented here](https://en.cppreference.com/w/cpp/regex/ecmascript).

      At most only one of `PathExact`, `PathPrefix`, or `PathRegex` may be configured.

- `Header` `(array<ServiceRouteHTTPMatchHeader>)` - A set of criteria that can
  match on HTTP request headers. If more than one is configured all must match
  for the overall match to apply.

    - `Name` `(string: <required>)` - Name of the header to match.

    - `Present` `(bool: false)` - Match if the header with the given name is
      present with any value.

        At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or `Present`
        may be configured.

    - `Exact` `(string: "")` - Match if the header with the given name is this
      value.

        At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or `Present`
        may be configured.

    - `Prefix` `(string: "")` - Match if the header with the given name has
      this prefix.

        At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or `Present`
        may be configured.

    - `Suffix` `(string: "")` - Match if the header with the given name has
      this suffix.

        At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or `Present`
        may be configured.

    - `Regex` `(string: "")` - Match if the header with the given name matches
      this pattern.

        The syntax when using the Envoy proxy is [documented here](https://en.cppreference.com/w/cpp/regex/ecmascript).

        At most only one of `Exact`, `Prefix`, `Suffix`, `Regex`, or `Present`
        may be configured.

    - `Invert` `(bool: false)` - Inverts the logic of the match.

- `QueryParam` `(array<ServiceRouteHTTPMatchQueryParam>)` - A set of criteria
  that can match on HTTP query parameters. If more than one is configured all
  must match for the overall match to apply.

    - `Name` `(string: <required>)` - The name of the query parameter to match on.

    - `Present` `(bool: false)` - Match if the query parameter with the given
      name is present with any value.

        At most only one of `Exact`, `Regex`, or `Present` may be configured.

    - `Exact` `(string: "")` - Match if the query parameter with the given name
      is this value.

        At most only one of `Exact`, `Regex`, or `Present` may be configured.

    - `Regex` `(string: "")` - Match if the query parameter with the given name
      matches this pattern.

        The syntax when using the Envoy proxy is [documented here](https://en.cppreference.com/w/cpp/regex/ecmascript).

        At most only one of `Exact`, `Regex`, or `Present` may be configured.

- `Methods` `(array<string>)` - A list of HTTP methods for which this match
  applies. If unspecified all http methods are matched.

### `ServiceRouteDestination`

- `Service` `(string: "")` - The service to resolve instead of the default
  service. If empty then the default service name is used.

- `ServiceSubset` `(string: "")` - A named subset of the given service to
  resolve instead of the one defined as that service's `DefaultSubset`. If empty,
  the default subset is used.

- `Namespace` `(string: "")` - The namespace to resolve the service from
  instead of the current namespace. If empty the current namespace is assumed.

- `PrefixRewrite` `(string: "")` - Defines how to rewrite the HTTP request path
  before proxying it to its final destination.

      This requires that either `Match.HTTP.PathPrefix` or
      `Match.HTTP.PathExact` be configured on this route.

- `RequestTimeout` `(duration: 0s)` - The total amount of time permitted for
  the entire downstream request (and retries) to be processed.

- `NumRetries` `(int: 0)` - The number of times to retry the request when a
  retryable result occurs.

- `RetryOnConnectFailure` `(bool: false)` - Allows for connection failure
  errors to trigger a retry.

- `RetryOnStatusCodes` `(array<int>)` - A flat list of http response status
  codes that are eligible for retry.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `service-router` config entry requires `service:read` on itself.

Creating, updating, or deleting a `service-router` config entry requires
`service:write` on itself and `service:read` on any other service referenced by
name in these fields:

- [`Routes[].Destination.Service`](#service)
