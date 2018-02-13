---
layout: "docs"
page_title: "Geo Failover"
sidebar_current: "docs-guides-geo-failover"
description: |-
  Consul provides a prepared query capability that makes it easy to implement automatic geo failover policies for services.
---

# Geo Failover

Within a datacenter, Consul provides automatic failover for services by omitting failed service instances from DNS lookups, and by providing service health information in APIs. When there are no more instances of a service available in the local datacenter, it can be challenging to implement failover policies to other datacenters because typically that logic would need to be written into each application.

Fortunately, Consul has a [prepared query](/api/query.html) capability that lets users define failover policies in a centralized way. It's easy to expose these to applications using Consul's DNS interface and it's also available to applications that consume Consul's APIs. These policies range from fully static lists of alternate datacenters to fully dynamic policies that make use of Consul's [network coordinate](/docs/internals/coordinates.html) subsystem to automatically determine the next best datacenter to fail over to based on network round trip time. Prepared queries can be made with policies specific to certain services and prepared query templates allow one policy to apply to many, or even all services, with just a small number of templates.

This guide shows how to build geo failover policies using prepared queries through a set of examples.

## Prepared Queries

Prepared queries are objects that are defined at the datacenter level, similar to the values in Consul's KV store. They are created once and then invoked by applications to perform the query and get the latest results.

Here's an example request to create a prepared query:

```
$ curl \
    --request POST \
    --data \
'{
  "Name": "api",
  "Service": {
    "Service": "api",
    "Tags": ["v1.2.3"]
  }
}' http://127.0.0.1:8500/v1/query

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

This creates a prepared query called "api" that does a lookup for all instances of the "api" service with the tag "v1.2.3". This policy could be used to control which version of a "api" applications should be using in a centralized way. By [updating this prepared query](/api/query.html#update-prepared-query) to look for the tag "v1.2.4" applications could start to find the newer version of the service without having to reconfigure anything.

Applications can make use of this query in two ways. Since we gave the prepared query a name, they can simply do a DNS lookup for "api.query.consul" instead of "api.service.consul". Now with the prepared query, there's the additional filter policy working behind the scenes that the application doesn't have to know about. Queries can also be executed using the [prepared query execute API](/api/query.html#execute-prepared-query) for applications that integrate with Consul's APIs directly.

## Failover Policies

Using the techniques in this section we will develop prepared queries with failover policies where simply changing application configurations to look up "api.query.consul" instead of "api.service.consul" via DNS will result in automatic geo failover to the next closest federated Consul datacenters, in order of increasing network round trip time.

Failover is just another policy choice for a prepared query, it works in the same manner as the previous example and is similarly transparent to applications. The failover policy is configured using the `Failover` structure, which contains two fields, both of which are optional, and determine what happens if no healthy nodes are available in the local datacenter when the query is executed.

- `NearestN` `(int: 0)` - Specifies that the query will be forwarded to up to `NearestN` other datacenters based on their estimated network round trip time using [network coordinates](/docs/internals/coordinates.html).

- `Datacenters` `(array<string>: nil)` - Specifies a fixed list of remote datacenters to forward the query to if there are no healthy nodes in the local datacenter. Datacenters are queried in the order given in the list.

The following sections show examples using these fields to implement different geo failover policies.

### Static Policy

A static failover policy includes a fixed list of datacenters to contact once there are no healthy instances in the local datacenter.

Here's the example from the introduction, expanded with a static failover policy:

```
$ curl \
    --request POST \
    --data \
'{
  "Name": "api",
  "Service": {
    "Service": "api",
    "Tags": ["v1.2.3"],
    "Failover": {
      "Datacenters": ["dc1", "dc2"]
    }
  }
}' http://127.0.0.1:8500/v1/query

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

When this query is executed, such as with a DNS lookup to "api.query.consul", the following actions will occur:

1. Consul servers in the local datacenter will attempt to find healthy instances of the "api" service with the required tag.
2. If none are available locally, the Consul servers will make an RPC request to the Consul servers in "dc1" to perform the query there.
3. If none are available in "dc1", then an RPC will be made to the Consul servers in "dc2" to perform the query there.
4. Finally an error will be returned if none of these datacenters had any instances available.

### Dynamic Policy

In a complex federated environment with many Consul datacenters, it can be cumbersome to set static failover policies, so Consul offers a dynamic option based on Consul's [network coordinate](/docs/internals/coordinates.html) subsystem. Consul continuously maintains an estimate of the network round trip time from the local datacenter to the servers in other datacenters it is federated with. Each server uses the median round trip time from itself to the servers in the remote datacenter. This means that failover can simply try other remote datacenters in order of increasing network round trip time, and if datacenters come and go, or experience network issues, this order will adjust automatically.

Here's the example from the introduction, expanded with a dynamic failover policy:

```
$ curl \
    --request POST \
    --data \
'{
  "Name": "api",
  "Service": {
    "Service": "api",
    "Tags": ["v1.2.3"],
    "Failover": {
      "NearestN": 2
    }
  }
}' http://127.0.0.1:8500/v1/query

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

This query is resolved in a similar fashion to the previous example, except the choice of "dc1" or "dc2", or possibly some other datacenter, is made automatically.

### Hybrid Policy

It is possible to combine `Datacenters` and `NearestN` in the same policy. The `NearestN` queries will be performed first, followed by the list given by `Datacenters`. A given datacenter will only be queried one time during a failover, even if it is selected by both `NearestN` and is listed in `Datacenters`. This is useful for allowing a limited number of round trip-based attempts, followed by a static configuration for some known datacenter to failover to.

## Templates

For datacenters with many services, it can be cumbersome to define a prepared query to apply a geo failover policy for each service. Consul provides a [prepared query template](/api/query.html#prepared-query-templates) capability to allow one prepared query to apply to many, and even all, services.

Here's an example request to create a prepared query template that applies a dynamic geo failover policy to all services:

```
$ curl \
    --request POST \
    --data \
'{
  "Name": "",
  "Template": {
    "Type": "name_prefix_match"
  },
  "Service": {
    "Service": "${name.full}",
    "Failover": {
      "NearestN": 2
    }
  }
}' http://127.0.0.1:8500/v1/query

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

Templates can match on prefixes or use full regular expressions to determine which services they match. In this case, we've chosen the `name_prefix_match` type and given it an empty name, which means that it will match any service. If multiple queries are registered, the most specific one will be selected, so it's possible to have a template like this as a catch-all, and then apply more specific policies to certain services.

With this one prepared query template in place, simply changing application configurations to look up "api.query.consul" instead of "api.service.consul" via DNS will result in automatic geo failover to the next closest federated Consul datacenters, in order of increasing network round trip time.
