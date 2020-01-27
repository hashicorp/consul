---
layout: "docs"
page_title: "Application Leader Election with Sessions"
sidebar_current: "docs-guides-leader"
description: |-
  This guide describes how to build client-side leader election using Consul. If you are interested in the leader election used internally to Consul, please refer to the consensus protocol documentation instead.
---

# Application Leader Election with Sessions

For some applications, like HDFS, it is necessary to set one instance as
a leader. This ensures the application data is current and stable.

This guide describes how to build client-side leader elections for service 
instances, using Consul. Consul's support for
[sessions](/docs/internals/sessions.html) allows you to build a system that can gracefully handle failures.

If you
are interested in the leader election used internally by Consul, please refer to the
[consensus protocol](/docs/internals/consensus.html) documentation instead.

## Contending Service Instances 

Imagine you have a set of MySQL service instances who are attempting to acquire leadership. All service instances that are participating should agree on a given
key to coordinate. A good pattern is simply:

```text
service/<service name>/leader
```

This key will be used for all requests to the Consul KV API.

We will use the same, simple pattern for the MySQL services for the remainder of the guide.

```text
service/mysql/leader
```

### Create a Session 

The first step is to create a session using the
[Session HTTP API](/api/session.html#session_create).

```sh
$ curl  -X PUT -d '{"Name": "mysql-session"}' http://localhost:8500/v1/session/create
```

This will return a JSON object containing the session ID:

```json
{
  "ID": "4ca8e74b-6350-7587-addf-a18084928f3c"
}
```

### Acquire a Session 

The next step is to acquire a session for a given key from this instance
using the PUT method on a [KV entry](/api/kv.html) with the
`?acquire=<session>` query parameter. 

The `<body>` of the PUT should be a
JSON object representing the local instance. This value is opaque to
Consul, but it should contain whatever information clients require to
communicate with your application (e.g., it could be a JSON object
that contains the node's name and the application's port).

```sh
$ curl -X PUT -d <body> http://localhost:8500/v1/kv/service/mysql/leader?acquire=4ca8e74b-6350-7587-addf-a18084928f3c
 ```

This will either return `true` or `false`. If `true`, the lock has been acquired and
the local service instance is now the leader. If `false` is returned, some other node has acquired
the lock.

### Watch the Session 

All instances now remain in an idle waiting state. In this state, they watch for changes
on the key `service/mysql/leader`. This is because the lock may be released or the instance could fail, etc.

The leader must also watch for changes since its lock may be released by an operator
or automatically released due to a false positive in the failure detector.

By default, the session makes use of only the gossip failure detector. That
is, the session is considered held by a node as long as the default Serf health check
has not declared the node unhealthy. Additional checks can be specified if desired.

Watching for changes is done via a blocking query against the key. If they ever
notice that the `Session` field in the response is blank, there is no leader, and then should
retry lock acquisition. Each attempt to acquire the key should be separated by a timed
wait. This is because Consul may be enforcing a [`lock-delay`](/docs/internals/sessions.html).

### Release the Session

If the leader ever wishes to step down voluntarily, this should be done by simply
releasing the lock:

```sh
$ curl -X PUT http://localhost:8500/v1/kv/service/mysql/leader?release=4ca8e74b-6350-7587-addf-a18084928f3c
```

## Discover the Leader

It is possible to identify the leader of a set of service instances participating in the election process.

As with leader election, all instances that are participating should agree on the key being used to coordinate. 

### Retrieve the Key  

Instances have a very simple role, they simply read the Consul KV key to discover the current leader. If the key has an associated `Session`, then there is a leader.

```sh
$ curl -X GET http://localhost:8500/v1/kv/service/mysql/leader
[
  {
    "Session": "4ca8e74b-6350-7587-addf-a18084928f3c",
    "Value": "Ym9keQ==",
    "Flags": 0,
    "Key": "service/mysql/leader",
    "LockIndex": 1,
    "ModifyIndex": 29,
    "CreateIndex": 29
  }
]
```

If there is a leader then the value of the key will provide all the
application-dependent information required as a Base64 encoded blob in
the `Value` field.

### Retrieve Session Information

You can query the
[`/v1/session/info`](/api/session.html#session_info)
endpoint to get details about the session

```sh
$ curl -X GET http://localhost:8500/v1/session/info/4ca8e74b-6350-7587-addf-a18084928f3c
[
  {
    "LockDelay": 1.5e+10,
    "Checks": [
      "serfHealth"
    ],
    "Node": "consul-primary-bjsiobmvdij6-node-lhe5ihreel7y",
    "Name": "mysql-session",
    "ID": "4ca8e74b-6350-7587-addf-a18084928f3c",
    "CreateIndex": 28
  }
]
```

## Summary

In this guide you used a session to initiate manual leader election for a
set of service instances. To fully benefit from this process, instances should also watch the key using a blocking query for any
changes. If the leader steps down or fails, the `Session` associated
with the key will be cleared. When a new leader is elected, the key
value will also be updated.

Using the `acquire` parameter is optional. This means
that if you use leader election to update a key, you must not update the key
without the acquire parameter.
