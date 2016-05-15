---
layout: "docs"
page_title: "Key/Value Store (HTTP)"
sidebar_current: "docs-agent-http-kv"
description: >
  The KV endpoints are used to access Consul's simple key/value store, useful for storing
  service configuration or other metadata.
---

# Key/Value Store Endpoints

The KV endpoints are used to access Consul's simple key/value store, useful for storing
service configuration or other metadata.

The following endpoints are supported:

* [`/v1/kv/<key>`](#single): Manages updates of individual keys, deletes of individual
  keys or key prefixes, and fetches of individual keys or key prefixes
* [`/v1/txn`](#txn): Manages updates or fetches of multiple keys inside a single,
  atomic transaction

### <a name="single"></a> /v1/kv/&lt;key&gt;

This endpoint manages updates of individual keys, deletes of individual keys or key
prefixes, and fetches of individual keys or key prefixes. The `GET`, `PUT` and
`DELETE` methods are all supported.

By default, the datacenter of the agent is queried; however, the dc can be provided
using the "?dc=" query parameter. It is important to note that each datacenter has
its own KV store, and there is no built-in replication between datacenters. If you
are interested in replication between datacenters, look at the
[Consul Replicate project](https://github.com/hashicorp/consul-replicate).

The KV endpoint supports the use of ACL tokens using the "?token=" query parameter.

#### GET Method

When using the `GET` method, Consul will return the specified key.
If the "?recurse" query parameter is provided, it will return
all keys with the given prefix.

This endpoint supports blocking queries and all consistency modes.

Each object will look like:

```javascript
[
  {
    "CreateIndex": 100,
    "ModifyIndex": 200,
    "LockIndex": 200,
    "Key": "zip",
    "Flags": 0,
    "Value": "dGVzdA==",
    "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
  }
]
```

`CreateIndex` is the internal index value that represents
when the entry was created.

`ModifyIndex` is the last index that modified this key. This index corresponds
to the `X-Consul-Index` header value that is returned in responses, and it can
be used to establish blocking queries by setting the "?index" query parameter.
You can even perform blocking queries against entire subtrees of the KV store:
if "?recurse" is provided, the returned `X-Consul-Index` corresponds
to the latest `ModifyIndex` within the prefix, and a blocking query using that
"?index" will wait until any key within that prefix is updated.

`LockIndex` is the number of times this key has successfully been acquired in
a lock. If the lock is held, the `Session` key provides the session that owns
the lock.

`Key` is simply the full path of the entry.

`Flags` is an opaque unsigned integer that can be attached to each entry. Clients
can choose to use this however makes sense for their application.

`Value` is a Base64-encoded blob of data.  Note that values cannot be larger than
512kB.

It is possible to list just keys without their values by using the "?keys" query
parameter. This will return a list of the keys under the given prefix. The optional
"?separator=" can be used to list only up to a given separator.

For example, listing "/web/" with a "/" separator may return:

```javascript
[
  "/web/bar",
  "/web/foo",
  "/web/subdir/"
]
```

Using the key listing method may be suitable when you do not need
the values or flags or want to implement a key-space explorer.

If the "?raw" query parameter is used with a non-recursive GET,
the response is just the raw value of the key, without any
encoding.

If no entries are found, a 404 code is returned.

#### PUT method

When using the `PUT` method, Consul expects the request body to be the
value corresponding to the key. There are a number of query parameters that can
be used with a PUT request:

* ?flags=\<num\> : This can be used to specify an unsigned value between
  0 and (2^64)-1. Clients can choose to use this however makes sense for their application.

* ?cas=\<index\> : This flag is used to turn the `PUT` into a Check-And-Set
  operation. This is very useful as a building block for more complex
  synchronization primitives. If the index is 0, Consul will only
  put the key if it does not already exist. If the index is non-zero,
  the key is only set if the index matches the `ModifyIndex` of that key.

* ?acquire=\<session\> : This flag is used to turn the `PUT` into a lock acquisition
  operation. This is useful as it allows leader election to be built on top
  of Consul. If the lock is not held and the session is valid, this increments
  the `LockIndex` and sets the `Session` value of the key in addition to updating
  the key contents. A key does not need to exist to be acquired. If the lock is
  already held by the given session, then the `LockIndex` is not incremented but
  the key contents are updated. This lets the current lock holder update the key
  contents without having to give up the lock and reacquire it.

* ?release=\<session\> : This flag is used to turn the `PUT` into a lock release
  operation. This is useful when paired with "?acquire=" as it allows clients to
  yield a lock. This will leave the `LockIndex` unmodified but will clear the associated
  `Session` of the key. The key must be held by this session to be unlocked.

The return value is either `true` or `false`. If `false` is returned,
the update has not taken place.

#### DELETE method

The `DELETE` method can be used to delete a single key or all keys sharing
a prefix.  There are a few query parameters that can be used with a
DELETE request:

* ?recurse : This is used to delete all keys which have the specified prefix.
  Without this, only a key with an exact match will be deleted.

* ?cas=\<index\> : This flag is used to turn the `DELETE` into a Check-And-Set
  operation. This is very useful as a building block for more complex
  synchronization primitives. Unlike `PUT`, the index must be greater than 0
  for Consul to take any action: a 0 index will not delete the key. If the index
  is non-zero, the key is only deleted if the index matches the `ModifyIndex` of that key.

### <a name="txn"></a> /v1/txn

Available in Consul 0.7 and later, this endpoint manages updates or fetches of
multiple keys inside a single, atomic transaction. Only the `PUT` method is supported.

By default, the datacenter of the agent receives the transaction; however, the dc
can be provided using the "?dc=" query parameter. It is important to note that each
datacenter has its own KV store, and there is no built-in replication between
datacenters. If you are interested in replication between datacenters, look at the
[Consul Replicate project](https://github.com/hashicorp/consul-replicate).

The transaction endpoint supports the use of ACL tokens using the "?token=" query
parameter.

#### PUT Method

The `PUT` method lets you submit a list of operations to apply to the key/value store
inside a transaction. If any operation fails, the transaction will be rolled back and
none of the changes will be applied.

If the transaction doesn't contain any write operations then it will be fast-pathed
internally to an endpoint that works like other reads, except that blocking queries
are not currently supported. In this mode, you may supply the "?stale" or "?consistent"
query parameters with the request to control consistency. To support bounding the
acceptable staleness of data, read-only transaction responses provide the `X-Consul-LastContact`
header containing the time in milliseconds that a server was last contacted by the leader node.
The `X-Consul-KnownLeader` header also indicates if there is a known leader. These
won't be present if the transaction contains any write operations, and any consistency
query parameters will be ignored, since writes are always managed by the leader via
the Raft consensus protocol.

The body of the request should be a list of operations to perform inside the atomic
transaction. Up to 64 operations may be present in a single transaction. Operations
look like this:

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
  ...
]
```

`KV` is the only available operation type, though other types of operations may be added
in future versions of Consul to be mixed with key/value operations. The following fields
are available:

* `Verb` is the type of operation to perform. Please see the table below for
available verbs.

* `Key` is simply the full path of the entry.

* `Value` is a Base64-encoded blob of data.  Note that values cannot be larger than
512kB.

* `Flags` is an opaque unsigned integer that can be attached to each entry. Clients
can choose to use this however makes sense for their application.

* `Index` and `Session` are used for locking, unlocking, and check-and-set operations.
Please see the table below for details on how they are used.

The following table summarizes the available verbs and the fields that apply to that
operation ("X" means a field is required and "O" means it is optional):

<table class="table table-bordered table-striped">
  <tr>
    <th>Verb</th>
    <th>Operation</th>
    <th>Key</th>
    <th>Value</th>
    <th>Flags</th>
    <th>Index</th>
    <th>Session</th>
  </tr>
  <tr>
    <td>set</td>
    <td>Sets the `Key` to the given `Value`.</td>
    <td align="center">X</td>
    <td align="center">X</td>
    <td align="center">O</td>
    <td align="center"></td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>cas</td>
    <td>Sets the `Key` to the given `Value` with check-and-set semantics. The `Key` will only be set if its current modify index matches the supplied `Index`.</td>
    <td align="center">X</td>
    <td align="center">X</td>
    <td align="center">O</td>
    <td align="center">X</td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>lock</td>
    <td>Locks the `Key` with the given `Session`. The `Key` will only obtain the lock if the `Session` is valid, and no other session has it locked.</td>
    <td align="center">X</td>
    <td align="center">X</td>
    <td align="center">O</td>
    <td align="center"></td>
    <td align="center">X</td>
  </tr>
  <tr>
    <td>unlock</td>
    <td>Unlocks the `Key` with the given `Session`. The `Key` will only release the lock if the `Session` is valid and currently has it locked.</td>
    <td align="center">X</td>
    <td align="center">X</td>
    <td align="center">O</td>
    <td align="center"></td>
    <td align="center">X</td>
  </tr>
  <tr>
    <td>get</td>
    <td>Gets the `Key` during the transaction. This fails the transaction if the `Key` doesn't exist. The key may not be present in the results if ACLs do not permit it to be read.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>get-tree</td>
    <td>Gets all keys with a prefix of `Key` during the transaction. This does not fail the transaction if the `Key` doesn't exist. Not all keys may be present in the results if ACLs do not permit them to be read.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>check-index</td>
    <td>Fails the transaction if `Key` does not have a modify index equal to `Index`.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center">X</td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>check-session</td>
    <td>Fails the transaction if `Key` is not currently locked by `Session`.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center">X</td>
  </tr>
  <tr>
    <td>delete</td>
    <td>Deletes the `Key`.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>delete-tree</td>
    <td>Deletes all keys with a prefix of`Key`.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center"></td>
  </tr>
  <tr>
    <td>delete-cas</td>
    <td>Deletes the `Key` with check-and-set semantics. The `Key` will only be deleted if its current modify index matches the supplied `Index`.</td>
    <td align="center">X</td>
    <td align="center"></td>
    <td align="center"></td>
    <td align="center">X</td>
    <td align="center"></td>
  </tr>
</table>

If the transaction can be processed, a status code of 200 will be returned if it
was successfully applied, or a status code of 409 will be returned if it was rolled
back. If either of these status codes are returned, the response will look like this:

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
    ...
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

`Results` has entries for some operations if the transaction was successful. To save
space, the `Value` will be `null` for any `Verb` other than "get" or "get-tree". Like
the `/v1/kv/<key>` endpoint, `Value` will be Base64-encoded if it is present. Also,
no result entries  will be added for verbs that delete keys.

`Errors` has entries describing which operations failed if the transaction was rolled
back. The `OpIndex` gives the index of the failed operation in the transaction, and
`What` is a string with an error message about why that operation failed.

If any other status code is returned, such as 400 or 500, then the body of the response
will simply be an unstructured error message about what happened.
