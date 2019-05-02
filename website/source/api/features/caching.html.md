---
layout: api
page_title: Agent Caching
sidebar_current: api-features-caching
description: |-
   Some read endpoints support agent caching. They are clearly marked in the
   documentation.
---

# Agent Caching

Some read endpoints support agent caching. They are clearly marked in the
documentation. Agent caching can take two forms, [`simple`](#simple-caching) or
[`background refresh`](#background-refresh-caching) depending on the endpoint's
semantics. The documentation for each endpoint clearly identify which if any
form of caching is supported. The details for each are described below.

Where supported, caching can be enabled though the `?cached` parameter.
Combining `?cached` with `?consistent` is an error.

## Simple Caching

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

## Background Refresh Caching

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
