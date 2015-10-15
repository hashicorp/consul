---
layout: "docs"
page_title: "Multiple Datacenters"
sidebar_current: "docs-guides-datacenters"
description: |-
  One of the key features of Consul is its support for multiple datacenters. The architecture of Consul is designed to promote low coupling of datacenters so that connectivity issues or failure of any datacenter does not impact the availability of Consul in other datacenters. This means each datacenter runs independently, each having a dedicated group of servers and a private LAN gossip pool.
---

# Multiple Datacenters

One of the key features of Consul is its support for multiple datacenters.
The [architecture](/docs/internals/architecture.html) of Consul is designed to
promote a low coupling of datacenters so that connectivity issues or
failure of any datacenter does not impact the availability of Consul in other
datacenters. This means each datacenter runs independently, each having a dedicated
group of servers and a private LAN [gossip pool](/docs/internals/gossip.html).

To get started, follow the [bootstrapping guide](/docs/guides/bootstrapping.html) to
start each datacenter. After bootstrapping, we should have two datacenters now which
we can refer to as `dc1` and `dc2`. Note that datacenter names are opaque to Consul;
they are simply labels that help human operators reason about the Consul clusters.

To query the known WAN nodes, we use the [`members`](/docs/commands/members.html)
command with the `-wan` parameter:

```text
$ consul members -wan
...
```

This will provide a list of all known members in the WAN gossip pool. This should
only contain server nodes. Client nodes send requests to a datacenter-local server,
so they do not participate in WAN gossip. Client requests are forwarded by local
servers to a server in the target datacenter as necessary.

The next step is to ensure that all the server nodes join the WAN gossip pool:

```text
$ consul join -wan <server 1> <server 2> ...
...
```

The [`join`](/docs/commands/join.html) command is used with the `-wan` flag to indicate
we are attempting to join a server in the WAN gossip pool. As with LAN gossip, you only
need to join a single existing member, and the gossip protocol will be used to exchange
information about all known members. For the initial setup, however, each server
will only know about itself and must be added to the cluster.

Once the join is complete, the [`members`](/docs/commands/members.html) command can be
used to verify that all server nodes gossiping over WAN.

We can also verify that both datacenters are known using the
[HTTP Catalog API](/docs/agent/http/catalog.html#catalog_datacenters):

```text
$ curl http://localhost:8500/v1/catalog/datacenters
["dc1", "dc2"]
```

As a simple test, you can try to query the nodes in each datacenter:

```text
$ curl http://localhost:8500/v1/catalog/nodes?dc=dc1
...
$ curl http://localhost:8500/v1/catalog/nodes?dc=dc2
...
```

There are a few networking requirements that must be satisfied for this to
work. Of course, all server nodes must be able to talk to each other. Otherwise,
the gossip protocol as well as RPC forwarding will not work. If service discovery
is to be used across datacenters, the network must be able to route traffic
between IP addresses across regions as well. Usually, this means that all datacenters
must be connected using a VPN or other tunneling mechanism. Consul does not handle
VPN, address rewriting, or NAT traversal for you.
