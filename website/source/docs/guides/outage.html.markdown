---
layout: "docs"
page_title: "Outage Recovery"
sidebar_current: "docs-guides-outage"
description: |-
  Do not panic! This is a critical first step. Depending on your deployment configuration, it may take only a single server failure for cluster unavailability. Recovery requires an operator to intervene, but is straightforward.
---

# Outage Recovery

Do not panic! This is a critical first step. Depending on your
[deployment configuration](/docs/internals/consensus.html#toc_4), it may
take only a single server failure for cluster unavailability. Recovery
requires an operator to intervene, but is straightforward.

~>  This page covers recovery from Consul becoming unavailable due to a majority
of server nodes in a datacenter being lost. If you are just looking to
add or remove a server [see this page](/docs/guides/servers.html).

If you had only a single server and it has failed, simply restart it.
Note that a single server configuration requires the `-bootstrap` or
`-bootstrap-expect 1` flag. If that server cannot be recovered, you need to
bring up a new server.
See the [bootstrapping guide](/docs/guides/bootstrapping.html). Data loss
is inevitable, since data was not replicated to any other servers. This
is why a single server deploy is never recommended. Any services registered
with agents will be re-populated when the new server comes online, as
agents perform anti-entropy.

In a multi-server deploy, there are at least N remaining servers. The first step
is to simply stop all the servers. You can attempt a graceful leave, but
it will not work in most cases. Do not worry if the leave exits with an
error, since the cluster is in an unhealthy state.

The next step is to go to the `-data-dir` of each Consul server. Inside
that directory, there will be a `raft/` sub-directory. We need to edit
the `raft/peers.json` file. It should be something like:

```javascript
[
  "10.0.1.8:8300",
  "10.0.1.6:8300",
  "10.0.1.7:8300"
]
```

Simply delete the entries for all the failed servers. You must confirm
those servers have indeed failed, and will not later rejoin the cluster.
Ensure that this file is the same across all remaining server nodes.

At this point, you can restart all the remaining servers. If any servers
managed to perform a graceful leave, you may need to have then rejoin
the cluster using the `join` command:

```text
$ consul join <Node Address>
Successfully joined cluster by contacting 1 nodes.
```

It should be noted that any existing member can be used to rejoin the cluster,
as the gossip protocol will take care of discovering the server nodes.

At this point the cluster should be in an operable state again. One of the
nodes should claim leadership and emit a log like:

```text
[INFO] consul: cluster leadership acquired
```

Additional, the `info` command can be a useful debugging tool:

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

You should verify that one server claims to be the `Leader`, and all the
others should be in the `Follower` state. All the nodes should agree on the
peer count as well. This count is (N-1), since a server does not count itself
as a peer.
