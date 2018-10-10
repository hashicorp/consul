---
layout: "docs"
page_title: "Consul Reference Architecture"
sidebar_current: "docs-guides-reference-architecture"
description: |-
  This document provides recommended practices and a reference
  architecture for HashiCorp Consul production deployments.
product_version: 1.2
---

# Consul Reference Architecture

As applications are migrated to dynamically provisioned infrastructure, scaling services and managing the communications between them becomes challenging. Consul’s service discovery capabilities provide the connectivity between dynamic applications. Consul also monitors the health of each node and its applications to ensure that only healthy service instances are discovered. Consul’s distributed runtime configuration store allows updates across global infrastructure.

This document provides recommended practices and a reference architecture, including system requirements, datacenter design, networking, and performance optimizations for Consul production deployments.

## Infrastructure Requirements

### Consul Servers

Consul server agents are responsible for maintaining the cluster state, responding to RPC queries (read operations), and for processing all write operations. Given that Consul server agents do most of the heavy lifting, server sizing is critical for the overall performance efficiency and health of the Consul cluster.

The following table provides high-level server guidelines. Of particular
note is the strong recommendation to avoid non-fixed performance CPUs,
or "Burstable CPU".

| Type        | CPU      | Memory       | Disk  | Typical Cloud Instance Types                  |
|-------------|----------|--------------|-------|-----------------------------------------------|
| Small       | 2 core   | 8-16 GB RAM  | 50GB  | **AWS**: m5.large, m5.xlarge                  |
|             |          |              |       | **Azure**: Standard\_A4\_v2, Standard\_A8\_v2 |
|             |          |              |       | **GCE**: n1-standard-8, n1-standard-16        |
| Large       | 4-8 core | 32-64 GB RAM | 100GB | **AWS**: m5.2xlarge, m5.4xlarge               |
|             |          |              |       | **Azure**: Standard\_D4\_v3, Standard\_D5\_v3 |
|             |          |              |       | **GCE**: n1-standard-32, n1-standard-64       |

#### Hardware Sizing Considerations

- The small size would be appropriate for most initial production
  deployments, or for development/testing environments.

- The large size is for production environments where there is a
  consistently high workload.

~> **NOTE** For large workloads, ensure that the disks support a high number of IOPS to keep up with the rapid Raft log update rate.

For more information on server requirements, review the [server performance](/docs/guides/performance.html) documentation.

## Infrastructure Diagram

![Reference Diagram](/assets/images/consul-arch.png "Reference Diagram")

## Datacenter Design

A Consul cluster (typically three or five servers plus client agents) may be deployed in a single physical datacenter or it may span multiple datacenters. For a large cluster with high runtime reads and writes, deploying servers in the same physical location improves performance. In cloud environments, a single datacenter may be deployed across multiple availability zones i.e. each server in a separate availability zone on a single host. Consul also supports multi-datacenter deployments via separate clusters joined by WAN links. In some cases, one may also deploy two or more Consul clusters in the same LAN environment.

### Single Datacenter

A single Consul cluster is recommended for applications deployed in the same datacenter. Consul supports traditional three-tier applications as well as microservices.

Typically, there must be three or five servers to balance between availability and performance. These servers together run the Raft-driven consistent state store for catalog, session, prepared query, ACL, and KV updates.

The recommended maximum cluster size for a single datacenter is 5,000 nodes. For a write-heavy and/or a read-heavy cluster, the maximum number of nodes may need to be reduced further, considering the impact of the number and the size of KV pairs and the number of watches. The time taken for gossip to converge increases as more client machines are added. Similarly, the time taken by the new server to join an existing multi-thousand node cluster with a large KV store and update rate may increase as they are replicated to the new server’s log.

-> **TIP** For write-heavy clusters, consider scaling vertically with larger machine instances and lower latency storage.

One must take care to use service tags in a way that assists with the kinds of queries that will be run against the cluster. If two services (e.g. blue and green) are running on the same cluster, appropriate service tags must be used to identify between them. If a query is made without tags, nodes running both blue and green services may show up in the results of the query.

In cases where a full mesh among all agents cannot be established due to network segmentation, Consul’s own [network segments](/docs/enterprise/network-segments/index.html) can be used. Network segments is a Consul Enterprise feature that allows the creation of multiple tenants which share Raft servers in the same cluster. Each tenant has its own gossip pool and doesn’t communicate with the agents outside this pool. The KV store, however, is shared between all tenants. If Consul network segments cannot be used, isolation between agents can be accomplished by creating discrete [Consul datacenters](/docs/guides/datacenters.html).

### Multiple Datacenters

Consul clusters in different datacenters running the same service can be joined by WAN links. The clusters operate independently and only communicate over the WAN on port `8302`. Unless explicitly configured via CLI or API, the Consul server will only return results from the local datacenter. Consul does not replicate data between multiple datacenters. The [consul-replicate](https://github.com/hashicorp/consul-replicate) tool can be used to replicate the KV data periodically.

-> A good practice is to enable TLS server name checking to avoid accidental cross-joining of agents.

Advanced federation can be achieved with the [network areas](/api/operator/area.html) feature in Consul Enterprise.

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

| Name          | Port | Flag | Description |
|---------------|------|------|-------------|
| Server RPC    | 8300 |      | Used by servers to handle incoming requests from other agents. TCP only. |
| Serf LAN      | 8301 |      | Used to handle gossip in the LAN. Required by all agents. TCP and UDP. |
| Serf WAN      | 8302 | `-1` to disable (available in Consul 1.0.7) | Used by servers to gossip over the LAN and WAN to other servers. TCP and UDP. |
| HTTP API      | 8500 | `-1` to disable | Used by clients to talk to the HTTP API. TCP only. |
| DNS Interface | 8600 | `-1` to disable | |

-> As mentioned in the [datacenter design section](#datacenter-design), network areas and network segments can be used to prevent opening up firewall ports between different subnets.

By default agents will only listen for HTTP and DNS traffic on the local interface.

## Next steps

- Read [Deployment Guide](/docs/guides/deployment-guide.html) to learn
  the steps required to install and configure a single HashiCorp Consul cluster.

- Read [Server Performance](/docs/guides/performance.html) to learn about
  additional configuration that benefits production deployments.
