---
layout: "docs"
page_title: "Bootstrapping"
sidebar_current: "docs-guides-bootstrapping"
---

# Bootstrapping a Datacenter

When deploying Consul to a datacenter for the first time, there is an initial bootstrapping that
must be done. Generally, the first nodes that are started are the server nodes. Remember that an
agent can run in both client and server mode. Server nodes are responsible for running
the [consensus protocol](/docs/internals/consensus.html), and storing the cluster state.
The client nodes are mostly stateless and rely on the server nodes, so they can be started easily.

The first server that is deployed in a new datacenter must provide the `-bootstrap` [configuration
option](/docs/agent/options.html). This option allows the server to assert leadership of the cluster
without agreement from any other server. This is necessary because at this point, there are no other
servers running in the datacenter! Lets call this first server `Node A`. When starting `Node A` something
the following will be logged:

    2014/02/22 19:23:32 [INFO] consul: cluster leadership acquired

Once `Node A` is running, we can start the next set of servers. There is a [deployment table](/docs/internals/consensus.html)
that covers various options, but it is recommended to have 3 or 5 total servers per data center.
We start the next servers **without** specifying `-bootstrap`. This is critical, since only one server
should ever be running in bootstrap mode*. Once `Node B` and `Node C` are started, you should see a
message to the effect of:

    [WARN] raft: EnableSingleNode disabled, and no known peers. Aborting election.

This indicates that the node is not in bootstrap mode, and it will not elect itself as leader.
We can now join these machines together. Since a join operation is symmetric it does not matter
which node initiates it. From `Node B` and `Node C` you can do the following:

    $ consul join <Node A Address>
    Successfully joined cluster by contacting 1 nodes.

Alternatively, from `Node A` you can do the following:

    $ consul join <Node B Address> <Node C Address>
    Successfully joined cluster by contacting 2 nodes.

Once the join is successful, `Node A` should output something like:

    [INFO] raft: Added peer 127.0.0.2:8300, starting replication
    ....
    [INFO] raft: Added peer 127.0.0.3:8300, starting replication

This indicates that `Node B` and `Node C` have been added as peers. At this point,
all three nodes see each other as peers, `Node A` is the leader, and replication
should be working.

The final step is to remove the `-bootstrap` flag. This is important since we don't
want the node to be able to make unilateral decisions in the case of a failure of the
other two nodes. To do this, we send a `SIGINT` to `Node A` to allow it to perform
a graceful leave. Then we remove the `-bootstrap` flag and restart the node. The node
will need to rejoin the cluster, since the graceful exit leaves the cluster. Any transactions
that took place while `Node A` was offline will be replicated and the node will catch up.

Now that the servers are all started and replicating to each other, all the remaining
clients can be joined. Clients are much easier, as they can be started and perform
a `join` against any existing node. All nodes participate in a gossip protocol to
perform basic discovery, so clients will automatically find the servers and register
themselves.

<div class="alert alert-block alert-info">
* If you accidentally start another server with the flag set, do not fret.
Shutdown the node, and remove the `raft/` folder from the data directory. This will
remove the bad state caused by being in `-bootstrap` mode. Then restart the
node and join the cluster normally.
</div>

