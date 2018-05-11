---
layout: "docs"
page_title: "Production Deployment"
sidebar_current: "docs-guides-deployment"
description: |-
  Best practice approaches for Consul production deployments.
---

# Consul Production Deployment Guide

As applications are migrated to dynamically provisioned infrastructure, scaling services and managing the communications between them becomes challenging. Consul’s service discovery capabilities provide the connectivity between dynamic applications. Consul also monitors the health of each node and its applications to ensure that only healthy service instances are discovered. Consul’s distributed runtime configuration store allows updates across global infrastructure.

The goal of this document is to recommend best practice approaches for Consul production deployments. The guide provides recommendations on system requirements, datacenter design, networking, and performance optimizations for Consul cluster based on the latest Consul (1.0.7) release.

This document assumes a basic working knowledge of Consul.

## 1.0 System Requirements

Consul server agents are responsible for maintaining the cluster state, responding to RPC queries (read operations), and for processing all write operations. Given that Consul server agents do most of the heavy lifting, server sizing is critical for the overall performance efficiency and health of the Consul cluster.

The following instance configurations are recommended.

|Size|CPU|Memory|Disk|Typical Cloud Instance Types|
|----|---|------|----|----------------------------|
|Small|2 core|8-16 GB RAM|50 GB|**AWS**: m5.large, m5.xlarge|
|||||**Azure**: Standard\_A4\_v2, Standard\_A8\_v2|
|||||**GCE**: n1-standard-8, n1-standard-16|
|Large|4-8 core|32-64+ GB RAM|100 GB|**AWS**: m5.2xlarge, m5.4xlarge|
|||||**Azure**: Standard\_D4\_v3, Standard\_D5\_v3|
|||||**GCE**: n1-standard-32, n1-standard-64|

The **small size** instance configuration is appropriate for most initial production deployments, or for development/testing environments. The large size is for production environments where there is a consistently high workload.

~> Note: For high workloads, ensure that the disks support high IOPS to keep up with the high Raft log update rate.

## 1.1 Datacenter Design

A Consul cluster (typically 3 or 5 servers plus client agents) may be deployed in a single physical datacenter or it may span multiple datacenters. For a large cluster with high runtime reads and writes, deploying servers in the same physical location improves performance. In cloud environments, a single datacenter may be deployed across multiple availability zones i.e. each server in a separate availability zone within a single EC2 instance. Consul also supports multi-datacenter deployments via two separate clusters joined by WAN links. In some cases, one may also deploy two or more Consul clusters in the same LAN environment.

~> TODO Clarify third sentence above: datacenter, AZ, instance

### 1.1.1 Single Datacenter

A single Consul cluster is recommended for applications deployed in the same datacenter. Consul supports traditional three-tier applications as well as microservices.

Typically, there must be three or five servers to balance between availability and performance. These servers together run the Raft-driven consistent state store for catalog, session, prepared query, ACL, and K/V updates.

~> TODO Does the following mean `reduced` or `increased`? Also `size of K/V and watches` (is it the size of watches or the number?).

The recommended maximum cluster size for a single datacenter is `5,000` nodes. For a write-heavy and/or a read-heavy cluster, the sizing may need to be reduced further, considering the impact of the number and the size of K/V values and the watches. The time taken for gossip to converge increases as more client machines are added. Similarly, the time taken by the new server to join an existing multi-thousand node cluster with a large K/V store and update rate may increase as they are replicated to the new server’s log.

One must take care to use service tags in a way that assists with the kinds of queries that will be run against the cluster. If two services (e.g. green and blue) are running on the same cluster, appropriate service tags must be used to identify between them. If a query is made without tags, nodes running both blue and green services may show up in the results of the query.

~> TODO Might the above be something like a release number, or `current`, or is it something else?

