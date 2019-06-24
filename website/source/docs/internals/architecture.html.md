---
layout: "docs"
page_title: "Consul Architecture"
sidebar_current: "docs-internals-architecture"
description: |-
  Consul is a complex system that has many different moving parts. To help users and developers of Consul form a mental model of how it works, this page documents the system architecture.
---

# Consul Architecture

Consul is a complex system that has many different moving parts. To help
users and developers of Consul form a mental model of how it works, this
page documents the system architecture.

-> Before describing the architecture, we recommend reading the 
[glossary](/docs/glossary) of terms to help
clarify what is being discussed.


## 10,000 foot view

From a 10,000 foot altitude the architecture of Consul looks like this:

<div class="center">
[![Consul Architecture](/assets/images/consul-arch.png)](/assets/images/consul-arch.png)
</div>

Let's break down this image and describe each piece. First of all, we can see
that there are two datacenters, labeled "one" and "two". Consul has first
class support for [multiple datacenters](https://learn.hashicorp.com/consul/security-networking/datacenters) and
expects this to be the common case.

Within each datacenter, we have a mixture of clients and servers. It is expected
that there be between three to five servers. This strikes a balance between
availability in the case of failure and performance, as consensus gets progressively
slower as more machines are added. However, there is no limit to the number of clients,
and they can easily scale into the thousands or tens of thousands.

All the agents that are in a datacenter participate in a [gossip protocol](/docs/internals/gossip.html).
This means there is a gossip pool that contains all the agents for a given datacenter. This serves
a few purposes: first, there is no need to configure clients with the addresses of servers;
discovery is done automatically. Second, the work of detecting agent failures
is not placed on the servers but is distributed. This makes failure detection much more
scalable than naive heartbeating schemes. It also provides failure detection for the nodes; if the agent is not reachable, than the node may have experienced a failure. Thirdly, it is used as a messaging layer to notify
when important events such as leader election take place.

The servers in each datacenter are all part of a single Raft peer set. This means that
they work together to elect a single leader, a selected server which has extra duties. The leader
is responsible for processing all queries and transactions. Transactions must also be replicated to
all peers as part of the [consensus protocol](/docs/internals/consensus.html). Because of this
requirement, when a non-leader server receives an RPC request, it forwards it to the cluster leader.

The server agents also operate as part of a WAN gossip pool. This pool is different from the LAN pool
as it is optimized for the higher latency of the internet and is expected to contain only
other Consul server agents. The purpose of this pool is to allow datacenters to discover each
other in a low-touch manner. Bringing a new datacenter online is as easy as joining the existing
WAN gossip pool. Because the servers are all operating in this pool, it also enables cross-datacenter
requests. When a server receives a request for a different datacenter, it forwards it to a random
server in the correct datacenter. That server may then forward to the local leader.

This results in a very low coupling between datacenters, but because of failure detection,
connection caching and multiplexing, cross-datacenter requests are relatively fast and reliable.

In general, data is not replicated between different Consul datacenters. When a
request is made for a resource in another datacenter, the local Consul servers forward
an RPC request to the remote Consul servers for that resource and return the results.
If the remote datacenter is not available, then those resources will also not be
available, but that won't otherwise affect the local datacenter. There are some special
situations where a limited subset of data can be replicated, such as with Consul's built-in
[ACL replication](https://learn.hashicorp.com/consul/day-2-operations/acl-replication) capability, or
external tools like [consul-replicate](https://github.com/hashicorp/consul-replicate).

In some places, client agents may cache data from the servers to make it
available locally for performance and reliability. Examples include Connect
certificates and intentions which allow the client agent to make local decisions
about inbound connection requests without a round trip to the servers. Some API
endpoints also support optional result caching. This helps reliability because
the local agent can continue to respond to some queries like service-discovery
or Connect authorization from cache even if the connection to the servers is
disrupted or the servers are temporarily unavailable.

## Getting in depth

At this point we've covered the high level architecture of Consul, but there are many
more details for each of the subsystems. The [consensus protocol](/docs/internals/consensus.html) is
documented in detail as is the [gossip protocol](/docs/internals/gossip.html). The [documentation](/docs/internals/security.html)
for the security model and protocols used are also available.

For other details, either consult the code, ask in IRC, or reach out to the mailing list.
