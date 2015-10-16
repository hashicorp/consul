---
layout: "docs"
page_title: "Commands: RTT"
sidebar_current: "docs-commands-rtt"
description: >
  The rtt command estimates the network round trip time between two nodes.
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

Usage: `consul rtt [options] node1 [node2]`

At least one node name is required. If the second node name isn't given, it
is set to the agent's node name. Note that these are node names as known to
Consul as `consul members` would show, not IP addresses.

The list of available flags are:

* `-wan` - Instructs the command to use WAN coordinates instead of LAN
  coordinates. By default, the two nodes are assumed to be nodes in the local
  datacenter and the LAN coordinates are used. If the -wan option is given,
  then the WAN coordinates are used, and the node names must be suffixed by a period
  and the datacenter (eg. "myserver.dc1").

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

## Output

If coordinates are available, the command will print the estimated round trip
time between the given nodes:

```
$ consul rtt n1 n2
Estimated n1 <-> n2 rtt: 0.610 ms (using LAN coordinates)

$ consul rtt n2 # Running from n1
Estimated n1 <-> n2 rtt: 0.610 ms (using LAN coordinates)

$ consul rtt -wan n1.dc1 n2.dc2
Estimated n1.dc1 <-> n2.dc2 rtt: 1.275 ms (using WAN coordinates)
```
