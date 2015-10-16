---
layout: "docs"
page_title: "Commands: RTT"
sidebar_current: "docs-commands-rtt"
description: >
  The `rtt` command estimates the netowrk round trip time between two nodes using Consul's network coordinate model of the cluster.
---

# Consul RTT

Command: `consul rtt`

The 'rtt' command estimates the network round trip time between two nodes using
Consul's network coordinate model of the cluster. While contacting nodes as part
of its normal gossip protocol, Consul builds up a set of network coordinates for
all the nodes in the local datacenter (the LAN pool) and remote datacenters (the WAN
pool). Agents forward these to the servers and once the coordinates for two nodes
are known, it's possible to estimate the network round trip time between them using
a simple calculation.

It is not possible to measure between LAN coordinates and WAN coordinates
because they are maintained by independent Serf gossip pools, so they are
not compatible.

## Usage

Usage: `consul rtt [options] node1 node2`

The two node names are required. Note that these are node names as known to
Consul as `consul members` would show, not IP addresses.

The list of available flags are:

* `-wan` - Instructs the command to use WAN coordinates instead of LAN
  coordinates. If the -wan option is given, then the node names must be prefixed
  by the datacenter and a period (eg. "dc1.sever"). By default, the two nodes are
  assumed to be nodes in the local datacenter the LAN coordinates are used.

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

## Output

If coordinates are available, the command will print the estimated round trip
time beteeen the given nodes:

```
$ consul rtt n1 n2
Estimated n1 <-> n2 rtt=0.610 ms (using LAN coordinates)

$ consul rtt -wan dc1.n1 dc2.n2
Estimated dc1.n1 <-> dc2.n2 rtt=1.275 ms (using WAN coordinates)
```
