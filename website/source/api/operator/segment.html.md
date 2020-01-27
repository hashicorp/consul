---
layout: api
page_title: Network Segments - Operator - HTTP API
sidebar_current: api-operator-segment
description: |-
  The /operator/segment endpoint exposes the network segment information via
  Consul's HTTP API.
---

# Network Areas - Operator HTTP API

The `/operator/segment` endpoint provides tools to manage network segments via
Consul's HTTP API.

~> **Enterprise-only!** This API endpoint and functionality only exists in
Consul Enterprise. This is not present in the open source version of Consul.

The network area functionality described here is available only in
[Consul Enterprise](https://www.hashicorp.com/products/consul/) version 0.9.3 and
later. Network segments are operator-defined sections of agents on the LAN, typically
isolated from other segments by network configuration.

Please see the [Network Segments Guide](https://learn.hashicorp.com/consul/day-2-operations/network-segments) for more details.

## List Network Segments

This endpoint lists all network areas.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/segment`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `operator:read` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/operator/segment
```

### Sample Response

```json
["","alpha","beta"]
```
