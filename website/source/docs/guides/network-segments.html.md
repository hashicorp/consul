---
layout: "docs"
page_title: "Partial LAN Connectivity - Configuring Network Segments"
sidebar_current: "docs-guides-segments"
description: |-
  Many advanced Consul users have the need to run clusters with segmented networks, meaning that
  not all agents can be in a full mesh. This is usually the result of business policies enforced
  via network rules or firewalls. Prior to Consul 0.9.3 this was only possible through federation,
  which for some users is too heavyweight or expensive as it requires running multiple servers per
  segment.
---

# Network Segments [Enterprise Only]

~> Note, the network segment functionality described here is available only in [Consul Enterprise](https://www.hashicorp.com/products/consul/) version 0.9.3 and later.

Many advanced Consul users have the need to run clusters with segmented networks, meaning that
not all agents can be in a full mesh. This is usually the result of business policies enforced
via network rules or firewalls. Prior to Consul 0.9.3 this was only possible through federation,
which for some users is too heavyweight or expensive as it requires running multiple servers per
segment.

This guide will cover the basic configuration for setting up multiple segments, as well as
how to configure a prepared query to limit service discovery to the services in the local agent's
network segment.

To complete this guide you will need to complete the 
[Deployment Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/deployment-guide).


## Partial LAN Connectivity with Network Segments

By default, all Consul agents in one datacenter are part of a shared gossip pool over the LAN;
this means that the partial connectivity caused by segmented networks would cause health flapping
as nodes failed to communicate. In this guide we will cover the Network Segments feature, added
in [Consul Enterprise](https://www.hashicorp.com/products/consul/) version 0.9.3, which allows users
to configure Consul to support this kind of segmented network topology.


### Network Segments Overview

All Consul agents are part of the default network segment, unless a segment is specified in
their configuration. In a standard cluster setup, all agents will normally be part of this default
segment and as a result, part of one shared LAN gossip pool. 

Network segments can be used to break
up the LAN gossip pool into multiple isolated smaller pools by specifying the configuration for segments
on the servers. Each desired segment must be given a name and port, as well as optionally a custom
bind and advertise address for that segment's gossip listener to bind to on the server.

A few things to note:

1. Servers will be a part of all segments they have been configured with. They are the common point
linking the different segments together. The configured list of segments is specified by the
[`segments`](/docs/agent/options.html#segments) option.

2. Client agents can only be part of one segment at a given time, specified by the [`-segment`]
(/docs/agent/options.html#_segment) option.

3. Clients can only join agents in the same segment as them. If they attempt to join a client in
another segment, or the listening port of another segment on a server, they will get a segment mismatch error.

Once the servers have been configured with the correct segment info, the clients only need to specify
their own segment in the [Agent Config](/docs/agent/options.html#_segment) and join by connecting to another
agent within the same segment. If joining to a Consul server, client will need to provide the server's
port for their segment along with the address of the server when performing the join (for example,
`consul agent -retry-join "consul.domain.internal:1234"`).

## Setup Network Segments

### Configure Consul Servers

To get started, 
start a server or group of servers, with the following section added to the configuration. Note, you may need to
adjust the bind/advertise addresses for your setup.

```json
{
  "segments": [
    {"name": "alpha", "bind": "{{GetPrivateIP}}", "advertise": "{{GetPrivateIP}}", "port": 8303},
    {"name": "beta", "bind": "{{GetPrivateIP}}", "advertise": "{{GetPrivateIP}}", "port": 8304}
  ]
}
```

You should see a log message on the servers for each segment's listener as the agent starts up.

```sh
2017/08/30 19:05:13 [INFO] serf: EventMemberJoin: server1.dc1 192.168.0.4
2017/08/30 19:05:13 [INFO] serf: EventMemberJoin: server1 192.168.0.4
2017/08/30 19:05:13 [INFO] consul: Started listener for LAN segment "alpha" on 192.168.0.4:8303
2017/08/30 19:05:13 [INFO] serf: EventMemberJoin: server1 192.168.0.4
2017/08/30 19:05:13 [INFO] consul: Started listener for LAN segment "beta" on 192.168.0.4:8304
2017/08/30 19:05:13 [INFO] serf: EventMemberJoin: server1 192.168.0.4
```

Running `consul members` should show the server as being part of all segments.

```sh
(server1) $ consul members
Node     Address           Status  Type    Build      Protocol  DC   Segment
server1  192.168.0.4:8301  alive   server  0.9.3+ent  2         dc1  <all>
```

### Configure Consul Clients in Different Network Segments  

Next, start a client agent in the 'alpha' segment, with `-join` set to the server's segment
address/port for that segment.

```sh
(client1) $ consul agent ... -join 192.168.0.4:8303 -node client1 -segment alpha
```

After the join is successful, we should see the client show up by running the `consul members` command
on the server again.

```sh
(server1) $ consul members
Node     Address           Status  Type    Build      Protocol  DC   Segment
server1  192.168.0.4:8301  alive   server  0.9.3+ent  2         dc1  <all>
client1  192.168.0.5:8301  alive   client  0.9.3+ent  2         dc1  alpha
```

Now join another client in segment 'beta' and run the `consul members` command another time.

```sh
(client2) $ consul agent ... -join 192.168.0.4:8304 -node client2 -segment beta
```

```sh
(server1) $ consul members
Node     Address           Status  Type    Build      Protocol  DC   Segment
server1  192.168.0.4:8301  alive   server  0.9.3+ent  2         dc1  <all>
client1  192.168.0.5:8301  alive   client  0.9.3+ent  2         dc1  alpha
client2  192.168.0.6:8301  alive   client  0.9.3+ent  2         dc1  beta
```

### Filter Segmented Nodes

If we pass the `-segment` flag when running `consul members`, we can limit the view to agents
in a specific segment.

```sh
(server1) $ consul members -segment alpha
Node     Address           Status  Type    Build      Protocol  DC   Segment
client1  192.168.0.5:8301  alive   client  0.9.3+ent  2         dc1  alpha
server1  192.168.0.4:8303  alive   server  0.9.3+ent  2         dc1  alpha
```

Using the `consul catalog nodes` command, we can filter on an internal metadata key,
`consul-network-segment`, which stores the network segment of the node.

```sh
(server1) $ consul catalog nodes -node-meta consul-network-segment=alpha
Node     ID        Address      DC
client1  4c29819c  192.168.0.5  dc1
```

With this metadata key, we can construct a [Prepared Query](/api/query.html) that can be used
for DNS to return only services within the same network segment as the local agent.

## Configure a Prepared Query to Limit Service Discovery

### Create Services

First, register a service on each of the client nodes.

```sh
(client1) $ curl \
    --request PUT \
    --data '{"Name": "redis", "Port": 8000}' \
    localhost:8500/v1/agent/service/register
```

```sh
(client2) $ curl \
    --request PUT \
    --data '{"Name": "redis", "Port": 9000}' \
    localhost:8500/v1/agent/service/register
```

### Create the Prepared Query

Next, write the following to `query.json` and create the query using the HTTP endpoint.

```sh
(server1) $ curl \
    --request POST \
    --data \
'{
   "Name": "",
   "Template": {
     "Type": "name_prefix_match"
   },
   "Service": {
     "Service": "${name.full}",
     "NodeMeta": {"consul-network-segment": "${agent.segment}"}
   }
}' localhost:8500/v1/query

{"ID":"6f49dd24-de9b-0b6c-fd29-525eca069419"}
```

### Test the Segments with DNS Lookups

Now, we can replace any dns lookups of the form `<service>.service.consul` with
`<service>.query.consul` to look up only services within the same network segment.

**Client 1:**

```sh
(client1) $ dig @127.0.0.1 -p 8600 redis.query.consul SRV

; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 redis.query.consul SRV
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 3149
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;redis.query.consul.		IN	SRV

;; ANSWER SECTION:
redis.query.consul.	0	IN	SRV	1 1 8000 client1.node.dc1.consul.

;; ADDITIONAL SECTION:
client1.node.dc1.consul. 0	IN	A	192.168.0.5
```

**Client 2:**

```sh
(client2) $ dig @127.0.0.1 -p 8600 redis.query.consul SRV

; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 redis.query.consul SRV
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 3149
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;redis.query.consul.		IN	SRV

;; ANSWER SECTION:
redis.query.consul.	0	IN	SRV	1 1 9000 client2.node.dc1.consul.

;; ADDITIONAL SECTION:
client2.node.dc1.consul. 0	IN	A	192.168.0.6
```

## Summary

In this guide you configured the Consul agents to participate in partial 
LAN gossip based on network segments. You then set up a couple services and
a prepared query to test the segments. 

