---
layout: "docs"
page_title: "Commands: Info"
sidebar_current: "docs-commands-info"
---

# Consul Info

Command: `consul info`

The info command provides various debugging information that can be
useful to operators. Depending on if the agent is a client or server,
information about different sub-systems will be returned.

Here is an example output:

    agent:
        check_monitors = 0
        check_ttls = 0
        checks = 0
        services = 0
    consul:
        bootstrap = true
        known_datacenters = 1
        leader = true
        server = true
    raft:
        applied_index = 45824
        commit_index = 45824
        fsm_pending = 0
        last_log_index = 45824
        last_log_term = 4
        last_snapshot_index = 45713
        last_snapshot_term = 1
        num_peers = 0
        state = Leader
        term = 4
    serf-lan:
        event-queue = 1
        event-time = 2
        failed = 0
        intent-queue = 0
        left = 0
        member-time = 1
        members = 1
    serf-wan:
        event-queue = 0
        event-time = 1
        failed = 0
        intent-queue = 0
        left = 0
        member-time = 1
        members = 1

There are currently the top-level keys for:

* agent: Provides information about the agent
* consul: Information about the consul library (client or server)
* raft: Provides info about the Raft [consensus library](/docs/internals/consensus.html)
* serf-lan: Provides info about the LAN [gossip pool](/docs/internals/gossip.html)
* serf-wan: Provides info about the WAN [gossip pool](/docs/internals/gossip.html)

## Usage

Usage: `consul info`

The command-line flags are all optional. The list of available flags are:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8400" which is the default RPC address of a Consul agent.

