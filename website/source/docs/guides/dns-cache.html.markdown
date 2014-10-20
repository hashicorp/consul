---
layout: "docs"
page_title: "DNS Caching"
sidebar_current: "docs-guides-dns-cache"
description: |-
  One of the main interfaces to Consul is DNS. Using DNS is a simple way integrate Consul into an existing infrastructure without any high-touch integration.
---

# DNS Caching

One of the main interfaces to Consul is DNS. Using DNS is a simple way
integrate Consul into an existing infrastructure without any high-touch
integration.

By default, Consul serves all DNS results with a 0 TTL value. This prevents
any caching. The advantage of this is that each DNS lookup is always re-evaluated
and the most timely information is served. However this adds a latency hit
for each lookup and can potentially exhaust the query throughput of a cluster.

For this reason, Consul provides a number of tuning parameters that can
be used to customize how DNS queries are handled.

## Stale Reads

Stale reads can be used to reduce latency and increase the throughput
of DNS queries. By default, all reads are serviced by a [single leader node](/docs/internals/consensus.html).
These reads are strongly consistent but are limited by the throughput
of a single node. Doing a stale read allows any Consul server to
service a query, but non-leader nodes may return data that is potentially
out-of-date. By allowing data to be slightly stale, we get horizontal
read scalability. Now any Consul server can service the request, so we
increase throughput by the number of servers in a cluster.

The [settings](/docs/agent/options.html) used to control stale reads
are `dns_config.allow_stale` which must be set to enable stale reads,
and `dns_config.max_stale` which limits how stale results are allowed to
be.

By default, `allow_stale` is disabled meaning no stale results may be served.
The default for `max_stale` is 5 seconds. This means that if `allow_stale` is
enabled, we will use data from any Consul server that is within 5 seconds
of the leader.

## TTL Values

TTL values can be set to allow DNS results to be cached downstream
of Consul which can be used to reduce the number of lookups and to amortize
the latency of doing a DNS lookup. By default, all TTLs are zero,
preventing any caching.

To enable caching of node lookups (e.g. "foo.node.consul"), we can set
the `dns_config.node_ttl` value. This can be set to "10s" for example,
and all node lookups will serve results with a 10 second TTL.

Service TTLs can be specified at a more fine grain level. You can set
a TTL on a per-service level, and additionally a wildcard can be specified
that matches if there is no specific service TTL provided.

This is specified using the `dns_config.service_ttl` map. The "*" service
is the wildcard service. For example, if we specify:

```javascript
{
  "dns_config": {
    "service_ttl": {
      "*": "5s",
      "web": "30s"
    }
  }
}
```

This sets all lookups to "web.service.consul" to use a 30 second TTL,
while lookups to "db.service.consul" or "api.service.consul" will use the
5 second TTL from the wildcard.
