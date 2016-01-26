---
layout: "docs"
page_title: "Gossip Protocol"
sidebar_current: "docs-internals-gossip"
description: |-
  Consul uses a gossip protocol to manage membership and broadcast messages to the cluster. All of this is provided through the use of the Serf library. The gossip protocol used by Serf is based on SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol, with a few minor adaptations.
---

# Gossip Protocol

Consul uses a [gossip protocol](https://en.wikipedia.org/wiki/Gossip_protocol)
to manage membership and broadcast messages to the cluster. All of this is provided
through the use of the [Serf library](https://www.serfdom.io/). The gossip protocol
used by Serf is based on
["SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol"](http://www.cs.cornell.edu/~asdas/research/dsn02-swim.pdf),
with a few minor adaptations. There are more details about [Serf's protocol here](https://www.serfdom.io/docs/internals/gossip.html).

~> **Advanced Topic!** This page covers technical details of
the internals of Consul. You don't need to know these details to effectively
operate and use Consul. These details are documented here for those who wish
to learn about them without having to go spelunking through the source code.

## Gossip in Consul

Consul makes use of two different gossip pools. We refer to each pool as the
LAN or WAN pool respectively. Each datacenter Consul operates in has a LAN gossip pool
containing all members of the datacenter, both clients and servers. The LAN pool is
used for a few purposes. Membership information allows clients to automatically discover
servers, reducing the amount of configuration needed. The distributed failure detection
allows the work of failure detection to be shared by the entire cluster instead of
concentrated on a few servers. Lastly, the gossip pool allows for reliable and fast
event broadcasts for events like leader election.

The WAN pool is globally unique, as all servers should participate in the WAN pool
regardless of datacenter. Membership information provided by the WAN pool allows
servers to perform cross datacenter requests. The integrated failure detection
allows Consul to gracefully handle an entire datacenter losing connectivity, or just
a single server in a remote datacenter.

All of these features are provided by leveraging [Serf](https://www.serfdom.io/). It
is used as an embedded library to provide these features. From a user perspective,
this is not important, since the abstraction should be masked by Consul. It can be useful
however as a developer to understand how this library is leveraged.
