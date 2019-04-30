---
layout: api
page_title: Catalog - HTTP API
sidebar_current: api-catalog
description: |-
  The /catalog endpoints register and deregister nodes, services, and checks in
  Consul.
---

# Catalog HTTP API

The `/catalog` endpoints register and deregister nodes, services, and checks in
Consul. The catalog should not be confused with the agent, since some of the
API methods look similar.

## Register Entity

This endpoint is a low-level mechanism for registering or updating
entries in the catalog. It is usually preferable to instead use the
[agent endpoints](/api/agent.html) for registration as they are simpler and
perform [anti-entropy](/docs/internals/anti-entropy.html).

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/catalog/register`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required              |
| ---------------- | ----------------- | ------------- | ------------------------- |
| `NO`             | `none`            | `none`        |`node:write,service:write` |

### Parameters

- `ID` `(string: "")` - An optional UUID to assign to the node. This must be a 36-character UUID-formatted string.

- `Node` `(string: <required>)` - Specifies the node ID to register.

- `Address` `(string: <required>)` - Specifies the address to register.

- `Datacenter` `(string: "")` - Specifies the datacenter, which defaults to the
  agent's datacenter if not provided.

- `TaggedAddresses` `(map<string|string>: nil)` - Specifies the tagged
  addresses.

- `NodeMeta` `(map<string|string>: nil)` - Specifies arbitrary KV metadata
  pairs for filtering purposes.

- `Service` `(Service: nil)` - Specifies to register a service. If `ID` is not
  provided, it will be defaulted to the value of the `Service.Service` property.
  Only one service with a given `ID` may be present per node. The service
  `Tags`, `Address`, `Meta`, and `Port` fields are all optional. For more
  information about these fields and the implications of setting them,
  see the [Service - Agent API](https://www.consul.io/api/agent/service.html) page
  as registering services differs between using this or the Services Agent endpoint.

- `Check` `(Check: nil)` - Specifies to register a check. The register API
  manipulates the health check entry in the Catalog, but it does not setup the
  script, TTL, or HTTP check to monitor the node's health. To truly enable a new
  health check, the check must either be provided in agent configuration or set
  via the [agent endpoint](agent.html).

    The `CheckID` can be omitted and will default to the value of `Name`. As
    with `Service.ID`, the `CheckID` must be unique on this node. `Notes` is an
    opaque field that is meant to hold human-readable text. If a `ServiceID` is
    provided that matches the `ID` of a service on that node, the check is
    treated as a service level health check, instead of a node level health
    check. The `Status` must be one of `passing`, `warning`, or `critical`.

    The `Definition` field can be provided with details for a TCP or HTTP health
    check. For more information, see the [Health Checks](/docs/agent/checks.html) page.

    Multiple checks can be provided by replacing `Check` with `Checks` and
    sending an array of `Check` objects.

- `SkipNodeUpdate` `(bool: false)` - Specifies whether to skip updating the
  node's information in the registration. This is useful in the case where
  only a health check or service entry on a node needs to be updated or when
  a register request is intended to  update a service entry or health check.
  In both use cases, node information will not be overwritten, if the node is
  already registered. Note, if the paramater is enabled for a node that doesn't
  exist, it will still be created.

It is important to note that `Check` does not have to be provided with `Service`
and vice versa. A catalog entry can have either, neither, or both.

### Sample Payload

```json
{
  "Datacenter": "dc1",
  "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
  "Node": "foobar",
  "Address": "192.168.10.10",
  "TaggedAddresses": {
    "lan": "192.168.10.10",
    "wan": "10.0.10.10"
  },
  "NodeMeta": {
    "somekey": "somevalue"
  },
  "Service": {
    "ID": "redis1",
    "Service": "redis",
    "Tags": [
      "primary",
      "v1"
    ],
    "Address": "127.0.0.1",
    "Meta": {
        "redis_version": "4.0"
    },
    "Port": 8000
  },
  "Check": {
    "Node": "foobar",
    "CheckID": "service:redis1",
    "Name": "Redis health check",
    "Notes": "Script based health check",
    "Status": "passing",
    "ServiceID": "redis1",
    "Definition": {
      "TCP": "localhost:8888",
      "Interval": "5s",
      "Timeout": "1s",
      "DeregisterCriticalServiceAfter": "30s"
    }
  },
  "SkipNodeUpdate": false
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/catalog/register
```

## Deregister Entity

This endpoint is a low-level mechanism for directly removing
entries from the Catalog. It is usually preferable to instead use the
[agent endpoints](/api/agent.html) for deregistration as they are simpler and
perform [anti-entropy](/docs/internals/anti-entropy.html).

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/catalog/deregister`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required               |
| ---------------- | ----------------- | ------------- | -------------------------- |
| `NO`             | `none`            | `none`        | `node:write,service:write` |

### Parameters

The behavior of the endpoint depends on what keys are provided.

- `Node` `(string: <required>)` - Specifies the ID of the node. If no other
  values are provided, this node, all its services, and all its checks are
  removed.

- `Datacenter` `(string: "")` - Specifies the datacenter, which defaults to the
  agent's datacenter if not provided.

- `CheckID` `(string: "")` - Specifies the ID of the check to remove.

- `ServiceID` `(string: "")` - Specifies the ID of the service to remove. The
  service and all associated checks will be removed.

### Sample Payloads

```json
{
  "Datacenter": "dc1",
  "Node": "foobar"
}
```

```json
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "CheckID": "service:redis1"
}
```

```json
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "ServiceID": "redis1"
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/catalog/deregister
```

## List Datacenters

This endpoint returns the list of all known datacenters. The datacenters will be
sorted in ascending order based on the estimated median round trip time from the
server to the servers in that datacenter.

This endpoint does not require a cluster leader and will succeed even during an
availability outage. Therefore, it can be used as a simple check to see if any
Consul servers are routable.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/datacenters`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `none`       |

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/catalog/datacenters
```

### Sample Response

```json
["dc1", "dc2"]
```

## List Nodes

This endpoint and returns the nodes registered in a given datacenter.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/nodes`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `node:read`  |

### Parameters

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
    http://127.0.0.1:8500/v1/catalog/nodes
```

### Sample Response

```json
[
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "baz",
    "Address": "10.1.10.11",
    "Datacenter": "dc1",
    "TaggedAddresses": {
      "lan": "10.1.10.11",
      "wan": "10.1.10.11"
    },
    "Meta": {
      "instance_type": "t2.medium"
    }
  },
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Node": "foobar",
    "Address": "10.1.10.12",
    "Datacenter": "dc2",
    "TaggedAddresses": {
      "lan": "10.1.10.11",
      "wan": "10.1.10.12"
    },
    "Meta": {
      "instance_type": "t2.large"
    }
  }
]
```

### Filtering

The filter will be executed against each Node in the result list with
the following selectors and filter operations being supported:

| Selector                | Supported Operations               |
| ----------------------- | ---------------------------------- |
| `Address`               | Equal, Not Equal                   |
| `Datacenter`            | Equal, Not Equal                   |
| `ID`                    | Equal, Not Equal                   |
| `Meta`                  | In, Not In, Is Empty, Is Not Empty |
| `Meta.<any>`            | Equal, Not Equal                   |
| `Node`                  | Equal, Not Equal                   |
| `TaggedAddresses`       | In, Not In, Is Empty, Is Not Empty |
| `TaggedAddresses.<any>` | Equal, Not Equal                   |


## List Services

This endpoint returns the services registered in a given datacenter.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/services`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `YES`            | `all`             | `none`        | `service:read` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `node-meta` `(string: "")` - Specifies a desired node metadata key/value pair
  of the form `key:value`. This parameter can be specified multiple times, and
  will filter the results to nodes with the specified key/value pairs. This is
  specified as part of the URL as a query parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/catalog/services
```

### Sample Response

```json
{
  "consul": [],
  "redis": [],
  "postgresql": [
    "primary",
    "secondary"
  ]
}
```

The keys are the service names, and the array values provide all known tags for
a given service.

## List Nodes for Service

This endpoint returns the nodes providing a service in a given datacenter.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/service/:service`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching        | ACL Required             |
| ---------------- | ----------------- | -------------------- | ------------------------ |
| `YES`            | `all`             | `background refresh` | `node:read,service:read` |

### Parameters

- `service` `(string: <required>)` - Specifies the name of the service for which
  to list nodes. This is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `tag` `(string: "")` - Specifies the tag to filter on. This is specified as part of
  the URL as a query parameter. Can be used multiple times for additional filtering,
  returning only the results that include all of the tag values provided.

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
    http://127.0.0.1:8500/v1/catalog/service/my-service
```

### Sample Response

```json
[
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "foobar",
    "Address": "192.168.10.10",
    "Datacenter": "dc1",
    "TaggedAddresses": {
      "lan": "192.168.10.10",
      "wan": "10.0.10.10"
    },
    "NodeMeta": {
      "somekey": "somevalue"
    },
    "CreateIndex": 51,
    "ModifyIndex": 51,
    "ServiceAddress": "172.17.0.3",
    "ServiceEnableTagOverride": false,
    "ServiceID": "32a2a47f7992:nodea:5000",
    "ServiceName": "foobar",
    "ServicePort": 5000,
    "ServiceMeta": {
        "foobar_meta_value": "baz"
    },
    "ServiceTags": [
      "tacos"
    ],
    "ServiceProxyDestination": "",
    "ServiceProxy": {
        "DestinationServiceName": "",
        "DestinationServiceID": "",
        "LocalServiceAddress": "",
        "LocalServicePort": 0,
        "Config": null,
        "Upstreams": null
    },
    "ServiceConnect": {
        "Native": false,
        "Proxy": null
    },
  }
]
```

- `Address` is the IP address of the Consul node on which the service is
  registered.

- `Datacenter` is the data center of the Consul node on which the service is
  registered.

- `TaggedAddresses` is the list of explicit LAN and WAN IP addresses for the
  agent

- `NodeMeta` is a list of user-defined metadata key/value pairs for the node

- `CreateIndex` is an internal index value representing when the service was
  created

- `ModifyIndex` is the last index that modified the service

- `Node` is the name of the Consul node on which the service is registered

- `ServiceAddress` is the IP address of the service host — if empty, node
  address should be used

- `ServiceEnableTagOverride` indicates whether service tags can be overridden on
  this service

- `ServiceID` is a unique service instance identifier

- `ServiceName` is the name of the service

- `ServiceMeta` is a list of user-defined metadata key/value pairs for the service

- `ServicePort` is the port number of the service

- `ServiceTags` is a list of tags for the service

- `ServiceKind` is the kind of service, usually "". See the Agent
  registration API for more information.

- `ServiceProxyDestination` **Deprecated** this field duplicates
  `ServiceProxy.DestinationServiceName` for backwards compatibility. It will be
  removed in a future major version release.

- `ServiceProxy` is the proxy config as specified in
[Connect Proxies](/docs/connect/proxies.html).

- `ServiceConnect` are the [Connect](/docs/connect/index.html) settings. The
  value of this struct is equivalent to the `Connect` field for service
  registration.

### Filtering

Filtering is executed against each entry in the top level result list with the
following selectors and filter operations being supported:

| Selector                                      | Supported Operations               |
| --------------------------------------------- | ---------------------------------- |
| `Address`                                     | Equal, Not Equal                   |
| `Datacenter`                                  | Equal, Not Equal                   |
| `ID`                                          | Equal, Not Equal                   |
| `Node`                                        | Equal, Not Equal                   |
| `NodeMeta`                                    | In, Not In, Is Empty, Is Not Empty |
| `NodeMeta.<any>`                              | Equal, Not Equal                   |
| `ServiceAddress`                              | Equal, Not Equal                   |
| `ServiceConnect.Native`                       | Equal, Not Equal                   |
| `ServiceEnableTagOverride`                    | Equal, Not Equal                   |
| `ServiceID`                                   | Equal, Not Equal                   |
| `ServiceKind`                                 | Equal, Not Equal                   |
| `ServiceMeta`                                 | In, Not In, Is Empty, Is Not Empty |
| `ServiceMeta.<any>`                           | Equal, Not Equal                   |
| `ServiceName`                                 | Equal, Not Equal                   |
| `ServicePort`                                 | Equal, Not Equal                   |
| `ServiceProxy.DestinationServiceID`           | Equal, Not Equal                   |
| `ServiceProxy.DestinationServiceName`         | Equal, Not Equal                   |
| `ServiceProxy.LocalServiceAddress`            | Equal, Not Equal                   |
| `ServiceProxy.LocalServicePort`               | Equal, Not Equal                   |
| `ServiceProxy.Upstreams`                      | Is Empty, Is Not Empty             |
| `ServiceProxy.Upstreams.Datacenter`           | Equal, Not Equal                   |
| `ServiceProxy.Upstreams.DestinationName`      | Equal, Not Equal                   |
| `ServiceProxy.Upstreams.DestinationNamespace` | Equal, Not Equal                   |
| `ServiceProxy.Upstreams.DestinationType`      | Equal, Not Equal                   |
| `ServiceProxy.Upstreams.LocalBindAddress`     | Equal, Not Equal                   |
| `ServiceProxy.Upstreams.LocalBindPort`        | Equal, Not Equal                   |
| `ServiceTags`                                 | In, Not In, Is Empty, Is Not Empty |
| `ServiceWeights.Passing`                      | Equal, Not Equal                   |
| `ServiceWeights.Warning`                      | Equal, Not Equal                   |
| `TaggedAddresses`                             | In, Not In, Is Empty, Is Not Empty |
| `TaggedAddresses.<any>`                       | Equal, Not Equal                   |

## List Nodes for Connect-capable Service

This endpoint returns the nodes providing a
[Connect-capable](/docs/connect/index.html) service in a given datacenter.
This will include both proxies and native integrations. A service may
register both Connect-capable and incapable services at the same time,
so this endpoint may be used to filter only the Connect-capable endpoints.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/connect/:service`  | `application/json`         |

Parameters and response format are the same as
[`/catalog/service/:service`](/api/catalog.html#list-nodes-for-service).

## List Services for Node

This endpoint returns the node's registered services.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/catalog/node/:node`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required             |
| ---------------- | ----------------- | ------------- | ------------------------ |
| `YES`            | `all`             | `none`        | `node:read,service:read` |

### Parameters

- `node` `(string: <required>)` - Specifies the name of the node for which
  to list services. This is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `filter` `(string: "")` - Specifies the expression used to filter the
  queries results prior to returning the data.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/catalog/node/my-node
```

### Sample Response

```json
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
  "Services": {
    "consul": {
      "ID": "consul",
      "Service": "consul",
      "Tags": null,
      "Meta": {},
      "Port": 8300
    },
    "redis": {
      "ID": "redis",
      "Service": "redis",
      "Tags": [
        "v1"
      ],
      "Meta": {
        "redis_version": "4.0"
      },
      "Port": 8000
    }
  }
}
```

### Filtering

The filter will be executed against each value in the `Services` mapping within the
top level Node object. The following selectors and filter operations are supported:

| Selector                               | Supported Operations               |
| -------------------------------------- | ---------------------------------- |
| `Address`                              | Equal, Not Equal                   |
| `Connect.Native`                       | Equal, Not Equal                   |
| `EnableTagOverride`                    | Equal, Not Equal                   |
| `ID`                                   | Equal, Not Equal                   |
| `Kind`                                 | Equal, Not Equal                   |
| `Meta`                                 | In, Not In, Is Empty, Is Not Empty |
| `Meta.<any>`                           | Equal, Not Equal                   |
| `Port`                                 | Equal, Not Equal                   |
| `Proxy.DestinationServiceID`           | Equal, Not Equal                   |
| `Proxy.DestinationServiceName`         | Equal, Not Equal                   |
| `Proxy.LocalServiceAddress`            | Equal, Not Equal                   |
| `Proxy.LocalServicePort`               | Equal, Not Equal                   |
| `Proxy.Upstreams`                      | Is Empty, Is Not Empty             |
| `Proxy.Upstreams.Datacenter`           | Equal, Not Equal                   |
| `Proxy.Upstreams.DestinationName`      | Equal, Not Equal                   |
| `Proxy.Upstreams.DestinationNamespace` | Equal, Not Equal                   |
| `Proxy.Upstreams.DestinationType`      | Equal, Not Equal                   |
| `Proxy.Upstreams.LocalBindAddress`     | Equal, Not Equal                   |
| `Proxy.Upstreams.LocalBindPort`        | Equal, Not Equal                   |
| `Service`                              | Equal, Not Equal                   |
| `Tags`                                 | In, Not In, Is Empty, Is Not Empty |
| `Weights.Passing`                      | Equal, Not Equal                   |
| `Weights.Warning`                      | Equal, Not Equal                   |
