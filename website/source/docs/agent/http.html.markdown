---
layout: "docs"
page_title: "HTTP API"
sidebar_current: "docs-agent-http"
description: |-
  The main interface to Consul is a RESTful HTTP API. The API can be used to perform CRUD operations on nodes, services, checks, configuration, and more. The endpoints are versioned to enable changes without breaking backwards compatibility.
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can be used to perform CRUD
operations on nodes, services, checks, configuration, and more. The endpoints are versioned
to enable changes without breaking backwards compatibility.

Each endpoint manages a different aspect of Consul:

* [acl](http/acl.html) - Access Control Lists
* [agent](http/agent.html) - Consul Agent
* [catalog](http/catalog.html) - Nodes and Services
* [coordinate](http/coordinate.html) - Network Coordinates
* [event](http/event.html) - User Events
* [health](http/health.html) - Health Checks
* [kv](http/kv.html) - Key/Value Store
* [query](http/query.html) - Prepared Queries
* [session](http/session.html) - Sessions
* [status](http/status.html) - Consul System Status

Each of these is documented in detail at the links above. Consul also has a number
of internal APIs which are purposely undocumented and subject to change.

## Blocking Queries

Certain endpoints support a feature called a "blocking query." A blocking query
is used to wait for a potential change using long polling.

Not all endpoints support blocking, but those that do are clearly designated in the
documentation.  Any endpoint that supports blocking will also set the HTTP header
`X-Consul-Index`, a unique identifier representing the current state of the
requested resource.  On subsequent requests for this resource, the client can set the `index`
query string parameter to the value of `X-Consul-Index`, indicating that the client wishes
to wait for any changes subsequent to that index.

In addition to `index`, endpoints that support blocking will also honor a `wait`
parameter specifying a maximum duration for the blocking request. This is limited to
10 minutes. If not set, the wait time defaults to 5 minutes. This value can be specified
in the form of "10s" or "5m" (i.e., 10 seconds or 5 minutes, respectively).

A critical note is that the return of a blocking request is **no guarantee** of a change. It
is possible that the timeout was reached or that there was an idempotent write that does
not affect the result of the query.

## <a id="consistency"></a>Consistency Modes

Most of the read query endpoints support multiple levels of consistency. Since no policy will
suit all clients' needs, these consistency modes allow the user to have the ultimate say in
how to balance the trade-offs inherent in a distributed system.

The three read modes are:

* default - If not specified, the default is strongly consistent in almost all cases. However,
  there is a small window in which a new leader may be elected during which the old leader may
  service stale values. The trade-off is fast reads but potentially stale values. The condition
  resulting in stale reads is hard to trigger, and most clients should not need to worry about
  this case.  Also, note that this race condition only applies to reads, not writes.

* consistent - This mode is strongly consistent without caveats. It requires
  that a leader verify with a quorum of peers that it is still leader. This
  introduces an additional round-trip to all server nodes. The trade-off is
  increased latency due to an extra round trip. Most clients should not use this
  unless they cannot tolerate a stale read.

* stale - This mode allows any server to service the read regardless of whether
  it is the leader. This means reads can be arbitrarily stale; however, results are generally
  consistent to within 50 milliseconds of the leader. The trade-off is very fast and
  scalable reads with a higher likelihood of stale values. Since this mode allows reads without
  a leader, a cluster that is unavailable will still be able to respond to queries.

To switch these modes, either the `stale` or `consistent` query parameters
should be provided on requests. It is an error to provide both.

To support bounding the acceptable staleness of data, responses provide the `X-Consul-LastContact`
header containing the time in milliseconds that a server was last contacted by the leader node.
The `X-Consul-KnownLeader` header also indicates if there is a known leader. These can be used
by clients to gauge the staleness of a result and take appropriate action.

## Formatted JSON Output

By default, the output of all HTTP API requests is minimized JSON.  If the client passes `pretty`
on the query string, formatted JSON will be returned.

## ACLs

Several endpoints in Consul use or require ACL tokens to operate. An agent
can be configured to use a default token in requests using the `acl_token`
configuration option. However, the token can also be specified per-request
by using the `X-Consul-Token` request header or the `token` querystring
parameter. The request header takes precedence over the default token, and
the querystring parameter takes precedence over everything.
