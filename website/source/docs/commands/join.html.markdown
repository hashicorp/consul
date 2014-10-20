---
layout: "docs"
page_title: "Commands: Join"
sidebar_current: "docs-commands-join"
description: |-
  The `join` command tells a Consul agent to join an existing cluster. A new Consul agent must join with at least one existing member of a cluster in order to join an existing cluster. After joining that one member, the gossip layer takes over, propagating the updated membership state across the cluster.
---

# Consul Join

Command: `consul join`

The `join` command tells a Consul agent to join an existing cluster.
A new Consul agent must join with at least one existing member of a cluster
in order to join an existing cluster. After joining that one member,
the gossip layer takes over, propagating the updated membership state across
the cluster.

If you don't join an existing cluster, then that agent is part of its own
isolated cluster. Other nodes can join it.

Agents can join other agents multiple times without issue. If a node that
is already part of a cluster joins another node, then the clusters of the
two nodes join to become a single cluster.

## Usage

Usage: `consul join [options] address ...`

You may call join with multiple addresses if you want to try to join
multiple clusters. Consul will attempt to join all addresses, and the join
command will fail only if Consul was unable to join with any.

The command-line flags are all optional. The list of available flags are:

* `-wan` - For agents running in server mode, the agent will attempt to join
  other servers gossiping in a WAN cluster. This is used to form a bridge between
  multiple datacenters.

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8400" which is the default RPC address of a Consul agent.

