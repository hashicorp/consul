---
layout: "docs"
page_title: "Key/Value store (HTTP)"
sidebar_current: "docs-agent-http-kv"
description: >
  The KV endpoint is used to access Consul's simple key/value store, useful for storing
  service configuration or other metadata.
---

# Key/Value HTTP Endpoint

The KV endpoint is used to access Consul's simple key/value store, useful for storing
service configuration or other metadata.

It has only a single endpoint:

    /v1/kv/<key>

The `GET`, `PUT` and `DELETE` methods are all supported.

By default, the datacenter of the agent is queried; however, the dc can be provided
using the "?dc=" query parameter. It is important to note that each datacenter has
its own KV store, and there is no built-in replication between datacenters. If you
are interested in replication between datacenters, look at the
[Consul Replicate project](https://github.com/hashicorp/consul-replicate).

The KV endpoint supports the use of ACL tokens.

### GET Method

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

`LockIndex` is the last index of a successful lock acquisition. If the lock is
held, the `Session` key provides the session that owns the lock.

`Key` is simply the full path of the entry.

`Flags` are an opaque unsigned integer that can be attached to each entry. Clients
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

### PUT method

When using the `PUT` method, Consul expects the request body to be the
value corresponding to the key. There are a number of query parameters that can
be used with a PUT request:

* ?flags=\<num\> : This can be used to specify an unsigned value between
  0 and 2^64-1. Clients can choose to use this however makes sense for their application.

* ?cas=\<index\> : This flag is used to turn the `PUT` into a Check-And-Set
  operation. This is very useful as a building block for more complex
  synchronization primitives. If the index is 0, Consul will only
  put the key if it does not already exist. If the index is non-zero,
  the key is only set if the index matches the `ModifyIndex` of that key.

* ?acquire=\<session\> : This flag is used to turn the `PUT` into a lock acquisition
  operation. This is useful as it allows leader election to be built on top
  of Consul. If the lock is not held and the session is valid, this increments
  the `LockIndex` and sets the `Session` value of the key in addition to updating
  the key contents. A key does not need to exist to be acquired.

* ?release=\<session\> : This flag is used to turn the `PUT` into a lock release
  operation. This is useful when paired with "?acquire=" as it allows clients to
  yield a lock. This will leave the `LockIndex` unmodified but will clear the associated
  `Session` of the key. The key must be held by this session to be unlocked.

The return value is either `true` or `false`. If `false` is returned,
the update has not taken place.

### DELETE method

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
