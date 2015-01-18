---
layout: "docs"
page_title: "Key/Value store (HTTP)"
sidebar_current: "docs-agent-http-kv"
description: >
  The KV endpoint is used to expose a simple key/value store. This can be used
  to store service configurations or other meta data in a simple way.
---

# Key/Value HTTP Endpoint

The KV endpoint is used to expose a simple key/value store. This can be used
to store service configurations or other meta data in a simple way. It has only
a single endpoint:

    /v1/kv/<key>

This is the only endpoint that is used with the Key/Value store.
Its use depends on the HTTP method. The `GET`, `PUT` and `DELETE` methods
are all supported. It is important to note that each datacenter has its
own K/V store, and that there is no replication between datacenters.
By default the datacenter of the agent is queried, however the dc can
be provided using the "?dc=" query parameter. If a client wants to write
to all Datacenters, one request per datacenter must be made. The KV endpoint
supports the use of ACL tokens.

If you are interested in Key/Value replication between datacenters,
look at the [consul-replicate project](https://github.com/hashicorp/consul-replicate).

### GET Method

When using the `GET` method, Consul will return the specified key,
or if the "?recurse" query parameter is provided, it will return
all keys with the given prefix.

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

The `CreateIndex` is the internal index value that represents
when the entry was created. The `ModifyIndex` is the last index
that modified this key. This index corresponds to the `X-Consul-Index`
header value that is returned. A blocking query can be used to wait for
a value to change. If "?recurse" is used, the `X-Consul-Index` corresponds
to the latest `ModifyIndex` and so a blocking query waits until any of the
listed keys are updated.  The `LockIndex` is the last index of a successful
lock acquisition. If the lock is held, the `Session` key provides the
session that owns the lock.

The `Key` is simply the full path of the entry. `Flags` are an opaque
unsigned integer that can be attached to each entry. The use of this is
left totally to the user. The `Value` is a base64 key value.

It is possible to also only list keys without their values by using the
"?keys" query parameter along with a `GET` request. This will return
a list of the keys under the given prefix. The optional "?separator="
can be used to list only up to a given separator.

For example, listing "/web/" with a "/" separator may return:

```javascript
[
  "/web/bar",
  "/web/foo",
  "/web/subdir/"
]
```

Using the key listing method may be suitable when you do not need
the values or flags, or want to implement a key-space explorer.

If the "?raw" query parameter is used with a non-recursive GET,
then the response is just the raw value of the key, without any
encoding.

If no entries are found, a 404 code is returned.

This endpoint supports blocking queries and all consistency modes.

### PUT method

When using the `PUT` method, Consul expects the request body to be the
value corresponding to the key. There are a number of parameters that can
be used with a PUT request:

* ?flags=\<num\> : This can be used to specify an unsigned value between
  0 and 2^64-1. It is opaque to the user, but a client application may
  use it.

* ?cas=\<index\> : This flag is used to turn the `PUT` into a Check-And-Set
  operation. This is very useful as it allows clients to build more complex
  synchronization primitives on top. If the index is 0, then Consul will only
  put the key if it does not already exist. If the index is non-zero, then
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

The return value is simply either `true` or `false`. If `false` is returned,
then the update has not taken place.

### DELETE method

The `DELETE` method can be used to delete a single key or all keys sharing
a prefix.  There are a number of query parameters that can be used with a
DELETE request:

* ?recurse : This is used to delete all keys which have the specified prefix.
  Without this, only a key with an exact match will be deleted.

* ?cas=\<index\> : This flag is used to turn the `DELETE` into a Check-And-Set
  operation. This is very useful as it allows clients to build more complex
  synchronization primitives on top. If the index is 0, then Consul will only
  delete the key if it does not already exist (noop). If the index is non-zero, then
  the key is only deleted if the index matches the `ModifyIndex` of that key.
