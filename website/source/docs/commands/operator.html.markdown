---
layout: "docs"
page_title: "Commands: Operator"
sidebar_current: "docs-commands-operator"
description: >
  The operator command provides cluster-level tools for Consul operators.
---

# Consul Operator

Command: `consul operator`

The `operator` command provides cluster-level tools for Consul operators, such
as interacting with the Raft subsystem. This was added in Consul 0.7.

~> Use this command with extreme caution, as improper use could lead to a Consul
   outage and even loss of data.

If ACLs are enabled then a token with operator privileges may required in
order to use this command. Requests are forwarded internally to the leader
if required, so this can be run from any Consul node in a cluster. See the
[ACL](/docs/internals/acl.html#operator) internals guide for more information.

See the [Outage Recovery](/docs/guides/outage.html) guide for some examples of how
this command is used. For an API to perform these operations programatically,
please see the documentation for the [Operator](/docs/agent/http/operator.html)
endpoint.

## Usage

Usage: `consul operator <subcommand> [common options] [action] [options]`

Run `consul operator <subcommand>` with no arguments for help on that
subcommand. The following subcommands are available:

* `raft` - View and modify Consul's Raft configuration.

Options common to all subcommands include:

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

* `-token` - ACL token to use. Defaults to that of agent.

## Raft Operations

The `raft` subcommand is used to view and modify Consul's Raft configuration.
Two actions are available, as detailed in this section.

<a name="raft-list-peers"></a>
#### Display Peer Configuration
This action displays the current Raft peer configuration.

Usage: `raft -list-peers -stale=[true|false]`

* `-stale` - Optional and defaults to "false" which means the leader provides
the result. If the cluster is in an outage state without a leader, you may need
to set this to "true" to get the configuration from a non-leader server.

The output looks like this:

```
Node     ID              Address         State     Voter
alice    127.0.0.1:8300  127.0.0.1:8300  follower  true
bob      127.0.0.2:8300  127.0.0.2:8300  leader    true
carol    127.0.0.3:8300  127.0.0.3:8300  follower  true
```

`Node` is the node name of the server, as known to Consul, or "(unknown)" if
the node is stale and not known.

`ID` is the ID of the server. This is the same as the `Address` in Consul 0.7
but may  be upgraded to a GUID in a future version of Consul.

`Address` is the IP:port for the server.

`State` is either "follower" or "leader" depending on the server's role in the
Raft configuration.

`Voter` is "true" or "false", indicating if the server has a vote in the Raft
configuration. Future versions of Consul may add support for non-voting servers.

<a name="raft-remove-peer"></a>
#### Remove a Peer
This command removes Consul server with given address from the Raft configuration.

There are rare cases where a peer may be left behind in the Raft configuration
even though the server is no longer present and known to the cluster. This command
can be used to remove the failed server so that it is no longer affects the
Raft quorum. If the server still shows in the output of the
[`consul members`](/docs/commands/members.html) command, it is preferable to
clean up by simply running
[`consul force-leave`](/docs/commands/force-leave.html)
instead of this command.

Usage: `raft -remove-peer -address="IP:port"`

* `-address` - "IP:port" for the server to remove. The port number is usually
8300, unless configured otherwise.

The return code will indicate success or failure.
