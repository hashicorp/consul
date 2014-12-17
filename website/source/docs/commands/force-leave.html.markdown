---
layout: "docs"
page_title: "Commands: Force Leave"
sidebar_current: "docs-commands-forceleave"
description: |-
  The `force-leave` command forces a member of a Consul cluster to enter the left state. Note that if the member is still actually alive, it will eventually rejoin the cluster. The true purpose of this method is to force remove failed nodes.
---

# Consul Force Leave

Command: `consul force-leave`

The `force-leave` command forces a member of a Consul cluster to enter the
"left" state. Note that if the member is still actually alive, it will
eventually rejoin the cluster. The true purpose of this method is to force
remove "failed" nodes.

Consul periodically tries to reconnect to "failed" nodes in case it is a
network partition. After some configured amount of time (by default 72 hours),
Consul will reap "failed" nodes and stop trying to reconnect. The `force-leave`
command can be used to transition the "failed" nodes to "left" nodes more
quickly.

This can be particularly useful for a node that was running as a server,
as it will be removed from the Raft quorum.

## Usage

Usage: `consul force-leave [options] node`

The following command-line options are available for this command.
Every option is optional:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command checks the
  CONSUL_RPC_ADDR env variable. If this isn't set, the default RPC 
  address will be set to "127.0.0.1:8400". 

