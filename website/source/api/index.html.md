---
layout: api
page_title: HTTP API
sidebar_current: api-overview
description: |-
  Consul exposes a RESTful HTTP API to control almost every aspect of the
  Consul agent.
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can perform basic
CRUD operations on nodes, services, checks, configuration, and more.

## Version Prefix

All API routes are prefixed with `/v1/`. This documentation is only for the v1 API.

## ACLs

Several endpoints in Consul use or require ACL tokens to operate. An agent
can be configured to use a default token in requests using the `acl_token`
configuration option. However, the token can also be specified per-request
by using the `X-Consul-Token` request header or Bearer header in Authorization
header or the `token` query string parameter. The request header takes
precedence over the default token, and the query string parameter takes
precedence over everything.

For more details about ACLs, please see the [ACL Guide](/docs/guides/acl.html).

## Authentication

When authentication is enabled, a Consul token should be provided to API
requests using the `X-Consul-Token` header. This reduces the probability of the
token accidentally getting logged or exposed. When using authentication,
clients should communicate via TLS.

Here is an example using `curl`:

```text
$ curl \
    --header "X-Consul-Token: abcd1234" \
    http://127.0.0.1:8500/v1/agent/members
```

Previously this was provided via a `?token=` query parameter. This functionality
exists on many endpoints for backwards compatibility, but its use is **highly
discouraged**, since it can show up in access logs as part of the URL.

## Blocking Queries

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

### Implementation Details

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

### Hash-based Blocking Queries

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

## Consistency Modes

Most of the read query endpoints support multiple levels of consistency. Since
no policy will suit all clients' needs, these consistency modes allow the user
to have the ultimate say in how to balance the trade-offs inherent in a
distributed system.

The three read modes are:

- `default` - If not specified, the default is strongly consistent in almost all
  cases. However, there is a small window in which a new leader may be elected
  during which the old leader may service stale values. The trade-off is fast
  reads but potentially stale values. The condition resulting in stale reads is
  hard to trigger, and most clients should not need to worry about this case.
  Also, note that this race condition only applies to reads, not writes.

- `consistent` - This mode is strongly consistent without caveats. It requires
  that a leader verify with a quorum of peers that it is still leader. This
  introduces an additional round-trip to all server nodes. The trade-off is
  increased latency due to an extra round trip. Most clients should not use this
  unless they cannot tolerate a stale read.

- `stale` - This mode allows any server to service the read regardless of
  whether it is the leader. This means reads can be arbitrarily stale; however,
  results are generally consistent to within 50 milliseconds of the leader. The
  trade-off is very fast and scalable reads with a higher likelihood of stale
  values. Since this mode allows reads without a leader, a cluster that is
  unavailable will still be able to respond to queries.

To switch these modes, either the `stale` or `consistent` query parameters
should be provided on requests. It is an error to provide both.

