---
layout: "docs"
page_title: "Commands: Members"
sidebar_current: "docs-commands-members"
---

# Serf Members

Command: `serf members`

The members command outputs the current list of members that a Serf
agent knows about, along with their state. The state of a node can only
be "alive" or "failed".

Nodes in the "failed" state are still listed because Serf attempts to
reconnect with failed nodes for a certain amount of time in the case
that the failure is actually just a network partition.

## Usage

Usage: `serf members [options]`

The command-line flags are all optional. The list of available flags are:

* `-detailed` - Will show additional information per member, such as the
  protocol version that each can understand and that each is speaking.

* `-format` - Controls the output format. Supports `text` and `json`.
  The default format is `text`.

* `-role` - If provided, output is filtered to only nodes matching
  the regular expression for role

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:7373" which is the default RPC address of a Serf agent.

* `-status` - If provided, output is filtered to only nodes matching
  the regular expression for status

