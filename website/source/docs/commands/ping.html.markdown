---
layout: "docs"
page_title: "Commands: Ping"
sidebar_current: "docs-commands-ping"
description: |-
  The `ping` command issues Serf 'ping' messages to specified node and displays response statistics.
---

# Consul Ping

Command: `consul ping`

The `ping` command issues Serf 'ping' messages to the specified node.
If a count is specified, the test will issue the specified number of
packets to the node; if no count is specified, the test will continue
indefinitely until interrupted by CTRL-C.  Once the test is finished
(or interrupted), it displays statistics describing the number of
packets sent, the number of responses received, the percentage of
successful 'pings', and the average latency or round trip time of all
successful responses.

This can be used to troubleshoot configurations or network issues, since
nodes that are having issues responding to UDP traffic will either
intermittently respond or not respond at all.

In general, the following troubleshooting tips are recommended:

* Ensure that the bind addr:port is accessible by all other nodes
* If an advertise address is set, ensure it routes to the bind address
* Check that no nodes are behind a NAT
* If nodes are behind firewalls or iptables, check that Serf traffic is permitted (UDP and TCP)
* Verify networking equipment is functional

## Usage

Usage: `consul ping [options]`

The following command-line options are available for this command.
Every option is optional:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8400" which is the default RPC address of a Consul
  agent. This option can also be controlled using the `CONSUL_RPC_ADDR`
  environment variable.

* `-node` - Name of the node to ping

* `-count` - Number of pings to issue to target node.  If not
  specified, continue issuing pings until interrupted by CTRL-C.

## Output

If the node exists in the data center, the specified number of ping
messages will be sent and statistics displayed upon completion:

```
$ ./consul ping -node test-node-1 -count 5
Starting serf ping...
Count 0: Node test-node-1 responded in 785.244µs
Count 1: Node test-node-1 responded in 871.454µs
Count 2: Node test-node-1 responded in 714.955µs
Count 3: Node test-node-1 responded in 6.008223ms
Count 4: Node test-node-1 responded in 884.21µs
Statistics: total 5, success 100%, avg latency: 1.852817ms
```

If the node does not respong to all packets, the output indicates the
timeout value of ping messages for which no response was received:

```
$ ./consul ping -node test-node-5 -count 5
Starting serf ping...
Count 0: Node test-node-5 responded in 858.761µs
Count 1: Node test-node-5 responded in 862.887µs
Count 2: Node test-node-5 responded in 857.277µs
Count 3: Node test-node-5 failed to respond in 500ms
Count 4: Node test-node-5 responded in 953.343µs
Statistics: total 5, success 80%, avg latency: 883.067µs
```

If the node is not a member of the data center, an error will be
displayed:

```
$ ./consul ping -node nodeDoesNotExist
Starting serf ping...
Error sending serf ping: Member nodeDoesNotExist not found in data center.
```
