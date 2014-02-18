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
$ consul agent -data=/tmp/consul
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
         Node name: 'Armons-MacBook-Air.local'
        Datacenter: 'dc1'
    Advertise addr: '10.1.10.12'
          RPC addr: '127.0.0.1:8400'
         HTTP addr: '127.0.0.1:8500'
          DNS addr: '127.0.0.1:8600'
         Encrypted: false
            Server: false (bootstrap: false)

==> Log data will now stream in as it occurs:

    2014/02/18 14:25:02 [INFO] serf: EventMemberJoin: Armons-MacBook-Air.local 10.1.10.12
    2014/02/18 14:25:02 [ERR] agent: failed to sync remote state: No known Consul servers
...
```

There are several important components that `consul agent` outputs:

* **Node name**: This is a unique name for the agent. By default this
  is the hostname of the machine, but you may customize it to whatever
  you'd like using the `-node` flag.

* **Datacenter**: This is the datacenter the agent is configured to run
 in. Consul has first-class support for multiple datacenters, but to work efficiently
 each node must be configured to correctly report it's datacenter. The `-dc` flag
 can be used to set the datacenter. For single-DC configurations, the agent
 will default to "dc1".

* **Advertise addr**: This is the address and port used for communication between
  Consul agents in a cluster. Every Consul agent in a cluster does not have to
  use the same port, but this address **MUST** be reachable by all other nodes.

* **RPC addr**: This is the address and port used for RPC communications
  for other `consul` commands. Other Consul commands such as `consul members`
  connect to a running agent and use RPC to query and control the agent.
  By default, this binds only to localhost on the default port. If you
  change this address, you'll have to specify an `-rpc-addr` to commands
  such as `consul members` so they know how to talk to the agent. This is also
  the address other applications can use over [RPC to control Consul](/docs/agent/rpc.html).

* **HTTP/DNS addr**: This is the addresses the agent is listening on for the
  HTTP and DNS interfaces respectively. These are bound to localhost for security,
  but can be configured to listen on other addresses or ports.

* **Encrypted**: This shows if Consul is encrypting all traffic that it
  sends and expects to receive. It is a good sanity check to avoid sending
  non-encrypted traffic over any public networks. You can read more about
  [encryption here](/docs/agent/encryption.html).

* **Server**: This shows if the agent is running in the server or client mode.
  Server nodes have the extra burden of participating in the consensus quorum,
  storing cluster state, and handling queries. Additionally, a server may be
  in "bootstrap" mode. The first server must be in this mode to allow additional
  servers to join the cluster. Multiple servers cannot be in bootstrap mode,
  otherwise the cluster state will be inconsistent.

## Stopping an Agent

An agent can be stoped in two ways: gracefully or forcefully. To gracefully
halt an agent, send the process an interrupt signal, which is usually
`Ctrl-C` from a terminal. When gracefully exiting, the agent first notifies
the cluster it intends to leave the cluster. This way, other cluster members
notify the cluster that the node has _left_.

Alternatively, you can force kill the agent by sending it a kill signal.
When force killed, the agent ends immediately. The rest of the cluster will
eventually (usually within seconds) detect that the node has died and will
notify the cluster that the node has _failed_.

It is especially important that a server node be allowed to gracefully leave,
so that there will be a minimal impact on availablity as the server leaves
the consensus quorum.

For client agents, the difference between a node _failing_ and a node _leaving_
may not be important for your use case. For example, for a web server and load
balancer setup, both result in the same action: remove the web node
from the load balancer pool. But for other situations, you may handle
each scenario differently.