In cases where a full mesh among all agents cannot be established due to network segmentation, Consul’s own network segments can be used. Network segments is an Enterprise feature that allows the creation of multiple tenants which share Raft servers in the same cluster. Each tenant has its own gossip pool and doesn’t communicate with the agents outside this pool. The K/V store, however, is shared between all tenants. If Consul network segments cannot be used, isolation between agents can be accomplished by creating discrete Consul datacenters.

~> TODO The above makes it sound like `Consul datacenter` is a feature. Does this mean `running Consul clusters in separate datacenters`?

### 1.1.2 Multiple Datacenters

Consul clusters in different datacenters running the same service can be joined by WAN links. The clusters operate independently and only communicate over the WAN on port `8302`. Unless explicitly configured via CLI or API, the Consul server will only return results from the local datacenter. Consul does not replicate data between multiple datacenters. The Consul-replicate tool can be used to replicate the K/V data periodically.

~> NOTE: A good practice is to enable TLS server name checking to avoid accidental cross-joining of agents.

~> TODO Check capitalization of `consul-replicate`

~> TODO RESUME EDITING

Advanced federation can be achieved with Network Areas (Enterprise). A typical use case is datacenter1 (dc1) hosts shared services like LDAP (or ACL datacenter) leveraged by all other datacenters. However, due to compliance issues, servers in dc2 must not connect with servers in dc3. This cannot be accomplished with the basic WAN federation. Basic federation requires that all the servers in dc1, dc2 and dc3 are connected in a full mesh and opens both gossip (8302 tcp/udp) and RPC (8300) ports for communication. Network areas allows peering between datacenters to make the services discoverable over WAN. With network areas, servers in dc1 can communicate with those in dc2 and dc3. However, no connectivity needs to be established between dc2 and dc3 which meets the compliance requirement of the organization in this use case. Servers that are part of the network area communicate over RPC only. This removes the overhead of sharing and maintaining symmetric key used by gossip across datacenters. It also reduces the attack surface at the gossip ports no longer need to be opened in the security gateways/firewalls.

Consul’s prepared query allows clients to do a datacenter failover for service discovery. For e.g. if a service “foo” in the local datacenter dc1 goes down, prepared query lets users define a geo fall back order to the nearest datacenter to check for healthy instances of the same service. Consul clusters must be WAN linked for prepared query to take effect. Prepared query, by default, resolves the query in the local datacenter first. Querying K/V store features is not supported by the prepared query. Prepared query works with ACL. Prepared query config/templates are maintained consistently in raft and are executed on the servers.

## 1.2 Network Connectivity

LAN gossip occurs between all agents in a single datacenter with each agent sending periodic probe to random agents from its member list. Initial probe is sent over UDP every 1 second and if a node fails to acknowledge within 200ms, the agent pings over TCP. If the TCP probe fails (10 seconds timeout), it asks configurable number of random nodes to probe the same node (also known as indirect probe). If there is no response from the peers regarding the status, that agent is marked down. The status of agent directly affects the service discovery results. If the agent is down the services that it is monitoring will also be marked as down.

