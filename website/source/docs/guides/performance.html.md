---
layout: "docs"
page_title: "Server Performance"
sidebar_current: "docs-guides-performance"
description: |-
  Consul requires different amounts of compute resources, depending on cluster size and expected workload. This guide provides guidance on choosing compute resources.
---

# Server Performance

Since Consul servers run a [consensus protocol](/docs/internals/consensus.html) to
process all write operations and are contacted on nearly all read operations, server
performance is critical for overall throughput and health of a Consul cluster. Servers
are generally I/O bound for writes because the underlying Raft log store performs a sync
to disk every time an entry is appended. Servers are generally CPU bound for reads since
reads work from a fully in-memory data store that is optimized for concurrent access.

<a name="minimum"></a>
## Minimum Server Requirements

In Consul 0.7, the default server [performance parameters](/docs/agent/options.html#performance)
were tuned to allow Consul to run reliably (but relatively slowly) on a server cluster of three
[AWS t2.micro](https://aws.amazon.com/ec2/instance-types/) instances. These thresholds
were determined empirically using a leader instance that was under sufficient read, write,
and network load to cause it to permanently be at zero CPU credits, forcing it to the baseline
performance mode for that instance type. Real-world workloads typically have more bursts of
activity, so this is a conservative and pessimistic tuning strategy.

This default was chosen based on feedback from users, many of whom wanted a low cost way
to run small production or development clusters with low cost compute resources, at the
expense of some performance in leader failure detection and leader election times.

The default performance configuration is equivalent to this:

```javascript
{
  "performance": {
    "raft_multiplier": 5
  }
}
```

<a name="production"></a>
## Production Server Requirements

When running Consul 0.7 and later in production, it is recommended to configure the server
[performance parameters](/docs/agent/options.html#performance) back to Consul's original
high-performance settings. This will let Consul servers detect a failed leader and complete
leader elections much more quickly than the default configuration which extends key Raft
timeouts by a factor of 5, so it can be quite slow during these events.

The high performance configuration is simple and looks like this:

```javascript
{
  "performance": {
    "raft_multiplier": 1
  }
}
```

This value must take into account the network latency between the servers and the read/write load on the servers.

The value of `raft_multiplier` is a scaling factor and directly affects the following parameters:

|Param|Value||
|-----|----:|-:|
|HeartbeatTimeout|1000ms|default|
|ElectionTimeout|1000ms|default|
|LeaderLeaseTimeout|500ms|default|

So a scaling factor of `5` (i.e. `raft_multiplier: 5`) updates the following values:

|Param|Value|Calculation|
|-----|----:|-:|
|HeartbeatTimeout|5000ms|5 x 1000ms|
|ElectionTimeout|5000ms|5 x 1000ms|
|LeaderLeaseTimeout|2500ms|5 x 500ms|

~> **NOTE** Wide networks with more latency will perform better with larger values of `raft_multiplier`.

The trade off is between leader stability and time to recover from an actual leader failure. A short multiplier minimizes failure detection and election time but may be triggered frequently in high latency situations. This can cause constant leadership churn and associated unavailability. A high multiplier reduces the chances that spurious failures will cause leadership churn but it does this at the expense of taking longer to detect real failures and thus takes longer to restore cluster availability.

Leadership instability can also be caused by under-provisioned CPU resources and is more likely in environments where CPU cycles are shared with other workloads. In order for a server to remain the leader, it must send frequent heartbeat messages to all other servers every few hundred milliseconds. If some number of these are missing or late due to the leader not having sufficient CPU to send them on time, the other servers will detect it as failed and hold a new election.

It's best to benchmark with a realistic workload when choosing a production server for Consul.
Here are some general recommendations:

* Consul will make use of multiple cores, and at least 2 cores are recommended.

* <a name="last-contact"></a>Spurious leader elections can be caused by networking issues between
the servers or insufficient CPU resources. Users in cloud environments often bump their servers
up to the next instance class with improved networking and CPU until leader elections stabilize,
and in Consul 0.7 or later the [performance parameters](/docs/agent/options.html#performance)
configuration now gives you tools to trade off performance instead of upsizing servers. You can
use the [`consul.raft.leader.lastContact` telemetry](/docs/agent/telemetry.html#last-contact)
to observe how the Raft timing is performing and guide the decision to de-tune Raft performance
or add more powerful servers.

* For DNS-heavy workloads, configuring all Consul agents in a cluster with the
[`allow_stale`](/docs/agent/options.html#allow_stale) configuration option will allow reads to
scale across all Consul servers, not just the leader. Consul 0.7 and later enables stale reads
for DNS by default. See [Stale Reads](/docs/guides/dns-cache.html#stale) in the
[DNS Caching](/docs/guides/dns-cache.html) guide for more details. It's also good to set
reasonable, non-zero [DNS TTL values](/docs/guides/dns-cache.html#ttl) if your clients will
respect them.

* In other applications that perform high volumes of reads against Consul, consider using the
[stale consistency mode](/api/index.html#consistency) available to allow reads to scale
across all the servers and not just be forwarded to the leader.

* In Consul 0.9.3 and later, a new [`limits`](/docs/agent/options.html#limits) configuration is
available on Consul clients to limit the RPC request rate they are allowed to make against the
Consul servers. After hitting the limit, requests will start to return rate limit errors until
time has passed and more requests are allowed. Configuring this across the cluster can help with
enforcing a max desired application load level on the servers, and can help mitigate abusive
applications.

## Memory Requirements

Consul server agents operate on a working set of data comprised of key/value
entries, the service catalog, prepared queries, access control lists, and
sessions in memory. These data are persisted through Raft to disk in the form
of a snapshot and log of changes since the previous snapshot for durability.

When planning for memory requirements, you should typically allocate
enough RAM for your server agents to contain between 2 to 4 times the working
set size. You can determine the working set size by noting the value of
`consul.runtime.alloc_bytes` in the [Telemetry data](/docs/agent/telemetry.html).

> NOTE: Consul is not designed to serve as a general purpose database, and you
> should keep this in mind when choosing what data are populated to the
> key/value store.

## Read/Write Tuning

Consul is write limited by disk I/O and read limited by CPU. Memory requirements will be dependent on the total size of KV pairs stored and should be sized according to that data (as should the hard drive storage). The limit on a key’s value size is `512KB`.

-> Consul is write limited by disk I/O and read limited by CPU.

For **write-heavy** workloads, the total RAM available for overhead must approximately be equal to

    RAM NEEDED = number of keys * average key size * 2-3x

Since writes must be synced to disk (persistent storage) on a quorum of servers before they are committed, deploying a disk with high write throughput (or an SSD) will enhance performance on the write side. ([Documentation](/docs/agent/options.html#\_data\_dir))

For a **read-heavy** workload, configure all Consul server agents with the `allow_stale` DNS option, or query the API with the `stale` [consistency mode](/api/index.html#consistency-modes). By default, all queries made to the server are RPC forwarded to and serviced by the leader. By enabling stale reads, any server will respond to any query, thereby reducing overhead on the leader. Typically, the stale response is `100ms` or less from consistent mode but it drastically improves performance and reduces latency under high load.

If the leader server is out of memory or the disk is full, the server eventually stops responding, loses its election and cannot move past its last commit time. However, by configuring `max_stale` and setting it to a large value, Consul will continue to respond to queries during such outage scenarios. ([max_stale documentation](/docs/agent/options.html#max_stale)).

It should be noted that `stale` is not appropriate for coordination where strong consistency is important (i.e. locking or application leader election). For critical cases, the optional `consistent` API query mode is required for true linearizability; the trade off is that this turns a read into a full quorum write so requires more resources and takes longer.

**Read-heavy** clusters may take advantage of the [enhanced reading](/docs/enterprise/read-scale/index.html) feature (Enterprise) for better scalability. This feature allows additional servers to be introduced as non-voters. Being a non-voter, the server will still participate in data replication, but it will not block the leader from committing log entries.

Consul’s agents use network sockets for communicating with the other nodes (gossip) and with the server agent. In addition, file descriptors are also opened for watch handlers, health checks, and log files. For a **write heavy** cluster, the `ulimit` size must be increased from the default  value (`1024`) to prevent the leader from running out of file descriptors.

To prevent any CPU spikes from a misconfigured client, RPC requests to the server should be [rate limited](/docs/agent/options.html#limits)

~> **NOTE** Rate limiting is configured on the client agent only.

In addition, two [performance indicators](/docs/agent/telemetry.html) &mdash; `consul.runtime.alloc_bytes` and `consul.runtime.heap_objects` &mdash; can help diagnose if the current sizing is not adequately meeting the load.
