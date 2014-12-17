---
layout: "docs"
page_title: "Commands: Members"
sidebar_current: "docs-commands-members"
description: |-
  The `members` command outputs the current list of members that a Consul agent knows about, along with their state. The state of a node can only be alive, left, or failed.
---

# Consul Members

Command: `consul members`

The `members` command outputs the current list of members that a Consul
agent knows about, along with their state. The state of a node can only
be "alive", "left", or "failed".

Nodes in the "failed" state are still listed because Consul attempts to
reconnect with failed nodes for a certain amount of time in the case
that the failure is actually just a network partition.

## Usage

Usage: `consul members [options]`

The command-line flags are all optional. The list of available flags are:

* `-detailed` - If provided, output shows more detailed information
  about each node.

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command.If this isn't specified, the command checks the
  CONSUL_RPC_ADDR env variable. If this isn't set, the default RPC 
  address will be set to "127.0.0.1:8400".

* `-status` - If provided, output is filtered to only nodes matching
  the regular expression for status

* `-wan` - For agents in Server mode, this will return the list of nodes
  in the WAN gossip pool. These are generally all the server nodes in
  each datacenter.

