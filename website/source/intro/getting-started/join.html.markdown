---
layout: "intro"
page_title: "Join a Cluster"
sidebar_current: "gettingstarted-join"
---

# Join a Cluster

In the previous page, we started our first agent. While it showed how easy
it is to run Serf, it wasn't very exciting since we simply made a cluster of
one member. In this page, we'll create a real cluster with multiple members.

When starting a Serf agent, it begins without knowledge of any other node, and is
an isolated cluster of one.  To learn about other cluster members, the agent must
_join_ an existing cluster.  To join an existing cluster, Serf only needs to know
about a _single_ existing member. After it joins, the agent will gossip with this
member and quickly discover the other members in the cluster.

## Starting the Agents

First, let's start two agents. Serf agents must all listen on a unique
IP and port pair, so we must bind each agent to a different ports.

The first agent we'll start will listen on `127.0.0.1:7946`. We also will
specify a node name. The node name must be unique and is how a machine
is uniquely identified. By default it is the hostname of the machine, but
since we'll be running multiple agents on a single machine, we'll manually
override it.

```
$ serf agent -node=agent-one -bind=127.0.0.1:7946
...
```

Then, in another terminal, start a second agent. We'll bind this agent
to `127.0.0.1:7947`. In addition to overriding the node name, we're also going
to override the RPC address. The RPC address is the address that Serf binds
to for RPC operations. The other `serf` commands communicate with a running
Serf agent over RPC. We left the first agent with the default RPC address
so lets select another for this agent.

```
$ serf agent -node=agent-two -bind=127.0.0.1:7947 -rpc-addr=127.0.0.1:7374
...
```

At this point, you have two Serf agents running. The two Serf agents
still don't know anything about each other, and are each part of their own
clusters (of one member). You can verify this by running `serf members`
against each agent and noting that only one member is a part of each.

## Joining a Cluster

Now, let's tell the first agent to join the second agent by running
the following command in a new terminal:

```
$ serf join 127.0.0.1:7947
Successfully joined cluster by contacting 1 nodes.
```

You should see some log output in each of the agent logs. If you read
carefully, you'll see that they received join information. If you
run `serf members` against each agent, you'll see that both agents now
know about each other:

```
$ serf members
agent-one    127.0.0.1:7946    alive
agent-two    127.0.0.1:7947    alive

$ serf members -rpc-addr=127.0.0.1:7374
agent-two    127.0.0.1:7947    alive
agent-one    127.0.0.1:4946    alive
```

<div class="alert alert-block alert-info">
<p><strong>Remember:</strong> To join a cluster, a Serf agent needs to only
learn about <em>one existing member</em>. After joining the cluster, the
agents gossip with each other to propagate full membership information.
</p>
</div>

In addition to using `serf join` you can use the `-join` flag on
`serf agent` to join a cluster as part of starting up the agent.

## Leaving a Cluster

To leave the cluster, you can either gracefully quit an agent (using
`Ctrl-C`) or force kill one of the agents. Gracefully leaving allows
the node to transition into the _left_ state, otherwise other nodes
will detect it as having _failed_. The difference is covered
in more detail [here](/intro/getting-started/agent.html#toc_3).
