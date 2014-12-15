---
layout: "docs"
page_title: "Commands: Leave"
sidebar_current: "docs-commands-leave"
description: |-
  The `leave` command triggers a graceful leave and shutdown of the agent. It is used to ensure other nodes see the agent as left instead of failed. Nodes that leave will not attempt to re-join the cluster on restarting with a snapshot.
---

# Consul Leave

Command: `consul leave`

The `leave` command triggers a graceful leave and shutdown of the agent.
It is used to ensure other nodes see the agent as "left" instead of
"failed". Nodes that leave will not attempt to re-join the cluster
on restarting with a snapshot.

For nodes in server mode, the node is removed from the Raft peer set
in a graceful manner. This is critical, as in certain situations a
non-graceful leave can affect cluster availability.

## Usage

Usage: `consul leave`

The command-line flags are all optional. The list of available flags are:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command checks the
  CONSUL_RPC_ADDR env variable. If this isn't set, the default RPC 
  address will be set to "127.0.0.1:8400". 

