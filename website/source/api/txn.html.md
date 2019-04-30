---
layout: api
page_title: Transaction - HTTP API
sidebar_current: api-txn
description: |-
  The /txn endpoint manages multiple operations in Consul, including catalog updates and fetches of multiple KV entries inside a single, atomic transaction.
---

# Transactions HTTP API

The `/txn` endpoint manages multiple operations in Consul, including catalog
updates and fetches of multiple KV entries inside a single, atomic 
transaction.

## Create Transaction

This endpoint permits submitting a list of operations to apply to Consul
inside of a transaction. If any operation fails, the transaction is rolled back
and none of the changes are applied.

If the transaction does not contain any write operations then it will be
fast-pathed internally to an endpoint that works like other reads, except that
blocking queries are not currently supported. In this mode, you may supply the
`?stale` or `?consistent` query parameters with the request to control
consistency. To support bounding the acceptable staleness of data, read-only
transaction responses provide the `X-Consul-LastContact` header containing the
time in milliseconds that a server was last contacted by the leader node. The
`X-Consul-KnownLeader` header also indicates if there is a known leader. These
won't be present if the transaction contains any write operations, and any
consistency query parameters will be ignored, since writes are always managed by
the leader via the Raft consensus protocol.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/txn`                       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `all`<sup>1</sup> | `none`        | `key:read,key:write`<br>`node:read,node:write`<br>`service:read,service:write`<sup>2</sup>

<sup>1</sup> For read-only transactions
<br>
<sup>2</sup> The ACL required depends on the operations in the transaction.

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default
  to the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `KV` operations have the following fields:

  - `Verb` `(string: <required>)` - Specifies the type of operation to perform.
    Please see the table below for available verbs.

  - `Key` `(string: <required>)` - Specifies the full path of the entry.

  - `Value` `(string: "")` - Specifies a **base64-encoded** blob of data. Values
    cannot be larger than 512kB.

  - `Flags` `(int: 0)` - Specifies an opaque unsigned integer that can be
    attached to each entry. Clients can choose to use this however makes sense
    for their application.

  - `Index` `(int: 0)` - Specifies an index. See the table below for more
    information.

  - `Session` `(string: "")` - Specifies a session. See the table below for more
    information.
    
- `Node` operations have the following fields:

  - `Verb` `(string: <required>)` - Specifies the type of operation to perform.

  - `Node` `(Node: <required>)` - Specifies the node information to use
  for the operation. See the [catalog endpoint](/api/catalog.html#parameters) for the fields in this object. Note the only the node can be specified here, not any services or checks - separate service or check operations must be used for those.

- `Service` operations have the following fields:

  - `Verb` `(string: <required>)` - Specifies the type of operation to perform.

  - `Node` `(string: <required>)` = Specifies the name of the node to use for
  this service operation.

  - `Service` `(Service: <required>)` - Specifies the service instance  information to use
  for the operation. See the [catalog endpoint](/api/catalog.html#parameters) for the fields in this object.

- `Check` operations have the following fields:

  - `Verb` `(string: <required>)` - Specifies the type of operation to perform.

  - `Service` `(Service: <required>)` - Specifies the check to use
  for the operation. See the [catalog endpoint](/api/catalog.html#parameters) for the fields in this object.

  Please see the table below for available verbs.
### Sample Payload

The body of the request should be a list of operations to perform inside the
atomic transaction. Up to 64 operations may be present in a single transaction.

```javascript
[
  {
    "KV": {
      "Verb": "<verb>",
      "Key": "<key>",
      "Value": "<Base64-encoded blob of data>",
      "Flags": <flags>,
      "Index": <index>,
      "Session": "<session id>"
    }
  },
  {
    "Node": {
      "Verb": "set",
      "Node": {
        "ID": "67539c9d-b948-ba67-edd4-d07a676d6673",
        "Node": "bar",
        "Address": "192.168.0.1",
        "Datacenter": "dc1",
        "Meta": {
          "instance_type": "m2.large"
        }
      }
    }
  },
  {
    "Service": {
      "Verb": "delete",
      "Node": "foo",
      "Service": {
        "ID": "db1"
      }
    }
  },
  {
    "Check": {
      "Verb": "cas",
      "Check": {
	      "Node": "bar",
        "CheckID": "service:web1",
        "Name": "Web HTTP Check",
        "Status": "critical",
        "ServiceID": "web1",
        "ServiceName": "web",
        "ServiceTags": null,
        "Definition": {
          "HTTP": "http://localhost:8080",
          "Interval": "10s"
        },
        "ModifyIndex": 22
      }
    }
  }
]
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/txn
```

### Sample Response

If the transaction can be processed, a status code of 200 will be returned if it
was successfully applied, or a status code of 409 will be returned if it was
rolled back. If either of these status codes are returned, the response will
look like this:

```javascript
{
  "Results": [
    {
      "KV": {
        "LockIndex": <lock index>,
        "Key": "<key>",
        "Flags": <flags>,
        "Value": "<Base64-encoded blob of data, or null>",
        "CreateIndex": <index>,
        "ModifyIndex": <index>
      }
    },
    {
      "Node": {
        "ID": "67539c9d-b948-ba67-edd4-d07a676d6673",
        "Node": "bar",
        "Address": "192.168.0.1",
        "Datacenter": "dc1",
        "TaggedAddresses": null,
        "Meta": {
          "instance_type": "m2.large"
        },
        "CreateIndex": 32,
        "ModifyIndex": 32
      }
    },
    {
      "Check": {
        "Node": "bar",
        "CheckID": "service:web1",
        "Name": "Web HTTP Check",
        "Status": "critical",
        "Notes": "",
        "Output": "",
        "ServiceID": "web1",
        "ServiceName": "web",
        "ServiceTags": null,
        "Definition": {
          "HTTP": "http://localhost:8080",
          "Interval": "10s"
        },
        "CreateIndex": 22,
        "ModifyIndex": 35
      }
    }
  ],
  "Errors": [
    {
      "OpIndex": <index of failed operation>,
      "What": "<error message for failed operation>"
    },
    ...
  ]
}
```

- `Results` has entries for some operations if the transaction was successful.
  To save space, the `Value` for KV results will be `null` for any `Verb` other than "get" or
  "get-tree". Like the `/v1/kv/<key>` endpoint, `Value` will be Base64-encoded
  if it is present. Also, no result entries  will be added for verbs that delete
  keys.

- `Errors` has entries describing which operations failed if the transaction was
  rolled back. The `OpIndex` gives the index of the failed operation in the
  transaction, and `What` is a string with an error message about why that
  operation failed.

### Tables of Operations

#### KV Operations

The following tables summarize the available verbs and the fields that apply to
those operations ("X" means a field is required and "O" means it is optional):

| Verb               | Operation                                    | Key  | Value | Flags | Index | Session |
| ------------------ | -------------------------------------------- | :--: | :---: | :---: | :---: | :-----: |
| `set`              | Sets the `Key` to the given `Value`          | `x`  | `x`   | `o`   |       |         |
| `cas`              | Sets, but with CAS semantics                 | `x`  | `x`   | `o`   | `x`   |         |
| `lock`             | Lock with the given `Session`                | `x`  | `x`   | `o`   |       | `x`     |
| `unlock`           | Unlock with the given `Session`              | `x`  | `x`   | `o`   |       | `x`     |
| `get`              | Get the key, fails if it does not exist      | `x`  |       |       |       |         |
| `get-tree`         | Gets all keys with the prefix                | `x`  |       |       |       |         |
| `check-index`      | Fail if modify index != index                | `x`  |       |       | `x`   |         |
| `check-session`    | Fail if not locked by session                | `x`  |       |       |       | `x`     |
| `check-not-exists` | Fail if key exists                           | `x`  |       |       |       |         |
| `delete`           | Delete the key                               | `x`  |       |       |       |         |
| `delete-tree`      | Delete all keys with a prefix                | `x`  |       |       |       |         |
| `delete-cas`       | Delete, but with CAS semantics               | `x`  |       |       | `x`   |         |

#### Node Operations

Node operations act on an individual node and require either a Node ID or name, giving precedence 
to the ID if both are set. Delete operations will not return a result on success.

| Verb               | Operation                                    |
| ------------------ | -------------------------------------------- |
| `set`              | Sets the node to the given state            |
| `cas`              | Sets, but with CAS semantics using the given ModifyIndex |
| `get`              | Get the node, fails if it does not exist |
| `delete`           | Delete the node |
| `delete-cas`       | Delete, but with CAS semantics |

#### Service Operations

Service operations act on an individual service instance on the given node name. Both a node name
and valid service name are required. Delete operations will not return a result on success.

| Verb               | Operation                                    |
| ------------------ | -------------------------------------------- |
| `set`              | Sets the service to the given state            |
| `cas`              | Sets, but with CAS semantics using the given ModifyIndex |
| `get`              | Get the service, fails if it does not exist |
| `delete`           | Delete the service |
| `delete-cas`       | Delete, but with CAS semantics |

#### Check Operations

Check operations act on an individual health check instance on the given node name. Both a node name
and valid check ID are required. Delete operations will not return a result on success.

| Verb               | Operation                                    |
| ------------------ | -------------------------------------------- |
| `set`              | Sets the health check to the given state            |
| `cas`              | Sets, but with CAS semantics using the given ModifyIndex |
| `get`              | Get the check, fails if it does not exist |
| `delete`           | Delete the check |
| `delete-cas`       | Delete, but with CAS semantics |