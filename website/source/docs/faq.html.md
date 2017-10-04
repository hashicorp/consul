---
layout: "docs"
page_title: "Frequently Asked Questions"
sidebar_current: "docs-faq"
---

# Frequently Asked Questions

## Q: What is Checkpoint? / Does Consul call home?

Consul makes use of a HashiCorp service called [Checkpoint](http://checkpoint.hashicorp.com)
which is used to check for updates and critical security bulletins.
Only anonymous information, which cannot be used to identify the user or host, is
sent to Checkpoint . An anonymous ID is sent which helps de-duplicate warning messages.
This anonymous ID can be disabled. In fact, using the Checkpoint service is optional
and can be disabled.

See [`disable_anonymous_signature`](/docs/agent/options.html#disable_anonymous_signature)
and [`disable_update_check`](/docs/agent/options.html#disable_update_check).

## Q: How does Atlas integration work?

Hosted Consul Enterprise in Atlas was officially deprecated on March 7th,
2017.

There are strong alternatives available and they are listed below.

For users on supported cloud platform the
[-retry-join](/docs/agent/options.html#_retry_join) option allows bootstrapping
by automatically discovering instances with a given tag key/value at startup. 

For users on other cloud platforms [-join and retry-join
functionality](/docs/agent/options.html#_join) can be used to join clusters by
ip address or hostname.

Other features of Consul Enterprise, such as the UI and Alerts also have
suitable open source alternatives.

For replacing the UI, we recommend the [free UI packaged as part of Consul open source](https://www.consul.io/docs/agent/options.html#_ui). A live demo can be access at [https://demo.consul.io/ui/](https://demo.consul.io/ui/).

For replacing alerts, we recommend the [open source Consul alerts daemon](https://github.com/acalephstorage/consul-alerts). This is not maintained or supported by HashiCorp, however there is active development from the community.

## Q: Does Consul rely on UDP Broadcast or Multicast?

Consul uses the [Serf](https://www.serf.io) gossip protocol which relies on
TCP and UDP unicast. Broadcast and Multicast are rarely available in a multi-tenant
or cloud network environment. For that reason, Consul and Serf were both
designed to avoid any dependence on those capabilities.

## Q: Is Consul eventually or strongly consistent?

Consul has two important subsystems, the service catalog and the gossip protocol.
The service catalog stores all the nodes, service instances, health check data,
ACLs, and KV information. It is strongly consistent, and replicated
using the [consensus protocol](/docs/internals/consensus.html).

The [gossip protocol](/docs/internals/gossip.html) is used to track which
nodes are part of the cluster and to detect a node or agent failure. This information
is eventually consistent by nature. When the servers detects a change in membership,
or receive a health update, they update the service catalog appropriately.

Because of this split, the answer to the question is subtle. Almost all client APIs
interact with the service catalog and are strongly consistent. Updates to the
catalog may come via the gossip protocol which is eventually consistent, meaning
the current state of the catalog can lag behind until the state is reconciled.

## Q: Are _failed_ or _left_ nodes ever removed?

To prevent an accumulation of dead nodes (nodes in either _failed_ or _left_
states), Consul will automatically remove dead nodes out of the catalog. This
process is called _reaping_. This is currently done on a configurable
interval of 72 hours. Reaping is similar to leaving, causing all associated
services to be deregistered. Changing the reap interval for aesthetic
reasons to trim the number of _failed_ or _left_ nodes is not advised (nodes
in the _failed_ or _left_ state do not cause any additional burden on
Consul).

## Q: Does Consul support delta updates for watchers or blocking queries?

Consul does not currently support sending a delta or a change only response
to a watcher or a blocking query. The API simply allows for an edge-trigger
return with the full result. A client should keep the results of their last
read and compute the delta client side.

By design, Consul offloads this to clients instead of attempting to support
the delta calculation. This avoids expensive state maintenance on the servers
as well as race conditions between data updates and watch registrations.

## Q: What network ports does Consul use?

The [Ports Used](https://www.consul.io/docs/agent/options.html#ports) section of the Configuration documentation lists all ports that Consul uses.

## Q: Does Consul require certain user process resource limits?

There should be only a small number of open file descriptors required for a
Consul client agent. The gossip layers perform transient connections with
other nodes, each connection to the client agent (such as for a blocking
query) will open a connection, and there will typically be connections to one
of the Consul servers. A small number of file descriptors are also required
for watch handlers, health checks, log files, and so on.

For a Consul server agent, you should plan on the above requirements and
an additional incoming connection from each of the nodes in the cluster. This
should not be the common case, but in the worst case if there is a problem
with the other servers you would expect the other client agents to all
connect to a single server and so preparation for this possibility is helpful.

The default ulimits are usually sufficient for Consul, but you should closely
scrutinize your own environment's specific needs and identify the root cause
of any excessive resource utilization before arbitrarily increasing the limits.

## Q: What is the per-key value size limitation for Consul's key/value store?

The limit on a key's value size is 512KB. This is is strictly enforced and an
HTTP 413 status will be returned to any client that attempts to store more
than that limit in a value. It should be noted that the Consul key/value store
is not designed to be used as a general purpose database. See
[Server Performance](/docs/guides/performance.html) for more details.

## Q: What data is replicated between Consul datacenters?

In general, data is not replicated between different Consul datacenters. When a
request is made for a resource in another datacenter, the local Consul servers forward
an RPC request to the remote Consul servers for that resource and return the results.
If the remote datacenter is not available, then those resources will also not be
available, but that won't otherwise affect the local datacenter. There are some special
situations where a limited subset of data can be replicated, such as with Consul's built-in
[ACL replication](/docs/guides/acl.html#outages-and-acl-replication) capability, or
external tools like [consul-replicate](https://github.com/hashicorp/consul-replicate).
