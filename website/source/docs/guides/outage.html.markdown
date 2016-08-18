---
layout: "docs"
page_title: "Outage Recovery"
sidebar_current: "docs-guides-outage"
description: |-
  Don't panic! This is a critical first step. Depending on your deployment configuration, it may take only a single server failure for cluster unavailability. Recovery requires an operator to intervene, but recovery is straightforward.
---

# Outage Recovery

Don't panic! This is a critical first step.

Depending on your
[deployment configuration](/docs/internals/consensus.html#deployment_table), it
may take only a single server failure for cluster unavailability. Recovery
requires an operator to intervene, but the process is straightforward.

~>  This guide is for recovery from a Consul outage due to a majority
of server nodes in a datacenter being lost. If you are just looking to
add or remove a server, [see this guide](/docs/guides/servers.html).

## Failure of a Single Server Cluster

If you had only a single server and it has failed, simply restart it.
Note that a single server configuration requires the
[`-bootstrap`](/docs/agent/options.html#_bootstrap) or
[`-bootstrap-expect 1`](/docs/agent/options.html#_bootstrap_expect) flag. If
the server cannot be recovered, you need to bring up a new server.
See the [bootstrapping guide](/docs/guides/bootstrapping.html) for more detail.

In the case of an unrecoverable server failure in a single server cluster, data
loss is inevitable since data was not replicated to any other servers. This is
why a single server deploy is **never** recommended.

Any services registered with agents will be re-populated when the new server
comes online as agents perform [anti-entropy](/docs/internals/anti-entropy.html).

## Failure of a Server in a Multi-Server Cluster

If you think the failed server is recoverable, the easiest option is to bring
it back online and have it rejoin the cluster, returning the cluster to a fully
healthy state. Similarly, even if you need to rebuild a new Consul server to
replace the failed node, you may wish to do that immediately. Keep in mind that
the rebuilt server needs to have the same IP as the failed server. Again, once
this server is online, the cluster will return to a fully healthy state.

Both of these strategies involve a potentially lengthy time to reboot or rebuild
a failed server. If this is impractical or if building a new server with the same
IP isn't an option, you need to remove the failed server. Usually, you can issue
a [`force-leave`](/docs/commands/force-leave.html) command to remove the failed
server if it's still a member of the cluster.

If the `force-leave` isn't able to remove the server, you can remove it manually
using the `raft/peers.json` recovery file on all remaining servers.

To begin, stop all remaining servers. You can attempt a graceful leave,
but it will not work in most cases. Do not worry if the leave exits with an
error. The cluster is in an unhealthy state, so this is expected.

~> Note that in Consul 0.7 and later, the peers.json file is no longer present
by default and is only used when performing recovery. This file will be deleted
after Consul starts and ingests this file. Consul 0.7 also uses a new, automatically-
created `raft/peers.info` file to avoid ingesting the `raft/peers.json` file on the
first start after upgrading. Be sure to leave `raft/peers.info` in place for proper
operation.
<br>
<br>
Using `raft/peers.json` for recovery can cause uncommitted Raft log entries to be
implicitly committed, so this should only be used after an outage where no
other option is available to recover a lost server. Make sure you don't have
any automated processes that will put the peers file in place on a periodic basis,
for example.
<br>
<br>
When the final version of Consul 0.7 ships, it should include a command to
remove a dead peer without having to stop servers and edit the `raft/peers.json`
recovery file.

The next step is to go to the [`-data-dir`](/docs/agent/options.html#_data_dir)
of each Consul server. Inside that directory, there will be a `raft/`
sub-directory. We need to create a `raft/peers.json` file. It should look
something like:

```javascript
[
  "10.0.1.8:8300",
  "10.0.1.6:8300",
  "10.0.1.7:8300"
]
```

Simply create entries for all remaining servers. You must confirm
that servers you do not include here have indeed failed and will not later
rejoin the cluster. Ensure that this file is the same across all remaining
server nodes.

At this point, you can restart all the remaining servers. In Consul 0.7 and
later you will see them ingest recovery file:

```text
...
2016/08/16 14:39:20 [INFO] consul: found peers.json file, recovering Raft configuration...
2016/08/16 14:39:20 [INFO] consul.fsm: snapshot created in 12.484µs
2016/08/16 14:39:20 [INFO] snapshot: Creating new snapshot at /tmp/peers/raft/snapshots/2-5-1471383560779.tmp
2016/08/16 14:39:20 [INFO] consul: deleted peers.json file after successful recovery
2016/08/16 14:39:20 [INFO] raft: Restored from snapshot 2-5-1471383560779
2016/08/16 14:39:20 [INFO] raft: Initial configuration (index=1): [{Suffrage:Voter ID:10.212.15.121:8300 Address:10.212.15.121:8300}]
...
```

If any servers managed to perform a graceful leave, you may need to have them
rejoin the cluster using the [`join`](/docs/commands/join.html) command:

```text
$ consul join <Node Address>
Successfully joined cluster by contacting 1 nodes.
```

It should be noted that any existing member can be used to rejoin the cluster
as the gossip protocol will take care of discovering the server nodes.

At this point, the cluster should be in an operable state again. One of the
nodes should claim leadership and emit a log like:

```text
[INFO] consul: cluster leadership acquired
```

Additionally, the [`info`](/docs/commands/info.html) command can be a useful
debugging tool:

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
	num_peers = 2
	state = Leader
	term = 21
...
```

You should verify that one server claims to be the `Leader` and all the
others should be in the `Follower` state. All the nodes should agree on the
peer count as well. This count is (N-1), since a server does not count itself
as a peer.

## Failure of Multiple Servers in a Multi-Server Cluster

In the event that multiple servers are lost, causing a loss of quorum and a
complete outage, partial recovery is possible using data on the remaining
servers in the cluster. There may be data loss in this situation because multiple
servers were lost, so information about what's committed could be incomplete.
The recovery process implicitly commits all outstanding Raft log entries, so
it's also possible to commit data that was uncommitted before the failure.

The procedure is the same as for the single-server case above; you simply include
just the remaining servers in the `raft/peers.json` recovery file. The cluster
should be able to elect a leader once the remaining servers are all restarted with
an identical `raft/peers.json` configuration.

Any new servers you introduce later can be fresh with totally clean data directories
and joined using Consul's `join` command.

In extreme cases, it should be possible to recover with just a single remaining
server by starting that single server with itself as the only peer in the
`raft/peers.json` recovery file.

Note that prior to Consul 0.7 it wasn't always possible to recover from certain
types of outages with `raft/peers.json` because this was ingested before any Raft
log entries were played back. In Consul 0.7 and later, the `raft/peers.json`
recovery file is final, and a snapshot is taken after it is ingested, so you are
guaranteed to start with your recovered configuration. This does implicitly commit
all Raft log entries, so should only be used to recover from an outage, but it
should allow recovery from any situation where there's some cluster data available.
