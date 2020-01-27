---
layout: api
page_title: Status - HTTP API
sidebar_current: api-status
description: |-
  The /status endpoints return information about the status of the Consul
  cluster. This information is generally very low level and not often useful for
  clients.
---

# Status HTTP API

The `/status` endpoints return information about the status of the Consul
cluster. This information is generally very low level and not often useful for
clients.

## Get Raft Leader

This endpoint returns the Raft leader for the datacenter in which the agent is
running.

| Method | Path                         | Produces               |
| :----- | :--------------------------- | ---------------------- |
| `GET`  | `/status/leader`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `none`       |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

### Sample Request

```text
$ curl http://127.0.0.1:8500/v1/status/leader
```

### Sample Response

```json
"10.1.10.12:8300"
```

## List Raft Peers

This endpoint retrieves the Raft peers for the datacenter in which the the agent
is running. This list of peers is strongly consistent and can be useful in
determining when a given server has successfully joined the cluster.

| Method | Path                         | Produces               |
| :----- | :--------------------------- | ---------------------- |
| `GET`  | `/status/peers`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `none`       |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

### Sample Request

```text
$ curl http://127.0.0.1:8500/v1/status/peers
```

### Sample Response

```json
[
  "10.1.10.12:8300",
  "10.1.10.11:8300",
  "10.1.10.10:8300"
]
```
