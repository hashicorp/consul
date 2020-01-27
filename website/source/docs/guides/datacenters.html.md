---
layout: "docs"
page_title: "Multiple Datacenters - Basic Federation with the WAN Gossip Pool"
sidebar_current: "docs-guides-datacenters"
description: |-
  One of the key features of Consul is its support for multiple datacenters. The architecture of Consul is designed to promote low coupling of datacenters so that connectivity issues or failure of any datacenter does not impact the availability of Consul in other datacenters. This means each datacenter runs independently, each having a dedicated group of servers and a private LAN gossip pool.
---


# Multiple Datacenters: Basic Federation with the WAN Gossip

One of the key features of Consul is its support for multiple datacenters.
The [architecture](/docs/internals/architecture.html) of Consul is designed to
promote a low coupling of datacenters so that connectivity issues or
failure of any datacenter does not impact the availability of Consul in other
datacenters. This means each datacenter runs independently, each having a dedicated
group of servers and a private LAN [gossip pool](/docs/internals/gossip.html).

## The WAN Gossip Pool

This guide covers the basic form of federating Consul clusters using a single
WAN gossip pool, interconnecting all Consul servers.
[Consul Enterprise](https://www.hashicorp.com/products/consul/) version 0.8.0 added support
for an advanced multiple datacenter capability. Please see the
[Advanced Federation Guide](/docs/guides/advanced-federation.html) for more details.

## Setup Two Datacenters

To get started, follow the [
Deployment guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/deployment-guide/) to
start each datacenter. After bootstrapping, we should have two datacenters now which
we can refer to as `dc1` and `dc2`. Note that datacenter names are opaque to Consul;
they are simply labels that help human operators reason about the Consul clusters.

To query the known WAN nodes, we use the [`members`](/docs/commands/members.html)
command with the `-wan` parameter on either datacenter.

```sh
$ consul members -wan
```

This will provide a list of all known members in the WAN gossip pool. In
this case, we have not connected the servers so there will be no output.

`consul members -wan` should
only contain server nodes. Client nodes send requests to a datacenter-local server,
so they do not participate in WAN gossip. Client requests are forwarded by local
servers to a server in the target datacenter as necessary.

## Join the Servers

The next step is to ensure that all the server nodes join the WAN gossip pool (include all the servers in all the datacenters).

```sh
$ consul join -wan <server 1> <server 2> ...
```

The [`join`](/docs/commands/join.html) command is used with the `-wan` flag to indicate
we are attempting to join a server in the WAN gossip pool. As with LAN gossip, you only
need to join a single existing member, and the gossip protocol will be used to exchange
information about all known members. For the initial setup, however, each server
will only know about itself and must be added to the cluster. Consul 0.8.0 added WAN join
flooding, so if one Consul server in a datacenter joins the WAN, it will automatically
join the other servers in its local datacenter that it knows about via the LAN.

### Persist Join with Retry Join

In order to persist the `join` information, the following can be added to each server's configuration file, in both datacenters. For example, in `dc1` server nodes.

```json
  "retry_join_wan":[
    "dc2-server-1",
    "dc2-server-2"
  ],
```

## Verify Multi-DC Configuration

Once the join is complete, the [`members`](/docs/commands/members.html) command can be
used to verify that all server nodes gossiping over WAN.

```sh
$ consul members -wan
Node          Address          Status  Type    Build  Protocol  DC   Segment
dc1-server-1  127.0.0.1:8701   alive   server  1.4.3  2         dc1  <all>
dc2-server-1  127.0.0.1:8702   alive   server  1.4.3  2         dc2  <all>
```

We can also verify that both datacenters are known using the
[HTTP Catalog API](/api/catalog.html#catalog_datacenters):

```sh
$ curl http://localhost:8500/v1/catalog/datacenters
["dc1", "dc2"]
```

As a simple test, you can try to query the nodes in each datacenter:

```sh
$ curl http://localhost:8500/v1/catalog/nodes?dc=dc1
{
	"ID": "ee8b5f7b-9cc1-a382-978c-5ce4b1219a55",
	"Node": "dc1-server-1",
	"Address": "127.0.0.1",
	"Datacenter": "dc1",
	"TaggedAddresses": {
		"lan": "127.0.0.1",
		"wan": "127.0.0.1"
	},
	"Meta": {
		"consul-network-segment": ""
	},
	"CreateIndex": 12,
	"ModifyIndex": 14
}
```
```sh
$ curl http://localhost:8500/v1/catalog/nodes?dc=dc2
{
	"ID": "ee8b5f7b-9cc1-a382-978c-5ce4b1219a55",
	"Node": "dc2-server-1",
	"Address": "127.0.0.1",
	"Datacenter": "dc1",
	"TaggedAddresses": {
		"lan": "127.0.0.1",
		"wan": "127.0.0.1"
	},
	"Meta": {
		"consul-network-segment": ""
	},
	"CreateIndex": 11,
	"ModifyIndex": 16
}
```

## Network Configuration 

There are a few networking requirements that must be satisfied for this to
work. Of course, all server nodes must be able to talk to each other. Otherwise,
the gossip protocol as well as RPC forwarding will not work. If service discovery
is to be used across datacenters, the network must be able to route traffic
between IP addresses across regions as well. Usually, this means that all datacenters
must be connected using a VPN or other tunneling mechanism. Consul does not handle
VPN or NAT traversal for you.

Note that for RPC forwarding to work the bind address must be accessible from remote nodes. 
Configuring `serf_wan`, `advertise_wan_addr` and `translate_wan_addrs` can lead to a
situation where `consul members -wan` lists remote nodes but RPC operations fail with one 
of the following errors:

- `No path to datacenter`
- `rpc error getting client: failed to get conn: dial tcp <LOCAL_ADDR>:0-><REMOTE_ADDR>:<REMOTE_RPC_PORT>: i/o timeout`

The most likely cause of these errors is that `bind_addr` is set to a private address preventing
the RPC server from accepting connections across the WAN. Setting `bind_addr` to a public
address (or one that can be routed across the WAN) will resolve this issue. Be aware that
exposing the RPC server on a public port should only be done **after** firewall rules have
been established.

The [`translate_wan_addrs`](/docs/agent/options.html#translate_wan_addrs) configuration
provides a basic address rewriting capability.

## Data Replication

In general, data is not replicated between different Consul datacenters. When a
request is made for a resource in another datacenter, the local Consul servers forward
an RPC request to the remote Consul servers for that resource and return the results.
If the remote datacenter is not available, then those resources will also not be
available, but that won't otherwise affect the local datacenter. There are some special
situations where a limited subset of data can be replicated, such as with Consul's built-in
[ACL replication](/docs/guides/acl.html#outages-and-acl-replication) capability, or
external tools like [consul-replicate](https://github.com/hashicorp/consul-replicate/).

## Summary

In this guide you setup WAN gossip across two datacenters to create
basic federation. You also used the Consul HTTP API to ensure the 
datacenters were properly configured.
