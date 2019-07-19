---
layout: api
page_title: Network Areas - Operator - HTTP API
sidebar_current: api-operator-area
description: |-
  The /operator/area endpoints expose the network tomography information via
  Consul's HTTP API.
---

# Network Areas - Operator HTTP API

The `/operator/area` endpoints provide tools to manage network areas via
Consul's HTTP API.

~> **Enterprise Only!** This API endpoint and functionality only exists in
Consul Enterprise. This is not present in the open source version of Consul.

The network area functionality described here is available only in
[Consul Enterprise](https://www.hashicorp.com/products/consul/) version 0.8.0 and
later. Network areas are operator-defined relationships between servers in two
different Consul datacenters.

Unlike Consul's WAN feature, network areas use just the server RPC port for
communication, and relationships can be made between independent pairs of
datacenters, so not all servers need to be fully connected. This allows for
complex topologies among Consul datacenters like hub/spoke and more general
trees.

Please see the [Network Areas Guide](https://learn.hashicorp.com/consul/day-2-operations/advanced-federation) for more details.

## Create Network Area

This endpoint creates a new network area and returns its ID if it is created
successfully.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/operator/area`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required     |
| ---------------- | ----------------- | ------------- | ---------------- |
| `NO`             | `none`            | `none`        | `operator:write` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

- `PeerDatacenter` `(string: <required>)` - Specifies the name of the Consul
  datacenter that will be joined the Consul servers in the current datacenter to
  form the area. Only one area is allowed for each possible `PeerDatacenter`,
  and a datacenter cannot form an area with itself.

- `RetryJoin` `(array<string>: nil)`- Specifies a list of Consul servers to
  attempt to join. Servers can be given as `IP`, `IP:port`, `hostname`, or
  `hostname:port`. Consul will spawn a background task that tries to
  periodically join the servers in this list and will run until a join succeeds.
  If this list is not supplied, joining can be done with a call to the
  [join endpoint](#area-join) once the network area is created.

- `UseTLS` `(bool: <optional>)` - Specifies whether gossip over this area should be
  encrypted with TLS if possible.

### Sample Payload

```json
{
  "PeerDatacenter": "dc2",
  "RetryJoin": [ "10.1.2.3", "10.1.2.4", "10.1.2.5" ],
  "UseTLS": false
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/area
```

### Sample Response

```json
{
  "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05"
}
```

## List Network Areas

This endpoint lists all network areas.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/area`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `YES`            | `all`             | `none`        | `operator:read` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/operator/area
```

### Sample Response

```json
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "PeerDatacenter": "dc2",
    "RetryJoin": ["10.1.2.3", "10.1.2.4", "10.1.2.5"]
  }
]
```

## Update Network Area

This endpoint updates a network area to the given configuration.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/operator/area/:uuid`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required     |
| ---------------- | ----------------- | ------------- | ---------------- |
| `NO`             | `none`            | `none`        | `operator:write` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

- `UseTLS` `(bool: <optional>)` - Specifies whether gossip over this area should be
  encrypted with TLS if possible.

### Sample Payload

```json
{
  "UseTLS": true
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/area/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

## List Specific Network Area

This endpoint lists a specific network area.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/area/:uuid`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `YES`            | `all`             | `none`        | `operator:read` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the area to list. This
  is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/operator/area/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response

```json
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "PeerDatacenter": "dc2",
    "RetryJoin": ["10.1.2.3", "10.1.2.4", "10.1.2.5"]
  }
]
```

## Delete Network Area

This endpoint deletes a specific network area.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/operator/area/:uuid`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required     |
| ---------------- | ----------------- | ------------- | ---------------- |
| `NO`             | `none`            | `none`        | `operator:write` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the area to delete. This
  is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    --request DELETE \
    http://127.0.0.1:8500/v1/operator/area/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

## Join Network Area

This endpoint attempts to join the given Consul servers into a specific network
area.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/operator/area/:uuid/join`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required     |
| ---------------- | ----------------- | ------------- | ---------------- |
| `NO`             | `none`            | `none`        | `operator:write` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the area to join. This
  is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Payload

```json
["10.1.2.3", "10.1.2.4", "10.1.2.5"]
```

This can be provided as `IP`, `IP:port`, `hostname`, or `hostname:port`.

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/area/8f246b77-f3e1-ff88-5b48-8ec93abf3e05/join
```

### Sample Response

```json
[
  {
    "Address": "10.1.2.3",
    "Joined": true,
    "Error": ""
  },
  {
    "Address": "10.1.2.4",
    "Joined": true,
    "Error": ""
  },
  {
    "Address": "10.1.2.5",
    "Joined": true,
    "Error": ""
  }
]
```

- `Address` has the address requested to join.

- `Joined` will be `true` if the Consul server at the given address was
  successfully joined into the network area. Otherwise, this will be `false` and
  `Error` will have a human-readable message about why the join didn't succeed.

## List Network Area Members

This endpoint provides a listing of the Consul servers present in a specific
network area.

| Method | Path                           | Produces                   |
| ------ | ------------------------------ | -------------------------- |
| `GET`  | `/operator/area/:uuid/members` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `operator:read` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the area to list. This
  is specified as part of the URL.

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/operator/area/8f246b77-f3e1-ff88-5b48-8ec93abf3e05/members
```

### Sample Response

```json
[
  {
    "ID": "afc5d95c-1eee-4b46-b85b-0efe4c76dd48",
    "Name": "node-2.dc1",
    "Addr": "127.0.0.2",
    "Port": 8300,
    "Datacenter": "dc1",
    "Role": "server",
    "Build": "0.8.0",
    "Protocol": 2,
    "Status": "alive",
    "RTT": 256478
  },
]
```

- `ID` is the node ID of the server.

- `Name` is the node name of the server, with its datacenter appended.

- `Addr` is the IP address of the node.

- `Port` is the server RPC port of the node.

- `Datacenter` is the node's Consul datacenter.

- `Role` is always "server" since only Consul servers can participate in network
  areas.

- `Build` has the Consul version running on the node.

- `Protocol` is the [protocol version](/docs/upgrading.html#protocol-versions)
  being spoken by the node.

- `Status` is the current health status of the node, as determined by the
  network area distributed failure detector. This will be "alive", "leaving",
  "left", or "failed". A "failed" status means that other servers are not able
  to probe this server over its server RPC interface.

- `RTT` is an estimated network round trip time from the server answering the
  query to the given server, in nanoseconds. This is computed using [network
  coordinates](/docs/internals/coordinates.html).
