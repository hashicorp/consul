---
layout: "docs"
page_title: "Semaphore"
sidebar_current: "docs-guides-semaphore"
description: |-
  This guide demonstrates how to implement a distributed semaphore using the Consul KV store.
---

# Semaphore

A distributed semaphore can be useful when you want to coordinate many services, while
restricting access to certain resources. In this guide we will focus on using Consul's support for
sessions and Consul KV to build a distributed
semaphore. Note, there are a number of ways that a semaphore can be built, we will not cover all the possible methods in this guide. 

To complete this guide successfully, you should have familiarity with 
[Consul KV](/docs/agent/kv.html) and Consul [sessions](/docs/internals/sessions.html). 

~>  If you only need mutual exclusion or leader election,
[this guide](/docs/guides/leader-election.html)
provides a simpler algorithm that can be used instead.


## Contending Nodes in the Semaphore

Let's imagine we have a set of nodes who are attempting to acquire a slot in the
semaphore. All nodes that are participating should agree on three decisions

- the prefix in the KV store used to coordinate.
- a single key to use as a lock.
- a limit on the number of slot holders.

### Session

The first step is for each contending node to create a session. Sessions allow us to build a system that
can gracefully handle failures. 

This is done using the
[Session HTTP API](/api/session.html#session_create).

```sh
curl  -X PUT -d '{"Name": "db-semaphore"}' \
  http://localhost:8500/v1/session/create
 ```

This will return a JSON object contain the session ID.

```json
{
  "ID": "4ca8e74b-6350-7587-addf-a18084928f3c"
}
```

->  **Note:** Sessions by default only make use of the gossip failure detector. That is, the session is considered held by a node as long as the default Serf health check has not declared the node unhealthy. Additional checks can be specified at session creation if desired.

### KV Entry for Node Locks

Next, we create a lock contender entry. Each contender creates a kv entry that is tied
to a session. This is done so that if a contender is holding a slot and fails, its session
is detached from the key, which can then be detected by the other contenders.

Create the contender key by doing an `acquire` on `<prefix>/<session>` via `PUT`.

```sh
curl -X PUT -d <body> http://localhost:8500/v1/kv/<prefix>/<session>?acquire=<session>
 ```

`body` can be used to associate a meaningful value with the contender, such as its node’s name. 
This body is opaque to Consul but can be useful for human operators.

The `<session>` value is the ID returned by the call to
[`/v1/session/create`](/api/session.html#session_create).

The call will either return `true` or `false`. If `true`, the contender entry has been
created. If `false`, the contender node was not created; it's likely that this indicates
a session invalidation.

### Single Key for Coordination

The next step is to create a single key to coordinate which holders are currently
reserving a slot. A good choice for this lock key is simply `<prefix>/.lock`. We will
refer to this special coordinating key as `<lock>`.

```sh
curl -X PUT -d <body> http://localhost:8500/v1/kv/<lock>?cas=0
```

Since the lock is being created, a `cas` index of 0 is used so that the key is only put if it does not exist.

The `body` of the request should contain both the intended slot limit for the semaphore and the session ids
of the current holders (initially only of the creator). A simple JSON body like the following works.

```json
{
    "Limit": 2,
    "Holders": [
      "<session>"
    ]
}
```

## Semaphore Management

The current state of the semaphore is read by doing a `GET` on the entire `<prefix>`.

```sh
curl http://localhost:8500/v1/kv/<prefix>?recurse
```

Within the list of the entries, we should find two keys: the `<lock>` and the
contender key ‘<prefix>/<session>’. 

```json
[
  {
    "LockIndex": 0,
    "Key": "<lock>",
    "Flags": 0,
    "Value": "eyJMaW1pdCI6IDIsIkhvbGRlcnMiOlsiPHNlc3Npb24+Il19",
    "Session": "",
    "CreateIndex": 898,
    "ModifyIndex": 901
  },
  {
    "LockIndex": 1,
    "Key": "<prefix>/<session>",
    "Flags": 0,
    "Value": null,
    "Session": "<session>",
    "CreateIndex": 897,
    "ModifyIndex": 897
  }
]
```
Note that the `Value` we embedded into `<lock>` is Base64 encoded when returned by the API.

When the `<lock>` is read and its `Value` is decoded, we can verify the `Limit` agrees with the `Holders` count. 
This is used to detect a potential conflict. The next step is to determine which of the current
slot holders are still alive. As part of the results of the `GET`, we also have all the contender
entries. By scanning those entries, we create a set of all the `Session` values. Any of the
`Holders` that are not in that set are pruned. In effect, we are creating a set of live contenders
based on the list results and doing a set difference with the `Holders` to detect and prune
any potentially failed holders. In this example `<session>` is present in `Holders` and 
is attached to the key `<prefix>/<session>`, so no pruning is required.

If the number of holders after pruning is less than the limit, a contender attempts acquisition
by adding its own session to the `Holders` list and doing a Check-And-Set update of the `<lock>`. 
This performs an optimistic update.

This is done with:

```sh
curl -X PUT -d <Updated Lock Body> http://localhost:8500/v1/kv/<lock>?cas=<lock-modify-index>
```
`lock-modify-index` is the latest `ModifyIndex` value known for `<lock>`, 901 in this example.

If this request succeeds with `true`, the contender now holds a slot in the semaphore. 
If this fails with `false`, then likely there was a race with another contender to acquire the slot.

To re-attempt the acquisition, we watch for changes on `<prefix>`. This is because a slot
may be released, a node may fail, etc. Watching for changes is done via a blocking query
against `/kv/<prefix>?recurse`. 

Slot holders **must** continuously watch for changes to `<prefix>` since their slot can be 
released by an operator or automatically released due to a false positive in the failure detector. 
On changes to `<prefix>` the lock’s `Holders` list must be re-checked to ensure the slot
is still held. Additionally, if the watch fails to connect the slot should be considered lost. 

This semaphore system is purely *advisory*. Therefore it is up to the client to verify
that a slot is held before (and during) execution of some critical operation.

Lastly, if a slot holder ever wishes to release its slot voluntarily, it should be done by doing a
Check-And-Set operation against `<lock>` to remove its session from the `Holders` object.
Once that is done, both its contender key `<prefix>/<session>` and session should be deleted.

## Summary

In this guide we created a distributed semaphore using Consul KV and Consul sessions. We also learned how to manage the newly created semaphore. 