Additionally, the agent also periodically performs a full state sync over TCP which is gossiping each agent’s understanding of the member list around it (node names + IPs) and their health status. These operations are expensive relative to the standard gossip protocol mentioned above and are synced every 30 seconds. For more details, refer to [Serf Gossip docs](https://www.serf.io/docs/internals/gossip.html)

Datacenter designs may opt for a layer 2 or a layer 3 network. Consul’s gossip protocol uses UDP probes initially to detect the health of its peers. In layer 2 networks, the arp request will be forwarded to all the devices so if you are running a fairly large cluster i.e. multi-thousand node Consul cluster, this may create some congestion as the gossip probe request for the nodes is sent almost once every second. This can be improved by increasing the host arp table sizeand arp cache expiration timer.

Layer 3 restricts the ARP requests to a smaller segment of network. Traffic between the segments typically traverses through a firewall and/or a router. ACL or firewall rules must be updated to allow the following ports:

|Name|Port|Flag|Description|
|----|----|----|-----------|
|Server RPC|8300||Used by servers to handle incoming requests from other agents. TCP only.|
|Serf LAN|8301||Used to handle gossip in the LAN. Required by all agents. TCP and UDP.|
|Serf WAN|8302|`-1` to disable (available in Consul 1.0.7)|Used by servers to gossip over the LAN and WAN to other servers. TCP and UDP.|
|HTTP API|8500|`-1` to disable|Used by clients to talk to the HTTP API. TCP only.|
|DNS&nbsp;Interface|8600|`-1` to disable||

As mentioned in the datacenters design section, network areas and network segments can be used to prevent opening up firewall ports between different subnets.

### 1.2.1 RAFT Tuning

Leader elections can be affected by networking issues between the servers. If the cluster spans across multiple zones, the network latency between them must be taken into consideration and the raft_multiplier must be adjusted accordingly.

By default, the recommended value for the production environments is `1`. This value must take into account the network latency between the servers and the read/write load on the servers.

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

It is recommended to increase the scaling factor for networks with wider latency.

The trade off is between leader stability and time to recover from an actual leader failure. A short multiplier minimises failure detection and election time but may be triggered frequently in high latency situations causing constant leadership churn and associated unavailability. A high multiplier reduces chances of spurious failures causing leadership churn at the expense of taking longer to detect a real failure and recover cluster availability.

Leadership instability can also be caused by under-provisioned CPU resources and is more likely in environments where CPU cycles available are shared with other workloads. In order to remain leader, the elected leader must send frequent heartbeat messages to all other servers every few hundred milliseconds. If some number of these are missing or late due to the leader not having sufficient CPU to send them on time, the other servers will detect it as failed and hold a new election.

## 1.3 Performance Tuning

Consul is write limited by disk I/O and read limited by CPU. Memory requirements will be dependent on the total size of K/V pairs stored and should be sized according to that data (as should the hard drive storage). The limit on a key’s value size is `512KB`.

For **write-heavy** workloads, the total RAM available for overhead must approximately be equal to

    number of keys * average key size * 2-3x

Since writes must be synced to disk (persistent storage) on a quorum of servers before they are committed, deploying a disk with high write throughput or SSD will enhance the performance on the write side.  ([Documentation](https://www.consul.io/docs/agent/options.html#\_data\_dir))

~> NOTE: Is `stale` an API thing or should it just be bold?

For a **read-heavy** workload, configure all the Consul server agents with the `allow_stale` DNS option, or query the API with **stale** [consistency mode](https://www.consul.io/api/index.html#consistency-modes). By default, all the queries made to the server are RPC forwarded to and serviced by the leader. By enabling stale reads, the queries will be responded to by any server thereby reducing the overhead on the leader. Typically, the stale response is `100ms` or less from the consistent mode but it drastically improves the performance and reduces the latency under high load. If the leader server is out of memory or the disk is full, the server eventually stops responding, loses its election and cannot move past its last commit time. However, by configuring max_stale and setting it to a large value, Consul will continue to respond to the queries in case of such outage scenarios. ([max_stale documentation](https://www.consul.io/docs/agent/options.html#max_stale)).

It should be noted that **stale** is not appropriate for coordination where strong consistency is important like locking or application leader election. For critical cases the optional **consistent** API query mode is required for true linearizability; the trade off is that this turns a read into a full quorum write so requires more resources and takes longer.

Read-heavy clusters may take advantage of the enhanced reading feature (Enterprise) for better scalability. This feature allows additional servers to be introduced as non-voters. Being a non-voter, the server will still have data replicated to it, but it will not be part of the quorum that the leader must wait for before committing log entries.

Consul’s agents open network sockets for communicating with the other nodes (gossip) and with the server agent. In addition, file descriptors are also opened for watch handlers, health checks, logs files. For write heavy cluster, the ulimit size must be increased from the default  value (`1024`) to prevent the leader from running out of file descriptors.

To prevent any CPU spikes from a misconfigured client, RPC requests to the server should be [rate limited](https://www.consul.io/docs/agent/options.html#limits) (Note: this is configured on the client agent only).

In addition, two performance indicators &mdash; `consul.runtime.alloc_bytes` and `consul.runtime.heap_objects` &mdash; can help diagnose if the current sizing is not adequately meeting the load. ([documentation](https://www.consul.io/docs/agent/telemetry.html))

## 1.4 Backups

Creating server backups is an important step in production deployments. Backups provide a mechanism for the server to recover from an outage (network loss, operator error or corrupted data directory). All agents write to the `-data-dir` before commit. This directory persists the local agent’s state and in the case of servers, it also holds the Raft information.Consul provides the snapshot command that can be run using the CLI command or the API to save the point in time snapshot of the state of the Consul servers which includes key/value entries, service catalog, prepared queries, sessions and ACL. Alternatively, with Consul Enterprise, snapshots (snapshot agent command) are automatically taken at the specified interval and stored remotely in S3 storage.

By default, all the snapshots are taken using consistent mode where the requests are forwarded to the leader and the leader verifies that it is still in power before taking the snapshot. The snapshots will not be saved if there is a cluster degradation or no leader available. To reduce the burden on the leader, it is possible to run the snapshot on any non-leader server using stale consistency mode by running the command:

    $consul snapshot save -stale backup.snap

[documentation](https://www.consul.io/docs/commands/snapshot/save.html). This spreads the load at the possible expense of losing full consistency guarantees. Typically this means only a very small amount of recent writes might not be included (typically data written in the last `100ms` or less from the recovery point) which is usually suitable for disaster recovery, however the system can’t bound how stale this may be if executed against a partitioned server.

## 1.5 Node Removal

Sometimes when a node is shutdown uncleanly or the process supervisor doesn't give the agent long enough to gracefully leave that node will be marked as failed and then cleaned up after 72 hours automatically. Until then, Consul periodically tries to reconnect to the failed node. After 72 hours, Consul will reap failed nodes and stop trying to reconnect. However, to be able to transition out the failed nodes quickly, force-leave command may be used. The nodes running as servers will be removed from the Raft quorum. Force-leave may also be used to remove the nodes that have accidentally joined the datacenter where it isn’t supposed to be. Force-leave can only be applied to the nodes in its respective datacenter and cannot be executed on the nodes outside the datacenter.

Alternately, the nodes can also be removed using `remove-peer` if `force-leave` is not effective in removing the nodes:

    $consul operator raft remove-peer -address=x.x.x.x:8300

Note that the above command only works on clusters that still have a leader. If the leader is affected by an outage, then manual recovery needs to be done. [documentation](https://www.consul.io/docs/guides/outage.html#manual-recovery-using-peers-json)

To remove all the agents that accidentally joined the wrong set of servers and start fresh, clear out contents in the data directory (`-data-dir`) on both client and server nodes. **Note that removing data on server nodes will destroy all state in the cluster and can’t be undone.**

Autopilot (Enterprise) feature automatically cleans up the dead servers instead of waiting for 72 hours to get reaped or running a script with force-leave or raft remove-peer. Dead servers will periodically be cleaned up and removed from the Raft peer set, to prevent them from interfering with the quorum size and leader elections.

Removing any server must be done carefully. For a cluster of `N` servers, `(N/2) + 1` must be available to function. To remove any old server from the cluster, first the new server must be added to make the cluster failure tolerant and then the old server can be removed.

~> TODO Resolve comment

[\[a\]](#cmnt_ref1)Somewhat true, but actually the size of the KV data and the update rate (i.e. amount of log to reply since last snapshot) are probably more important here and I don't think they are major bottlenecks in practice.

Time it takes for gossip to converge seems more likely to be a concern for very large numbers of clients but Preetha can probably talk with more authority on this.
