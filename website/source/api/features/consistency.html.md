---
layout: api
page_title: Consistency Modes
sidebar_current: api-features-consistency
description: |-
   Most of the read query endpoints support multiple levels of consistency. Since no policy will suit all clients' needs, these consistency modes allow the user to have the ultimate say in how to balance the trade-offs inherent in a distributed system.
---

# Consistency Modes

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
semantics as `stale` but different trade offs. This behavior is described in
[agent caching feature documentation](/api/features/caching.html).

To support bounding the acceptable staleness of data, responses provide the
`X-Consul-LastContact` header containing the time in milliseconds that a server
was last contacted by the leader node. The `X-Consul-KnownLeader` header also
indicates if there is a known leader. These can be used by clients to gauge the
staleness of a result and take appropriate action.