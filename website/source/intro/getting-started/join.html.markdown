---
layout: "intro"
page_title: "Consul Cluster"
sidebar_current: "gettingstarted-join"
description: |-
  By this point, we've started our first agent and registered and queried one or more services on that agent. This showed how easy it is to use Consul, but didn't show how this could be extended to a scalable production service discovery infrastructure. On this page, we'll create our first real cluster with multiple members.
---

# Consul Cluster

By this point, we've started our first agent and registered and queried
one or more services on that agent. This showed how easy it is to use
Consul, but didn't show how this could be extended to a scalable production
service discovery infrastructure. On this page, we'll create our first
real cluster with multiple members.

When starting a Consul agent, it begins without knowledge of any other node, and is
an isolated cluster of one.  To learn about other cluster members, the agent must
_join_ an existing cluster.  To join an existing cluster, it only needs to know
about a _single_ existing member. After it joins, the agent will gossip with this
member and quickly discover the other members in the cluster. A Consul
agent can join any other agent, it doesn't have to be an agent in server mode.

## Starting the Agents

To simulate a more realistic cluster, we are using a two node cluster in
Vagrant. The Vagrantfile can be found in the demo section of the repo
[here](https://github.com/hashicorp/consul/tree/master/demo/vagrant-cluster).

We start the first agent on our first node and also specify a node name.
The node name must be unique and is how a machine is uniquely identified.
By default it is the hostname of the machine, but we'll manually override it.
We are also providing a bind address. This is the address that Consul listens on,
and it *must* be accessible by all other nodes in the cluster. The first node
will act as our server in this cluster. We're still not making a cluster
of servers.

```text
$ consul agent -server -bootstrap-expect 1 -data-dir /tmp/consul \
    -node=agent-one -bind=172.20.20.10
...
```

Then, in another terminal, start the second agent on the new node.
This time, we set the bind address to match the IP of the second node
as specified in the Vagrantfile. In production, you will generally want
to provide a bind address or interface as well.

```text
$ consul agent -data-dir /tmp/consul -node=agent-two -bind=172.20.20.11
...
```

At this point, you have two Consul agents running, one server and one client.
The two Consul agents still don't know anything about each other, and are each part of their own
clusters (of one member). You can verify this by running `consul members`
against each agent and noting that only one member is a part of each.

## Joining a Cluster

Now, let's tell the first agent to join the second agent by running
the following command in a new terminal:

```text
$ consul join 172.20.20.11
Successfully joined cluster by contacting 1 nodes.
```

You should see some log output in each of the agent logs. If you read
carefully, you'll see that they received join information. If you
run `consul members` against each agent, you'll see that both agents now
know about each other:

```text
$ consul members -detailed
Node        Address             Status  Tags
agent-one   172.20.20.10:8301   alive   role=consul,dc=dc1,vsn=2,vsn_min=1,vsn_max=2,port=8300,bootstrap=1
agent-two   172.20.20.11:8301   alive   role=node,dc=dc1,vsn=2,vsn_min=1,vsn_max=2
```

-> **Remember:** To join a cluster, a Consul agent needs to only
learn about <em>one existing member</em>. After joining the cluster, the
agents gossip with each other to propagate full membership information.

In addition to using `consul join` you can use the `-join` flag on
`consul agent` to join a cluster as part of starting up the agent.

## Querying Nodes

Just like querying services, Consul has an API for querying the
nodes themselves. You can do this via the DNS or HTTP API.

For the DNS API, the structure of the names is `NAME.node.consul` or
`NAME.DATACENTER.node.consul`. If the datacenter is omitted, Consul
will only search the local datacenter.

From "agent-one", query "agent-two":

```
$ dig @127.0.0.1 -p 8600 agent-two.node.consul
...

;; QUESTION SECTION:
;agent-two.node.consul.	IN	A

;; ANSWER SECTION:
agent-two.node.consul.	0 IN	A	172.20.20.11
```

The ability to look up nodes in addition to services is incredibly
useful for system administration tasks. For example, knowing the address
of the node to SSH into is as easy as making it part of the Consul cluster
and querying it.

## Leaving a Cluster

To leave the cluster, you can either gracefully quit an agent (using
`Ctrl-C`) or force kill one of the agents. Gracefully leaving allows
the node to transition into the _left_ state, otherwise other nodes
will detect it as having _failed_. The difference is covered
in more detail [here](/intro/getting-started/agent.html#toc_3).
