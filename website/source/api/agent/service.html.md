---
layout: api
page_title: Service - Agent - HTTP API
sidebar_current: api-agent-service
description: |-
  The /agent/service endpoints interact with services on the local agent in
  Consul.
---

# Service - Agent HTTP API

The `/agent/service` endpoints interact with services on the local agent in
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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `NO`             | `none`            | `none`        | `service:read` |

### Parameters

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

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
      "TaggedAddresses": {
        "lan": {
          "address": "127.0.0.1",
          "port": 8000
        },
        "wan": {
          "address": "198.18.0.53",
          "port": 80
        }
      },
      "Meta": {
          "redis_version": "4.0"
      },
      "Port": 8000,
      "Address": "",
      "EnableTagOverride": false,
      "Weights": {
          "Passing": 10,
          "Warning": 1
      }
  }
}
```

### Filtering

The filter is executed against each value in the service mapping with the
following selectors and filter operations being supported:

| Selector                               | Supported Operations                               |
| -------------------------------------- | -------------------------------------------------  |
| `Address`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Connect.Native`                       | Equal, Not Equal                                   |
| `EnableTagOverride`                    | Equal, Not Equal                                   |
| `ID`                                   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Kind`                                 | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Meta`                                 | Is Empty, Is Not Empty, In, Not In                 |
| `Meta.<any>`                           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Port`                                 | Equal, Not Equal                                   |
| `Proxy.DestinationServiceID`           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.DestinationServiceName`         | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.LocalServiceAddress`            | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.LocalServicePort`               | Equal, Not Equal                                   |
| `Proxy.MeshGateway.Mode`               | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams`                      | Is Empty, Is Not Empty                             |
| `Proxy.Upstreams.Datacenter`           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams.DestinationName`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams.DestinationNamespace` | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams.DestinationType`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams.LocalBindAddress`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Proxy.Upstreams.LocalBindPort`        | Equal, Not Equal                                   |
| `Proxy.Upstreams.MeshGateway.Mode`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `TaggedAddresses`                      | Is Empty, Is Not Empty, In, Not In                 |
| `TaggedAddresses.<any>.Address`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `TaggedAddresses.<any>.Port`           | Equal, Not Equal                                   |
| `Tags`                                 | In, Not In, Is Empty, Is Not Empty                 |
| `Weights.Passing`                      | Equal, Not Equal                                   |
| `Weights.Warning`                      | Equal, Not Equal                                   |


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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `YES`<sup>1</sup>| `none`            | `none`        | `service:read` |

