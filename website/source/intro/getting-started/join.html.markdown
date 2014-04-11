---
layout: "intro"
page_title: "Join a Cluster"
sidebar_current: "gettingstarted-join"
---

# Join a Cluster

In the previous page, we started our first agent. While it showed how easy
it is to run Consul, it wasn't very exciting since we simply made a cluster of
one member. In this page, we'll create a real cluster with multiple members.

When starting a Consul agent, it begins without knowledge of any other node, and is
an isolated cluster of one.  To learn about other cluster members, the agent must
_join_ an existing cluster.  To join an existing cluster, only needs to know
about a _single_ existing member. After it joins, the agent will gossip with this
member and quickly discover the other members in the cluster.

## Starting the Agents

To simulate a more realistic cluster, we are using a two node cluster in
Vagrant. The Vagrantfile can be found in the demo section of the repo
[here](https://github.com/hashicorp/consul/tree/master/demo/vagrant-cluster).

We start the first agent on our first node and also specify a node name.
The node name must be unique and is how a machine is uniquely identified.
By default it is the hostname of the machine, but we'll manually override it.
We are also providing a bind address. This is the address that Consul listens on,
and it *must* be accessible by all other nodes in the cluster. The first node
will act as our server in this cluster.

```
$ consul agent -server -bootstrap -data-dir /tmp/consul -node=agent-one -serf-bind=172.20.20.10 -server-addr=172.20.20.10:8300 -advertise=172.20.20.10
...
```

Then, in another terminal, start the second agent on the new node.
This time, we set the bind address to match the IP of the second node
as specified in the Vagrantfile. In production, you will generally want
to provide a bind address or interface as well.

```
$ consul agent -data-dir /tmp/consul -node=agent-two -serf-bind=172.20.20.11 -server-addr=172.20.20.11:8300 -advertise=172.20.20.11
...
```

At this point, you have two Consul agents running, one server and one client.
The two Consul agents still don't know anything about each other, and are each part of their own
clusters (of one member). You can verify this by running `consul members`
against each agent and noting that only one member is a part of each.

## Joining a Cluster

Now, let's tell the first agent to join the second agent by running
the following command in a new terminal:

```
$ consul join 172.20.20.11
Successfully joined cluster by contacting 1 nodes.
```

You should see some log output in each of the agent logs. If you read
carefully, you'll see that they received join information. If you
run `consul members` against each agent, you'll see that both agents now
know about each other:

```
$ consul members
agent-one  172.20.20.10:8301  alive  role=consul,dc=dc1,vsn=1,vsn_min=1,vsn_max=1,port=8300,bootstrap=1
agent-two  172.20.20.11:8301  alive  role=node,dc=dc1,vsn=1,vsn_min=1,vsn_max=1
```

<div class="alert alert-block alert-info">
<p><strong>Remember:</strong> To join a cluster, a Consul agent needs to only
learn about <em>one existing member</em>. After joining the cluster, the
agents gossip with each other to propagate full membership information.
</p>
</div>

In addition to using `consul join` you can use the `-join` flag on
`consul agent` to join a cluster as part of starting up the agent.

## Leaving a Cluster

To leave the cluster, you can either gracefully quit an agent (using
`Ctrl-C`) or force kill one of the agents. Gracefully leaving allows
the node to transition into the _left_ state, otherwise other nodes
will detect it as having _failed_. The difference is covered
in more detail [here](/intro/getting-started/agent.html#toc_3).

