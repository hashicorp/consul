---
layout: "intro"
page_title: "Run the Agent"
sidebar_current: "gettingstarted-agent"
description: |-
  After Consul is installed, the agent must be run. The agent can either run in server or client mode. Each datacenter must have at least one server, though a cluster of 3 or 5 servers is recommended. A single server deployment is highly discouraged as data loss is inevitable in a failure scenario.
---

# Run the Consul Agent

After Consul is installed, the agent must be run. The agent can run either
in server or client mode. Each datacenter must have at least one server,
though a cluster of 3 or 5 servers is recommended. A single server deployment
is _**highly**_ discouraged as data loss is inevitable in a failure scenario.

All other agents run in client mode. A client is a very lightweight
process that registers services, runs health checks, and forwards queries to
servers. The agent must be run on every node that is part of the cluster.

For more detail on bootstrapping a datacenter, see
[this guide](/docs/guides/bootstrapping.html).

## Starting the Agent

For simplicity, we'll run a single Consul agent in server mode:

```text
$ consul agent -server -bootstrap-expect 1 -data-dir /tmp/consul
==> WARNING: BootstrapExpect Mode is specified as 1; this is the same as Bootstrap mode.
==> WARNING: Bootstrap mode enabled! Do not enable unless necessary
==> WARNING: It is highly recommended to set GOMAXPROCS higher than 1
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
       Node name: 'Armons-MacBook-Air'
      Datacenter: 'dc1'
          Server: true (bootstrap: true)
     Client Addr: 127.0.0.1 (HTTP: 8500, DNS: 8600, RPC: 8400)
    Cluster Addr: 10.1.10.38 (LAN: 8301, WAN: 8302)

==> Log data will now stream in as it occurs:

[INFO] serf: EventMemberJoin: Armons-MacBook-Air.local 10.1.10.38
[INFO] raft: Node at 10.1.10.38:8300 [Follower] entering Follower state
[INFO] consul: adding server for datacenter: dc1, addr: 10.1.10.38:8300
[ERR] agent: failed to sync remote state: rpc error: No cluster leader
[WARN] raft: Heartbeat timeout reached, starting election
[INFO] raft: Node at 10.1.10.38:8300 [Candidate] entering Candidate state
[INFO] raft: Election won. Tally: 1
[INFO] raft: Node at 10.1.10.38:8300 [Leader] entering Leader state
[INFO] consul: cluster leadership acquired
[INFO] consul: New leader elected: Armons-MacBook-Air
[INFO] consul: member 'Armons-MacBook-Air' joined, marking health alive
```

As you can see, the Consul agent has started and has output some log
data. From the log data, you can see that our agent is running in server mode
and has claimed leadership of the cluster. Additionally, the local member has
been marked as a healthy member of the cluster.

~> **Note for OS X Users:** Consul uses your hostname as the
default node name. If your hostname contains periods, DNS queries to
that node will not work with Consul. To avoid this, explicitly set
the name of your node with the `-node` flag.

## Cluster Members

If you run [`consul members`](/docs/commands/members.html) in another terminal, you
can see the members of the Consul cluster. We'll cover joining clusters in the next
section, but for now, you should only see one member (yourself):

```text
$ consul members
Node                    Address             Status  Type    Build  Protocol
Armons-MacBook-Air      10.1.10.38:8301     alive   server  0.5.1  2
```

The output shows our own node, the address it is running on, its
health state, its role in the cluster, and some version information.
Additional metadata can be viewed by providing the `-detailed` flag.

The output of the [`members`](/docs/commands/members.html) command is based on
the [gossip protocol](/docs/internals/gossip.html) and is eventually consistent.
That is, at any point in time, the view of the world as seen by your local
agent may not exactly match the state on the servers. For a strongly consistent
view of the world, use the [HTTP API](/docs/agent/http.html) as it forwards the
request to the Consul servers:

```text
$ curl localhost:8500/v1/catalog/nodes
[{"Node":"Armons-MacBook-Air","Address":"10.1.10.38"}]
```

In addition to the HTTP API, the [DNS interface](/docs/agent/dns.html) can
be used to query the node. Note that you have to make sure to point your DNS
lookups to the Consul agent's DNS server which runs on port 8600 by default.
The format of the DNS entries (such as "Armons-MacBook-Air.node.consul") will
be covered in more detail later.

```text
$ dig @127.0.0.1 -p 8600 Armons-MacBook-Air.node.consul
...

;; QUESTION SECTION:
;Armons-MacBook-Air.node.consul.	IN	A

;; ANSWER SECTION:
Armons-MacBook-Air.node.consul.	0 IN	A	10.1.10.38
```

## <a name="stopping"></a>Stopping the Agent

You can use `Ctrl-C` (the interrupt signal) to gracefully halt the agent.
After interrupting the agent, you should see it leave the cluster
and shut down.

By gracefully leaving, Consul notifies other cluster members that the
node _left_. If you had forcibly killed the agent process, other members
of the cluster would have detected that the node _failed_. When a member leaves,
its services and checks are removed from the catalog. When a member fails,
its health is simply marked as critical, but it is not removed from the catalog.
Consul will automatically try to reconnect to _failed_ nodes, allowing it
to recover from certain network conditions, while _left_ nodes are no longer contacted.

Additionally, if an agent is operating as a server, a graceful leave is important
to avoid causing a potential availability outage affecting the
[consensus protocol](/docs/internals/consensus.html). See the
[guides section](/docs/guides/index.html) for details on how to safely add
and remove servers.

## Next Steps

Your simple Consul cluster is up and running. Let's give it some
[services](services.html)!
