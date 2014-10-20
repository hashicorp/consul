---
layout: "docs"
page_title: "Commands: Info"
sidebar_current: "docs-commands-info"
description: |-
  The `info` command provides various debugging information that can be useful to operators. Depending on if the agent is a client or server, information about different sub-systems will be returned.
---

# Consul Info

Command: `consul info`

The `info` command provides various debugging information that can be
useful to operators. Depending on if the agent is a client or server,
information about different sub-systems will be returned.

There are currently the top-level keys for:

* agent: Provides information about the agent
* consul: Information about the consul library (client or server)
* raft: Provides info about the Raft [consensus library](/docs/internals/consensus.html)
* serf_lan: Provides info about the LAN [gossip pool](/docs/internals/gossip.html)
* serf_wan: Provides info about the WAN [gossip pool](/docs/internals/gossip.html)

Here is an example output:

```text
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
    applied_index = 45832
    commit_index = 45832
    fsm_pending = 0
    last_log_index = 45832
    last_log_term = 4
    last_snapshot_index = 45713
    last_snapshot_term = 1
    num_peers = 2
    state = Leader
    term = 4
serf_lan:
    event_queue = 0
    event_time = 2
    failed = 0
    intent_queue = 0
    left = 0
    member_time = 7
    members = 3
    query_queue = 0
    query_time = 1
serf_wan:
    event_queue = 0
    event_time = 1
    failed = 0
    intent_queue = 0
    left = 0
    member_time = 1
    members = 1
    query_queue = 0
    query_time = 1
```

## Usage

Usage: `consul info`

The command-line flags are all optional. The list of available flags are:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8400" which is the default RPC address of a Consul agent.

