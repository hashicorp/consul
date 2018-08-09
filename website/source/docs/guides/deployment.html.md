---
layout: "docs"
page_title: "Production Deployment"
sidebar_current: "docs-guides-deployment"
description: |-
  Best practice approaches for Consul production architecture.
---

# Consul Production Deployment

The goal of this document is to recommend best practice approaches for Consul production deployments. The guide provides recommendations on system requirements, datacenter design, networking, and performance optimizations for Consul cluster based on the latest Consul release.

This document assumes a basic working knowledge of Consul.

## System Requirements

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

The **small size** instance configuration is appropriate for most initial production deployments, or for development/testing environments. The large size is for production environments where there is a consistently high workload. Suggested instance types are provided for common platforms, but do refer to platform documentation for up-to-date instance types that align with the recommended resources.

~> **NOTE** For large workloads, ensure that the disks support a high number of IOPS to keep up with the rapid Raft log update rate.

## Datacenter Design

Consul may be deployed in a single physical datacenter or it may span multiple datacenters.

A single Consul datacenter includes a set of servers that maintain consensus. These are designed to be deployed in a single geographical location to ensure low latency for consensus operations otherwise performance and stability can be significantly compromised. In cloud environments, spreading the servers over multiple availability zones within the same region is recommended for fault tolerance while maintaining low-latency and high performance networking.

Consul supports multi-datacenter deployments via federation. The servers in each datacenter are joined by WAN gossip to enable forwarding service discovery and KV requests between datacenters. The separate datacenters do not replicate services or KV state between them to ensure they remain as isolated failure domains. Each set of servers must remain available (with at least a quorum online) for cross-datacenter requests to be made. If one datacenter has an outage of the servers, services within that datacenter will not be discoverable outside it until the servers recover.

### Single Datacenter

A single Consul cluster is recommended for applications deployed in the same datacenter.

Typically, there must be three or five servers to balance between availability and performance. These servers together run the Raft-driven consistent state store for catalog, session, prepared query, ACL, and KV updates.

Consul is proven to work well with up to `5,000` nodes in a single datacenter gossip pool. Some deployments have stretched this number much further but they require care and testing to ensure they remain reliable and can converge their cluster state quickly. It's highly recommended that clusters are increased in size gradually when approaching or exceeding `5,000` nodes.

~> For write-heavy clusters, consider scaling vertically with larger machine instances and lower latency storage.

In cases where a full mesh among all agents cannot be established due to network segmentation, Consul’s own [network segments](/docs/enterprise/network-segments/index.html) can be used. Network segments is an Enterprise feature that allows the creation of multiple tenants which share Raft servers in the same cluster. Each tenant has its own gossip pool and doesn’t communicate with the agents outside this pool. The KV store, however, is shared between all tenants. If Consul network segments cannot be used, isolation between agents can be accomplished by creating discrete [Consul datacenters](/docs/guides/datacenters.html).

### Multiple Datacenters

