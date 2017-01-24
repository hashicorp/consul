---
layout: "docs"
page_title: "Catalog (HTTP)"
sidebar_current: "docs-agent-http-catalog"
description: >
  The Catalog is the endpoint used to register and deregister nodes,
  services, and checks. It also provides query endpoints.
---

# Catalog HTTP Endpoint

The Catalog is the endpoint used to register and deregister nodes,
services, and checks. It also provides query endpoints.

The following endpoints are supported:

* [`/v1/catalog/register`](#catalog_register) : Registers a new node, service, or check
* [`/v1/catalog/deregister`](#catalog_deregister) : Deregisters a node, service, or check
* [`/v1/catalog/datacenters`](#catalog_datacenters) : Lists known datacenters
* [`/v1/catalog/nodes`](#catalog_nodes) : Lists nodes in a given DC
* [`/v1/catalog/services`](#catalog_services) : Lists services in a given DC
* [`/v1/catalog/service/<service>`](#catalog_service) : Lists the nodes in a given service
* [`/v1/catalog/node/<node>`](#catalog_node) : Lists the services provided by a node

The `nodes` and `services` endpoints support blocking queries and
tunable consistency modes.

### <a name="catalog_register"></a> /v1/catalog/register

The register endpoint is a low-level mechanism for registering or updating
entries in the catalog. It is usually preferable to instead use the
[agent endpoints](agent.html) for registration as they are simpler and perform
[anti-entropy](/docs/internals/anti-entropy.html).

The register endpoint expects a JSON request body to be `PUT`. The request
body must look something like:

```javascript
{
  "Datacenter": "dc1",
  "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
  "Node": "foobar",
  "Address": "192.168.10.10",
  "TaggedAddresses": {
    "lan": "192.168.10.10",
    "wan": "10.0.10.10"
  },
  "NodeMeta": {
    "somekey": "somevalue"
  },
  "Service": {
    "ID": "redis1",
    "Service": "redis",
    "Tags": [
      "primary",
      "v1"
    ],
    "Address": "127.0.0.1",
    "Port": 8000
  },
  "Check": {
    "Node": "foobar",
    "CheckID": "service:redis1",
    "Name": "Redis health check",
    "Notes": "Script based health check",
    "Status": "passing",
    "ServiceID": "redis1"
  }
}
```

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` and `Address` to be provided while `Datacenter` will be defaulted
to match that of the agent. If only those are provided, the endpoint will register
the node with the catalog. `TaggedAddresses` can be used in conjunction with the
[`translate_wan_addrs`](/docs/agent/options.html#translate_wan_addrs) configuration
option and the `wan` address. The `lan` address was added in Consul 0.7 to help find
the LAN address if address translation is enabled. The `ID` field was added in Consul
0.7.3 and is optional, but if supplied must be in the form of a hex string, 36
characters long. This is a unique identifier for this node across all time, even if
the node name or address changes.

The `Meta` block was added in Consul 0.7.3 to enable associating arbitrary metadata
key/value pairs with a node for filtering purposes. For more information on node metadata,
see the [node meta](/docs/agent/options.html#_node_meta) section of the configuration page.

If the `Service` key is provided, the service will also be registered. If
`ID` is not provided, it will be defaulted to the value of the `Service.Service` property.
Only one service with a given `ID` may be present per node. The service `Tags`, `Address`,
and `Port` fields are all optional.

If the `Check` key is provided, a health check will also be registered. The register API manipulates the health check entry in the Catalog, but it does not setup
the script, TTL, or HTTP check to monitor the node's health. To truly enable a new
health check, the check must either be provided in agent configuration or set via
the [agent endpoint](agent.html).

The `CheckID` can be omitted and will default to the value of `Name`. As with `Service.ID`,
the `CheckID` must be unique on this node. `Notes` is an opaque field that is meant to
hold human-readable text. If a `ServiceID` is provided that matches the `ID`
of a service on that node, the check is treated as a service level health
check, instead of a node level health check. The `Status` must be one of `passing`, `warning`, or `critical`.

Multiple checks can be provided by replacing `Check` with `Checks` and sending
an array of `Check` objects.

It is important to note that `Check` does not have to be provided with `Service`
and vice versa. A catalog entry can have either, neither, or both.

An optional ACL token may be provided to perform the registration by including a
`WriteRequest` block in the query payload, like this:

```javascript
{
  "WriteRequest": {
    "Token": "foo"
  }
}
```

If the API call succeeds, a 200 status code is returned.

### <a name="catalog_deregister"></a> /v1/catalog/deregister

The deregister endpoint is a low-level mechanism for directly removing
entries from the Catalog. It is usually preferable to instead use the
[agent endpoints](agent.html) for deregistration as they are simpler and perform
[anti-entropy](/docs/internals/anti-entropy.html).

The deregister endpoint expects a JSON request body to be `PUT`. The request
body must look like one of the following:

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
}
```

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "CheckID": "service:redis1"
}
```

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "ServiceID": "redis1",
}
```

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` to be provided while `Datacenter` will be defaulted
to match that of the agent. If only `Node` is provided, the node and
all associated services and checks are deleted. If `CheckID` is provided, only
that check is removed. If `ServiceID` is provided, the
service and its associated health check (if any) are removed.

An optional ACL token may be provided to perform the deregister action by adding
a `WriteRequest` block to the payload, like this:

```javascript
{
  "WriteRequest": {
    "Token": "foo"
  }
}
```

If the API call succeeds a 200 status code is returned.

### <a name="catalog_datacenters"></a> /v1/catalog/datacenters

This endpoint is hit with a `GET` and is used to return all the
datacenters that are known by the Consul server.

The datacenters will be sorted in ascending order based on the
estimated median round trip time from the server to the servers
in that datacenter.

It returns a JSON body like this:

```javascript
["dc1", "dc2"]
```

This endpoint does not require a cluster leader and
will succeed even during an availability outage. Therefore, it can be
used as a simple check to see if any Consul servers are routable.

### <a name="catalog_nodes"></a> /v1/catalog/nodes

This endpoint is hit with a `GET` and returns the nodes registered
in a given DC. By default, the datacenter of the agent is queried;
however, the `dc` can be provided using the `?dc=` query parameter.

Adding the optional `?near=` parameter with a node name will sort
the node list in ascending order based on the estimated round trip
time from that node. Passing `?near=_agent` will use the agent's
node for the sort.

In Consul 0.7.3 and later, the optional `?node-meta=` parameter can be
provided with a desired node metadata key/value pair of the form `key:value`.
This parameter can be specified multiple times, and will filter the results to
nodes with the specified key/value pair(s).

It returns a JSON body like this:

```javascript
[
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "baz",
    "Address": "10.1.10.11",
    "TaggedAddresses": {
      "lan": "10.1.10.11",
      "wan": "10.1.10.11"
    },
    "Meta": {
      "instance_type": "t2.medium"
    }
  },
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Node": "foobar",
    "Address": "10.1.10.12",
    "TaggedAddresses": {
      "lan": "10.1.10.11",
      "wan": "10.1.10.12"
    },
    "Meta": {
      "instance_type": "t2.large"
    }
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_services"></a> /v1/catalog/services

This endpoint is hit with a `GET` and returns the services registered
in a given DC. By default, the datacenter of the agent is queried;
however, the `dc` can be provided using the `?dc=` query parameter.

In Consul 0.7.3 and later, the optional `?node-meta=` parameter can be
provided with a desired node metadata key/value pair of the form `key:value`.
This parameter can be specified multiple times, and will filter the results to
services on nodes with the specified key/value pair(s).

It returns a JSON body like this:

```javascript
{
  "consul": [],
  "redis": [],
  "postgresql": [
    "primary",
    "secondary"
  ]
}
```

The keys are the service names, and the array values provide all known
tags for a given service.

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_service"></a> /v1/catalog/service/\<service\>

This endpoint is hit with a `GET` and returns the nodes providing a service
in a given DC. By default, the datacenter of the agent is queried;
however, the `dc` can be provided using the `?dc=` query parameter.

The service being queried must be provided on the path. By default
all nodes in that service are returned. However, the list can be filtered
by tag using the `?tag=` query parameter.

Adding the optional `?near=` parameter with a node name will sort
the node list in ascending order based on the estimated round trip
time from that node. Passing `?near=_agent` will use the agent's
node for the sort.

In Consul 0.7.3 and later, the optional `?node-meta=` parameter can be
provided with a desired node metadata key/value pair of the form `key:value`.
This parameter can be specified multiple times, and will filter the results to
service entries on nodes with the specified key/value pair(s).

It returns a JSON body like this:

```javascript
[
  {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "foobar",
    "Address": "192.168.10.10",
    "TaggedAddresses": {
      "lan": "192.168.10.10",
      "wan": "10.0.10.10"
    },
    "Meta": {
      "instance_type": "t2.medium"
    }
    "CreateIndex": 51,
    "ModifyIndex": 51,
    "ServiceAddress": "172.17.0.3",
    "ServiceEnableTagOverride": false,
    "ServiceID": "32a2a47f7992:nodea:5000",
    "ServiceName": "foobar",
    "ServicePort": 5000,
    "ServiceTags": [
      "tacos"
    ]
  }
]
```

This endpoint supports blocking queries and all consistency modes.

The returned fields are as follows:

- `Address`: IP address of the Consul node on which the service is registered
- `TaggedAddresses`: List of explicit LAN and WAN IP addresses for the agent
- `Meta`: Added in Consul 0.7.3, a list of user-defined metadata key/value pairs for the node
- `CreateIndex`: Internal index value representing when the service was created
- `ModifyIndex`: Last index that modified the service
- `Node`: Node name of the Consul node on which the service is registered
- `ServiceAddress`: IP address of the service host — if empty, node address should be used
- `ServiceEnableTagOverride`: Whether service tags can be overridden on this service
- `ServiceID`: A unique service instance identifier
- `ServiceName`: Name of the service
- `ServicePort`: Port number of the service
- `ServiceTags`: List of tags for the service

### <a name="catalog_node"></a> /v1/catalog/node/\<node\>

This endpoint is hit with a `GET` and returns the node's registered services.
By default, the datacenter of the agent is queried;
however, the `dc` can be provided using the `?dc=` query parameter.
The node being queried must be provided on the path.

It returns a JSON body like this:

```javascript
{
  "Node": {
    "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
    "Node": "foobar",
    "Address": "10.1.10.12",
    "TaggedAddresses": {
      "lan": "10.1.10.12",
      "wan": "10.1.10.12"
    },
    "Meta": {
      "instance_type": "t2.medium"
    }
  },
  "Services": {
    "consul": {
      "ID": "consul",
      "Service": "consul",
      "Tags": null,
      "Port": 8300
    },
    "redis": {
      "ID": "redis",
      "Service": "redis",
      "Tags": [
        "v1"
      ],
      "Port": 8000
    }
  }
}
```

This endpoint supports blocking queries and all consistency modes.
