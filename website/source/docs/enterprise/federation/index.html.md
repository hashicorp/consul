---
layout: "docs"
page_title: "Consul Enterprise Advanced Federation"
sidebar_current: "docs-enterprise-federation"
description: |-
  Consul Enterprise enables you to federate Consul datacenters together on a pairwise basis, enabling partially-connected network topologies like hub-and-spoke.
---

# Consul Enterprise Advanced Federation

Consul's core federation capability uses the same gossip mechanism that is used
for a single datacenter. This requires that every server from every datacenter
be in a fully connected mesh with an open gossip port (8302/tcp and 8302/udp)
and an open server RPC port (8300/tcp). For organizations with large numbers of
datacenters, it becomes difficult to support a fully connected mesh. It is often
desirable to have topologies like hub-and-spoke with central management
datacenters and "spoke" datacenters that can't interact with each other.

[Consul Enterprise](https://www.hashicorp.com/consul.html) offers a [network
area mechanism](https://learn.hashicorp.com/consul/day-2-operations/advanced-federation) that allows operators to
federate Consul datacenters together on a pairwise basis, enabling
partially-connected network topologies. Once a link is created, Consul agents
can make queries to the remote datacenter in service of both API and DNS
requests for remote resources (in spite of the partially-connected nature of the
topology as a whole). Consul datacenters can simultaneously participate in both
network areas and the existing WAN pool, which eases migration.
