---
layout: api
page_title: Service - Agent - HTTP API
sidebar_current: api-agent-service
description: |-
  The /agent/service endpoints interact with services on the local agent in
  Consul.
---

# Service - Agent HTTP API

The `/agent/service` endpoints interact with checks on the local agent in
Consul. These should not be confused with services in the catalog.

## List Services

This endpoint returns all the services that are registered with
the local agent. These services were either provided through configuration files
or added dynamically using the HTTP API.

It is important to note that the services known by the agent may be different
from those reported by the catalog. This is usually due to changes being made
while there is no leader elected. The agent performs active
[anti-entropy](/docs/internals/anti-entropy.html), so in most situations
everything will be in sync within a few seconds.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/services`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `NO`             | `none`            | `none`        | `service:read` |

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/agent/services
```

### Sample Response

```json
{
  "redis": {
      "ID": "redis",
      "Service": "redis",
      "Tags": [],
      "Meta": {
          "redis_version": "4.0"
      },
      "Port": 8000,
      "Address": "",
      "EnableTagOverride": false
  }
}
```

## Get Service Configuration

This endpoint was added in Consul 1.3.0 and returns the full service definition
for a single service instance registered on the local agent. It is used by
[Connect proxies](/docs/connect/proxies.html) to discover the embedded proxy
configuration that was registered with the instance.

It is important to note that the services known by the agent may be different
from those reported by the catalog. This is usually due to changes being made
while there is no leader elected. The agent performs active
[anti-entropy](/docs/internals/anti-entropy.html), so in most situations
everything will be in sync within a few seconds.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/service/:service_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `YES`<sup>1</sup>| `none`            | `none`        | `service:read` |

