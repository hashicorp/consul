---
layout: "docs"
page_title: "Consul Enterprise Network Segments"
sidebar_current: "docs-enterprise-network-segments"
description: |-
  Consul Enterprise enables you create separate LAN gossip pools within one
  cluster to segment network groups.
---

# Consul Enterprise Network Segments

Consul Network Segments enables operators to create separate LAN gossip segments
in one Consul cluster. Agents in a segment are only able to join and communicate
with other agents in its network segment. This functionality is useful for
clusters that have multiple tenants that should not be able to communicate
with each other.

To get started with Network Segments,
[read the guide](/docs/guides/segments.html).

# Consul Networking Models

To help set context for this feature, it is useful to understand the various
Consul networking models and their capabilities.

**Cluster:** A set of Consul servers forming a Raft quorum along with a
collection of Consul clients, all set to the same
[datacenter](/docs/agent/options.html#_datacenter), and joined together to form
what we will call a "local cluster". Consul clients discover the Consul servers
in their local cluster through the gossip mechanism and make RPC requests to
them. LAN Gossip (OSS) is an open intra-cluster networking model, and  Network
Segments (Enterprise) creates multiple segments within one cluster.

**Federated Cluster:** A cluster of clusters with a Consul server group per
cluster each set per "datacenter". These Consul servers are federated together
over the WAN. Consul clients make use of resources in federated clusters by
forwarding RPCs through the Consul servers in their local cluster, but they
never interact with remote Consul servers directly. There are currently two
inter-cluster network models: [WAN Gossip (OSS)](/docs/guides/datacenters.html)
and [Network Areas (Enterprise)](/docs/guides/areas.html).

**LAN Gossip Pool**: A set of Consul agents that have full mesh connectivity
among themselves, and use Serf to maintain a shared view of the members of the
pool for different purposes, like finding a Consul server in a local cluster,
or finding servers in a remote cluster. A **segmented** LAN Gossip Pool limits a
group of agents to only connect with the agents in its segment.
