---
layout: "docs"
page_title: "Commands: Reachability"
sidebar_current: "docs-commands-reachability"
description: |-
  The `reachability` command performs a basic network reachability test. The local node will gossip out a Serf  ping message and request that all other nodes acknowledge delivery of the message.
---

# Consul Reachability

Command: `consul reachability`

The `reachability` command performs a basic network reachability test.
The local node will gossip out a Serf "ping" message and request that
all other nodes acknowledge delivery of the message.

This can be used to troubleshoot configurations or network issues, since
nodes that are detected as having failed may respond, indicating false-failure
detection, or live nodes may fail to respond, indicating networking issues.

In general, the following troubleshooting tips are recommended:

* Ensure that the bind addr:port is accessible by all other nodes
* If an advertise address is set, ensure it routes to the bind address
* Check that no nodes are behind a NAT
* If nodes are behind firewalls or iptables, check that Serf traffic is permitted (UDP and TCP)
* Verify networking equipment is functional

## Usage

Usage: `consul reachability [options]`

The following command-line options are available for this command.
Every option is optional:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8400" which is the default RPC address of a Consul
  agent. This option can also be controlled using the `CONSUL_RPC_ADDR`
  environment variable.

* `-verbose` - Enables verbose output