<sup>1</sup> Supports [hash-based
blocking](/api/index.html#hash-based-blocking-queries) only.

### Parameters

- `service_id` `(string: <required>)` - Specifies the ID of the service to
  fetch. This is specified as part of the URL.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/agent/service/web-sidecar-proxy
```

### Sample Response

```json
{
    "Kind": "connect-proxy",
    "ID": "web-sidecar-proxy",
    "Service": "web-sidecar-proxy",
    "Tags": null,
    "Meta": null,
    "Port": 18080,
    "Address": "",
    "Weights": {
        "Passing": 1,
        "Warning": 1
    },
    "EnableTagOverride": false,
    "ContentHash": "4ecd29c7bc647ca8",
    "Proxy": {
        "DestinationServiceName": "web",
        "DestinationServiceID": "web",
        "LocalServiceAddress": "127.0.0.1",
        "LocalServicePort": 8080,
        "Config": {
            "foo": "bar"
        },
        "Upstreams": [
            {
                "DestinationType": "service",
                "DestinationName": "db",
                "LocalBindPort": 9191
            }
        ]
    }
}
```

The response has the same structure as the [service
definition](/docs/agent/services.html) with one extra field `ContentHash` which
contains the [hash-based blocking
query](/api/index.html#hash-based-blocking-queries) hash for the result. The
same hash is also present in `X-Consul-ContentHash`.

## Register Service

This endpoint adds a new service, with an optional health check, to the local
agent.

The agent is responsible for managing the status of its local services, and for
sending updates about its local services to the servers to keep the global
catalog in sync.

For "connect-proxy" kind services, the `service:write` ACL for the
`Proxy.DestinationServiceName` value is also required to register the service.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/agent/service/register`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `service:write` |

### Parameters

Note that this endpoint, unlike most also [supports `snake_case`](/docs/agent/services.html#service-definition-parameter-case)
service definition keys for compatibility with the config file format.

- `Name` `(string: <required>)` - Specifies the logical name of the service.
  Many service instances may share the same logical service name.

- `ID` `(string: "")` - Specifies a unique ID for this service. This must be
  unique per _agent_. This defaults to the `Name` parameter if not provided.

- `Tags` `(array<string>: nil)` - Specifies a list of tags to assign to the
  service. These tags can be used for later filtering and are exposed via the
  APIs.

- `Address` `(string: "")` - Specifies the address of the service. If not
  provided, the agent's address is used as the address for the service during
  DNS queries.

- `Meta` `(map<string|string>: nil)` - Specifies arbitrary KV metadata
  linked to the service instance.

- `Port` `(int: 0)` - Specifies the port of the service.

- `Kind` `(string: "")` - The kind of service. Defaults to "" which is a
  typical Consul service. This value may also be "connect-proxy" for
  services that are [Connect-capable](/docs/connect/index.html)
  proxies representing another service.

- `ProxyDestination` `(string: "")` - **Deprecated** From 1.2.0 to 1.2.3 this
  was used for "connect-proxy" `Kind` services however the equivalent field is
  now in `Proxy.DestinationServiceName`. Registrations using this field will
  continue to work until some later major version where this will be removed
  entirely. It's strongly recommended to switch to using the new field.

- `Proxy` `(Proxy: nil)` - From 1.2.3 on, specifies the configuration for a
  Connect proxy instance. This is only valid if `Kind == "connect-proxy"`. See
  the [Proxy documentation](/docs/connect/proxies.html) for full details.

- `Connect` `(Connect: nil)` - Specifies the 
  [configuration for Connect](/docs/connect/configuration.html). See the 
  [Connect Structure](#connect-structure) section below for supported fields.

- `Check` `(Check: nil)` - Specifies a check. Please see the
  [check documentation](/api/agent/check.html) for more information about the
  accepted fields. If you don't provide a name or id for the check then they
  will be generated. To provide a custom id and/or name set the `CheckID`
  and/or `Name` field.

- `Checks` `(array<Check>: nil`) - Specifies a list of checks. Please see the
  [check documentation](/api/agent/check.html) for more information about the
  accepted fields. If you don't provide a name or id for the check then they
  will be generated. To provide a custom id and/or name set the `CheckID`
  and/or `Name` field. The automatically generated `Name` and `CheckID` depend
  on the position of the check within the array, so even though the behavior is
  deterministic, it is recommended for all checks to either let consul set the
  `CheckID` by leaving the field empty/omitting it or to provide a unique value.

- `EnableTagOverride` `(bool: false)` - Specifies to disable the anti-entropy
  feature for this service's tags. If `EnableTagOverride` is set to `true` then
  external agents can update this service in the [catalog](/api/catalog.html)
  and modify the tags. Subsequent local sync operations by this agent will
  ignore the updated tags. For instance, if an external agent modified both the
  tags and the port for this service and `EnableTagOverride` was set to `true`
  then after the next sync cycle the service's port would revert to the original
  value but the tags would maintain the updated value. As a counter example, if
  an external agent modified both the tags and port for this service and
  `EnableTagOverride` was set to `false` then after the next sync cycle the
  service's port _and_ the tags would revert to the original value and all
  modifications would be lost.

    It is important to note that this applies only to the locally registered
    service. If you have multiple nodes all registering the same service their
    `EnableTagOverride` configuration and all other service configuration items
    are independent of one another. Updating the tags for the service registered
    on one node is independent of the same service (by name) registered on
    another node. If `EnableTagOverride` is not specified the default value is
    `false`. See [anti-entropy syncs](/docs/internals/anti-entropy.html) for
    more info.

#### Connect Structure

For the `Connect` field, the parameters are:

- `Native` `(bool: false)` - Specifies whether this service supports
  the [Connect](/docs/connect/index.html) protocol [natively](/docs/connect/native.html).
  If this is true, then Connect proxies, DNS queries, etc. will be able to
  service discover this service.
- `Proxy` `(Proxy: nil)` -
  [**Deprecated**](/docs/connect/proxies/managed-deprecated.html) Specifies that
  a managed Connect proxy should be started for this service instance, and
  optionally provides configuration for the proxy. The format is as documented
  in [Managed Proxy Deprecation](/docs/connect/proxies/managed-deprecated.html).
- `SidecarService` `(ServiceDefinition: nil)` - Specifies an optional nested
  service definition to register. For more information see
  [Sidecar Service Registration](/docs/connect/proxies/sidecar-service.html).

### Sample Payload

```json
{
  "ID": "redis1",
  "Name": "redis",
  "Tags": [
    "primary",
    "v1"
  ],
  "Address": "127.0.0.1",
  "Port": 8000,
  "Meta": {
    "redis_version": "4.0"
  },
  "EnableTagOverride": false,
  "Check": {
    "DeregisterCriticalServiceAfter": "90m",
    "Args": ["/usr/local/bin/check_redis.py"],
    "HTTP": "http://localhost:5000/health",
    "Interval": "10s",
    "TTL": "15s"
  }
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/agent/service/register
```

## Deregister Service

This endpoint removes a service from the local agent. If the service does not
exist, no action is taken.

The agent will take care of deregistering the service with the catalog. If there
is an associated check, that is also deregistered.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/agent/service/deregister/:service_id` | `application/json` |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `service:write` |

### Parameters

- `service_id` `(string: <required>)` - Specifies the ID of the service to
  deregister. This is specified as part of the URL.

### Sample Request

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/agent/service/deregister/my-service-id
```

## Enable Maintenance Mode

This endpoint places a given service into "maintenance mode". During maintenance
mode, the service will be marked as unavailable and will not be present in DNS
or API queries. This API call is idempotent. Maintenance mode is persistent and
will be automatically restored on agent restart.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/agent/service/maintenance/:service_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `service:write` |

### Parameters

- `service_id` `(string: <required>)` - Specifies the ID of the service to put
  in maintenance mode. This is specified as part of the URL.

- `enable` `(bool: <required>)` - Specifies whether to enable or disable
  maintenance mode. This is specified as part of the URL as a query string
  parameter.

- `reason` `(string: "")` - Specifies a text string explaining the reason for
  placing the node into maintenance mode. This is simply to aid human operators.
  If no reason is provided, a default value will be used instead. This is
  specified as part of the URL as a query string parameter, and, as such, must
  be URI-encoded.

### Sample Request

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/agent/service/maintenance/my-service-id?enable=true&reason=For+the+docs
```
