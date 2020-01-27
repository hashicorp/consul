---
layout: "docs"
page_title: "Bootstrapping a Datacenter"
sidebar_current: "docs-install-bootstrapping"
description: |-
  An agent can run in both client and server mode. Server nodes are responsible for running the consensus protocol and storing the cluster state. Before a Consul cluster can begin to service requests, a server node must be elected leader. Thus, the first nodes that are started are generally the server nodes. Bootstrapping is the process of joining these server nodes into a cluster.
---

# Bootstrapping a Datacenter

An agent can run in either client or server mode. Server nodes are responsible for running the
[consensus protocol](/docs/internals/consensus.html) and storing the cluster state.
The client nodes are mostly stateless and rely heavily on the server nodes.

Before a Consul cluster can begin to service requests, 
a server node must be elected leader. Bootstrapping is the process
of joining these initial server nodes into a cluster. Read the 
[architecture documentation](/docs/internals/architecture.html) to learn more about 
the internals of Consul.

It is recommended to have three or five total servers per datacenter. A single server deployment is _highly_ discouraged
as data loss is inevitable in a failure scenario. Please refer to the
[deployment table](/docs/internals/consensus.html#deployment-table) for more detail.

~> **Note**: In versions of Consul prior to 0.4, bootstrapping was a manual process. For details on using the `-bootstrap` flag directly, see the
[manual bootstrapping documentation](/docs/install/bootstrapping.html#manually-join-the-servers).
Manual bootstrapping with `-bootstrap` is not recommended in 
newer versions of Consul (0.5 and newer) as it is more error-prone. 
Instead you should use automatic bootstrapping
with [`-bootstrap-expect`](/docs/agent/options.html#_bootstrap_expect).

## Bootstrapping the Servers

The recommended way to bootstrap the servers is to use the [`-bootstrap-expect`](/docs/agent/options.html#_bootstrap_expect)
configuration option. This option informs Consul of the expected number of
server nodes and automatically bootstraps when that many servers are available. To prevent
inconsistencies and split-brain (clusters where multiple servers consider
themselves leader) situations, you should either specify the same value for
[`-bootstrap-expect`](/docs/agent/options.html#_bootstrap_expect)
or specify no value at all on all the servers. Only servers that specify a value will attempt to bootstrap the cluster.

Suppose we are starting a three server cluster. We can start `Node A`, `Node B`, and `Node C` with each
providing the `-bootstrap-expect 3` flag. Once the nodes are started, you should see a warning message in the service output. 

```text
[WARN] raft: EnableSingleNode disabled, and no known peers. Aborting election.
```

The warning indicates that the nodes are expecting 2 peers but none are known yet. Below you will learn how to connect the servers so that one can be 
elected leader.

## Creating the Cluster

You can trigger leader election by joining the servers together, to create a cluster. You can either configure the nodes to join automatically or manually.

### Automatically Join the Servers

There are multiple options for joining the servers. Choose the method which best suits your environment and specific use case.

- Specify a list of servers with
  [-join](/docs/agent/options.html#_join) and
  [start_join](https://www.consul.io/docs/agent/options.html#start_join)
  options.
- Specify a list of servers with [-retry-join](https://www.consul.io/docs/agent/options.html#_retry_join) option.
- Use automatic joining by tag for supported cloud environments with the [-retry-join](https://www.consul.io/docs/agent/options.html#_retry_join) option.

All three methods can be set in the agent configuration file or 
the command line flag. 

### Manually Join the Servers

To manually create a cluster, you should connect to one of the servers
and run the `consul join` command.

```
$ consul join <Node A Address> <Node B Address> <Node C Address>
Successfully joined cluster by contacting 3 nodes.
```

Since a join operation is symmetric, it does not matter which node initiates it. Once the join is successful, one of the nodes will output something like:

```
[INFO] consul: adding server foo (Addr: 127.0.0.2:8300) (DC: dc1)
[INFO] consul: adding server bar (Addr: 127.0.0.1:8300) (DC: dc1)
[INFO] consul: Attempting bootstrap with nodes: [127.0.0.3:8300 127.0.0.2:8300 127.0.0.1:8300]
    ...
[INFO] consul: cluster leadership acquired
```

### Verifying the Cluster and Connect the Clients

As a sanity check, the [`consul info`](/docs/commands/info.html) command
is a useful tool. It can be used to verify the `raft.num_peers` 
and to view the latest log index under `raft.last_log_index`. When
running [`consul info`](/docs/commands/info.html) on the followers, you
should see `raft.last_log_index` converge to the same value once the
leader begins replication. That value represents the last log entry that
has been stored on disk.

Now that the servers are all started and replicating to each other, you can 
join the clients with the same join method you used for the servers. 
Clients are much easier as they can join against any existing node. All nodes participate in a gossip
protocol to perform basic discovery, so once joined to any member of the
cluster, new clients will automatically find the servers and register
themselves.

-> **Note:** It is not strictly necessary to start the server nodes before the clients; however, most operations will fail until the servers are available.