Note that some endpoints support a `cached` parameter which has some of the same
semantics as `stale` but different trade offs. This behaviour is described in
[Agent Caching](#agent-caching).

To support bounding the acceptable staleness of data, responses provide the
`X-Consul-LastContact` header containing the time in milliseconds that a server
was last contacted by the leader node. The `X-Consul-KnownLeader` header also
indicates if there is a known leader. These can be used by clients to gauge the
staleness of a result and take appropriate action.

## Agent Caching

Some read endpoints support agent caching. They are clearly marked in the
documentation. Agent caching can take two forms, [`simple`](#simple-caching) or 
[`background refresh`](#blocking-refresh-caching) depending on the endpoint's 
semantics. The documentation for each endpoint clearly identify which if any 
form of caching is supported. The details for each are described below.

Where supported, caching can be enabled though the `?cached` parameter.
Combining `?cached` with `?consistent` is an error.

### Simple Caching

Endpoints supporting simple caching may return a result directly from the local
agent's cache without a round trip to the servers. By default the agent caches
results for a relatively long time (3 days) such that it can still return a
result even if the servers are unavailable for an extended period to enable
"fail static" semantics.

That means that with no other arguments, `?cached` queries might receive a
response which is days old. To request better freshness, the HTTP
`Cache-Control` header may be set with a directive like `max-age=<seconds>`. In
this case the agent will attempt to re-fetch the result from the servers if the
cached value is older than the given `max-age`. If the servers can't be reached
a 500 is returned as normal.

To allow clients to maintain fresh results in normal operation but allow stale
ones if the servers are unavailable, the `stale-if-error=<seconds>` directive
may be additionally provided in the `Cache-Control` header. This will return the
cached value anyway even it it's older than `max-age` (provided it's not older
than `stale-if-error`) rather than a 500. It must be provided along with a
`max-age` or `must-revalidate`. The `Age` response header, if larger than
`max-age` can be used to determine if the server was unreachable and a cached
version returned instead.

For example, assuming there is a cached response that is 65 seconds old, and
that the servers are currently unavailable, `Cache-Control: max-age=30` will
result in a 500 error, while `Cache-Control: max-age=30 stale-if-error=259200`
will result in the cached response being returned.

A request setting either `max-age=0` or `must-revalidate` directives will cause
the agent to always re-fetch the response from servers. Either can be combined
with `stale-if-error=<seconds>` to ensure fresh results when the servers are
available, but falling back to cached results if the request to the servers
fails.

Requests that do not use `?cached` currently bypass the cache entirely so the
cached response returned might be more stale than the last uncached response
returned on the same agent. If this causes problems, it is possible to make
requests using `?cached` and setting `Cache-Control: must-revalidate` to have
always-fresh results yet keeping the cache populated with the most recent
result.

In all cases the HTTP `X-Cache` header is always set in the response to either
`HIT` or `MISS` indicating whether the response was served from cache or not.

For cache hits, the HTTP `Age` header is always set in the response to indicate
how many seconds since that response was fetched from the servers.

### Background Refresh Caching

Endpoints supporting background refresh caching may return a result directly
from the local agent's cache without a round trip to the severs. The first fetch
that is a miss will cause an initial fetch from the servers, but will also
trigger the agent to begin a background blocking query that watches for any
changes to that result and updates the cached value if changes occur.

Following requests will _always_ be a cache hit until there has been no request
for the resource for the TTL (which is typically 3 days).

Clients can perform blocking queries against the local agent which will be
served from the cache. This allows multiple clients to watch the same resource
locally while only a single blocking watch for that resource will be made to the
servers from a given client agent.

HTTP `Cache-Control` headers are ignored in this mode since the cache is being
actively updated and has different semantics to a typical passive cache.

In all cases the HTTP `X-Cache` header is always set in the response to either
`HIT` or `MISS` indicating whether the response was served from cache or not.

For cache hits, the HTTP `Age` header is always set in the response to indicate
how many seconds since that response was fetched from the servers. As long as
the local agent has an active connection to the servers, the age will always be
`0` since the value is up-to-date. If the agent get's disconnected, the cached
result is still returned but with an `Age` that indicates how many seconds have
elapsed since the local agent got disconnected from the servers, during which
time updates to the result might have been missed.

## Formatted JSON Output

By default, the output of all HTTP API requests is minimized JSON. If the client
passes `pretty` on the query string, formatted JSON will be returned.

## HTTP Methods

Consul's API aims to be RESTful, although there are some exceptions. The API
responds to the standard HTTP verbs GET, PUT, and DELETE. Each API method will
clearly document the verb(s) it responds to and the generated response. The same
path with different verbs may trigger different behavior. For example:

```text
PUT /v1/kv/foo
GET /v1/kv/foo
```

Even though these share a path, the `PUT` operation creates a new key whereas
the `GET` operation reads an existing key.

Here is the same example using `curl`:

```shell
$ curl \
    --request PUT \
    --data 'hello consul' \
    http://127.0.0.1:8500/v1/kv/foo
```

## Translated Addresses

Consul 0.7 added the ability to translate addresses in HTTP response based on
the configuration setting for
[`translate_wan_addrs`](/docs/agent/options.html#translate_wan_addrs). In order
to allow clients to know if address translation is in effect, the
`X-Consul-Translate-Addresses` header will be added if translation is enabled,
and will have a value of `true`. If translation is not enabled then this header
will not be present.

## UUID Format

UUID-format identifiers generated by the Consul API use the
[hashicorp/go-uuid](https://github.com/hashicorp/go-uuid) library.

These UUID-format strings are generated using high quality, purely random bytes.
It is not intended to be RFC compliant, merely to use a well-understood string
representation of a 128-bit value.
