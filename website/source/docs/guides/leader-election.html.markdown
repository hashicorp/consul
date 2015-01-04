---
layout: "docs"
page_title: "Leader Election"
sidebar_current: "docs-guides-leader"
description: |-
  The goal of this guide is to cover how to build client-side leader election using Consul. If you are interested in the leader election used internally to Consul, you want to read about the consensus protocol instead.
---

# Leader Election

The goal of this guide is to cover how to build client-side leader election using Consul.
If you are interested in the leader election used internally to Consul, you want to
read about the [consensus protocol](/docs/internals/consensus.html) instead.

There are a number of ways that leader election can be built, so our goal is not to
cover all the possible methods. Instead, we will focus on using Consul's support for
[sessions](/docs/internals/sessions.html), which allow us to build a system that can
gracefully handle failures.

Note that JSON output in this guide has been pretty-printed for easier
reading.  Actual values returned from the API will not be formatted.

## Contending Nodes

The first flow we cover is for nodes who are attempting to acquire leadership
for a given service. All nodes that are participating should agree on a given
key being used to coordinate. A good choice is simply:

```text
service/<service name>/leader
```

We will refer to this as just `<key>` for simplicity.

The first step is to create a session. This is done using the [/v1/session/create endpoint][session-api]:

[session-api]: http://www.consul.io/docs/agent/http.html#_v1_session_create

```text
curl  -X PUT -d '{"Name": "dbservice"}' \
  http://localhost:8500/v1/session/create
 ```

This will return a JSON object contain the session ID:

```text
{
  "ID": "4ca8e74b-6350-7587-addf-a18084928f3c"
}
```

The session by default makes use of only the gossip failure detector. Additional checks
can be specified if desired.

Create `<body>` to represent the local node. This value is opaque to
Consul and should contain whatever information clients require to
communicate with your application (e.g., it could be a JSON object
that contains the node's name and the application's port).

Attempt to `acquire` the `<key>` by doing a `PUT`. This is something like:

```text
curl -X PUT -d <body> http://localhost:8500/v1/kv/<key>?acquire=<session>
 ```

Where `<session>` is the ID returned by the call to
`/v1/session/create`.

This will either return `true` or `false`. If `true` is returned, the lock
has been acquired and the local node is now the leader. If `false` is returned,
some other node has acquired the lock.

All nodes now remain in an idle waiting state. In this state, we watch for changes
on `<key>`. This is because the lock may be released, the node may fail, etc.
The leader must also watch for changes since it's lock may be released by an operator,
or automatically released due to a false positive in the failure detector.

Watching for changes is done by doing a blocking query against `<key>`. If we ever
notice that the `Session` of the `<key>` is blank, then there is no leader, and we should
retry acquiring the lock. Each attempt to acquire the key should be separated by a timed
wait. This is because Consul may be enforcing a [`lock-delay`](/docs/internals/sessions.html).

If the leader ever wishes to step down voluntarily, this should be done by simply
releasing the lock:

```text
curl -X PUT http://localhost:8500/v1/kv/<key>?release=<session>
```

## Discovering a Leader

The second flow is for nodes who are attempting to discover the leader
for a given service. All nodes that are participating should agree on the key
being used to coordinate, including the contenders. This key will be referred
to as just `key`.

Clients have a very simple role, they simply read `<key>` to discover who the current
leader is:

```text
curl  http://localhost:8500/v1/kv/<key>
[
  {
    "Session": "4ca8e74b-6350-7587-addf-a18084928f3c",
    "Value": "Ym9keQ==",
    "Flags": 0,
    "Key": "<key>",
    "LockIndex": 1,
    "ModifyIndex": 29,
    "CreateIndex": 29
  }
]
```

If the key has no associated `Session`, then there is no leader.
Otherwise, the value of the key will provide all the
application-dependent information required as a base64 encoded blob in
the `Value` key.  You can query the `/v1/session/info` endpoint to get
details about the session:

```text
curl http://localhost:8500/v1/session/info/4ca8e74b-6350-7587-addf-a18084928f3c
[
  {
    "LockDelay": 1.5e+10,
    "Checks": [
      "serfHealth"
    ],
    "Node": "consul-master-bjsiobmvdij6-node-lhe5ihreel7y",
    "Name": "dbservice",
    "ID": "4ca8e74b-6350-7587-addf-a18084928f3c",
    "CreateIndex": 28
  }
]
```

Clients should also watch the key using a blocking query for any changes. If the leader
steps down, or fails, then the `Session` associated with the key will be cleared. When
a new leader is elected, the key value will also be updated.
