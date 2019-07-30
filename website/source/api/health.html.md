---
layout: api
page_title: Health - HTTP API
sidebar_current: api-health
description: |-
  The /health endpoints query health-related information for services and checks
  in Consul.
---

# Health HTTP Endpoint

The `/health` endpoints query health-related information. They are provided
separately from the `/catalog` endpoints since users may prefer not to use the
optional health checking mechanisms. Additionally, some of the query results
from the health endpoints are filtered while the catalog endpoints provide the
raw entries.

## List Checks for Node

This endpoint returns the checks specific to the node provided on the path.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/health/node/:node`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required             |
| ---------------- | ----------------- | ------------- | ------------------------ |
| `YES`            | `all`             | `none`        | `node:read,service:read` |

### Parameters

- `node` `(string: <required>)` - Specifies the name or ID of the node to query.
  This is specified as part of the URL

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/health/node/my-node
```

### Sample Response

```json
[
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "foobar",
    "CheckID": "serfHealth",
    "Name": "Serf Health Status",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "",
    "ServiceName": "",
    "ServiceTags": []
  },
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis",
    "ServiceTags": ["primary"]
  }
]
```

### Filtering

The filter will be executed against each health check in the results list with
the following selectors and filter operations being supported:

| Selector      | Supported Operations                               |
| ------------- | -------------------------------------------------- |
| `CheckID`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Name`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Notes`       | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Output`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceID`   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceName` | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceTags` | In, Not In, Is Empty, Is Not Empty                 |
| `Status`      | Equal, Not Equal, In, Not In, Matches, Not Matches |

## List Checks for Service

This endpoint returns the checks associated with the service provided on the
path.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/health/checks/:service`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required             |
| ---------------- | ----------------- | ------------- | ------------------------ |
| `YES`            | `all`             | `none`        | `node:read,service:read` |

### Parameters

- `service` `(string: <required>)` - Specifies the service to list checks for.
  This is provided as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `near` `(string: "")` - Specifies a node name to sort the node list in
  ascending order based on the estimated round trip time from that node. Passing
  `?near=_agent` will use the agent's node for the sort. This is specified as
  part of the URL as a query parameter.

- `node-meta` `(string: "")` - Specifies a desired node metadata key/value pair
  of the form `key:value`. This parameter can be specified multiple times, and
  will filter the results to nodes with the specified key/value pairs. This is
  specified as part of the URL as a query parameter.

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/health/checks/my-service
```

### Sample Response

```json
[
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis",
	  "ServiceTags": ["primary"]
  }
]
```

### Filtering

The filter will be executed against each health check in the results list with
the following selectors and filter operations being supported:


| Selector      | Supported Operations                               |
| ------------- | -------------------------------------------------- |
| `CheckID`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Name`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Notes`       | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Output`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceID`   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceName` | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceTags` | In, Not In, Is Empty, Is Not Empty                 |
| `Status`      | Equal, Not Equal, In, Not In, Matches, Not Matches |

## List Nodes for Service

This endpoint returns the nodes providing the service indicated on the path.
Users can also build in support for dynamic load balancing and other features by
incorporating the use of health checks.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/health/service/:service`   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching        | ACL Required             |
| ---------------- | ----------------- | -------------------- | ------------------------ |
| `YES`            | `all`             | `background refresh` | `node:read,service:read` |

### Parameters

- `service` `(string: <required>)` - Specifies the service to list services for.
  This is provided as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `near` `(string: "")` - Specifies a node name to sort the node list in
  ascending order based on the estimated round trip time from that node. Passing
  `?near=_agent` will use the agent's node for the sort. This is specified as
  part of the URL as a query parameter.

- `tag` `(string: "")` - Specifies the tag to filter the list. This is
  specified as part of the URL as a query parameter. Can be used multiple times
  for additional filtering, returning only the results that include all of the tag
  values provided.

- `node-meta` `(string: "")` - Specifies a desired node metadata key/value pair
  of the form `key:value`. This parameter can be specified multiple times, and
  will filter the results to nodes with the specified key/value pairs. This is
  specified as part of the URL as a query parameter.

- `passing` `(bool: false)` - Specifies that the server should return only nodes
  with all checks in the `passing` state. This can be used to avoid additional
  filtering on the client side.

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/health/service/my-service
```

### Sample Response

```json
[
  {
    "Node": {
      "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
      "Node": "foobar",
      "Address": "10.1.10.12",
      "Datacenter": "dc1",
      "TaggedAddresses": {
        "lan": "10.1.10.12",
        "wan": "10.1.10.12"
      },
      "Meta": {
        "instance_type": "t2.medium"
      }
    },
    "Service": {
      "ID": "redis",
      "Service": "redis",
      "Tags": ["primary"],
      "Address": "10.1.10.12",
      "TaggedAddresses": {
        "lan": {
          "address": "10.1.10.12",
          "port": 8000
        },
        "wan": {
          "address": "198.18.1.2",
          "port": 80
        }
      },
      "Meta": {
        "redis_version": "4.0"
      },
      "Port": 8000,
      "Weights": {
        "Passing": 10,
        "Warning": 1
      }
    },
    "Checks": [
      {
        "Node": "foobar",
        "CheckID": "service:redis",
        "Name": "Service 'redis' check",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "redis",
        "ServiceName": "redis",
        "ServiceTags": ["primary"]
      },
      {
        "Node": "foobar",
        "CheckID": "serfHealth",
        "Name": "Serf Health Status",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": "",
        "ServiceTags": []
      }
    ]
  }
]
```

