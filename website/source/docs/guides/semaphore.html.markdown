---
layout: "docs"
page_title: "Semaphore"
sidebar_current: "docs-guides-semaphore"
description: |-
  This guide demonstrates how to implement a distributed semaphore using the Consul Key/Value store.
---

# Semaphore

The goal of this guide is to cover how to build a client-side semaphore using Consul.
This is useful when you want to coordinate many services while restricting access to
certain resources.

If you only need mutual exclusion or leader election, [this guide](/docs/guides/leader-election.html)
provides a simpler algorithm that can be used instead.

There are a number of ways that a semaphore can be built, so our goal is not to
cover all the possible methods. Instead, we will focus on using Consul's support for
[sessions](/docs/internals/sessions.html), which allow us to build a system that can
gracefully handle failures.

Note that JSON output in this guide has been pretty-printed for easier
reading.  Actual values returned from the API will not be formatted.

## Contending Nodes

The primary flow is for nodes who are attempting to acquire a slot in the semaphore.
All nodes that are participating should agree on a given prefix being used to coordinate,
a single lock key, and a limit of slot holders. A good choice is simply:

```text
service/<service name>/lock/
```

We will refer to this as just `<prefix>` for simplicity.

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

Next, we create a contender entry. Each contender makes an entry that is tied
to a session. This is done so that if a contender is holding a slot and fails
it can be detected by the other contenders. Optionally, an opaque value
can be associated with the contender via a `<body>`.

Create the contender key by doing an `acquire` on `<prefix>/<session>` by doing a `PUT`.
This is something like:

```text
curl -X PUT -d <body> http://localhost:8500/v1/kv/<prefix>/<session>?acquire=<session>
 ```

Where `<session>` is the ID returned by the call to `/v1/session/create`.

This will either return `true` or `false`. If `true` is returned, the contender
entry has been created.  If `false` is returned, the contender node was not created and
likely this indicates a session invalidation.

The next step is to use a single key to coordinate which holders are currently
reserving a slot. A good choice is simply `<prefix>/.lock`. We will refer to this
special coordinating key as `<lock>`. The current state of the semaphore is read by
doing a `GET` on the entire `<prefix>`:

```text
curl http://localhost:8500/v1/kv/<prefix>?recurse
 ```

Within the list of the entries, we should find the `<lock>`. That entry should hold
both the slot limit and the current holders. A simple JSON body like the following works:

```text
{
    "Limit": 3,
    "Holders": {
        "4ca8e74b-6350-7587-addf-a18084928f3c": true,
        "adf4238a-882b-9ddc-4a9d-5b6758e4159e": true
    }
}
```

When the `<lock>` is read, we can verify the remote `Limit` agrees with the local value. This
is used to detect a potential conflict. The next step is to determine which of the current
slot holders are still alive. As part of the results of the `GET`, we have all the contender
entries. By scanning those entries, we create a set of all the `Session` values. Any of the
`Holders` that are not in that set are pruned. In effect, we are creating a set of live contenders
based on the list results, and doing a set difference with the `Holders` to detect and prune
any potentially failed holders.

If the number of holders (after pruning) is less than the limit, a contender attempts acquisition
by adding its own session to the `Holders` and doing a Check-And-Set update of the `<lock>`. This
performs an optimistic update.

This is done by:

```text
curl -X PUT -d <Updated Lock> http://localhost:8500/v1/kv/<lock>?cas=<lock-modify-index>
 ```

If this suceeds with `true` the contender now holds a slot in the semaphore. If this fails
with `false`, then likely there was a race with another contender to acquire the slot.
Both code paths now go into an idle waiting state. In this state, we watch for changes
on `<prefix>`. This is because a slot may be released, a node may fail, etc.
Slot holders must also watch for changes since the slot may be released by an operator,
or automatically released due to a false positive in the failure detector.

Watching for changes is done by doing a blocking query against `<prefix>`. If a contender
holds a slot, then on any change the `<lock>` should be re-checked to ensure the slot is
still held.  If no slot is held, then the same acquisition logic is triggered to check
and potentially re-attempt acquisition. This allows a contender to steal the slot from
a failed contender or one that has voluntarily released its slot.

If a slot holder ever wishes to release voluntarily, this should be done by doing a
Check-And-Set operation against `<lock>` to remove its session from the `Holders`. Once
that is done, the contender entry at `<prefix>/<session>` should be delete. Finally the
session should be destroyed.

