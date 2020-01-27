---
layout: api
page_title: Blocking Queries
sidebar_current: api-features-blocking
description: |-
   Many endpoints in Consul support a feature known as "blocking queries". A
   blocking query is used to wait for a potential change using long polling.
---


# Blocking Queries

Many endpoints in Consul support a feature known as "blocking queries". A
blocking query is used to wait for a potential change using long polling. Not
all endpoints support blocking, but each endpoint uniquely documents its support
for blocking queries in the documentation.

Endpoints that support blocking queries return an HTTP header named
`X-Consul-Index`. This is a unique identifier representing the current state of
the requested resource.

On subsequent requests for this resource, the client can set the `index` query
string parameter to the value of `X-Consul-Index`, indicating that the client
wishes to wait for any changes subsequent to that index.

When this is provided, the HTTP request will "hang" until a change in the system
occurs, or the maximum timeout is reached. A critical note is that the return of
a blocking request is **no guarantee** of a change. It is possible that the
timeout was reached or that there was an idempotent write that does not affect
the result of the query.

In addition to `index`, endpoints that support blocking will also honor a `wait`
parameter specifying a maximum duration for the blocking request. This is
limited to 10 minutes. If not set, the wait time defaults to 5 minutes. This
value can be specified in the form of "10s" or "5m" (i.e., 10 seconds or 5
minutes, respectively). A small random amount of additional wait time is added
to the supplied maximum `wait` time to spread out the wake up time of any
concurrent requests. This adds up to `wait / 16` additional time to the maximum
duration.

## Implementation Details

While the mechanism is relatively simple to work with, there are a few edge
cases that must be handled correctly.

 * **Reset the index if it goes backwards**. While indexes in general are
   monotonically increasing(i.e. they should only ever increase as time passes),
   there are several real-world scenarios in
   which they can go backwards for a given query. Implementations must check
   to see if a returned index is lower than the previous value,
   and if it is, should reset index to `0` - effectively restarting their blocking loop.
   Failure to do so may cause the client to miss future updates for an unbounded
   time, or to use an invalid index value that causes no blocking and increases
   load on the servers. Cases where this can occur include:
   * If a raft snapshot is restored on the servers with older version of the data.
   * KV list operations where an item with the highest index is removed.
   * A Consul upgrade changes the way watches work to optimize them with more
   granular indexes.

 * **Sanity check index is greater than zero**. After the initial request (or a
   reset as above) the `X-Consul-Index` returned _should_ always be greater than zero. It
   is a bug in Consul if it is not, however this has happened a few times and can
   still be triggered on some older Consul versions. It's especially bad because it
   causes blocking clients that are not aware to enter a busy loop, using excessive
   client CPU and causing high load on servers. It is _always_ safe to use an
   index of `1` to wait for updates when the data being requested doesn't exist
   yet, so clients _should_ sanity check that their index is at least 1 after
   each blocking response is handled to be sure they actually block on the next
   request.

 * **Rate limit**. The blocking query mechanism is reasonably efficient when updates
   are relatively rare (order of tens of seconds to minutes between updates). In cases
   where a result gets updated very fast however - possibly during an outage or incident
   with a badly behaved client - blocking query loops degrade into busy loops that
   consume excessive client CPU and cause high server load. While it's possible to just add a sleep
   to every iteration of the loop, this is **not** recommended since it causes update
   delivery to be delayed in the happy case, and it can exacerbate the problem since
   it increases the chance that the index has changed on the next request. Clients
   _should_ instead rate limit the loop so that in the happy case they proceed without
   waiting, but when values start to churn quickly they degrade into polling at a
   reasonable rate (say every 15 seconds). Ideally this is done with an algorithm that
   allows a couple of quick successive deliveries before it starts to limit rate - a
   [token bucket](https://en.wikipedia.org/wiki/Token_bucket) with burst of 2 is a simple
   way to achieve this.

## Hash-based Blocking Queries

A limited number of agent endpoints also support blocking however because the
state is local to the agent and not managed with a consistent raft index, their
blocking mechanism is different.

Since there is no monotonically increasing index, each response instead contains
a header `X-Consul-ContentHash` which is an opaque hash digest generated by
hashing over all fields in the response that are relevant.

Subsequent requests may be sent with a query parameter `hash=<value>` where
`value` is the last hash header value seen, and this will block until the `wait`
timeout is passed or until the local agent's state changes in such a way that
the hash would be different.

Other than the different header and query parameter names, the biggest
difference is that hash values are opaque and can't be compared to see if one
result is older or newer than another. In general hash-based blocking will not
return too early due to an idempotent update since the hash will remain the same
unless the result actually changes, however as with index-based blocking there
is no strict guarantee that clients will never observe the same result delivered
before the full timeout has elapsed.