### Filtering

The filter will be executed against each entry in the top level results list with the
following selectors and filter operations being supported:

| Selector                                       | Supported Operations                               |
| ---------------------------------------------- | -------------------------------------------------- |
| `Checks`                                       | Is Empty, Is Not Empty                             |
| `Checks.CheckID`                               | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.Name`                                  | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.Node`                                  | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.Notes`                                 | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.Output`                                | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.ServiceID`                             | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.ServiceName`                           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Checks.ServiceTags`                           | In, Not In, Is Empty, Is Not Empty                 |
| `Checks.Status`                                | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.Address`                                 | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.Datacenter`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.ID`                                      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.Meta`                                    | Is Empty, Is Not Empty, In, Not In                 |
| `Node.Meta.<any>`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.Node`                                    | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node.TaggedAddresses`                         | Is Empty, Is Not Empty, In, Not In                 |
| `Node.TaggedAddresses.<any>`                   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Address`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Connect.Native`                       | Equal, Not Equal                                   |
| `Service.EnableTagOverride`                    | Equal, Not Equal                                   |
| `Service.ID`                                   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Kind`                                 | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Meta`                                 | Is Empty, Is Not Empty, In, Not In                 |
| `Service.Meta.<any>`                           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Port`                                 | Equal, Not Equal                                   |
| `Service.Proxy.DestinationServiceID`           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.DestinationServiceName`         | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.LocalServiceAddress`            | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.LocalServicePort`               | Equal, Not Equal                                   |
| `Service.Proxy.MeshGateway.Mode`               | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams`                      | Is Empty, Is Not Empty                             |
| `Service.Proxy.Upstreams.Datacenter`           | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams.DestinationName`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams.DestinationNamespace` | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams.DestinationType`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams.LocalBindAddress`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Proxy.Upstreams.LocalBindPort`        | Equal, Not Equal                                   |
| `Service.Proxy.Upstreams.MeshGateway.Mode`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.Service`                              | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.TaggedAddresses`                      | Is Empty, Is Not Empty, In, Not In                 |
| `Service.TaggedAddresses.<any>.Address`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Service.TaggedAddresses.<any>.Port`           | Equal, Not Equal                                   |
| `Service.Tags`                                 | In, Not In, Is Empty, Is Not Empty                 |
| `Service.Weights.Passing`                      | Equal, Not Equal                                   |
| `Service.Weights.Warning`                      | Equal, Not Equal                                   |

## List Nodes for Connect-capable Service

This endpoint returns the nodes providing a
[Connect-capable](/docs/connect/index.html) service in a given datacenter.
This will include both proxies and native integrations. A service may
register both Connect-capable and incapable services at the same time,
so this endpoint may be used to filter only the Connect-capable endpoints.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/health/connect/:service`   | `application/json`         |

Parameters and response format are the same as
[`/health/service/:service`](/api/health.html#list-nodes-for-service).

## List Checks in State

This endpoint returns the checks in the state provided on the path.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/health/state/:state`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required             |
| ---------------- | ----------------- | ------------- | ------------------------ |
| `YES`            | `all`             | `none`        | `node:read,service:read` |

### Parameters

- `state` `(string: <required>)` - Specifies the state to query. Supported states
  are `any`, `passing`, `warning`, or `critical`. The `any` state is a wildcard
  that can be used to return all checks.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `near` `(string: "")` - Specifies a node name to sort the node list in
  ascending order based on the estimated round trip time from that node. Passing
  `?near=_agent` will use the agent's node for the sort. This is specified as
  part of the URL as a query parameter.

- `node-meta` `(string: "")` - Specifies a desired node metadata key/value pair
  of the form `key:value`. This parameter can be specified multiple times, and
  will filter the results to nodes with the specified key/value pairs. This is
  specified as part of the URL as a query parameter.

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/health/state/passing
```

### Sample Response

```json
[
  {
    "Node": "foobar",
    "CheckID": "serfHealth",
    "Name": "Serf Health Status",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "",
    "ServiceName": "",
    "ServiceTags": []
  },
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis",
	  "ServiceTags": ["primary"]
  }
]
```

### Filtering

The filter will be executed against each health check in the results list with
the following selectors and filter operations being supported:


| Selector      | Supported Operations                               |
| ------------- | -------------------------------------------------- |
| `CheckID`     | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Name`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Node`        | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Notes`       | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `Output`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceID`   | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceName` | Equal, Not Equal, In, Not In, Matches, Not Matches |
| `ServiceTags` | In, Not In, Is Empty, Is Not Empty                 |
| `Status`      | Equal, Not Equal, In, Not In, Matches, Not Matches |
