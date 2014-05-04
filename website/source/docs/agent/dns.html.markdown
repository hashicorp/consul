---
layout: "docs"
page_title: "DNS Interface"
sidebar_current: "docs-agent-dns"
---

# DNS Interface

One of the primary query interfaces for Consul is using DNS.
The DNS interface allows applications to make use of service
discovery without any high-touch integration with Consul. For
example, instead of making any HTTP API requests to Consul,
a host can use the DNS server directly and just do a name lookup
for "redis.service.east-aws.consul".

This query automatically translates to a lookup of nodes that
provide the redis service, located in the "east-aws" datacenter,
with no failing health checks. It's that simple!

There are a number of [configuration options](/docs/agent/options.html) that
are important for the DNS interface. They are `ports.dns`, `recursor`,
and `domain`. By default Consul will listen on 127.0.0.1:8600 for DNS queries
in the "consul." domain, without support for DNS recursion.

There are a few ways to use the DNS interface. One option is to use a custom
DNS resolver library and point it at Consul. Another option is to set Consul
as the DNS server for a node, and provide a `recursor` so that non-Consul queries
can also be resolved. The last method is to forward all queries for the "consul."
domain to a Consul agent from the existing DNS server. To play with the DNS server
on the command line, dig can be used:

    $ dig @127.0.0.1 -p 8600 redis.service.dc1.consul. ANY

## Node Lookups

For Consul to resolve names, it relies on a very specific format for queries.
There are fundamentally two types of queries, node lookups and service lookups.
A node lookup is a simple query for the address of a named node, and takes on
the following format:

    <node>.node.<datacenter>.<domain>

So, for example, if we have a "foo" node with default settings, we could look for
"foo.node.dc1.consul." The datacenter is an optional part of the FQDN, and if not
provided defaults to the datacenter of the agent. So if we know "foo" is running in our
same datacenter, we can instead use "foo.node.consul." Alternatively, we can do a
DNS lookup for nodes in other datacenters, with no additional effort.

For a node lookup, the only records returned are A records with the IP address of
the node.

    $ dig @127.0.0.1 -p 8600 foobar.node.consul ANY

    ; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 foobar.node.consul ANY
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24355
    ;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 1, ADDITIONAL: 0
    ;; WARNING: recursion requested but not available

    ;; QUESTION SECTION:
    ;foobar.node.consul.		IN	ANY

    ;; ANSWER SECTION:
    foobar.node.consul.	0	IN	A	10.1.10.12

    ;; AUTHORITY SECTION:
    consul.			0	IN	SOA	ns.consul. postmaster.consul. 1392836399 3600 600 86400 0


## Service Lookups

A service lookup is the alternate type of query. It is used to query for service
providers. The format of a service lookup is like the following:

    <tag>.<service>.service.<datacenter>.<domain>

As with node lookups, the `datacenter` is optional, as is the `tag`. If no tag is
provided, then no filtering is done on tag. So, if we want to find any redis service
providers in our local datacenter, we could lookup "redis.service.consul.", however
if we care about the PostgreSQL master in a particular datacenter, we could lookup
"master.postgresql.service.dc2.consul."

The DNS query system makes use of health check information to prevent routing
to unhealthy nodes. When a service query is made, any services failing their health
check, or failing a node system check will be omitted from the results. To allow
for simple load balancing, the set of nodes returned is also randomized each time.
These simple mechanisms make it easy to use DNS along with application level retries
as a simple foundation for an auto-healing service oriented architecture.

For these lookups, both A and SRV records may be served. The SRV records will also
provide the port that a service is registered on, enabling services to avoid relying
on well-known ports. SRV records are only served if the client specifically requests
SRV records.

    $ dig @127.0.0.1 -p 8600 consul.service.consul SRV

    ; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 consul.service.consul ANY
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 50483
    ;; flags: qr aa rd; QUERY: 1, ANSWER: 3, AUTHORITY: 1, ADDITIONAL: 1
    ;; WARNING: recursion requested but not available

    ;; QUESTION SECTION:
    ;consul.service.consul.		IN	SRV

    ;; ANSWER SECTION:
    consul.service.consul.	0	IN	SRV	1 1 8300 foobar.node.dc1.consul.

    ;; ADDITIONAL SECTION:
    foobar.node.dc1.consul.	0	IN	A	10.1.10.12


