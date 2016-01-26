---
layout: "docs"
page_title: "Commands: RTT"
sidebar_current: "docs-commands-rtt"
description: >
  The rtt command estimates the network round trip time between two nodes.
---

# Consul RTT

Command: `consul rtt`

The `rtt` command estimates the network round trip time between two nodes using
Consul's network coordinate model of the cluster.

See the [Network Coordinates](/docs/internals/coordinates.html) internals guide
for more information on how these coordinates are computed.

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
  and the datacenter (eg. "myserver.dc1"). It is not possible to measure between
  LAN coordinates and WAN coordinates, so both nodes must be in the same pool.


* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

The following environment variables control accessing the HTTP server via SSL:

* `CONSUL_HTTP_SSL` Set this to enable SSL
* `CONSUL_HTTP_SSL_VERIFY` Set this to disable certificate checking (not recommended)

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