<sup>1</sup> Supports [hash-based
blocking](/api/features/blocking.html#hash-based-blocking-queries) only.

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
    "TaggedAddresses": {
        "lan": {
          "address": "127.0.0.1",
          "port": 8000
        },
        "wan": {
          "address": "198.18.0.53",
          "port": 80
        }
      },
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
query](/api/features/blocking.html#hash-based-blocking-queries) hash for the result. The
same hash is also present in `X-Consul-ContentHash`.

## Get local service health

Retrieve an aggregated state of service(s) on the local agent by name.

This endpoints support JSON format and text/plain formats, JSON being the
default. In order to get the text format, you can append `?format=text` to
the URL or use Mime Content negotiation by specifying a HTTP Header
`Accept` starting with `text/plain`.

| Method | Path                                                      | Produces           |
| ------ | --------------------------------------------------------- | ------------------ |
| `GET`  | `/v1/agent/health/service/name/:service_name`             | `application/json` |
| `GET`  | `/v1/agent/health/service/name/:service_name?format=text` | `text/plain`       |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `NO`             | `none`            | `none`        | `service:read` |

Those endpoints return the aggregated values of all healthchecks for the
service instance(s) and will return the corresponding HTTP codes:

| Result | Meaning                                                         |
| ------ | ----------------------------------------------------------------|
| `200`  | All healthchecks of every matching service instance are passing |
| `400`  | Bad parameter (missing service name of id)                      |
| `404`  | No such service id or name                                      |
| `429`  | Some healthchecks are passing, at least one is warning          |
| `503`  | At least one of the healthchecks is critical                    |

Those endpoints might be usefull for the following use-cases:

* a load-balancer wants to check IP connectivity with an agent and retrieve
  the aggregated status of given service
* create aliases for a given service (thus, the healthcheck of alias uses
  http://localhost:8500/v1/agent/service/id/aliased_service_id healthcheck)


##### Note
If you know the ID of service you want to target, it is recommended to use
[`/v1/agent/health/service/id/:service_id`](/api/agent/service.html#get-local-service-health-by-id)
so you have the result for the service only. When requesting
`/v1/agent/health/service/name/:service_name`, the caller will receive the
worst state of all services having the given name.

### Sample Requests

Given 2 services with name `web`, with web2 critical and web1 passing:

#### List worst statuses of all instances of web-demo services (HTTP 503)

##### By Name, Text

```shell
curl http://localhost:8500/v1/agent/health/service/name/web?format=text
critical
```

##### By Name, JSON

In JSON, the detail of passing/warning/critical services is present in output,
in a array.

```shell
curl localhost:8500/v1/agent/health/service/name/web
```

```json
{
    "critical": [
        {
            "ID": "web2",
            "Service": "web",
            "Tags": [
                "rails"
            ],
            "Address": "",
            "TaggedAddresses": {
              "lan": {
                "address": "127.0.0.1",
                "port": 8000
              },
              "wan": {
                "address": "198.18.0.53",
                "port": 80
              }
            },
            "Meta": null,
            "Port": 80,
            "EnableTagOverride": false,
            "Connect": {
                "Native": false,
                "Proxy": null
            },
            "CreateIndex": 0,
            "ModifyIndex": 0
        }
    ],
    "passing": [
        {
            "ID": "web1",
            "Service": "web",
            "Tags": [
                "rails"
            ],
            "Address": "",
            "TaggedAddresses": {
              "lan": {
                "address": "127.0.0.1",
                "port": 8000
              },
              "wan": {
                "address": "198.18.0.53",
                "port": 80
              }
            },
            "Meta": null,
            "Port": 80,
            "EnableTagOverride": false,
            "Connect": {
                "Native": false,
                "Proxy": null
            },
            "CreateIndex": 0,
            "ModifyIndex": 0
        }
    ]
}
```

#### List status of web2 (HTTP 503)

##### Failure By ID, Text

```shell
curl http://localhost:8500/v1/agent/health/service/id/web2?format=text
critical
```

##### Failure By ID, JSON

In JSON, the output per ID is not an array, but only contains the value
of service.

```shell
curl localhost:8500/v1/agent/health/service/id/web2
```

```json
{
    "critical": {
        "ID": "web2",
        "Service": "web",
        "Tags": [
            "rails"
        ],
        "Address": "",
        "TaggedAddresses": {
          "lan": {
            "address": "127.0.0.1",
            "port": 8000
          },
          "wan": {
            "address": "198.18.0.53",
            "port": 80
          }
        },
        "Meta": null,
        "Port": 80,
        "EnableTagOverride": false,
        "Connect": {
            "Native": false,
            "Proxy": null
        },
        "CreateIndex": 0,
        "ModifyIndex": 0
    }
}
```

#### List status of web2 (HTTP 200)

##### Success By ID, Text

```shell
curl localhost:8500/v1/agent/health/service/id/web1?format=text
passing
```

#### Success By ID, JSON

```shell
curl localhost:8500/v1/agent/health/service/id/web1
```

```json
{
    "passing": {
        "ID": "web1",
        "Service": "web",
        "Tags": [
            "rails"
        ],
        "Address": "",
        "TaggedAddresses": {
          "lan": {
            "address": "127.0.0.1",
            "port": 8000
          },
          "wan": {
            "address": "198.18.0.53",
            "port": 80
          }
        },
        "Meta": null,
        "Port": 80,
        "EnableTagOverride": false,
        "Connect": {
            "Native": false,
            "Proxy": null
        },
        "CreateIndex": 0,
        "ModifyIndex": 0
    }
}
```

## Get local service health by its ID

Retrive an aggregated state of service(s) on the local agent by ID.

See:

| Method | Path                                                   | Produces           |
| ------ | ------------------------------------------------------ | ------------------ |
| `GET`  | `/v1/agent/health/service/id/:service_id`             | `application/json` |
| `GET`  | `/v1/agent/health/service/id/:service_id?format=text` | `text/plain`       |

Parameters and response format are the same as
[`/v1/agent/health/service/name/:service_name`](/api/agent/service.html#get-local-service-health).

## Register Service

This endpoint adds a new service, with optional health checks, to the local
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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `service:write` |

### Query string parameters

- `replace-existing-checks` - Missing healthchecks from the request will be deleted from the agent. Using this parameter allows to idempotently register a service and its checks whithout having to manually deregister checks.

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

- `TaggedAddresses` `(map<string|object>: nil)` - Specifies a map of explicit LAN
  and WAN addresses for the service instance. Both the address and port can be
  specified within the map values.

- `Meta` `(map<string|string>: nil)` - Specifies arbitrary KV metadata
  linked to the service instance.

- `Port` `(int: 0)` - Specifies the port of the service.

- `Kind` `(string: "")` - The kind of service. Defaults to "" which is a
  typical Consul service. This value may also be "connect-proxy" for
  services that are [Connect-capable](/docs/connect/index.html)
  proxies representing another service or "mesh-gateway" for instances of
  a [mesh gateway](/docs/connect/mesh_gateway.html)

- `Proxy` `(Proxy: nil)` - From 1.2.3 on, specifies the configuration for a
  Connect proxy instance. This is only valid if `Kind == "connect-proxy"` or
  `Kind == "mesh-gateway"`. See the [Proxy documentation](/docs/connect/registration/service-registration.html)
  for full details.

- `Connect` `(Connect: nil)` - Specifies the
  [configuration for Connect](/docs/connect/configuration.html). See the
  [Connect Structure](#connect-structure) section below for supported fields.

- `Check` `(Check: nil)` - Specifies a check. Please see the
  [check documentation](/api/agent/check.html) for more information about the
  accepted fields. If you don't provide a name or id for the check then they
  will be generated. To provide a custom id and/or name set the `CheckID`
  and/or `Name` field.

- `Checks` `(array<Check>: nil)` - Specifies a list of checks. Please see the
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

- `Weights` `(Weights: nil)` - Specifies weights for the service. Please see the
  [service documentation](/docs/agent/services.html) for more information about
  weights. If this field is not provided weights will default to
  `{"Passing": 1, "Warning": 1}`.

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
  [Sidecar Service Registration](/docs/connect/registration/sidecar-service.html).

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
  },
  "Weights": {
    "Passing": 10,
    "Warning": 1
  }
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/agent/service/register?replace-existing-checks=1
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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

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
