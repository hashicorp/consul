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

Please see the [Network Segments Guide](/docs/guides/segments.html) for more details.

## List Network Segments

This endpoint lists all network areas.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/segment`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `NO`             | `none`            | `operator:read` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as a URL query
  parameter.

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/operator/segment
```

### Sample Response

```json
["","alpha","beta"]
```