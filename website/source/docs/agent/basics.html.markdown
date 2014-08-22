---
layout: "docs"
page_title: "Agent"
sidebar_current: "docs-agent-running"
---

# Consul Agent

The Consul agent is the core process of Consul. The agent maintains membership
information, registers services, runs checks, responds to queries
and more. The agent must run on every node that is part of a Consul cluster.

Any Agent may run in one of two modes: client or server. A server
node takes on the additional responsibility of being part of the [consensus quorum](#).
These nodes take part in Raft, and provide strong consistency and availability in
the case of failure. The higher burden on the server nodes means that usually they
should be run on dedicated instances, as they are more resource intensive than a client
node. Client nodes make up the majority of the cluster, and they are very lightweight
as they maintain very little state and interface with the server nodes for most operations.

## Running an Agent

The agent is started with the `consul agent` command. This command blocks,
running forever or until told to quit. The agent command takes a variety
of configuration options but the defaults are usually good enough. When
running `consul agent`, you should see output similar to that below:

```
$ consul agent -data-dir=/tmp/consul
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
       Node name: 'Armons-MacBook-Air'
      Datacenter: 'dc1'
          Server: false (bootstrap: false)
     Client Addr: 127.0.0.1 (HTTP: 8500, DNS: 8600, RPC: 8400)
    Cluster Addr: 192.168.1.43 (LAN: 8301, WAN: 8302)

==> Log data will now stream in as it occurs:

    [INFO] serf: EventMemberJoin: Armons-MacBook-Air.local 192.168.1.43
...
```

There are several important components that `consul agent` outputs:

* **Node name**: This is a unique name for the agent. By default this
  is the hostname of the machine, but you may customize it to whatever
  you'd like using the `-node` flag.

* **Datacenter**: This is the datacenter the agent is configured to run
 in. Consul has first-class support for multiple datacenters, but to work efficiently
 each node must be configured to correctly report its datacenter. The `-dc` flag
 can be used to set the datacenter. For single-DC configurations, the agent
 will default to "dc1".

* **Server**: This shows if the agent is running in the server or client mode.
  Server nodes have the extra burden of participating in the consensus quorum,
  storing cluster state, and handling queries. Additionally, a server may be
  in "bootstrap" mode. Multiple servers cannot be in bootstrap mode,
  otherwise the cluster state will be inconsistent.

* **Client Addr**: This is the address used for client interfaces to the agent.
  This includes the ports for the HTTP, DNS, and RPC interfaces. The RPC
  address is used for other `consul` commands. Other Consul commands such
  as `consul members` connect to a running agent and use RPC to query and
  control the agent. By default, this binds only to localhost. If you
  change this address or port, you'll have to specify an `-rpc-addr` whenever
  you run commands such as `consul members` so they know how to talk to the
  agent. This is also the address other applications can use over [RPC to control Consul](/docs/agent/rpc.html).

* **Cluster Addr**: This is the address and ports used for communication between
  Consul agents in a cluster. Every Consul agent in a cluster does not have to
  use the same port, but this address **MUST** be reachable by all other nodes.

## Stopping an Agent

An agent can be stopped in two ways: gracefully or forcefully. To gracefully
halt an agent, send the process an interrupt signal, which is usually
`Ctrl-C` from a terminal. When gracefully exiting, the agent first notifies
the cluster it intends to leave the cluster. This way, other cluster members
notify the cluster that the node has _left_.

Alternatively, you can force kill the agent by sending it a kill signal.
When force killed, the agent ends immediately. The rest of the cluster will
eventually (usually within seconds) detect that the node has died and will
notify the cluster that the node has _failed_.

It is especially important that a server node be allowed to gracefully leave,
so that there will be a minimal impact on availability as the server leaves
the consensus quorum.

For client agents, the difference between a node _failing_ and a node _leaving_
may not be important for your use case. For example, for a web server and load
balancer setup, both result in the same action: remove the web node
from the load balancer pool. But for other situations, you may handle
each scenario differently.

## Lifecycle

Every agent in the Consul cluster goes through a lifecycle. Understanding
this lifecycle is useful to building a mental model of an agent's interactions
with a cluster, and how the cluster treats a node.

When an agent is first started, it does not know about any other node in the cluster.
To discover it's peers, it must _join_ the cluster. This is done with the `join`
command or by providing the proper configuration to auto-join on start. Once a node
joins, this information is gossiped to the entire cluster, meaning all nodes will
eventually be aware of each other. If the agent is a server, existing servers will
begin replicating to the new node.

In the case of a network failure, some nodes may be unreachable by other nodes.
In this case, unreachable nodes are marked as _failed_. It is impossible to distinguish
between a network failure and an agent crash, so both cases are handled the same.
Once a node is marked as failed, this information is updated in the service catalog.
There is some nuance here relating, since this update is only possible if the
servers can still [form a quorum](/docs/internals/consensus.html). Once the network
failure recovers, or a crashed agent restarts, the cluster will repair itself,
and unmark a node as failed. The health check in the catalog will also be updated
to reflect this.

When a node _leaves_, it specifies it's intent to do so, and so the cluster
marks that node as having _left_. Unlike the _failed_ case, all of the
services provided by a node are immediately deregistered. If the agent was
a server, replication to it will stop. To prevent an accumulation
of dead nodes, Consul will automatically reap _failed_ nodes out of the
catalog as well. This is currently done on a non-configurable interval
which defaults to 72 hours. Reaping is similar to leaving, causing all
associated services to be deregistered.

