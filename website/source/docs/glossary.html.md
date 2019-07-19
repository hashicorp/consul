---
layout: "docs"
page_title: "Consul Glossary"
sidebar_current: "docs-glossary"
description: |-
  This page collects brief definitions of some of the technical terms used in the documentation.
---

# Consul Glossary

This page collects brief definitions of some of the technical terms used in the documentation for Consul and Consul Enterprise, as well as some terms that come up frequently in conversations throughout the Consul community.

* [Agent](#agent) 
* [Client](#client) 
* [Server](#server)
* [Datacenter](#datacenter)
* [Consensus](#consensus) 
* [Gossip](#gossip)
* [LAN Gossip](#lan-gossip)
* [WAN Gossip](#wan-gossip)
* [RPC](#rpc) 

## Agent 

An agent is the long running daemon on every member of the Consul cluster.
It is started by running `consul agent`. The agent is able to run in either *client*
or *server* mode. Since all nodes must be running an agent, it is simpler to refer to
the node as being either a client or server, but there are other instances of the agent. All
agents can run the DNS or HTTP interfaces, and are responsible for running checks and
keeping services in sync.

## Client

A client is an agent that forwards all RPCs to a server. The client is relatively
stateless. The only background activity a client performs is taking part in the LAN gossip
pool. This has a minimal resource overhead and consumes only a small amount of network
bandwidth.

## Server 

A server is an agent with an expanded set of responsibilities including
participating in the Raft quorum, maintaining cluster state, responding to RPC queries,
exchanging WAN gossip with other datacenters, and forwarding queries to leaders or
remote datacenters.

## Datacenter  

We define a datacenter to be a networking environment that is
private, low latency, and high bandwidth. This excludes communication that would traverse
the public internet, but for our purposes multiple availability zones within a single EC2
region would be considered part of a single datacenter.

## Consensus 

When used in our documentation we use consensus to mean agreement upon
the elected leader as well as agreement on the ordering of transactions. Since these
transactions are applied to a
[finite-state machine](https://en.wikipedia.org/wiki/Finite-state_machine), our definition
of consensus implies the consistency of a replicated state machine. Consensus is described
in more detail on [Wikipedia](https://en.wikipedia.org/wiki/Consensus_(computer_science)),
and our implementation is described [here](/docs/internals/consensus.html).

## Gossip 

Consul is built on top of [Serf](https://www.serf.io/) which provides a full
[gossip protocol](https://en.wikipedia.org/wiki/Gossip_protocol) that is used for multiple purposes.
Serf provides membership, failure detection, and event broadcast. Our use of these
is described more in the [gossip documentation](/docs/internals/gossip.html). It is enough to know
that gossip involves random node-to-node communication, primarily over UDP.

## LAN Gossip 

Refers to the LAN gossip pool which contains nodes that are all
located on the same local area network or datacenter.

## WAN Gossip 

Refers to the WAN gossip pool which contains only servers. These
servers are primarily located in different datacenters and typically communicate
over the internet or wide area network.

## RPC 

Remote Procedure Call. This is a request / response mechanism allowing a
client to make a request of a server.