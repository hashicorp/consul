---
layout: "docs"
page_title: "Catalog (HTTP)"
sidebar_current: "docs-agent-http-catalog"
description: >
  The Catalog is the endpoint used to register and deregister nodes,
  services, and checks. It also provides a number of query endpoints.
---

# Catalog HTTP Endpoint

The Catalog is the endpoint used to register and deregister nodes,
services, and checks. It also provides a number of query endpoints.

The following endpoints are supported:

* [`/v1/catalog/register`](#catalog_register) : Registers a new node, service, or check
* [`/v1/catalog/deregister`](#catalog_deregister) : Deregisters a node, service, or check
* [`/v1/catalog/datacenters`](#catalog_datacenters) : Lists known datacenters
* [`/v1/catalog/nodes`](#catalog_nodes) : Lists nodes in a given DC
* [`/v1/catalog/services`](#catalog_services) : Lists services in a given DC
* [`/v1/catalog/service/<service>`](#catalog_service) : Lists the nodes in a given service
* [`/v1/catalog/node/<node>`](#catalog_nodes) : Lists the services provided by a node

The last 4 endpoints of the catalog support blocking queries and
consistency modes.

### <a name="catalog_register"></a> /v1/catalog/register

The register endpoint is a low level mechanism for directly registering
or updating entries in the catalog. It is usually recommended to use
the agent local endpoints, as they are simpler and perform anti-entropy.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "Address": "192.168.10.10",
  "Service": {
    "ID": "redis1",
    "Service": "redis",
    "Tags": [
      "master",
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
requires `Node` and `Address` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only those are provided, the endpoint will register
the node with the catalog.

If the `Service` key is provided, then the service will also be registered. If
`ID` is not provided, it will be defaulted to `Service`. It is mandated that the
ID be node-unique. The `Tags`, `Address` and `Port` fields can be omitted.

If the `Check` key is provided, then a health check will also be registered. It
is important to remember that this register API is very low level. This manipulates
the health check entry, but does not setup a script or TTL to actually update the
status. For that behavior, an agent local check should be setup.

The `CheckID` can be omitted, and will default to the `Name`. Like before, the
`CheckID` must be node-unique. The `Notes` is an opaque field that is meant to
hold human readable text. If a `ServiceID` is provided that matches the `ID`
of a service on that node, then the check is treated as a service level health
check, instead of a node level health check. The `Status` must be one of
"unknown", "passing", "warning", or "critical". The "unknown" status is used
to indicate that the initial check has not been performed yet.

It is important to note that `Check` does not have to be provided with `Service`
and visa-versa. They can be provided or omitted at will.

If the API call succeeds a 200 status code is returned.

### <a name="catalog_deregister"></a> /v1/catalog/deregister

The deregister endpoint is a low level mechanism for directly removing
entries in the catalog. It is usually recommended to use the agent local
endpoints, as they are simpler and perform anti-entropy.

The deregister endpoint expects a JSON request body to be PUT. The request
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
requires `Node` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only `Node` is provided, then the node, and
all associated services and checks are deleted. If `CheckID` is provided, only
that check belonging to the node is removed. If `ServiceID` is provided, then the
service along with its associated health check (if any) is removed.

If the API call succeeds a 200 status code is returned.


### <a name="catalog_datacenters"></a> /v1/catalog/datacenters

This endpoint is hit with a GET and is used to return all the
datacenters that are known by the Consul server.

It returns a JSON body like this:

```javascript
["dc1", "dc2"]
```

This endpoint does not require a cluster leader, and as such
will succeed even during an availability outage. It can thus be
a simple check to see if any Consul servers are routable.

### <a name="catalog_nodes"></a> /v1/catalog/nodes

This endpoint is hit with a GET and returns the nodes known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "baz",
    "Address": "10.1.10.11"
  },
  {
    "Node": "foobar",
    "Address": "10.1.10.12"
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_services"></a> /v1/catalog/services

This endpoint is hit with a GET and returns the services known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
{
  "consul": [],
  "redis": [],
  "postgresql": [
    "master",
    "slave"
  ]
}
```

The main object keys are the service names, while the array
provides all the known tags for a given service.

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_service"></a> /v1/catalog/service/\<service\>

This endpoint is hit with a GET and returns the nodes providing a service
in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The service being queried must be provided after the slash. By default
all nodes in that service are returned. However, the list can be filtered
by tag using the "?tag=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "foobar",
    "Address": "10.1.10.12",
    "ServiceID": "redis",
    "ServiceName": "redis",
    "ServiceTags": null,
    "ServiceAddress": "",
    "ServicePort": 8000
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_node"></a> /v1/catalog/node/\<node\>

This endpoint is hit with a GET and returns the node provided services.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
{
  "Node": {
    "Node": "foobar",
    "Address": "10.1.10.12"
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