Consul clusters in different datacenters running the same service can be joined by WAN links. The clusters operate independently and only communicate over the WAN on port `8302`. Unless explicitly configured via CLI or API, the Consul server will only return results from the local datacenter. Consul does not replicate data between multiple datacenters. The [consul-replicate](https://github.com/hashicorp/consul-replicate) tool can be used to replicate the KV data periodically.

-> A good practice is to enable TLS server name checking to avoid accidental cross-joining of agents.

Advanced federation can be achieved with [Network Areas](/api/operator/area.html) (Enterprise).

A typical use case is where datacenter1 (dc1) hosts share services like LDAP (or ACL datacenter) which are leveraged by all other datacenters. However, due to compliance issues, servers in dc2 must not connect with servers in dc3. This cannot be accomplished with the basic WAN federation. Basic federation requires that all the servers in dc1, dc2 and dc3 are connected in a full mesh and opens both gossip (`8302 tcp/udp`) and RPC (`8300`) ports for communication.

Network areas allows peering between datacenters to make the services discoverable over WAN. With network areas, servers in dc1 can communicate with those in dc2 and dc3. However, no connectivity needs to be established between dc2 and dc3 which meets the compliance requirement of the organization in this use case. Servers that are part of the network area communicate over RPC only. This removes the overhead of sharing and maintaining the symmetric key used by the gossip protocol across datacenters. It also reduces the attack surface at the gossip ports since they no longer need to be opened in security gateways or firewalls.

Consul’s [prepared queries](/api/query.html) allow clients to do a datacenter failover for service discovery. For example, if a service `payment` in the local datacenter dc1 goes down, a prepared query lets users define a geographic fallback order to the nearest datacenter to check for healthy instances of the same service.

~> **NOTE** Consul clusters must be WAN linked for a prepared query to work across datacenters.

Prepared queries, by default, resolve the query in the local datacenter first. Querying KV store features is not supported by the prepared query. Prepared queries work with ACL. Prepared query config/templates are maintained consistently in Raft and are executed on the servers.

## Network Connectivity

LAN gossip occurs between all agents in a single datacenter with each agent sending a periodic probe to random agents from its member list. Agents run in either client or server mode, both participate in the gossip. The initial probe is sent over UDP every second. If a node fails to acknowledge within `200ms`, the agent pings over TCP. If the TCP probe fails (10 second timeout), it asks configurable number of random nodes to probe the same node (also known as an indirect probe). If there is no response from the peers regarding the status of the node, that agent is marked as down.

The agent's status directly affects the service discovery results. If an agent is down, the services it is monitoring will also be marked as down.

In addition, the agent also periodically performs a full state sync over TCP which gossips each agent’s understanding of the member list around it (node names, IP addresses, and health status). These operations are expensive relative to the standard gossip protocol mentioned above and are synced at a rate determined by cluster size to keep overhead low. It's typically between 30 seconds and 5 minutes. For more details, refer to [Serf Gossip docs](https://www.serf.io/docs/internals/gossip.html)

In a larger network that spans L2 segments, traffic typically traverses through a firewall and/or a router. ACL or firewall rules must be updated to allow the following ports:

|Name|Port|Flag|Description|
|----|----|----|-----------|
|Server RPC|8300||Used by servers to handle incoming requests from other agents (clients and servers). TCP only.|
|Serf LAN|8301||Used to handle gossip in the LAN. Required by all agents. TCP and UDP.|
|Serf WAN|8302|`-1` to disable (available in Consul 1.0.7)|Used by servers to gossip over the LAN and WAN to other servers. TCP and UDP.|
|HTTP API|8500|`-1` to disable|Used by clients to talk to the HTTP API. TCP only.|
|DNS&nbsp;Interface|8600|`-1` to disable||

-> As mentioned in the [datacenter design section](#1-1-1-single-datacenter), network areas and network segments can be used to prevent opening up firewall ports between different subnets.

By default agents will only listen for HTTP and DNS traffic on the local interface.

### Raft Tuning

Leader elections can be affected by network communication issues between servers. If the cluster spans multiple zones, the network latency between them must be taken into consideration and the [`raft_multiplier`](/docs/agent/options.html#raft_multiplier) must be adjusted accordingly.

By default, the recommended value for production environments is `1`. This value must take into account the network latency between the servers and the read/write load on the servers.

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

## Performance Tuning

Consul is write limited by disk I/O and read limited by CPU. Memory requirements will be dependent on the total size of KV pairs stored and should be sized according to that data (as should the hard drive storage). The limit on a key’s value size is `512KB`.

-> Consul is write limited by disk I/O and read limited by CPU.

For **write-heavy** workloads, the total RAM available for overhead must approximately be equal to

    RAM NEEDED = number of keys * (average key + value size) * 2-3x

Since writes must be synced to disk (persistent storage) on a quorum of servers before they are committed, deploying a disk with high write throughput (or an SSD) will enhance performance on the write side ([configuration reference](/docs/agent/options.html#\_data\_dir)).

For a **read-heavy** workload, configure all Consul server agents with the `allow_stale` DNS option, or query the API with the `stale` [consistency mode](/api/index.html#consistency-modes). By default, all queries made to the server are RPC forwarded to and serviced by the leader. By enabling stale reads, any server will respond to any query, thereby reducing overhead on the leader. Typically, the stale response is `100ms` or less from consistent mode but it drastically improves performance and reduces latency under high load.

If the leader server is out of memory or the disk is full, the server eventually stops responding, loses its election and cannot move past its last commit time. However, by configuring `max_stale` and setting it to a large value, Consul will continue to respond to queries during such outage scenarios ([max_stale documentation](/docs/agent/options.html#max_stale)).

It should be noted that `stale` is not appropriate for coordination where strong consistency is important (i.e. locking or application leader election). For critical cases, the optional `consistent` API query mode is required for true linearizability; the trade off is that this turns a read into a full quorum operation so requires more resources and takes longer.

**Read-heavy** clusters may take advantage of the [enhanced reading](/docs/enterprise/read-scale/index.html) feature (Enterprise) for better scalability. This feature allows additional servers to be introduced as non-voters. Being a non-voter, the server will still participate in data replication, but it will not block the leader from committing log entries.

Consul’s agents use network sockets for communicating with the other nodes (gossip) and with the server agent. In addition, file descriptors are also opened for watch handlers, health checks, and log files. For a **write heavy** cluster, the `ulimit` size must be increased from the default  value (`1024`) to prevent the leader from running out of file descriptors.

A safe limit to set for `ulimit` will depend heavily on cluster size and workload but there is usually no downside to being liberal and allowing tens of thousands of descriptors for large servers (or even more).

To prevent any CPU spikes from a misconfigured client, RPC requests to the server should be [rate limited](/docs/agent/options.html#limits)

~> **NOTE** Rate limiting is configured on the client agent only.

In addition, two [performance indicators](/docs/agent/telemetry.html) &mdash; `consul.runtime.alloc_bytes` and `consul.runtime.heap_objects` &mdash; can help diagnose if the current sizing is not adequately meeting the load.

## Backups

Creating server backups is an important step in production deployments. Backups provide a mechanism for the server to recover from an outage (network loss, operator error, or a corrupted data directory). All agents write to the `-data-dir` before commit. This directory persists the local agent’s state and &mdash; in the case of servers &mdash; it also holds the Raft information.

The local agent state on client agents is largely an optimization and does not typically need backing up. If this data is lost, any API registrations will need to be replayed and if Connect managed proxy instances were running, they will need to be manually stopped as they will no longer be managed by the agent, other than that a new agent can rejoin the cluster with no issues.

Consul provides the [snapshot](/docs/commands/snapshot.html) command which can be run using the CLI command or the API. The `snapshot` command saves the point-in-time snapshot of the state of the Consul servers which includes all cluster state including but not limited to KV entries, the service catalog, prepared queries, sessions, ACL, Connect CA state, and the Connect intention graph.

With [Consul Enterprise](/docs/commands/snapshot/agent.html), the `snapshot agent` command runs periodically and writes to local or remote storage (such as Amazon S3).

By default, all snapshots are taken using `consistent` mode where requests are forwarded to the leader which verifies that it is still in power before taking the snapshot. Snapshots will not be saved if the clusted is degraded or if no leader is available. To reduce the burden on the leader, it is possible to [run the snapshot](/docs/commands/snapshot/save.html) on any non-leader server using `stale` consistency mode:

    $ consul snapshot save -stale backup.snap

This spreads the load across nodes at the possible expense of losing full consistency guarantees. Typically this means that a very small number of recent writes may not be included. The omitted writes are typically limited to data written in the last `100ms` or less from the recovery point. This is usually suitable for disaster recovery. However, the system can’t guarantee how stale this may be if executed against a partitioned server.

## Node Removal

Failed nodes will be automatically removed after 72 hours. This can happen if a node does not shutdown cleanly or if the process supervisor does not give the agent long enough to gracefully leave the cluster. Until then, Consul periodically tries to reconnect to the failed node. After 72 hours, Consul will reap failed nodes and stop trying to reconnect.

This sequence can be accelerated with the [`force-leave`](/docs/commands/force-leave.html) command. Nodes running as servers will be removed from the Raft quorum. Force-leave may also be used to remove nodes that have accidentally joined the datacenter. Force-leave can only be applied to the nodes in its respective datacenter and cannot be executed on the nodes outside the datacenter.

Alternately, servers can be removed using [`remove-peer`](/docs/commands/operator/raft.html#remove-peer) if `force-leave` is not effective in removing the nodes.

    $ consul operator raft remove-peer -address=x.x.x.x:8300

~> **NOTE** `remove-peer` only works on clusters that still have a leader.

If the leader is affected by an outage, then read the [outage recovery](/docs/guides/outage.html) guide. Depending on your scenario, several options for recovery may be possible.

To remove all agents that accidentally joined the wrong set of servers, clear out the contents of the data directory (`-data-dir`) on both client and server nodes.

!> **WARNING** Removing data on server nodes will destroy all state in the cluster and can’t be undone.

The [Autopilot](/docs/guides/autopilot.html) (Enterprise) feature automatically cleans up dead servers instead of waiting 72 hours. Dead servers will periodically be cleaned up and removed from the Raft peer set, to prevent them from interfering with the quorum size and leader elections.

Removing any server must be done carefully. For a cluster of `N` servers to function properly, `(N/2) + 1` must be available. Before removing an old server from the cluster, the new server must be added in order to make the cluster failure tolerant. The old server can then be removed.
