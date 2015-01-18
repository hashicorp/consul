---
layout: "docs"
page_title: "HTTP API"
sidebar_current: "docs-agent-http"
description: |-
  The main interface to Consul is a RESTful HTTP API. The API can be used for CRUD for nodes, services, checks, and configuration. The endpoints are versioned to enable changes without breaking backwards compatibility.
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can be
used for CRUD for nodes, services, checks, and configuration. The endpoints are
versioned to enable changes without breaking backwards compatibility.

All endpoints fall into one of several categories:

* [kv](http/kv.html) - Key/Value store
* [agent](http/agent.html) - Agent control
* [catalog](http/catalog.html) - Manages nodes and services
* [health](http/health.html) - Manages health checks
* [session](http/session.html) - Session manipulation
* [acl](http/acl.html) - ACL creations and management
* [event](http/event.html) - User Events
* [status](http/status.html) - Consul system status
* internal - Internal APIs. Purposely undocumented, subject to change.

Each of the categories and their respective endpoints are documented below.

## Blocking Queries

Certain endpoints support a feature called a "blocking query." A blocking query
is used to wait for a change to potentially take place using long polling.

Queries that support this will mention it specifically, however the use of this
feature is the same for all. If supported, the query will set an HTTP header
"X-Consul-Index". This is an opaque handle that the client will use.

To cause a query to block, the query parameters "?wait=\<interval\>&index=\<idx\>" are added
to a request. The "?wait=" query parameter limits how long the query will potentially
block for. It not set, it will default to 10 minutes. It can be specified in the form of
"10s" or "5m", which is 10 seconds or 5 minutes respectively. The "?index=" parameter is an
opaque handle, which is used by Consul to detect changes. The  "X-Consul-Index" header for a
query provides this value, and can be used to wait for changes since the query was run.

When provided, Consul blocks sending a response until there is an update that
could have cause the output to change, and thus advancing the index. A critical
note is that when the query returns there is **no guarantee** of a change. It is
possible that the timeout was reached, or that there was an idempotent write that
does not affect the result.

## Consistency Modes

Most of the read query endpoints support multiple levels of consistency.
These are to provide a tuning knob that clients can be used to find a happy
medium that best matches their needs.

The three read modes are:

* default - If not specified, this mode is used. It is strongly consistent
  in almost all cases. However, there is a small window in which an new
  leader may be elected, and the old leader may service stale values. The
  trade off is fast reads, but potentially stale values. This condition is
  hard to trigger, and most clients should not need to worry about the stale read.
  This only applies to reads, and a split-brain is not possible on writes.

* consistent - This mode is strongly consistent without caveats. It requires
  that a leader verify with a quorum of peers that it is still leader. This
  introduces an additional round-trip to all server nodes. The trade off is
  always consistent reads, but increased latency due to an extra round trip.
  Most clients should not use this unless they cannot tolerate a stale read.

* stale - This mode allows any server to service the read, regardless of if
  it is the leader. This means reads can be arbitrarily stale, but are generally
  within 50 milliseconds of the leader. The trade off is very fast and scalable
  reads but values will be stale. This mode allows reads without a leader, meaning
  a cluster that is unavailable will still be able to respond.

To switch these modes, either the "?stale" or "?consistent" query parameters
are provided. It is an error to provide both.

To support bounding how stale data is, there is an "X-Consul-LastContact"
which is the last time a server was contacted by the leader node in
milliseconds. The "X-Consul-KnownLeader" also indicates if there is a known
leader. These can be used to gauge if a stale read should be used.

## Formatted JSON Output

By default, the output of all HTTP API requests return minimized JSON with all
whitespace removed.  By adding "?pretty" to the HTTP request URL,
formatted JSON will be returned.

## ACLs

Several endpoints in Consul use or require ACL tokens to operate. An agent
can be configured to use a default token in requests using the `acl_token`
configuration option. However, the token can also be specified per-request
by using the "?token=" query parameter. This will take precedence over the
default token.
