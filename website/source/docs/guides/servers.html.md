---
layout: "docs"
page_title: "Adding & Removing Servers"
sidebar_current: "docs-guides-servers"
description: |-
  Consul is designed to require minimal operator involvement, however any changes to the set of Consul servers must be handled carefully. To better understand why, reading about the consensus protocol will be useful. In short, the Consul servers perform leader election and replication. For changes to be processed, a minimum quorum of servers (N/2)+1 must be available. That means if there are 3 server nodes, at least 2 must be available.
---

# Adding & Removing Servers

Consul is designed to require minimal operator involvement, however any changes
to the set of Consul servers must be handled carefully. To better understand
why, reading about the [consensus protocol](/docs/internals/consensus.html) will
be useful. In short, the Consul servers perform leader election and replication.
For changes to be processed, a minimum quorum of servers (N/2)+1 must be available.
That means if there are 3 server nodes, at least 2 must be available.

In general, if you are ever adding and removing nodes simultaneously, it is better
to first add the new nodes and then remove the old nodes.

In this guide, we will cover the different methods for adding and removing servers.

## Manually Add a New Server

Manually adding new servers is generally straightforward, start the new
agent with the `-server` flag. At this point the server will not be a member of
any cluster, and should emit something like:

```sh
consul agent -server
[WARN] raft: EnableSingleNode disabled, and no known peers. Aborting election.
```

This means that it does not know about any peers and is not configured to elect itself.
This is expected, and we can now add this node to the existing cluster using `join`.
From the new server, we can join any member of the existing cluster:

```sh
$ consul join <Existing Node Address>
Successfully joined cluster by contacting 1 nodes.
```

It is important to note that any node, including a non-server may be specified for
join. Generally, this method is good for testing purposes but not recommended for production
deployments. For production clusters, you will likely want to use the agent configuration
option to add additional servers.

## Add a Server with Agent Configuration

In production environments, you should use the [agent configuration](https://www.consul.io/docs/agent/options.html) option, `retry_join`. `retry_join` can be used as a command line flag or in the agent configuration file. 

With the Consul CLI:

```sh
$ consul agent -retry-join=["52.10.110.11", "52.10.110.12", "52.10.100.13"]
```

In the agent configuration file:

```sh
{
  "bootstrap": false,
  "bootstrap_expect": 3,
  "server": true,
  "retry_join": ["52.10.110.11", "52.10.110.12", "52.10.100.13"]
}
```

[`retry_join`](https://www.consul.io/docs/agent/options.html#retry-join)
will ensure that if any server loses connection
with the cluster for any reason, including the node restarting, it can
rejoin when it comes back. In additon to working with static IPs, it 
can also be  useful for other discovery mechanisms, such as auto joining 
based on cloud metadata and discovery. Both servers and clients can use this method.

### Server Coordination

To ensure Consul servers are joining the cluster properly, you should monitor
the server coordination. The gossip protocol is used to properly discover all
the nodes in the cluster. Once the node has joined, the existing cluster
leader should log something like:

```text
[INFO] raft: Added peer 127.0.0.2:8300, starting replication
```

This means that raft, the underlying consensus protocol, has added the peer and begun
replicating state. Since the existing cluster may be very far ahead, it can take some
time for the new node to catch up. To check on this, run `info` on the leader:

```text
$ consul info
...
raft:
	applied_index = 47244
	commit_index = 47244
	fsm_pending = 0
	last_log_index = 47244
	last_log_term = 21
	last_snapshot_index = 40966
	last_snapshot_term = 20
	num_peers = 4
	state = Leader
	term = 21
...
```

This will provide various information about the state of Raft. In particular
the `last_log_index` shows the last log that is on disk. The same `info` command
can be run on the new server to see how far behind it is. Eventually the server
will be caught up, and the values should match.

It is best to add servers one at a time, allowing them to catch up. This avoids
the possibility of data loss in case the existing servers fail while bringing
the new servers up-to-date.

## Manually Remove a Server

Removing servers must be done carefully to avoid causing an availability outage.
For a cluster of N servers, at least (N/2)+1 must be available for the cluster
to function. See this [deployment table](/docs/internals/consensus.html#toc_4).
If you have 3 servers and 1 of them is currently failing, removing any other servers
will cause the cluster to become unavailable.

To avoid this, it may be necessary to first add new servers to the cluster,
increasing the failure tolerance of the cluster, and then to remove old servers.
Even if all 3 nodes are functioning, removing one leaves the cluster in a state
that cannot tolerate the failure of any node.

Once you have verified the existing servers are healthy, and that the cluster
can handle a node leaving, the actual process is simple. You simply issue a
`leave` command to the server.

```sh
consul leave
```

The server leaving should contain logs like:

```text
...
[INFO] consul: server starting leave
...
[INFO] raft: Removed ourself, transitioning to follower
...
```

The leader should also emit various logs including:

```text
...
[INFO] consul: member 'node-10-0-1-8' left, deregistering
[INFO] raft: Removed peer 10.0.1.8:8300, stopping replication
...
```

At this point the node has been gracefully removed from the cluster, and
will shut down.

~> Running `consul leave` on a server explicitly will reduce the quorum size. Even if the cluster used `bootstrap_expect` to set a quorum size initially, issuing `consul leave` on a server will reconfigure the cluster to have fewer servers. This means you could end up with just one server that is still able to commit writes because quorum is only 1, but those writes might be lost if that server fails before more are added.

To remove all agents that accidentally joined the wrong set of servers, clear out the contents of the data directory (`-data-dir`) on both client and server nodes.

These graceful methods to remove servres assumse you have a healthly cluster. 
If the cluster has no leader due to loss of quorum or data corruption, you should 
plan for [outage recovery](/docs/guides/outage.html#manual-recovery-using-peers-json).

!> **WARNING** Removing data on server nodes will destroy all state in the cluster

## Manual Forced Removal

In some cases, it may not be possible to gracefully remove a server. For example,
if the server simply fails, then there is no ability to issue a leave. Instead,
the cluster will detect the failure and replication will continuously retry.

If the server can be recovered, it is best to bring it back online and then gracefully
leave the cluster. However, if this is not a possibility, then the `force-leave` command
can be used to force removal of a server.

```sh
consul force-leave <node>
```

This is done by invoking that command with the name of the failed node. At this point,
the cluster leader will mark the node as having left the cluster and it will stop attempting to replicate.

## Summary

In this guide we learned the straightforward process of adding and removing servers including;
manually adding servers, adding servers through the agent configuration, gracefully removing
servers, and forcing removal of servers. Finally, we should restate that manually adding servers
 is good for testing purposes, however, for production it is recommended to add servers with
the agent configuration.
