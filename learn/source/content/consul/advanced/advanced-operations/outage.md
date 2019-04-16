---
name: 'Outage Recovery'
content_length: 15
id: /advanced-operations/outage
layout: content_layout
products_used:
  - Consul
description: Outage recovery requires an operator to intervene, however the recovery process is straightforward.
level: Advanced
---

Don't panic! This is a critical first step.

Depending on your [deployment
configuration](https://www.consul.io/docs/internals/consensus.html#deployment_table),
it may take only a single server failure for cluster unavailability. Recovery
requires an operator to intervene, but the process is straightforward.

This guide outlines the processes for recovering from a Consul outage due to a
majority of server nodes in a cluster being lost. There are several types of
outages, depending on the number of server nodes and number of failed server
nodes. We will outline how to recover from:

- Failure of a Single Server Cluster. This is when you have a single Consul
  server and it fails.
- Failure of a Server in a Multi-Server Cluster. This is when one server fails,
  but the Consul cluster has three or more servers.
- Failure of Multiple Servers in a Multi-Server Cluster. This when more than
  one server fails in a cluster of three or more servers. This scenario is
  potentially the most serious, because it can result in data loss.

## Failure of a Single Server Cluster

If you had only a single server and it has failed, simply restart it. A single
server configuration requires the
[`-bootstrap`](https://www.consul.io/docs/agent/options.html#_bootstrap) or
[`-bootstrap-expect=1`](https://www.consul.io/docs/agent/options.html#_bootstrap_expect)
flag.

```sh
consul agent -bootstrap-expect=1
```

If the server cannot be recovered, you need to bring up a new server using the
[deployment guide](https://www.consul.io/docs/guides/deployment-guide.html).

In the case of an unrecoverable server failure in a single server cluster and
there is no backup procedure, data loss is inevitable since data was not
replicated to any other servers. This is why a single server deploy is
**never** recommended.

Any services registered with agents will be re-populated when the new server
comes online as agents perform
[anti-entropy](https://www.consul.io/docs/internals/anti-entropy.html).

### Failure of a Single Server Cluster: After Upgrading Raft Protocol

After upgrading the server's
[-raft-protocol](https://www.consul.io/docs/agent/options.html#_raft_protocol)
from version `2` to `3`, server's running Raft protocol version 3 will no
longer allow servers running an older version to be added.

In a single server cluster scenario, restarting the server may result in the
server not being able to elect itself as a leader, rendering the cluster
unusable.

To recover from this, go to the
[`-data-dir`](https://www.consul.io/docs/agent/options.html#_data_dir) of the failing
Consul server. Inside the data directory there will be a `raft/`
sub-directory. Create a `raft/peers.json` file in the `raft/` directory.

For Raft protocol version 3 and later, this should be formatted as a JSON array
containing the node ID, address:port, and suffrage information of the Consul
server, like this:

```json
[{ "id": "<node-id>", "address": "<node-ip>:8300", "non_voter": false }]
```

Make sure you replace `node-id` and `node-ip` with the correct value for your
Consul server.

Finally, restart your Consul server.

## Failure of a Server in a Multi-Server Cluster

If the failed server is recoverable, the best option is to bring it back online
and have it rejoin the cluster with the same IP address. This will return the
cluster to a fully healthy state. Similarly, if you need to rebuild a new
Consul server, to replace the failed node, you may wish to do that immediately.
Note, the rebuilt server needs to have the same IP address as the failed
server. Again, once this server is online and has rejoined, the cluster will
return to a fully healthy state.

```sh
consul agent -bootstrap-expect=3 -bind=192.172.2.4 -auto-rejoin=192.172.2.3
```

Both of these strategies involve a potentially lengthy time to reboot or
rebuild a failed server. If this is impractical or if building a new server
with the same IP isn't an option, you need to remove the failed server.
Usually, you can issue a [`consul force-leave`](https://www.consul.io/docs/commands/force-leave.html) command to
remove the failed server if it's still a member of the cluster.

```sh
consul force-leave <node.name.consul>
```

If [`consul force-leave`](https://www.consul.io/docs/commands/force-leave.html)
isn't able to remove the server, you have two methods available to remove it,
depending on your version of Consul:

- In Consul 0.7 and later, you can use the [`consul operator`](https://www.consul.io/docs/commands/operator.html#raft-remove-peer) command to remove the stale peer server on the fly with no downtime if the cluster has a leader.

- In versions of Consul prior to 0.7, you can manually remove the stale peer
  server using the `raft/peers.json` recovery file on all remaining servers.
  See the [section below](#peers.json) for details on this procedure. This
  process requires a Consul downtime to complete.

In Consul 0.7 and later, you can use the [`consul operator`](https://www.consul.io/docs/commands/operator.html#raft-list-peers)
command to inspect the Raft configuration:

```sh
$ consul operator raft list-peers
Node     ID              Address         State     Voter RaftProtocol
alice    10.0.1.8:8300   10.0.1.8:8300   follower  true  3
bob      10.0.1.6:8300   10.0.1.6:8300   leader    true  3
carol    10.0.1.7:8300   10.0.1.7:8300   follower  true  3
```

## Failure of Multiple Servers in a Multi-Server Cluster

In the event that multiple servers are lost, causing a loss of quorum and a
complete outage, partial recovery is possible using data on the remaining
servers in the cluster. There may be data loss in this situation because
multiple servers were lost, so information about what's committed could be
incomplete. The recovery process implicitly commits all outstanding Raft log
entries, so it's also possible to commit data that was uncommitted before the
failure.

See the section below on manual recovery using peers.json for details of the
recovery procedure. You simply include just the remaining servers in the
`raft/peers.json` recovery file. The cluster should be able to elect a leader
once the remaining servers are all restarted with an identical
`raft/peers.json` configuration.

Any new servers you introduce later can be fresh with totally clean data
directories and joined using Consul's `join` command.

```sh
consul agent -join=192.172.2.3
```

In extreme cases, it should be possible to recover with just a single remaining
server by starting that single server with itself as the only peer in the
`raft/peers.json` recovery file.

Prior to Consul 0.7 it wasn't always possible to recover from certain types of
outages with `raft/peers.json` because this was ingested before any Raft log
entries were played back. In Consul 0.7 and later, the `raft/peers.json`
recovery file is final, and a snapshot is taken after it is ingested, so you
are guaranteed to start with your recovered configuration. This does implicitly
commit all Raft log entries, so should only be used to recover from an outage,
but it should allow recovery from any situation where there's some cluster data
available.

### Manual Recovery Using peers.json

To begin, stop all remaining servers. You can attempt a graceful leave, but it
will not work in most cases. Do not worry if the leave exits with an error. The
cluster is in an unhealthy state, so this is expected.

In Consul 0.7 and later, the `peers.json` file is no longer present by default
and is only used when performing recovery. This file will be deleted after
Consul starts and ingests this file. Consul 0.7 also uses a new, automatically-
created `raft/peers.info` file to avoid ingesting the `raft/peers.json` file on
the first start after upgrading. Be sure to leave `raft/peers.info` in place
for proper operation.

Using `raft/peers.json` for recovery can cause uncommitted Raft log entries to
be implicitly committed, so this should only be used after an outage where no
other option is available to recover a lost server. Make sure you don't have
any automated processes that will put the peers file in place on a periodic
basis.

The next step is to go to the
[`-data-dir`](https://www.consul.io/docs/agent/options.html#_data_dir) of each
Consul server. Inside that directory, there will be a `raft/` sub-directory. We
need to create a `raft/peers.json` file. The format of this file depends on
what the server has configured for its [Raft
protocol](https://www.consul.io/docs/agent/options.html#_raft_protocol)
version.

For Raft protocol version 2 and earlier, this should be formatted as a JSON
array containing the address and port of each Consul server in the cluster,
like this:

```json
["10.1.0.1:8300", "10.1.0.2:8300", "10.1.0.3:8300"]
```

For Raft protocol version 3 and later, this should be formatted as a JSON array
containing the node ID, address:port, and suffrage information of each Consul
server in the cluster, like this:

```json
[
  {
    "id": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "address": "10.1.0.1:8300",
    "non_voter": false
  },
  {
    "id": "8b6dda82-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.2:8300",
    "non_voter": false
  },
  {
    "id": "97e17742-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.3:8300",
    "non_voter": false
  }
]
```

- `id` `(string: <required>)` - Specifies the [node
  ID](https://www.consul.io/docs/agent/options.html#_node_id) of the server.
  This can be found in the logs when the server starts up if it was
  auto-generated, and it can also be found inside the `node-id` file in the
  server's data directory.

- `address` `(string: <required>)` - Specifies the IP and port of the server.
  The port is the server's RPC port used for cluster communications.

- `non_voter` `(bool: <false>)` - This controls whether the server is a
  non-voter, which is used in some advanced
  [Autopilot](/consul/day-2-operations/advanced-operations/autopilot)
  configurations. If omitted, it will default to false, which is typical for most
  clusters.

Simply create entries for all servers. You must confirm that servers you do not
include here have indeed failed and will not later rejoin the cluster. Ensure
that this file is the same across all remaining server nodes.

At this point, you can restart all the remaining servers. In Consul 0.7 and
later you will see them ingest recovery file:

```text
...
2016/08/16 14:39:20 [INFO] consul: found peers.json file,
recovering Raft configuration...  2016/08/16 14:39:20 [INFO] consul.fsm:
snapshot created in 12.484µs 2016/08/16 14:39:20 [INFO] snapshot: Creating new
snapshot at /tmp/peers/raft/snapshots/2-5-1471383560779.tmp 2016/08/16 14:39:20
[INFO] consul: deleted peers.json file after successful recovery 2016/08/16
14:39:20 [INFO] raft: Restored from snapshot 2-5-1471383560779 2016/08/16
14:39:20 [INFO] raft: Initial configuration (index=1): [{Suffrage:Voter
ID:10.212.15.121:8300 Address:10.212.15.121:8300}] ...
```

If any servers managed to perform a graceful leave, you may need to have them
rejoin the cluster using the
[`join`](https://www.consul.io/docs/commands/join.html) command:

```sh
$ consul join <Node Address> Successfully joined cluster by contacting
1 nodes.
```

It should be noted that any existing member can be used to rejoin the cluster
as the gossip protocol will take care of discovering the server nodes.

At this point, the cluster should be in an operable state again. One of the
nodes should claim leadership and emit a log like:

```text
[INFO] consul: cluster leadership acquired
```

In Consul 0.7 and later, you can use the [`consul operator`](https://www.consul.io/docs/commands/operator.html#raft-list-peers)
command to inspect the Raft configuration:

```sh
$ consul operator raft list-peers Node     ID              Address
State     Voter  RaftProtocol alice    10.0.1.8:8300   10.0.1.8:8300   follower
true   3 bob      10.0.1.6:8300   10.0.1.6:8300   leader    true   3 carol
10.0.1.7:8300   10.0.1.7:8300   follower  true   3
```

## Summary

In this guide, we reviewed how to recover from a Consul server outage.
Depending on the quorum size and number of failed servers, the recovery process
will vary. In the event of complete failure it is beneficial to have a [backup
process](/consul/advanced/day-1-operations/backup).
